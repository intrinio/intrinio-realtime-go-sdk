package composite

import (
	"github.com/intrinio/intrinio-realtime-go-sdk"
)

// SecurityData represents the interface for security data
type SecurityData interface {
	GetTickerSymbol() string
	
	GetLatestEquitiesTrade() *intrinio.EquityTrade
	GetLatestEquitiesAskQuote() *intrinio.EquityQuote
	GetLatestEquitiesBidQuote() *intrinio.EquityQuote
	
	GetLatestEquitiesTradeCandleStick() *TradeCandleStick
	GetLatestEquitiesAskQuoteCandleStick() *QuoteCandleStick
	GetLatestEquitiesBidQuoteCandleStick() *QuoteCandleStick

	GetSupplementaryDatum(key string) *float64
	SetSupplementaryDatum(key string, datum *float64, update SupplementalDatumUpdate) bool
	SetSupplementaryDatumWithCallback(key string, datum *float64, callback OnSecuritySupplementalDatumUpdated, dataCache DataCache, update SupplementalDatumUpdate) bool

	GetAllSupplementaryData() map[string]*float64

	SetEquitiesTrade(trade *intrinio.EquityTrade) bool
	SetEquitiesTradeWithCallback(trade *intrinio.EquityTrade, callback OnEquitiesTradeUpdated, dataCache DataCache) bool

	SetEquitiesQuote(quote *intrinio.EquityQuote) bool
	SetEquitiesQuoteWithCallback(quote *intrinio.EquityQuote, callback OnEquitiesQuoteUpdated, dataCache DataCache) bool

	SetEquitiesTradeCandleStick(tradeCandleStick *TradeCandleStick) bool
	SetEquitiesTradeCandleStickWithCallback(tradeCandleStick *TradeCandleStick, callback OnEquitiesTradeCandleStickUpdated, dataCache DataCache) bool

	SetEquitiesQuoteCandleStick(quoteCandleStick *QuoteCandleStick) bool
	SetEquitiesQuoteCandleStickWithCallback(quoteCandleStick *QuoteCandleStick, callback OnEquitiesQuoteCandleStickUpdated, dataCache DataCache) bool

	GetOptionsContractData(contract string) OptionsContractData
	
	GetAllOptionsContractData() map[string]OptionsContractData

	GetContractNames() []string

	GetOptionsContractTrade(contract string) *intrinio.OptionTrade

	SetOptionsContractTrade(trade *intrinio.OptionTrade) bool
	SetOptionsContractTradeWithCallback(trade *intrinio.OptionTrade, callback OnOptionsTradeUpdated, dataCache DataCache) bool

	GetOptionsContractQuote(contract string) *intrinio.OptionQuote

	SetOptionsContractQuote(quote *intrinio.OptionQuote) bool
	SetOptionsContractQuoteWithCallback(quote *intrinio.OptionQuote, callback OnOptionsQuoteUpdated, dataCache DataCache) bool

	GetOptionsContractRefresh(contract string) *intrinio.OptionRefresh

	SetOptionsContractRefresh(refresh *intrinio.OptionRefresh) bool
	SetOptionsContractRefreshWithCallback(refresh *intrinio.OptionRefresh, callback OnOptionsRefreshUpdated, dataCache DataCache) bool

	GetOptionsContractUnusualActivity(contract string) *OptionsUnusualActivity

	SetOptionsContractUnusualActivity(unusualActivity *OptionsUnusualActivity) bool
	SetOptionsContractUnusualActivityWithCallback(unusualActivity *OptionsUnusualActivity, callback OnOptionsUnusualActivityUpdated, dataCache DataCache) bool

	GetOptionsContractTradeCandleStick(contract string) *OptionsTradeCandleStick

	SetOptionsContractTradeCandleStick(tradeCandleStick *OptionsTradeCandleStick) bool
	SetOptionsContractTradeCandleStickWithCallback(tradeCandleStick *OptionsTradeCandleStick, callback OnOptionsTradeCandleStickUpdated, dataCache DataCache) bool

	GetOptionsContractBidQuoteCandleStick(contract string) *OptionsQuoteCandleStick
	GetOptionsContractAskQuoteCandleStick(contract string) *OptionsQuoteCandleStick

	SetOptionsContractQuoteCandleStick(quoteCandleStick *OptionsQuoteCandleStick) bool
	SetOptionsContractQuoteCandleStickWithCallback(quoteCandleStick *OptionsQuoteCandleStick, callback OnOptionsQuoteCandleStickUpdated, dataCache DataCache) bool

	GetOptionsContractSupplementalDatum(contract, key string) *float64

	SetOptionsContractSupplementalDatum(contract, key string, datum *float64, update SupplementalDatumUpdate) bool
	SetOptionsContractSupplementalDatumWithCallback(contract, key string, datum *float64, callback OnOptionsContractSupplementalDatumUpdated, dataCache DataCache, update SupplementalDatumUpdate) bool
	
	GetOptionsContractGreekData(contract, key string) *Greek

	SetOptionsContractGreekData(contract, key string, data *Greek, update GreekDataUpdate) bool
	SetOptionsContractGreekDataWithCallback(contract, key string, data *Greek, callback OnOptionsContractGreekDataUpdated, dataCache DataCache, update GreekDataUpdate) bool
}

