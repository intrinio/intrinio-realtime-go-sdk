package composite

import (
	"sync"
	"github.com/intrinio/intrinio-realtime-go-sdk"
)

// dataCache implements the DataCache interface
type dataCache struct {
	securities                      map[string]SecurityData
	securitiesMutex                 sync.RWMutex
	supplementaryData               map[string]*float64
	supplementaryDataMutex          sync.RWMutex
	
	// Callbacks
	supplementalDatumUpdatedCallback                    OnSupplementalDatumUpdated
	securitySupplementalDatumUpdatedCallback            OnSecuritySupplementalDatumUpdated
	optionsContractSupplementalDatumUpdatedCallback     OnOptionsContractSupplementalDatumUpdated
	
	equitiesTradeUpdatedCallback                        OnEquitiesTradeUpdated
	equitiesQuoteUpdatedCallback                        OnEquitiesQuoteUpdated
	equitiesTradeCandleStickUpdatedCallback             OnEquitiesTradeCandleStickUpdated
	equitiesQuoteCandleStickUpdatedCallback             OnEquitiesQuoteCandleStickUpdated
	
	optionsTradeUpdatedCallback                         OnOptionsTradeUpdated
	optionsQuoteUpdatedCallback                         OnOptionsQuoteUpdated
	optionsRefreshUpdatedCallback                       OnOptionsRefreshUpdated
	optionsUnusualActivityUpdatedCallback               OnOptionsUnusualActivityUpdated
	optionsTradeCandleStickUpdatedCallback              OnOptionsTradeCandleStickUpdated
	optionsQuoteCandleStickUpdatedCallback              OnOptionsQuoteCandleStickUpdated
}

// NewDataCache creates a new DataCache instance
func NewDataCache() DataCache {
	return &dataCache{
		securities:             make(map[string]SecurityData),
		supplementaryData:      make(map[string]*float64),
	}
}

// GetSupplementaryDatum returns a supplementary datum
func (d *dataCache) GetSupplementaryDatum(key string) *float64 {
	d.supplementaryDataMutex.RLock()
	defer d.supplementaryDataMutex.RUnlock()
	
	if value, exists := d.supplementaryData[key]; exists {
		return value
	}
	return nil
}

// SetSupplementaryDatum sets a supplementary datum
func (d *dataCache) SetSupplementaryDatum(key string, datum *float64, update SupplementalDatumUpdate) bool {
	d.supplementaryDataMutex.Lock()
	defer d.supplementaryDataMutex.Unlock()
	
	oldValue := d.supplementaryData[key]
	newValue := update(key, oldValue, datum)
	
	if newValue != oldValue {
		d.supplementaryData[key] = newValue
		
		// Call callback if set
		if d.supplementalDatumUpdatedCallback != nil {
			go func() {
				defer func() {
					if r := recover(); r != nil {
						// Log error here if logging is available
					}
				}()
				d.supplementalDatumUpdatedCallback(key, datum, d)
			}()
		}
		return true
	}
	return false
}

// GetAllSupplementaryData returns all supplementary data
func (d *dataCache) GetAllSupplementaryData() map[string]*float64 {
	d.supplementaryDataMutex.RLock()
	defer d.supplementaryDataMutex.RUnlock()
	
	result := make(map[string]*float64)
	for k, v := range d.supplementaryData {
		result[k] = v
	}
	return result
}

// GetSecuritySupplementalDatum returns a security supplemental datum
func (d *dataCache) GetSecuritySupplementalDatum(tickerSymbol, key string) *float64 {
	d.securitiesMutex.RLock()
	defer d.securitiesMutex.RUnlock()
	
	if securityData, exists := d.securities[tickerSymbol]; exists {
		return securityData.GetSupplementaryDatum(key)
	}
	return nil
}

// SetSecuritySupplementalDatum sets a security supplemental datum
func (d *dataCache) SetSecuritySupplementalDatum(tickerSymbol, key string, datum *float64, update SupplementalDatumUpdate) bool {
	if tickerSymbol == "" {
		return false
	}
	
	d.securitiesMutex.Lock()
	securityData, exists := d.securities[tickerSymbol]
	if !exists {
		securityData = NewSecurityData(tickerSymbol)
		d.securities[tickerSymbol] = securityData
	}
	d.securitiesMutex.Unlock()
	
	return securityData.SetSupplementaryDatumWithCallback(key, datum, d.securitySupplementalDatumUpdatedCallback, d, update)
}

