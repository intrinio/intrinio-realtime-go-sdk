package intrinio

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
)

const (
	equityTradePriority = iota
	equityQuotePriority
)

type EquityTrade struct {
	Symbol       string
	Source       uint8
	MarketCenter rune
	Price        float64
	Size         uint32
	TotalVolume  uint64
	Timestamp    float64
	Conditions   string
}

func parseEquityTrade(bytes []byte) EquityTrade {
	symbolLen := bytes[2]
	symbol := string(bytes[3 : 3+symbolLen])
	source := bytes[3+symbolLen]
	marketCenter := rune(binary.LittleEndian.Uint16(bytes[4+symbolLen : 6+symbolLen]))
	price := float64(math.Float32frombits(binary.LittleEndian.Uint32(bytes[6+symbolLen : 10+symbolLen])))
	size := binary.LittleEndian.Uint32(bytes[10+symbolLen : 14+symbolLen])
	timestamp := float64(binary.LittleEndian.Uint64(bytes[14+symbolLen:22+symbolLen])) / 1000000000.0
	totalVolume := uint64(binary.LittleEndian.Uint32(bytes[22+symbolLen : 26+symbolLen]))
	conditionsLen := bytes[26+symbolLen]
	conditions := ""
	if conditionsLen > 0 {
		conditions = string(bytes[27+symbolLen : 27+symbolLen+conditionsLen])
	}
	return EquityTrade{
		Symbol:       symbol,
		Source:       source,
		MarketCenter: marketCenter,
		Price:        price,
		Size:         size,
		Timestamp:    timestamp,
		TotalVolume:  totalVolume,
		Conditions:   conditions,
	}
}

type QuoteType uint8

const (
	ASK QuoteType = 1
	BID QuoteType = 2
)

type EquityQuote struct {
	Type         QuoteType
	Symbol       string
	Source       uint8
	MarketCenter rune
	Price        float64
	Size         uint32
	Timestamp    float64
	Conditions   string
}

func parseEquityQuote(bytes []byte) EquityQuote {
	symbolLen := bytes[2]
	symbol := string(bytes[3 : 3+symbolLen])
	source := bytes[3+symbolLen]
	marketCenter := rune(binary.LittleEndian.Uint16(bytes[4+symbolLen : 6+symbolLen]))
	price := float64(math.Float32frombits(binary.LittleEndian.Uint32(bytes[6+symbolLen : 10+symbolLen])))
	size := binary.LittleEndian.Uint32(bytes[10+symbolLen : 14+symbolLen])
	timestamp := float64(binary.LittleEndian.Uint64(bytes[14+symbolLen:22+symbolLen])) / 1000000000.0
	conditionsLen := bytes[22+symbolLen]
	conditions := ""
	if conditionsLen > 0 {
		conditions = string(bytes[23+symbolLen : 23+symbolLen+conditionsLen])
	}
	return EquityQuote{
		Type:         QuoteType(bytes[0]),
		Symbol:       symbol,
		Source:       source,
		MarketCenter: marketCenter,
		Price:        price,
		Size:         size,
		Timestamp:    timestamp,
		Conditions:   conditions,
	}
}

func workOnEquities(
	queue *priorityQueue,
	onTrade func(EquityTrade),
	onQuote func(EquityQuote)) bool {
	data, ok := queue.dequeue()
	if !ok {
		return false
	}

	msgType := data[0]
	if (msgType == 1) || (msgType == 2) {
		quote := parseEquityQuote(data)
		if onQuote != nil {
			onQuote(quote)
		}
	} else if msgType == 0 {
		trade := parseEquityTrade(data)
		if onTrade != nil {
			onTrade(trade)
		}
	} else {
		log.Printf("Equity Client - Invalid message type: %d", msgType)
	}
	return true
}

func queueEquityMessages(
	data []byte,
	queue *priorityQueue,
	processNext func() bool) error {
	if len(data) == 0 {
		return fmt.Errorf("message is empty")
	}

	count := int(data[0])
	startIndex := 1
	for i := 0; i < count; i++ {
		if startIndex+2 > len(data) {
			return fmt.Errorf("event %d is missing its type or length", i)
		}

		messageLength := int(data[startIndex+1])
		if messageLength < 2 || startIndex+messageLength > len(data) {
			return fmt.Errorf("event %d has invalid length %d", i, messageLength)
		}

		message := data[startIndex : startIndex+messageLength]
		var priority int
		switch message[0] {
		case 0:
			priority = equityTradePriority
		case 1, 2:
			priority = equityQuotePriority
		default:
			return fmt.Errorf("event %d has invalid message type %d", i, message[0])
		}

		enqueuePriorityMessage(queue, priority, message, processNext)
		startIndex += messageLength
	}
	return nil
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
