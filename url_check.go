package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

func main() {
	// err - stores any error
	// resp - the response of the get request
	resp, err := http.Get("[URL]")
	if err != nil {
		log.Fatal(err)
	}

	// Check that status code is ok
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Bad status: %s", resp.Status)
	} else {
		fmt.Println("Status code is:", resp.StatusCode)
	}

	// Check content-type (whether its an image or zip file)
	contentType := resp.Header.Get("Content-Type")
	fmt.Println("Content-Type:", contentType)

	//headerContent := resp.Header.Get("Content-Disposition")
	//fmt.Println(resp.Header)

	// Create output file
	out, err := os.Create("exmaple.png")
	if err != nil {
		log.Fatal("File creation failed:", err)
	}
	defer out.Close()

	// Copy response body to file
	n, err := io.Copy(out, resp.Body)
	if err != nil {
		log.Fatal("Copy failed:", err)
	}
	fmt.Printf("Downloaded %d bytes\n", n)
}