// GetOptionsContractSupplementalDatum returns an options contract supplemental datum
func (d *dataCache) GetOptionsContractSupplementalDatum(tickerSymbol, contract, key string) *float64 {
	d.securitiesMutex.RLock()
	defer d.securitiesMutex.RUnlock()
	
	if securityData, exists := d.securities[tickerSymbol]; exists {
		return securityData.GetOptionsContractSupplementalDatum(contract, key)
	}
	return nil
}

// SetOptionSupplementalDatum sets an options contract supplemental datum
func (d *dataCache) SetOptionSupplementalDatum(tickerSymbol, contract, key string, datum *float64, update SupplementalDatumUpdate) bool {
	if tickerSymbol == "" || contract == "" {
		return false
	}
	
	d.securitiesMutex.Lock()
	securityData, exists := d.securities[tickerSymbol]
	if !exists {
		securityData = NewSecurityData(tickerSymbol)
		d.securities[tickerSymbol] = securityData
	}
	d.securitiesMutex.Unlock()
	
	return securityData.SetOptionsContractSupplementalDatumWithCallback(contract, key, datum, d.optionsContractSupplementalDatumUpdatedCallback, d, update)
}

// GetSecurityData returns security data for a ticker symbol
func (d *dataCache) GetSecurityData(tickerSymbol string) SecurityData {
	d.securitiesMutex.RLock()
	defer d.securitiesMutex.RUnlock()
	
	if securityData, exists := d.securities[tickerSymbol]; exists {
		return securityData
	}
	return nil
}

// GetAllSecurityData returns all security data
func (d *dataCache) GetAllSecurityData() map[string]SecurityData {
	d.securitiesMutex.RLock()
	defer d.securitiesMutex.RUnlock()
	
	result := make(map[string]SecurityData)
	for k, v := range d.securities {
		result[k] = v
	}
	return result
}

// GetOptionsContractData returns options contract data
func (d *dataCache) GetOptionsContractData(tickerSymbol, contract string) OptionsContractData {
	if securityData := d.GetSecurityData(tickerSymbol); securityData != nil {
		return securityData.GetOptionsContractData(contract)
	}
	return nil
}

// GetAllOptionsContractData returns all options contract data for a ticker symbol
func (d *dataCache) GetAllOptionsContractData(tickerSymbol string) map[string]OptionsContractData {
	if securityData := d.GetSecurityData(tickerSymbol); securityData != nil {
		return securityData.GetAllOptionsContractData()
	}
	return make(map[string]OptionsContractData)
}

// GetLatestEquityTrade returns the latest equity trade
func (d *dataCache) GetLatestEquityTrade(tickerSymbol string) *intrinio.EquityTrade {
	if securityData := d.GetSecurityData(tickerSymbol); securityData != nil {
		return securityData.GetLatestEquitiesTrade()
	}
	return nil
}

// SetEquityTrade sets the latest equity trade
func (d *dataCache) SetEquityTrade(trade *intrinio.EquityTrade) bool {
	if trade == nil {
		return false
	}
	
	d.securitiesMutex.Lock()
	securityData, exists := d.securities[trade.Symbol]
	if !exists {
		securityData = NewSecurityData(trade.Symbol)
		d.securities[trade.Symbol] = securityData
	}
	d.securitiesMutex.Unlock()
	
	return securityData.SetEquitiesTradeWithCallback(trade, d.equitiesTradeUpdatedCallback, d)
}

// GetLatestEquityAskQuote returns the latest equity ask quote
func (d *dataCache) GetLatestEquityAskQuote(tickerSymbol string) *intrinio.EquityQuote {
	if securityData := d.GetSecurityData(tickerSymbol); securityData != nil {
		return securityData.GetLatestEquitiesAskQuote()
	}
	return nil
}

// GetLatestEquityBidQuote returns the latest equity bid quote
func (d *dataCache) GetLatestEquityBidQuote(tickerSymbol string) *intrinio.EquityQuote {
	if securityData := d.GetSecurityData(tickerSymbol); securityData != nil {
		return securityData.GetLatestEquitiesBidQuote()
	}
	return nil
}

