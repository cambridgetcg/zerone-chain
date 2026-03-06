package agentsdk

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Mock chain client ──────────────────────────────────────────────────────

type mockChainClient struct {
	mu           sync.Mutex
	address      string
	params       *types.Params
	broadcasts   []interface{}
	samples      map[string]*types.Sample
	rounds       map[string]*types.QualityRound
	domains      []*types.Domain
	bounties     []*types.DataBounty
	activeRounds uint64
	broadcastErr error
}

func newMockChain(address string) *mockChainClient {
	return &mockChainClient{
		address: address,
		params: &types.Params{
			MinSubmissionStake: "1000000",
		},
		samples:  make(map[string]*types.Sample),
		rounds:   make(map[string]*types.QualityRound),
		domains:  []*types.Domain{{Name: "code"}, {Name: "math"}},
		bounties: nil,
	}
}

func (m *mockChainClient) BroadcastMsg(_ context.Context, msg interface{}) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.broadcastErr != nil {
		return "", m.broadcastErr
	}
	m.broadcasts = append(m.broadcasts, msg)
	return fmt.Sprintf("TX_%d", len(m.broadcasts)), nil
}

func (m *mockChainClient) QueryParams(_ context.Context) (*types.Params, error) {
	return m.params, nil
}

func (m *mockChainClient) QuerySamplesBySubmitter(_ context.Context, submitter string) ([]*types.Sample, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*types.Sample
	for _, s := range m.samples {
		if s.Submitter == submitter {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *mockChainClient) QuerySample(_ context.Context, id string) (*types.Sample, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.samples[id]
	if !ok {
		return nil, fmt.Errorf("sample %s not found", id)
	}
	return s, nil
}

func (m *mockChainClient) QueryQualityRound(_ context.Context, id string) (*types.QualityRound, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rounds[id]
	if !ok {
		return nil, fmt.Errorf("round %s not found", id)
	}
	return r, nil
}

func (m *mockChainClient) QueryDomains(_ context.Context) ([]*types.Domain, error) {
	return m.domains, nil
}

func (m *mockChainClient) QueryDataBounties(_ context.Context, domain string) ([]*types.DataBounty, error) {
	if domain == "" {
		return m.bounties, nil
	}
	var result []*types.DataBounty
	for _, b := range m.bounties {
		if b.Domain == domain {
			result = append(result, b)
		}
	}
	return result, nil
}

func (m *mockChainClient) QueryProtocolStats(_ context.Context) (uint64, error) {
	return m.activeRounds, nil
}

func (m *mockChainClient) GetAddress() string {
	return m.address
}

func (m *mockChainClient) lastBroadcast() interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.broadcasts) == 0 {
		return nil
	}
	return m.broadcasts[len(m.broadcasts)-1]
}

// ─── Test helpers ───────────────────────────────────────────────────────────

func testClient(t *testing.T) (*ToKClient, *mockChainClient) {
	t.Helper()
	tmpDir := t.TempDir()
	mock := newMockChain("zerone1agent1testaddr")
	client := NewToKClientWithChain(Config{
		KeyringDir: tmpDir,
		MaxRetries: 1,
		RetryDelay: time.Millisecond,
	}, mock)
	return client, mock
}

// ─── Tests ──────────────────────────────────────────────────────────────────

