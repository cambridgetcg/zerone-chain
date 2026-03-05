package access

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Ticket authorizes a buyer to access dataset chunks.
type Ticket struct {
	BuyerAddr   string `json:"buyer_addr"`
	DatasetID   string `json:"dataset_id"`
	ChunkScope  string `json:"chunk_scope"` // "all" or "0,1,2,3" (comma-separated indices)
	ExpiryBlock int64  `json:"expiry_block"`
	Signature   string `json:"signature"` // HMAC signature from payment bridge
}

// SignTicket creates a signed access ticket.
func SignTicket(buyerAddr, datasetID, chunkScope string, expiryBlock int64, signingKey []byte) *Ticket {
	t := &Ticket{
		BuyerAddr:   buyerAddr,
		DatasetID:   datasetID,
		ChunkScope:  chunkScope,
		ExpiryBlock: expiryBlock,
	}
	t.Signature = computeSignature(t, signingKey)
	return t
}

// Verify checks the ticket signature and scope.
func (t *Ticket) Verify(signingKey []byte, currentBlock int64, requestedChunkIndex int) error {
	// Check expiry
	if currentBlock > t.ExpiryBlock {
		return fmt.Errorf("ticket expired at block %d, current %d", t.ExpiryBlock, currentBlock)
	}

	// Check signature
	expected := computeSignature(t, signingKey)
	if !hmac.Equal([]byte(t.Signature), []byte(expected)) {
		return fmt.Errorf("invalid ticket signature")
	}

	// Check scope
	if t.ChunkScope != "all" {
		allowed := parseChunkScope(t.ChunkScope)
		found := false
		for _, idx := range allowed {
			if idx == requestedChunkIndex {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("chunk %d not in ticket scope", requestedChunkIndex)
		}
	}

	return nil
}

func computeSignature(t *Ticket, key []byte) string {
	msg := fmt.Sprintf("%s|%s|%s|%d", t.BuyerAddr, t.DatasetID, t.ChunkScope, t.ExpiryBlock)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

func parseChunkScope(scope string) []int {
	parts := strings.Split(scope, ",")
	var indices []int
	for _, p := range parts {
		idx, err := strconv.Atoi(strings.TrimSpace(p))
		if err == nil {
			indices = append(indices, idx)
		}
	}
	return indices
}

// Ensure time is imported (for potential future use)
var _ = time.Now
