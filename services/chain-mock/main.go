// main.go - just a health endpoint and request logger
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	addr := ":9090"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		addr = v
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})
	// Catch-all logs requests from services trying to reach the chain
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[chain-mock] %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"mock": true}`)
	})

	log.Printf("chain-mock listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
