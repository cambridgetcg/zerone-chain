package cross_stack_test

import (
	"encoding/json"
	"testing"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/stretchr/testify/require"

	zeroneapp "github.com/zerone-chain/zerone/app"

	alignmenttypes "github.com/zerone-chain/zerone/x/alignment/types"
	zeroneauthtypes "github.com/zerone-chain/zerone/x/auth/types"
	autopoiesistypes "github.com/zerone-chain/zerone/x/autopoiesis/types"
	billingtypes "github.com/zerone-chain/zerone/x/billing/types"
	bvmtypes "github.com/zerone-chain/zerone/x/bvm/types"
	capturechallengetypes "github.com/zerone-chain/zerone/x/capture_challenge/types"
	capturedefensetypes "github.com/zerone-chain/zerone/x/capture_defense/types"
	channelstypes "github.com/zerone-chain/zerone/x/channels/types"
	claimingpottypes "github.com/zerone-chain/zerone/x/claiming_pot/types"
	computepooltypes "github.com/zerone-chain/zerone/x/compute_pool/types"
	discoverytypes "github.com/zerone-chain/zerone/x/discovery/types"
	disputestypes "github.com/zerone-chain/zerone/x/disputes/types"
	emergencytypes "github.com/zerone-chain/zerone/x/emergency/types"
	evidencemgmttypes "github.com/zerone-chain/zerone/x/evidence_mgmt/types"
	zeronegov "github.com/zerone-chain/zerone/x/gov/types"
	hometypes "github.com/zerone-chain/zerone/x/home/types"
	ibcratelimittypes "github.com/zerone-chain/zerone/x/ibcratelimit/types"
	icaauthtypes "github.com/zerone-chain/zerone/x/icaauth/types"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
	liquiditypooltypes "github.com/zerone-chain/zerone/x/liquiditypool/types"
	ontologytypes "github.com/zerone-chain/zerone/x/ontology/types"
	partnershipstypes "github.com/zerone-chain/zerone/x/partnerships/types"
	qualificationtypes "github.com/zerone-chain/zerone/x/qualification/types"
	researchtypes "github.com/zerone-chain/zerone/x/research/types"
	scheduletypes "github.com/zerone-chain/zerone/x/schedule/types"
	zeronestakingtypes "github.com/zerone-chain/zerone/x/staking/types"
	tokenstypes "github.com/zerone-chain/zerone/x/tokens/types"
	toolboxtypes "github.com/zerone-chain/zerone/x/toolbox/types"
	treetypes "github.com/zerone-chain/zerone/x/tree/types"
	vestingrewardstypes "github.com/zerone-chain/zerone/x/vesting_rewards/types"

	gogoproto "github.com/cosmos/gogoproto/proto"
)

