package main

import (
	"fmt"
	"os"
)

func main() {
	dirPath := "./temp" // Path to the directory to create

	// Create the directory and any necessary parent directories
	err := os.MkdirAll(dirPath, 0755) // 0755 sets the permissions for the new directory
	if err != nil {
		fmt.Printf("Error creating directory: %v\n", err)
		return
	}
	fmt.Printf("Directory '%s' created or already exists.\n", dirPath)

	// create channel to receive data
	dataChan := make(chan DataPayload)

	// run the server in a goroutine
	go receive(dataChan)

	// wait for data to come from handler
	fmt.Println("Waiting for data...")

	for {
		payload := <-dataChan // blocks until data arrives
		fmt.Println("")
		fmt.Println("Main received:", payload)

		fmt.Printf("Received id: %d, download_url: %s, mime: %s\n", payload.ID, payload.URL, payload.MIME)
		fmt.Println("")
	}
}
