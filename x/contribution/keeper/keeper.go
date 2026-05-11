package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/binary"

	corestoretypes "cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/contribution/types"
)

// Keeper is the x/contribution module keeper.
type Keeper struct {
	cdc          codec.BinaryCodec
	storeService corestoretypes.KVStoreService

	// authority is the gov module address (used by Phase 6+ msg
	// handlers; Phase 1 stores it but doesn't enforce it).
	authority string

	// adapters is the per-class registry, populated at app init.
	adapters types.AdapterRegistry
}

// NewKeeper constructs the Keeper.
func NewKeeper(
	storeService corestoretypes.KVStoreService,
	cdc codec.BinaryCodec,
	authority string,
) Keeper {
	return Keeper{
		cdc:          cdc,
		storeService: storeService,
		authority:    authority,
		adapters:     types.NewAdapterRegistry(),
	}
}

// Authority returns the gov authority address.
func (k Keeper) Authority() string { return k.authority }

// Logger returns a sub-logger.
func (k Keeper) Logger(ctx context.Context) log.Logger {
	return sdk.UnwrapSDKContext(ctx).Logger().With("module", "x/"+types.ModuleName)
}

// RegisterAdapter exposes the registry for app-init wiring.
func (k *Keeper) RegisterAdapter(a types.ContributionAdapter) {
	k.adapters.Register(a)
}

// GetAdapter looks up the adapter for a class.
func (k Keeper) GetAdapter(class types.ContributionClass) (types.ContributionAdapter, bool) {
	return k.adapters.Get(class)
}

// ── store ops ──

func contributionKey(id []byte) []byte {
	return append(types.ContributionKey, id...)
}

// WriteContribution stores or updates a Contribution and refreshes secondary indexes.
func (k Keeper) WriteContribution(ctx context.Context, c *types.Contribution) error {
	store := k.storeService.OpenKVStore(ctx)

	// Read prior contribution (if any) so we can clean up stale secondary indexes
	// when status or other indexed fields change.
	priorBytes, err := store.Get(contributionKey(c.Id))
	if err != nil {
		return err
	}
	var prior *types.Contribution
	if priorBytes != nil {
		prior = &types.Contribution{}
		// Use google.golang.org/protobuf/proto directly: Contribution is a
		// modern protoc-gen-go message with a oneof Payload field, which
		// trips the gogoproto table marshaler used by codec.BinaryCodec
		// (nil deref in computeOneofFieldInfo). Wire-compatible swap.
		if err := proto.Unmarshal(priorBytes, prior); err != nil {
			return err
		}
	}

	// Write primary record. See note above re: proto.Marshal vs k.cdc.Marshal.
	bz, err := proto.Marshal(c)
	if err != nil {
		return err
	}
	if err := store.Set(contributionKey(c.Id), bz); err != nil {
		return err
	}

	// Refresh secondary indexes.
	if prior != nil {
		if err := store.Delete(byContributorIdxKey(prior.Contributor, prior.Id)); err != nil {
			return err
		}
		if err := store.Delete(byClassIdxKey(prior.Class, prior.Id)); err != nil {
			return err
		}
		if err := store.Delete(byPhaseIdxKey(prior.Phase, prior.Id)); err != nil {
			return err
		}
		if err := store.Delete(byStatusIdxKey(prior.Status, prior.Id)); err != nil {
			return err
		}
		if prior.BackRef != "" {
			if err := store.Delete(byBackRefKey(prior.BackRef)); err != nil {
				return err
			}
		}
	}
	if err := store.Set(byContributorIdxKey(c.Contributor, c.Id), []byte{}); err != nil {
		return err
	}
	if err := store.Set(byClassIdxKey(c.Class, c.Id), []byte{}); err != nil {
		return err
	}
	if err := store.Set(byPhaseIdxKey(c.Phase, c.Id), []byte{}); err != nil {
		return err
	}
	if err := store.Set(byStatusIdxKey(c.Status, c.Id), []byte{}); err != nil {
		return err
	}
	if c.BackRef != "" {
		if err := store.Set(byBackRefKey(c.BackRef), c.Id); err != nil {
			return err
		}
	}
	return nil
}

