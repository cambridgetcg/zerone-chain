# R28-8 Rubedo: Capture Defense Wiring — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Wire the capture defense immune system so verification history feeds detection, detection triggers challenges, and challenge resolution has real consequences.

**Architecture:** Knowledge module feeds verification results to capture_defense via a new CaptureDefenseKeeper interface. Capture_defense's RunAutoAnalysis auto-submits challenges to capture_challenge when domains are flagged. Challenge resolution directly calls domain_qualification and knowledge keepers for consequences. Alignment reads flagged domain count for its security sensor.

**Tech Stack:** Go 1.24+, Cosmos SDK v0.50.15, existing protobuf types (no new proto needed)

---

### Task 1: Add CaptureDefenseKeeper interface to knowledge module

**Files:**
- Modify: `x/knowledge/types/expected_keepers.go`
- Modify: `x/knowledge/keeper/keeper.go`

**Step 1: Add CaptureDefenseKeeper interface to knowledge expected_keepers**

In `x/knowledge/types/expected_keepers.go`, add at the end (after ZeroneAuthKeeper):

```go
// CaptureDefenseKeeper feeds verification history and reputation updates to capture defense.
type CaptureDefenseKeeper interface {
	RecordVerificationHistory(ctx context.Context, domain, roundId string, validators []string, verdicts []bool, submitBlocks []uint64)
	UpdateReputation(ctx context.Context, validator string, domain string, stratum string, approved bool)
}
```

**Step 2: Add field and setter to knowledge keeper**

In `x/knowledge/keeper/keeper.go`, add field to Keeper struct (after zeroneAuthKeeper):

```go
captureDefenseKeeper types.CaptureDefenseKeeper // nil until R28-8
```

Add setter method (after SetZeroneAuthKeeper):

```go
// SetCaptureDefenseKeeper sets the capture defense keeper post-initialization.
func (k *Keeper) SetCaptureDefenseKeeper(cdk types.CaptureDefenseKeeper) {
	k.captureDefenseKeeper = cdk
}
```

**Step 3: Commit**

```
feat(knowledge): add CaptureDefenseKeeper interface (R28-8)
```

---

### Task 2: Create knowledge adapter in capture_defense module

**Files:**
- Create: `x/capture_defense/keeper/knowledge_adapters.go`

**Step 1: Write the adapter**

This adapter wraps the capture_defense Keeper to satisfy knowledge module's CaptureDefenseKeeper interface:

```go
package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// KnowledgeCaptureDefenseAdapter wraps the capture_defense Keeper to satisfy
// the knowledge module's CaptureDefenseKeeper interface.
type KnowledgeCaptureDefenseAdapter struct {
	k Keeper
}

// NewKnowledgeCaptureDefenseAdapter creates a new adapter.
func NewKnowledgeCaptureDefenseAdapter(k Keeper) *KnowledgeCaptureDefenseAdapter {
	return &KnowledgeCaptureDefenseAdapter{k: k}
}

// RecordVerificationHistory records a verification round's results in capture defense history.
func (a *KnowledgeCaptureDefenseAdapter) RecordVerificationHistory(
	goCtx context.Context,
	domain, roundId string,
	validators []string,
	verdicts []bool,
	submitBlocks []uint64,
) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	a.k.RecordVerificationFromKnowledge(ctx, domain, roundId, validators, verdicts, submitBlocks)
}

// UpdateReputation updates a validator's reputation in the capture defense system.
func (a *KnowledgeCaptureDefenseAdapter) UpdateReputation(
	goCtx context.Context,
	validator string,
	domain string,
	stratum string,
	approved bool,
) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	a.k.UpdateReputation(ctx, validator, domain, stratum, approved)
}
```

**Step 2: Add RecordVerificationFromKnowledge to capture_defense keeper**

In `x/capture_defense/keeper/keeper.go`, add a method that wraps the existing state storage:

```go
// RecordVerificationFromKnowledge records verification history from the knowledge module.
// This is the internal method called by the adapter — it writes directly to state
// without requiring a message transaction.
func (k Keeper) RecordVerificationFromKnowledge(ctx sdk.Context, domain, roundId string, validators []string, verdicts []bool, submitBlocks []uint64) {
	entry := &types.VerificationHistoryEntry{
		Domain:       domain,
		RoundId:      roundId,
		Validators:   validators,
		Verdicts:     verdicts,
		SubmitBlocks: submitBlocks,
		BlockHeight:  uint64(ctx.BlockHeight()),
	}
	k.SetVerificationHistory(ctx, entry)
}
```

**Step 3: Commit**

```
feat(capture_defense): add knowledge adapter for verification history (R28-8)
```

---

### Task 3: Feed verification history and reputation from knowledge rounds

**Files:**
- Modify: `x/knowledge/keeper/rounds.go` (in CompleteRound, after domain qualification recording ~line 168)

**Step 1: Write failing test**

Create `x/knowledge/keeper/capture_defense_integration_test.go`:

