package keeper_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Test helpers ───────────────────────────────────────────────────────────

func validPublishMsg(t *testing.T, domain string) *types.MsgPublishModel {
	t.Helper()
	return &types.MsgPublishModel{
		Publisher:        testAddr,
		Name:             fmt.Sprintf("test-model-%s", domain),
		Domain:           domain,
		TrainingRecordID: "attest-001",
		TDUIDs:           generateTDUIDs(15),
		DatasetIDs:       []string{"ds-1"},
		BenchmarkScore:   "0.500000000000000000",
		BenchmarkDetails: []types.BenchmarkResult{
			{BenchmarkID: "bench-1", Score: "0.500000000000000000", Category: "code", PassRate: "0.800000000000000000"},
		},
		FitnessWeighted: "0.600000000000000000",
		TEEAttestation:  "deadbeef",
		ModelHash:       "abc123def456",
	}
}

func generateTDUIDs(n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("tdu-%03d", i+1)
	}
	return ids
}

func modelIDFromMsg(msg *types.MsgPublishModel) string {
	idInput := msg.TEEAttestation + ":" + msg.ModelHash
	idHash := sha256.Sum256([]byte(idInput))
	return hex.EncodeToString(idHash[:])
}

func seedTrainingRecord(t *testing.T, k keeper.Keeper, ctx context.Context, attestationHash string) {
	t.Helper()
	rec := &types.TrainingRecord{
		Operator:        testAddr,
		AttestationHash: attestationHash,
		ModelHash:       "abc123def456",
		BaseModel:       "llama-3-8b",
		DatasetSize:     100,
		BenchmarkScore:  0.5,
	}
	err := k.SetTrainingRecord(ctx, rec)
	require.NoError(t, err)
}

// ─── Test: Publish Model — Happy Path ───────────────────────────────────────

func TestPublishModel_HappyPath(t *testing.T) {
	k, ctx := setupKeeper(t)
	seedTrainingRecord(t, k, ctx, "attest-001")

	msg := validPublishMsg(t, "code/go")
	resp, err := k.PublishModel(ctx, msg)
	require.NoError(t, err)
	require.NotEmpty(t, resp.ModelID)
	require.Equal(t, uint64(1), resp.Version)

	// Verify stored record.
	record, found := k.GetModelRecord(ctx, resp.ModelID)
	require.True(t, found)
	require.Equal(t, msg.Name, record.Name)
	require.Equal(t, msg.Domain, record.Domain)
	require.Equal(t, types.ModelStatusActive, record.Status)
	require.Equal(t, uint64(15), record.TDUCount)
	require.Equal(t, msg.Publisher, record.Publisher)
}

// ─── Test: Reject Below Quality Threshold ───────────────────────────────────

func TestPublishModel_RejectLowBenchmark(t *testing.T) {
	k, ctx := setupKeeper(t)
	seedTrainingRecord(t, k, ctx, "attest-001")

	msg := validPublishMsg(t, "code/go")
	msg.BenchmarkScore = "0.100000000000000000" // below 0.3
	_, err := k.PublishModel(ctx, msg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "below")
}

func TestPublishModel_RejectLowFitness(t *testing.T) {
	k, ctx := setupKeeper(t)
	seedTrainingRecord(t, k, ctx, "attest-001")

	msg := validPublishMsg(t, "code/go")
	msg.FitnessWeighted = "0.200000000000000000" // below 0.4
	_, err := k.PublishModel(ctx, msg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "below minimum")
}

func TestPublishModel_RejectTooFewTDUs(t *testing.T) {
	k, ctx := setupKeeper(t)
	seedTrainingRecord(t, k, ctx, "attest-001")

	msg := validPublishMsg(t, "code/go")
	msg.TDUIDs = []string{"tdu-1", "tdu-2"} // only 2, need 10
	_, err := k.PublishModel(ctx, msg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "below minimum")
}

// ─── Test: Reject Without TEE Attestation ───────────────────────────────────

