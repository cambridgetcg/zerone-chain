package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Route B Wave 11: incident response pipeline ─────────────────────────
//
// The on-chain incident log. Every bug found on a live chain that warrants
// a formal response gets an IncidentRecord. The record's remediation
// lineage links each action to the concrete mechanism that fired
// (named upgrade, param amendment, emergency ceremony, schema amendment,
// structured state correction).
//
// This is the connective tissue between the upgrade protocol (Wave 10),
// the x/emergency halt ceremonies, and the governance param amendments.
// Authority-gated throughout; the record is append-only with respect to
// resolution (remediations accrue, status advances, never retracts).

// ─── SLA defaults (blocks) ──────────────────────────────────────────────
//
// Default response-time windows per severity. Operators can override per
// incident via MsgOpenIncident.sla_window_blocks. These are pragmatic
// starting points — a chain with a shorter block time can shrink them;
// a chain with longer blocks should expand.

const (
	// P0 — chain halt / consensus break. Target: 1 hour at 5s blocks.
	SlaP0Blocks uint64 = 720
	// P1 — high-impact; immediate fix. Target: 4 hours.
	SlaP1Blocks uint64 = 2_880
	// P2 — scheduled upgrade. Target: 7 days.
	SlaP2Blocks uint64 = 120_960
	// P3 — next-release or documentation-only. Target: 30 days.
	SlaP3Blocks uint64 = 518_400
)

// ─── CRUD ────────────────────────────────────────────────────────────────

// SetIncidentRecord persists the record and maintains all reverse indexes.
// Idempotent on (id, same-content); safe to call on status transitions.
func (k Keeper) SetIncidentRecord(ctx context.Context, r *types.IncidentRecord) error {
	if r == nil || r.Id == "" {
		return fmt.Errorf("invalid incident record")
	}
	store := k.storeService.OpenKVStore(ctx)

	// Purge old status/severity entries if this is an update with drift.
	if prev, ok := k.GetIncidentRecord(ctx, r.Id); ok && prev != nil {
		if prev.Status != r.Status {
			_ = store.Delete(types.IncidentByStatusKey(byte(prev.Status), r.Id))
		}
		if prev.Severity != r.Severity {
			_ = store.Delete(types.IncidentBySeverityKey(byte(prev.Severity), r.Id))
		}
		// Drop open-set marker when leaving OPEN/MITIGATING.
		if isOpen(prev.Status) && !isOpen(r.Status) {
			_ = store.Delete(types.OpenIncidentKey(r.Id))
		}
	}

	bz, err := marshalOpts.Marshal(r)
	if err != nil {
		return err
	}
	if err := store.Set(types.IncidentRecordKey(r.Id), bz); err != nil {
		return err
	}
	if err := store.Set(types.IncidentBySeverityKey(byte(r.Severity), r.Id), []byte{1}); err != nil {
		return err
	}
	if err := store.Set(types.IncidentByStatusKey(byte(r.Status), r.Id), []byte{1}); err != nil {
		return err
	}
	if isOpen(r.Status) {
		return store.Set(types.OpenIncidentKey(r.Id), []byte{1})
	}
	return store.Delete(types.OpenIncidentKey(r.Id))
}

// GetIncidentRecord fetches an incident by id.
func (k Keeper) GetIncidentRecord(ctx context.Context, id string) (*types.IncidentRecord, bool) {
	if id == "" {
		return nil, false
	}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.IncidentRecordKey(id))
	if err != nil || bz == nil {
		return nil, false
	}
	var r types.IncidentRecord
	if err := proto.Unmarshal(bz, &r); err != nil {
		return nil, false
	}
	return &r, true
}

// IterateIncidents yields every recorded incident.
func (k Keeper) IterateIncidents(ctx context.Context, cb func(*types.IncidentRecord) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.IncidentRecordKeyPrefix, prefixEndBytes(types.IncidentRecordKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var r types.IncidentRecord
		if err := proto.Unmarshal(iter.Value(), &r); err != nil {
			continue
		}
		if cb(&r) {
			return
		}
	}
}

// IterateOpenIncidents yields only OPEN or MITIGATING incidents.
func (k Keeper) IterateOpenIncidents(ctx context.Context, cb func(*types.IncidentRecord) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.OpenIncidentKeyPrefix, prefixEndBytes(types.OpenIncidentKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		id := string(key[len(types.OpenIncidentKeyPrefix):])
		r, ok := k.GetIncidentRecord(ctx, id)
		if !ok {
			continue
		}
		if cb(r) {
			return
		}
	}
}