// OptionsContractData represents the interface for options contract data
type OptionsContractData interface {
	GetContract() string
	
	GetLatestTrade() *intrinio.OptionTrade
	GetLatestQuote() *intrinio.OptionQuote
	GetLatestRefresh() *intrinio.OptionRefresh
	GetLatestUnusualActivity() *OptionsUnusualActivity
	GetLatestTradeCandleStick() *OptionsTradeCandleStick
	GetLatestAskQuoteCandleStick() *OptionsQuoteCandleStick
	GetLatestBidQuoteCandleStick() *OptionsQuoteCandleStick
	
	SetTrade(trade *intrinio.OptionTrade) bool
	SetTradeWithCallback(trade *intrinio.OptionTrade, callback OnOptionsTradeUpdated, securityData SecurityData, dataCache DataCache) bool
	SetQuote(quote *intrinio.OptionQuote) bool
	SetQuoteWithCallback(quote *intrinio.OptionQuote, callback OnOptionsQuoteUpdated, securityData SecurityData, dataCache DataCache) bool
	SetRefresh(refresh *intrinio.OptionRefresh) bool
	SetRefreshWithCallback(refresh *intrinio.OptionRefresh, callback OnOptionsRefreshUpdated, securityData SecurityData, dataCache DataCache) bool
	SetUnusualActivity(unusualActivity *OptionsUnusualActivity) bool
	SetUnusualActivityWithCallback(unusualActivity *OptionsUnusualActivity, callback OnOptionsUnusualActivityUpdated, securityData SecurityData, dataCache DataCache) bool
	SetTradeCandleStick(tradeCandleStick *OptionsTradeCandleStick) bool
	SetTradeCandleStickWithCallback(tradeCandleStick *OptionsTradeCandleStick, callback OnOptionsTradeCandleStickUpdated, securityData SecurityData, dataCache DataCache) bool
	SetQuoteCandleStick(quoteCandleStick *OptionsQuoteCandleStick) bool
	SetQuoteCandleStickWithCallback(quoteCandleStick *OptionsQuoteCandleStick, callback OnOptionsQuoteCandleStickUpdated, securityData SecurityData, dataCache DataCache) bool
	
	GetSupplementaryDatum(key string) *float64
	SetSupplementaryDatum(key string, datum *float64, update SupplementalDatumUpdate) bool
	SetSupplementaryDatumWithCallback(key string, datum *float64, callback OnOptionsContractSupplementalDatumUpdated, securityData SecurityData, dataCache DataCache, update SupplementalDatumUpdate) bool
	GetAllSupplementaryData() map[string]*float64
	
	GetGreekData(key string) *Greek
	SetGreekData(key string, datum *Greek, update GreekDataUpdate) bool
	SetGreekDataWithCallback(key string, datum *Greek, callback OnOptionsContractGreekDataUpdated, securityData SecurityData, dataCache DataCache, update GreekDataUpdate) bool
	GetAllGreekData() map[string]*Greek
}

