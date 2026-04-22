package keeper

import (
	"context"
	"fmt"
	"math/big"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/toolbox/types"
)

// msgServer implements types.MsgServer.
type msgServer struct {
	types.UnimplementedMsgServer
	k Keeper
}

// NewMsgServerImpl returns a MsgServer implementation.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{k: keeper}
}

var _ types.MsgServer = &msgServer{}

// RegisterTool registers a new tool in the registry.
func (ms *msgServer) RegisterTool(ctx context.Context, msg *types.MsgRegisterTool) (*types.MsgRegisterToolResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockHeight := uint64(sdkCtx.BlockHeight())

	// Validate tool type.
	if !types.ValidToolTypes[msg.ToolType] {
		return nil, types.ErrInvalidToolType.Wrapf("unknown tool type: %s", msg.ToolType)
	}

	// Validate category.
	if msg.Category != "" && !types.IsValidCategory(msg.Category) {
		return nil, types.ErrInvalidCategory.Wrapf("unknown category: %s", msg.Category)
	}

	// Validate license.
	if msg.License != "" && !types.ValidLicenses[msg.License] {
		return nil, types.ErrInvalidLicense.Wrapf("unknown license: %s", msg.License)
	}

	// Validate USD pricing fields.
	if err := validateUSDPricingFields(msg.TargetPriceUsd, msg.MinPricePerCall, msg.MaxPricePerCall); err != nil {
		return nil, err
	}

	// Enforce governance-set price collar (L5).
	if err := enforcePriceCollar(ms.k.GetParams(ctx), msg.PricePerCall, msg.MaxPricePerCall); err != nil {
		return nil, err
	}

	// Check name uniqueness.
	if ms.k.ToolNameExists(ctx, msg.Name) {
		return nil, types.ErrToolAlreadyExists.Wrapf("tool name %q already exists", msg.Name)
	}

	// Validate BVM contract if applicable.
	if msg.ToolType == types.ToolTypeBVMContract && ms.k.bvmKeeper != nil {
		if !ms.k.bvmKeeper.ContractExists(ctx, msg.ContractAddress) {
			return nil, types.ErrInvalidContractOwner.Wrapf("contract %s does not exist", msg.ContractAddress)
		}
		creator, err := ms.k.bvmKeeper.GetContractCreator(ctx, msg.ContractAddress)
		if err != nil || creator != msg.Deployer {
			return nil, types.ErrInvalidContractOwner.Wrapf("deployer is not the contract creator")
		}
	}

	// Validate dependencies.
	toolID := ms.k.nextToolID(ctx)
	if len(msg.DependencyIds) > 0 {
		if err := ms.k.ValidateDependencies(ctx, toolID, msg.DependencyIds); err != nil {
			return nil, err
		}
	}

	// Default price.
	pricePerCall := msg.PricePerCall
	if pricePerCall == "" {
		pricePerCall = "0"
	}

	tool := &types.Tool{
		Id:                   toolID,
		Name:                 msg.Name,
		Description:          msg.Description,
		Version:              msg.Version,
		ToolType:             msg.ToolType,
		Deployer:             msg.Deployer,
		ContractAddress:      msg.ContractAddress,
		ServiceId:            msg.ServiceId,
		DependencyIds:        msg.DependencyIds,
		PricePerCall:         pricePerCall,
		TargetPriceUsd:       msg.TargetPriceUsd,
		MinPricePerCall:      msg.MinPricePerCall,
		MaxPricePerCall:      msg.MaxPricePerCall,
		TotalRevenue:         "0",
		TotalCalls:           "0",
		UniqueCallers:        0,
		TrustScore:           InitialTrustScore(),
		DeployedAtBlock:      blockHeight,
		Status:               types.ToolStatusDraft,
		SourceHash:           msg.SourceHash,
		ApiSchema:            msg.ApiSchema,
		License:              msg.License,
		Tags:                 msg.Tags,
		Category:             msg.Category,
		RequiredCapabilities: msg.RequiredCapabilities,
		Contributors: []*types.ContributorShare{
			{
				Address:      msg.Deployer,
				Role:         types.RoleArchitect,
				ShareBps:     types.BpsDenominator, // 100%
				JoinedAtBlock: blockHeight,
				TotalEarned:  "0",
				Accepted:     true,
			},
		},
	}

	ms.k.SetTool(ctx, tool)

	// Store dependency edges.
	if len(msg.DependencyIds) > 0 {
		ms.k.StoreDependencyEdges(ctx, toolID, msg.DependencyIds, blockHeight)
	}

	// Initialize trust snapshot.
	ms.k.SetTrustSnapshot(ctx, &types.TrustSnapshot{
		ToolId:          toolID,
		Score:           InitialTrustScore(),
		ComputedAtBlock: blockHeight,
	})

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.toolbox.tool_registered",
			sdk.NewAttribute("tool_id", toolID),
			sdk.NewAttribute("deployer", msg.Deployer),
			sdk.NewAttribute("name", msg.Name),
			sdk.NewAttribute("tool_type", msg.ToolType),
		),
	)

	return &types.MsgRegisterToolResponse{ToolId: toolID}, nil
}

