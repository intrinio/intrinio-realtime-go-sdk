package composite

import (
	"encoding/json"
	"fmt"
	"github.com/intrinio/intrinio-realtime-go-sdk"
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// GreekClient calculates real-time Greeks from a stream of equities and options trades and quotes
type GreekClient struct {
	cache                       DataCache
	dividendYieldKey            string
	riskFreeInterestRateKey     string
	blackScholesKey             string
	calcLookup                  map[string]CalculateNewGreek
	updateSupplementalDatumFunc SupplementalDatumUpdate
	updateGreekDataFunc         GreekDataUpdate
	seenTickers                 map[string]time.Time
	dividendYieldWorking        bool
	selfCache                   bool
	mu                          sync.RWMutex
	apiKey                      string
}

// NewGreekClient creates a new GreekClient instance
func NewGreekClient(greekUpdateFrequency GreekUpdateFrequency, onGreekValueUpdated OnOptionsContractSupplementalDatumUpdated, apiKey string, cache DataCache) *GreekClient {
	if cache == nil {
		cache = NewDataCache()
	}

	client := &GreekClient{
		cache:                       cache,
		dividendYieldKey:            "DividendYield",
		riskFreeInterestRateKey:     "RiskFreeInterestRate",
		blackScholesKey:             "IntrinioBlackScholes",
		calcLookup:                  make(map[string]CalculateNewGreek),
		updateSupplementalDatumFunc: func(key string, oldValue, newValue *float64) *float64 { return newValue },
		updateGreekDataFunc:         func(key string, oldValue, newValue *Greek) *Greek { return newValue },
		seenTickers:                 make(map[string]time.Time),
		dividendYieldWorking:        false,
		selfCache:                   cache == nil,
		apiKey:                      apiKey,
	}

	// Set up callbacks based on update frequency
	if greekUpdateFrequency.Has(EveryOptionsTradeUpdate) {
		cache.SetOptionsTradeUpdatedCallback(client.updateGreeksForOptionsContractTrade)
	}

	if greekUpdateFrequency.Has(EveryOptionsQuoteUpdate) {
		cache.SetOptionsQuoteUpdatedCallback(client.updateGreeksForOptionsContractQuote)
	}

	if greekUpdateFrequency.Has(EveryDividendYieldUpdate) {
		cache.SetSecuritySupplementalDatumUpdatedCallback(client.updateGreeksSecuritySupplementalDatumUpdated)
	}

	if greekUpdateFrequency.Has(EveryRiskFreeInterestRateUpdate) {
		cache.SetSupplementalDatumUpdatedCallback(client.updateGreeks)
	}

	if greekUpdateFrequency.Has(EveryEquityTradeUpdate) {
		cache.SetEquitiesTradeUpdatedCallback(client.updateGreeksForSecurityTrade)
	}

	if greekUpdateFrequency.Has(EveryEquityQuoteUpdate) {
		cache.SetEquitiesQuoteUpdatedCallback(client.updateGreeksForSecurityQuote)
	}

	// Set the Greek value updated callback
	cache.SetOptionsContractSupplementalDatumUpdatedCallback(onGreekValueUpdated)

	return client
}

// Start starts the Greek client
func (g *GreekClient) Start() {

}

// Stop stops the Greek client
func (g *GreekClient) Stop() {
	// Cleanup if needed
}

// OnTrade handles equities trade updates
func (g *GreekClient) OnTrade(trade *intrinio.EquityTrade) {
	if trade != nil {
		g.cache.SetEquityTrade(trade)
	}
}

// OnQuote handles equities quote updates
func (g *GreekClient) OnQuote(quote *intrinio.EquityQuote) {
	if quote != nil {
		g.cache.SetEquityQuote(quote)
	}
}

// OnTrade handles options trade updates
func (g *GreekClient) OnOptionsTrade(trade *intrinio.OptionTrade) {
	if trade != nil {
		g.cache.SetOptionsTrade(trade)
	}
}

// OnQuote handles options quote updates
func (g *GreekClient) OnOptionsQuote(quote *intrinio.OptionQuote) {
	if quote != nil {
		g.cache.SetOptionsQuote(quote)
	}
}

// OnRefresh handles options refresh updates
func (g *GreekClient) OnRefresh(refresh *intrinio.OptionRefresh) {
	if refresh != nil {
		g.cache.SetOptionsRefresh(refresh)
	}
}

// OnUnusualActivity handles options unusual activity updates
func (g *GreekClient) OnUnusualActivity(unusualActivity *OptionsUnusualActivity) {
	if unusualActivity != nil {
		g.cache.SetOptionsUnusualActivity(unusualActivity)
	}
}

// TryAddOrUpdateGreekCalculation adds or updates a Greek calculation function
func (g *GreekClient) TryAddOrUpdateGreekCalculation(name string, calc CalculateNewGreek) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.calcLookup[name] = calc
	return true
}

