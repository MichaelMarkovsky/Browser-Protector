package main

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
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

	IsZip := false
	if strings.Contains(contentType, "zip") {
		IsZip = true
		fmt.Println("Zip file found.")
	}

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

	// If the file is of zip type then unzip (if its compressed then uncompress)
	if IsZip {

		// Open a zip archive for reading.
		r, err := zip.OpenReader(filename)
		if err != nil {
			log.Fatalf("impossible to open zip reader: %s", err)
		}
		defer r.Close()

		// Iterate through the files in the archive,
		for k, f := range r.File {
			fmt.Printf("Unzipping %s:\n", f.Name)
			rc, err := f.Open()
			if err != nil {
				log.Fatalf("impossible to open file n°%d in archine: %s", k, err)
			}
			defer rc.Close()
			// define the new file path
			newFilePath := fmt.Sprintf("uncompressed/%s", f.Name)

			// CASE 1 : we have a directory
			if f.FileInfo().IsDir() {
				// if we have a directory we have to create it
				err = os.MkdirAll(newFilePath, 0777)
				if err != nil {
					log.Fatalf("impossible to MkdirAll: %s", err)
				}
				// we can go to next iteration
				continue
			}

			// CASE 2 : we have a file
			// create new uncompressed file
			uncompressedFile, err := os.Create(newFilePath)
			if err != nil {
				log.Fatalf("impossible to create uncompressed: %s", err)
			}
			_, err = io.Copy(uncompressedFile, rc)
			if err != nil {
				log.Fatalf("impossible to copy file n°%d: %s", k, err)
			}
		}

	}

}
