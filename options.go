package intrinio

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"strings"
	"time"
)

type Exchange uint8

func (e Exchange) String() string {
	switch e {
	case 'A':
		return "NYSE_AMERICAN"
	case 'B':
		return "BOSTON"
	case 'C':
		return "CBOE"
	case 'D':
		return "MIAMI_EMERALD"
	case 'E':
		return "BATS_EDGX"
	case 'H':
		return "ISE_GEMINI"
	case 'I':
		return "ISE"
	case 'J':
		return "MERCURY"
	case 'M':
		return "MIAMI"
	case 'O':
		return "MIAMI_PEARL"
	case 'P':
		return "NYSE_ARCA"
	case '!':
		return "NASDAQ"
	case 'T':
		return "NASDAQ_BX"
	case 'U':
		return "MEMX"
	case 'W':
		return "CBOE_C2"
	case 'X':
		return "PHLX"
	case 'Z':
		return "BATS_BZX"
	}
	return "unknown"
}

const (
	NYSE_AMERICAN Exchange = 'A'
	BOSTON        Exchange = 'B'
	CBOE          Exchange = 'C'
	MIAMI_EMERALD Exchange = 'D'
	BATS_EDGX     Exchange = 'E'
	ISE_GEMINI    Exchange = 'H'
	ISE           Exchange = 'I'
	MERCURY       Exchange = 'J'
	MIAMI         Exchange = 'M'
	MIAMI_PEARL   Exchange = 'O'
	NYSE_ARCA     Exchange = 'P'
	NASDAQ        Exchange = 'Q'
	NASDAQ_BX     Exchange = 'T'
	MEMX          Exchange = 'U'
	CBOE_C2       Exchange = 'W'
	PHLX          Exchange = 'X'
	BATS_BZX      Exchange = 'Z'
)

const (
	MAX_OPTION_SYMBOL_SIZE  int = 21
	OPTION_TRADE_MSG_SIZE   int = 72
	OPTION_QUOTE_MSG_SIZE   int = 52
	OPTION_REFRESH_MSG_SIZE int = 52
	OPTION_UA_MSG_SIZE      int = 74
)

var priceTypeDivisorTable [16]float64 = [16]float64{1.0, 10.0, 100.0, 1000.0, 10000.0, 100000.0, 1000000.0, 10000000.0, 100000000.0, 1000000000.0, 512.0, 0.0, 0.0, 0.0, 0.0, math.NaN()}

func extractUInt64Price(priceBytes []byte, priceType uint8) float64 {
	return float64(binary.LittleEndian.Uint64(priceBytes)) / priceTypeDivisorTable[priceType]
}

func extractUInt32Price(priceBytes []byte, priceType uint8) float64 {
	return float64(binary.LittleEndian.Uint32(priceBytes)) / priceTypeDivisorTable[priceType]
}

func scaleTimestamp(timestamp uint64) float64 {
	return (float64(timestamp) / 1000000000.0)
}

func convertOldContractIdToNew(oldContractId string) string {
	if (len(oldContractId) < 13) || (strings.IndexByte(oldContractId, byte('.')) > 9) {
		return oldContractId
	}
	symbol := strings.TrimRight(oldContractId[0:6], "_")
	exp := oldContractId[6:12]
	pc := oldContractId[12]
	var whole string
	if whole = strings.TrimLeft(oldContractId[13:18], "0"); whole == "" {
		whole = "0"
	}
	var part string
	if part = oldContractId[18:]; part[2] == '0' {
		part = part[0:2]
	}
	return fmt.Sprintf(`%s_%s%c%s.%s`, symbol, exp, pc, whole, part)
}

func extractOldContractId(newContractBytes []byte) string {
	oldContractBytes := [21]byte{'_', '_', '_', '_', '_', '_', '0', '0', '0', '0', '0', '0', 'X', '0', '0', '0', '0', '0', '0', '0', '0'}
	i := 0
	j := 0
	for ; newContractBytes[i] != '_'; i++ {
		oldContractBytes[j] = newContractBytes[i]
		j++
	}
	i++
	for j = 6; j < 13; j++ {
		oldContractBytes[j] = newContractBytes[i]
		i++
	}
	indexOfPC := i - 1
	for i = len(newContractBytes) - 2; newContractBytes[i] != '.'; i-- {
	}
	indexOfDecimal := i
	j = 17
	for i--; i > indexOfPC; i-- {
		oldContractBytes[j] = newContractBytes[i]
		j--
	}
	j = 18
	for i = indexOfDecimal + 1; i < len(newContractBytes)-1; i++ {
		oldContractBytes[j] = newContractBytes[i]
		j++
	}
	return string(oldContractBytes[:])
}

