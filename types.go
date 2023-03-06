package intrinio

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"time"
)

type Provider string

const (
	OPRA   Provider = "OPRA"
	MANUAL Provider = "MANUAL"
)

type Config struct {
	ApiKey    string
	Provider  Provider
	IPAddress string
}

func (config Config) getAuthUrl() string {
	if config.Provider == "OPRA" {
		return ("https://realtime-options.intrinio.com/auth?api_key=" + config.ApiKey)
	} else if config.Provider == "MANUAL" {
		return ("http://" + config.IPAddress + "/auth?api_key=" + config.ApiKey)
	} else {
		panic("Client - Provider not specified in config")
	}
}

func (config Config) getWSUrl(token string) string {
	if config.Provider == "OPRA" {
		return ("wss://realtime-options.intrinio.com/socket/websocket?vsn=1.0.0&token=" + token)
	} else if config.Provider == "MANUAL" {
		return ("ws://" + config.IPAddress + "/socket/websocket?vsn=1.0.0&token=" + token)
	} else {
		panic("Client - Provider not specified in config")
	}
}

func LoadConfig() Config {
	wd, getWdErr := os.Getwd()
	if getWdErr != nil {
		panic(getWdErr)
	}
	filepath := wd + string(os.PathSeparator) + "config.json"
	log.Printf("Client - Loading application configuration from: %s\n", filepath)
	data, readFileErr := os.ReadFile(filepath)
	if readFileErr != nil {
		log.Fatal(readFileErr)
	}
	var config Config
	unmarshalErr := json.Unmarshal(data, &config)
	if unmarshalErr != nil {
		log.Fatal(unmarshalErr)
	}
	if strings.TrimSpace(config.ApiKey) == "" {
		log.Fatal("Client - Config must provide a valid API key")
	}
	if (config.Provider != "OPRA") && (config.Provider != "MANUAL") {
		log.Fatal("Client - Config must specify a valid provider")
	}
	if (config.Provider == "MANUAL") && (strings.TrimSpace(config.IPAddress) == "") {
		log.Fatal("Client - Config must specify an IP address for manual configuration")
	}
	return config
}

var priceTypeDivisorTable [16]float64 = [16]float64{1.0, 10.0, 100.0, 1000.0, 10000.0, 100000.0, 1000000.0, 10000000.0, 100000000.0, 1000000000.0, 512.0, 0.0, 0.0, 0.0, 0.0, math.NaN()}

func extractUInt64Price(priceBytes []byte, priceType uint8) float32 {
	return float32(float64(binary.LittleEndian.Uint64(priceBytes)) / priceTypeDivisorTable[priceType])
}