// SetEquityQuote sets the latest equity quote
func (d *dataCache) SetEquityQuote(quote *intrinio.EquityQuote) bool {
	if quote == nil {
		return false
	}
	
	d.securitiesMutex.Lock()
	securityData, exists := d.securities[quote.Symbol]
	if !exists {
		securityData = NewSecurityData(quote.Symbol)
		d.securities[quote.Symbol] = securityData
	}
	d.securitiesMutex.Unlock()
	
	return securityData.SetEquitiesQuoteWithCallback(quote, d.equitiesQuoteUpdatedCallback, d)
}

// GetLatestEquityTradeCandleStick returns the latest equity trade candlestick
func (d *dataCache) GetLatestEquityTradeCandleStick(tickerSymbol string) *TradeCandleStick {
	if securityData := d.GetSecurityData(tickerSymbol); securityData != nil {
		return securityData.GetLatestEquitiesTradeCandleStick()
	}
	return nil
}

// SetEquityTradeCandleStick sets the latest equity trade candlestick
func (d *dataCache) SetEquityTradeCandleStick(tradeCandleStick *TradeCandleStick) bool {
	if tradeCandleStick == nil {
		return false
	}
	
	d.securitiesMutex.Lock()
	securityData, exists := d.securities[tradeCandleStick.Symbol]
	if !exists {
		securityData = NewSecurityData(tradeCandleStick.Symbol)
		d.securities[tradeCandleStick.Symbol] = securityData
	}
	d.securitiesMutex.Unlock()
	
	return securityData.SetEquitiesTradeCandleStickWithCallback(tradeCandleStick, d.equitiesTradeCandleStickUpdatedCallback, d)
}

// GetLatestEquityAskQuoteCandleStick returns the latest equity ask quote candlestick
func (d *dataCache) GetLatestEquityAskQuoteCandleStick(tickerSymbol string) *QuoteCandleStick {
	if securityData := d.GetSecurityData(tickerSymbol); securityData != nil {
		return securityData.GetLatestEquitiesAskQuoteCandleStick()
	}
	return nil
}

// GetLatestEquityBidQuoteCandleStick returns the latest equity bid quote candlestick
func (d *dataCache) GetLatestEquityBidQuoteCandleStick(tickerSymbol string) *QuoteCandleStick {
	if securityData := d.GetSecurityData(tickerSymbol); securityData != nil {
		return securityData.GetLatestEquitiesBidQuoteCandleStick()
	}
	return nil
}

// SetEquityQuoteCandleStick sets the latest equity quote candlestick
func (d *dataCache) SetEquityQuoteCandleStick(quoteCandleStick *QuoteCandleStick) bool {
	if quoteCandleStick == nil {
		return false
	}
	
	d.securitiesMutex.Lock()
	securityData, exists := d.securities[quoteCandleStick.Symbol]
	if !exists {
		securityData = NewSecurityData(quoteCandleStick.Symbol)
		d.securities[quoteCandleStick.Symbol] = securityData
	}
	d.securitiesMutex.Unlock()
	
	return securityData.SetEquitiesQuoteCandleStickWithCallback(quoteCandleStick, d.equitiesQuoteCandleStickUpdatedCallback, d)
}

// GetLatestOptionsTrade returns the latest options trade
func (d *dataCache) GetLatestOptionsTrade(tickerSymbol, contract string) *intrinio.OptionTrade {
	if securityData := d.GetSecurityData(tickerSymbol); securityData != nil {
		return securityData.GetOptionsContractTrade(contract)
	}
	return nil
}

// SetOptionsTrade sets the latest options trade
func (d *dataCache) SetOptionsTrade(trade *intrinio.OptionTrade) bool {
	if trade == nil {
		return false
	}
	
	// Extract ticker symbol from contract (assuming format like AAPL__201016C00100000)
	tickerSymbol := extractTickerFromContract(trade.ContractId)
	if tickerSymbol == "" {
		return false
	}
	
	d.securitiesMutex.Lock()
	securityData, exists := d.securities[tickerSymbol]
	if !exists {
		securityData = NewSecurityData(tickerSymbol)
		d.securities[tickerSymbol] = securityData
	}
	d.securitiesMutex.Unlock()
	
	return securityData.SetOptionsContractTradeWithCallback(trade, d.optionsTradeUpdatedCallback, d)
}

// GetLatestOptionsQuote returns the latest options quote
func (d *dataCache) GetLatestOptionsQuote(tickerSymbol, contract string) *intrinio.OptionQuote {
	if securityData := d.GetSecurityData(tickerSymbol); securityData != nil {
		return securityData.GetOptionsContractQuote(contract)
	}
	return nil
}

