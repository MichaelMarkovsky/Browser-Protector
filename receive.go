// receive.go
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
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

	// For preflight OPTIONS requests (browser does this sometimes)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Only POST requests are allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload DataPayload
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		fmt.Printf("Error decoding payload: %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Printf("Checking URL with VirusTotal: %s, Filename: %s, MIME: %s\n", payload.URL, payload.FILENAME, payload.MIME)

	// url_check now returns (bool, proxyURL)
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

// serveOnce serves a file exactly once, then forgets it
func serveOnce(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.URL.Path, "/safe/")

	safeFiles.mu.Lock()
	path, ok := safeFiles.m[token]
	if ok {
		delete(safeFiles.m, token) // one-shot
	}
	safeFiles.mu.Unlock()

	if !ok {
		http.NotFound(w, r)
		return
	}

	// Set a download header
	attach := `attachment; filename="` + fileBase(path) + `"`
	w.Header().Set("Content-Disposition", attach)

	http.ServeFile(w, r, path)

	// remove after serving
	_ = safeRemove(path)
	// clean parent temp dirs if empty
	cleanEmpties(path)
}

func receive(c chan DataPayload) {
	dataChan = c
	http.HandleFunc("/submit-data", handler)
	http.HandleFunc("/safe/", serveOnce) // new endpoint that serves the already-downloaded file

	fmt.Println("Server listening on :8080")
	http.ListenAndServe(":8080", nil)
}