// AddBlackScholes adds the Black-Scholes Greek calculation
func (g *GreekClient) AddBlackScholes() {
	g.TryAddOrUpdateGreekCalculation("BlackScholes", g.blackScholesCalc)
}

func (g *GreekClient) FetchRiskFreeInterestRate() {
	success := false
	tryCount := 0

	log.Printf("Getting Risk Free Rate")

	for success == false && tryCount < 10 {
		tryCount++

		resp, err := http.Get(fmt.Sprintf("https://api-v2.intrinio.com/indices/economic/$DTB3/data_point/level?&api_key=%s", g.apiKey))

		if err != nil {
			fmt.Printf("Unable to retrieve Risk Free Rate attempt %i", tryCount)
		} else {
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)

			if err == nil {
				bodyString := string(body)
				rate, err := strconv.ParseFloat(bodyString, 64)

				if err == nil {
					adjRate := rate / 100

					log.Printf("Setting Risk Free Rate to %v", adjRate)

					g.cache.SetSupplementaryDatum(g.riskFreeInterestRateKey, &adjRate, func(key string, oldValue, newValue *float64) *float64 {
						return newValue
					})
					success = true
				}
			}
		}
	}
}

func (g *GreekClient) FetchDividendYields() {
	g.fetchBulkCompanyDividendYield()
	g.FetchMissingDividendYields()
}

// This loadsd dividend yields for ETFs
func (g *GreekClient) FetchMissingDividendYields() {
	securities := g.cache.GetAllSecurityData()

	for _, security := range securities {
		g.FetchDividendYieldForSecurity(security)
	}
}

func (g *GreekClient) FetchDividendYieldForSecurity(security SecurityData) {
	if security.GetSupplementaryDatum(g.dividendYieldKey) != nil {
		return
	}

	ticker := security.GetTickerSymbol()

	g.FetchDividendYieldForTicker(ticker)
}

func (g *GreekClient) FetchDividendYieldForTicker(ticker string) {
	success := false
	tryCount := 0

	for tryCount < 3 && success == false {
		tryCount++

		resp, err := http.Get(fmt.Sprintf("https://api-v2.intrinio.com/securities/%s/data_point/trailing_dividend_yield?api_key=%s", ticker, g.apiKey))

		if err == nil {
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)

			if err == nil {
				bodyString := string(body)
				dividendYield, err := strconv.ParseFloat(bodyString, 64)

				if err == nil {
					g.cache.SetSecuritySupplementalDatum(ticker, g.dividendYieldKey, &dividendYield, g.updateSupplementalDatumFunc)
					success = true
					break
				} else {
					// Unable to set dividend yield
				}
			}
		} else {
			// Unable to set dividend yield
		}
	}
}