// SetOptionsQuote sets the latest options quote
func (d *dataCache) SetOptionsQuote(quote *intrinio.OptionQuote) bool {
	if quote == nil {
		return false
	}
	
	// Extract ticker symbol from contract
	tickerSymbol := extractTickerFromContract(quote.ContractId)
	if tickerSymbol == "" {
		return false
	}
	
	d.securitiesMutex.Lock()
	securityData, exists := d.securities[tickerSymbol]
	if !exists {
		securityData = NewSecurityData(tickerSymbol)
		d.securities[tickerSymbol] = securityData
	}
	d.securitiesMutex.Unlock()
	
	return securityData.SetOptionsContractQuoteWithCallback(quote, d.optionsQuoteUpdatedCallback, d)
}

// GetLatestOptionsRefresh returns the latest options refresh
func (d *dataCache) GetLatestOptionsRefresh(tickerSymbol, contract string) *intrinio.OptionRefresh {
	if securityData := d.GetSecurityData(tickerSymbol); securityData != nil {
		return securityData.GetOptionsContractRefresh(contract)
	}
	return nil
}

// SetOptionsRefresh sets the latest options refresh
func (d *dataCache) SetOptionsRefresh(refresh *intrinio.OptionRefresh) bool {
	if refresh == nil {
		return false
	}
	
	// Extract ticker symbol from contract
	tickerSymbol := extractTickerFromContract(refresh.ContractId)
	if tickerSymbol == "" {
		return false
	}
	
	d.securitiesMutex.Lock()
	securityData, exists := d.securities[tickerSymbol]
	if !exists {
		securityData = NewSecurityData(tickerSymbol)
		d.securities[tickerSymbol] = securityData
	}
	d.securitiesMutex.Unlock()
	
	return securityData.SetOptionsContractRefreshWithCallback(refresh, d.optionsRefreshUpdatedCallback, d)
}

// GetLatestOptionsUnusualActivity returns the latest options unusual activity
func (d *dataCache) GetLatestOptionsUnusualActivity(tickerSymbol, contract string) *OptionsUnusualActivity {
	if securityData := d.GetSecurityData(tickerSymbol); securityData != nil {
		return securityData.GetOptionsContractUnusualActivity(contract)
	}
	return nil
}

// SetOptionsUnusualActivity sets the latest options unusual activity
func (d *dataCache) SetOptionsUnusualActivity(unusualActivity *OptionsUnusualActivity) bool {
	if unusualActivity == nil {
		return false
	}
	
	// Extract ticker symbol from contract
	tickerSymbol := extractTickerFromContract(unusualActivity.Contract)
	if tickerSymbol == "" {
		return false
	}
	
	d.securitiesMutex.Lock()
	securityData, exists := d.securities[tickerSymbol]
	if !exists {
		securityData = NewSecurityData(tickerSymbol)
		d.securities[tickerSymbol] = securityData
	}
	d.securitiesMutex.Unlock()
	
	return securityData.SetOptionsContractUnusualActivityWithCallback(unusualActivity, d.optionsUnusualActivityUpdatedCallback, d)
}

// GetLatestOptionsTradeCandleStick returns the latest options trade candlestick
func (d *dataCache) GetLatestOptionsTradeCandleStick(tickerSymbol, contract string) *OptionsTradeCandleStick {
	if securityData := d.GetSecurityData(tickerSymbol); securityData != nil {
		return securityData.GetOptionsContractTradeCandleStick(contract)
	}
	return nil
}

// SetOptionsTradeCandleStick sets the latest options trade candlestick
func (d *dataCache) SetOptionsTradeCandleStick(tradeCandleStick *OptionsTradeCandleStick) bool {
	if tradeCandleStick == nil {
		return false
	}
	
	// Extract ticker symbol from contract
	tickerSymbol := extractTickerFromContract(tradeCandleStick.Contract)
	if tickerSymbol == "" {
		return false
	}
	
	d.securitiesMutex.Lock()
	securityData, exists := d.securities[tickerSymbol]
	if !exists {
		securityData = NewSecurityData(tickerSymbol)
		d.securities[tickerSymbol] = securityData
	}
	d.securitiesMutex.Unlock()
	
	return securityData.SetOptionsContractTradeCandleStickWithCallback(tradeCandleStick, d.optionsTradeCandleStickUpdatedCallback, d)
}

