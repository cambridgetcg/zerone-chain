package keeper

import (
	"context"
	"fmt"

	"github.com/zerone-chain/zerone/x/private_corpus/types"
)

type queryServer struct {
	types.UnimplementedQueryServer
	keeper Keeper
}

func NewQueryServerImpl(k Keeper) types.QueryServer {
	return &queryServer{keeper: k}
}

var _ types.QueryServer = &queryServer{}

func (q *queryServer) Vault(ctx context.Context, req *types.QueryVaultRequest) (*types.QueryVaultResponse, error) {
	if req == nil || req.Id == "" {
		return nil, fmt.Errorf("id required")
	}
	v, ok := q.keeper.GetVault(ctx, req.Id)
	if !ok {
		return nil, fmt.Errorf("%w: %s", types.ErrVaultNotFound, req.Id)
	}
	return &types.QueryVaultResponse{Vault: v}, nil
}

func (q *queryServer) Vaults(ctx context.Context, req *types.QueryVaultsRequest) (*types.QueryVaultsResponse, error) {
	if req == nil {
		req = &types.QueryVaultsRequest{}
	}
	limit := req.Limit
	if limit == 0 || limit > 200 {
		limit = 50
	}
	out := make([]*types.Vault, 0, limit)
	skipped := req.StartAfterId == ""
	var nextCursor string
	_ = q.keeper.IterateVaults(ctx, func(v *types.Vault) bool {
		if !skipped {
			if v.Id == req.StartAfterId {
				skipped = true
			}
			return false
		}
		if uint32(len(out)) >= limit {
			nextCursor = v.Id
			return true
		}
		out = append(out, v)
		return false
	})
	return &types.QueryVaultsResponse{Vaults: out, NextStartAfterId: nextCursor}, nil
}

func (q *queryServer) VaultsByOperator(ctx context.Context, req *types.QueryVaultsByOperatorRequest) (*types.QueryVaultsByOperatorResponse, error) {
	if req == nil || req.Operator == "" {
		return nil, fmt.Errorf("operator required")
	}
	out := []*types.Vault{}
	_ = q.keeper.IterateVaultsByOperator(ctx, req.Operator, func(v *types.Vault) bool {
		out = append(out, v)
		return false
	})
	return &types.QueryVaultsByOperatorResponse{Vaults: out}, nil
}

func (q *queryServer) Manifest(ctx context.Context, req *types.QueryManifestRequest) (*types.QueryManifestResponse, error) {
	if req == nil || req.Id == "" {
		return nil, fmt.Errorf("id required")
	}
	m, ok := q.keeper.GetManifest(ctx, req.Id)
	if !ok {
		return nil, fmt.Errorf("%w: %s", types.ErrManifestNotFound, req.Id)
	}
	return &types.QueryManifestResponse{Manifest: m}, nil
}

func (q *queryServer) ManifestsByVault(ctx context.Context, req *types.QueryManifestsByVaultRequest) (*types.QueryManifestsByVaultResponse, error) {
	if req == nil || req.VaultId == "" {
		return nil, fmt.Errorf("vault_id required")
	}
	limit := req.Limit
	if limit == 0 || limit > 200 {
		limit = 50
	}
	out := make([]*types.CorpusManifest, 0, limit)
	skipped := req.StartAfterId == ""
	var nextCursor string
	_ = q.keeper.IterateManifestsByVault(ctx, req.VaultId, func(m *types.CorpusManifest) bool {
		if !skipped {
			if m.Id == req.StartAfterId {
				skipped = true
			}
			return false
		}
		if uint32(len(out)) >= limit {
			nextCursor = m.Id
			return true
		}
		out = append(out, m)
		return false
	})
	return &types.QueryManifestsByVaultResponse{Manifests: out, NextStartAfterId: nextCursor}, nil
}

func (q *queryServer) AccessRecords(ctx context.Context, req *types.QueryAccessRecordsRequest) (*types.QueryAccessRecordsResponse, error) {
	if req == nil || req.VaultId == "" {
		return nil, fmt.Errorf("vault_id required")
	}
	params := q.keeper.GetParams(ctx)
	limit := req.Limit
	maxLimit := params.MaxAccessRecordsPerQuery
	if maxLimit == 0 {
		maxLimit = 200
	}
	if limit == 0 || limit > maxLimit {
		limit = maxLimit
	}
	out := make([]*types.AccessRecord, 0, limit)
	var nextCursor uint64
	_ = q.keeper.IterateAccessRecordsByVault(ctx, req.VaultId, req.StartAfterSeq, limit+1, func(r *types.AccessRecord) bool {
		if uint32(len(out)) >= limit {
			nextCursor = r.Seq
			return true
		}
		out = append(out, r)
		return false
	})
	return &types.QueryAccessRecordsResponse{Records: out, NextStartAfterSeq: nextCursor}, nil
}

func (q *queryServer) Params(ctx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	p := q.keeper.GetParams(ctx)
	return &types.QueryParamsResponse{Params: &p}, nil
}
