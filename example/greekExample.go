package main

import (
	"github.com/intrinio/intrinio-realtime-go-sdk"
	"github.com/intrinio/intrinio-realtime-go-sdk/composite"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// GreekSampleApp demonstrates real-time Greek calculations
type GreekSampleApp struct {
	timer                 *time.Ticker
	greekClient           *composite.GreekClient
	dataCache             composite.DataCache
	seenGreekTickers      map[string]string
	seenGreekTickersMutex sync.RWMutex

	// Event counters
	optionsTradeEventCount  uint64
	optionsQuoteEventCount  uint64
	equitiesTradeEventCount uint64
	equitiesQuoteEventCount uint64
	greekUpdatedEventCount  uint64

	// Mock data generation
	stopChan chan bool
	wg       sync.WaitGroup
}

// NewGreekSampleApp creates a new Greek sample app
func NewGreekSampleApp() *GreekSampleApp {
	return &GreekSampleApp{
		seenGreekTickers: make(map[string]string),
		stopChan:         make(chan bool),
	}
}

// OnOptionsQuote handles options quote updates
func (g *GreekSampleApp) OnOptionsQuote(quote intrinio.OptionQuote) {
	atomic.AddUint64(&g.optionsQuoteEventCount, 1)
	g.dataCache.SetOptionsQuote(&quote)
}

// OnOptionsTrade handles options trade updates
func (g *GreekSampleApp) OnOptionsTrade(trade intrinio.OptionTrade) {
	atomic.AddUint64(&g.optionsTradeEventCount, 1)
	g.dataCache.SetOptionsTrade(&trade)
}

// OnEquitiesQuote handles equities quote updates
func (g *GreekSampleApp) OnEquitiesQuote(quote intrinio.EquityQuote) {
	atomic.AddUint64(&g.equitiesQuoteEventCount, 1)
	g.dataCache.SetEquityQuote(&quote)
}

// OnEquitiesTrade handles equities trade updates
func (g *GreekSampleApp) OnEquitiesTrade(trade intrinio.EquityTrade) {
	atomic.AddUint64(&g.equitiesTradeEventCount, 1)
	g.dataCache.SetEquityTrade(&trade)
}

// OnGreek handles Greek calculation updates
func (g *GreekSampleApp) OnGreek(key string, datum *composite.Greek, optionsContractData composite.OptionsContractData, securityData composite.SecurityData, dataCache composite.DataCache) {
	atomic.AddUint64(&g.greekUpdatedEventCount, 1)

	g.seenGreekTickersMutex.Lock()
	g.seenGreekTickers[securityData.GetTickerSymbol()] = optionsContractData.GetContract()
	g.seenGreekTickersMutex.Unlock()
}

// timerCallback prints statistics every minute
func (g *GreekSampleApp) timerCallback() {
	log.Printf("=== Statistics Update ===")
	log.Printf("Options Trade Events: %d", atomic.LoadUint64(&g.optionsTradeEventCount))
	log.Printf("Options Quote Events: %d", atomic.LoadUint64(&g.optionsQuoteEventCount))
	log.Printf("Equities Trade Events: %d", atomic.LoadUint64(&g.equitiesTradeEventCount))
	log.Printf("Equities Quote Events: %d", atomic.LoadUint64(&g.equitiesQuoteEventCount))
	log.Printf("Greek Updates: %d", atomic.LoadUint64(&g.greekUpdatedEventCount))

	allSecurityData := g.dataCache.GetAllSecurityData()
	log.Printf("Data Cache Security Count: %d", len(allSecurityData))

	// Count securities with dividend yield
	dividendYieldCount := 0
	for _, securityData := range allSecurityData {
		if securityData.GetSupplementaryDatum("DividendYield") != nil {
			dividendYieldCount++
		}
	}
	log.Printf("Dividend Yield Count: %d", dividendYieldCount)

	g.seenGreekTickersMutex.RLock()
	uniqueSecuritiesCount := len(g.seenGreekTickers)
	g.seenGreekTickersMutex.RUnlock()
	log.Printf("Unique Securities with Greeks Count: %d", uniqueSecuritiesCount)
	log.Printf("=== End Statistics ===\n")
}

// Run starts the Greek sample app
func (g *GreekSampleApp) runGreekExample() error {
	log.Println("Starting Greek sample app")

	//symbols := []string{"AAPL", "MSFT", "SPY", "QQQ"}
	symbols := []string{"JPM", "SPY", "QQQ", "AAPL", "NVDA"}

	// Create data cache
	g.dataCache = composite.NewDataCache()

	var equitiesConfig intrinio.Config = intrinio.LoadConfig("equities-config.json")
	var optionsConfig intrinio.Config = intrinio.LoadConfig("options-config.json")

	// Set up Greek update frequency
	updateFrequency := composite.EveryDividendYieldUpdate |
		composite.EveryRiskFreeInterestRateUpdate |
		composite.EveryOptionsTradeUpdate |
		composite.EveryEquityTradeUpdate

	// Create Greek client
	g.greekClient = composite.NewGreekClient(updateFrequency, g.OnGreek, optionsConfig.ApiKey, g.dataCache)

	// Load static data from REST API
	g.greekClient.FetchRiskFreeInterestRate()
	g.greekClient.FetchDividendYields()

	if optionsConfig.Provider == intrinio.OPTIONS_EDGE {
		g.greekClient.AddBlackScholesOptionsEdge()
	} else {
		g.greekClient.AddBlackScholes()
	}

	g.greekClient.Start()

	for _, symbol := range symbols {
		g.greekClient.FetchDividendYieldForTicker(symbol)
	}

	// Start statistics timer
	g.timer = time.NewTicker(60 * time.Second)
	go func() {
		for {
			select {
			case <-g.stopChan:
				return
			case <-g.timer.C:
				g.timerCallback()
			}
		}
	}()

	var equitiesClient *intrinio.Client = intrinio.NewEquitiesClient(equitiesConfig, g.OnEquitiesTrade, g.OnEquitiesQuote)
	equitiesClient.Start()
	equitiesClient.JoinMany(symbols)

	var optionsClient *intrinio.Client = intrinio.NewOptionsClient(optionsConfig, g.OnOptionsTrade, g.OnOptionsQuote, nil, nil)
	optionsClient.Start()
	optionsClient.JoinMany(symbols)

	// Wait for interrupt signal
	log.Println("Greek sample app running. Press Ctrl+C to stop.")

	// Keep the app running
	select {
	case <-g.stopChan:
		break
	}

	return nil
}

// Stop stops the Greek sample app
func (g *GreekSampleApp) Stop() {
	log.Println("Stopping Greek sample app")

	// Stop the timer
	if g.timer != nil {
		g.timer.Stop()
	}

	// Stop mock data generation
	close(g.stopChan)

	// Stop Greek client
	if g.greekClient != nil {
		g.greekClient.Stop()
	}

	// Wait for all goroutines to finish
	g.wg.Wait()

	log.Println("Greek sample app stopped")
}

// Helper function to extract ticker symbol from contract
func extractTickerFromContract(contract string) string {
	if len(contract) < 6 {
		return ""
	}

	// Find the first underscore sequence
	for i := 0; i < len(contract)-1; i++ {
		if contract[i] == '_' && contract[i+1] == '_' {
			return contract[:i]
		}
	}

	// Fallback: take first 6 characters
	if len(contract) >= 6 {
		return contract[:6]
	}

	return ""
}
