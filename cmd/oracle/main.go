package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
)

// newOracleServer creates an HTTP mux with /health, /evaluate, and /prefetch routes.
func newOracleServer(eval *CombinedEvaluator, tier string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
			"tier":   tier,
		})
	})

	mux.HandleFunc("POST /evaluate", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit
		var req EvaluateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusBadRequest)
			return
		}

		resp, err := eval.Evaluate(req)
		if err != nil {
			// On eval error, return uncertain response instead of 500.
			resp = &EvaluateResponse{
				Verdict:    "uncertain",
				Confidence: 0.5,
				Reasoning:  "evaluation error: " + err.Error(),
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("POST /prefetch", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit
		var req EvaluateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusBadRequest)
			return
		}

		go func() {
			_, _ = eval.Evaluate(req)
		}()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "prefetching",
		})
	})

	return mux
}

func main() {
	port := flag.Int("port", 8081, "HTTP server port")
	tier := flag.String("tier", "static", "evaluator tier: static or llm")
	llmAPIKey := flag.String("llm-api-key", "", "Anthropic API key (required for llm tier)")
	llmAPIURL := flag.String("llm-api-url", defaultAnthropicURL, "Anthropic API base URL")
	llmModel := flag.String("llm-model", "claude-sonnet-4-5-20250514", "LLM model name")
	llmMaxTokens := flag.Int("llm-max-tokens", 500, "LLM max tokens")
	llmTimeout := flag.Duration("llm-timeout", 2*time.Second, "LLM request timeout")
	flag.Parse()

	// 1. Create static evaluator.
	staticEval, err := NewStaticEvaluator()
	if err != nil {
		log.Fatalf("failed to create static evaluator: %v", err)
	}
	log.Printf("static evaluator loaded %d axioms", staticEval.AxiomCount())

	// 2. Create combined evaluator with static.
	combined := &CombinedEvaluator{Static: staticEval}

	// 3. If tier == "llm", add LLM evaluator.
	if *tier == "llm" {
		if *llmAPIKey == "" {
			log.Fatal("--llm-api-key is required when --tier=llm")
		}
		llmEval := NewLLMEvaluator(*llmAPIURL, *llmAPIKey, *llmModel, *llmMaxTokens, *llmTimeout)
		combined.LLM = llmEval
		log.Printf("llm evaluator configured: model=%s url=%s", *llmModel, *llmAPIURL)
	}

	// 4. Create and start server.
	srv := newOracleServer(combined, *tier)
	addr := fmt.Sprintf(":%d", *port)
	log.Printf("zerone-oracle starting on %s (tier=%s)", addr, *tier)
	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
