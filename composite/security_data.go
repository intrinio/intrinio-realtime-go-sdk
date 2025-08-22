package composite

import (
	"github.com/intrinio/intrinio-realtime-go-sdk"
	"sync"
)

// securityData implements the SecurityData interface
type securityData struct {
	tickerSymbol              string
	latestTrade               *intrinio.EquityTrade
	latestAskQuote            *intrinio.EquityQuote
	latestBidQuote            *intrinio.EquityQuote
	latestTradeCandleStick    *TradeCandleStick
	latestAskQuoteCandleStick *QuoteCandleStick
	latestBidQuoteCandleStick *QuoteCandleStick
	contracts                 map[string]OptionsContractData
	contractsMutex            sync.RWMutex
	supplementaryData         map[string]*float64
	supplementaryDataMutex    sync.RWMutex
}

// NewSecurityData creates a new SecurityData instance
func NewSecurityData(tickerSymbol string) SecurityData {
	return &securityData{
		tickerSymbol:      tickerSymbol,
		contracts:         make(map[string]OptionsContractData),
		supplementaryData: make(map[string]*float64),
	}
}

// GetTickerSymbol returns the ticker symbol
func (s *securityData) GetTickerSymbol() string {
	return s.tickerSymbol
}

// GetLatestEquitiesTrade returns the latest equities trade
func (s *securityData) GetLatestEquitiesTrade() *intrinio.EquityTrade {
	return s.latestTrade
}

// GetLatestEquitiesAskQuote returns the latest equities ask quote
func (s *securityData) GetLatestEquitiesAskQuote() *intrinio.EquityQuote {
	return s.latestAskQuote
}

// GetLatestEquitiesBidQuote returns the latest equities bid quote
func (s *securityData) GetLatestEquitiesBidQuote() *intrinio.EquityQuote {
	return s.latestBidQuote
}

// GetLatestEquitiesTradeCandleStick returns the latest equities trade candlestick
func (s *securityData) GetLatestEquitiesTradeCandleStick() *TradeCandleStick {
	return s.latestTradeCandleStick
}

// GetLatestEquitiesAskQuoteCandleStick returns the latest equities ask quote candlestick
func (s *securityData) GetLatestEquitiesAskQuoteCandleStick() *QuoteCandleStick {
	return s.latestAskQuoteCandleStick
}

// GetLatestEquitiesBidQuoteCandleStick returns the latest equities bid quote candlestick
func (s *securityData) GetLatestEquitiesBidQuoteCandleStick() *QuoteCandleStick {
	return s.latestBidQuoteCandleStick
}

// GetSupplementaryDatum returns a supplementary datum
func (s *securityData) GetSupplementaryDatum(key string) *float64 {
	s.supplementaryDataMutex.RLock()
	defer s.supplementaryDataMutex.RUnlock()

	if value, exists := s.supplementaryData[key]; exists {
		return value
	}
	return nil
}

// SetSupplementaryDatum sets a supplementary datum
func (s *securityData) SetSupplementaryDatum(key string, datum *float64, update SupplementalDatumUpdate) bool {
	s.supplementaryDataMutex.Lock()
	defer s.supplementaryDataMutex.Unlock()

	oldValue := s.supplementaryData[key]
	newValue := update(key, oldValue, datum)

	if newValue != oldValue {
		s.supplementaryData[key] = newValue
		return true
	}
	return false
}

// SetSupplementaryDatumWithCallback sets a supplementary datum with callback
func (s *securityData) SetSupplementaryDatumWithCallback(key string, datum *float64, callback OnSecuritySupplementalDatumUpdated, dataCache DataCache, update SupplementalDatumUpdate) bool {
	result := s.SetSupplementaryDatum(key, datum, update)
	if result && callback != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Log error here if logging is available
				}
			}()
			callback(key, datum, s, dataCache)
		}()
	}
	return result
}

// GetAllSupplementaryData returns all supplementary data
func (s *securityData) GetAllSupplementaryData() map[string]*float64 {
	s.supplementaryDataMutex.RLock()
	defer s.supplementaryDataMutex.RUnlock()

	result := make(map[string]*float64)
	for k, v := range s.supplementaryData {
		result[k] = v
	}
	return result
}

// SetEquitiesTrade sets the latest equities trade
func (s *securityData) SetEquitiesTrade(trade *intrinio.EquityTrade) bool {
	if s.latestTrade == nil || (trade != nil && trade.Timestamp > s.latestTrade.Timestamp) {
		s.latestTrade = trade
		return true
	}
	return false
}

