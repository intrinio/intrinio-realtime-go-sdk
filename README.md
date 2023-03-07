# intrinio-realtime-options-go-sdk
Go SDK for working with Intrinio's Real-Time Option Price WebSocket Feed

[Intrinio](https://intrinio.com/) provides real-time stock option prices via a two-way WebSocket connection. To get started, [subscribe to a real-time data feed](https://intrinio.com/financial-market-data/options-data) and follow the instructions below.

## Requirements

- Go 1.20 (or newer) recommended

## Installation

### Option 1 - Docker

1. Source files can be downloaded from: github.com/intrinio/intrinio-realtime-options-go-sdk
2. Navigate to the project root
3. Update example/config.json with your api key
3. Run `docker compose build`
4. Run `docker compose run example`

### Option 2 - From source

1. Source files can be downloaded from: github.com/intrinio/intrinio-realtime-options-go-sdk
2. Navigate to the project root
3. Open the example project at project-root/example
4. Build your project using example.go as the base

### Option 3 - Pre built package
1. Create a new Go project
2. import "github.com/intrinio/intrinio-realtime-options-go-sdk"
3. Reference the package "intrinio"

## Example Project
For a sample Go application see: [intrinio-realtime-options-go-sdk](https://github.com/intrinio/intrinio-realtime-options-go-sdk/example)

## Features

* Receive streaming, real-time option price updates:
	* every trade
	* conflated bid and ask
	* open interest, open, close, high, low
	* unusual activity(block trades, sweeps, whale trades, unusual sweeps)
* Subscribe to updates from individual option contracts (or option chains)
* Subscribe to updates for the entire universe of option contracts (~1.5M option contracts)

## Example Usage
```go

package main

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/intrinio/intrinio-realtime-options-go-sdk"
)

func handleRefresh(refresh intrinio.Refresh) {
}

func handleTrade(trade intrinio.Trade) {
}

func handleQuote(quote intrinio.Quote) {
}

func handleUA(ua intrinio.UnusualActivity) {
}

func main() {
	var config intrinio.Config = intrinio.LoadConfig()
	var client *intrinio.Client = intrinio.NewClient(config, handleTrade, nil, handleRefresh, nil)
	close := make(chan os.Signal, 1)
	signal.Notify(close, syscall.SIGINT, syscall.SIGTERM)
	client.Start()
	symbols := []string{"GE", "MSFT", "SPY___230227P00397000", "SPY___230227P00399000"}
	client.JoinMany(symbols)
	//client.JoinLobby()
	<-close
	client.Stop()
}
```

## Handling Quotes

There are millions of options contracts, each with their own feed of activity.
We highly encourage you to make your OnTrade, OnQuote, OnUnusualActivity, and OnRefresh callback methods has short as possible and follow a queue pattern so your app can handle the large volume of activity.
Note that quotes (ask and bid updates) comprise 99% of the volume of the entire feed. Be cautious when deciding to receive quote updates. You will receive the latest 'ask' and 'bid' price with each each trade update. You may subscribe to receive a quote updates for ask/bid prices (by providing an OnQuote callback to `intrinio.NewClient`) but, again, we recommend caution when electing to do this.

## Providers

Currently, Intrino offers realtime data for this SDK from the following providers:

* OPRA - [Homepage](https://www.opraplan.com/)

Please be sure that the correct provider is specified in the `intrinio.Config` object that is passed to the `intrinio.NewClient` routine.

## Data Format

### Trade Message

```go
type Trade struct
```

* **ContractId** - Identifier for the option contract. This includes the ticker symbol, put/call, expiry, and strike price.
* **Price** - The trade price in USD
* **Size** - The size of the trade (note: each contract represents a lot of 100 underlying shares).
* **TotalVolume** - The total number of contracts (with the given Id) traded so far, today.
* **Timestamp** - The time of the trade, as a Unix timestamp (with microsecond precision)
* **AskPriceAtExecution** - The best, last ask price in USD
* **BidPriceAtExecution** - The best, last bid price in USD
* **UnderlyingPriceAtExecution** - The price of the underlying security in USD

### Quote Message

```go
type Quote struct
```

* **ContractId** - Identifier for the option contract. This includes the ticker symbol, put/call, expiry, and strike price.
* **AskPrice** - The last, best ask price in USD
* **AskSize** - The last, best ask size (note: each contract represents a lot of 100 underlying shares).
* **BidPrice** - The last, best bid price in USD
* **BidSize** - The last, best bid size (note: each contract represents a lot of 100 underlying shares).
* **Timestamp** - The time of the quote, as a Unix timestamp (with microsecond precision)

### Refresh Message

```go
type Refresh
```

* **ContractId** - Identifier for the options contract. This includes the ticker symbol, put/call, expiry, and strike price.
* **OpenInterest** - The total quantity of opened contracts, as reported at the start of the trading day
* **OpenPrice** - The open price price in USD
* **ClosePrice** - The close price in USD
* **HighPrice** - The current high price in USD
* **LowPrice** - The current low price in USD

### Unusual Activity Message

```go
type UnusualActivity
```

* **ContractId** - Identifier for the options contract. This includes the ticker symbol, put/call, expiry, and strike price.
* **Type** - The type of unusual activity that was detected
  * **`Block`** - represents a 'block' trade of at least $20,000
  * **`Sweep`** - represents an intermarket sweep of at least $10,000
  * **`Large`** - represents a trade of at least $100,000
  * **`Unusual Sweep`** - represents an unusually large sweep (more than 2 standard deviation above the market-wide sweep mean).
* **Sentiment** - The sentiment of the unusual activity event
  *    **`Neutral`** - The event was executed with apparent neutral outlook of the underlying security
  *    **`Bullish`** - The event was executed with apparent positive outlook of the underlying security
  *    **`Bearish`** - The event was executed with apparent negative outlook of the underlying security
* **TotalValue** - The total value of the event in USD. 'Sweeps' and 'blocks' can be comprised of multiple trades. This is the value of the entire event.
* **TotalSize** - The total size of the event in number of contracts. 'Sweeps' and 'blocks' can be comprised of multiple trades. This is the total number of contracts exchanged during the event.
* **AveragePrice** - The average price at which the event was executed. 'Sweeps' and 'blocks' can be comprised of multiple trades. This is the average trade price for the entire event.
* **AskPriceAtExecution** - The 'ask' price of the contract at execution of the event.
* **BidPriceAtExecution** - The 'bid' price of the contract at execution of the event.
* **UnderlyingPriceAtExecution** - The last trade price of the underlying security at execution of the event.
* **Timestamp** - The time of the event, as a Unix timestamp (with microsecond precision).

## API Keys

You will receive your Intrinio API Key after [creating an account](https://intrinio.com/signup). You will need a subscription to a [realtime data feed](https://intrinio.com/financial-market-data/options-data) as well.

Please be sure to include you API key in the `intrinio.Config` object passed to the `intrinio.NewClient` routine.

## Documentation

### Overview

The Intrinio Realtime Client will handle authorization as well as establishment and management of all necessary WebSocket connections. All you need to get started is your API key.
The first thing that you'll do is create a new `intrinio.Client` object, passing in an `intrinio.Config` object as well as a series of callback functions. These callback methods tell the client what types of subscriptions you will be setting up.
A helper function, `intrinio.LoadConfig()`, is provided to automatically load a `config.json` file that exists in your application's working directory. Please be sure that your API key is specified in the `intrinio.Config` object that is passed to the `intrinio.NewClient` routine.
Creating an `intrinio.Client` object (with `intrinio.NewClient`) will initialize the object but you will need to call the client object's `Start()` method in order to open the session and start communication with the server.
After an `intrinio.Client` object has been created and started, you may subscribe to receive feed updates from the server.
You may subscribe, dynamically, to option contracts, option chains, or a mixed list thereof.
It is also possible to subscribe to the entire universe of option contracts (i.e. the firehose) by calling the client object's `JoinLobby()` method.
The volume of data provided by the `Firehose` can exceed 100Mbps and requires special authorization.
You may update your subscriptions on the fly, using the client object's `Join` and `Leave` methods.
The WebSocket client is designed for near-indefinite operation. It will automatically reconnect if a connection drops/fails and when then servers turn on every morning.
If you wish to perform a shutdown of the application, please call the client's `Stop` method. See the example application for an example of how to handle system SIGINT (Ctrl+C) and SIGTERM signals

### Methods

`var client Client = NewClient(config, onTrade, onQuote, onRefresh, onUnusualActivity)` - Creates an Intrinio Real-Time client.
* **Parameter** `config`: Required. The configuration object necessary to set up the client.
* **Parameter** `onTrade`: Optional. The callback accepting `intrinio.Trade` updates. If `onTrade` is `nil`, you will not receive trade updates from the server.
* **Parameter** `onQuote`: Optional. The callback accepting `intrinio.Quote` updates. If `onQuote` is `nil`, you will not receive quote (ask, bid) updates from the server.
* **Parameter** `onRefresh`: Optional. The callback accepting `intrinio.Refresh` updates. If `onRefresh` is `nil`, you will not receive open interest, open, close, high, low data from the server. Note: open interest data is only updated at the beginning of every trading day. If this callback is provided you will recieve an update immediately, as well as every 15 minutes (approx).
* **Parameter** `onUnusualActivity`: Optional. The callback accepting `intrinio.UnusualActivity` updats. If `onUnusualActivity` is `nil`, you will not receive unusual activity updates from the server.

`client.Start()` - Starts the client (authenticates the user and establishes the websocket connection)
`client.Stop()` - Leaves all joined channels and gracefully terminates the session. 

`client.Join(symbol string)` - Joins the channel identified by the given symbol (e.g. "AAPL" or "GOOG__210917C01040000")
`client.JoinMany(symbols []string)` - Joins the channels identified by the given symbol slice (e.g. `[]string{"AAPL", "MSFT__210917C00180000", "GOOG__210917C01040000"}`)
`client.JoinLobby()` - Joins the lobby (i.e. 'Firehose') channel. This requires special account permissions.

`client.LeaveAll()` - Leaves all channels that have been subscribed to by the client
`client.Leave(symbol string)` - Leaves the channel identified by the given symbol
`client.LeaveMany(symbols []string)` - Leaves the channels identified by the given symbol slice
`client.LeaveLobby()` - Leaves the lobby channel.

## Configuration

Configuration is done through a configuration object (`intrinio.Config`) that is passed to the `intrinio.NewClient` routine. You may create a configuration directly in code using:

```go
var config intrinio.Config = intrinio.Config{ApiKey: "YOUR-API-KEY", Provider: "OPRA"}
```
 
 Or, you can create a `config.json` file, of the following form, and place it in your application root. An example of this is provided in the sample project.

```json
{
	"ApiKey": "YOUR-API-KEY",
	"Provider": "OPRA",
}
```