// CallTool executes a tool call with payment.
func (ms *msgServer) CallTool(ctx context.Context, msg *types.MsgCallTool) (*types.MsgCallToolResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockHeight := uint64(sdkCtx.BlockHeight())

	tool, ok := ms.k.GetTool(ctx, msg.ToolId)
	if !ok {
		return nil, types.ErrToolNotFound.Wrapf("tool %s not found", msg.ToolId)
	}

	// Status check.
	if tool.Status == types.ToolStatusRetired {
		return nil, types.ErrToolRetired.Wrapf("tool %s is retired", msg.ToolId)
	}
	if tool.Status == types.ToolStatusDraft {
		return nil, types.ErrInvalidStatus.Wrapf("tool %s is in draft status", msg.ToolId)
	}

	// Agent registration check.
	if ms.k.discoveryKeeper != nil && !ms.k.discoveryKeeper.IsRegisteredAgent(ctx, msg.Caller) {
		return nil, types.ErrNotRegisteredAgent
	}

	// Collect payment.
	maxFee := parseUint64(msg.MaxFee)
	paymentAmount, isFree, err := ms.k.CollectPayment(ctx, msg.Caller, tool, maxFee)
	if err != nil {
		return nil, err
	}

	// Execute the tool.
	var execResult []byte
	var execErr error
	success := true

	switch tool.ToolType {
	case types.ToolTypeBVMContract:
		if ms.k.bvmKeeper != nil {
			params := ms.k.GetParams(ctx)
			execResult, execErr = ms.k.bvmKeeper.CallContract(ctx, msg.Caller, tool.ContractAddress, msg.Input, params.ToolGasLimit)
			if execErr != nil {
				success = false
			}
		}
	case types.ToolTypeKnowledgeTemplate:
		if ms.k.knowledgeKeeper != nil {
			factIDs, searchErr := ms.k.knowledgeKeeper.SearchFactsByContent(ctx, "", []string{tool.KnowledgeQuery}, 10)
			if searchErr != nil {
				success = false
				execErr = searchErr
			} else {
				execResult = []byte(fmt.Sprintf(`{"facts":%d}`, len(factIDs)))
			}
		}
	case types.ToolTypeComposite:
		if len(tool.DependencyIds) > 0 {
			// Revenue cascade: execute deps in topological order, then own pipeline.
			cascadeResult, cascadeErr := ms.k.ExecuteCompositeWithCascade(ctx, tool, msg.Caller, msg.Input, paymentAmount)
			if cascadeErr != nil {
				success = false
				execErr = cascadeErr
			} else {
				execResult = cascadeResult.Output
			}
		} else {
			execResult, execErr = ms.k.ExecuteCompositeTool(ctx, tool, msg.Caller, msg.Input)
			if execErr != nil {
				success = false
			}
		}
	}

	// Record the call.
	callID := ms.k.nextCallID(ctx)
	errMsg := ""
	if execErr != nil {
		errMsg = execErr.Error()
	}
	_ = execResult // Result stored off-chain; call record tracks success.

	callRecord := &types.ToolCall{
		CallId:      callID,
		ToolId:      msg.ToolId,
		Caller:      msg.Caller,
		Payment:     strconv.FormatUint(paymentAmount, 10),
		BlockHeight: blockHeight,
		Success:     success,
		Error:       errMsg,
	}
	ms.k.SetToolCall(ctx, callRecord)

	// Update tool stats.
	tool.TotalCalls = addUint64Str(tool.TotalCalls, 1)
	tool.TotalRevenue = addUint64Str(tool.TotalRevenue, paymentAmount)
	tool.LastCalledBlock = blockHeight

	// Update trust score (EMA).
	oldScore := tool.TrustScore
	ms.k.UpdateTrustScore(ctx, tool, success)

	// Record caller.
	ms.k.RecordCaller(ctx, msg.ToolId, msg.Caller, blockHeight, success)
	tool.UniqueCallers = ms.k.CountUniqueCallers(ctx, msg.ToolId)

	ms.k.SetTool(ctx, tool)

	// Record demand.
	ms.k.RecordToolCall(ctx, msg.ToolId)

	// Update agent tool usage.
	usage := ms.k.GetAgentToolUsage(ctx, msg.Caller, msg.ToolId)
	ms.k.SetAgentToolUsage(ctx, msg.Caller, msg.ToolId, usage+1)

	// Distribute revenue (if paid).
	if paymentAmount > 0 && !isFree {
		coin := uzrnCoin(paymentAmount)
		if err := ms.k.DistributeRevenue(ctx, tool, coin); err != nil {
			ms.k.Logger(ctx).Error("revenue distribution failed", "tool_id", msg.ToolId, "error", err)
		}
	}

	// Emit trust update event.
	if oldScore != tool.TrustScore {
		ms.k.EmitTrustScoreEvent(ctx, msg.ToolId, oldScore, tool.TrustScore)
	}

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.toolbox.tool_called",
			sdk.NewAttribute("call_id", callID),
			sdk.NewAttribute("tool_id", msg.ToolId),
			sdk.NewAttribute("caller", msg.Caller),
			sdk.NewAttribute("payment", strconv.FormatUint(paymentAmount, 10)),
			sdk.NewAttribute("success", strconv.FormatBool(success)),
		),
	)

	return &types.MsgCallToolResponse{CallId: callID, Success: success}, nil
}