// SetEquitiesTradeWithCallback sets the latest equities trade with callback
func (s *securityData) SetEquitiesTradeWithCallback(trade *intrinio.EquityTrade, callback OnEquitiesTradeUpdated, dataCache DataCache) bool {
	result := s.SetEquitiesTrade(trade)
	if result && callback != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Log error here if logging is available
				}
			}()
			callback(s, dataCache, trade)
		}()
	}
	return result
}

// SetEquitiesQuote sets the latest equities quote
func (s *securityData) SetEquitiesQuote(quote *intrinio.EquityQuote) bool {
	if quote == nil {
		return false
	}

	if quote.Type == intrinio.ASK {
		if s.latestAskQuote == nil || (quote.Timestamp > s.latestAskQuote.Timestamp) {
			s.latestAskQuote = quote
			return true
		}
	} else if quote.Type == intrinio.BID {
		if s.latestBidQuote == nil || (quote.Timestamp > s.latestBidQuote.Timestamp) {
			s.latestBidQuote = quote
			return true
		}
	}
	return false
}

// SetEquitiesQuoteWithCallback sets the latest equities quote with callback
func (s *securityData) SetEquitiesQuoteWithCallback(quote *intrinio.EquityQuote, callback OnEquitiesQuoteUpdated, dataCache DataCache) bool {
	result := s.SetEquitiesQuote(quote)
	if result && callback != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Log error here if logging is available
				}
			}()
			callback(s, dataCache, quote)
		}()
	}
	return result
}

// SetEquitiesTradeCandleStick sets the latest equities trade candlestick
func (s *securityData) SetEquitiesTradeCandleStick(tradeCandleStick *TradeCandleStick) bool {
	if s.latestTradeCandleStick == nil || (tradeCandleStick != nil && tradeCandleStick.Timestamp.After(s.latestTradeCandleStick.Timestamp)) {
		s.latestTradeCandleStick = tradeCandleStick
		return true
	}
	return false
}

// SetEquitiesTradeCandleStickWithCallback sets the latest equities trade candlestick with callback
func (s *securityData) SetEquitiesTradeCandleStickWithCallback(tradeCandleStick *TradeCandleStick, callback OnEquitiesTradeCandleStickUpdated, dataCache DataCache) bool {
	result := s.SetEquitiesTradeCandleStick(tradeCandleStick)
	if result && callback != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Log error here if logging is available
				}
			}()
			callback(s, dataCache, tradeCandleStick)
		}()
	}
	return result
}

// SetEquitiesQuoteCandleStick sets the latest equities quote candlestick
func (s *securityData) SetEquitiesQuoteCandleStick(quoteCandleStick *QuoteCandleStick) bool {
	if quoteCandleStick == nil {
		return false
	}

	if quoteCandleStick.Type == QuoteTypeAsk {
		if s.latestAskQuoteCandleStick == nil || quoteCandleStick.Timestamp.After(s.latestAskQuoteCandleStick.Timestamp) {
			s.latestAskQuoteCandleStick = quoteCandleStick
			return true
		}
	} else if quoteCandleStick.Type == QuoteTypeBid {
		if s.latestBidQuoteCandleStick == nil || quoteCandleStick.Timestamp.After(s.latestBidQuoteCandleStick.Timestamp) {
			s.latestBidQuoteCandleStick = quoteCandleStick
			return true
		}
	}
	return false
}

// SetEquitiesQuoteCandleStickWithCallback sets the latest equities quote candlestick with callback
func (s *securityData) SetEquitiesQuoteCandleStickWithCallback(quoteCandleStick *QuoteCandleStick, callback OnEquitiesQuoteCandleStickUpdated, dataCache DataCache) bool {
	result := s.SetEquitiesQuoteCandleStick(quoteCandleStick)
	if result && callback != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Log error here if logging is available
				}
			}()
			callback(s, dataCache, quoteCandleStick)
		}()
	}
	return result
}

// GetOptionsContractData returns the options contract data for a contract
func (s *securityData) GetOptionsContractData(contract string) OptionsContractData {
	s.contractsMutex.RLock()
	defer s.contractsMutex.RUnlock()

	if contractData, exists := s.contracts[contract]; exists {
		return contractData
	}
	return nil
}

// GetAllOptionsContractData returns all options contract data
func (s *securityData) GetAllOptionsContractData() map[string]OptionsContractData {
	s.contractsMutex.RLock()
	defer s.contractsMutex.RUnlock()

	result := make(map[string]OptionsContractData)
	for k, v := range s.contracts {
		result[k] = v
	}
	return result
}