// WrapAsSubstrateContribution constructs a Substrate-class Contribution
// describing a privileged action being taken, writes it through the
// orchestrator at STATUS_ADMITTED (Phase 1 stub: skip the full pipeline;
// just persist + emit event), and returns the resulting Contribution.id
// for cross-reference.
//
// This is the self-application primitive: the chain treats its own
// privileged state changes as Contributions reviewed by itself. UW
// states that ZERONE is recursive; this helper is the runtime expression
// of that doctrine. Phase 6 will wire real verification + a revert
// window; at Phase 1 the recording is unconditional so the principle
// is structurally observable.
//
// subClass is one of: "doctrine", "taxonomy", "parameter", "code",
// "ops", "audit". It is stored in claims_about_self alongside the
// caller's description so off-chain tooling can route by category.
//
// parentContributionID is optional. When non-nil, the new Contribution
// is nested under that parent via payload.nested — the proto-layer
// recursion (Layer 1) becomes a runtime relationship. When nil, the
// payload is a stub PipelineImprovement carrying the description bytes.
//
// Returns ErrNestingDepthExceeded if the parent + new layer would
// breach MaxNestingDepth — the recursion is bounded, not unbounded.
func (k Keeper) WrapAsSubstrateContribution(
	ctx context.Context,
	subClass string,
	actor string,
	description []byte,
	parentContributionID []byte,
) ([]byte, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	// Build the claims_about_self envelope. Prefix with subClass so the
	// description carries the route in-band; off-chain tooling reads
	// the first \n-delimited token as the category.
	claims := append([]byte(subClass+"\n"), description...)

	// Optional payload.nested: if a parent id was supplied, look it up
	// and embed it as the nested Contribution. Falls back to a stub
	// PipelineImprovement when no parent is given, so the payload
	// oneof is never empty (downstream code may assume payload != nil).
	var payload *types.ContributionPayload
	if len(parentContributionID) > 0 {
		parent, ok := k.GetContribution(ctx, parentContributionID)
		if !ok {
			// Defensive: parent must exist. Without it the nested chain
			// would be malformed. Fall through to the stub payload so
			// the caller still gets a recorded action.
			payload = &types.ContributionPayload{
				Payload: &types.ContributionPayload_PipelineImprovement{
					PipelineImprovement: &types.PipelineImprovement{
						OpaquePayload: description,
					},
				},
			}
		} else {
			// Depth-check: parent_depth + 1 must not exceed
			// MaxNestingDepth. The recursion is structural but bounded.
			parentDepth, err := types.ContributionNestingDepth(parent)
			if err != nil {
				return nil, err
			}
			if parentDepth+1 > types.MaxNestingDepth {
				return nil, types.ErrNestingDepthExceeded
			}
			payload = &types.ContributionPayload{
				Payload: &types.ContributionPayload_Nested{Nested: parent},
			}
		}
	} else {
		payload = &types.ContributionPayload{
			Payload: &types.ContributionPayload_PipelineImprovement{
				PipelineImprovement: &types.PipelineImprovement{
					OpaquePayload: description,
				},
			},
		}
	}

	// Deterministic id derived from class+phase+actor+description+height.
	// Distinct from the user-submission id (which hashes payload bytes)
	// because two Substrate Contributions emitted in the same block from
	// the same actor with the same description must still be unique —
	// they describe distinct privileged actions.
	h := sha256.New()
	classBz := make([]byte, 4)
	binary.BigEndian.PutUint32(classBz, uint32(types.ContributionClass_PIPELINE_IMPROVEMENT))
	h.Write(classBz)
	phaseBz := make([]byte, 4)
	binary.BigEndian.PutUint32(phaseBz, uint32(types.LifecyclePhase_PHASE_SUBSTRATE))
	h.Write(phaseBz)
	h.Write([]byte(subClass))
	h.Write([]byte(actor))
	h.Write(description)
	heightBz := make([]byte, 8)
	binary.BigEndian.PutUint64(heightBz, height)
	h.Write(heightBz)
	if len(parentContributionID) > 0 {
		h.Write(parentContributionID)
	}
	id := h.Sum(nil)

	c := &types.Contribution{
		Id:              id,
		Contributor:     actor,
		Class:           types.ContributionClass_PIPELINE_IMPROVEMENT,
		Phase:           types.LifecyclePhase_PHASE_SUBSTRATE,
		Status:          types.ContributionStatus_STATUS_ADMITTED,
		ClaimsAboutSelf: claims,
		CreatedAtBlock:  height,
		AdmittedAtBlock: height,
		Payload:         payload,
		Recursion: &types.RecursionImpact{
			Type:          types.RecursionType_RECURSION_TYPE_NONE,
			MultiplierBps: 10_000, // 1× at Phase 1; Phase 6 ratifies via LIP
			Revocable:     true,
			Axes:          &types.RecursionAxisScores{},
		},
	}

	if err := k.WriteContribution(ctx, c); err != nil {
		return nil, err
	}
	// Emit submitted then admitted so indexers see the same stage flow
	// they expect from the regular submission path, even though Phase 1
	// shortcuts the pipeline. Phase 6 will replace this with the real
	// Classify+Verify pass once the substrate verifier is wired.
	k.EmitContributionSubmitted(ctx, c)
	k.EmitContributionAdmitted(ctx, c, "")
	return id, nil
}

