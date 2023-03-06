package main

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/intrinio/intrinio-realtime-options-go-sdk"
)

var refreshCount int = 0
var refreshCountLock sync.RWMutex
var tradeCount int = 0
var tradeCountLock sync.RWMutex
var quoteCount int = 0
var quoteCountLock sync.RWMutex
var uaCount int = 0
var uaCountLock sync.RWMutex

func handleRefresh(refresh intrinio.Refresh) {
	refreshCountLock.Lock()
	defer refreshCountLock.Unlock()
	refreshCount++
	//log.Printf("%+v\n", refresh)
}

func handleTrade(trade intrinio.Trade) {
	tradeCountLock.Lock()
	defer tradeCountLock.Unlock()
	tradeCount++
	log.Printf("Contract: %s, Strike Price: %.2f, Exp: %v", trade.ContractId, trade.GetStrikePrice(), trade.GetExpirationDate())
	//log.Printf("%+v\n", trade)
}

func handleQuote(quote intrinio.Quote) {
	quoteCountLock.Lock()
	defer quoteCountLock.Unlock()
	quoteCount++
	//log.Printf("%+v\n", quote)
}

func handleUA(ua intrinio.UnusualActivity) {
	uaCountLock.Lock()
	defer uaCountLock.Unlock()
	uaCount++
	//log.Printf("%+v\n", ua)
}
func report(ticker <-chan time.Time) {
	for {
		<-ticker
		refreshCountLock.RLock()
		rc := refreshCount
		refreshCountLock.RUnlock()
		tradeCountLock.RLock()
		tc := tradeCount
		tradeCountLock.RUnlock()
		quoteCountLock.RLock()
		qc := quoteCount
		quoteCountLock.RUnlock()
		uaCountLock.RLock()
		uac := uaCount
		uaCountLock.RUnlock()
		log.Printf("Trade Count: %d, Quote Count: %d, UA Count: %d, Refresh Count: %d\n", tc, qc, uac, rc)
	}
}

func main() {
	log.Println("EXAMPLE - Starting")
	var config intrinio.Config = intrinio.LoadConfig()
	var client *intrinio.Client = intrinio.NewClient(config, handleTrade, nil, handleRefresh, handleUA)
	close := make(chan os.Signal, 1)
	signal.Notify(close, syscall.SIGINT, syscall.SIGTERM)
	client.Start()
	// symbols := []string{"SPY___230227C00400000", "SPY___230227C00398000", "SPY___230227C00399000",
	// 	"SPY___230227P00397000", "SPY___230227P00398000", "SPY___230227P00399000"}
	symbols := []string{"SPX", "SPXW"}
	client.JoinMany(symbols)
	//client.JoinLobby()
	var ticker *time.Ticker = time.NewTicker(30 * time.Second)
	go report(ticker.C)
	<-close
	log.Println("EXAMPLE - Closing")
	client.Stop()
}