const TIME_FORMAT string = "060102"

var newYork, loadLocationErr = time.LoadLocation("America/New_York")

type OptionTrade struct {
	ContractId                 string
	Exchange                   Exchange
	Price                      float64
	Size                       uint32
	Qualifiers                 [4]byte
	TotalVolume                uint64
	AskPriceAtExecution        float64
	BidPriceAtExecution        float64
	UnderlyingPriceAtExecution float64
	Timestamp                  float64
}

func (trade OptionTrade) GetStrikePrice() float64 {
	whole := uint32(trade.ContractId[13]-'0')*10000 + uint32(trade.ContractId[14]-'0')*1000 + uint32(trade.ContractId[15]-'0')*100 + uint32(trade.ContractId[16]-'0')*10 + uint32(trade.ContractId[17]-'0')
	part := float64(trade.ContractId[18]-'0')*0.1 + float64(trade.ContractId[19]-'0')*0.01 + float64(trade.ContractId[20]-'0')*0.001
	return (float32(whole) + part)
}

func (trade OptionTrade) IsPut() bool {
	return (trade.ContractId[12] == 'P')
}

func (trade OptionTrade) IsCall() bool {
	return (trade.ContractId[12] == 'C')
}

func (trade OptionTrade) GetExpirationDate() time.Time {
	if loadLocationErr != nil {
		log.Printf("Client - Failure to load time location - %v\n", loadLocationErr)
	}
	time, err := time.ParseInLocation(TIME_FORMAT, trade.ContractId[6:12], newYork)
	if err != nil {
		log.Printf("Client - Failure to parse expiration date from: %s - %v\n", trade.ContractId, err)
	}
	return time
}

func (trade OptionTrade) GetUnderlyingSymbol() string {
	return strings.TrimRight(trade.ContractId[0:6], "_")
}

func parseOptionTrade(bytes []byte) OptionTrade {
	return OptionTrade{
		ContractId:                 extractOldContractId(bytes[1:(1 + bytes[0])]),
		Price:                      extractUInt32Price(bytes[25:29], bytes[23]),
		Size:                       binary.LittleEndian.Uint32(bytes[29:33]),
		Timestamp:                  scaleTimestamp(binary.LittleEndian.Uint64(bytes[33:41])),
		TotalVolume:                binary.LittleEndian.Uint64(bytes[41:49]),
		AskPriceAtExecution:        extractUInt32Price(bytes[49:53], bytes[23]),
		BidPriceAtExecution:        extractUInt32Price(bytes[53:57], bytes[23]),
		UnderlyingPriceAtExecution: extractUInt32Price(bytes[57:61], bytes[24]),
		Qualifiers:                 [4]byte(bytes[61:65]),
		Exchange:                   Exchange(bytes[65]),
	}
}

type OptionQuote struct {
	ContractId string
	AskPrice   float32
	BidPrice   float32
	AskSize    uint32
	BidSize    uint32
	Timestamp  float64
}

func (quote OptionQuote) GetStrikePrice() float32 {
	whole := uint16(quote.ContractId[13]-'0')*10000 + uint16(quote.ContractId[14]-'0')*1000 + uint16(quote.ContractId[15]-'0')*100 + uint16(quote.ContractId[16]-'0')*10 + uint16(quote.ContractId[17]-'0')
	part := float32(quote.ContractId[18]-'0')*0.1 + float32(quote.ContractId[19]-'0')*0.01 + float32(quote.ContractId[20]-'0')*0.001
	return (float32(whole) + part)
}

func (quote OptionQuote) IsPut() bool {
	return (quote.ContractId[12] == 'P')
}

func (quote OptionQuote) IsCall() bool {
	return (quote.ContractId[12] == 'C')
}

func (quote OptionQuote) GetExpirationDate() time.Time {
	if loadLocationErr != nil {
		log.Printf("Client - Failure to load time location - %v\n", loadLocationErr)
	}
	time, err := time.ParseInLocation(TIME_FORMAT, quote.ContractId[6:12], newYork)
	if err != nil {
		log.Printf("Client - Failure to parse expiration date from: %s - %v\n", quote.ContractId, err)
	}
	return time
}