// defaultSlaBlocks returns the response window for a severity.
func defaultSlaBlocks(s types.IncidentSeverity) uint64 {
	switch s {
	case types.IncidentSeverity_INCIDENT_SEVERITY_P0:
		return SlaP0Blocks
	case types.IncidentSeverity_INCIDENT_SEVERITY_P1:
		return SlaP1Blocks
	case types.IncidentSeverity_INCIDENT_SEVERITY_P2:
		return SlaP2Blocks
	case types.IncidentSeverity_INCIDENT_SEVERITY_P3:
		return SlaP3Blocks
	}
	return SlaP3Blocks
}

func isOpen(s types.IncidentStatus) bool {
	return s == types.IncidentStatus_INCIDENT_STATUS_OPEN ||
		s == types.IncidentStatus_INCIDENT_STATUS_MITIGATING
}

// ─── msg handlers ────────────────────────────────────────────────────────

// OpenIncident records a newly-discovered issue. Authority-gated.
// Severity locks the SLA window at open time; later reclassification
// cannot alter the measured SLA target for this incident.
func (m *msgServer) OpenIncident(ctx context.Context, msg *types.MsgOpenIncident) (*types.MsgOpenIncidentResponse, error) {
	if msg == nil || msg.Id == "" {
		return nil, fmt.Errorf("incident id required")
	}
	if msg.Authority != m.keeper.GetAuthority() {
		return nil, fmt.Errorf("unauthorized: only governance authority may open incidents")
	}
	if msg.Severity == types.IncidentSeverity_INCIDENT_SEVERITY_UNSPECIFIED {
		return nil, fmt.Errorf("severity required")
	}
	if _, exists := m.keeper.GetIncidentRecord(ctx, msg.Id); exists {
		return nil, fmt.Errorf("incident %s already exists", msg.Id)
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())
	window := msg.SlaWindowBlocks
	if window == 0 {
		window = defaultSlaBlocks(msg.Severity)
	}

	rec := &types.IncidentRecord{
		Id:              msg.Id,
		Severity:        msg.Severity,
		Status:          types.IncidentStatus_INCIDENT_STATUS_OPEN,
		Title:           msg.Title,
		Description:     msg.Description,
		Reporter:        msg.Reporter,
		ReportedAtBlock: height,
		AffectedModules: msg.AffectedModules,
		SlaTargetBlock:  height + window,
	}
	if err := m.keeper.SetIncidentRecord(ctx, rec); err != nil {
		return nil, err
	}

	m.keeper.RecordPrivilegedAction(ctx,
		types.PrivilegedActionType_PRIVILEGED_ACTION_TYPE_INCIDENT_OPEN,
		msg.Authority, rec.Id, rec.Id, msg.Title)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.incident_opened",
		sdk.NewAttribute("incident_id", rec.Id),
		sdk.NewAttribute("severity", msg.Severity.String()),
		sdk.NewAttribute("title", rec.Title),
		sdk.NewAttribute("sla_target_block", fmt.Sprintf("%d", rec.SlaTargetBlock)),
	))
	return &types.MsgOpenIncidentResponse{SlaTargetBlock: rec.SlaTargetBlock}, nil
}

// RecordRemediation appends a remediation action to an incident. Advances
// status OPEN → MITIGATING on the first remediation. Multiple remediations
// may attach — param fix first, then upgrade, then documentation.
func (m *msgServer) RecordRemediation(ctx context.Context, msg *types.MsgRecordRemediation) (*types.MsgRecordRemediationResponse, error) {
	if msg == nil || msg.IncidentId == "" {
		return nil, fmt.Errorf("incident_id required")
	}
	if msg.Authority != m.keeper.GetAuthority() {
		return nil, fmt.Errorf("unauthorized: only governance authority may record remediation")
	}
	if msg.Type == types.RemediationType_REMEDIATION_TYPE_UNSPECIFIED {
		return nil, fmt.Errorf("remediation type required")
	}
	rec, ok := m.keeper.GetIncidentRecord(ctx, msg.IncidentId)
	if !ok {
		return nil, fmt.Errorf("incident %s not found", msg.IncidentId)
	}
	if rec.Status == types.IncidentStatus_INCIDENT_STATUS_CLOSED {
		return nil, fmt.Errorf("cannot remediate a CLOSED incident")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	rec.Remediations = append(rec.Remediations, &types.Remediation{
		Type:           msg.Type,
		Reference:      msg.Reference,
		AppliedAtBlock: uint64(sdkCtx.BlockHeight()),
		Operator:       msg.Authority,
		Note:           msg.Note,
	})
	if rec.Status == types.IncidentStatus_INCIDENT_STATUS_OPEN {
		rec.Status = types.IncidentStatus_INCIDENT_STATUS_MITIGATING
	}
	if err := m.keeper.SetIncidentRecord(ctx, rec); err != nil {
		return nil, err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.incident_remediation_recorded",
		sdk.NewAttribute("incident_id", rec.Id),
		sdk.NewAttribute("remediation_type", msg.Type.String()),
		sdk.NewAttribute("reference", msg.Reference),
		sdk.NewAttribute("total_remediations", fmt.Sprintf("%d", len(rec.Remediations))),
	))
	return &types.MsgRecordRemediationResponse{TotalRemediations: uint32(len(rec.Remediations))}, nil
}

