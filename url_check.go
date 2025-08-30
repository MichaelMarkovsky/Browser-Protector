// url_check.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime" // for ParseMediaType (Content-Disposition)
	"mime/multipart"
	"net/http"
	neturl "net/url" // for decoding RFC5987 filename*
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

// url_check now returns (isSafe, proxyURL). proxyURL is a local one-shot URL if safe.
func url_check(URL string, FILENAME string, MIME string) (bool, string) {
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
	defer resp.Body.Close()

	// Check that status code is ok
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Bad status: %s", resp.Status)
	} else {
		fmt.Println("Status code is:", resp.StatusCode)
	}

	// Check content-type (whether its an image or zip file)
	// contentType := resp.Header.Get("Content-Type")
	contentType := MIME
	fmt.Println("Content-Type:", contentType)

	compressedTypes := []string{"zip", "rar", "tar", "7z"}
	IsCompressed := false
	for _, t := range compressedTypes {
		if strings.Contains(strings.ToLower(contentType), t) {
			IsCompressed = true
			fmt.Println("Compressed file found.")
			break
		}
	}

	// --------------------------------------------------------------------------------
	// 		 Try to extract a clean filename from Content-Disposition first,
	//       then fall back to the provided FILENAME, then to URL path.
	// --------------------------------------------------------------------------------
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if disp, params, err := mime.ParseMediaType(cd); err == nil && (strings.EqualFold(disp, "attachment") || strings.EqualFold(disp, "inline")) {
			// RFC 6266 - filename takes precedence if present
			if fn, ok := params["filename"]; ok && strings.TrimSpace(fn) != "" {
				FILENAME = fn
			} else if fnStar, ok := params["filename*"]; ok && strings.TrimSpace(fnStar) != "" {
				// RFC 5987: filename*=UTF-8''percent-encoded
				FILENAME = decodeRFC5987(fnStar)
			}
		}
	}

	// If still empty, try to use the path segment from the URL
	if strings.TrimSpace(FILENAME) == "" {
		if u, err := neturl.Parse(URL); err == nil {
			base := filepath.Base(u.Path)
			if base != "" && base != "." && base != "/" {
				FILENAME = base
			}
		}
	}

	// Sanitize filename (remove any path components)
	if FILENAME != "" {
		FILENAME = filepath.Base(FILENAME)
		// make sure it's not empty/just dots after sanitization
		if FILENAME == "." || FILENAME == ".." || FILENAME == "" {
			FILENAME = ""
		}
	}

	// If still empty, fall back to a generic name with MIME-derived extension
	if FILENAME == "" {
		ext := guessExtFromMIME(MIME)
		FILENAME = fmt.Sprintf("download_%d%s", time.Now().UnixMilli(), ext)
	}
	// --------------------------------------------------------------------------------

	// filename is now decided
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
			// out can be nil if Create fails
			log.Fatal("File creation failed:", err)
		}

		// Copy response body to file
		n, err := io.Copy(out, resp.Body)
		if err != nil {
			out.Close()
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

		// NOTE: scanning the extracted content
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

		for i := 0; i < 15; i++ { //  15 iterations (45 seconds)
			<-pollLimiter.C // wait for allowed slot before each GET

			req_p, err := http.NewRequest("GET", urlID, nil)
			if err != nil {
				fmt.Printf("error creating analysis request for %s: %v\n", urlID, err)
				break
			}

			req_p.Header.Add("accept", "application/json")
			req_p.Header.Add("x-apikey", API_KEY)

			res_p, err := client.Do(req_p)
			if err != nil {
				fmt.Printf("error sending analysis request for %s: %v\n", urlID, err)
				break
			}

			body_res, err := io.ReadAll(res_p.Body)
			res_p.Body.Close()
			if err != nil {
				fmt.Printf("error reading analysis response for %s: %v\n", urlID, err)
				break
			}

			if err := json.Unmarshal(body_res, &obj_res); err != nil {
				fmt.Printf("error decoding analysis json for %s: %v\n", urlID, err)
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

			fmt.Printf("Analysis not completed for %s, status: %s, iteration: %d\n", urlID, obj_res.Data.StatusInfo.Status, i+1)
			time.Sleep(5 * time.Second) // wait before next poll
		}
		if !completed {
			fmt.Printf("warning: analysis did not complete for %s after 15 iterations, marking as unsafe\n", urlID)
			safe = false
		}
	}

	//================= Return or delete files ==============
	fmt.Println("")

	if !safe {
		// unsafe -> nuke everything
		nukeTemp()
		fmt.Println("Download failed, one or more files are not safe.")
		return false, ""
	}

	// if safe then, provide a one-shot proxy URL for the original file we downloaded
	// serve the original downloaded file path when not compressed,
	// and the archive when compressed.
	var servePath string
	if IsCompressed {
		servePath = "./temp/compressed/" + FILENAME
	} else {
		servePath = "./temp/uncompressed/" + FILENAME
	}

	// Register a one-shot token
	token := fmt.Sprintf("%d_%s", time.Now().UnixNano(), filepath.Base(servePath))
	safeFiles.mu.Lock()
	safeFiles.m[token] = servePath
	safeFiles.mu.Unlock()

	proxyURL := "http://localhost:8080/safe/" + token

	fmt.Println("Download is safe and was successful.")
	return true, proxyURL
}

// ----------------- helpers for cleanup and names -----------------

func nukeTemp() {
	// list of folders to nuke
	folders := []string{"./temp/compressed", "./temp/uncompressed", "./temp"}

	for _, folder := range folders {
		if err := os.RemoveAll(folder); err != nil {
			log.Printf("warning: error deleting %s: %v", folder, err)
		} else {
			log.Printf("%s folder has been deleted successfully.", folder)
		}
	}
}

func safeRemove(p string) error {
	if p == "" {
		return nil
	}
	if err := os.Remove(p); err != nil {
		// not fatal
		return err
	}
	return nil
}

func fileBase(p string) string {
	return filepath.Base(p)
}

func cleanEmpties(leaf string) {
	dir := filepath.Dir(leaf)
	// delete empty dirs up to ./temp
	for i := 0; i < 5; i++ {
		if dir == "." || dir == "/" || dir == "C:\\" {
			return
		}
		_ = os.Remove(dir) // will only remove if empty
		if filepath.Base(dir) == "temp" {
			return
		}
		dir = filepath.Dir(dir)
	}
}

// decodeRFC5987 decodes the filename*= value per RFC 5987 (very common on S3/CDNs)
func decodeRFC5987(v string) string {
	// expected form: UTF-8''percent-encoded
	parts := strings.SplitN(v, "''", 2)
	if len(parts) == 2 {
		if dec, err := neturl.QueryUnescape(parts[1]); err == nil && dec != "" {
			return dec
		}
	}
	// fallback: try plain percent-decoding
	if dec, err := neturl.QueryUnescape(v); err == nil {
		return dec
	}
	return v
}

// best-effort extension guess for when filename is unknown
func guessExtFromMIME(m string) string {
	m = strings.ToLower(strings.TrimSpace(m))
	switch m {
	case "application/pdf":
		return ".pdf"
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "text/plain":
		return ".txt"
	case "application/zip":
		return ".zip"
	case "video/mp4":
		return ".mp4"
	case "audio/mpeg":
		return ".mp3"
	default:
		// try to map "audio/" or "image/" generically
		if strings.HasPrefix(m, "audio/") {
			return ".mp3"
		}
		if strings.HasPrefix(m, "image/") {
			return ".bin"
		}
		return ".bin"
	}
}
