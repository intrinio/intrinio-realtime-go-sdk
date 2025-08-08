package composite

import (
	"time"
)

// QuoteType represents the type of quote
type QuoteType string

const (
	QuoteTypeAsk QuoteType = "ask"
	QuoteTypeBid QuoteType = "bid"
)

// TradeCandleStick represents a candlestick for trades
type TradeCandleStick struct {
	Symbol    string
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    uint64
	Timestamp time.Time
	Interval  string
}

// QuoteCandleStick represents a candlestick for quotes
type QuoteCandleStick struct {
	Symbol    string
	Type      QuoteType
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    uint64
	Timestamp time.Time
	Interval  string
}

// OptionsRefresh represents an options refresh
type OptionsRefresh struct {
	Contract                    string
	Exchange                    string
	Timestamp                   float64
	UnderlyingPrice             float64
	BidPrice                    float64
	AskPrice                    float64
	BidSize                     uint32
	AskSize                     uint32
}

// OptionsUnusualActivity represents unusual options activity
type OptionsUnusualActivity struct {
	Contract                    string
	Exchange                    string
	Type                        string
	Sentiment                   string
	Price                       float64
	Size                        uint32
	Timestamp                   float64
	UnderlyingPrice             float64
}

// OptionsTradeCandleStick represents a candlestick for options trades
type OptionsTradeCandleStick struct {
	Contract    string
	Exchange    string
	Open        float64
	High        float64
	Low         float64
	Close       float64
	Volume      uint64
	Timestamp   float64
	Interval    string
}

// OptionsQuoteCandleStick represents a candlestick for options quotes
type OptionsQuoteCandleStick struct {
	Contract    string
	Exchange    string
	Type        QuoteType
	Open        float64
	High        float64
	Low         float64
	Close       float64
	Volume      uint64
	Timestamp   float64
	Interval    string
}


type DailyMetricResponse struct {
	DailyMetrics []DailyMetric `json:"daily_metrics"` 
}

type DailyMetric struct {
	DividendYield string `json:"dividend_yield"`
	Company CompanySummary `json:"company"`
}

type CompanySummary struct {
	Ticker string `json:"ticker"`
}

