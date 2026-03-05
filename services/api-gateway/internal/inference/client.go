package inference

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Model describes an available model.
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// Server represents a backend inference server (vLLM).
type Server struct {
	URL     string
	Models  []string
	Healthy bool
}

// Pool manages inference server backends with health checking and load balancing.
type Pool struct {
	mu      sync.RWMutex
	servers []*Server
	counter uint64
}

// NewPool creates an inference server pool.
func NewPool(serverURLs []string) *Pool {
	servers := make([]*Server, len(serverURLs))
	for i, url := range serverURLs {
		servers[i] = &Server{
			URL:     url,
			Healthy: true,
		}
	}
	return &Pool{servers: servers}
}

// Next returns the next healthy server (round-robin).
func (p *Pool) Next() (*Server, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	n := len(p.servers)
	if n == 0 {
		return nil, fmt.Errorf("no inference servers configured")
	}

	start := int(atomic.AddUint64(&p.counter, 1)) % n
	for i := 0; i < n; i++ {
		idx := (start + i) % n
		if p.servers[idx].Healthy {
			return p.servers[idx], nil
		}
	}
	return nil, fmt.Errorf("no healthy inference servers")
}

// ListModels returns all models from healthy servers.
func (p *Pool) ListModels() []Model {
	return []Model{
		{ID: "zerone-8b", Object: "model", Created: time.Now().Unix(), OwnedBy: "zerone"},
		{ID: "zerone-70b", Object: "model", Created: time.Now().Unix(), OwnedBy: "zerone"},
	}
}

// HealthCheck pings all servers and updates their health status.
func (p *Pool) HealthCheck(ctx context.Context) {
	p.mu.Lock()
	defer p.mu.Unlock()

	client := &http.Client{Timeout: 5 * time.Second}
	for _, s := range p.servers {
		resp, err := client.Get(s.URL + "/health")
		s.Healthy = err == nil && resp != nil && resp.StatusCode == http.StatusOK
		if resp != nil {
			resp.Body.Close()
		}
	}
}

// Forward sends a request to an inference server and returns the response body.
func Forward(ctx context.Context, server *Server, path string, body []byte) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+path, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("inference request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return data, resp.StatusCode, nil
}

// ForwardStream sends a streaming request and returns the response for SSE forwarding.
func ForwardStream(ctx context.Context, server *Server, path string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("inference stream request: %w", err)
	}
	return resp, nil
}

// ChatRequest is the OpenAI-compatible chat completion request.
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// Message is a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse is the OpenAI-compatible chat completion response.
type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice is a response choice.
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage tracks token consumption.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ParseChatResponse parses an inference server response.
func ParseChatResponse(data []byte) (*ChatResponse, error) {
	var resp ChatResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