// GetContractNames returns all contract names
func (s *securityData) GetContractNames() []string {
	s.contractsMutex.RLock()
	defer s.contractsMutex.RUnlock()

	names := make([]string, 0, len(s.contracts))
	for contract := range s.contracts {
		names = append(names, contract)
	}
	return names
}

// GetOptionsContractTrade returns the latest options trade for a contract
func (s *securityData) GetOptionsContractTrade(contract string) *intrinio.OptionTrade {
	if contractData := s.GetOptionsContractData(contract); contractData != nil {
		return contractData.GetLatestTrade()
	}
	return nil
}

// SetOptionsContractTrade sets the latest options trade for a contract
func (s *securityData) SetOptionsContractTrade(trade *intrinio.OptionTrade) bool {
	if trade == nil {
		return false
	}

	contractData := s.GetOptionsContractData(trade.ContractId)
	if contractData == nil {
		contractData = NewOptionsContractData(trade.ContractId)
		s.contractsMutex.Lock()
		s.contracts[trade.ContractId] = contractData
		s.contractsMutex.Unlock()
	}

	return contractData.SetTrade(trade)
}

// SetOptionsContractTradeWithCallback sets the latest options trade for a contract with callback
func (s *securityData) SetOptionsContractTradeWithCallback(trade *intrinio.OptionTrade, callback OnOptionsTradeUpdated, dataCache DataCache) bool {
	if trade == nil {
		return false
	}

	contractData := s.GetOptionsContractData(trade.ContractId)
	if contractData == nil {
		contractData = NewOptionsContractData(trade.ContractId)
		s.contractsMutex.Lock()
		s.contracts[trade.ContractId] = contractData
		s.contractsMutex.Unlock()
	}

	result := contractData.SetTradeWithCallback(trade, callback, s, dataCache)
	if result && callback != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Log error here if logging is available
				}
			}()
			callback(contractData, dataCache, s, trade)
		}()
	}
	return result
}

// GetOptionsContractQuote returns the latest options quote for a contract
func (s *securityData) GetOptionsContractQuote(contract string) *intrinio.OptionQuote {
	if contractData := s.GetOptionsContractData(contract); contractData != nil {
		return contractData.GetLatestQuote()
	}
	return nil
}

// SetOptionsContractQuote sets the latest options quote for a contract
func (s *securityData) SetOptionsContractQuote(quote *intrinio.OptionQuote) bool {
	if quote == nil {
		return false
	}

	contractData := s.GetOptionsContractData(quote.ContractId)
	if contractData == nil {
		contractData = NewOptionsContractData(quote.ContractId)
		s.contractsMutex.Lock()
		s.contracts[quote.ContractId] = contractData
		s.contractsMutex.Unlock()
	}

	return contractData.SetQuote(quote)
}

// SetOptionsContractQuoteWithCallback sets the latest options quote for a contract with callback
func (s *securityData) SetOptionsContractQuoteWithCallback(quote *intrinio.OptionQuote, callback OnOptionsQuoteUpdated, dataCache DataCache) bool {
	if quote == nil {
		return false
	}

	contractData := s.GetOptionsContractData(quote.ContractId)
	if contractData == nil {
		contractData = NewOptionsContractData(quote.ContractId)
		s.contractsMutex.Lock()
		s.contracts[quote.ContractId] = contractData
		s.contractsMutex.Unlock()
	}

	result := contractData.SetQuoteWithCallback(quote, callback, s, dataCache)
	if result && callback != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Log error here if logging is available
				}
			}()
			callback(contractData, dataCache, s, quote)
		}()
	}
	return result
}

// GetOptionsContractRefresh returns the latest options refresh for a contract
func (s *securityData) GetOptionsContractRefresh(contract string) *intrinio.OptionRefresh {
	if contractData := s.GetOptionsContractData(contract); contractData != nil {
		return contractData.GetLatestRefresh()
	}
	return nil
}

// SetOptionsContractRefresh sets the latest options refresh for a contract
func (s *securityData) SetOptionsContractRefresh(refresh *intrinio.OptionRefresh) bool {
	if refresh == nil {
		return false
	}

	contractData := s.GetOptionsContractData(refresh.ContractId)
	if contractData == nil {
		contractData = NewOptionsContractData(refresh.ContractId)
		s.contractsMutex.Lock()
		s.contracts[refresh.ContractId] = contractData
		s.contractsMutex.Unlock()
	}

	return contractData.SetRefresh(refresh)
}