// GetOptionsAskQuoteCandleStick returns the latest options ask quote candlestick
func (d *dataCache) GetOptionsAskQuoteCandleStick(tickerSymbol, contract string) *OptionsQuoteCandleStick {
	if securityData := d.GetSecurityData(tickerSymbol); securityData != nil {
		return securityData.GetOptionsContractAskQuoteCandleStick(contract)
	}
	return nil
}

// GetOptionsBidQuoteCandleStick returns the latest options bid quote candlestick
func (d *dataCache) GetOptionsBidQuoteCandleStick(tickerSymbol, contract string) *OptionsQuoteCandleStick {
	if securityData := d.GetSecurityData(tickerSymbol); securityData != nil {
		return securityData.GetOptionsContractBidQuoteCandleStick(contract)
	}
	return nil
}

// SetOptionsQuoteCandleStick sets the latest options quote candlestick
func (d *dataCache) SetOptionsQuoteCandleStick(quoteCandleStick *OptionsQuoteCandleStick) bool {
	if quoteCandleStick == nil {
		return false
	}
	
	// Extract ticker symbol from contract
	tickerSymbol := extractTickerFromContract(quoteCandleStick.Contract)
	if tickerSymbol == "" {
		return false
	}
	
	d.securitiesMutex.Lock()
	securityData, exists := d.securities[tickerSymbol]
	if !exists {
		securityData = NewSecurityData(tickerSymbol)
		d.securities[tickerSymbol] = securityData
	}
	d.securitiesMutex.Unlock()
	
	return securityData.SetOptionsContractQuoteCandleStickWithCallback(quoteCandleStick, d.optionsQuoteCandleStickUpdatedCallback, d)
}

// Callback setters
func (d *dataCache) SetSupplementalDatumUpdatedCallback(callback OnSupplementalDatumUpdated) {
	d.supplementalDatumUpdatedCallback = callback
}

func (d *dataCache) SetSecuritySupplementalDatumUpdatedCallback(callback OnSecuritySupplementalDatumUpdated) {
	d.securitySupplementalDatumUpdatedCallback = callback
}

func (d *dataCache) SetOptionsContractSupplementalDatumUpdatedCallback(callback OnOptionsContractSupplementalDatumUpdated) {
	d.optionsContractSupplementalDatumUpdatedCallback = callback
}

func (d *dataCache) SetEquitiesTradeUpdatedCallback(callback OnEquitiesTradeUpdated) {
	d.equitiesTradeUpdatedCallback = callback
}

func (d *dataCache) SetEquitiesQuoteUpdatedCallback(callback OnEquitiesQuoteUpdated) {
	d.equitiesQuoteUpdatedCallback = callback
}

func (d *dataCache) SetEquitiesTradeCandleStickUpdatedCallback(callback OnEquitiesTradeCandleStickUpdated) {
	d.equitiesTradeCandleStickUpdatedCallback = callback
}

func (d *dataCache) SetEquitiesQuoteCandleStickUpdatedCallback(callback OnEquitiesQuoteCandleStickUpdated) {
	d.equitiesQuoteCandleStickUpdatedCallback = callback
}

func (d *dataCache) SetOptionsTradeUpdatedCallback(callback OnOptionsTradeUpdated) {
	d.optionsTradeUpdatedCallback = callback
}

func (d *dataCache) SetOptionsQuoteUpdatedCallback(callback OnOptionsQuoteUpdated) {
	d.optionsQuoteUpdatedCallback = callback
}

func (d *dataCache) SetOptionsRefreshUpdatedCallback(callback OnOptionsRefreshUpdated) {
	d.optionsRefreshUpdatedCallback = callback
}

func (d *dataCache) SetOptionsUnusualActivityUpdatedCallback(callback OnOptionsUnusualActivityUpdated) {
	d.optionsUnusualActivityUpdatedCallback = callback
}

func (d *dataCache) SetOptionsTradeCandleStickUpdatedCallback(callback OnOptionsTradeCandleStickUpdated) {
	d.optionsTradeCandleStickUpdatedCallback = callback
}

func (d *dataCache) SetOptionsQuoteCandleStickUpdatedCallback(callback OnOptionsQuoteCandleStickUpdated) {
	d.optionsQuoteCandleStickUpdatedCallback = callback
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