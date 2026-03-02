package e2e_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/stretchr/testify/require"
)

// SubmitClaim submits a knowledge claim and returns the claim ID.
// reviewFee is a raw integer string (e.g., "1000000" for 1 ZRN).
func SubmitClaim(t *testing.T, chain *cosmos.CosmosChain, ctx context.Context,
	keyName, content, domain, category, reviewFee string,
) string {
	t.Helper()

	ExecTx(t, chain, ctx, keyName,
		"knowledge", "submit-claim", content, domain, category, reviewFee,
	)

	// Query pending claims to find the one we just submitted.
	out := QueryModule(t, chain, ctx, "knowledge", "pending-claims")

	var resp struct {
		Claims []struct {
			Id          string `json:"id"`
			FactContent string `json:"fact_content"`
		} `json:"claims"`
	}
	require.NoError(t, json.Unmarshal(out, &resp), "unmarshal pending-claims response")

	// Find the claim matching our content.
	for _, c := range resp.Claims {
		if c.FactContent == content {
			return c.Id
		}
	}

	// Fallback: claim may have already moved past PENDING (fast blocks).
	// Query all claims by looking at the last one submitted.
	require.FailNow(t, "submitted claim not found in pending-claims",
		"content=%q, response=%s", content, string(out))
	return ""
}

// GetClaimRoundID queries a claim by ID and returns its verification_round_id.
func GetClaimRoundID(t *testing.T, chain *cosmos.CosmosChain, ctx context.Context, claimID string) string {
	t.Helper()

	out := QueryModule(t, chain, ctx, "knowledge", "claim", claimID)

	var resp struct {
		Claim struct {
			VerificationRoundId string `json:"verification_round_id"`
			Status              string `json:"status"`
		} `json:"claim"`
	}
	require.NoError(t, json.Unmarshal(out, &resp), "unmarshal claim response")
	require.NotEmpty(t, resp.Claim.VerificationRoundId, "claim %s has no round ID", claimID)

	return resp.Claim.VerificationRoundId
}

// CommitVote computes SHA-256(vote || salt) and submits a commitment.
func CommitVote(t *testing.T, chain *cosmos.CosmosChain, ctx context.Context,
	keyName, roundID, vote string, salt []byte,
) {
	t.Helper()

	h := sha256.New()
	h.Write([]byte(vote))
	h.Write(salt)
	hashHex := hex.EncodeToString(h.Sum(nil))

	ExecTx(t, chain, ctx, keyName,
		"knowledge", "submit-commitment", roundID, hashHex,
	)
}

// RevealVote submits a reveal with the vote and salt.
func RevealVote(t *testing.T, chain *cosmos.CosmosChain, ctx context.Context,
	keyName, roundID, vote string, salt []byte,
) {
	t.Helper()

	saltHex := hex.EncodeToString(salt)

	ExecTx(t, chain, ctx, keyName,
		"knowledge", "submit-reveal", roundID, vote, saltHex,
	)
}

// QueryRound queries a verification round and returns the parsed JSON.
func QueryRound(t *testing.T, chain *cosmos.CosmosChain, ctx context.Context, roundID string) map[string]interface{} {
	t.Helper()

	out := QueryModule(t, chain, ctx, "knowledge", "verification-round", roundID)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(out, &resp), "unmarshal verification-round response")
	return resp
}

// QueryFact queries a fact by ID and returns the parsed JSON.
func QueryFact(t *testing.T, chain *cosmos.CosmosChain, ctx context.Context, factID string) map[string]interface{} {
	t.Helper()

	out := QueryModule(t, chain, ctx, "knowledge", "fact", factID)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(out, &resp), "unmarshal fact response")
	return resp
}

// QueryFactsByDomain returns all facts in a domain.
func QueryFactsByDomain(t *testing.T, chain *cosmos.CosmosChain, ctx context.Context, domain string) []interface{} {
	t.Helper()

	out := QueryModule(t, chain, ctx, "knowledge", "facts-by-domain", domain)

	var resp struct {
		Facts []interface{} `json:"facts"`
	}
	require.NoError(t, json.Unmarshal(out, &resp), "unmarshal facts-by-domain response")
	return resp.Facts
}

// QueryDomainCapacity queries the domain carrying capacity and pressure.
func QueryDomainCapacity(t *testing.T, chain *cosmos.CosmosChain, ctx context.Context, domain string) map[string]interface{} {
	t.Helper()

	out := QueryModule(t, chain, ctx, "knowledge", "domain-capacity", domain)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(out, &resp), "unmarshal domain-capacity response")
	return resp
}

// QueryMetabolismStatus queries the global metabolism status.
func QueryMetabolismStatus(t *testing.T, chain *cosmos.CosmosChain, ctx context.Context) map[string]interface{} {
	t.Helper()

	out := QueryModule(t, chain, ctx, "knowledge", "metabolism-status")

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(out, &resp), "unmarshal metabolism-status response")
	return resp
}