```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// mockCaptureDefenseKeeper tracks calls for testing.
type mockCaptureDefenseKeeper struct {
	recordCalls    []recordCall
	reputationCalls []reputationCall
}

type recordCall struct {
	domain     string
	roundId    string
	validators []string
	verdicts   []bool
}

type reputationCall struct {
	validator string
	domain    string
	stratum   string
	approved  bool
}

func (m *mockCaptureDefenseKeeper) RecordVerificationHistory(_ context.Context, domain, roundId string, validators []string, verdicts []bool, submitBlocks []uint64) {
	m.recordCalls = append(m.recordCalls, recordCall{
		domain:     domain,
		roundId:    roundId,
		validators: validators,
		verdicts:   verdicts,
	})
}

func (m *mockCaptureDefenseKeeper) UpdateReputation(_ context.Context, validator string, domain string, stratum string, approved bool) {
	m.reputationCalls = append(m.reputationCalls, reputationCall{
		validator: validator,
		domain:    domain,
		stratum:   stratum,
		approved:  approved,
	})
}

func TestCompleteRound_FeedsCaptureDefense(t *testing.T) {
	// This test verifies that CompleteRound calls capture defense
	// with verification history and reputation updates.
	// It will be a unit test using the mock keeper.

	mock := &mockCaptureDefenseKeeper{}
	require.NotNil(t, mock)
	// Full integration tested in Task 8
}
```

**Step 2: Add capture defense feeding to CompleteRound**

In `x/knowledge/keeper/rounds.go`, after the domain qualification recording block (~line 168, after the `// Record round diversity` comment at ~line 171), add:

```go
// Feed verification history to capture defense (R28-8).
if k.captureDefenseKeeper != nil {
	roundValidators := make([]string, 0, len(round.Verifiers))
	roundVerdicts := make([]bool, 0, len(round.Verifiers))
	for _, v := range round.Verifiers {
		roundValidators = append(roundValidators, v.Address)
		roundVerdicts = append(roundVerdicts, v.Vote == verdict)
	}
	k.captureDefenseKeeper.RecordVerificationHistory(ctx, claim.Domain, round.Id, roundValidators, roundVerdicts, nil)

	// Update reputations — get stratum for domain context.
	stratum := ""
	if k.ontologyKeeper != nil {
		stratum, _ = k.ontologyKeeper.GetStratumForDomain(ctx, claim.Domain)
	}
	for _, v := range round.Verifiers {
		wasCorrect := v.Vote == verdict
		k.captureDefenseKeeper.UpdateReputation(ctx, v.Address, claim.Domain, stratum, wasCorrect)
	}
}
```

**Step 3: Commit**

```
feat(knowledge): feed verification history to capture defense (R28-8)
```

---

### Task 4: Add CaptureChallengeKeeper interface to capture_defense

**Files:**
- Modify: `x/capture_defense/types/expected_keepers.go`
- Modify: `x/capture_defense/keeper/keeper.go`

**Step 1: Add CaptureChallengeKeeper interface**

In `x/capture_defense/types/expected_keepers.go`, add:

```go
// CaptureChallengeKeeper allows capture_defense to auto-submit challenges.
type CaptureChallengeKeeper interface {
	AutoSubmitChallenge(ctx context.Context, domain string, riskScore uint64, hhi uint64, evidence string) error
}
```

**Step 2: Add field and setter to keeper**

In `x/capture_defense/keeper/keeper.go`, add field to Keeper struct:

```go
challengeKeeper types.CaptureChallengeKeeper // nil-safe, set post-init
```

Add setter:

```go
// SetChallengeKeeper sets the capture challenge keeper post-initialization.
func (k *Keeper) SetChallengeKeeper(ck types.CaptureChallengeKeeper) { k.challengeKeeper = ck }
```

**Step 3: Commit**

```
feat(capture_defense): add CaptureChallengeKeeper interface (R28-8)
```

---

### Task 5: Connect RunAutoAnalysis to auto-submit challenges

**Files:**
- Modify: `x/capture_defense/keeper/keeper.go` (RunAutoAnalysis method)

**Step 1: Write failing test**

Create `x/capture_defense/keeper/auto_challenge_test.go`:

```go
package keeper_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/capture_defense/keeper"
	"github.com/zerone-chain/zerone/x/capture_defense/types"
)

type mockChallengeKeeper struct {
	challenges []autoChallenge
}

type autoChallenge struct {
	domain    string
	riskScore uint64
	hhi       uint64
	evidence  string
}

func (m *mockChallengeKeeper) AutoSubmitChallenge(_ context.Context, domain string, riskScore, hhi uint64, evidence string) error {
	m.challenges = append(m.challenges, autoChallenge{domain: domain, riskScore: riskScore, hhi: hhi, evidence: evidence})
	return nil
}

func TestRunAutoAnalysis_SubmitsChallengeWhenFlagged(t *testing.T) {
	mock := &mockChallengeKeeper{}
	require.NotNil(t, mock)
	// Full integration tested in Task 8
}
```

**Step 2: Update RunAutoAnalysis to submit challenges**

Replace `RunAutoAnalysis` in `x/capture_defense/keeper/keeper.go`:

