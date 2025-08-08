package composite

import (
	"sync"
	"github.com/intrinio/intrinio-realtime-go-sdk"
)

// optionsContractData implements the OptionsContractData interface
type optionsContractData struct {
	contract                        string
	latestTrade                     *intrinio.OptionTrade
	latestQuote                     *intrinio.OptionQuote
	latestRefresh                   *intrinio.OptionRefresh
	latestUnusualActivity           *OptionsUnusualActivity
	latestTradeCandleStick          *OptionsTradeCandleStick
	latestAskQuoteCandleStick       *OptionsQuoteCandleStick
	latestBidQuoteCandleStick       *OptionsQuoteCandleStick
	supplementaryData               map[string]*float64
	supplementaryDataMutex          sync.RWMutex
}

// NewOptionsContractData creates a new OptionsContractData instance
func NewOptionsContractData(contract string) OptionsContractData {
	return &optionsContractData{
		contract:              contract,
		supplementaryData:     make(map[string]*float64),
	}
}

// GetContract returns the contract identifier
func (o *optionsContractData) GetContract() string {
	return o.contract
}

// GetLatestTrade returns the latest trade
func (o *optionsContractData) GetLatestTrade() *intrinio.OptionTrade {
	return o.latestTrade
}

// GetLatestQuote returns the latest quote
func (o *optionsContractData) GetLatestQuote() *intrinio.OptionQuote {
	return o.latestQuote
}

// GetLatestRefresh returns the latest refresh
func (o *optionsContractData) GetLatestRefresh() *intrinio.OptionRefresh {
	return o.latestRefresh
}

// GetLatestUnusualActivity returns the latest unusual activity
func (o *optionsContractData) GetLatestUnusualActivity() *OptionsUnusualActivity {
	return o.latestUnusualActivity
}

// GetLatestTradeCandleStick returns the latest trade candlestick
func (o *optionsContractData) GetLatestTradeCandleStick() *OptionsTradeCandleStick {
	return o.latestTradeCandleStick
}

// GetLatestAskQuoteCandleStick returns the latest ask quote candlestick
func (o *optionsContractData) GetLatestAskQuoteCandleStick() *OptionsQuoteCandleStick {
	return o.latestAskQuoteCandleStick
}

// GetLatestBidQuoteCandleStick returns the latest bid quote candlestick
func (o *optionsContractData) GetLatestBidQuoteCandleStick() *OptionsQuoteCandleStick {
	return o.latestBidQuoteCandleStick
}

// SetTrade sets the latest trade
func (o *optionsContractData) SetTrade(trade *intrinio.OptionTrade) bool {
	if o.latestTrade == nil || (trade != nil && trade.Timestamp > o.latestTrade.Timestamp) {
		o.latestTrade = trade
		return true
	}
	return false
}

// SetTradeWithCallback sets the latest trade with callback
func (o *optionsContractData) SetTradeWithCallback(trade *intrinio.OptionTrade, callback OnOptionsTradeUpdated, securityData SecurityData, dataCache DataCache) bool {
	result := o.SetTrade(trade)
	if result && callback != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Log error here if logging is available
				}
			}()
			callback(o, dataCache, securityData)
		}()
	}
	return result
}

// SetQuote sets the latest quote
func (o *optionsContractData) SetQuote(quote *intrinio.OptionQuote) bool {
	if o.latestQuote == nil || (quote != nil && quote.Timestamp > o.latestQuote.Timestamp) {
		o.latestQuote = quote
		return true
	}
	return false
}

// SetQuoteWithCallback sets the latest quote with callback
func (o *optionsContractData) SetQuoteWithCallback(quote *intrinio.OptionQuote, callback OnOptionsQuoteUpdated, securityData SecurityData, dataCache DataCache) bool {
	result := o.SetQuote(quote)
	if result && callback != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Log error here if logging is available
				}
			}()
			callback(o, dataCache, securityData)
		}()
	}
	return result
}

// SetRefresh sets the latest refresh
func (o *optionsContractData) SetRefresh(refresh *intrinio.OptionRefresh) bool {
	o.latestRefresh = refresh
	return true
}

// SetRefreshWithCallback sets the latest refresh with callback
func (o *optionsContractData) SetRefreshWithCallback(refresh *intrinio.OptionRefresh, callback OnOptionsRefreshUpdated, securityData SecurityData, dataCache DataCache) bool {
	result := o.SetRefresh(refresh)
	if result && callback != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Log error here if logging is available
				}
			}()
			callback(o, dataCache, securityData)
		}()
	}
	return result
}

// SetUnusualActivity sets the latest unusual activity
func (o *optionsContractData) SetUnusualActivity(unusualActivity *OptionsUnusualActivity) bool {
	if o.latestUnusualActivity == nil || (unusualActivity != nil && unusualActivity.Timestamp > o.latestUnusualActivity.Timestamp) {
		o.latestUnusualActivity = unusualActivity
		return true
	}
	return false
}

