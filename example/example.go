package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log.Println("EXAMPLE - Starting")
	close := make(chan os.Signal, 1)
	signal.Notify(close, syscall.SIGINT, syscall.SIGTERM)
	//eClient := runEquitiesExample()
	oClient := runOptionsExample()
	<-close
	log.Println("EXAMPLE - Closing")
	oClient.Stop()
	//eClient.Stop()
}
