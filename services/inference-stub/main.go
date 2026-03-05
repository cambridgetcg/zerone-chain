package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	addr := ":8000"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		addr = v
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
	})

	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data": []map[string]any{
				{
					"id":       "zerone-dev-stub",
					"object":   "model",
					"created":  time.Now().Unix(),
					"owned_by": "zerone-local",
				},
			},
		})
	})

	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[inference-stub] POST /v1/chat/completions from %s", r.RemoteAddr)

		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)

		stream, _ := req["stream"].(bool)

		if stream {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "streaming not supported", http.StatusInternalServerError)
				return
			}

			chunk := map[string]any{
				"id":      "chatcmpl-stub",
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   "zerone-dev-stub",
				"choices": []map[string]any{
					{
						"index": 0,
						"delta": map[string]any{
							"role":    "assistant",
							"content": "This is a stub response from the ZERONE inference dev server. Connect a real vLLM instance for actual inference.",
						},
						"finish_reason": "stop",
					},
				},
			}
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			fmt.Fprint(w, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-stub",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "zerone-dev-stub",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "This is a stub response from the ZERONE inference dev server. Connect a real vLLM instance for actual inference.",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 25,
				"total_tokens":      35,
			},
		})
	})

	log.Printf("inference-stub listening on %s (CPU-only dev mode)", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