func (quote OptionQuote) GetUnderlyingSymbol() string {
	return strings.TrimRight(quote.ContractId[0:6], "_")
}

func parseOptionQuote(bytes []byte) OptionQuote {
	return OptionQuote{
		ContractId: extractOldContractId(bytes[1:(1 + bytes[0])]),
		AskPrice:   extractUInt32Price(bytes[24:28], bytes[23]),
		AskSize:    binary.LittleEndian.Uint32(bytes[28:32]),
		BidPrice:   extractUInt32Price(bytes[32:36], bytes[23]),
		BidSize:    binary.LittleEndian.Uint32(bytes[36:40]),
		Timestamp:  scaleTimestamp(binary.LittleEndian.Uint64(bytes[40:48])),
	}
}

type OptionRefresh struct {
	ContractId   string
	OpenInterest uint32
	OpenPrice    float32
	ClosePrice   float32
	HighPrice    float32
	LowPrice     float32
}

func (refresh OptionRefresh) GetStrikePrice() float32 {
	whole := uint16(refresh.ContractId[13]-'0')*10000 + uint16(refresh.ContractId[14]-'0')*1000 + uint16(refresh.ContractId[15]-'0')*100 + uint16(refresh.ContractId[16]-'0')*10 + uint16(refresh.ContractId[17]-'0')
	part := float32(refresh.ContractId[18]-'0')*0.1 + float32(refresh.ContractId[19]-'0')*0.01 + float32(refresh.ContractId[20]-'0')*0.001
	return (float32(whole) + part)
}

func (refresh OptionRefresh) IsPut() bool {
	return (refresh.ContractId[12] == 'P')
}

func (refresh OptionRefresh) IsCall() bool {
	return (refresh.ContractId[12] == 'C')
}

func (refresh OptionRefresh) GetExpirationDate() time.Time {
	if loadLocationErr != nil {
		log.Printf("Client - Failure to load time location - %v\n", loadLocationErr)
	}
	time, err := time.ParseInLocation(TIME_FORMAT, refresh.ContractId[6:12], newYork)
	if err != nil {
		log.Printf("Client - Failure to parse expiration date from: %s - %v\n", refresh.ContractId, err)
	}
	return time
}

func (refresh OptionRefresh) GetUnderlyingSymbol() string {
	return strings.TrimRight(refresh.ContractId[0:6], "_")
}

func parseOptionRefresh(bytes []byte) OptionRefresh {
	return OptionRefresh{
		ContractId:   extractOldContractId(bytes[1:(1 + bytes[0])]),
		OpenInterest: binary.LittleEndian.Uint32(bytes[24:28]),
		OpenPrice:    extractUInt32Price(bytes[28:32], bytes[23]),
		ClosePrice:   extractUInt32Price(bytes[32:36], bytes[23]),
		HighPrice:    extractUInt32Price(bytes[36:40], bytes[23]),
		LowPrice:     extractUInt32Price(bytes[40:44], bytes[23]),
	}
}

type UAType uint8

const (
	BLOCK         UAType = 3
	SWEEP         UAType = 4
	LARGE         UAType = 5
	UNUSUAL_SWEEP UAType = 6
)

type UASentiment uint8

const (
	NEUTRAL UASentiment = 0
	BULLISH UASentiment = 1
	BEARISH UASentiment = 2
)

type OptionUnusualActivity struct {
	ContractId                 string
	Type                       UAType
	Sentiment                  UASentiment
	TotalValue                 float32
	TotalSize                  uint32
	AveragePrice               float32
	AskPriceAtExecution        float32
	BidPriceAtExecution        float32
	UnderlyingPriceAtExecution float32
	Timestamp                  float64
}

func (ua OptionUnusualActivity) GetStrikePrice() float32 {
	whole := uint16(ua.ContractId[13]-'0')*10000 + uint16(ua.ContractId[14]-'0')*1000 + uint16(ua.ContractId[15]-'0')*100 + uint16(ua.ContractId[16]-'0')*10 + uint16(ua.ContractId[17]-'0')
	part := float32(ua.ContractId[18]-'0')*0.1 + float32(ua.ContractId[19]-'0')*0.01 + float32(ua.ContractId[20]-'0')*0.001
	return (float32(whole) + part)
}

func (ua OptionUnusualActivity) IsPut() bool {
	return (ua.ContractId[12] == 'P')
}