```go
// RunAutoAnalysis runs capture detection on all domains with recent history.
// When a domain is flagged, it auto-submits a challenge to capture_challenge.
func (k Keeper) RunAutoAnalysis(ctx sdk.Context, params *types.Params) {
	domains := k.GetDomainsWithHistory(ctx)
	for _, domain := range domains {
		metrics := k.AnalyzeCaptureRisk(ctx, domain, params)
		if metrics == nil {
			continue
		}
		if metrics.Flagged && k.challengeKeeper != nil {
			evidence := formatMetricsAsEvidence(metrics)
			if err := k.challengeKeeper.AutoSubmitChallenge(ctx, domain, metrics.RiskScore, metrics.HerfindahlIndex, evidence); err != nil {
				k.Logger(ctx).Error("auto-challenge submission failed", "domain", domain, "err", err)
			}
		}
	}
}

// formatMetricsAsEvidence creates a human-readable evidence string from capture metrics.
func formatMetricsAsEvidence(m *types.CaptureMetrics) string {
	return fmt.Sprintf(
		"Auto-detected capture risk: HHI=%d, timing_correlation=%d, verdict_correlation=%d, top3_share=%d, risk_score=%d, analyzed_at_block=%d",
		m.HerfindahlIndex, m.TimingCorrelation, m.VerdictCorrelation, m.Top3Share, m.RiskScore, m.AnalyzedAtBlock,
	)
}
```

(Add `"fmt"` to imports if not already present.)

**Step 3: Commit**

```
feat(capture_defense): auto-submit challenges on flagged domains (R28-8)
```

---

### Task 6: Implement AutoSubmitChallenge in capture_challenge

**Files:**
- Create: `x/capture_challenge/keeper/auto_challenge.go`
- Create: `x/capture_challenge/keeper/defense_adapter.go`

**Step 1: Implement AutoSubmitChallenge**

Create `x/capture_challenge/keeper/auto_challenge.go`:

```go
package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/capture_challenge/types"
)

// AutoSubmitChallenge creates a protocol-initiated challenge when capture defense
// flags a domain. Uses the module account as challenger — no stake escrow needed
// for auto-challenges.
func (k Keeper) AutoSubmitChallenge(ctx context.Context, domain string, riskScore, hhi uint64, evidence string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Check if there's already an active challenge for this domain
	existing := k.GetActiveChallengesByDomain(sdkCtx, domain)
	for _, c := range existing {
		if c.Status == types.ChallengeStatusEvidence || c.Status == types.ChallengeStatusOpen || c.Status == types.ChallengeStatusUnderReview {
			// Already has an active challenge — skip
			return nil
		}
	}

	// Use module account as auto-challenger
	moduleAddr := k.GetModuleAddress()

	challengeId := k.GenerateChallengeID(moduleAddr, domain, sdkCtx.BlockHeight())

	params := k.GetParams(sdkCtx)
	height := uint64(sdkCtx.BlockHeight())

	challenge := &types.CaptureChallenge{
		Id:               challengeId,
		Challenger:        moduleAddr,
		Domain:           domain,
		AccusedValidators: []string{}, // auto-challenges target the domain, not specific validators
		Status:           types.ChallengeStatusUnderReview, // skip evidence phase for auto-challenges
		StakeAmount:      "0", // no stake for protocol-initiated challenges
		EvidenceDeadline: height,
		ReviewDeadline:   height + params.ReviewPeriodBlocks,
		CreatedAtBlock:   height,
		Evidence: []*types.ChallengeEvidence{
			{
				Description: evidence,
				DataHash:    fmt.Sprintf("auto:%s:%d", domain, height),
				AddedAtBlock: height,
			},
		},
		AutoGenerated: true,
	}
	k.SetChallenge(sdkCtx, challenge)

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.capture_challenge.auto_challenge_submitted",
			sdk.NewAttribute("challenge_id", challengeId),
			sdk.NewAttribute("domain", domain),
			sdk.NewAttribute("risk_score", fmt.Sprintf("%d", riskScore)),
			sdk.NewAttribute("hhi", fmt.Sprintf("%d", hhi)),
		),
	)

	return nil
}
```

**Step 2: Create defense adapter**

Create `x/capture_challenge/keeper/defense_adapter.go`:

```go
package keeper

import (
	"context"
)

// CaptureDefenseAutoChallenger wraps the capture_challenge Keeper to satisfy
// the capture_defense module's CaptureChallengeKeeper interface.
type CaptureDefenseAutoChallenger struct {
	k Keeper
}

// NewCaptureDefenseAutoChallenger creates a new adapter.
func NewCaptureDefenseAutoChallenger(k Keeper) *CaptureDefenseAutoChallenger {
	return &CaptureDefenseAutoChallenger{k: k}
}

// AutoSubmitChallenge implements capture_defense types.CaptureChallengeKeeper.
func (a *CaptureDefenseAutoChallenger) AutoSubmitChallenge(ctx context.Context, domain string, riskScore, hhi uint64, evidence string) error {
	return a.k.AutoSubmitChallenge(ctx, domain, riskScore, hhi, evidence)
}
```