// GetContribution reads a Contribution by id.
func (k Keeper) GetContribution(ctx context.Context, id []byte) (*types.Contribution, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(contributionKey(id))
	if err != nil || bz == nil {
		return nil, false
	}
	c := &types.Contribution{}
	// proto.Unmarshal — bypasses gogoproto table marshaler (oneof nil-deref).
	if err := proto.Unmarshal(bz, c); err != nil {
		return nil, false
	}
	return c, true
}

// GetContributionByBackRef looks up a Contribution via the back_ref index.
func (k Keeper) GetContributionByBackRef(ctx context.Context, backRef string) (*types.Contribution, bool) {
	if backRef == "" {
		return nil, false
	}
	store := k.storeService.OpenKVStore(ctx)
	idBz, err := store.Get(byBackRefKey(backRef))
	if err != nil || idBz == nil {
		return nil, false
	}
	return k.GetContribution(ctx, idBz)
}

// ── index key builders ──

func uint32BE(v uint32) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, v)
	return buf
}

func uvarintBytes(v uint64) []byte {
	buf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(buf, v)
	return buf[:n]
}

func byContributorIdxKey(contributor string, id []byte) []byte {
	addrBz := []byte(contributor)
	out := append([]byte{}, types.ByContributorKey...)
	out = append(out, uvarintBytes(uint64(len(addrBz)))...)
	out = append(out, addrBz...)
	out = append(out, id...)
	return out
}

func byClassIdxKey(class types.ContributionClass, id []byte) []byte {
	out := append([]byte{}, types.ByClassKey...)
	out = append(out, uint32BE(uint32(class))...)
	out = append(out, id...)
	return out
}

func byPhaseIdxKey(phase types.LifecyclePhase, id []byte) []byte {
	out := append([]byte{}, types.ByPhaseKey...)
	out = append(out, uint32BE(uint32(phase))...)
	out = append(out, id...)
	return out
}

func byStatusIdxKey(status types.ContributionStatus, id []byte) []byte {
	out := append([]byte{}, types.ByStatusKey...)
	out = append(out, uint32BE(uint32(status))...)
	out = append(out, id...)
	return out
}

func byBackRefKey(backRef string) []byte {
	bz := []byte(backRef)
	out := append([]byte{}, types.ByBackRefKey...)
	out = append(out, uvarintBytes(uint64(len(bz)))...)
	out = append(out, bz...)
	return out
}