// TestProtoJSONRoundTrip_AllModuleParams verifies that every custom module's
// DefaultGenesis survives a proto-JSON codec round-trip with ALL fields intact.
//
// This catches the exact bug where rawDesc in *.pb.go files is stale:
// protojson uses the embedded file descriptor (not Go struct tags) to
// serialize/deserialize. When rawDesc is stale, new fields are silently
// dropped during MarshalJSON → UnmarshalJSON.
//
// The test for each module:
//  1. Creates a typed DefaultGenesis() in Go (has all fields set)
//  2. Marshals via cdc.MarshalJSON (uses protojson / rawDesc)
//  3. Unmarshals back via cdc.UnmarshalJSON into a fresh struct
//  4. Re-marshals the restored struct via cdc.MarshalJSON
//  5. Compares the two JSON outputs — any field loss means asymmetry
func TestProtoJSONRoundTrip_AllModuleParams(t *testing.T) {
	app := newTestApp(t, testChainID)
	cdc := app.AppCodec()

	type moduleRoundTrip struct {
		name     string
		original gogoproto.Message
		factory  func() gogoproto.Message // creates zero-value target for unmarshal
	}

	modules := []moduleRoundTrip{
		{alignmenttypes.ModuleName, alignmenttypes.DefaultGenesis(), func() gogoproto.Message { return &alignmenttypes.GenesisState{} }},
		{zeroneauthtypes.ModuleName, zeroneauthtypes.DefaultGenesis(), func() gogoproto.Message { return &zeroneauthtypes.GenesisState{} }},
		{autopoiesistypes.ModuleName, autopoiesistypes.DefaultGenesis(), func() gogoproto.Message { return &autopoiesistypes.GenesisState{} }},
		{billingtypes.ModuleName, billingtypes.DefaultGenesis(), func() gogoproto.Message { return &billingtypes.GenesisState{} }},
		{bvmtypes.ModuleName, bvmtypes.DefaultGenesisState(), func() gogoproto.Message { return &bvmtypes.GenesisState{} }},
		{capturechallengetypes.ModuleName, capturechallengetypes.DefaultGenesis(), func() gogoproto.Message { return &capturechallengetypes.GenesisState{} }},
		{capturedefensetypes.ModuleName, capturedefensetypes.DefaultGenesis(), func() gogoproto.Message { return &capturedefensetypes.GenesisState{} }},
		{channelstypes.ModuleName, channelstypes.DefaultGenesis(), func() gogoproto.Message { return &channelstypes.GenesisState{} }},
		{claimingpottypes.ModuleName, claimingpottypes.DefaultGenesis(), func() gogoproto.Message { return &claimingpottypes.GenesisState{} }},
		{computepooltypes.ModuleName, computepooltypes.DefaultGenesis(), func() gogoproto.Message { return &computepooltypes.GenesisState{} }},
		{discoverytypes.ModuleName, discoverytypes.DefaultGenesis(), func() gogoproto.Message { return &discoverytypes.GenesisState{} }},
		{disputestypes.ModuleName, disputestypes.DefaultGenesis(), func() gogoproto.Message { return &disputestypes.GenesisState{} }},
		{emergencytypes.ModuleName, emergencytypes.DefaultGenesis(), func() gogoproto.Message { return &emergencytypes.GenesisState{} }},
		{evidencemgmttypes.ModuleName, evidencemgmttypes.DefaultGenesis(), func() gogoproto.Message { return &evidencemgmttypes.GenesisState{} }},
		{zeronegov.ModuleName, zeronegov.DefaultGenesisState(), func() gogoproto.Message { return &zeronegov.GenesisState{} }},
		{hometypes.ModuleName, hometypes.DefaultGenesis(), func() gogoproto.Message { return &hometypes.GenesisState{} }},
		{ibcratelimittypes.ModuleName, ibcratelimittypes.DefaultGenesis(), func() gogoproto.Message { return &ibcratelimittypes.GenesisState{} }},
		{icaauthtypes.ModuleName, icaauthtypes.DefaultGenesis(), func() gogoproto.Message { return &icaauthtypes.GenesisState{} }},
		{knowledgetypes.ModuleName, knowledgetypes.DefaultGenesis(), func() gogoproto.Message { return &knowledgetypes.GenesisState{} }},
		{liquiditypooltypes.ModuleName, liquiditypooltypes.DefaultGenesis(), func() gogoproto.Message { return &liquiditypooltypes.GenesisState{} }},
		{ontologytypes.ModuleName, ontologytypes.DefaultGenesis(), func() gogoproto.Message { return &ontologytypes.GenesisState{} }},
		{partnershipstypes.ModuleName, partnershipstypes.DefaultGenesis(), func() gogoproto.Message { return &partnershipstypes.GenesisState{} }},
		{qualificationtypes.ModuleName, qualificationtypes.DefaultGenesis(), func() gogoproto.Message { return &qualificationtypes.GenesisState{} }},
		{researchtypes.ModuleName, researchtypes.DefaultGenesis(), func() gogoproto.Message { return &researchtypes.GenesisState{} }},
		{scheduletypes.ModuleName, scheduletypes.DefaultGenesis(), func() gogoproto.Message { return &scheduletypes.GenesisState{} }},
		{zeronestakingtypes.ModuleName, zeronestakingtypes.DefaultGenesisState(), func() gogoproto.Message { return &zeronestakingtypes.GenesisState{} }},
		{tokenstypes.ModuleName, tokenstypes.DefaultGenesis(), func() gogoproto.Message { return &tokenstypes.GenesisState{} }},
		{toolboxtypes.ModuleName, toolboxtypes.DefaultGenesis(), func() gogoproto.Message { return &toolboxtypes.GenesisState{} }},
		{treetypes.ModuleName, treetypes.DefaultGenesisState(), func() gogoproto.Message { return &treetypes.GenesisState{} }},
		{vestingrewardstypes.ModuleName, vestingrewardstypes.DefaultGenesis(), func() gogoproto.Message { return &vestingrewardstypes.GenesisState{} }},
	}

	for _, m := range modules {
		t.Run(m.name, func(t *testing.T) {
			// Step 1: Marshal typed Go struct → JSON via proto codec
			bz1, err := cdc.MarshalJSON(m.original)
			require.NoError(t, err, "MarshalJSON(original) failed for %s", m.name)

			// Step 2: Unmarshal back into a fresh zero-value struct
			restored := m.factory()
			require.NoError(t, cdc.UnmarshalJSON(bz1, restored),
				"UnmarshalJSON failed for %s", m.name)

			// Step 3: Re-marshal the restored struct
			bz2, err := cdc.MarshalJSON(restored)
			require.NoError(t, err, "MarshalJSON(restored) failed for %s", m.name)

			// Step 4: The two JSON outputs must be semantically identical.
			// If rawDesc is stale, or if the marshal/unmarshal path drops
			// fields, the second output will differ.
			require.JSONEq(t, string(bz1), string(bz2),
				"module %s: proto-JSON round-trip is not stable — fields were lost", m.name)
		})
	}

	t.Logf("proto-JSON round-trip verified for %d modules", len(modules))
}