// Company dividend yield can be grabbed in bulk
func (g *GreekClient) fetchBulkCompanyDividendYield() {
	success := false
	tryCount := 0

	for success == false && tryCount < 5 {
		tryCount++

		resp, err := http.Get(fmt.Sprintf("https://api-v2.intrinio.com/companies/daily_metrics?page_size=10000&api_key=%s", g.apiKey))

		if err != nil {
			fmt.Printf("Unable to retrieve Dividend Yield attempt %i", tryCount)
		} else {
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)

			if err == nil {
				var companyMetricResponse DailyMetricResponse
				err := json.Unmarshal(body, &companyMetricResponse) // don't forget to check the error

				if err == nil {
					success = true

					for _, metric := range companyMetricResponse.DailyMetrics {
						yield, err := strconv.ParseFloat(metric.DividendYield, 64)

						if err == nil {
							g.cache.SetSecuritySupplementalDatum(metric.Company.Ticker, g.dividendYieldKey, &yield, func(key string, oldValue, newValue *float64) *float64 {
								return newValue
							})

						} else {
							// Unable to set dividend yield, proably null
						}
					}
				} else {
					log.Printf("-------------ERROR----------")
					log.Printf("Unable to parse json")
					log.Printf("%v", err)
					log.Printf("----------------------------")
				}
			}
		}
	}
}

// updateGreeks updates Greeks for all relevant data
func (g *GreekClient) updateGreeks(key string, datum *float64, dataCache DataCache) {
	// Update Greeks for all securities when risk-free rate changes
	if key == g.riskFreeInterestRateKey {
		allSecurities := dataCache.GetAllSecurityData()
		for _, securityData := range allSecurities {
			g.updateGreeksForSecurity(securityData, dataCache)
		}
	}

}

// updateGreeksForSecurity updates Greeks for a specific security
func (g *GreekClient) updateGreeksForSecurity(securityData SecurityData, dataCache DataCache) {
	// Get all options contracts for this security
	allOptionsContracts := securityData.GetAllOptionsContractData()
	for _, optionsContractData := range allOptionsContracts {
		g.updateGreeksForOptionsContract(optionsContractData, dataCache, securityData)
	}
}

// updateGreeksForSecurity updates Greeks for a specific security
func (g *GreekClient) updateGreeksForSecurityTrade(securityData SecurityData, dataCache DataCache, trade *intrinio.EquityTrade) {
	// Get all options contracts for this security
	allOptionsContracts := securityData.GetAllOptionsContractData()
	for _, optionsContractData := range allOptionsContracts {
		g.updateGreeksForOptionsContract(optionsContractData, dataCache, securityData)
	}
}

// updateGreeksForSecurity updates Greeks for a specific security
func (g *GreekClient) updateGreeksForSecurityQuote(securityData SecurityData, dataCache DataCache, quote *intrinio.EquityQuote) {
	// Get all options contracts for this security
	allOptionsContracts := securityData.GetAllOptionsContractData()
	for _, optionsContractData := range allOptionsContracts {
		g.updateGreeksForOptionsContract(optionsContractData, dataCache, securityData)
	}
}

// updateGreeksForOptionsContract updates Greeks for a specific options contract
func (g *GreekClient) updateGreeksForOptionsContract(optionsContractData OptionsContractData, dataCache DataCache, securityData SecurityData) {
	// Execute all registered calculation functions
	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, calc := range g.calcLookup {
		calc(optionsContractData, securityData, dataCache)
	}
}

// updateGreeksForOptionsContract updates Greeks for a specific options contract
func (g *GreekClient) updateGreeksForOptionsContractTrade(optionsContractData OptionsContractData, dataCache DataCache, securityData SecurityData, trade *intrinio.OptionTrade) {
	// Execute all registered calculation functions
	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, calc := range g.calcLookup {
		calc(optionsContractData, securityData, dataCache)
	}
}

// updateGreeksForOptionsContract updates Greeks for a specific options contract
func (g *GreekClient) updateGreeksForOptionsContractQuote(optionsContractData OptionsContractData, dataCache DataCache, securityData SecurityData, quote *intrinio.OptionQuote) {
	// Execute all registered calculation functions
	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, calc := range g.calcLookup {
		calc(optionsContractData, securityData, dataCache)
	}
}

