package intrinio

import (
	"encoding/json"
	"log"
	"os"
	"strings"
)

type Provider string

const (
	OPRA         Provider = "OPRA"
	IEX          Provider = "IEX"
	DELAYED_SIP  Provider = "DELAYED_SIP"
	NASDAQ_BASIC Provider = "NASDAQ_BASIC"
	CBOE_ONE     Provider = "CBOE_ONE"
	MANUAL       Provider = "MANUAL"
)

type Config struct {
	ApiKey    string
	Provider  Provider
	IPAddress string
}

func (config Config) getAuthUrl() string {
	if config.Provider == "OPRA" {
		return ("https://realtime-options.intrinio.com/auth?api_key=" + config.ApiKey)
	} else if config.Provider == "DELAYED_SIP" {
		return ("https://realtime-delayed-sip.intrinio.com/auth?api_key=" + config.ApiKey)
	} else if config.Provider == "NASDAQ_BASIC" {
		return ("https://realtime-nasdaq-basic.intrinio.com/auth?api_key=" + config.ApiKey)
	} else if config.Provider == "CBOE_ONE" {
		return ("https://cboe-one.intrinio.com/auth?api_key=" + config.ApiKey)
	} else if config.Provider == "IEX" {
		return ("https://realtime-mx.intrinio.com/auth?api_key=" + config.ApiKey)
	} else if config.Provider == "MANUAL" {
		return ("http://" + config.IPAddress + "/auth?api_key=" + config.ApiKey)
	} else {
		panic("Client - Provider not specified in config")
	}
}

func (config Config) getWSUrl(token string) string {
	if config.Provider == "OPRA" {
		return ("wss://realtime-options.intrinio.com/socket/websocket?vsn=1.0.0&token=" + token)
	} else if config.Provider == "DELAYED_SIP" {
		return ("wss://realtime-delayed-sip.intrinio.com/socket/websocket?vsn=1.0.0&token=" + token)
	} else if config.Provider == "NASDAQ_BASIC" {
		return ("wss://realtime-nasdaq-basic.intrinio.com/socket/websocket?vsn=1.0.0&token=" + token)
	} else if config.Provider == "CBOE_ONE" {
		return ("wss://cboe-one.intrinio.com/socket/websocket?vsn=1.0.0&token=" + token)
	} else if config.Provider == "IEX" {
		return ("wss://realtime-mx.intrinio.com/socket/websocket?vsn=1.0.0&token=" + token)
	} else if config.Provider == "MANUAL" {
		return ("ws://" + config.IPAddress + "/socket/websocket?vsn=1.0.0&token=" + token)
	} else {
		panic("Client - Provider not specified in config")
	}
}

func LoadConfig(filename string) Config {
	wd, getWdErr := os.Getwd()
	if getWdErr != nil {
		panic(getWdErr)
	}
	filepath := wd + string(os.PathSeparator) + filename
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
		config.ApiKey = os.Getenv("INTRINIO_API_KEY")
		if strings.TrimSpace(config.ApiKey) == "" {
			log.Fatal("Client - A valid API key must be provided (either via the config file or the INTRINIO_API_KEY env variable)")
		}
	}
	if (config.Provider != "OPRA") &&
		(config.Provider != "DELAYED_SIP") &&
		(config.Provider != "NASDAQ_BASIC") &&
		(config.Provider != "IEX") &&
		(config.Provider != "CBOE_ONE") &&
		(config.Provider != "MANUAL") {
		log.Fatal("Client - Config must specify a valid provider")
	}
	if (config.Provider == "MANUAL") && (strings.TrimSpace(config.IPAddress) == "") {
		log.Fatal("Client - Config must specify an IP address for manual configuration")
	}
	return config
}