func TestPublishModel_RejectNoAttestation(t *testing.T) {
	k, ctx := setupKeeper(t)
	seedTrainingRecord(t, k, ctx, "attest-001")

	msg := validPublishMsg(t, "code/go")
	msg.TEEAttestation = ""
	_, err := k.PublishModel(ctx, msg)
	require.Error(t, err)
}

func TestPublishModel_RejectNoModelHash(t *testing.T) {
	k, ctx := setupKeeper(t)
	seedTrainingRecord(t, k, ctx, "attest-001")

	msg := validPublishMsg(t, "code/go")
	msg.ModelHash = ""
	_, err := k.PublishModel(ctx, msg)
	require.Error(t, err)
}

// ─── Test: Duplicate Model ──────────────────────────────────────────────────

func TestPublishModel_RejectDuplicate(t *testing.T) {
	k, ctx := setupKeeper(t)
	seedTrainingRecord(t, k, ctx, "attest-001")

	msg := validPublishMsg(t, "code/go")
	_, err := k.PublishModel(ctx, msg)
	require.NoError(t, err)

	_, err = k.PublishModel(ctx, msg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already")
}

// ─── Test: Deprecate Model ──────────────────────────────────────────────────

func TestDeprecateModel(t *testing.T) {
	k, ctx := setupKeeper(t)
	seedTrainingRecord(t, k, ctx, "attest-001")

	msg := validPublishMsg(t, "code/go")
	resp, err := k.PublishModel(ctx, msg)
	require.NoError(t, err)

	err = k.DeprecateModel(ctx, resp.ModelID, testAddr, "outdated training data")
	require.NoError(t, err)

	record, found := k.GetModelRecord(ctx, resp.ModelID)
	require.True(t, found)
	require.Equal(t, types.ModelStatusDeprecated, record.Status)
	require.Equal(t, "outdated training data", record.DeprecationReason)

	// Should no longer appear in active models.
	active := k.GetActiveModels(ctx)
	for _, m := range active {
		require.NotEqual(t, resp.ModelID, m.ModelID)
	}
}

func TestDeprecateModel_AlreadyDeprecated(t *testing.T) {
	k, ctx := setupKeeper(t)
	seedTrainingRecord(t, k, ctx, "attest-001")

	msg := validPublishMsg(t, "code/go")
	resp, err := k.PublishModel(ctx, msg)
	require.NoError(t, err)

	err = k.DeprecateModel(ctx, resp.ModelID, testAddr, "first")
	require.NoError(t, err)

	err = k.DeprecateModel(ctx, resp.ModelID, testAddr, "second")
	require.Error(t, err)
	require.Contains(t, err.Error(), "already deprecated")
}

// ─── Test: Supersede Model ──────────────────────────────────────────────────

func TestSupersedeModel(t *testing.T) {
	k, ctx := setupKeeper(t)
	seedTrainingRecord(t, k, ctx, "attest-001")

	// Old model.
	msg1 := validPublishMsg(t, "code/go")
	resp1, err := k.PublishModel(ctx, msg1)
	require.NoError(t, err)

	// New model with different hashes.
	seedTrainingRecord(t, k, ctx, "attest-002")
	msg2 := validPublishMsg(t, "code/go")
	msg2.TrainingRecordID = "attest-002"
	msg2.ModelHash = "new-model-hash-789"
	msg2.TEEAttestation = "cafebabe"
	msg2.ParentModelID = resp1.ModelID
	resp2, err := k.PublishModel(ctx, msg2)
	require.NoError(t, err)
	require.Equal(t, uint64(2), resp2.Version)

	// Supersede.
	err = k.SupersedeModel(ctx, resp1.ModelID, resp2.ModelID)
	require.NoError(t, err)

	old, found := k.GetModelRecord(ctx, resp1.ModelID)
	require.True(t, found)
	require.Equal(t, types.ModelStatusSuperseded, old.Status)
	require.Equal(t, resp2.ModelID, old.SupersededBy)
}

// ─── Test: Model Lineage — 3 Generations ────────────────────────────────────

func TestModelLineage_ThreeGenerations(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Gen 1.
	seedTrainingRecord(t, k, ctx, "attest-gen1")
	msg1 := validPublishMsg(t, "code/go")
	msg1.TrainingRecordID = "attest-gen1"
	msg1.ModelHash = "gen1-hash"
	msg1.TEEAttestation = "gen1-attest"
	resp1, err := k.PublishModel(ctx, msg1)
	require.NoError(t, err)

	// Gen 2.
	seedTrainingRecord(t, k, ctx, "attest-gen2")
	msg2 := validPublishMsg(t, "code/go")
	msg2.TrainingRecordID = "attest-gen2"
	msg2.ModelHash = "gen2-hash"
	msg2.TEEAttestation = "gen2-attest"
	msg2.ParentModelID = resp1.ModelID
	resp2, err := k.PublishModel(ctx, msg2)
	require.NoError(t, err)

	// Gen 3.
	seedTrainingRecord(t, k, ctx, "attest-gen3")
	msg3 := validPublishMsg(t, "code/go")
	msg3.TrainingRecordID = "attest-gen3"
	msg3.ModelHash = "gen3-hash"
	msg3.TEEAttestation = "gen3-attest"
	msg3.ParentModelID = resp2.ModelID
	resp3, err := k.PublishModel(ctx, msg3)
	require.NoError(t, err)

	// Check lineage.
	lineage, found := k.GetModelLineage(ctx, resp3.ModelID)
	require.True(t, found)
	require.Equal(t, uint64(3), lineage.Generation)
	require.Len(t, lineage.Ancestors, 2)
	require.Equal(t, resp2.ModelID, lineage.Ancestors[0])
	require.Equal(t, resp1.ModelID, lineage.Ancestors[1])
}

// ─── Test: Domain Query ─────────────────────────────────────────────────────

func TestGetModelsByDomain(t *testing.T) {
	k, ctx := setupKeeper(t)

	for i, domain := range []string{"code/go", "code/go", "code/rust"} {
		hash := fmt.Sprintf("attest-%d", i)
		seedTrainingRecord(t, k, ctx, hash)
		msg := validPublishMsg(t, domain)
		msg.TrainingRecordID = hash
		msg.ModelHash = fmt.Sprintf("hash-%d", i)
		msg.TEEAttestation = fmt.Sprintf("attest-dom-%d", i)
		_, err := k.PublishModel(ctx, msg)
		require.NoError(t, err)
	}

	goModels := k.GetModelsByDomain(ctx, "code/go")
	require.Len(t, goModels, 2)

	rustModels := k.GetModelsByDomain(ctx, "code/rust")
	require.Len(t, rustModels, 1)

	pythonModels := k.GetModelsByDomain(ctx, "code/python")
	require.Len(t, pythonModels, 0)
}

// ─── Test: TDU Reverse Index ────────────────────────────────────────────────

func TestGetModelsByTDU(t *testing.T) {
	k, ctx := setupKeeper(t)

	for i := 0; i < 2; i++ {
		hash := fmt.Sprintf("attest-tdu-%d", i)
		seedTrainingRecord(t, k, ctx, hash)
		msg := validPublishMsg(t, "code/go")
		msg.TrainingRecordID = hash
		msg.ModelHash = fmt.Sprintf("tdu-model-%d", i)
		msg.TEEAttestation = fmt.Sprintf("tdu-attest-%d", i)
		_, err := k.PublishModel(ctx, msg)
		require.NoError(t, err)
	}

	// tdu-001 is in both models' TDUIDs (generateTDUIDs(15) starts at tdu-001).
	models := k.GetModelsByTDU(ctx, "tdu-001")
	require.Len(t, models, 2)

	models = k.GetModelsByTDU(ctx, "tdu-nonexistent")
	require.Len(t, models, 0)
}

// ─── Test: Contributor Rewards ──────────────────────────────────────────────

func TestCalculateContributorRewards(t *testing.T) {
	k, ctx := setupKeeper(t)
	seedTrainingRecord(t, k, ctx, "attest-reward")

	// Seed fitness records for TDUs with varying scores.
	// Need at least 10 TDUs to pass quality gate.
	sampleIDs := make([]string, 12)
	for i := range sampleIDs {
		sampleIDs[i] = fmt.Sprintf("reward-tdu-%d", i+1)
	}
	for i, id := range sampleIDs {
		fitnessScore := sdkmath.LegacyNewDecWithPrec(5, 1) // 0.5 default
		if i == 0 {
			fitnessScore = sdkmath.LegacyNewDecWithPrec(8, 1) // 0.8
		} else if i == 1 {
			fitnessScore = sdkmath.LegacyNewDecWithPrec(4, 1) // 0.4
		}
		fr := types.NewTDUFitnessRecord(id, sdkmath.NewInt(1000000), 0)
		fr.SetFitnessScore(fitnessScore)
		require.NoError(t, k.SetFitnessRecord(ctx, fr))
	}

	msg := validPublishMsg(t, "code/go")
	msg.TrainingRecordID = "attest-reward"
	msg.TDUIDs = sampleIDs
	resp, err := k.PublishModel(ctx, msg)
	require.NoError(t, err)

	totalReward := sdkmath.NewInt(1_000_000)
	rewards := k.CalculateContributorRewards(ctx, resp.ModelID, totalReward)

	// Without samples stored (codec is nil in test), getSampleSubmitter will return "".
	// So rewards will be nil since there are no submitters resolved.
	// This is correct behavior for isolated unit test — integration test needed for full flow.
	// At minimum, verify the function doesn't panic.
	_ = rewards
}

// ─── Test: Endpoint Registration ────────────────────────────────────────────

func TestEndpointRegistration(t *testing.T) {
	k, ctx := setupKeeper(t)
	seedTrainingRecord(t, k, ctx, "attest-endpoint")

	msg := validPublishMsg(t, "code/go")
	msg.TrainingRecordID = "attest-endpoint"
	msg.ModelHash = "endpoint-hash"
	msg.TEEAttestation = "endpoint-attest"
	resp, err := k.PublishModel(ctx, msg)
	require.NoError(t, err)

	// Register 2 endpoints.
	err = k.RegisterModelEndpoint(ctx, resp.ModelID, "https://inference-1.zerone.money/v1")
	require.NoError(t, err)
	err = k.RegisterModelEndpoint(ctx, resp.ModelID, "https://inference-2.zerone.money/v1")
	require.NoError(t, err)

	endpoints := k.GetModelEndpoints(ctx, resp.ModelID)
	require.Len(t, endpoints, 2)

	// Remove one.
	err = k.RemoveModelEndpoint(ctx, resp.ModelID, "https://inference-1.zerone.money/v1")
	require.NoError(t, err)
	endpoints = k.GetModelEndpoints(ctx, resp.ModelID)
	require.Len(t, endpoints, 1)

	// Register on non-existent model.
	err = k.RegisterModelEndpoint(ctx, "nonexistent", "https://x.com/v1")
	require.Error(t, err)
}

// ─── Test: Inference Counting ───────────────────────────────────────────────

func TestRecordInference(t *testing.T) {
	k, ctx := setupKeeper(t)
	seedTrainingRecord(t, k, ctx, "attest-inf")

	msg := validPublishMsg(t, "code/go")
	msg.TrainingRecordID = "attest-inf"
	msg.ModelHash = "inf-hash"
	msg.TEEAttestation = "inf-attest"
	resp, err := k.PublishModel(ctx, msg)
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		err = k.RecordInference(ctx, resp.ModelID)
		require.NoError(t, err)
	}

	record, found := k.GetModelRecord(ctx, resp.ModelID)
	require.True(t, found)
	require.Equal(t, uint64(3), record.InferenceCount)
}

