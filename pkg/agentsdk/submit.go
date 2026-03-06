package agentsdk

import (
	"context"
	"fmt"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// SubmitData submits a single training data unit for quality review.
// If StakeOverride is empty, stake is auto-calculated from chain params × difficulty.
func (c *ToKClient) SubmitData(ctx context.Context, req SubmitRequest) (*SubmitResult, error) {
	if req.Content == "" {
		return nil, fmt.Errorf("content is required")
	}
	if req.Domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	if req.Type == "" {
		return nil, fmt.Errorf("type is required")
	}

	// Parse TDU type
	sampleType, err := parseTDUType(req.Type)
	if err != nil {
		return nil, err
	}

	// Compute stake
	stake, err := c.computeStake(ctx, req.Difficulty, req.StakeOverride)
	if err != nil {
		return nil, fmt.Errorf("failed to compute stake: %w", err)
	}

	// Build consent proof
	var consent *types.ConsentProof
	if req.ConsentType != "" {
		ct, err := parseConsentType(req.ConsentType)
		if err != nil {
			return nil, err
		}
		consent = &types.ConsentProof{Type: ct}
	}

	// Compute content hash
	hashHex := contentHashHex([]byte(req.Content))

	msg := &types.MsgSubmitData{
		Submitter:          c.chain.GetAddress(),
		Content:            req.Content,
		SampleType:         sampleType,
		Domain:             req.Domain,
		SourceUri:          req.SourceURI,
		SourcePlatform:     req.SourcePlatform,
		Consent:            consent,
		OriginalAuthor:     req.OriginalAuthor,
		License:            req.License,
		Tags:               req.Tags,
		Language:           req.Language,
		Stake:              stake,
		ParentSubmissionId: req.ParentSubmissionID,
		ThreadId:           req.ThreadID,
		Sponsored:          req.Sponsored,
	}

	txHash, err := c.broadcastWithRetry(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to submit data: %w", err)
	}

	return &SubmitResult{
		TxHash:      txHash,
		ContentHash: hashHex,
		Stake:       stake,
	}, nil
}

// SubmitThread submits a multi-turn conversation thread as a batch TDU.
func (c *ToKClient) SubmitThread(ctx context.Context, req ThreadSubmitRequest) (*ThreadSubmitResult, error) {
	if len(req.Turns) < 2 {
		return nil, fmt.Errorf("thread must have at least 2 turns, got %d", len(req.Turns))
	}
	if req.Domain == "" {
		return nil, fmt.Errorf("domain is required")
	}

	// Compute stake
	stake, err := c.computeStake(ctx, req.Difficulty, req.StakeOverride)
	if err != nil {
		return nil, fmt.Errorf("failed to compute stake: %w", err)
	}

	// Build consent proof
	var consent *types.ConsentProof
	if req.ConsentType != "" {
		ct, err := parseConsentType(req.ConsentType)
		if err != nil {
			return nil, err
		}
		consent = &types.ConsentProof{Type: ct}
	}

	// Build items from turns
	submitter := c.chain.GetAddress()
	var items []*types.MsgSubmitData
	for _, turn := range req.Turns {
		item := &types.MsgSubmitData{
			Submitter:  submitter,
			Content:    fmt.Sprintf("[%s]: %s", turn.Role, turn.Content),
			SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION,
			Domain:     req.Domain,
			Consent:    consent,
			ThreadId:   req.ThreadID,
		}
		items = append(items, item)
	}

	msg := &types.MsgSubmitThread{
		Submitter: submitter,
		Domain:    req.Domain,
		Stake:     stake,
		ThreadId:  req.ThreadID,
		Items:     items,
	}

	txHash, err := c.broadcastWithRetry(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to submit thread: %w", err)
	}

	return &ThreadSubmitResult{
		ThreadID: req.ThreadID,
		TxHash:   txHash,
		Stake:    stake,
	}, nil
}

// SubmitCorrection submits a correction that supersedes an existing TDU.
func (c *ToKClient) SubmitCorrection(ctx context.Context, req CorrectionRequest) (*SubmitResult, error) {
	if req.TargetID == "" {
		return nil, fmt.Errorf("target ID is required")
	}
	if req.Content == "" {
		return nil, fmt.Errorf("content is required")
	}
	if req.Reason == "" {
		return nil, fmt.Errorf("reason is required")
	}

	// Default difficulty for corrections is standard
	difficulty := req.Difficulty
	if difficulty == "" {
		difficulty = DifficultyStandard
	}

	// Compute stake
	stake, err := c.computeStake(ctx, difficulty, req.StakeOverride)
	if err != nil {
		return nil, fmt.Errorf("failed to compute stake: %w", err)
	}

	// Build consent proof (corrections are self-authored by default)
	consentType := req.ConsentType
	if consentType == "" {
		consentType = ConsentOriginal
	}
	ct, err := parseConsentType(consentType)
	if err != nil {
		return nil, err
	}
	consent := &types.ConsentProof{Type: ct}

	hashHex := contentHashHex([]byte(req.Content))

	msg := &types.MsgSubmitData{
		Submitter:          c.chain.GetAddress(),
		Content:            req.Content,
		SampleType:         types.SampleType_SAMPLE_TYPE_CORRECTION,
		Domain:             req.Domain,
		Consent:            consent,
		Stake:              stake,
		ParentSubmissionId: req.TargetID,
		Tags:               []string{"correction", "reason:" + req.Reason},
	}

	txHash, err := c.broadcastWithRetry(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to submit correction: %w", err)
	}

	return &SubmitResult{
		TxHash:      txHash,
		ContentHash: hashHex,
		Stake:       stake,
	}, nil
}