// SetUnusualActivityWithCallback sets the latest unusual activity with callback
func (o *optionsContractData) SetUnusualActivityWithCallback(unusualActivity *OptionsUnusualActivity, callback OnOptionsUnusualActivityUpdated, securityData SecurityData, dataCache DataCache) bool {
	result := o.SetUnusualActivity(unusualActivity)
	if result && callback != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Log error here if logging is available
				}
			}()
			callback(o, dataCache, securityData)
		}()
	}
	return result
}

// SetTradeCandleStick sets the latest trade candlestick
func (o *optionsContractData) SetTradeCandleStick(tradeCandleStick *OptionsTradeCandleStick) bool {
	if o.latestTradeCandleStick == nil || (tradeCandleStick != nil && tradeCandleStick.Timestamp > o.latestTradeCandleStick.Timestamp) {
		o.latestTradeCandleStick = tradeCandleStick
		return true
	}
	return false
}

// SetTradeCandleStickWithCallback sets the latest trade candlestick with callback
func (o *optionsContractData) SetTradeCandleStickWithCallback(tradeCandleStick *OptionsTradeCandleStick, callback OnOptionsTradeCandleStickUpdated, securityData SecurityData, dataCache DataCache) bool {
	result := o.SetTradeCandleStick(tradeCandleStick)
	if result && callback != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Log error here if logging is available
				}
			}()
			callback(o, dataCache, securityData)
		}()
	}
	return result
}

// SetQuoteCandleStick sets the latest quote candlestick
func (o *optionsContractData) SetQuoteCandleStick(quoteCandleStick *OptionsQuoteCandleStick) bool {
	if quoteCandleStick == nil {
		return false
	}
	
	if quoteCandleStick.Type == QuoteTypeAsk {
		if o.latestAskQuoteCandleStick == nil || quoteCandleStick.Timestamp > o.latestAskQuoteCandleStick.Timestamp {
			o.latestAskQuoteCandleStick = quoteCandleStick
			return true
		}
	} else if quoteCandleStick.Type == QuoteTypeBid {
		if o.latestBidQuoteCandleStick == nil || quoteCandleStick.Timestamp > o.latestBidQuoteCandleStick.Timestamp {
			o.latestBidQuoteCandleStick = quoteCandleStick
			return true
		}
	}
	return false
}

// SetQuoteCandleStickWithCallback sets the latest quote candlestick with callback
func (o *optionsContractData) SetQuoteCandleStickWithCallback(quoteCandleStick *OptionsQuoteCandleStick, callback OnOptionsQuoteCandleStickUpdated, securityData SecurityData, dataCache DataCache) bool {
	result := o.SetQuoteCandleStick(quoteCandleStick)
	if result && callback != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Log error here if logging is available
				}
			}()
			callback(o, dataCache, securityData)
		}()
	}
	return result
}

// GetSupplementaryDatum returns a supplementary datum
func (o *optionsContractData) GetSupplementaryDatum(key string) *float64 {
	o.supplementaryDataMutex.RLock()
	defer o.supplementaryDataMutex.RUnlock()
	
	if value, exists := o.supplementaryData[key]; exists {
		return value
	}
	return nil
}

// SetSupplementaryDatum sets a supplementary datum
func (o *optionsContractData) SetSupplementaryDatum(key string, datum *float64, update SupplementalDatumUpdate) bool {
	o.supplementaryDataMutex.Lock()
	defer o.supplementaryDataMutex.Unlock()
	
	oldValue := o.supplementaryData[key]
	newValue := update(key, oldValue, datum)
	
	if newValue != oldValue {
		o.supplementaryData[key] = newValue
		return true
	}
	return false
}

// SetSupplementaryDatumWithCallback sets a supplementary datum with callback
func (o *optionsContractData) SetSupplementaryDatumWithCallback(key string, datum *float64, callback OnOptionsContractSupplementalDatumUpdated, securityData SecurityData, dataCache DataCache, update SupplementalDatumUpdate) bool {
	result := o.SetSupplementaryDatum(key, datum, update)
	if result && callback != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Log error here if logging is available
				}
			}()
			callback(key, datum, o, securityData, dataCache)
		}()
	}
	return result
}

// GetAllSupplementaryData returns all supplementary data
func (o *optionsContractData) GetAllSupplementaryData() map[string]*float64 {
	o.supplementaryDataMutex.RLock()
	defer o.supplementaryDataMutex.RUnlock()
	
	result := make(map[string]*float64)
	for k, v := range o.supplementaryData {
		result[k] = v
	}
	return result
} 