// DataCache represents the interface for the data cache
type DataCache interface {
	// Supplementary Data methods
	GetSupplementaryDatum(key string) *float64
	SetSupplementaryDatum(key string, datum *float64, update SupplementalDatumUpdate) bool
	GetAllSupplementaryData() map[string]*float64
	
	GetSecuritySupplementalDatum(tickerSymbol, key string) *float64
	SetSecuritySupplementalDatum(tickerSymbol, key string, datum *float64, update SupplementalDatumUpdate) bool
	
	GetOptionsContractSupplementalDatum(tickerSymbol, contract, key string) *float64
	SetOptionSupplementalDatum(tickerSymbol, contract, key string, datum *float64, update SupplementalDatumUpdate) bool
	
	// Greek Data methods
	GetOptionsContractGreekData(tickerSymbol, contract, key string) *Greek
	SetOptionGreekData(tickerSymbol, contract, key string, data *Greek, update GreekDataUpdate) bool
	
	// Sub-caches
	GetSecurityData(tickerSymbol string) SecurityData
	GetAllSecurityData() map[string]SecurityData
	
	GetOptionsContractData(tickerSymbol, contract string) OptionsContractData
	GetAllOptionsContractData(tickerSymbol string) map[string]OptionsContractData
	
	// Equities methods
	GetLatestEquityTrade(tickerSymbol string) *intrinio.EquityTrade
	SetEquityTrade(trade *intrinio.EquityTrade) bool
	
	GetLatestEquityAskQuote(tickerSymbol string) *intrinio.EquityQuote
	GetLatestEquityBidQuote(tickerSymbol string) *intrinio.EquityQuote
	SetEquityQuote(quote *intrinio.EquityQuote) bool
	
	GetLatestEquityTradeCandleStick(tickerSymbol string) *TradeCandleStick
	SetEquityTradeCandleStick(tradeCandleStick *TradeCandleStick) bool
	
	GetLatestEquityAskQuoteCandleStick(tickerSymbol string) *QuoteCandleStick
	GetLatestEquityBidQuoteCandleStick(tickerSymbol string) *QuoteCandleStick
	SetEquityQuoteCandleStick(quoteCandleStick *QuoteCandleStick) bool
	
	// Options methods
	GetLatestOptionsTrade(tickerSymbol, contract string) *intrinio.OptionTrade
	SetOptionsTrade(trade *intrinio.OptionTrade) bool
	
	GetLatestOptionsQuote(tickerSymbol, contract string) *intrinio.OptionQuote
	SetOptionsQuote(quote *intrinio.OptionQuote) bool
	
	GetLatestOptionsRefresh(tickerSymbol, contract string) *intrinio.OptionRefresh
	SetOptionsRefresh(refresh *intrinio.OptionRefresh) bool
	
	GetLatestOptionsUnusualActivity(tickerSymbol, contract string) *OptionsUnusualActivity
	SetOptionsUnusualActivity(unusualActivity *OptionsUnusualActivity) bool
	
	GetLatestOptionsTradeCandleStick(tickerSymbol, contract string) *OptionsTradeCandleStick
	SetOptionsTradeCandleStick(tradeCandleStick *OptionsTradeCandleStick) bool
	
	GetOptionsAskQuoteCandleStick(tickerSymbol, contract string) *OptionsQuoteCandleStick
	GetOptionsBidQuoteCandleStick(tickerSymbol, contract string) *OptionsQuoteCandleStick
	SetOptionsQuoteCandleStick(quoteCandleStick *OptionsQuoteCandleStick) bool
	
	// Callbacks
	SetSupplementalDatumUpdatedCallback(callback OnSupplementalDatumUpdated)
	SetSecuritySupplementalDatumUpdatedCallback(callback OnSecuritySupplementalDatumUpdated)
	SetOptionsContractSupplementalDatumUpdatedCallback(callback OnOptionsContractSupplementalDatumUpdated)
	
	SetEquitiesTradeUpdatedCallback(callback OnEquitiesTradeUpdated)
	SetEquitiesQuoteUpdatedCallback(callback OnEquitiesQuoteUpdated)
	SetEquitiesTradeCandleStickUpdatedCallback(callback OnEquitiesTradeCandleStickUpdated)
	SetEquitiesQuoteCandleStickUpdatedCallback(callback OnEquitiesQuoteCandleStickUpdated)
	
	SetOptionsTradeUpdatedCallback(callback OnOptionsTradeUpdated)
	SetOptionsQuoteUpdatedCallback(callback OnOptionsQuoteUpdated)
	SetOptionsRefreshUpdatedCallback(callback OnOptionsRefreshUpdated)
	SetOptionsUnusualActivityUpdatedCallback(callback OnOptionsUnusualActivityUpdated)
	SetOptionsTradeCandleStickUpdatedCallback(callback OnOptionsTradeCandleStickUpdated)
	SetOptionsQuoteCandleStickUpdatedCallback(callback OnOptionsQuoteCandleStickUpdated)
	SetOptionsContractGreekDataUpdatedCallback(callback OnOptionsContractGreekDataUpdated)
}