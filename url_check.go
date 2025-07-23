package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	// err - stores any error
	// resp - the response of the get request
	resp, err := http.Get("[URL]")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Status code is:", resp.StatusCode)
}