// WaitForRoundPhase polls a verification round until it reaches the target phase.
// phase should be one of: "VERIFICATION_PHASE_COMMIT", "VERIFICATION_PHASE_REVEAL",
// "VERIFICATION_PHASE_AGGREGATION", "VERIFICATION_PHASE_COMPLETE", "VERIFICATION_PHASE_EXPIRED".
func WaitForRoundPhase(t *testing.T, chain *cosmos.CosmosChain, ctx context.Context,
	roundID, targetPhase string, maxBlocks int,
) {
	t.Helper()

	for i := 0; i < maxBlocks; i++ {
		round := QueryRound(t, chain, ctx, roundID)
		roundData, ok := round["round"].(map[string]interface{})
		if ok {
			if phase, _ := roundData["phase"].(string); phase == targetPhase {
				return
			}
		}
		WaitBlocks(t, chain, ctx, 1)
	}

	// Final check
	round := QueryRound(t, chain, ctx, roundID)
	roundData, _ := round["round"].(map[string]interface{})
	phase := ""
	if roundData != nil {
		phase, _ = roundData["phase"].(string)
	}
	require.Equal(t, targetPhase, phase,
		"round %s did not reach phase %s within %d blocks (current: %s)",
		roundID, targetPhase, maxBlocks, phase)
}

// DriveClaimToFact drives a claim through the full verification lifecycle
// (submit -> commit -> reveal -> aggregation) and returns the claim ID.
// All verifierKeys vote "accept". Caller must wait for the fact to be queryable.
func DriveClaimToFact(t *testing.T, chain *cosmos.CosmosChain, ctx context.Context,
	submitterKey string, verifierKeys []string,
	content, domain, category, reviewFee string,
) string {
	t.Helper()

	// 1. Submit claim
	claimID := SubmitClaim(t, chain, ctx, submitterKey, content, domain, category, reviewFee)
	t.Logf("Submitted claim %s", claimID)

	// 2. Get round ID
	roundID := GetClaimRoundID(t, chain, ctx, claimID)
	t.Logf("Verification round %s", roundID)

	// 3. Commit phase -- each verifier commits "accept"
	salts := make(map[string][]byte)
	for _, key := range verifierKeys {
		salt := []byte(fmt.Sprintf("salt-%s-%s", key, claimID))
		salts[key] = salt
		CommitVote(t, chain, ctx, key, roundID, "accept", salt)
	}

	// 4. Wait for reveal phase
	WaitForRoundPhase(t, chain, ctx, roundID, "VERIFICATION_PHASE_REVEAL", 15)

	// 5. Reveal phase -- each verifier reveals
	for _, key := range verifierKeys {
		RevealVote(t, chain, ctx, key, roundID, "accept", salts[key])
	}

	// 6. Wait for round completion
	WaitForRoundPhase(t, chain, ctx, roundID, "VERIFICATION_PHASE_COMPLETE", 20)

	return claimID
}

// CommitVoteOnNode computes SHA-256(vote || salt) and submits a commitment
// via a specific validator node. Use this when testing with multiple validators
// since each node has its own keyring with a key named "validator".
func CommitVoteOnNode(t *testing.T, node *cosmos.ChainNode, ctx context.Context,
	roundID, vote string, salt []byte,
) {
	t.Helper()

	h := sha256.New()
	h.Write([]byte(vote))
	h.Write(salt)
	hashHex := hex.EncodeToString(h.Sum(nil))

	txHash, err := node.ExecTx(ctx, "validator",
		"knowledge", "submit-commitment", roundID, hashHex,
	)
	require.NoError(t, err, "CommitVoteOnNode failed (tx=%s)", txHash)
}

// RevealVoteOnNode submits a reveal via a specific validator node.
func RevealVoteOnNode(t *testing.T, node *cosmos.ChainNode, ctx context.Context,
	roundID, vote string, salt []byte,
) {
	t.Helper()

	saltHex := hex.EncodeToString(salt)

	txHash, err := node.ExecTx(ctx, "validator",
		"knowledge", "submit-reveal", roundID, vote, saltHex,
	)
	require.NoError(t, err, "RevealVoteOnNode failed (tx=%s)", txHash)
}

// jsonNumber extracts a numeric value from a JSON map, handling both
// json.Number and float64 representations.
func jsonNumber(m map[string]interface{}, key string) (uint64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, false
		}
		return uint64(i), true
	case float64:
		return uint64(n), true
	case string:
		// Some proto fields serialize as strings
		var i uint64
		_, err := fmt.Sscanf(n, "%d", &i)
		return i, err == nil
	}
	return 0, false
}