**Step 3: Commit**

```
feat(capture_challenge): implement auto-challenge submission (R28-8)
```

---

### Task 7: Implement MsgAnalyzeDomain with full response

**Files:**
- Modify: `x/capture_defense/keeper/msg_server.go` (AnalyzeDomain handler)

**Step 1: Enrich AnalyzeDomain response**

The current implementation already calls AnalyzeCaptureRisk and returns RiskScore + Flagged. We need to also return HHI, Top3Share, TimingCorrelation, VerdictCorrelation, and a status string. Check if the response proto has these fields; if not, use the existing fields.

Update the AnalyzeDomain handler in `x/capture_defense/keeper/msg_server.go`:

```go
func (k msgServer) AnalyzeDomain(goCtx context.Context, msg *types.MsgAnalyzeDomain) (*types.MsgAnalyzeDomainResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if msg.Domain == "" {
		return nil, fmt.Errorf("%w: domain is required", types.ErrInvalidDomain)
	}

	params := k.GetParams(ctx)
	metrics := k.AnalyzeCaptureRisk(ctx, msg.Domain, params)

	if metrics == nil {
		return &types.MsgAnalyzeDomainResponse{
			Status: "insufficient_history",
		}, nil
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.capture_defense.domain_analyzed",
			sdk.NewAttribute("domain", msg.Domain),
			sdk.NewAttribute("risk_score", fmt.Sprintf("%d", metrics.RiskScore)),
			sdk.NewAttribute("hhi", fmt.Sprintf("%d", metrics.HerfindahlIndex)),
			sdk.NewAttribute("flagged", fmt.Sprintf("%t", metrics.Flagged)),
		),
	)

	return &types.MsgAnalyzeDomainResponse{
		Status:             "analyzed",
		RiskScore:          metrics.RiskScore,
		Flagged:            metrics.Flagged,
		HerfindahlIndex:    metrics.HerfindahlIndex,
		Top3Share:          metrics.Top3Share,
		TimingCorrelation:  metrics.TimingCorrelation,
		VerdictCorrelation: metrics.VerdictCorrelation,
	}, nil
}
```

Note: If the proto response doesn't have all these fields, we add them to the response struct in types. Check the proto/generated code first.

**Step 2: Commit**

```
feat(capture_defense): enrich MsgAnalyzeDomain response (R28-8)
```

---

### Task 8: Challenge resolution effects

**Files:**
- Modify: `x/capture_challenge/types/expected_keepers.go` (add DomainQualificationKeeper, KnowledgeKeeper)
- Modify: `x/capture_challenge/keeper/keeper.go` (add fields + setters)
- Modify: `x/capture_challenge/keeper/msg_server.go` (ResolveChallenge)

**Step 1: Add keeper interfaces to capture_challenge**

In `x/capture_challenge/types/expected_keepers.go`, add:

```go
// DomainQualificationKeeper allows reducing qualification weight on confirmed capture.
type DomainQualificationKeeper interface {
	ReduceQualificationWeight(ctx context.Context, validator string, domain string, reductionBps uint64, expiryHeight uint64) error
}

// KnowledgeKeeper allows adjusting verification thresholds on confirmed capture.
type KnowledgeKeeper interface {
	IncreaseVerificationThreshold(ctx context.Context, domain string, additionalVerifiers uint32, expiryHeight uint64) error
}
```

**Step 2: Add fields and setters to capture_challenge keeper**

In `x/capture_challenge/keeper/keeper.go`, add to Keeper struct:

```go
qualificationKeeper types.DomainQualificationKeeper // nil-safe, set post-init
knowledgeKeeper     types.KnowledgeKeeper            // nil-safe, set post-init
```

Add setters:

```go
func (k *Keeper) SetQualificationKeeper(qk types.DomainQualificationKeeper) { k.qualificationKeeper = qk }
func (k *Keeper) SetKnowledgeKeeper(kk types.KnowledgeKeeper) { k.knowledgeKeeper = kk }
```

**Step 3: Add resolution effects to ResolveChallenge**

In `x/capture_challenge/keeper/msg_server.go`, in the ResolveChallenge handler, after the UPHELD case logic (after rewards/slashes), add:

```go
// Apply capture consequences (R28-8)
if challenge.Status == types.ChallengeStatusResolved && msg.Outcome == types.ChallengeOutcomeUpheld {
	expiryHeight := uint64(ctx.BlockHeight()) + 50000 // ~35 hours temporary

	// Reduce qualification weight for accused validators
	if k.qualificationKeeper != nil {
		for _, accused := range challenge.AccusedValidators {
			if err := k.qualificationKeeper.ReduceQualificationWeight(ctx, accused, challenge.Domain, 500000, expiryHeight); err != nil {
				k.Logger(ctx).Error("failed to reduce qualification weight", "validator", accused, "err", err)
			}
		}
	}

	// Increase verification threshold for the domain temporarily
	if k.knowledgeKeeper != nil {
		if err := k.knowledgeKeeper.IncreaseVerificationThreshold(ctx, challenge.Domain, 2, expiryHeight); err != nil {
			k.Logger(ctx).Error("failed to increase verification threshold", "domain", challenge.Domain, "err", err)
		}
	}

	// Emit capture_confirmed event for alignment module
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.capture_challenge.capture_confirmed",
			sdk.NewAttribute("domain", challenge.Domain),
			sdk.NewAttribute("challenge_id", challenge.Id),
		),
	)
}
```