// AddContributor adds a new contributor to a tool with share reallocation.
func (ms *msgServer) AddContributor(ctx context.Context, msg *types.MsgAddContributor) (*types.MsgAddContributorResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockHeight := uint64(sdkCtx.BlockHeight())

	tool, ok := ms.k.GetTool(ctx, msg.ToolId)
	if !ok {
		return nil, types.ErrToolNotFound
	}

	// Only deployer can add contributors.
	if tool.Deployer != msg.Authority {
		return nil, types.ErrNotDeployer
	}

	// Check share lock.
	if tool.ShareLockHeight > 0 {
		return nil, types.ErrSharesLocked
	}

	// Validate role.
	if !types.ValidRoles[msg.Role] {
		return nil, types.ErrInvalidRole.Wrapf("unknown role: %s", msg.Role)
	}

	// Check max contributors.
	params := ms.k.GetParams(ctx)
	if uint32(len(tool.Contributors)) >= params.MaxContributors {
		return nil, types.ErrTooManyContributors
	}

	// Check contributor doesn't already exist.
	for _, c := range tool.Contributors {
		if c.Address == msg.ContributorAddress {
			return nil, types.ErrContributorExists
		}
	}

	// Check for existing pending contributorship.
	if _, found := ms.k.GetPendingContributorship(ctx, msg.ToolId, msg.ContributorAddress); found {
		return nil, types.ErrPendingContributorship
	}

	// Apply share reallocation and validate.
	if err := applyShareReallocation(tool, msg.Reallocations, msg.ContributorAddress, msg.ShareBps); err != nil {
		return nil, err
	}

	// Create pending contributorship.
	pc := &types.PendingContributorship{
		ToolId:              msg.ToolId,
		ContributorAddress:  msg.ContributorAddress,
		Role:                msg.Role,
		ShareBps:            msg.ShareBps,
		ProposedAtBlock:     blockHeight,
	}
	ms.k.SetPendingContributorship(ctx, pc)

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.toolbox.contributor_added",
			sdk.NewAttribute("tool_id", msg.ToolId),
			sdk.NewAttribute("contributor", msg.ContributorAddress),
			sdk.NewAttribute("role", msg.Role),
		),
	)

	return &types.MsgAddContributorResponse{}, nil
}

