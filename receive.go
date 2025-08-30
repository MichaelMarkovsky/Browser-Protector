// receive.go
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
)

// file path to serve once
var safeFiles = struct {
	m  map[string]string
	mu sync.Mutex
}{m: make(map[string]string)}

type DataPayload struct {
	ID       int    `json:"id"`
	URL      string `json:"url"`
	FILENAME string `json:"filename"`
	MIME     string `json:"mime"`
}

var dataChan chan DataPayload

func handler(w http.ResponseWriter, r *http.Request) {
	// Allow requests from other origins (CORS)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// For preflight OPTIONS requests
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Only POST requests are allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload DataPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		fmt.Printf("Error decoding payload: %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Printf("Checking URL with VirusTotal: %s, Filename: %s, MIME: %s\n", payload.URL, payload.FILENAME, payload.MIME)

	// url_check returns (isSafe, proxyURL)
	isSafe, proxyURL := url_check(payload.URL, payload.FILENAME, payload.MIME)
	fmt.Printf("VirusTotal result for URL %s isSafe: %v\n", payload.URL, isSafe)

	w.Header().Set("Content-Type", "application/json")
	resp := map[string]any{
		"isSafe":   isSafe,
		"proxyUrl": proxyURL, // empty when unsafe
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		fmt.Printf("Error encoding response for URL %s: %v\n", payload.URL, err)
	}

	// push to channel after responding
	dataChan <- payload
}

// serveOnce serves a file exactly once, then forgets it and cleans temp
func serveOnce(w http.ResponseWriter, r *http.Request) {
	// (Optional) CORS for GETs too
	w.Header().Set("Access-Control-Allow-Origin", "*")

	token := strings.TrimPrefix(r.URL.Path, "/safe/")

	safeFiles.mu.Lock()
	p, ok := safeFiles.m[token]
	if ok {
		delete(safeFiles.m, token) // one-shot
	}
	safeFiles.mu.Unlock()

	if !ok {
		http.NotFound(w, r)
		return
	}

	// set a download header
	w.Header().Set("Content-Disposition", `attachment; filename="`+fileBase(p)+`"`)

	// open -> serve -> close (ensures Windows releases the handle)
	f, err := os.Open(p)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	fi, _ := f.Stat()
	http.ServeContent(w, r, fileBase(p), fi.ModTime(), f)
	_ = f.Close()

	// remove the served file
	_ = safeRemove(p)

	// remove empty parents up to ./temp
	cleanEmpties(p)

	// best-effort: try removing the temp roots if now empty
	_ = os.Remove("./temp/uncompressed")
	_ = os.Remove("./temp/compressed")
	_ = os.Remove("./temp")
}

func receive(c chan DataPayload) {
	dataChan = c
	http.HandleFunc("/submit-data", handler)
	http.HandleFunc("/safe/", serveOnce) // serves the already-downloaded file once

	fmt.Println("Server listening on :8080")
	http.ListenAndServe(":8080", nil)
}