func extractUInt32Price(priceBytes []byte, priceType uint8) float32 {
	return float32(float64(binary.LittleEndian.Uint32(priceBytes)) / priceTypeDivisorTable[priceType])
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

type Trade struct {
	ContractId                 string
	Price                      float32
	Size                       uint32
	TotalVolume                uint64
	AskPriceAtExecution        float32
	BidPriceAtExecution        float32
	UnderlyingPriceAtExecution float32
	Timestamp                  float64
}

func (trade Trade) GetStrikePrice() float32 {
	whole := uint16(trade.ContractId[13]-'0')*10000 + uint16(trade.ContractId[14]-'0')*1000 + uint16(trade.ContractId[15]-'0')*100 + uint16(trade.ContractId[16]-'0')*10 + uint16(trade.ContractId[17]-'0')
	part := float32(trade.ContractId[18]-'0')*0.1 + float32(trade.ContractId[19]-'0')*0.01 + float32(trade.ContractId[20]-'0')*0.001
	return (float32(whole) + part)
}

func (trade Trade) IsPut() bool {
	return (trade.ContractId[12] == 'P')
}

func (trade Trade) IsCall() bool {
	return (trade.ContractId[12] == 'C')
}

func (trade Trade) GetExpirationDate() time.Time {
	if loadLocationErr != nil {
		log.Printf("Client - Failure to load time location - %v\n", loadLocationErr)
	}
	time, err := time.ParseInLocation(TIME_FORMAT, trade.ContractId[6:12], newYork)
	if err != nil {
		log.Printf("Client - Failure to parse expiration date from: %s - %v\n", trade.ContractId, err)
	}
	return time
}

func (trade Trade) GetUnderlyingSymbol() string {
	return strings.TrimRight(trade.ContractId[0:6], "_")
}

func parseTrade(bytes []byte) Trade {
	return Trade{
		ContractId:                 extractOldContractId(bytes[1:(1 + bytes[0])]),
		Price:                      extractUInt32Price(bytes[25:29], bytes[23]),
		Size:                       binary.LittleEndian.Uint32(bytes[29:33]),
		Timestamp:                  scaleTimestamp(binary.LittleEndian.Uint64(bytes[33:41])),
		TotalVolume:                binary.LittleEndian.Uint64(bytes[41:49]),
		AskPriceAtExecution:        extractUInt32Price(bytes[49:53], bytes[23]),
		BidPriceAtExecution:        extractUInt32Price(bytes[53:57], bytes[23]),
		UnderlyingPriceAtExecution: extractUInt32Price(bytes[57:61], bytes[24]),
	}
}

type Quote struct {
	ContractId string
	AskPrice   float32
	BidPrice   float32
	AskSize    uint32
	BidSize    uint32
	Timestamp  float64
}

func (quote Quote) GetStrikePrice() float32 {
	whole := uint16(quote.ContractId[13]-'0')*10000 + uint16(quote.ContractId[14]-'0')*1000 + uint16(quote.ContractId[15]-'0')*100 + uint16(quote.ContractId[16]-'0')*10 + uint16(quote.ContractId[17]-'0')
	part := float32(quote.ContractId[18]-'0')*0.1 + float32(quote.ContractId[19]-'0')*0.01 + float32(quote.ContractId[20]-'0')*0.001
	return (float32(whole) + part)
}

func (quote Quote) IsPut() bool {
	return (quote.ContractId[12] == 'P')
}

func (quote Quote) IsCall() bool {
	return (quote.ContractId[12] == 'C')
}

func (quote Quote) GetExpirationDate() time.Time {
	if loadLocationErr != nil {
		log.Printf("Client - Failure to load time location - %v\n", loadLocationErr)
	}
	time, err := time.ParseInLocation(TIME_FORMAT, quote.ContractId[6:12], newYork)
	if err != nil {
		log.Printf("Client - Failure to parse expiration date from: %s - %v\n", quote.ContractId, err)
	}
	return time
}

func (quote Quote) GetUnderlyingSymbol() string {
	return strings.TrimRight(quote.ContractId[0:6], "_")
}

func parseQuote(bytes []byte) Quote {
	return Quote{
		ContractId: extractOldContractId(bytes[1:(1 + bytes[0])]),
		AskPrice:   extractUInt32Price(bytes[24:28], bytes[23]),
		AskSize:    binary.LittleEndian.Uint32(bytes[28:32]),
		BidPrice:   extractUInt32Price(bytes[32:36], bytes[23]),
		BidSize:    binary.LittleEndian.Uint32(bytes[36:40]),
		Timestamp:  scaleTimestamp(binary.LittleEndian.Uint64(bytes[40:48])),
	}
}

type Refresh struct {
	ContractId   string
	OpenInterest uint32
	OpenPrice    float32
	ClosePrice   float32
	HighPrice    float32
	LowPrice     float32
}

func (refresh Refresh) GetStrikePrice() float32 {
	whole := uint16(refresh.ContractId[13]-'0')*10000 + uint16(refresh.ContractId[14]-'0')*1000 + uint16(refresh.ContractId[15]-'0')*100 + uint16(refresh.ContractId[16]-'0')*10 + uint16(refresh.ContractId[17]-'0')
	part := float32(refresh.ContractId[18]-'0')*0.1 + float32(refresh.ContractId[19]-'0')*0.01 + float32(refresh.ContractId[20]-'0')*0.001
	return (float32(whole) + part)
}

func (refresh Refresh) IsPut() bool {
	return (refresh.ContractId[12] == 'P')
}

func (refresh Refresh) IsCall() bool {
	return (refresh.ContractId[12] == 'C')
}

func (refresh Refresh) GetExpirationDate() time.Time {
	if loadLocationErr != nil {
		log.Printf("Client - Failure to load time location - %v\n", loadLocationErr)
	}
	time, err := time.ParseInLocation(TIME_FORMAT, refresh.ContractId[6:12], newYork)
	if err != nil {
		log.Printf("Client - Failure to parse expiration date from: %s - %v\n", refresh.ContractId, err)
	}
	return time
}

func (refresh Refresh) GetUnderlyingSymbol() string {
	return strings.TrimRight(refresh.ContractId[0:6], "_")
}

func parseRefresh(bytes []byte) Refresh {
	return Refresh{
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

type UnusualActivity struct {
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

func (ua UnusualActivity) GetStrikePrice() float32 {
	whole := uint16(ua.ContractId[13]-'0')*10000 + uint16(ua.ContractId[14]-'0')*1000 + uint16(ua.ContractId[15]-'0')*100 + uint16(ua.ContractId[16]-'0')*10 + uint16(ua.ContractId[17]-'0')
	part := float32(ua.ContractId[18]-'0')*0.1 + float32(ua.ContractId[19]-'0')*0.01 + float32(ua.ContractId[20]-'0')*0.001
	return (float32(whole) + part)
}

func (ua UnusualActivity) IsPut() bool {
	return (ua.ContractId[12] == 'P')
}

func (ua UnusualActivity) IsCall() bool {
	return (ua.ContractId[12] == 'C')
}

func (ua UnusualActivity) GetExpirationDate() time.Time {
	if loadLocationErr != nil {
		log.Printf("Client - Failure to load time location - %v\n", loadLocationErr)
	}
	time, err := time.ParseInLocation(TIME_FORMAT, ua.ContractId[6:12], newYork)
	if err != nil {
		log.Printf("Client - Failure to parse expiration date from: %s - %v\n", ua.ContractId, err)
	}
	return time
}

func (ua UnusualActivity) GetUnderlyingSymbol() string {
	return strings.TrimRight(ua.ContractId[0:6], "_")
}

func parseUA(bytes []byte) UnusualActivity {
	return UnusualActivity{
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
