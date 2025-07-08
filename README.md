# intrinio-realtime-go-sdk
Go SDK for working with Intrinio's Real-Time WebSocket Feeds. This package provides facilities for working with both equity and option feeds.

[Intrinio](https://intrinio.com/) provides real-time stock prices and option prices via two-way WebSocket connections. To get started, subscribe to a [real-time equity data feed](https://intrinio.com/real-time-multi-exchange) or [real-time option data feed](https://intrinio.com/financial-market-data/options-data) and follow the instructions below.

## Requirements

- Go 1.20 (or newer) recommended

## Installation

### Option 1 - Docker

1. Source files can be downloaded from: github.com/intrinio/intrinio-realtime-go-sdk
2. Navigate to the project root
3. Update the ENV parameter in the dockerfile with your API key
3. Run `docker compose build`
4. Run `docker compose run example`

### Option 2 - From source

1. Source files can be downloaded from: github.com/intrinio/intrinio-realtime-go-sdk
2. Navigate to the project root
3. Open the example project at project-root/example
4. Build your project using example.go as the base

### Option 3 - Pre built package
1. Create a new Go project
2. import "github.com/intrinio/intrinio-realtime-go-sdk"
3. Reference the package "intrinio"

## Example Project
For a sample Go application see: [intrinio-realtime-options-go-sdk](https://github.com/intrinio/intrinio-realtime-go-sdk/example)

## Features

* Receive streaming, real-time equity price updates:
	* every trade
	* top-of-book ask and bid
* Subscribe to updates from individual securities
* Subscribe to updates for all securities
* Receive streaming, real-time option price updates:
	* every trade
	* conflated bid and ask
	* open interest, open, close, high, low
	* unusual activity(block trades, sweeps, whale trades, unusual sweeps)
* Subscribe to updates from individual option contracts (or option chains)
* Subscribe to updates for the entire universe of option contracts (~1.5M option contracts)
* Receive updates for both equity share and option contract updates, simultaneously

## Example Usage
```go

package main

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/intrinio/intrinio-realtime-go-sdk"
)

func handleEquityTrade(trade intrinio.EquityTrade) {
}

func handleEquityQuote(quote intrinio.EquityQuote) {
}

func handleOptionRefresh(refresh intrinio.OptionRefresh) {
}

func handleOptionTrade(trade intrinio.OptionTrade) {
}

func handleOptionQuote(quote intrinio.OptionQuote) {
}

func handleOptionUA(ua intrinio.OptionUnusualActivity) {
}

func main() {
	var equitiesConfig intrinio.Config = intrinio.LoadConfig("equities-config.json")
	var optionsConfig intrinio.Config = intrinio.LoadConfig("options-config.json")
	var equitiesClient *intrinio.Client = intrinio.NewEquitiesClient(equitiesConfig, handleEquityTrade, handleEquityQuote)
	var optionsClient *intrinio.Client = intrinio.NewOptionsClient(optionsConfig, handleOptionTrade, nil, handleOptionRefresh, nil)
	close := make(chan os.Signal, 1)
	signal.Notify(close, syscall.SIGINT, syscall.SIGTERM)
	equitiesClient.Start()
	optionsClient.Start()
	symbols := []string{"GE", "MSFT"}
	equitiesClient.JoinMany(symbols)
	optionsClient.JoinMany(symbols)
	//client.JoinLobby()
	<-close
	equitiesClient.Stop()
	optionsClient.Stop()
}
```

## Usage notes (applies to both equity and option clients)

There are thousands of securities and millions of options contracts, each with their own feed of activity.
We highly encourage you to make your callback methods (e.g. onTrade, onQuote, onUnusualActivity, onRefresh) as short as possible and follow a queue pattern so your app can handle the large volume of activity.
Note that quotes (ask and bid updates) comprise 90-99% of the volume of the entire feed. Be cautious when deciding to receive quote updates. With the option feed, you will receive the latest 'ask' and 'bid' price with each each trade update. You may subscribe to receive a quote updates for ask/bid prices (by providing an OnQuote callback to `intrinio.NewOptionsClient` or `intrinio.NewEquitiesClient`) but, again, we recommend caution when electing to do this.

## Providers

Currently, Intrino offers realtime data for this SDK from the following providers:

* DELAYED_SIP
* OPRA - The Option Price Reporting Authority
* IEX
* NASDAQ_BASIC
* CBOE_ONE

Please be sure that the correct provider is specified in the `intrinio.Config` object(s) that are passed to the `intrinio.NewEquitiesClient` or `intrinio.NewOptionsClient` routines. DSIP should be specified for an equities client and OPRA should be specified for an options client.

## Data Format (Equities)

### Trade Message

```go
type EquityTrade struct
```

* **Symbol** - Ticker symbol
* **Price** - The trade price in USD
* **Size** - The size of the trade
* **TotalVolume** - The total number of shares traded so far, today.
* **Timestamp** - The time of the trade, as a Unix timestamp (with microsecond precision)
* **Source** - The sub-source for this trade. NONE = 0, CTA_A = 1, CTA_B = 2, UTP = 3, OTC = 4, NASDAQ_BASIC = 5, IEX = 6, CBOE_ONE = 7

### Quote Message

```go
type EquityQuote struct
```
* **Type** - The quote type
  * **`Ask`** - Represents an 'Ask' type
  * **`Bid`** - Represents a 'Bid' type
* **Symbol** - Ticker symbol
* **Price** - The last, best ask or bid price in USD
* **Size** - The last, best ask or bid size
* **Timestamp** - The time of the quote, as a Unix timestamp (with microsecond precision)

### Trade Conditions (Equities)

| Value | Description                                       |
|-------|---------------------------------------------------|
| @     | Regular Sale                                      |
| A     | Acquisition                                       |
| B     | Bunched Trade                                     |
| C     | Cash Sale                                         |
| D     | Distribution                                      |
| E     | Placeholder                                       |
| F     | Intermarket Sweep                                 |
| G     | Bunched Sold Trade                                |
| H     | Priced Variation Trade                            |
| I     | Odd Lot Trade                                     |
| K     | Rule 155 Trade (AMEX)                             |
| L     | Sold Last                                         |
| M     | Market Center Official Close                      |
| N     | Next Day                                          |
| O     | Opening Prints                                    |
| P     | Prior Reference Price                             |
| Q     | Market Center Official Open                       |
| R     | Seller                                            |
| S     | Split Trade                                       |
| T     | Form T                                            |
| U     | Extended Trading Hours (Sold Out of Sequence)     |
| V     | Contingent Trade                                  |
| W     | Average Price Trade                               |
| X     | Cross/Periodic Auction Trade                      |
| Y     | Yellow Flag Regular Trade                         |
| Z     | Sold (Out of Sequence)                            |
| 1     | Stopped Stock (Regular Trade)                     |
| 4     | Derivatively Priced                               |
| 5     | Re-Opening Prints                                 |
| 6     | Closing Prints                                    |
| 7     | Qualified Contingent Trade (QCT)                  |
| 8     | Placeholder for 611 Exempt                        |
| 9     | Corrected Consolidated Close (Per Listing Market) |


### Equities Trade Conditions (CBOE One)
Trade conditions for CBOE One are represented as the integer representation of a bit flag.

None                      = 0,
UpdateHighLowConsolidated = 1,
UpdateLastConsolidated    = 2,
UpdateHighLowMarketCenter = 4,
UpdateLastMarketCenter    = 8,
UpdateVolumeConsolidated  = 16,
OpenConsolidated          = 32,
OpenMarketCenter          = 64,
CloseConsolidated         = 128,
CloseMarketCenter         = 256,
UpdateVolumeMarketCenter  = 512


### Equities Quote Conditions

| Value | Description                                 |
|-------|---------------------------------------------|
| R     | Regular                                     |
| A     | Slow on Ask                                 |
| B     | Slow on Bid                                 |
| C     | Closing                                     |
| D     | News Dissemination                          |
| E     | Slow on Bid (LRP or Gap Quote)              |
| F     | Fast Trading                                |
| G     | Trading Range Indication                    |
| H     | Slow on Bid and Ask                         |
| I     | Order Imbalance                             |
| J     | Due to Related - News Dissemination         |
| K     | Due to Related - News Pending               |
| O     | Open                                        |
| L     | Closed                                      |
| M     | Volatility Trading Pause                    |
| N     | Non-Firm Quote                              |
| O     | Opening                                     |
| P     | News Pending                                |
| S     | Due to Related                              |
| T     | Resume                                      |
| U     | Slow on Bid and Ask (LRP or Gap Quote)      |
| V     | In View of Common                           |
| W     | Slow on Bid and Ask (Non-Firm)              |
| X     | Equipment Changeover                        |
| Y     | Sub-Penny Trading                           |
| Z     | No Open / No Resume                         |
| 1     | Market Wide Circuit Breaker Level 1         |
| 2     | Market Wide Circuit Breaker Level 2         |        
| 3     | Market Wide Circuit Breaker Level 3         |
| 4     | On Demand Intraday Auction                  |        
| 45    | Additional Information Required (CTS)       |      
| 46    | Regulatory Concern (CTS)                    |     
| 47    | Merger Effective                            |    
| 49    | Corporate Action (CTS)                      |   
| 50    | New Security Offering (CTS)                 |  
| 51    | Intraday Indicative Value Unavailable (CTS) |


## Data Format (Options)

### Trade Message

```go
type OptionTrade struct
```

* **ContractId** - Identifier for the option contract. This includes the ticker symbol, put/call, expiry, and strike price.
* **Exchange** - The specific exchange through which the trade occurred (enum)
* **Price** - The trade price in USD
* **Size** - The size of the trade (note: each contract represents a lot of 100 underlying shares).
* **Qualifiers** - A 4-byte array: each byte represents one trade qualifier. see list of possible [Trade Qualifiers](#trade-qualifiers), below.
* **TotalVolume** - The total number of contracts (with the given Id) traded so far, today.
* **Timestamp** - The time of the trade, as a Unix timestamp (with microsecond precision)
* **AskPriceAtExecution** - The best, last ask price in USD
* **BidPriceAtExecution** - The best, last bid price in USD
* **UnderlyingPriceAtExecution** - The price of the underlying security in USD

### Trade Qualifiers

The trade qualifiers field is represented by a tuple containing 4 integers. Each integer can take one of the following values:
* **`0`** - Regular transaction
* **`2`** - Cancel
* **`3`** - This is the last price and it's cancelled
* **`4`** - Late but in sequence / sold last late
* **`5`** - This was the open price and it's cancelled
* **`6`** - Late report of opening trade and is out of sequence: or set the open
* **`7`** - Cancel only trade reported
* **`8`** - Transaction was executed electronically
* **`9`** - Reopen of a previously halted contract
* **`11`** - Spread
* **`23`** - Intermarket Sweep
* **`30`** - Extended hours
* **`33`** - Crossed trade including Request For Cross RFC
* **`87`** - Complex trade with equity leg
* **`107`** - Auction
* **`123`** - Stock option trade
* **`136`** - Ex-Pit trade
* **`192`** - Message received locally out-of-sequence
* **`222`** - Combo trade
* **`0`** - Blank

Each trade can be qualified by a maximum of 4(four) values. The combination of these values can have special values. These special values are:

* **`107, 23`** - Single leg auction ISO
* **`23, 33`** - Single leg cross ISO
* **`8, 11`** - Multi leg auto-electronic trade
* **`107, 11`** - Multi leg auction
* **`11, 33`** - Multi leg cross
* **`136, 11`** - Multi leg floor trade
* **`8, 11, 87`** - Multi leg auto-electronic trade against single leg(s)
* **`107, 123`** - Stock options auction
* **`107, 11, 87`** - Multi leg auction against single leg(s)
* **`136, 11, 87`** - Multi leg floor trade against single leg(s)
* **`8, 123`** - Stock options auto-electronic trade
* **`123, 33`** - Stock options cross
* **`136, 123`** - Stock options floor trade
* **`8, 87, 123`** - Stock options auto-electronic trade against single leg(s)
* **`107, 87, 123`** - Stock options auction against single leg(s)
* **`136, 87, 123`** - Stock options floor trade against single leg(s)
* **`136, 11, 222`** - Multi leg floor trade of proprietary products
* **`222, 30`** - Multilateral Compression Trade of Proprietary Data Products

### Quote Message

```go
type OptionQuote struct
```

* **ContractId** - Identifier for the option contract. This includes the ticker symbol, put/call, expiry, and strike price.
* **AskPrice** - The last, best ask price in USD
* **AskSize** - The last, best ask size (note: each contract represents a lot of 100 underlying shares).
* **BidPrice** - The last, best bid price in USD
* **BidSize** - The last, best bid size (note: each contract represents a lot of 100 underlying shares).
* **Timestamp** - The time of the quote, as a Unix timestamp (with microsecond precision)

### Refresh Message

```go
type OptionRefresh
```

* **ContractId** - Identifier for the options contract. This includes the ticker symbol, put/call, expiry, and strike price.
* **OpenInterest** - The total quantity of opened contracts, as reported at the start of the trading day
* **OpenPrice** - The open price price in USD
* **ClosePrice** - The close price in USD
* **HighPrice** - The current high price in USD
* **LowPrice** - The current low price in USD

### Unusual Activity Message

```go
type OptionUnusualActivity
```

* **ContractId** - Identifier for the options contract. This includes the ticker symbol, put/call, expiry, and strike price.
* **Type** - The type of unusual activity that was detected
  * **`Block`** - represents a 'block' trade of at least $20,000
  * **`Sweep`** - represents an intermarket sweep of at least $10,000
  * **`Large`** - represents a trade of at least $100,000
  * **`Unusual Sweep`** - represents an unusually large sweep (more than 2 standard deviation above the market-wide sweep mean).
* **Sentiment** - The sentiment of the unusual activity event
  * **`Neutral`** - The event was executed with apparent neutral outlook of the underlying security
  * **`Bullish`** - The event was executed with apparent positive outlook of the underlying security
  * **`Bearish`** - The event was executed with apparent negative outlook of the underlying security
* **TotalValue** - The total value of the event in USD. 'Sweeps' and 'blocks' can be comprised of multiple trades. This is the value of the entire event.
* **TotalSize** - The total size of the event in number of contracts. 'Sweeps' and 'blocks' can be comprised of multiple trades. This is the total number of contracts exchanged during the event.
* **AveragePrice** - The average price at which the event was executed. 'Sweeps' and 'blocks' can be comprised of multiple trades. This is the average trade price for the entire event.
* **AskPriceAtExecution** - The 'ask' price of the contract at execution of the event.
* **BidPriceAtExecution** - The 'bid' price of the contract at execution of the event.
* **UnderlyingPriceAtExecution** - The last trade price of the underlying security at execution of the event.
* **Timestamp** - The time of the event, as a Unix timestamp (with microsecond precision).

## API Keys

You will receive your Intrinio API Key after [creating an account](https://intrinio.com/signup). You will need a subscription to a [realtime equity data feed](https://intrinio.com/real-time-multi-exchange) or [realtime option data feed](https://intrinio.com/financial-market-data/options-data) as well.

Please be sure to include you API key in the `intrinio.Config` object passed to either the `intrinio.NewEquitiesClient` or `intrinio.NewOptionsClient` routine.

Alternatively, you may create an environment variable, `INTRINIO_API_KEY`, and set your API key as the value. The `intrinio.LoadConfig(filename)` function will pick it up from there, automatically. 

## Documentation

### Overview

The Intrinio Realtime Client will handle authorization as well as establishment and management of all necessary WebSocket connections. All you need to get started is your API key.
The first thing that you'll do is create a new `intrinio.Client` object using either the `NewEquitiesClient` or `NewOptionsClient` routine, passing in an `intrinio.Config` object as well as a series of callback functions. These callback methods tell the client what types of subscriptions you will be setting up.
A helper function, `intrinio.LoadConfig(filename string)`, is provided to automatically load a `.json` file that exists in your application's working directory. Please be sure that your API key is specified in the `intrinio.Config` object that is passed to one of the `intrinio.New[Equities/Options]Client` routine.
Creating an `intrinio.Client` object will initialize the object but you will need to call the client object's `Start()` method in order to open the session and start communication with the server.
After an `intrinio.Client` object has been created and started, you may subscribe to receive feed updates from the server.
You may subscribe, dynamically, to individual or multiple ticker symbols (in the case of an Equities client) or to option contracts, option chains, or a mixed list thereof (in the case of an Options client).
It is also possible to subscribe to the entire universe of ticker symbols or option contracts (i.e. the firehose) by calling the client object's `JoinLobby()` method.
The volume of data provided by the `Firehose` can exceed 100Mbps and requires special authorization.
You may update your subscriptions on the fly, using the client object's `Join` and `Leave` methods.
The WebSocket client is designed for near-indefinite operation. It will automatically reconnect if a connection drops/fails and when then servers turn on every morning.
If you wish to perform a shutdown of the application, please call the client's `Stop` method. See the example application for an example of how to handle system SIGINT (Ctrl+C) and SIGTERM signals

### Methods

`var client Client = NewEquitiesClient(config, onTrade, onQuote)` - Creates an Intrinio Real-Time client for use with a real-time equity feed (DSIP).
* **Parameter** `config`: Required. The configuration object necessary to set up the client.
* **Parameter** `onTrade`: Required. The callback accepting `intrinio.EquityTrade` updates.
* **Parameter** `onQuote`: Optional. The callback accepting `intrinio.EquityQuote` updates. If `onQuote` is `nil`, you will not receive quote (ask, bid) updates from the server.

`var client Client = NewOptionsClient(config, onTrade, onQuote, onRefresh, onUnusualActivity)` - Creates an Intrinio Real-Time client for use with a real-time option feed (OPRA).
* **Parameter** `config`: Required. The configuration object necessary to set up the client.
* **Parameter** `onTrade`: Optional. The callback accepting `intrinio.OptionTrade` updates. If `onTrade` is `nil`, you will not receive trade updates from the server.
* **Parameter** `onQuote`: Optional. The callback accepting `intrinio.OptionQuote` updates. If `onQuote` is `nil`, you will not receive quote (ask, bid) updates from the server.
* **Parameter** `onRefresh`: Optional. The callback accepting `intrinio.OptionRefresh` updates. If `onRefresh` is `nil`, you will not receive open interest, open, close, high, low data from the server. Note: open interest data is only updated at the beginning of every trading day. If this callback is provided you will recieve an update immediately, as well as every 15 minutes (approx).
* **Parameter** `onUnusualActivity`: Optional. The callback accepting `intrinio.OptionUnusualActivity` updats. If `onUnusualActivity` is `nil`, you will not receive unusual activity updates from the server.

`client.Start()` - Starts the client (authenticates the user and establishes the websocket connection)
`client.Stop()` - Leaves all joined channels and gracefully terminates the session. 

`client.Join(symbol string)` - Joins the channel identified by the given symbol, contractId, or option chain (e.g. "AAPL" or "GOOG__210917C01040000")
`client.JoinMany(symbols []string)` - Joins the channels identified by the given symbol slice (e.g. `[]string{"AAPL", "MSFT__210917C00180000", "GOOG__210917C01040000"}`)
`client.JoinLobby()` - Joins the lobby (i.e. 'Firehose') channel. This requires special account permissions.

`client.LeaveAll()` - Leaves all channels that have been subscribed to by the client
`client.Leave(symbol string)` - Leaves the channel identified by the given symbol
`client.LeaveMany(symbols []string)` - Leaves the channels identified by the given symbol slice
`client.LeaveLobby()` - Leaves the lobby channel.

## Configuration

Configuration is done through a configuration object (`intrinio.Config`) that is passed to the `intrinio.New[Equities/Options]Client` routine. You may create a configuration directly, in code, like so:

```go
var config intrinio.Config = intrinio.Config{ApiKey: "YOUR-API-KEY", Provider: "OPRA/DSIP"}
```

 Or, you can create `.json` config files, of the following form, and place them in your application root. An example of this is provided in the sample project.


```json
{
	"ApiKey": "YOUR-API-KEY",
	"Provider": "OPRA/DSIP",
}
```

You can then create your config objects using:

```go
var config intrinio.Config = intrinio.LoadConfig("[options/equities]Config.json")
```
