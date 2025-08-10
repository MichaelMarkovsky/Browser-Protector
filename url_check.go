package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
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
	ID         string     `json:"id"`
	Links      Links      `json:"links"`
	StatusInfo Attributes `json:"attributes"`
}
type Links struct {
	Self string `json:"self"`
}
type Stats struct {
	Malicious  int `json:"malicious"`
	Suspicious int `json:"suspicious"`
}

type Attributes struct {
	Status string `json:"status"`
	Stats  Stats  `json:"stats"`
}

func mustMkdir(path string) {
	// Create the directory and any necessary parent directories
	if err := os.MkdirAll(path, 0o755); err != nil {
		log.Fatal(err)
	}
}

func url_check(URL string, FILENAME string) bool {
	temp := "./temp" // Path to the directory to create
	uncompressed := "./temp/uncompressed"
	compressed := "./temp/compressed"

	// ensure dirs exist
	mustMkdir(temp)
	mustMkdir(uncompressed)
	mustMkdir(compressed)

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
	// filename := ""
	// // Try to extract the filename from disposition (if a file is meant to be downloaded it most likey has a 'Content-Dosposition' section in the header)
	// disposition, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition"))
	// fmt.Println(disposition)
	// if err != nil {
	// 	// If the file has no disposition section, the file could have the name from the url itself, extracting from the url
	// 	fmt.Println("There is no disposition in the header,\nUsing the name from the url parsed:")

	// 	u, _ := url.Parse(URL)
	// 	cleanPath := u.Path
	// 	filename = path.Base(cleanPath)
	// 	fmt.Println("filename:", filename)

	// } else {
	// 	filename = params["filename"]
	// 	fmt.Println("filename:", filename)
	// }

	filename := FILENAME

	filename_path := ""
	if IsCompressed {
		filename_path = "./temp/compressed/" + filename
	} else {
		filename_path = "./temp/uncompressed/" + filename
	}

	//If we found a name for the file then create and download the file

	if filename != "" {
		// Create output file
		out, err := os.Create(filename_path)
		if err != nil {
			out.Close()
			log.Fatal("File creation failed:", err)
		}

		// Copy response body to file
		n, err := io.Copy(out, resp.Body)
		if err != nil {
			log.Fatal("Copy failed:", err)
		}
		fmt.Printf("Downloaded %d bytes\n", n)

		out.Sync()
		out.Close()
	} else {
		log.Fatal("File name has not been located, download failed.")
	}

	// If the file is of zip type then unzip (if its compressed then uncompress)
	if IsCompressed {
		a, err := unarr.NewArchive(filename_path)
		if err != nil {
			panic(err)
		}

		// ensure destination folder exists
		dest := "./temp/uncompressed"
		err = os.MkdirAll(dest, os.ModePerm)
		if err != nil {
			a.Close()
			panic(err)
		}

		// extract to the uncompressed folder
		_, err = a.Extract(dest)
		if err != nil {
			a.Close()
			panic(err)
		}

		a.Close()

		filename_path = "./temp/uncompressed/"
	}

	// godotenv package
	API_KEY := goDotEnvVariable("API_KEY")

	//===================================== GET ALL FILE PATHS ==========================================
	//Get list of file's path's , to then send and get easily their IDs to then upload, meaning its going to work on nested folders.
	var File_Paths = []string{}

	rootPath := "./temp/uncompressed"

	//walk recursivly in the temp folder to find all the paths for all the files
	err = filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err // Handle any errors encountered during traversal
		}
		if !info.IsDir() { // Check if it's a file (not a directory)
			fmt.Println("File to upload: ", path) // Print the path of the file
			File_Paths = append(File_Paths, path)
		}
		return nil // Continue walking the tree
	})

	if err != nil {
		log.Fatal(err) // Handle errors from filepath.Walk itself
	}

	//===================================== SEND FILE TO VIRUS TOTAL =====================================
	var IDs []string

	client := &http.Client{Timeout: 10 * time.Second}

	// 1 request every 15 seconds , 4/min
	limiter := time.NewTicker(15 * time.Second)
	defer limiter.Stop()

	for _, path := range File_Paths {
		<-limiter.C // wait before sending next request

		fileName := filepath.Base(path)

		// open file for reading
		f, err := os.Open(path)
		if err != nil {
			fmt.Println("error opening file:", err)
			continue
		}

		// build proper multipart form with raw file bytes
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, err := mw.CreateFormFile("file", fileName)
		if err != nil {
			fmt.Println("error creating form file:", err)
			f.Close()
			continue
		}
		if _, err := io.Copy(fw, f); err != nil {
			fmt.Println("error copying file contents:", err)
			f.Close()
			continue
		}
		f.Close()
		mw.Close()

		req, err := http.NewRequest("POST", "https://www.virustotal.com/api/v3/files", &buf)
		if err != nil {
			fmt.Println("error creating request:", err)
			continue
		}

		req.Header.Set("accept", "application/json")
		req.Header.Set("x-apikey", API_KEY)
		req.Header.Set("content-type", mw.FormDataContentType())

		res, err := client.Do(req)
		if err != nil {
			fmt.Println("error sending request:", err)
			continue
		}

		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			fmt.Println("error reading response:", err)
			continue
		}

		// fmt.Println("response body:")
		// fmt.Println(string(body), "\n")

		var obj Object
		if err := json.Unmarshal(body, &obj); err != nil {
			fmt.Println("error decoding json:", err)
			continue
		}

		urlID := obj.Data.Links.Self
		fmt.Println("Analysis url id:", urlID)
		IDs = append(IDs, urlID)
	}

	//=====================================GET Virus Total analysis on the file/folder=====================================
	safe := true

	pollLimiter := time.NewTicker(3 * time.Second) // 1 poll every 3s
	defer pollLimiter.Stop()

	for _, urlID := range IDs { // loop over all collected analysis URLs
		var obj_res Object

		completed := false

		for i := 0; i < 10; i++ {
			<-pollLimiter.C // wait for allowed slot before each GET

			req_p, err := http.NewRequest("GET", urlID, nil)
			if err != nil {
				fmt.Println("error creating request:", err)
				break
			}

			req_p.Header.Add("accept", "application/json")
			req_p.Header.Add("x-apikey", API_KEY)

			res_p, err := client.Do(req_p)
			if err != nil {
				fmt.Println("error sending request:", err)
				break
			}

			body_res, err := io.ReadAll(res_p.Body)
			res_p.Body.Close()
			if err != nil {
				fmt.Println("error reading body:", err)
				break
			}

			// fmt.Println("Analysis body:")
			// fmt.Println(string(body_res), "\n")

			if err := json.Unmarshal(body_res, &obj_res); err != nil {
				fmt.Println("error decoding json:", err)
				break
			}

			if obj_res.Data.StatusInfo.Status == "completed" {
				result_mal := obj_res.Data.StatusInfo.Stats.Malicious
				result_sus := obj_res.Data.StatusInfo.Stats.Suspicious
				fmt.Printf("File %s | Malicious: %d, Suspicious: %d\n", urlID, result_mal, result_sus)
				if result_mal > 0 || result_sus > 0 {
					safe = false
				}
				completed = true
				break
			}

			time.Sleep(5 * time.Second) // wait before next poll
		}
		if !completed {
			fmt.Printf("warning: analysis did not complete for %s, marking as unsafe\n", urlID)
			safe = false
		}
	}

	//================= Delete files ==============
	// Delete the file
	fmt.Println("")

	// list of folders to nuke
	folders := []string{"./temp/compressed", "./temp/uncompressed", "./temp"}

	for _, folder := range folders {
		if err := os.RemoveAll(folder); err != nil {
			log.Fatalf("error deleting %s: %v", folder, err)
		}
		log.Printf("%s folder has been deleted successfully.", folder)
	}
	//=============================================

	if !safe {
		fmt.Println("Download failed, on or more files are not safe.")
		return false
	} else {
		fmt.Println("Download is safe and was successful.")
		return true
	}
}