func (g *GreekClient) updateGreeksSecuritySupplementalDatumUpdated(key string, datum *float64, securityData SecurityData, dataCache DataCache) {
	// Update Greeks for all options contracts of this security
	allOptionsContracts := securityData.GetAllOptionsContractData()
	for _, optionsContractData := range allOptionsContracts {
		g.updateGreeksForOptionsContract(optionsContractData, dataCache, securityData)
	}
}

// blackScholesCalc performs Black-Scholes Greek calculations
func (g *GreekClient) blackScholesCalc(optionsContractData OptionsContractData, securityData SecurityData, dataCache DataCache) {
	// Get required data
	latestTrade := optionsContractData.GetLatestTrade()
	latestQuote := optionsContractData.GetLatestQuote()
	underlyingTrade := securityData.GetLatestEquitiesTrade()

	if latestTrade == nil || latestQuote == nil || underlyingTrade == nil {
		return
	}

	// Get market data
	riskFreeRate := dataCache.GetSupplementaryDatum(g.riskFreeInterestRateKey)
	dividendYield := securityData.GetSupplementaryDatum(g.dividendYieldKey)

	if riskFreeRate == nil {
		riskFreeRate = float64Ptr(0.0416) // Default
	}
	if dividendYield == nil {
		dividendYield = float64Ptr(0.0) // Default 0%
	}

	strike := (g.getStrikePrice(latestQuote.ContractId))
	isPut := g.isPut(latestQuote.ContractId)
	yearsToExpiration := g.getYearsToExpiration(latestTrade, latestQuote)

	// Calculate Greeks using Black-Scholes
	calculator := &BlackScholesGreekCalculator{}
	greek := calculator.Calculate(*riskFreeRate, *dividendYield, float64(underlyingTrade.Price), float64((latestQuote.AskPrice+latestQuote.BidPrice)/2.0), strike, isPut, yearsToExpiration)

	if greek.IsValid {
		// Store calculated Greeks
		contract := optionsContractData.GetContract()
		tickerSymbol := securityData.GetTickerSymbol()

		dataCache.SetOptionGreekData(tickerSymbol, contract, g.blackScholesKey, &greek, g.updateGreekDataFunc)
	}
}

// getYearsToExpiration calculates the years to expiration
func (b *GreekClient) getYearsToExpiration(latestOptionTrade *intrinio.OptionTrade, latestOptionQuote *intrinio.OptionQuote) float64 {
	// Use the expiration date from the contract
	expirationDate := b.getExpirationDate(latestOptionTrade.ContractId)
	now := time.Now()

	diff := expirationDate.Sub(now).Seconds()
	if diff <= 0.0 {
		return 0.0
	}
	return diff / 31557600.0
}

// getExpirationDate extracts the expiration date from the contract identifier
func (b *GreekClient) getExpirationDate(contract string) time.Time {
	if len(contract) < 12 {
		return time.Time{}
	}

	// Extract date from contract (format: AAPL__201016C00100000)
	dateStr := contract[6:12]

	// Parse date in format "yyMMdd"
	expirationDate, err := time.Parse("060102", dateStr)
	if err != nil {
		return time.Time{}
	}

	return expirationDate
}

// isPut checks if the option is a put
func (b *GreekClient) isPut(contract string) bool {
	if len(contract) < 13 {
		return false
	}
	return contract[12] == 'P'
}

// getStrikePrice extracts the strike price from the contract identifier
func (b *GreekClient) getStrikePrice(contract string) float64 {
	if len(contract) < 19 {
		return 0.0
	}

	// Extract strike price from contract (format: AAPL__201016C00100000)
	strikeStr := contract[13:19]

	var whole uint32
	for i := 0; i < 5; i++ {
		whole += uint32(strikeStr[i]-'0') * uint32(math.Pow10(4-i))
	}

	part := float64(strikeStr[5]-'0') * 0.1

	return float64(whole) + part
}

// Helper function to create float64 pointers
func float64Ptr(v float64) *float64 {
	return &v
}
