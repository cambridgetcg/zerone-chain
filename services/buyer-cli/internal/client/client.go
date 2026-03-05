package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client interacts with the ZERONE marketplace API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// New creates a marketplace API client.
func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Dataset represents a marketplace dataset.
type Dataset struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Domain       string           `json:"domain"`
	Description  string           `json:"description"`
	SampleCount  int64            `json:"sample_count"`
	SizeBytes    int64            `json:"size_bytes"`
	QualityStats map[string]int64 `json:"quality_stats"`
	ChunkCount   int              `json:"chunk_count"`
	Version      string           `json:"version"`
	AvgRating    float64          `json:"avg_rating"`
	RatingCount  int64            `json:"rating_count"`
	Pricing      []TierConfig     `json:"pricing"`
}

// TierConfig mirrors the server-side pricing tier.
type TierConfig struct {
	Tier        string `json:"tier"`
	PriceUZRN   int64  `json:"price_uzrn"`
	Description string `json:"description"`
}

// Purchase represents a purchase record.
type Purchase struct {
	ID            string   `json:"id"`
	DatasetID     string   `json:"dataset_id"`
	Tier          string   `json:"tier"`
	AmountUZRN    int64    `json:"amount_uzrn"`
	Status        string   `json:"status"`
	ShamirShares  int      `json:"shamir_shares"`
	AccessTickets []string `json:"access_tickets"`
	ExpiresAt     string   `json:"expires_at"`
}

// DownloadInfo contains chunk download details.
type DownloadInfo struct {
	PurchaseID    string   `json:"purchase_id"`
	DatasetID     string   `json:"dataset_id"`
	AccessTickets []string `json:"access_tickets"`
	ChunkOrder    []int    `json:"chunk_order"`
	TotalChunks   int      `json:"total_chunks"`
	ExpiresAt     string   `json:"expires_at"`
	WatermarkID   string   `json:"watermark_id"`
}

// PreviewResponse contains dataset preview data.
type PreviewResponse struct {
	DatasetID string          `json:"dataset_id"`
	Name      string          `json:"name"`
	Previews  []PreviewSample `json:"previews"`
}

// PreviewSample is a single preview item.
type PreviewSample struct {
	Index    int    `json:"index"`
	Domain   string `json:"domain"`
	Category string `json:"category"`
	Snippet  string `json:"snippet"`
}

// Browse lists available datasets with optional filters.
func (c *Client) Browse(domain string) ([]Dataset, error) {
	params := url.Values{}
	if domain != "" {
		params.Set("domain", domain)
	}

	endpoint := "/v1/datasets"
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}

	var resp struct {
		Datasets []Dataset `json:"datasets"`
		Count    int       `json:"count"`
	}
	if err := c.get(endpoint, &resp); err != nil {
		return nil, err
	}
	return resp.Datasets, nil
}

// Preview fetches a free preview of a dataset.
func (c *Client) Preview(datasetID string) (*PreviewResponse, error) {
	var resp PreviewResponse
	if err := c.get(fmt.Sprintf("/v1/datasets/%s/preview", datasetID), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Purchase initiates a dataset purchase.
func (c *Client) Purchase(datasetID, buyerAddr string, amountUZRN int64) (*Purchase, error) {
	body := map[string]interface{}{
		"buyer_addr":  buyerAddr,
		"amount_uzrn": amountUZRN,
	}
	var resp Purchase
	if err := c.post(fmt.Sprintf("/v1/datasets/%s/purchase", datasetID), body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// PurchaseStatus checks the status of a purchase.
func (c *Client) PurchaseStatus(datasetID, purchaseID string) (*Purchase, error) {
	var resp Purchase
	if err := c.get(fmt.Sprintf("/v1/datasets/%s/purchase/%s", datasetID, purchaseID), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Download retrieves chunk download info for a confirmed purchase.
func (c *Client) Download(datasetID, purchaseID string) (*DownloadInfo, error) {
	var resp DownloadInfo
	endpoint := fmt.Sprintf("/v1/datasets/%s/download?purchase_id=%s", datasetID, purchaseID)
	if err := c.get(endpoint, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SubmitRating submits a rating for a purchased dataset.
func (c *Client) SubmitRating(datasetID, purchaseID, buyerAddr string, stars int, comment string, weakAreas []string) error {
	body := map[string]interface{}{
		"purchase_id": purchaseID,
		"buyer_addr":  buyerAddr,
		"stars":       stars,
		"comment":     comment,
		"weak_areas":  weakAreas,
	}
	return c.post(fmt.Sprintf("/v1/datasets/%s/rate", datasetID), body, nil)
}

// Pricing retrieves available pricing tiers.
func (c *Client) Pricing() ([]TierConfig, error) {
	var resp struct {
		Tiers []TierConfig `json:"tiers"`
	}
	if err := c.get("/v1/pricing", &resp); err != nil {
		return nil, err
	}
	return resp.Tiers, nil
}

func (c *Client) get(path string, result interface{}) error {
	req, err := http.NewRequest("GET", c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}

func (c *Client) post(path string, body interface{}, result interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}