func (ua OptionUnusualActivity) IsCall() bool {
	return (ua.ContractId[12] == 'C')
}

func (ua OptionUnusualActivity) GetExpirationDate() time.Time {
	if loadLocationErr != nil {
		log.Printf("Client - Failure to load time location - %v\n", loadLocationErr)
	}
	time, err := time.ParseInLocation(TIME_FORMAT, ua.ContractId[6:12], newYork)
	if err != nil {
		log.Printf("Client - Failure to parse expiration date from: %s - %v\n", ua.ContractId, err)
	}
	return time
}

func (ua OptionUnusualActivity) GetUnderlyingSymbol() string {
	return strings.TrimRight(ua.ContractId[0:6], "_")
}

func parseOptionUA(bytes []byte) OptionUnusualActivity {
	return OptionUnusualActivity{
		ContractId:                 extractOldContractId(bytes[1:(1 + bytes[0])]),
		Type:                       UAType(bytes[22]),
		Sentiment:                  UASentiment(bytes[23]),
		TotalValue:                 extractUInt64Price(bytes[26:34], bytes[24]),
		TotalSize:                  binary.LittleEndian.Uint32(bytes[34:38]),
		AveragePrice:               extractUInt32Price(bytes[38:42], bytes[25]),
		AskPriceAtExecution:        extractUInt32Price(bytes[42:46], bytes[24]),
		BidPriceAtExecution:        extractUInt32Price(bytes[46:50], bytes[24]),
		UnderlyingPriceAtExecution: extractUInt32Price(bytes[50:54], bytes[25]),
		Timestamp:                  scaleTimestamp(binary.LittleEndian.Uint64(bytes[54:62])),
	}
}

func workOnOptions(
	readChannel <-chan []byte,
	onTrade func(OptionTrade),
	onQuote func(OptionQuote),
	onRefresh func(OptionRefresh),
	onUA func(OptionUnusualActivity)) {
	select {
	case data := <-readChannel:
		count := data[0]
		startIndex := 1
		for i := 0; i < int(count); i++ {
			msgType := data[startIndex+1+MAX_OPTION_SYMBOL_SIZE]
			if msgType == 1 {
				quote := parseOptionQuote(data[startIndex:(startIndex + OPTION_QUOTE_MSG_SIZE)])
				startIndex = startIndex + OPTION_QUOTE_MSG_SIZE
				if onQuote != nil {
					onQuote(quote)
				}
			} else if msgType == 0 {
				trade := parseOptionTrade(data[startIndex:(startIndex + OPTION_TRADE_MSG_SIZE)])
				startIndex = startIndex + OPTION_TRADE_MSG_SIZE
				if onTrade != nil {
					onTrade(trade)
				}
			} else if msgType > 2 {
				ua := parseOptionUA(data[startIndex:(startIndex + OPTION_UA_MSG_SIZE)])
				startIndex = startIndex + OPTION_UA_MSG_SIZE
				if onUA != nil {
					onUA(ua)
				}
			} else if msgType == 2 {
				refresh := parseOptionRefresh(data[startIndex:(startIndex + OPTION_REFRESH_MSG_SIZE)])
				startIndex = startIndex + OPTION_REFRESH_MSG_SIZE
				if onRefresh != nil {
					onRefresh(refresh)
				}
			} else {
				log.Printf("Option Client - Invalid message type: %d", msgType)
			}
		}
	default:
	}
}

func composeOptionJoinMsg(
	useTrade bool,
	useQuote bool,
	useRefresh bool,
	useUA bool,
	symbol string) []byte {
	newSymbol := convertOldContractIdToNew(symbol)
	var mask uint8 = 0
	if useTrade {
		mask = mask | 1
	}
	if useQuote {
		mask = mask | 2
	}
	if useRefresh {
		mask = mask | 4
	}
	if useUA {
		mask = mask | 8
	}
	message := make([]byte, 0, len(newSymbol)+2)
	message = append(message, 74, mask)
	message = append(message, []byte(newSymbol)...)
	log.Printf("Option Client - Composed join msg for channel %s\n", newSymbol)
	return message
}

func composeOptionLeaveMsg(symbol string) []byte {
	newSymbol := convertOldContractIdToNew(symbol)
	message := make([]byte, 0, len(newSymbol)+2)
	message = append(message, 76, 0)
	message = append(message, []byte(newSymbol)...)
	log.Printf("Option Client - Composed leave msg for channel %s\n", newSymbol)
	return message
}
