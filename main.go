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
		fmt.Println("Main received:", payload)

		fmt.Printf("Received id: %d, download_url: %s, mime: %s\n", payload.ID, payload.URL, payload.MIME)

		fmt.Println("Sending link to Virus Total...")

		url_check(payload.URL)
	}
}