// ResolveIncident advances MITIGATING → RESOLVED and stamps the
// post-mortem URI. The incident remains observable in history but drops
// out of the "open incidents" dashboard.
func (m *msgServer) ResolveIncident(ctx context.Context, msg *types.MsgResolveIncident) (*types.MsgResolveIncidentResponse, error) {
	if msg == nil || msg.IncidentId == "" {
		return nil, fmt.Errorf("incident_id required")
	}
	if msg.Authority != m.keeper.GetAuthority() {
		return nil, fmt.Errorf("unauthorized")
	}
	rec, ok := m.keeper.GetIncidentRecord(ctx, msg.IncidentId)
	if !ok {
		return nil, fmt.Errorf("incident %s not found", msg.IncidentId)
	}
	if rec.Status == types.IncidentStatus_INCIDENT_STATUS_RESOLVED ||
		rec.Status == types.IncidentStatus_INCIDENT_STATUS_CLOSED {
		return nil, fmt.Errorf("incident already resolved")
	}
	if len(rec.Remediations) == 0 {
		return nil, fmt.Errorf("cannot resolve an incident with zero remediations")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	rec.Status = types.IncidentStatus_INCIDENT_STATUS_RESOLVED
	rec.ResolvedAtBlock = uint64(sdkCtx.BlockHeight())
	rec.PostMortemUri = msg.PostMortemUri
	if err := m.keeper.SetIncidentRecord(ctx, rec); err != nil {
		return nil, err
	}

	m.keeper.RecordPrivilegedAction(ctx,
		types.PrivilegedActionType_PRIVILEGED_ACTION_TYPE_INCIDENT_RESOLVE,
		msg.Authority, rec.Id, rec.Id, msg.PostMortemUri)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.incident_resolved",
		sdk.NewAttribute("incident_id", rec.Id),
		sdk.NewAttribute("post_mortem_uri", msg.PostMortemUri),
		sdk.NewAttribute("sla_met", fmt.Sprintf("%t", rec.ResolvedAtBlock <= rec.SlaTargetBlock)),
	))
	return &types.MsgResolveIncidentResponse{}, nil
}

// CloseIncident permanently archives an incident. Only RESOLVED incidents
// may be closed; archival is a terminal state.
func (m *msgServer) CloseIncident(ctx context.Context, msg *types.MsgCloseIncident) (*types.MsgCloseIncidentResponse, error) {
	if msg == nil || msg.IncidentId == "" {
		return nil, fmt.Errorf("incident_id required")
	}
	if msg.Authority != m.keeper.GetAuthority() {
		return nil, fmt.Errorf("unauthorized")
	}
	rec, ok := m.keeper.GetIncidentRecord(ctx, msg.IncidentId)
	if !ok {
		return nil, fmt.Errorf("incident %s not found", msg.IncidentId)
	}
	if rec.Status != types.IncidentStatus_INCIDENT_STATUS_RESOLVED {
		return nil, fmt.Errorf("only RESOLVED incidents can be closed")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	rec.Status = types.IncidentStatus_INCIDENT_STATUS_CLOSED
	rec.ClosedAtBlock = uint64(sdkCtx.BlockHeight())
	if err := m.keeper.SetIncidentRecord(ctx, rec); err != nil {
		return nil, err
	}

	m.keeper.RecordPrivilegedAction(ctx,
		types.PrivilegedActionType_PRIVILEGED_ACTION_TYPE_INCIDENT_CLOSE,
		msg.Authority, rec.Id, rec.Id, "")
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.incident_closed",
		sdk.NewAttribute("incident_id", rec.Id),
	))
	return &types.MsgCloseIncidentResponse{}, nil
}