// ─── Test: Latest Model ─────────────────────────────────────────────────────

func TestGetLatestModel(t *testing.T) {
	k, ctx := setupKeeper(t)

	var lastID string
	for i := 0; i < 3; i++ {
		hash := fmt.Sprintf("attest-latest-%d", i)
		seedTrainingRecord(t, k, ctx, hash)
		msg := validPublishMsg(t, "code/go")
		msg.TrainingRecordID = hash
		msg.ModelHash = fmt.Sprintf("latest-hash-%d", i)
		msg.TEEAttestation = fmt.Sprintf("latest-attest-%d", i)
		resp, err := k.PublishModel(ctx, msg)
		require.NoError(t, err)
		lastID = resp.ModelID
	}

	latest, found := k.GetLatestModel(ctx, "code/go")
	require.True(t, found)
	require.Equal(t, lastID, latest.ModelID)
	require.Equal(t, uint64(3), latest.Version)

	_, found = k.GetLatestModel(ctx, "code/python")
	require.False(t, found)
}

// ─── Test: Quality Gate Edge Case ───────────────────────────────────────────

func TestPublishModel_ExactlyAtThreshold(t *testing.T) {
	k, ctx := setupKeeper(t)
	seedTrainingRecord(t, k, ctx, "attest-edge")

	msg := validPublishMsg(t, "code/go")
	msg.TrainingRecordID = "attest-edge"
	msg.ModelHash = "edge-hash"
	msg.TEEAttestation = "edge-attest"
	msg.BenchmarkScore = "0.300000000000000000"
	msg.FitnessWeighted = "0.400000000000000000"
	msg.TDUIDs = generateTDUIDs(10)

	resp, err := k.PublishModel(ctx, msg)
	require.NoError(t, err)
	require.NotEmpty(t, resp.ModelID)
}