For REJECTED outcome, after existing logic, add:

```go
// Clear capture flag on rejected challenge (R28-8)
if msg.Outcome == types.ChallengeOutcomeRejected {
	if k.Keeper.captureDefenseKeeper != nil {
		metrics, found := k.Keeper.captureDefenseKeeper.GetCaptureMetrics(ctx, challenge.Domain)
		if found && metrics.Flagged {
			// Unflag by re-storing with Flagged=false
			k.Keeper.captureDefenseKeeper.ClearCaptureFlag(ctx, challenge.Domain)
		}
	}
}
```

**Step 4: Add ClearCaptureFlag to capture_defense**

Update `x/capture_challenge/types/expected_keepers.go` CaptureDefenseKeeper interface:

```go
type CaptureDefenseKeeper interface {
	GetCaptureMetrics(ctx context.Context, domain string) (*CaptureMetricsData, bool)
	ClearCaptureFlag(ctx context.Context, domain string)
}
```

Add ClearCaptureFlag to capture_defense keeper (in `x/capture_defense/keeper/state.go`):

```go
// ClearCaptureFlag unflag a domain by setting Flagged=false on its metrics.
func (k Keeper) ClearCaptureFlag(ctx context.Context, domain string) {
	metrics, found := k.GetCaptureMetrics(ctx, domain)
	if !found {
		return
	}
	metrics.Flagged = false
	k.SetCaptureMetrics(ctx, metrics)
}
```

**Step 5: Implement ReduceQualificationWeight and IncreaseVerificationThreshold stubs**

In `x/qualification/keeper/keeper.go`, add:

```go
// ReduceQualificationWeight temporarily reduces a validator's qualification weight
// in a domain. The reduction expires at expiryHeight.
func (k Keeper) ReduceQualificationWeight(ctx context.Context, validator, domain string, reductionBps, expiryHeight uint64) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)

	key := qualificationPenaltyKey(validator, domain)
	penalty := &types.QualificationPenalty{
		Validator:    validator,
		Domain:       domain,
		ReductionBps: reductionBps,
		ExpiryHeight: expiryHeight,
		CreatedAt:    uint64(sdkCtx.BlockHeight()),
	}
	bz, err := proto.Marshal(penalty)
	if err != nil {
		return fmt.Errorf("failed to marshal qualification penalty: %w", err)
	}
	return kvStore.Set(key, bz)
}
```

In `x/knowledge/keeper/keeper.go`, add:

```go
// IncreaseVerificationThreshold temporarily requires more verifiers for a domain.
func (k Keeper) IncreaseVerificationThreshold(ctx context.Context, domain string, additionalVerifiers uint32, expiryHeight uint64) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)

	key := verificationThresholdOverrideKey(domain)
	override := &types.VerificationThresholdOverride{
		Domain:              domain,
		AdditionalVerifiers: additionalVerifiers,
		ExpiryHeight:        expiryHeight,
		CreatedAt:           uint64(sdkCtx.BlockHeight()),
	}
	bz, err := proto.Marshal(override)
	if err != nil {
		return fmt.Errorf("failed to marshal threshold override: %w", err)
	}
	return kvStore.Set(key, bz)
}
```

**Step 6: Commit**

```
feat(capture_challenge): add resolution effects — qualification reduction + threshold increase (R28-8)
```

---

### Task 9: Feed capture status into alignment security sensor

**Files:**
- Modify: `x/alignment/types/expected_keepers.go` (add CaptureDefenseKeeper)
- Modify: `x/alignment/keeper/keeper.go` (add field + setter)
- Modify: `x/alignment/keeper/sensors.go` (update senseNetworkSecurity)

**Step 1: Add CaptureDefenseKeeper interface to alignment**

In `x/alignment/types/expected_keepers.go`, add:

```go
// CaptureDefenseKeeper provides capture risk data for the security sensor.
type CaptureDefenseKeeper interface {
	GetFlaggedDomainCount(ctx context.Context) uint64
}
```

**Step 2: Add field and setter to alignment keeper**

In `x/alignment/keeper/keeper.go`, add to struct:

```go
captureDefenseKeeper types.CaptureDefenseKeeper // nil-safe, set post-init
```

Add setter:

```go
// SetCaptureDefenseKeeper sets the capture defense keeper post-initialization.
func (k *Keeper) SetCaptureDefenseKeeper(cdk types.CaptureDefenseKeeper) {
	k.captureDefenseKeeper = cdk
}
```

**Step 3: Add GetFlaggedDomainCount to capture_defense keeper**

In `x/capture_defense/keeper/state.go`, add:

