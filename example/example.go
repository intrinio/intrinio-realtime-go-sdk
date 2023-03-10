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
	//client := runOptionsExample()
	client := runEquitiesExample()
	<-close
	log.Println("EXAMPLE - Closing")
	client.Stop()
}
