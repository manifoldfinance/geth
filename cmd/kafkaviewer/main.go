package main

import (
	"fmt"
	"github.com/ethereum/go-ethereum/ethdb/cdc"
	"log"
	"os"
	"os/signal"
)

func main() {
	consumer, err := cdc.NewKafkaLogConsumerFromURL(os.Args[1], "geth", 0)
	if err != nil {
		log.Fatalf("%v", err.Error())
	}
	c := make(chan os.Signal, 1)
	messages := consumer.Messages()
	signal.Notify(c, os.Interrupt)
	for {
		select {
		case op := <-messages:
			log.Printf("%v\n", op)
		case _ = <-c:
			break
		}
		fmt.Scanln()
	}
}