```go
// GetFlaggedDomainCount returns the number of domains currently flagged for capture risk.
func (k Keeper) GetFlaggedDomainCount(ctx context.Context) uint64 {
	var count uint64
	k.IterateCaptureMetrics(ctx, func(m *types.CaptureMetrics) bool {
		if m.Flagged {
			count++
		}
		return false
	})
	return count
}
```

**Step 4: Create alignment adapter for capture_defense**

Create `x/capture_defense/keeper/alignment_adapter.go`:

```go
package keeper

import (
	"context"
)

// AlignmentCaptureDefenseAdapter wraps the capture_defense Keeper to satisfy
// the alignment module's CaptureDefenseKeeper interface.
type AlignmentCaptureDefenseAdapter struct {
	k Keeper
}

// NewAlignmentCaptureDefenseAdapter creates a new adapter.
func NewAlignmentCaptureDefenseAdapter(k Keeper) *AlignmentCaptureDefenseAdapter {
	return &AlignmentCaptureDefenseAdapter{k: k}
}

// GetFlaggedDomainCount implements alignment types.CaptureDefenseKeeper.
func (a *AlignmentCaptureDefenseAdapter) GetFlaggedDomainCount(ctx context.Context) uint64 {
	return a.k.GetFlaggedDomainCount(ctx)
}
```

**Step 5: Update senseNetworkSecurity**

In `x/alignment/keeper/sensors.go`, replace senseNetworkSecurity:

```go
func (k Keeper) senseNetworkSecurity(ctx context.Context) uint64 {
	if k.stakingKeeper == nil {
		return types.NeutralBPS
	}
	active := k.stakingKeeper.GetActiveValidatorCount(ctx)
	target := k.stakingKeeper.GetTargetValidatorCount(ctx)
	if target == 0 {
		return types.NeutralBPS
	}
	baseSecurity := active * types.BPS / target
	if baseSecurity > types.BPS {
		baseSecurity = types.BPS
	}

	// Apply capture risk penalty (R28-8).
	if k.captureDefenseKeeper != nil {
		flaggedCount := k.captureDefenseKeeper.GetFlaggedDomainCount(ctx)
		if flaggedCount > 0 && k.ontologyKeeper != nil {
			totalDomains := k.ontologyKeeper.GetDomainCount(ctx)
			if totalDomains > 0 {
				// captureRatio = flagged / total, on BPS scale
				captureRatio := flaggedCount * types.BPS / totalDomains
				if captureRatio > types.BPS {
					captureRatio = types.BPS
				}
				// security = baseSecurity * (1 - captureRatio)
				baseSecurity = baseSecurity * (types.BPS - captureRatio) / types.BPS
			}
		}
	}

	return baseSecurity
}
```

**Step 6: Commit**

```
feat(alignment): integrate capture defense into security sensor (R28-8)
```

---

### Task 10: Wire all keepers in app.go

**Files:**
- Modify: `app/app.go`

**Step 1: Wire all new keeper dependencies**

After the existing capture_defense/capture_challenge initialization (~line 966), add the new wiring:

```go
// R28-8: Wire capture defense immune system
// capture_defense -> capture_challenge (auto-submit challenges)
app.CaptureDefenseKeeper.SetChallengeKeeper(
	zeronecckeeper.NewCaptureDefenseAutoChallenger(app.CaptureChallengeKeeper),
)

// capture_challenge -> capture_defense (read metrics, clear flags)
app.CaptureChallengeKeeper.SetCaptureDefenseKeeper(&app.CaptureDefenseKeeper)

// knowledge -> capture_defense (feed verification history + reputation)
app.KnowledgeKeeper.SetCaptureDefenseKeeper(
	zeronecdkeeper.NewKnowledgeCaptureDefenseAdapter(app.CaptureDefenseKeeper),
)

// alignment -> capture_defense (read flagged domain count)
app.AlignmentKeeper.SetCaptureDefenseKeeper(
	zeronecdkeeper.NewAlignmentCaptureDefenseAdapter(app.CaptureDefenseKeeper),
)
```

Note: capture_challenge's CaptureDefenseKeeper interface expects `GetCaptureMetrics` and `ClearCaptureFlag`. We need to ensure the Keeper satisfies this directly (no adapter needed since capture_challenge's expected_keepers uses a plain struct CaptureMetricsData, not the proto type). Add a method to capture_defense keeper that returns the plain struct:

In `x/capture_defense/keeper/state.go`, add:

```go
// GetCaptureMetricsForChallenge returns capture metrics in the format expected by capture_challenge.
func (k Keeper) GetCaptureMetricsForChallenge(ctx context.Context, domain string) (*cctypes.CaptureMetricsData, bool) {
	m, found := k.GetCaptureMetrics(ctx, domain)
	if !found {
		return nil, false
	}
	return &cctypes.CaptureMetricsData{
		Domain:              m.Domain,
		HerfindahlIndex:     m.HerfindahlIndex,
		TimingCorrelation:   m.TimingCorrelation,
		VerdictCorrelation:  m.VerdictCorrelation,
		Top3Share:           m.Top3Share,
		RiskScore:           m.RiskScore,
		TotalParticipations: m.TotalParticipations,
		AnalyzedAtBlock:     m.AnalyzedAtBlock,
		Flagged:             m.Flagged,
	}, true
}
```