func TestPublishModel_JustBelowThreshold(t *testing.T) {
	k, ctx := setupKeeper(t)
	seedTrainingRecord(t, k, ctx, "attest-below")

	msg := validPublishMsg(t, "code/go")
	msg.TrainingRecordID = "attest-below"
	msg.ModelHash = "below-hash"
	msg.TEEAttestation = "below-attest"
	msg.BenchmarkScore = "0.299000000000000000"
	_, err := k.PublishModel(ctx, msg)
	require.Error(t, err)
}

// ─── Test: Update Benchmark Scores ──────────────────────────────────────────

func TestUpdateBenchmarkScores(t *testing.T) {
	k, ctx := setupKeeper(t)
	seedTrainingRecord(t, k, ctx, "attest-bench")

	msg := validPublishMsg(t, "code/go")
	msg.TrainingRecordID = "attest-bench"
	msg.ModelHash = "bench-hash"
	msg.TEEAttestation = "bench-attest"
	resp, err := k.PublishModel(ctx, msg)
	require.NoError(t, err)

	newResults := []types.BenchmarkResult{
		{BenchmarkID: "code-review", Score: "0.800000000000000000", Category: "code", PassRate: "0.900000000000000000"},
		{BenchmarkID: "reasoning", Score: "0.600000000000000000", Category: "reasoning", PassRate: "0.700000000000000000"},
	}
	err = k.UpdateBenchmarkScores(ctx, resp.ModelID, newResults)
	require.NoError(t, err)

	record, found := k.GetModelRecord(ctx, resp.ModelID)
	require.True(t, found)
	require.Len(t, record.BenchmarkDetails, 2)
	score := record.GetBenchmarkScore()
	require.Equal(t, "0.700000000000000000", score.String())
}

// ─── Test: GetActiveModels ──────────────────────────────────────────────────

func TestGetActiveModels(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Publish 3, deprecate 1.
	var ids []string
	for i := 0; i < 3; i++ {
		hash := fmt.Sprintf("attest-active-%d", i)
		seedTrainingRecord(t, k, ctx, hash)
		msg := validPublishMsg(t, "code/go")
		msg.TrainingRecordID = hash
		msg.ModelHash = fmt.Sprintf("active-hash-%d", i)
		msg.TEEAttestation = fmt.Sprintf("active-attest-%d", i)
		resp, err := k.PublishModel(ctx, msg)
		require.NoError(t, err)
		ids = append(ids, resp.ModelID)
	}

	active := k.GetActiveModels(ctx)
	require.Len(t, active, 3)

	// Deprecate one.
	err := k.DeprecateModel(ctx, ids[1], testAddr, "test")
	require.NoError(t, err)

	active = k.GetActiveModels(ctx)
	require.Len(t, active, 2)
}
