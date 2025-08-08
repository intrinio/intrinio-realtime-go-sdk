package composite

// OnSupplementalDatumUpdated is called when supplemental data is updated
type OnSupplementalDatumUpdated func(key string, datum *float64, dataCache DataCache)

// OnSecuritySupplementalDatumUpdated is called when security supplemental data is updated
type OnSecuritySupplementalDatumUpdated func(key string, datum *float64, securityData SecurityData, dataCache DataCache)

// OnOptionsContractSupplementalDatumUpdated is called when options contract supplemental data is updated
type OnOptionsContractSupplementalDatumUpdated func(key string, datum *float64, optionsContractData OptionsContractData, securityData SecurityData, dataCache DataCache)

// OnEquitiesTradeUpdated is called when equities trade is updated
type OnEquitiesTradeUpdated func(securityData SecurityData, dataCache DataCache)

// OnEquitiesQuoteUpdated is called when equities quote is updated
type OnEquitiesQuoteUpdated func(securityData SecurityData, dataCache DataCache)

// OnEquitiesTradeCandleStickUpdated is called when equities trade candlestick is updated
type OnEquitiesTradeCandleStickUpdated func(securityData SecurityData, dataCache DataCache)

// OnEquitiesQuoteCandleStickUpdated is called when equities quote candlestick is updated
type OnEquitiesQuoteCandleStickUpdated func(securityData SecurityData, dataCache DataCache)

// OnOptionsTradeUpdated is called when options trade is updated
type OnOptionsTradeUpdated func(optionsTrade OptionsContractData, dataCache DataCache, securityData SecurityData)

// OnOptionsQuoteUpdated is called when options quote is updated
type OnOptionsQuoteUpdated func(optionsContractData OptionsContractData, dataCache DataCache, securityData SecurityData)

// OnOptionsRefreshUpdated is called when options refresh is updated
type OnOptionsRefreshUpdated func(optionsContractData OptionsContractData, dataCache DataCache, securityData SecurityData)

// OnOptionsUnusualActivityUpdated is called when options unusual activity is updated
type OnOptionsUnusualActivityUpdated func(optionsContractData OptionsContractData, dataCache DataCache, securityData SecurityData)

// OnOptionsTradeCandleStickUpdated is called when options trade candlestick is updated
type OnOptionsTradeCandleStickUpdated func(optionsContractData OptionsContractData, dataCache DataCache, securityData SecurityData)

// OnOptionsQuoteCandleStickUpdated is called when options quote candlestick is updated
type OnOptionsQuoteCandleStickUpdated func(optionsContractData OptionsContractData, dataCache DataCache, securityData SecurityData)

// SupplementalDatumUpdate is a function that determines how to update supplemental data
type SupplementalDatumUpdate func(key string, oldValue, newValue *float64) *float64

// CalculateNewGreek is called to calculate new Greeks
type CalculateNewGreek func(optionsContractData OptionsContractData, securityData SecurityData, dataCache DataCache) 