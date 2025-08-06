package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/gen2brain/go-unarr"
)

// use godot package to load/read the .env file and
// return the value of the key
func goDotEnvVariable(key string) string {

	// load .env file
	err := godotenv.Load(".env")

	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	return os.Getenv(key)
}

type Object struct {
	Data Data `json:"data"`
}

type Data struct {
	ID    string `json:"id"`
	Links Links  `json:"links"`
	Stats Stats  `json:"stats"`
}
type Links struct {
	Self string `json:"self"`
}
type Stats struct {
	Malicious  int `json:"malicious"`
	Suspicious int `json:"suspicious"`
}

func url_check(URL string) bool {

	//URL := "https://images.pexels.com/photos/18810025/pexels-photo-18810025.jpeg?cs=srgb&dl=pexels-aleksandra-s-282932122-18810025.jpg&fm=jpg&w=4000&h=6000&_gl=1*8cago4*_ga*OTg5MTIzOTIuMTc1Mzk2NzY2Nw..*_ga_8JE65Q40S6*czE3NTM5Njc2NjckbzEkZzEkdDE3NTM5Njc2NjgkajU5JGwwJGgw"

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

	compressedTypes := []string{"zip", "rar", "tar", "7z"}
	IsCompressed := false
	for _, t := range compressedTypes {
		if strings.Contains(contentType, t) {
			IsCompressed = true
			fmt.Println("Compressed file found.")
			break
		}
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
	if IsCompressed {

		a, err := unarr.NewArchive(filename)
		if err != nil {
			panic(err)
		}
		defer a.Close()

		_, err = a.Extract("./")
		if err != nil {
			panic(err)
		}

	}

	// godotenv package
	API_KEY := goDotEnvVariable("API_KEY")

	//===================================== SEND FILE TO VIRUS TOTAL =====================================
	Vurl := "https://www.virustotal.com/api/v3/files"

	fileBytes, err := os.ReadFile(filename)
	if err != nil {
		panic(err)
	}

	// Encode file to base64
	fileBase64 := base64.StdEncoding.EncodeToString(fileBytes)

	// Define boundary
	boundary := "-----011000010111000001101001"

	// Create multipart form body as string
	payloadStr := fmt.Sprintf(
		"%s\r\nContent-Disposition: form-data; name=\"file\"; filename=\"%s\"\r\nContent-Type: %s\r\n\r\n"+
			"data:%s;name=%s;base64,%s\r\n%s--",
		boundary, filename, contentType, contentType, filename, fileBase64, boundary)

	payload := strings.NewReader(payloadStr)

	Vreq, _ := http.NewRequest("POST", Vurl, payload)

	Vreq.Header.Add("accept", "application/json")
	Vreq.Header.Add("x-apikey", API_KEY)
	Vreq.Header.Add("content-type", "multipart/form-data; boundary=---011000010111000001101001")

	// Creating http timeout of 10 seconds for getting the response
	client := &http.Client{Timeout: 10 * time.Second}
	Vres, err := client.Do(Vreq)
	if err != nil {
		fmt.Println("Error:", err)
	}

	defer Vres.Body.Close()
	// Read the full body into memory
	body, err := io.ReadAll(Vres.Body)
	if err != nil {
		panic(err)
	}
	// Print the raw body
	fmt.Println("Response body:")
	fmt.Println(string(body) + "\n")

	// Decode the JSON from the body bytes and get id
	var obj Object
	if err := json.Unmarshal(body, &obj); err != nil {
		panic(err)
	}

	url_id := obj.Data.Links.Self
	fmt.Println("Analysis url id: " + url_id)

	//=====================================GET Virus Total analysis on the file/folder=====================================

	req_p, err := http.NewRequest("GET", url_id, nil)
	if err != nil {
		panic(err)
	}

	req_p.Header.Add("accept", "application/json")
	req_p.Header.Add("x-apikey", API_KEY)

	res_p, err := client.Do(req_p) // response for result also have timeout of 10 seconds
	if err != nil {
		fmt.Println("Error:", err)
	}

	defer res_p.Body.Close()

	body_res, err := io.ReadAll(res_p.Body)
	if err != nil {
		panic(err)
	}

	fmt.Println("Analysis body:")
	fmt.Println(string(body_res) + "\n")

	// Decode the JSON from the body bytes and get id
	var obj_res Object
	if err := json.Unmarshal(body, &obj); err != nil {
		panic(err)
	}

	result_mal := obj_res.Data.Stats.Malicious
	result_sus := obj_res.Data.Stats.Suspicious
	fmt.Printf("Malicious: %d,Suspicious: %d", result_mal, result_sus)

	if result_mal > 0 || result_sus > 0 {
		fmt.Println("Download failed, file is not safe.")
		return false
	} else {
		return true
	}
}