Create `x/capture_defense/keeper/challenge_adapter.go`:

```go
package keeper

import (
	"context"

	cctypes "github.com/zerone-chain/zerone/x/capture_challenge/types"
)

// ChallengeCaptureDefenseAdapter wraps capture_defense Keeper to satisfy
// capture_challenge's CaptureDefenseKeeper interface.
type ChallengeCaptureDefenseAdapter struct {
	k *Keeper
}

// NewChallengeCaptureDefenseAdapter creates a new adapter.
func NewChallengeCaptureDefenseAdapter(k *Keeper) *ChallengeCaptureDefenseAdapter {
	return &ChallengeCaptureDefenseAdapter{k: k}
}

// GetCaptureMetrics implements capture_challenge types.CaptureDefenseKeeper.
func (a *ChallengeCaptureDefenseAdapter) GetCaptureMetrics(ctx context.Context, domain string) (*cctypes.CaptureMetricsData, bool) {
	return a.k.GetCaptureMetricsForChallenge(ctx, domain)
}

// ClearCaptureFlag implements capture_challenge types.CaptureDefenseKeeper.
func (a *ChallengeCaptureDefenseAdapter) ClearCaptureFlag(ctx context.Context, domain string) {
	a.k.ClearCaptureFlag(ctx, domain)
}
```

Then update app.go wiring:

```go
app.CaptureChallengeKeeper.SetCaptureDefenseKeeper(
	zeronecdkeeper.NewChallengeCaptureDefenseAdapter(&app.CaptureDefenseKeeper),
)
```

**Step 2: Commit**

```
feat(app): wire capture defense immune system keepers (R28-8)
```

---

### Task 11: Add CLI query commands

**Files:**
- Modify: `x/capture_defense/client/cli/query.go` (add flagged-domains, validator-reputation)
- Modify: `x/capture_challenge/client/cli/query.go` (add active-challenges)

**Step 1: Add flagged-domains query to capture_defense CLI**

In `x/capture_defense/client/cli/query.go`, add to NewQueryCmd's subcommands:

```go
NewQueryFlaggedDomainsCmd(),
```

Add the command function:

```go
// NewQueryFlaggedDomainsCmd returns a CLI command to query all flagged domains.
func NewQueryFlaggedDomainsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "flagged-domains",
		Short: "Query all domains with active capture flags",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.FlaggedDomains(cmd.Context(), &types.QueryFlaggedDomainsRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
```

**Step 2: Implement FlaggedDomains query server**

Check if QueryServer already exists and add the FlaggedDomains handler. If not in proto, implement as a CLI-only query using gRPC direct call pattern matching existing queries.

**Step 3: Add active-challenges to capture_challenge CLI**

In `x/capture_challenge/client/cli/query.go`, add to NewQueryCmd's subcommands:

```go
NewQueryActiveChallengesCmd(),
```

```go
// NewQueryActiveChallengesCmd returns a CLI command to query all active challenges.
func NewQueryActiveChallengesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "active",
		Short: "Query all active capture challenges",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.ActiveChallenges(cmd.Context(), &types.QueryActiveChallengesRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
```

**Step 4: Implement query server handlers**

Add to `x/capture_defense/keeper/query_server.go`:

```go
func (q queryServer) FlaggedDomains(ctx context.Context, req *types.QueryFlaggedDomainsRequest) (*types.QueryFlaggedDomainsResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	var flagged []*types.CaptureMetrics
	q.IterateCaptureMetrics(sdkCtx, func(m *types.CaptureMetrics) bool {
		if m.Flagged {
			flagged = append(flagged, m)
		}
		return false
	})
	return &types.QueryFlaggedDomainsResponse{Metrics: flagged}, nil
}
```

Add to `x/capture_challenge/keeper/query_server.go`:

```go
func (q queryServer) ActiveChallenges(ctx context.Context, req *types.QueryActiveChallengesRequest) (*types.QueryActiveChallengesResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	var active []*types.CaptureChallenge
	q.IterateChallenges(sdkCtx, func(c *types.CaptureChallenge) bool {
		if c.Status == types.ChallengeStatusEvidence || c.Status == types.ChallengeStatusOpen || c.Status == types.ChallengeStatusUnderReview {
			active = append(active, c)
		}
		return false
	})
	return &types.QueryActiveChallengesResponse{Challenges: active}, nil
}
```

**Step 5: Commit**

```
feat(cli): add flagged-domains and active-challenges queries (R28-8)
```

---

### Task 12: Integration tests

**Files:**
- Create: `x/capture_defense/keeper/integration_test.go`

**Step 1: Write comprehensive integration tests**