// SetOptionsContractRefreshWithCallback sets the latest options refresh for a contract with callback
func (s *securityData) SetOptionsContractRefreshWithCallback(refresh *intrinio.OptionRefresh, callback OnOptionsRefreshUpdated, dataCache DataCache) bool {
	if refresh == nil {
		return false
	}

	contractData := s.GetOptionsContractData(refresh.ContractId)
	if contractData == nil {
		contractData = NewOptionsContractData(refresh.ContractId)
		s.contractsMutex.Lock()
		s.contracts[refresh.ContractId] = contractData
		s.contractsMutex.Unlock()
	}

	result := contractData.SetRefreshWithCallback(refresh, callback, s, dataCache)
	if result && callback != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Log error here if logging is available
				}
			}()
			callback(contractData, dataCache, s, refresh)
		}()
	}
	return result
}

// GetOptionsContractUnusualActivity returns the latest options unusual activity for a contract
func (s *securityData) GetOptionsContractUnusualActivity(contract string) *OptionsUnusualActivity {
	if contractData := s.GetOptionsContractData(contract); contractData != nil {
		return contractData.GetLatestUnusualActivity()
	}
	return nil
}

// SetOptionsContractUnusualActivity sets the latest options unusual activity for a contract
func (s *securityData) SetOptionsContractUnusualActivity(unusualActivity *OptionsUnusualActivity) bool {
	if unusualActivity == nil {
		return false
	}

	contractData := s.GetOptionsContractData(unusualActivity.Contract)
	if contractData == nil {
		contractData = NewOptionsContractData(unusualActivity.Contract)
		s.contractsMutex.Lock()
		s.contracts[unusualActivity.Contract] = contractData
		s.contractsMutex.Unlock()
	}

	return contractData.SetUnusualActivity(unusualActivity)
}

// SetOptionsContractUnusualActivityWithCallback sets the latest options unusual activity for a contract with callback
func (s *securityData) SetOptionsContractUnusualActivityWithCallback(unusualActivity *OptionsUnusualActivity, callback OnOptionsUnusualActivityUpdated, dataCache DataCache) bool {
	if unusualActivity == nil {
		return false
	}

	contractData := s.GetOptionsContractData(unusualActivity.Contract)
	if contractData == nil {
		contractData = NewOptionsContractData(unusualActivity.Contract)
		s.contractsMutex.Lock()
		s.contracts[unusualActivity.Contract] = contractData
		s.contractsMutex.Unlock()
	}

	result := contractData.SetUnusualActivityWithCallback(unusualActivity, callback, s, dataCache)
	if result && callback != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Log error here if logging is available
				}
			}()
			callback(contractData, dataCache, s, unusualActivity)
		}()
	}
	return result
}

// GetOptionsContractTradeCandleStick returns the latest options trade candlestick for a contract
func (s *securityData) GetOptionsContractTradeCandleStick(contract string) *OptionsTradeCandleStick {
	if contractData := s.GetOptionsContractData(contract); contractData != nil {
		return contractData.GetLatestTradeCandleStick()
	}
	return nil
}

// SetOptionsContractTradeCandleStick sets the latest options trade candlestick for a contract
func (s *securityData) SetOptionsContractTradeCandleStick(tradeCandleStick *OptionsTradeCandleStick) bool {
	if tradeCandleStick == nil {
		return false
	}

	contractData := s.GetOptionsContractData(tradeCandleStick.Contract)
	if contractData == nil {
		contractData = NewOptionsContractData(tradeCandleStick.Contract)
		s.contractsMutex.Lock()
		s.contracts[tradeCandleStick.Contract] = contractData
		s.contractsMutex.Unlock()
	}

	return contractData.SetTradeCandleStick(tradeCandleStick)
}

// SetOptionsContractTradeCandleStickWithCallback sets the latest options trade candlestick for a contract with callback
func (s *securityData) SetOptionsContractTradeCandleStickWithCallback(tradeCandleStick *OptionsTradeCandleStick, callback OnOptionsTradeCandleStickUpdated, dataCache DataCache) bool {
	if tradeCandleStick == nil {
		return false
	}

	contractData := s.GetOptionsContractData(tradeCandleStick.Contract)
	if contractData == nil {
		contractData = NewOptionsContractData(tradeCandleStick.Contract)
		s.contractsMutex.Lock()
		s.contracts[tradeCandleStick.Contract] = contractData
		s.contractsMutex.Unlock()
	}

	result := contractData.SetTradeCandleStickWithCallback(tradeCandleStick, callback, s, dataCache)
	if result && callback != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Log error here if logging is available
				}
			}()
			callback(contractData, dataCache, s, tradeCandleStick)
		}()
	}
	return result
}

