package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type DataPayload struct {
	ID       int    `json:"id"`
	URL      string `json:"url"`
	FILENAME string `json:"filename"`
	MIME     string `json:"mime"`
}

// a global channel to be used by handler
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
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Println("Checking URL with VirusTotal...")

	isSafe := url_check(payload.URL, payload.FILENAME) // returns bool (true = safe, false = malicious)

	status := "malicious"
	if isSafe {
		status = "safe"
	}

	// Respond with JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":     true,
		"status": status,
	})

	dataChan <- payload
}

func receive(c chan DataPayload) {
	dataChan = c // assign channel so handler can use it
	http.HandleFunc("/submit-data", handler)
	fmt.Println("Server listening on :8080")
	http.ListenAndServe(":8080", nil)
}
