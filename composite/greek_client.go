package composite

import (
	"sync"
	"time"
	"github.com/intrinio/intrinio-realtime-go-sdk"
	"net/http"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"log"
)

// GreekClient calculates real-time Greeks from a stream of equities and options trades and quotes
type GreekClient struct {
	cache                           DataCache
	blackScholesImpliedVolatilityKey string
	blackScholesDeltaKey             string
	blackScholesGammaKey             string
	blackScholesThetaKey             string
	blackScholesVegaKey              string
	dividendYieldKey                 string
	riskFreeInterestRateKey          string
	blackScholesKey                  string
	calcLookup                       map[string]CalculateNewGreek
	updateFunc                       SupplementalDatumUpdate
	seenTickers                      map[string]time.Time
	dividendYieldWorking             bool
	selfCache                        bool
	mu                               sync.RWMutex
	apiKey							 string
}

// NewGreekClient creates a new GreekClient instance
func NewGreekClient(greekUpdateFrequency GreekUpdateFrequency, onGreekValueUpdated OnOptionsContractSupplementalDatumUpdated, apiKey string, cache DataCache) *GreekClient {
	if cache == nil {
		cache = NewDataCache()
	}
	
	client := &GreekClient{
		cache:                           cache,
		blackScholesImpliedVolatilityKey: "IntrinioBlackScholesImpliedVolatility",
		blackScholesDeltaKey:             "IntrinioBlackScholesDelta",
		blackScholesGammaKey:             "IntrinioBlackScholesGamma",
		blackScholesThetaKey:             "IntrinioBlackScholesTheta",
		blackScholesVegaKey:              "IntrinioBlackScholesVega",
		dividendYieldKey:                 "DividendYield",
		riskFreeInterestRateKey:          "RiskFreeInterestRate",
		blackScholesKey:                  "IntrinioBlackScholes",
		calcLookup:                       make(map[string]CalculateNewGreek),
		seenTickers:                      make(map[string]time.Time),
		updateFunc:                       func(key string, oldValue, newValue *float64) *float64 { return newValue },
		selfCache:                        cache == nil,
		apiKey:							  apiKey,
	}
	
	// Set up callbacks based on update frequency
	if greekUpdateFrequency.Has(EveryOptionsTradeUpdate) {
		cache.SetOptionsTradeUpdatedCallback(client.updateGreeksForOptionsContract)
	}
	
	if greekUpdateFrequency.Has(EveryOptionsQuoteUpdate) {
		cache.SetOptionsQuoteUpdatedCallback(client.updateGreeksForOptionsContract)
	}
	
	if greekUpdateFrequency.Has(EveryDividendYieldUpdate) {
		cache.SetSecuritySupplementalDatumUpdatedCallback(client.updateGreeksSecuritySupplementalDatumUpdated)
	}
	
	if greekUpdateFrequency.Has(EveryRiskFreeInterestRateUpdate) {
		cache.SetSupplementalDatumUpdatedCallback(client.updateGreeks)
	}
	
	if greekUpdateFrequency.Has(EveryEquityTradeUpdate) {
		cache.SetEquitiesTradeUpdatedCallback(client.updateGreeksForSecurity)
	}
	
	if greekUpdateFrequency.Has(EveryEquityQuoteUpdate) {
		cache.SetEquitiesQuoteUpdatedCallback(client.updateGreeksForSecurity)
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
		tryCount++;
		
		resp, err := http.Get(fmt.Sprintf("https://api-v2.intrinio.com/indices/economic/$DTB3/data_point/level?&api_key=%s", g.apiKey))
		
		if err != nil {
			fmt.Printf("Unable to retrieve Risk Free Rate attempt %i", tryCount)
		} else {
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)

			if (err == nil) {
				bodyString := string(body)
				rate, err := strconv.ParseFloat(bodyString, 64)

				if (err == nil) {
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
	if (security.GetSupplementaryDatum(g.dividendYieldKey) != nil) {
		return
	}

	ticker := security.GetTickerSymbol()
	
	g.FetchDividendYieldForTicker(ticker)
}

func (g *GreekClient) FetchDividendYieldForTicker(ticker string) {
	success := false
	tryCount := 0
	
	for (tryCount < 3 && success == false) {
		tryCount++

		resp, err := http.Get(fmt.Sprintf("https://api-v2.intrinio.com/securities/%s/data_point/trailing_dividend_yield?api_key=%s", ticker, g.apiKey))

		if (err == nil) {
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)

			if (err == nil) {
				bodyString := string(body)
				dividendYield, err := strconv.ParseFloat(bodyString, 64)

				if (err == nil) {
					g.cache.SetSecuritySupplementalDatum(ticker, g.dividendYieldKey, &dividendYield, g.updateFunc)
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
		tryCount++;
		
		resp, err := http.Get(fmt.Sprintf("https://api-v2.intrinio.com/companies/daily_metrics?page_size=10000&api_key=%s", g.apiKey))
		
		if err != nil {
			fmt.Printf("Unable to retrieve Dividend Yield attempt %i", tryCount)
		} else {
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)

			if (err == nil) {
				var companyMetricResponse DailyMetricResponse
				err := json.Unmarshal(body, &companyMetricResponse) // don't forget to check the error

				if (err == nil) {
					success = true

					for  _, metric := range companyMetricResponse.DailyMetrics {
						yield, err := strconv.ParseFloat(metric.DividendYield, 64)

						if (err == nil) {
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

// updateGreeksForOptionsContract updates Greeks for a specific options contract
func (g *GreekClient) updateGreeksForOptionsContract(optionsContractData OptionsContractData, dataCache DataCache, securityData SecurityData) {
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
	
	// Calculate Greeks using Black-Scholes
	calculator := &BlackScholesGreekCalculator{}
	greek := calculator.Calculate(*riskFreeRate, *dividendYield, underlyingTrade, latestTrade, latestQuote)
	
	if greek.IsValid {
		// Store calculated Greeks
		contract := optionsContractData.GetContract()
		tickerSymbol := securityData.GetTickerSymbol()
		
		dataCache.SetOptionSupplementalDatum(tickerSymbol, contract, g.blackScholesImpliedVolatilityKey, float64Ptr(greek.ImpliedVolatility), g.updateFunc)
		dataCache.SetOptionSupplementalDatum(tickerSymbol, contract, g.blackScholesDeltaKey, float64Ptr(greek.Delta), g.updateFunc)
		dataCache.SetOptionSupplementalDatum(tickerSymbol, contract, g.blackScholesGammaKey, float64Ptr(greek.Gamma), g.updateFunc)
		dataCache.SetOptionSupplementalDatum(tickerSymbol, contract, g.blackScholesThetaKey, float64Ptr(greek.Theta), g.updateFunc)
		dataCache.SetOptionSupplementalDatum(tickerSymbol, contract, g.blackScholesVegaKey, float64Ptr(greek.Vega), g.updateFunc)
	}
}



// Helper function to create float64 pointers
func float64Ptr(v float64) *float64 {
	return &v
} 