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
	//oClient := runOptionsExample()
	gClient := NewGreekSampleApp()
	err := gClient.runGreekExample()
	if err != nil {
		log.Printf("Error running Greek Example: %v", err)
		return
	}

	<-close

	log.Println("EXAMPLE - Closing")
	//oClient.Stop()
	//eClient.Stop()
	gClient.Stop()
}
