package cli

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

func TestDemandReportJSONRoundTrip(t *testing.T) {
	reports := []*types.DemandReport{
		{Domain: "physics", Subject: "gravity", Queries: 10, Fulfilled: 7, Unfulfilled: 3},
	}
	raw, err := json.Marshal(reports)
	require.NoError(t, err)

	var decoded []*types.DemandReport
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.Len(t, decoded, 1)
	require.Equal(t, "physics", decoded[0].Domain)
	require.Equal(t, uint64(3), decoded[0].Unfulfilled)
}