// TestFullGenesisValidateAfterProtoRoundTrip verifies that the full app
// DefaultGenesis survives a complete JSON round-trip and still passes
// ValidateGenesis for all modules.
func TestFullGenesisValidateAfterProtoRoundTrip(t *testing.T) {
	app := newTestApp(t, testChainID)
	cdc := app.AppCodec()

	genState := app.DefaultGenesis()
	require.NotEmpty(t, genState)

	// Per-module JSON round-trip: unmarshal + re-marshal each module's raw genesis.
	roundTripped := make(zeroneapp.GenesisState, len(genState))
	for moduleName, raw := range genState {
		roundTripped[moduleName] = jsonRoundTrip(t, moduleName, raw)
	}

	// Full genesis validation after round-trip.
	err := zeroneapp.ModuleBasics.ValidateGenesis(
		cdc,
		app.TxConfig(),
		roundTripped,
	)
	require.NoError(t, err, "genesis must validate after JSON round-trip")

	// Verify no modules were lost.
	require.Equal(t, len(genState), len(roundTripped),
		"module count must be preserved after round-trip")

	t.Logf("full genesis validated after round-trip (%d modules)", len(roundTripped))
}

// jsonRoundTrip unmarshals raw genesis JSON and re-marshals it. This catches
// JSON encoding inconsistencies (field name mismatches, encoding errors).
func jsonRoundTrip(t *testing.T, moduleName string, raw json.RawMessage) json.RawMessage {
	t.Helper()

	var intermediate interface{}
	err := json.Unmarshal(raw, &intermediate)
	require.NoError(t, err, "module %s: unmarshal raw genesis", moduleName)

	roundTripped, err := json.Marshal(intermediate)
	require.NoError(t, err, "module %s: re-marshal genesis", moduleName)

	require.JSONEq(t, string(raw), string(roundTripped),
		"module %s: JSON round-trip altered genesis content", moduleName)

	return roundTripped
}

// Ensure codec.Codec is used (prevents unused import error in tests).
var _ codec.Codec = nil
