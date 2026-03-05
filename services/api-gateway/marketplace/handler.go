package marketplace

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// Handler serves marketplace API endpoints.
type Handler struct {
	catalog   *Catalog
	purchases *PurchaseManager
	feedback  *FeedbackStore
	watermark *Watermarker
}

// NewHandler creates a marketplace HTTP handler.
func NewHandler(catalog *Catalog, purchases *PurchaseManager, feedback *FeedbackStore) *Handler {
	return &Handler{
		catalog:   catalog,
		purchases: purchases,
		feedback:  feedback,
		watermark: NewWatermarker(),
	}
}

// RegisterRoutes adds marketplace routes to the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/datasets", h.handleListDatasets)
	mux.HandleFunc("GET /v1/datasets/{id}", h.handleGetDataset)
	mux.HandleFunc("GET /v1/datasets/{id}/preview", h.handlePreview)
	mux.HandleFunc("POST /v1/datasets/{id}/purchase", h.handlePurchase)
	mux.HandleFunc("GET /v1/datasets/{id}/purchase/{txid}", h.handlePurchaseStatus)
	mux.HandleFunc("GET /v1/datasets/{id}/download", h.handleDownload)
	mux.HandleFunc("GET /v1/datasets/{id}/ratings", h.handleGetRatings)
	mux.HandleFunc("POST /v1/datasets/{id}/rate", h.handleSubmitRating)
	mux.HandleFunc("GET /v1/pricing", h.handlePricing)
}

func (h *Handler) handleListDatasets(w http.ResponseWriter, r *http.Request) {
	f := ListFilter{
		Domain: r.URL.Query().Get("domain"),
		MinQuality: r.URL.Query().Get("min_quality"),
	}
	if v := r.URL.Query().Get("min_samples"); v != "" {
		n, _ := strconv.ParseInt(v, 10, 64)
		f.MinSamples = n
	}
	if v := r.URL.Query().Get("max_price"); v != "" {
		n, _ := strconv.ParseInt(v, 10, 64)
		f.MaxPrice = n
	}
	if v := r.URL.Query().Get("tags"); v != "" {
		f.Tags = strings.Split(v, ",")
	}

	datasets := h.catalog.List(f)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"datasets": datasets,
		"count":    len(datasets),
	})
}

func (h *Handler) handleGetDataset(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ds, ok := h.catalog.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "dataset not found")
		return
	}
	writeJSON(w, http.StatusOK, ds)
}

func (h *Handler) handlePreview(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ds, ok := h.catalog.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "dataset not found")
		return
	}

	// Generate preview samples (5 low-quality snippets)
	previews := make([]PreviewSample, 0, 5)
	for i := 0; i < 5 && i < int(ds.SampleCount); i++ {
		previews = append(previews, PreviewSample{
			Index:    i,
			Domain:   ds.Domain,
			Category: "preview",
			Snippet:  "[Preview content — purchase for full access]",
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"dataset_id": ds.ID,
		"name":       ds.Name,
		"previews":   previews,
		"tier":       TierPreview,
		"note":       "Purchase dataset for full access",
	})
}

func (h *Handler) handlePurchase(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		BuyerAddr  string `json:"buyer_addr"`
		AmountUZRN int64  `json:"amount_uzrn"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.BuyerAddr == "" {
		writeError(w, http.StatusBadRequest, "buyer_addr required")
		return
	}

	purchase, err := h.purchases.Initiate(req.BuyerAddr, id, req.AmountUZRN)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// In production, payment verification happens async via payment bridge.
	// For now, auto-confirm for non-zero amounts.
	if req.AmountUZRN > 0 {
		if err := h.purchases.ConfirmPayment(purchase.ID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		// Re-fetch to get updated status and tickets
		purchase, _ = h.purchases.Get(purchase.ID)
	}

	writeJSON(w, http.StatusCreated, purchase)
}

func (h *Handler) handlePurchaseStatus(w http.ResponseWriter, r *http.Request) {
	txid := r.PathValue("txid")

	purchase, ok := h.purchases.Get(txid)
	if !ok {
		writeError(w, http.StatusNotFound, "purchase not found")
		return
	}

	writeJSON(w, http.StatusOK, purchase)
}

func (h *Handler) handleDownload(w http.ResponseWriter, r *http.Request) {
	purchaseID := r.URL.Query().Get("purchase_id")
	if purchaseID == "" {
		writeError(w, http.StatusBadRequest, "purchase_id query parameter required")
		return
	}

	purchase, ok := h.purchases.Get(purchaseID)
	if !ok {
		writeError(w, http.StatusNotFound, "purchase not found")
		return
	}

	if purchase.Status != PurchaseConfirmed && purchase.Status != PurchaseComplete {
		writeError(w, http.StatusForbidden, "purchase not confirmed")
		return
	}

	ds, ok := h.catalog.Get(purchase.DatasetID)
	if !ok {
		writeError(w, http.StatusNotFound, "dataset not found")
		return
	}

	// Generate watermarked chunk ordering for this buyer
	ordering := h.watermark.Permutation(purchase.WatermarkSeed, ds.ChunkCount)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"purchase_id":    purchase.ID,
		"dataset_id":    purchase.DatasetID,
		"access_tickets": purchase.AccessTickets,
		"chunk_order":   ordering[:min(len(ordering), purchase.ShamirShares*10)],
		"total_chunks":  ds.ChunkCount,
		"expires_at":    purchase.ExpiresAt,
		"watermark_id":  h.watermark.Fingerprint(purchase.WatermarkSeed, ds.ID),
	})
}

func (h *Handler) handleGetRatings(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	ratings := h.feedback.GetRatings(id)
	weakAreas := h.feedback.WeakAreaSummary(id)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"dataset_id": id,
		"ratings":    ratings,
		"count":      len(ratings),
		"weak_areas": weakAreas,
	})
}

func (h *Handler) handleSubmitRating(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		PurchaseID string   `json:"purchase_id"`
		BuyerAddr  string   `json:"buyer_addr"`
		Stars      int      `json:"stars"`
		Comment    string   `json:"comment"`
		WeakAreas  []string `json:"weak_areas"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	rating, err := h.feedback.Submit(req.PurchaseID, id, req.BuyerAddr, req.Stars, req.Comment, req.WeakAreas)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, rating)
}

func (h *Handler) handlePricing(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tiers": DefaultTiers(),
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"message": msg,
			"type":    "marketplace_error",
			"code":    http.StatusText(status),
		},
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
