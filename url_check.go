package main

import (
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
)

func main() {

	URL := "https://github.com/MichaelMarkovsky/Browser-Protector/archive/refs/heads/dev/url_check.zip"

	// err - stores any error
	// resp - the response of the get request
	resp, err := http.Get(URL)
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

	// Get the file name
	filename := ""
	// Try to extract the filename from disposition (if a file is meant to be downloaded it most likey has a 'Content-Dosposition' section in the header)
	disposition, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition"))
	fmt.Println(disposition)
	if err != nil {
		// If the file has no disposition section, the file could have the name from the url itself, extracting from the url
		fmt.Println("There is no disposition in the header,\nUsing the name from the url parsed:")

		u, _ := url.Parse(URL)
		cleanPath := u.Path
		filename = path.Base(cleanPath)
		fmt.Println("filename:", filename)

	} else {
		filename = params["filename"]
		fmt.Println("filename:", filename)
	}

	//If we found a name for the file then create and download the file
	if filename != "" {
		// Create output file
		out, err := os.Create(filename)
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
	} else {
		log.Fatal("File name has not been located, download failed.")
	}

}
