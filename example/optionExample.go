package main

import (
	"log"
	"sync"
	"time"

	"github.com/intrinio/intrinio-realtime-options-go-sdk"
)

var oRefreshCount int = 0
var oRefreshCountLock sync.RWMutex
var oTradeCount int = 0
var oTradeCountLock sync.RWMutex
var oQuoteCount int = 0
var oQuoteCountLock sync.RWMutex
var oUACount int = 0
var oUACountLock sync.RWMutex

func handleOptionRefresh(refresh intrinio.OptionRefresh) {
	oRefreshCountLock.Lock()
	oRefreshCount++
	oRefreshCountLock.Unlock()
	// if refreshCount%100000 == 0 {
	// 	log.Printf("%+v\n", refresh)
	// }
}

func handleOptionTrade(trade intrinio.OptionTrade) {
	oTradeCountLock.Lock()
	oTradeCount++
	oTradeCountLock.Unlock()
	// if tradeCount%20 == 0 {
	// 	log.Printf("%+v\n", trade)
	// }
}

func handleOptionQuote(quote intrinio.OptionQuote) {
	oQuoteCountLock.Lock()
	oQuoteCount++
	oQuoteCountLock.Unlock()
	// if quoteCount%200 == 0 {
	// 	log.Printf("%+v\n", quote)
	// }
}

func handleOptionUA(ua intrinio.OptionUnusualActivity) {
	oUACountLock.Lock()
	oUACount++
	oUACountLock.Unlock()
	//log.Printf("%+v\n", ua)
}

func reportOptions(ticker <-chan time.Time) {
	for {
		<-ticker
		oRefreshCountLock.RLock()
		rc := oRefreshCount
		oRefreshCountLock.RUnlock()
		oTradeCountLock.RLock()
		tc := oTradeCount
		oTradeCountLock.RUnlock()
		oQuoteCountLock.RLock()
		qc := oQuoteCount
		oQuoteCountLock.RUnlock()
		oUACountLock.RLock()
		uac := oUACount
		oUACountLock.RUnlock()
		log.Printf("Option Trade Count: %d, Option Quote Count: %d, Option UA Count: %d, Option Refresh Count: %d\n", tc, qc, uac, rc)
	}
}

func runOptionsExample() *intrinio.Client {
	var config intrinio.Config = intrinio.LoadConfig("options-config.json")
	var client *intrinio.Client = intrinio.NewOptionsClient(config, handleOptionTrade, handleOptionQuote, handleOptionRefresh, handleOptionUA)
	client.Start()
	//symbols := []string{"SPY_230306C404.00", "SPY_230306C405.00", "SPY_230306C406.00"}
	//symbols := []string{"SPY", "AAPL", "SPX", "MSFT", "GE", "TSLA"}
	//symbols := []string{"SPX", "SPXW"}
	//client.Join("GE")
	//client.JoinMany(symbols)
	client.JoinLobby()
	var ticker *time.Ticker = time.NewTicker(30 * time.Second)
	go reportOptions(ticker.C)
	return client
}
