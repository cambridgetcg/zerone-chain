package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

func TestAddScrapedSource(t *testing.T) {
	k, ctx := setupKeeper(t)

	resp, err := k.AddScrapedSource(ctx, &types.MsgAddScrapedSource{
		Authority:      "authority",
		Platform:       "reddit",
		Domain:         "science",
		Description:    "Reddit r/science heavily scraped",
		NoveltyPenalty: 200000,
	})
	require.NoError(t, err)
	require.Equal(t, "reddit/science", resp.Id)

	entry, found := k.GetScrapedSource(ctx, "reddit/science")
	require.True(t, found)
	require.Equal(t, "reddit", entry.Platform)
	require.Equal(t, uint64(200000), entry.NoveltyPenalty)
	require.Equal(t, uint64(100), entry.AddedBlock)
}

func TestAddScrapedSource_Unauthorized(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.AddScrapedSource(ctx, &types.MsgAddScrapedSource{
		Authority:      testAddr,
		Platform:       "reddit",
		Domain:         "science",
		NoveltyPenalty: 200000,
	})
	require.ErrorIs(t, err, types.ErrUnauthorized)
}

func TestAddScrapedSource_Upsert(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.AddScrapedSource(ctx, &types.MsgAddScrapedSource{
		Authority: "authority", Platform: "reddit", Domain: "science", NoveltyPenalty: 200000,
	})
	require.NoError(t, err)

	_, err = k.AddScrapedSource(ctx, &types.MsgAddScrapedSource{
		Authority: "authority", Platform: "reddit", Domain: "science", NoveltyPenalty: 300000,
	})
	require.NoError(t, err)

	entry, found := k.GetScrapedSource(ctx, "reddit/science")
	require.True(t, found)
	require.Equal(t, uint64(300000), entry.NoveltyPenalty)
}

func TestRemoveScrapedSource(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.AddScrapedSource(ctx, &types.MsgAddScrapedSource{
		Authority: "authority", Platform: "reddit", Domain: "science", NoveltyPenalty: 200000,
	})
	require.NoError(t, err)

	_, err = k.RemoveScrapedSource(ctx, &types.MsgRemoveScrapedSource{
		Authority: "authority", Id: "reddit/science",
	})
	require.NoError(t, err)

	_, found := k.GetScrapedSource(ctx, "reddit/science")
	require.False(t, found)
}

func TestRemoveScrapedSource_Unauthorized(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.RemoveScrapedSource(ctx, &types.MsgRemoveScrapedSource{
		Authority: testAddr, Id: "reddit/science",
	})
	require.ErrorIs(t, err, types.ErrUnauthorized)
}

func TestRemoveScrapedSource_NotFound(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.RemoveScrapedSource(ctx, &types.MsgRemoveScrapedSource{
		Authority: "authority", Id: "nonexistent",
	})
	require.Error(t, err)
}

func TestScrapedSourcePenaltyIntegration(t *testing.T) {
	k, ctx := setupKeeper(t)

	require.Equal(t, uint64(0), k.GetScrapedSourcePenalty(ctx, "reddit", "science"))

	_, err := k.AddScrapedSource(ctx, &types.MsgAddScrapedSource{
		Authority: "authority", Platform: "stackoverflow", Domain: "technology", NoveltyPenalty: 350000,
	})
	require.NoError(t, err)

	require.Equal(t, uint64(350000), k.GetScrapedSourcePenalty(ctx, "stackoverflow", "technology"))
	require.Equal(t, uint64(0), k.GetScrapedSourcePenalty(ctx, "stackoverflow", "science"))
}