// AcceptContributorship accepts a pending contributorship invitation.
func (ms *msgServer) AcceptContributorship(ctx context.Context, msg *types.MsgAcceptContributorship) (*types.MsgAcceptContributorshipResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockHeight := uint64(sdkCtx.BlockHeight())

	pc, found := ms.k.GetPendingContributorship(ctx, msg.ToolId, msg.ContributorAddress)
	if !found {
		return nil, types.ErrNoPendingContributorship
	}

	tool, ok := ms.k.GetTool(ctx, msg.ToolId)
	if !ok {
		return nil, types.ErrToolNotFound
	}

	// Add the new contributor.
	tool.Contributors = append(tool.Contributors, &types.ContributorShare{
		Address:       pc.ContributorAddress,
		Role:          pc.Role,
		ShareBps:      pc.ShareBps,
		JoinedAtBlock: blockHeight,
		TotalEarned:   "0",
		Accepted:      true,
	})

	ms.k.SetTool(ctx, tool)
	ms.k.DeletePendingContributorship(ctx, msg.ToolId, msg.ContributorAddress)

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.toolbox.contributorship_accepted",
			sdk.NewAttribute("tool_id", msg.ToolId),
			sdk.NewAttribute("contributor", msg.ContributorAddress),
		),
	)

	return &types.MsgAcceptContributorshipResponse{}, nil
}

// UpgradeTool creates a new version of an existing tool.
func (ms *msgServer) UpgradeTool(ctx context.Context, msg *types.MsgUpgradeTool) (*types.MsgUpgradeToolResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockHeight := uint64(sdkCtx.BlockHeight())

	prevTool, ok := ms.k.GetTool(ctx, msg.PreviousToolId)
	if !ok {
		return nil, types.ErrToolNotFound.Wrapf("previous tool %s not found", msg.PreviousToolId)
	}

	if prevTool.Deployer != msg.Deployer {
		return nil, types.ErrNotDeployer
	}

	if prevTool.Status == types.ToolStatusRetired {
		return nil, types.ErrToolRetired
	}

	// Validate USD pricing.
	if err := validateUSDPricingFields(msg.TargetPriceUsd, msg.MinPricePerCall, msg.MaxPricePerCall); err != nil {
		return nil, err
	}

	// Enforce governance-set price collar on updated prices (L5).
	if err := enforcePriceCollar(ms.k.GetParams(ctx), msg.PricePerCall, msg.MaxPricePerCall); err != nil {
		return nil, err
	}

	// Validate new dependencies.
	newToolID := ms.k.nextToolID(ctx)
	if len(msg.DependencyIds) > 0 {
		if err := ms.k.ValidateDependencies(ctx, newToolID, msg.DependencyIds); err != nil {
			return nil, err
		}
	}

	pricePerCall := msg.PricePerCall
	if pricePerCall == "" {
		pricePerCall = prevTool.PricePerCall
	}

	// Inherit fields from previous version.
	contractAddr := msg.ContractAddress
	if contractAddr == "" {
		contractAddr = prevTool.ContractAddress
	}
	serviceId := msg.ServiceId
	if serviceId == "" {
		serviceId = prevTool.ServiceId
	}

	newTool := &types.Tool{
		Id:                   newToolID,
		Name:                 prevTool.Name,
		Description:          msg.Description,
		Version:              msg.NewVersion,
		PreviousVersionId:    msg.PreviousToolId,
		ToolType:             prevTool.ToolType,
		Deployer:             msg.Deployer,
		ContractAddress:      contractAddr,
		ServiceId:            serviceId,
		KnowledgeQuery:       prevTool.KnowledgeQuery,
		DependencyIds:        msg.DependencyIds,
		Contributors:         prevTool.Contributors,
		PricePerCall:         pricePerCall,
		TargetPriceUsd:       msg.TargetPriceUsd,
		MinPricePerCall:      msg.MinPricePerCall,
		MaxPricePerCall:      msg.MaxPricePerCall,
		TotalRevenue:         "0",
		TotalCalls:           "0",
		UniqueCallers:        0,
		TrustScore:           prevTool.TrustScore,
		DeployedAtBlock:      blockHeight,
		Status:               types.ToolStatusDraft,
		IsVerified:           prevTool.IsVerified,
		VerifiedSince:        prevTool.VerifiedSince,
		SourceHash:           msg.SourceHash,
		ApiSchema:            msg.ApiSchema,
		License:              prevTool.License,
		Tags:                 prevTool.Tags,
		Category:             prevTool.Category,
		RequiredCapabilities: prevTool.RequiredCapabilities,
	}

	ms.k.SetTool(ctx, newTool)

	if len(msg.DependencyIds) > 0 {
		ms.k.StoreDependencyEdges(ctx, newToolID, msg.DependencyIds, blockHeight)
	}

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.toolbox.tool_upgraded",
			sdk.NewAttribute("new_tool_id", newToolID),
			sdk.NewAttribute("previous_tool_id", msg.PreviousToolId),
			sdk.NewAttribute("version", msg.NewVersion),
		),
	)

	return &types.MsgUpgradeToolResponse{NewToolId: newToolID}, nil
}