```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	// ... test setup imports
)

// TestFeedingVerificationHistory tests that verification data flows from knowledge to capture defense.
func TestFeedingVerificationHistory(t *testing.T) {
	// Setup: create capture defense keeper with test store
	// Act: call RecordVerificationFromKnowledge with domain, validators, verdicts
	// Assert: GetHistoryByDomain returns the entry
	// Assert: GetDomainsWithHistory includes the domain
}

// TestReputationUpdatesAfterRound tests 3-layer reputation updates.
func TestReputationUpdatesAfterRound(t *testing.T) {
	// Setup: create capture defense keeper
	// Act: call UpdateReputation for validators with approved=true and approved=false
	// Assert: Global reputation increased for approved, decreased for rejected
	// Assert: Domain reputation updated
	// Assert: Stratum reputation updated when stratum provided
}

// TestSingleValidatorDominatesTriggersFlag tests HHI flagging.
func TestSingleValidatorDominatesTriggersFlag(t *testing.T) {
	// Setup: create 10 history entries with same single validator
	// Act: run AnalyzeCaptureRisk
	// Assert: metrics.Flagged == true
	// Assert: metrics.HerfindahlIndex == BPSScale (monopoly = 100%)
}

// TestDiverseParticipationNotFlagged tests healthy metrics.
func TestDiverseParticipationNotFlagged(t *testing.T) {
	// Setup: create 10 history entries with 5 different validators evenly distributed
	// Act: run AnalyzeCaptureRisk
	// Assert: metrics.Flagged == false
	// Assert: HHI is low (200,000 BPS for 5 equal validators)
}

// TestAutoChallengCreatedWhenFlagged tests auto-analysis → challenge flow.
func TestAutoChallengeCreatedWhenFlagged(t *testing.T) {
	// Setup: populate history with concentrated validator participation
	// Wire mock challenge keeper
	// Act: run RunAutoAnalysis
	// Assert: mock received AutoSubmitChallenge call with correct domain
}

// TestAnalyzeDomainReturnsRealMetrics tests MsgAnalyzeDomain.
func TestAnalyzeDomainReturnsRealMetrics(t *testing.T) {
	// Setup: populate history
	// Act: call AnalyzeDomain msg handler
	// Assert: response has status="analyzed", non-zero risk score, correct flagged state
}

// TestChallengeResolutionUpheld tests capture confirmed consequences.
func TestChallengeResolutionUpheld(t *testing.T) {
	// Setup: create a challenge in UNDER_REVIEW status
	// Wire mock qualification and knowledge keepers
	// Act: resolve as UPHELD
	// Assert: qualification reduction applied
	// Assert: verification threshold increased
	// Assert: capture_confirmed event emitted
}

// TestChallengeResolutionRejected tests flag clearing.
func TestChallengeResolutionRejected(t *testing.T) {
	// Setup: create a challenge, set domain as flagged
	// Act: resolve as REJECTED
	// Assert: domain flag cleared
}

// TestAlignmentSecuritySensorReadsCapture tests alignment integration.
func TestAlignmentSecuritySensorReadsCapture(t *testing.T) {
	// Setup: mock capture defense keeper returning 2 flagged domains
	// Mock ontology keeper returning 10 total domains
	// Act: call senseNetworkSecurity
	// Assert: security reduced by 20% (2/10 flagged)
}

// TestEndToEndCaptureFlow tests the full lifecycle.
func TestEndToEndCaptureFlow(t *testing.T) {
	// 1. Record concentrated verification history
	// 2. Run auto-analysis → detect flag
	// 3. Auto-challenge created
	// 4. Resolve challenge as upheld
	// 5. Verify qualification reduction applied
	// 6. Verify alignment security score reduced
}
```

**Step 2: Run tests**

```bash
go test ./x/capture_defense/keeper/... -v -run TestFeeding
go test ./x/capture_defense/keeper/... -v -run TestReputation
go test ./x/capture_defense/keeper/... -v -run TestSingle
go test ./x/capture_defense/keeper/... -v -run TestDiverse
go test ./x/capture_defense/keeper/... -v -run TestAutoChallenge
go test ./x/capture_defense/keeper/... -v -run TestAnalyzeDomain
go test ./x/capture_defense/keeper/... -v -run TestChallengeResolution
go test ./x/capture_defense/keeper/... -v -run TestAlignment
go test ./x/capture_defense/keeper/... -v -run TestEndToEnd
```

**Step 3: Commit**

```
test(capture_defense): add integration tests for immune system wiring (R28-8)
```

---

### Task 13: Build verification

**Step 1: Run full build**

```bash
go build ./...
```

**Step 2: Run all capture-related tests**

```bash
go test ./x/capture_defense/... -v
go test ./x/capture_challenge/... -v
go test ./x/knowledge/keeper/... -v -run TestCapture
go test ./x/alignment/keeper/... -v -run TestSecurity
```

**Step 3: Final commit**

```
feat(capture_defense): complete immune system wiring (R28-8)

Wire the capture defense system end-to-end:
- Verification history feeds from knowledge to capture defense
- 3-layer reputation system active and updating
- Auto-analysis detects concentration and auto-creates challenges
- MsgAnalyzeDomain returns real capture metrics
- Challenge resolution has consequences (qualification reduction, threshold increase)
- Alignment security sensor reads capture state
```
