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

// BridgeClient is the interface for communicating with the payment bridge.
type BridgeClient interface {
	GetBalance(ctx context.Context, walletAddr string) (int64, error)
	Deduct(ctx context.Context, walletAddr string, costUZRN int64) (int64, error)
	RecordUsage(ctx context.Context, walletAddr, requestID string, inputTokens, outputTokens int64, model string) error
}

// Gateway is the API gateway HTTP handler.
type Gateway struct {
	authStore   *auth.Store
	pool        *inference.Pool
	rateLimiter *ratelimit.TokenBucket
	bridge      BridgeClient
	mux         *http.ServeMux

	// API pricing (uzrn per 1000 tokens)
	pricePerInputToken  int64
	pricePerOutputToken int64
}

// New creates a new API gateway handler.
func New(authStore *auth.Store, pool *inference.Pool, rl *ratelimit.TokenBucket) *Gateway {
	g := &Gateway{
		authStore:           authStore,
		pool:                pool,
		rateLimiter:         rl,
		pricePerInputToken:  1, // 1 uzrn per 1000 input tokens
		pricePerOutputToken: 3, // 3 uzrn per 1000 output tokens
		mux:                 http.NewServeMux(),
	}
	g.routes()
	return g
}

// SetBridgeClient sets the payment bridge client for balance/usage operations.
func (g *Gateway) SetBridgeClient(bridge BridgeClient) {
	g.bridge = bridge
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

	// Compute token cost and deduct from prepaid balance
	inputTokens := int64(chatResp.Usage.PromptTokens)
	outputTokens := int64(chatResp.Usage.CompletionTokens)
	cost := computeTokenCost(inputTokens, outputTokens, g.pricePerInputToken, g.pricePerOutputToken)

	if g.bridge != nil && cost > 0 {
		newBal, err := g.bridge.Deduct(r.Context(), walletAddr, cost)
		if err != nil {
			writeError(w, http.StatusPaymentRequired, fmt.Sprintf("insufficient API credits: %v", err))
			return
		}

		// Record usage for batch settlement
		reqID := NewRequestID()
		_ = g.bridge.RecordUsage(r.Context(), walletAddr, reqID, inputTokens, outputTokens, chatResp.Model)

		// Low balance warning (10% of total deposited)
		if newBal > 0 && newBal < cost*10 {
			w.Header().Set("X-ZRN-Balance-Warning", fmt.Sprintf("low balance: %d uzrn remaining", newBal))
		}
	}

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

	var balanceStr string
	if g.bridge != nil {
		bal, err := g.bridge.GetBalance(r.Context(), walletAddr)
		if err != nil {
			balanceStr = "0"
		} else {
			balanceStr = fmt.Sprintf("%d", bal)
		}
	} else {
		balanceStr = "0"
	}

	resp := map[string]interface{}{
		"wallet":  walletAddr,
		"balance": balanceStr,
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

// computeTokenCost calculates the uzrn cost for a request based on token counts.
// Prices are per 1000 tokens.
func computeTokenCost(inputTokens, outputTokens, pricePerInput, pricePerOutput int64) int64 {
	inputCost := (inputTokens * pricePerInput) / 1000
	outputCost := (outputTokens * pricePerOutput) / 1000
	total := inputCost + outputCost
	if total < 1 && (inputTokens > 0 || outputTokens > 0) {
		total = 1 // minimum 1 uzrn per request
	}
	return total
}