// DeprecateTool marks a tool as deprecated.
func (ms *msgServer) DeprecateTool(ctx context.Context, msg *types.MsgDeprecateTool) (*types.MsgDeprecateToolResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	tool, ok := ms.k.GetTool(ctx, msg.ToolId)
	if !ok {
		return nil, types.ErrToolNotFound
	}

	if tool.Deployer != msg.Authority {
		return nil, types.ErrNotDeployer
	}

	if tool.Status == types.ToolStatusRetired {
		return nil, types.ErrToolRetired.Wrapf("cannot deprecate a retired tool")
	}

	// If successor specified, verify it exists.
	if msg.SuccessorToolId != "" {
		if _, ok := ms.k.GetTool(ctx, msg.SuccessorToolId); !ok {
			return nil, types.ErrToolNotFound.Wrapf("successor tool %s not found", msg.SuccessorToolId)
		}
	}

	ms.k.UpdateToolStatus(ctx, tool, types.ToolStatusDeprecated)

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.toolbox.tool_deprecated",
			sdk.NewAttribute("tool_id", msg.ToolId),
			sdk.NewAttribute("successor_tool_id", msg.SuccessorToolId),
		),
	)

	return &types.MsgDeprecateToolResponse{}, nil
}

// RetireTool permanently retires a tool.
func (ms *msgServer) RetireTool(ctx context.Context, msg *types.MsgRetireTool) (*types.MsgRetireToolResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	tool, ok := ms.k.GetTool(ctx, msg.ToolId)
	if !ok {
		return nil, types.ErrToolNotFound
	}

	if tool.Deployer != msg.Authority {
		return nil, types.ErrNotDeployer
	}

	if tool.Status == types.ToolStatusRetired {
		return nil, types.ErrToolRetired.Wrapf("tool is already retired")
	}

	ms.k.UpdateToolStatus(ctx, tool, types.ToolStatusRetired)

	// Clean up dependency edges.
	ms.k.deleteDependencyEdgesFrom(ctx, tool.Id)

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.toolbox.tool_retired",
			sdk.NewAttribute("tool_id", msg.ToolId),
		),
	)

	return &types.MsgRetireToolResponse{}, nil
}

// LockShares locks contributor shares for a tool.
func (ms *msgServer) LockShares(ctx context.Context, msg *types.MsgLockShares) (*types.MsgLockSharesResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockHeight := uint64(sdkCtx.BlockHeight())

	tool, ok := ms.k.GetTool(ctx, msg.ToolId)
	if !ok {
		return nil, types.ErrToolNotFound
	}

	if tool.Deployer != msg.Deployer {
		return nil, types.ErrNotDeployer
	}

	if tool.ShareLockHeight > 0 {
		return nil, types.ErrSharesLocked.Wrapf("shares already locked at block %d", tool.ShareLockHeight)
	}

	params := ms.k.GetParams(ctx)
	tool.ShareLockHeight = blockHeight + params.ShareLockCooldownBlocks
	ms.k.SetTool(ctx, tool)

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.toolbox.shares_locked",
			sdk.NewAttribute("tool_id", msg.ToolId),
			sdk.NewAttribute("lock_height", strconv.FormatUint(tool.ShareLockHeight, 10)),
		),
	)

	return &types.MsgLockSharesResponse{}, nil
}

