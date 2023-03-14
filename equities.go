package intrinio

import (
	"encoding/binary"
	"log"
	"math"
)

const (
	EQUITY_TRADE_MSG_SIZE int = 22
	EQUITY_QUOTE_MSG_SIZE int = 18
)

type EquityTrade struct {
	Symbol      string
	Price       float32
	Size        uint32
	TotalVolume uint32
	Timestamp   float64
}

func parseEquityTrade(bytes []byte, symbolLen int) EquityTrade {
	return EquityTrade{
		Symbol:      string(bytes[2 : 2+symbolLen]),
		Price:       math.Float32frombits(binary.LittleEndian.Uint32(bytes[2+symbolLen : 6+symbolLen])),
		Size:        binary.LittleEndian.Uint32(bytes[6+symbolLen : 10+symbolLen]),
		Timestamp:   float64(binary.LittleEndian.Uint64(bytes[10+symbolLen:18+symbolLen])) / 1000000000.0,
		TotalVolume: binary.LittleEndian.Uint32(bytes[18+symbolLen : 22+symbolLen]),
	}
}

type QuoteType uint8

const (
	ASK QuoteType = 1
	BID QuoteType = 2
)

type EquityQuote struct {
	Type      QuoteType
	Symbol    string
	Price     float32
	Size      uint32
	Timestamp float64
}

func parseEquityQuote(bytes []byte, symbolLen int) EquityQuote {
	return EquityQuote{
		Type:      QuoteType(bytes[0]),
		Symbol:    string(bytes[2 : 2+symbolLen]),
		Price:     math.Float32frombits(binary.LittleEndian.Uint32(bytes[2+symbolLen : 6+symbolLen])),
		Size:      binary.LittleEndian.Uint32(bytes[6+symbolLen : 10+symbolLen]),
		Timestamp: float64(binary.LittleEndian.Uint64(bytes[10+symbolLen:18+symbolLen])) / 1000000000.0,
	}
}

func workOnEquities(
	readChannel <-chan []byte,
	onTrade func(EquityTrade),
	onQuote func(EquityQuote)) {
	select {
	case data := <-readChannel:
		count := data[0]
		startIndex := 1
		for i := 0; i < int(count); i++ {
			msgType := data[startIndex]
			symbolLen := int(data[startIndex+1])
			if (msgType == 1) || (msgType == 2) {
				endIndex := startIndex + symbolLen + EQUITY_QUOTE_MSG_SIZE
				quote := parseEquityQuote(data[startIndex:endIndex], symbolLen)
				startIndex = endIndex
				if onQuote != nil {
					onQuote(quote)
				}
			} else if msgType == 0 {
				endIndex := startIndex + symbolLen + EQUITY_TRADE_MSG_SIZE
				trade := parseEquityTrade(data[startIndex:endIndex], symbolLen)
				startIndex = endIndex
				if onTrade != nil {
					onTrade(trade)
				}
			} else {
				log.Printf("Equity Client - Invalid message type: %d", msgType)
			}
		}
	default:
	}
}

func composeEquityJoinMsg(
	useTrade bool,
	useQuote bool,
	symbol string) []byte {
	var tradesOnly uint8 = 0
	if !useQuote {
		tradesOnly = 1
	}
	message := make([]byte, 0, 11)
	message = append(message, 74, tradesOnly)
	message = append(message, []byte(symbol)...)
	log.Printf("Equity Client - Composed join msg for channel %s\n", symbol)
	return message
}

func composeEquityLeaveMsg(symbol string) []byte {
	message := make([]byte, 0, 10)
	message = append(message, 76)
	message = append(message, []byte(symbol)...)
	log.Printf("Equity Client - Composed leave msg for channel %s\n", symbol)
	return message
}