// GetOptionsContractBidQuoteCandleStick returns the latest options bid quote candlestick for a contract
func (s *securityData) GetOptionsContractBidQuoteCandleStick(contract string) *OptionsQuoteCandleStick {
	if contractData := s.GetOptionsContractData(contract); contractData != nil {
		return contractData.GetLatestBidQuoteCandleStick()
	}
	return nil
}

// GetOptionsContractAskQuoteCandleStick returns the latest options ask quote candlestick for a contract
func (s *securityData) GetOptionsContractAskQuoteCandleStick(contract string) *OptionsQuoteCandleStick {
	if contractData := s.GetOptionsContractData(contract); contractData != nil {
		return contractData.GetLatestAskQuoteCandleStick()
	}
	return nil
}

// SetOptionsContractQuoteCandleStick sets the latest options quote candlestick for a contract
func (s *securityData) SetOptionsContractQuoteCandleStick(quoteCandleStick *OptionsQuoteCandleStick) bool {
	if quoteCandleStick == nil {
		return false
	}

	contractData := s.GetOptionsContractData(quoteCandleStick.Contract)
	if contractData == nil {
		contractData = NewOptionsContractData(quoteCandleStick.Contract)
		s.contractsMutex.Lock()
		s.contracts[quoteCandleStick.Contract] = contractData
		s.contractsMutex.Unlock()
	}

	return contractData.SetQuoteCandleStick(quoteCandleStick)
}

// SetOptionsContractQuoteCandleStickWithCallback sets the latest options quote candlestick for a contract with callback
func (s *securityData) SetOptionsContractQuoteCandleStickWithCallback(quoteCandleStick *OptionsQuoteCandleStick, callback OnOptionsQuoteCandleStickUpdated, dataCache DataCache) bool {
	if quoteCandleStick == nil {
		return false
	}

	contractData := s.GetOptionsContractData(quoteCandleStick.Contract)
	if contractData == nil {
		contractData = NewOptionsContractData(quoteCandleStick.Contract)
		s.contractsMutex.Lock()
		s.contracts[quoteCandleStick.Contract] = contractData
		s.contractsMutex.Unlock()
	}

	result := contractData.SetQuoteCandleStickWithCallback(quoteCandleStick, callback, s, dataCache)
	if result && callback != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Log error here if logging is available
				}
			}()
			callback(contractData, dataCache, s, quoteCandleStick)
		}()
	}
	return result
}

// GetOptionsContractSupplementalDatum returns supplemental datum for an options contract
func (s *securityData) GetOptionsContractSupplementalDatum(contract, key string) *float64 {
	if contractData := s.GetOptionsContractData(contract); contractData != nil {
		return contractData.GetSupplementaryDatum(key)
	}
	return nil
}

// SetOptionsContractSupplementalDatum sets supplemental datum for an options contract
func (s *securityData) SetOptionsContractSupplementalDatum(contract, key string, datum *float64, update SupplementalDatumUpdate) bool {
	if contractData := s.GetOptionsContractData(contract); contractData != nil {
		return contractData.SetSupplementaryDatum(key, datum, update)
	}
	return false
}

// SetOptionsContractSupplementalDatumWithCallback sets supplemental datum for an options contract with callback
func (s *securityData) SetOptionsContractSupplementalDatumWithCallback(contract, key string, datum *float64, callback OnOptionsContractSupplementalDatumUpdated, dataCache DataCache, update SupplementalDatumUpdate) bool {
	if contractData := s.GetOptionsContractData(contract); contractData != nil {
		return contractData.SetSupplementaryDatumWithCallback(key, datum, callback, s, dataCache, update)
	}
	return false
}

// GetOptionsContractGreekData returns greek datum for an options contract
func (s *securityData) GetOptionsContractGreekData(contract, key string) *Greek {
	if contractData := s.GetOptionsContractData(contract); contractData != nil {
		return contractData.GetGreekData(key)
	}
	return nil
}

// SetOptionsContractGreekData sets greek datum for an options contract
func (s *securityData) SetOptionsContractGreekData(contract, key string, data *Greek, update GreekDataUpdate) bool {
	if contractData := s.GetOptionsContractData(contract); contractData != nil {
		return contractData.SetGreekData(key, data, update)
	}
	return false
}

// SetOptionsContractGreekDataWithCallback sets greek datum for an options contract with callback
func (s *securityData) SetOptionsContractGreekDataWithCallback(contract, key string, data *Greek, callback OnOptionsContractGreekDataUpdated, dataCache DataCache, update GreekDataUpdate) bool {
	if contractData := s.GetOptionsContractData(contract); contractData != nil {
		return contractData.SetGreekDataWithCallback(key, data, callback, s, dataCache, update)
	}
	return false
}