// UpdateDependency replaces one dependency with another.
func (ms *msgServer) UpdateDependency(ctx context.Context, msg *types.MsgUpdateDependency) (*types.MsgUpdateDependencyResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockHeight := uint64(sdkCtx.BlockHeight())

	tool, ok := ms.k.GetTool(ctx, msg.ToolId)
	if !ok {
		return nil, types.ErrToolNotFound
	}

	if tool.Deployer != msg.Authority {
		return nil, types.ErrNotDeployer
	}

	// Verify old dependency exists.
	if _, ok := ms.k.GetDependencyEdge(ctx, msg.ToolId, msg.OldDepId); !ok {
		return nil, types.ErrDependencyNotFound.Wrapf("dependency on %s not found", msg.OldDepId)
	}

	// Validate new dependency.
	if err := ms.k.ValidateDependencies(ctx, msg.ToolId, []string{msg.NewDepId}); err != nil {
		return nil, err
	}

	// Swap edges.
	ms.k.DeleteDependencyEdge(ctx, msg.ToolId, msg.OldDepId)
	ms.k.SetDependencyEdge(ctx, &types.DependencyEdge{
		FromToolId:     msg.ToolId,
		ToToolId:       msg.NewDepId,
		CreatedAtBlock: blockHeight,
	})

	// Update tool's dependency list.
	newDeps := make([]string, 0, len(tool.DependencyIds))
	for _, dep := range tool.DependencyIds {
		if dep == msg.OldDepId {
			newDeps = append(newDeps, msg.NewDepId)
		} else {
			newDeps = append(newDeps, dep)
		}
	}
	tool.DependencyIds = newDeps
	ms.k.SetTool(ctx, tool)

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.toolbox.dependency_updated",
			sdk.NewAttribute("tool_id", msg.ToolId),
			sdk.NewAttribute("old_dep_id", msg.OldDepId),
			sdk.NewAttribute("new_dep_id", msg.NewDepId),
		),
	)

	return &types.MsgUpdateDependencyResponse{}, nil
}

// ToolHeartbeat records agent tool activity.
func (ms *msgServer) ToolHeartbeat(ctx context.Context, msg *types.MsgToolHeartbeat) (*types.MsgToolHeartbeatResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	ms.k.SetAgentActiveTools(ctx, msg.Sender, msg.ActiveTools)

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.toolbox.tool_heartbeat",
			sdk.NewAttribute("tool_id", fmt.Sprintf("%d", len(msg.ActiveTools))),
			sdk.NewAttribute("caller", msg.Sender),
			sdk.NewAttribute("block", fmt.Sprintf("%d", sdkCtx.BlockHeight())),
		),
	)

	return &types.MsgToolHeartbeatResponse{}, nil
}

// UpdateParams updates module parameters (governance-gated).
func (ms *msgServer) UpdateParams(ctx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if msg.Authority != ms.k.authority {
		return nil, types.ErrUnauthorized.Wrapf("expected %s, got %s", ms.k.authority, msg.Authority)
	}

	if msg.Params == nil {
		return nil, types.ErrInvalidParams.Wrapf("params cannot be nil")
	}

	if err := msg.Params.Validate(); err != nil {
		return nil, types.ErrInvalidParams.Wrapf("invalid params: %v", err)
	}

	ms.k.SetParams(ctx, msg.Params)

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.toolbox.update_params",
			sdk.NewAttribute("authority", msg.Authority),
		),
	)

	return &types.MsgUpdateParamsResponse{}, nil
}

// ---------- Helpers ----------

// applyShareReallocation validates and applies share reallocations for adding a contributor.
func applyShareReallocation(tool *types.Tool, reallocations []*types.ShareReallocation, newAddr string, newShareBps uint64) error {
	// Build a map of current shares.
	shareMap := make(map[string]uint64)
	for _, c := range tool.Contributors {
		shareMap[c.Address] = c.ShareBps
	}

	// Apply reallocations.
	for _, r := range reallocations {
		if _, ok := shareMap[r.Address]; !ok {
			return types.ErrContributorNotFound.Wrapf("contributor %s not found for reallocation", r.Address)
		}
		shareMap[r.Address] = r.NewShareBps
	}

	// Add new contributor's share.
	shareMap[newAddr] = newShareBps

	// Validate total.
	var total uint64
	for _, share := range shareMap {
		total += share
	}
	if total != types.BpsDenominator {
		return types.ErrSharesNotSumTo100.Wrapf("shares sum to %d, expected %d", total, types.BpsDenominator)
	}

	// Apply to tool.
	for _, c := range tool.Contributors {
		if newShare, ok := shareMap[c.Address]; ok {
			c.ShareBps = newShare
		}
	}

	return nil
}

// Ensure big import is used.
var _ = (*big.Int)(nil)
var _ = fmt.Sprintf
