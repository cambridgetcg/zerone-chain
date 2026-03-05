package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/zerone-chain/zerone/services/api-gateway/internal/auth"
	"github.com/zerone-chain/zerone/services/api-gateway/internal/inference"
	"github.com/zerone-chain/zerone/services/api-gateway/internal/ratelimit"
)

// Gateway is the API gateway HTTP handler.
type Gateway struct {
	authStore   *auth.Store
	pool        *inference.Pool
	rateLimiter *ratelimit.TokenBucket
	mux         *http.ServeMux
}

// New creates a new API gateway handler.
func New(authStore *auth.Store, pool *inference.Pool, rl *ratelimit.TokenBucket) *Gateway {
	g := &Gateway{
		authStore:   authStore,
		pool:        pool,
		rateLimiter: rl,
		mux:         http.NewServeMux(),
	}
	g.routes()
	return g
}

func (g *Gateway) routes() {
	g.mux.HandleFunc("POST /v1/chat/completions", g.handleChatCompletions)
	g.mux.HandleFunc("GET /v1/models", g.handleListModels)
	g.mux.HandleFunc("GET /v1/balance", g.handleBalance)
	g.mux.HandleFunc("POST /v1/keys/create", g.handleCreateKey)
	g.mux.HandleFunc("GET /healthz", g.handleHealth)
	g.mux.HandleFunc("GET /readyz", g.handleReady)
}

// ServeHTTP implements http.Handler.
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mux.ServeHTTP(w, r)
}

func (g *Gateway) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	// Auth
	walletAddr, err := g.authenticate(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	// Rate limit
	if !g.rateLimiter.Allow(walletAddr) {
		writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
		return
	}

	// Parse request
	var req inference.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Route to inference
	server, err := g.pool.Next()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "no inference servers available")
		return
	}

	body, _ := json.Marshal(req)
	respData, status, err := inference.Forward(r.Context(), server, "/v1/chat/completions", body)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("inference error: %v", err))
		return
	}

	if status != http.StatusOK {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write(respData)
		return
	}

	// Parse for usage metering
	chatResp, err := inference.ParseChatResponse(respData)
	if err != nil {
		// Forward raw response even if we can't parse it
		w.Header().Set("Content-Type", "application/json")
		w.Write(respData)
		return
	}

	// TODO: Deduct from payment bridge based on chatResp.Usage
	_ = walletAddr
	_ = chatResp.Usage

	w.Header().Set("Content-Type", "application/json")
	w.Write(respData)
}

func (g *Gateway) handleListModels(w http.ResponseWriter, r *http.Request) {
	models := g.pool.ListModels()
	resp := map[string]interface{}{
		"object": "list",
		"data":   models,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (g *Gateway) handleBalance(w http.ResponseWriter, r *http.Request) {
	walletAddr, err := g.authenticate(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	// TODO: Query payment bridge for actual balance
	resp := map[string]interface{}{
		"wallet":  walletAddr,
		"balance": "0",
		"unit":    "uzrn",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (g *Gateway) handleCreateKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WalletAddr string `json:"wallet_addr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.WalletAddr == "" {
		writeError(w, http.StatusBadRequest, "wallet_addr required")
		return
	}

	fullKey, key, err := g.authStore.CreateKey(req.WalletAddr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := map[string]interface{}{
		"key":        fullKey,
		"key_id":     key.ID,
		"created_at": key.CreatedAt.Format(time.RFC3339),
		"note":       "Save this key — it will not be shown again",
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (g *Gateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func (g *Gateway) handleReady(w http.ResponseWriter, r *http.Request) {
	server, err := g.pool.Next()
	if err != nil || server == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"not ready","reason":"no inference servers"}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ready"}`))
}

func (g *Gateway) authenticate(r *http.Request) (string, error) {
	token := auth.ExtractBearerToken(r.Header.Get("Authorization"))
	if token == "" {
		return "", fmt.Errorf("missing authorization header")
	}
	return g.authStore.Validate(token)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"message": msg,
			"type":    "api_error",
			"code":    http.StatusText(status),
		},
	})
}

// NewRequestID generates a unique request ID.
func NewRequestID() string {
	return "chatcmpl-" + uuid.New().String()[:8]
}
