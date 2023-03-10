package main

import (
	"log"
	"sync"
	"time"

	"github.com/intrinio/intrinio-realtime-options-go-sdk"
)

var eTradeCount int = 0
var eTradeCountLock sync.RWMutex
var eQuoteCount int = 0
var eQuoteCountLock sync.RWMutex

func handleEquityTrade(trade intrinio.EquityTrade) {
	eTradeCountLock.Lock()
	eTradeCount++
	eTradeCountLock.Unlock()
	// if tradeCount%20 == 0 {
	// 	log.Printf("%+v\n", trade)
	// }
}

func handleEquityQuote(quote intrinio.EquityQuote) {
	eQuoteCountLock.Lock()
	eQuoteCount++
	eQuoteCountLock.Unlock()
	// if quoteCount%200 == 0 {
	// 	log.Printf("%+v\n", quote)
	// }
}

func reportEquities(ticker <-chan time.Time) {
	for {
		<-ticker
		eTradeCountLock.RLock()
		tc := eTradeCount
		eTradeCountLock.RUnlock()
		eQuoteCountLock.RLock()
		qc := eQuoteCount
		eQuoteCountLock.RUnlock()
		log.Printf("Equity Trade Count: %d, Equity Quote Count: %d\n", tc, qc)
	}
}

func runEquitiesExample() *intrinio.Client {
	var config intrinio.Config = intrinio.LoadConfig("equities-config.json")
	var client *intrinio.Client = intrinio.NewEquitiesClient(config, handleEquityTrade, handleEquityQuote)
	client.Start()
	//symbols := []string{"AAPL", "MSFT"}
	client.Join("GOOG")
	//client.JoinMany(symbols)
	//client.JoinLobby()
	var ticker *time.Ticker = time.NewTicker(30 * time.Second)
	go reportEquities(ticker.C)
	return client
}
