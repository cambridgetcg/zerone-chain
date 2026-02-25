package main

import (
	"context"
	"fmt"
	"os"

	dbm "github.com/cosmos/cosmos-db"
	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

func main() {
	home := os.Getenv("HOME") + "/.zeroned"
	if len(os.Args) > 1 {
		home = os.Args[1]
	}
	
	fmt.Printf("Opening store at %s/data\n", home)
	
	// Open the database
	db, err := dbm.NewDB("application", dbm.GoLevelDBBackend, home+"/data")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Create store key
	storeKey := storetypes.NewKVStoreKey("knowledge")
	
	// Create commit multi store
	cms := store.NewCommitMultiStore(db, log.NewNopLogger(), nil)
	cms.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, nil)
	if err := cms.LoadLatestVersion(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to load store: %v\n", err)
		os.Exit(1)
	}

	kvStore := cms.GetKVStore(storeKey)
	
	// Read current params
	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	
	paramsBz := kvStore.Get(types.ParamsKey)
	if paramsBz == nil {
		fmt.Println("No params found, using defaults")
	}
	
	var params types.Params
	if paramsBz != nil {
		if err := cdc.Unmarshal(paramsBz, &params); err != nil {
			fmt.Fprintf(os.Stderr, "failed to unmarshal params: %v\n", err)
			os.Exit(1)
		}
	}
	
	fmt.Printf("Current fitness_epoch_blocks: %d\n", params.FitnessEpochBlocks)
	fmt.Printf("Current metabolism_base_cost: %d\n", params.MetabolismBaseCost)
	
	// Set R20 params
	params.FitnessEpochBlocks = 50  // ~2 min at 2.5s blocks
	params.FitnessWeightQueryBps = 300_000
	params.FitnessWeightCitationBps = 250_000
	params.FitnessWeightBridgeBps = 100_000
	params.FitnessWeightDepthBps = 100_000
	params.FitnessWeightPatronBps = 50_000
	params.FitnessWeightUniqueBps = 100_000
	params.FitnessWeightAgeBps = 100_000
	
	params.MetabolismBaseCost = 100
	params.MetabolismContentLengthBps = 10_000
	params.MetabolismDomainCompetitionBps = 5_000
	params.MetabolismEnergyPerQuery = 10
	params.MetabolismEnergyPerCitation = 50
	params.MetabolismEnergyPerPatronage = 200
	params.MetabolismEnergyChallengeSurvival = 500
	params.MetabolismEnergyCap = 10_000
	params.MetabolismInitialEnergy = 5_000
	params.MetabolismAtRiskEpochs = 3
	params.MetabolismExpiredToPrunedEpochs = 3
	
	params.CompetitionNicheDominanceBonusBps = 100_000
	params.CompetitionRedundancyThresholdBps = 200_000
	params.CompetitionMaxNicheSize = 10
	params.CompetitionSymbiosisBonusBps = 50_000
	
	params.DemandBountyThreshold = 5
	params.DemandBountyBaseReward = "1000000"
	params.DemandBountyPerQueryBonus = "100000"
	params.DemandBountyExpiryEpochs = 10
	
	params.MaxClaimTextLength = 1000
	
	// Validate
	if err := params.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "validation failed: %v\n", err)
		os.Exit(1)
	}
	
	// Marshal and set
	bz, err := cdc.Marshal(&params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal params: %v\n", err)
		os.Exit(1)
	}
	
	kvStore.Set(types.ParamsKey, bz)
	
	// Commit
	_, err = cms.Commit()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to commit: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println("✅ R20 params set successfully")
	fmt.Printf("  fitness_epoch_blocks: %d\n", params.FitnessEpochBlocks)
	fmt.Printf("  metabolism_base_cost: %d\n", params.MetabolismBaseCost)
	fmt.Printf("  competition_max_niche_size: %d\n", params.CompetitionMaxNicheSize)
	fmt.Printf("  demand_bounty_threshold: %d\n", params.DemandBountyThreshold)
}
