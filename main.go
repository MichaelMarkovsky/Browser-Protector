package main

import (
	"fmt"
)

func main() {
	// create channel to receive data
	dataChan := make(chan DataPayload)

	// run the server in a goroutine
	go receive(dataChan)

	// wait for data to come from handler
	fmt.Println("Waiting for data...")

	for {
		payload := <-dataChan // blocks until data arrives
		fmt.Printf("The url %v has been successfully scanned.", payload.URL)

		fmt.Println("")
	}

}