func TestSubmitData_BuildsCorrectMsg(t *testing.T) {
	client, mock := testClient(t)
	ctx := context.Background()

	result, err := client.SubmitData(ctx, SubmitRequest{
		Type:        TypeInstructionResponse,
		Domain:      "code",
		Difficulty:  DifficultyStandard,
		Content:     `{"prompt":"hello","response":"world"}`,
		ConsentType: ConsentOriginal,
		Tags:        []string{"test"},
		Language:    "en",
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.TxHash)
	require.NotEmpty(t, result.ContentHash)
	require.Equal(t, "1500000", result.Stake) // 1000000 × 1.5

	// Verify the broadcast message
	msg := mock.lastBroadcast().(*types.MsgSubmitData)
	require.Equal(t, "zerone1agent1testaddr", msg.Submitter)
	require.Equal(t, types.SampleType_SAMPLE_TYPE_Q_AND_A, msg.SampleType)
	require.Equal(t, "code", msg.Domain)
	require.Equal(t, "1500000", msg.Stake)
	require.Equal(t, types.ConsentType_CONSENT_TYPE_SELF_AUTHORED, msg.Consent.Type)
	require.Equal(t, []string{"test"}, msg.Tags)
	require.Equal(t, "en", msg.Language)
}

func TestSubmitData_ContentHash(t *testing.T) {
	client, _ := testClient(t)
	ctx := context.Background()

	content := "test content for hashing"
	result, err := client.SubmitData(ctx, SubmitRequest{
		Type:    TypeConversation,
		Domain:  "general",
		Content: content,
	})

	require.NoError(t, err)
	expectedHash := contentHashHex([]byte(content))
	require.Equal(t, expectedHash, result.ContentHash)
}

func TestSubmitData_Validation(t *testing.T) {
	client, _ := testClient(t)
	ctx := context.Background()

	// Missing content
	_, err := client.SubmitData(ctx, SubmitRequest{Type: TypeConversation, Domain: "code"})
	require.ErrorContains(t, err, "content is required")

	// Missing domain
	_, err = client.SubmitData(ctx, SubmitRequest{Type: TypeConversation, Content: "x"})
	require.ErrorContains(t, err, "domain is required")

	// Missing type
	_, err = client.SubmitData(ctx, SubmitRequest{Domain: "code", Content: "x"})
	require.ErrorContains(t, err, "type is required")

	// Invalid type
	_, err = client.SubmitData(ctx, SubmitRequest{Type: "invalid", Domain: "code", Content: "x"})
	require.ErrorContains(t, err, "unknown TDU type")
}

func TestSubmitThread(t *testing.T) {
	client, mock := testClient(t)
	ctx := context.Background()

	result, err := client.SubmitThread(ctx, ThreadSubmitRequest{
		Domain: "code",
		Turns: []ThreadTurn{
			{Role: "user", Content: "How do I sort?"},
			{Role: "assistant", Content: "Use sort.Slice()"},
			{Role: "user", Content: "Thanks!"},
		},
		Difficulty:  DifficultyBasic,
		ConsentType: ConsentOriginal,
		ThreadID:    "thread-123",
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.TxHash)
	require.Equal(t, "1000000", result.Stake) // basic = 1×
	require.Equal(t, "thread-123", result.ThreadID)

	msg := mock.lastBroadcast().(*types.MsgSubmitThread)
	require.Equal(t, 3, len(msg.Items))
	require.Equal(t, "code", msg.Domain)
	require.Equal(t, "thread-123", msg.ThreadId)
}

func TestSubmitThread_MinTurns(t *testing.T) {
	client, _ := testClient(t)
	ctx := context.Background()

	_, err := client.SubmitThread(ctx, ThreadSubmitRequest{
		Domain: "code",
		Turns:  []ThreadTurn{{Role: "user", Content: "solo"}},
	})
	require.ErrorContains(t, err, "at least 2 turns")
}

func TestSubmitCorrection(t *testing.T) {
	client, mock := testClient(t)
	ctx := context.Background()

	result, err := client.SubmitCorrection(ctx, CorrectionRequest{
		TargetID: "sample-abc",
		Content:  `{"corrected": "fixed code"}`,
		Reason:   "Bug in original example",
		Domain:   "code",
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.TxHash)
	// Default difficulty for corrections is "standard" = 1.5×
	require.Equal(t, "1500000", result.Stake)

	msg := mock.lastBroadcast().(*types.MsgSubmitData)
	require.Equal(t, types.SampleType_SAMPLE_TYPE_CORRECTION, msg.SampleType)
	require.Equal(t, "sample-abc", msg.ParentSubmissionId)
	require.Contains(t, msg.Tags, "correction")
	require.Contains(t, msg.Tags, "reason:Bug in original example")
}

func TestCommitReview_ComputesCorrectSeal(t *testing.T) {
	client, mock := testClient(t)
	ctx := context.Background()

	score := ReviewScore{
		OverallQuality:  800000,
		ReasoningDepth:  700000,
		Novelty:         600000,
		Toxicity:        50000,
		FactualAccuracy: 900000,
		ConsentValid:    true,
		Duplicate:       false,
		Notes:           "good sample",
	}

	result, err := client.CommitReview(ctx, "round-42", score)
	require.NoError(t, err)
	require.NotEmpty(t, result.TxHash)

	// Verify the commitment message
	msg := mock.lastBroadcast().(*types.MsgSubmitCommitment)
	require.Equal(t, "zerone1agent1testaddr", msg.Verifier)
	require.Equal(t, "round-42", msg.RoundId)
	require.NotEmpty(t, msg.CommitHash)

	// Verify salt was stored
	salt, err := client.salts.Load("round-42")
	require.NoError(t, err)
	require.Len(t, salt, 32)

	// Verify score was stored
	storedScore, err := client.salts.LoadScore("round-42")
	require.NoError(t, err)
	require.Equal(t, score, *storedScore)

	// Verify the commitment hash matches
	vote := scoreToVote(score)
	expectedHash := types.ComputeQualityCommitHash("round-42", vote, salt)
	require.Equal(t, expectedHash, msg.CommitHash)
}

func TestRevealReview_LoadsSaltAndBuildsMsg(t *testing.T) {
	client, mock := testClient(t)
	ctx := context.Background()

	score := ReviewScore{
		OverallQuality:  750000,
		ReasoningDepth:  650000,
		ConsentValid:    true,
	}

	// First commit (stores salt + score)
	_, err := client.CommitReview(ctx, "round-99", score)
	require.NoError(t, err)

	// Load salt to compare
	salt, err := client.salts.Load("round-99")
	require.NoError(t, err)

	// Now reveal
	result, err := client.RevealReview(ctx, "round-99", score)
	require.NoError(t, err)
	require.NotEmpty(t, result.TxHash)

	msg := mock.lastBroadcast().(*types.MsgSubmitReveal)
	require.Equal(t, "zerone1agent1testaddr", msg.Verifier)
	require.Equal(t, "round-99", msg.RoundId)
	require.Equal(t, salt, msg.Salt)
	require.Equal(t, uint64(750000), msg.Scores.OverallQuality)
	require.Equal(t, uint64(650000), msg.Scores.ReasoningDepth)
	require.True(t, msg.Scores.ConsentValid)

	// Salt should be cleaned up after reveal
	_, err = client.salts.Load("round-99")
	require.Error(t, err)
}

func TestAutoRevealAll_FindsAndReveals(t *testing.T) {
	client, mock := testClient(t)
	ctx := context.Background()

	// Set up a round in reveal phase
	mock.rounds["round-A"] = &types.QualityRound{
		Id:           "round-A",
		SubmissionId: "sub-1",
		Phase:        types.VerificationPhase_VERIFICATION_PHASE_REVEAL,
		Commits: []*types.CommitEntry{
			{Verifier: "zerone1agent1testaddr", CommitHash: []byte("test")},
		},
	}

	// Set up a round NOT in reveal phase (should be skipped)
	mock.rounds["round-B"] = &types.QualityRound{
		Id:    "round-B",
		Phase: types.VerificationPhase_VERIFICATION_PHASE_COMMIT,
	}

	// Commit to both rounds
	scoreA := ReviewScore{OverallQuality: 800000, ConsentValid: true}
	_, err := client.CommitReview(ctx, "round-A", scoreA)
	require.NoError(t, err)

	scoreB := ReviewScore{OverallQuality: 600000, ConsentValid: true}
	_, err = client.CommitReview(ctx, "round-B", scoreB)
	require.NoError(t, err)

	// Auto-reveal all
	revealed, errs := client.AutoRevealAll(ctx)
	require.Equal(t, 1, revealed)     // Only round-A should be revealed
	require.Empty(t, errs)

	// round-A salt should be cleaned up
	_, err = client.salts.Load("round-A")
	require.Error(t, err)

	// round-B salt should still exist
	_, err = client.salts.Load("round-B")
	require.NoError(t, err)
}

func TestSaltRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewSaltStore(tmpDir)

	// Generate and store
	salt, err := store.GenerateAndStore("round-test-1")
	require.NoError(t, err)
	require.Len(t, salt, 32)

	// Load
	loaded, err := store.Load("round-test-1")
	require.NoError(t, err)
	require.Equal(t, salt, loaded)

	// Verify hex encoding on disk
	data, err := os.ReadFile(filepath.Join(tmpDir, "review-salts", "round-test-1.salt"))
	require.NoError(t, err)
	require.Equal(t, hex.EncodeToString(salt), string(data))

	// Score round-trip
	score := ReviewScore{OverallQuality: 500000, ConsentValid: true, Notes: "test"}
	err = store.StoreScore("round-test-1", score)
	require.NoError(t, err)

	loadedScore, err := store.LoadScore("round-test-1")
	require.NoError(t, err)
	require.Equal(t, score, *loadedScore)

	// List pending
	pending, err := store.ListPending()
	require.NoError(t, err)
	require.Equal(t, []string{"round-test-1"}, pending)

	// Delete
	err = store.Delete("round-test-1")
	require.NoError(t, err)

	_, err = store.Load("round-test-1")
	require.Error(t, err)

	pending, err = store.ListPending()
	require.NoError(t, err)
	require.Empty(t, pending)
}

func TestDashboard_AggregatesCorrectly(t *testing.T) {
	client, mock := testClient(t)
	ctx := context.Background()

	// Set up diverse samples
	mock.samples["s1"] = &types.Sample{
		Id: "s1", Submitter: "zerone1agent1testaddr",
		Domain: "code", Status: types.SampleStatus_SAMPLE_STATUS_GOLD,
	}
	mock.samples["s2"] = &types.Sample{
		Id: "s2", Submitter: "zerone1agent1testaddr",
		Domain: "code", Status: types.SampleStatus_SAMPLE_STATUS_SILVER,
	}
	mock.samples["s3"] = &types.Sample{
		Id: "s3", Submitter: "zerone1agent1testaddr",
		Domain: "code", Status: types.SampleStatus_SAMPLE_STATUS_REJECTED,
	}
	mock.samples["s4"] = &types.Sample{
		Id: "s4", Submitter: "zerone1agent1testaddr",
		Domain: "math", Status: types.SampleStatus_SAMPLE_STATUS_PENDING,
	}
	mock.samples["s5"] = &types.Sample{
		Id: "s5", Submitter: "zerone1agent1testaddr",
		Domain: "math", Status: types.SampleStatus_SAMPLE_STATUS_IN_REVIEW,
	}
	mock.samples["other"] = &types.Sample{
		Id: "other", Submitter: "zerone1other",
		Domain: "code", Status: types.SampleStatus_SAMPLE_STATUS_GOLD,
	}
	mock.activeRounds = 3

	dash, err := client.GetDashboard(ctx)
	require.NoError(t, err)
	require.Equal(t, "zerone1agent1testaddr", dash.Address)
	require.Equal(t, 5, dash.TotalSubmissions) // excludes "other"
	require.Equal(t, 2, dash.AcceptedCount)     // gold + silver
	require.Equal(t, 1, dash.RejectedCount)
	require.Equal(t, 2, dash.PendingCount)      // pending + in_review
	require.InDelta(t, 40.0, dash.AcceptanceRate, 0.1) // 2/5 × 100
	require.Len(t, dash.ActiveStakes, 2)        // pending + in_review
	require.Equal(t, 3, dash.PendingReviews)
}

func TestStakeCalculation_MatchesChainParams(t *testing.T) {
	client, mock := testClient(t)
	ctx := context.Background()

	// Set custom min_submission_stake
	mock.params.MinSubmissionStake = "2000000" // 2 ZRN

	// Force cache invalidation
	client.mu.Lock()
	client.cachedParams = nil
	client.mu.Unlock()

	result, err := client.SubmitData(ctx, SubmitRequest{
		Type:       TypeInstructionResponse,
		Domain:     "code",
		Difficulty: DifficultyExpert, // 3×
		Content:    "test",
	})

	require.NoError(t, err)
	require.Equal(t, "6000000", result.Stake) // 2000000 × 3.0
}

func TestStakeCalculation_AllDifficulties(t *testing.T) {
	tests := []struct {
		difficulty string
		expected   string
	}{
		{DifficultyBasic, "1000000"},    // 1×
		{DifficultyStandard, "1500000"}, // 1.5×
		{DifficultyAdvanced, "2000000"}, // 2×
		{DifficultyExpert, "3000000"},   // 3×
		{DifficultyFrontier, "5000000"}, // 5×
	}

	for _, tc := range tests {
		t.Run(tc.difficulty, func(t *testing.T) {
			client, _ := testClient(t)
			ctx := context.Background()

			result, err := client.SubmitData(ctx, SubmitRequest{
				Type:       TypeConversation,
				Domain:     "general",
				Difficulty: tc.difficulty,
				Content:    "test",
			})
			require.NoError(t, err)
			require.Equal(t, tc.expected, result.Stake)
		})
	}
}

func TestListOpenBounties(t *testing.T) {
	client, mock := testClient(t)
	ctx := context.Background()

	mock.bounties = []*types.DataBounty{
		{Id: "b1", Domain: "code", Subject: "Go tutorials", RewardAmount: "10000000", Claimed: false},
		{Id: "b2", Domain: "code", Subject: "Python basics", RewardAmount: "5000000", Claimed: true},
		{Id: "b3", Domain: "math", Subject: "Linear algebra", RewardAmount: "8000000", Claimed: false},
	}

	// All domains, open only
	bounties, err := client.ListOpenBounties(ctx, "")
	require.NoError(t, err)
	require.Len(t, bounties, 2) // b1 and b3 (unclaimed)

	// Domain filter
	bounties, err = client.ListOpenBounties(ctx, "code")
	require.NoError(t, err)
	require.Len(t, bounties, 1)
	require.Equal(t, "b1", bounties[0].ID)
}

func TestContestSample(t *testing.T) {
	client, mock := testClient(t)
	ctx := context.Background()

	result, err := client.ContestSample(ctx, ContestRequest{
		SampleID:    "sample-xyz",
		Stake:       "2000000",
		Reason:      "Contains factual errors",
		ContestType: "quality",
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.TxHash)

	msg := mock.lastBroadcast().(*types.MsgContestSample)
	require.Equal(t, "zerone1agent1testaddr", msg.Challenger)
	require.Equal(t, "sample-xyz", msg.SampleId)
	require.Equal(t, "2000000", msg.Stake)
	require.Equal(t, "Contains factual errors", msg.Reason)
	require.Equal(t, types.ContestType_CONTEST_TYPE_QUALITY, msg.ContestType)
}

func TestSponsorSample(t *testing.T) {
	client, mock := testClient(t)
	ctx := context.Background()

	txHash, err := client.SponsorSample(ctx, SponsorRequest{
		SampleID:       "sample-abc",
		Amount:         "5000000",
		DurationBlocks: 100000,
	})

	require.NoError(t, err)
	require.NotEmpty(t, txHash)

	msg := mock.lastBroadcast().(*types.MsgSponsorSample)
	require.Equal(t, "zerone1agent1testaddr", msg.Sponsor)
	require.Equal(t, "sample-abc", msg.SampleId)
	require.Equal(t, "5000000", msg.Amount)
	require.Equal(t, uint64(100000), msg.DurationBlocks)
}

func TestBroadcastRetry(t *testing.T) {
	tmpDir := t.TempDir()
	mock := newMockChain("zerone1test")
	client := NewToKClientWithChain(Config{
		KeyringDir: tmpDir,
		MaxRetries: 3,
		RetryDelay: time.Millisecond,
	}, mock)
	ctx := context.Background()

	// Set broadcast to fail
	mock.broadcastErr = fmt.Errorf("connection refused")

	_, err := client.SubmitData(ctx, SubmitRequest{
		Type:    TypeConversation,
		Domain:  "code",
		Content: "test",
	})

	require.ErrorContains(t, err, "broadcast failed after 3 retries")
	require.ErrorContains(t, err, "connection refused")

	// Should have attempted 3 times
	require.Len(t, mock.broadcasts, 0) // no successful broadcasts
}

func TestThreadSafe(t *testing.T) {
	client, _ := testClient(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	errs := make(chan error, 20)

	// Concurrent submissions
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := client.SubmitData(ctx, SubmitRequest{
				Type:    TypeConversation,
				Domain:  "code",
				Content: fmt.Sprintf("concurrent test %d", i),
			})
			if err != nil {
				errs <- err
			}
		}(i)
	}

	// Concurrent salt operations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			roundID := fmt.Sprintf("round-%d", i)
			_, err := client.salts.GenerateAndStore(roundID)
			if err != nil {
				errs <- err
				return
			}
			_, err = client.salts.Load(roundID)
			if err != nil {
				errs <- err
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent error: %v", err)
	}
}

func TestNewToKClient_Validation(t *testing.T) {
	_, err := NewToKClient(Config{})
	require.ErrorContains(t, err, "NodeURL is required")

	_, err = NewToKClient(Config{NodeURL: "http://localhost:26657"})
	require.ErrorContains(t, err, "ChainID is required")

	_, err = NewToKClient(Config{NodeURL: "http://localhost:26657", ChainID: "test"})
	require.ErrorContains(t, err, "FromName is required")

	client, err := NewToKClient(Config{
		NodeURL:  "http://localhost:26657",
		ChainID:  "test",
		FromName: "agent1",
	})
	require.NoError(t, err)
	require.NotNil(t, client)
	require.Equal(t, 3, client.config.MaxRetries)
	require.Equal(t, 2*time.Second, client.config.RetryDelay)
}

func TestHelpers_DifficultyMultiplier(t *testing.T) {
	tests := []struct {
		input string
		want  int64
		err   bool
	}{
		{"", 10, false},
		{"basic", 10, false},
		{"standard", 15, false},
		{"advanced", 20, false},
		{"expert", 30, false},
		{"frontier", 50, false},
		{"invalid", 0, true},
	}

	for _, tc := range tests {
		got, err := difficultyMultiplier(tc.input)
		if tc.err {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		}
	}
}

func TestHelpers_CalculateStake(t *testing.T) {
	result, err := calculateStake("1000000", 15) // 1.5×
	require.NoError(t, err)
	require.Equal(t, "1500000", result)

	result, err = calculateStake("2000000", 30) // 3×
	require.NoError(t, err)
	require.Equal(t, "6000000", result)

	_, err = calculateStake("notanumber", 10)
	require.Error(t, err)
}

func TestHelpers_ParseTypes(t *testing.T) {
	// TDU types
	st, err := parseTDUType("instruction-response")
	require.NoError(t, err)
	require.Equal(t, types.SampleType_SAMPLE_TYPE_Q_AND_A, st)

	st, err = parseTDUType("conversation")
	require.NoError(t, err)
	require.Equal(t, types.SampleType_SAMPLE_TYPE_DISCUSSION, st)

	_, err = parseTDUType("invalid")
	require.Error(t, err)

	// Consent types
	ct, err := parseConsentType("original")
	require.NoError(t, err)
	require.Equal(t, types.ConsentType_CONSENT_TYPE_SELF_AUTHORED, ct)

	ct, err = parseConsentType("public-domain")
	require.NoError(t, err)
	require.Equal(t, types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE, ct)

	_, err = parseConsentType("invalid")
	require.Error(t, err)

	// Contest types
	ctt, err := parseContestType("quality")
	require.NoError(t, err)
	require.Equal(t, types.ContestType_CONTEST_TYPE_QUALITY, ctt)

	_, err = parseContestType("invalid")
	require.Error(t, err)
}
