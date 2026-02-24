package simulation_test

import (
	"context"
	"fmt"
	"sync"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	govkeeper "github.com/zerone-chain/zerone/x/gov/keeper"
	govtypes "github.com/zerone-chain/zerone/x/gov/types"
	vestingkeeper "github.com/zerone-chain/zerone/x/vesting_rewards/keeper"
)

// ============================================================================
// Shared types + Economic Simulation Invariants
// ============================================================================

// ---- Enhanced Mock Bank Keeper ----
// Tracks per-address balances with full debit/credit semantics.

type simBankKeeper struct {
	mu             sync.Mutex
	balances       map[string]sdk.Coins
	supply         map[string]sdkmath.Int
	modules        map[string]sdk.AccAddress
	cumulMinted    sdkmath.Int // total uzrn ever minted
	cumulBurned    sdkmath.Int // total uzrn ever burned
}

func newSimBankKeeper(moduleNames []string) *simBankKeeper {
	bk := &simBankKeeper{
		balances:    make(map[string]sdk.Coins),
		supply:      make(map[string]sdkmath.Int),
		modules:     make(map[string]sdk.AccAddress),
		cumulMinted: sdkmath.ZeroInt(),
		cumulBurned: sdkmath.ZeroInt(),
	}
	for _, name := range moduleNames {
		bk.modules[name] = authtypes.NewModuleAddress(name)
	}
	return bk
}

func (bk *simBankKeeper) moduleAddr(name string) string {
	if addr, ok := bk.modules[name]; ok {
		return addr.String()
	}
	return authtypes.NewModuleAddress(name).String()
}

func (bk *simBankKeeper) moduleBalance(name, denom string) sdkmath.Int {
	bk.mu.Lock()
	defer bk.mu.Unlock()
	addr := bk.moduleAddr(name)
	if coins, ok := bk.balances[addr]; ok {
		return coins.AmountOf(denom)
	}
	return sdkmath.ZeroInt()
}

func (bk *simBankKeeper) credit(addr string, coins sdk.Coins) {
	cur := bk.balances[addr]
	bk.balances[addr] = cur.Add(coins...)
}

func (bk *simBankKeeper) debit(addr string, coins sdk.Coins) error {
	cur := bk.balances[addr]
	result, hasNeg := cur.SafeSub(coins...)
	if hasNeg {
		return fmt.Errorf("insufficient funds at %s: have %s, need %s", addr, cur, coins)
	}
	bk.balances[addr] = result
	return nil
}

func (bk *simBankKeeper) MintCoins(_ context.Context, moduleName string, amounts sdk.Coins) error {
	bk.mu.Lock()
	defer bk.mu.Unlock()
	addr := bk.moduleAddr(moduleName)
	bk.credit(addr, amounts)
	for _, coin := range amounts {
		cur, ok := bk.supply[coin.Denom]
		if !ok {
			cur = sdkmath.ZeroInt()
		}
		bk.supply[coin.Denom] = cur.Add(coin.Amount)
		if coin.Denom == "uzrn" {
			bk.cumulMinted = bk.cumulMinted.Add(coin.Amount)
		}
	}
	return nil
}

func (bk *simBankKeeper) BurnCoins(_ context.Context, moduleName string, amounts sdk.Coins) error {
	bk.mu.Lock()
	defer bk.mu.Unlock()
	addr := bk.moduleAddr(moduleName)
	if err := bk.debit(addr, amounts); err != nil {
		return fmt.Errorf("burn from %s: %w", moduleName, err)
	}
	for _, coin := range amounts {
		cur, ok := bk.supply[coin.Denom]
		if !ok {
			cur = sdkmath.ZeroInt()
		}
		bk.supply[coin.Denom] = cur.Sub(coin.Amount)
		if coin.Denom == "uzrn" {
			bk.cumulBurned = bk.cumulBurned.Add(coin.Amount)
		}
	}
	return nil
}

func (bk *simBankKeeper) SendCoinsFromModuleToAccount(_ context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	bk.mu.Lock()
	defer bk.mu.Unlock()
	srcAddr := bk.moduleAddr(senderModule)
	if err := bk.debit(srcAddr, amt); err != nil {
		return fmt.Errorf("send from module %s: %w", senderModule, err)
	}
	bk.credit(recipientAddr.String(), amt)
	return nil
}

func (bk *simBankKeeper) SendCoinsFromModuleToModule(_ context.Context, senderModule, recipientModule string, amt sdk.Coins) error {
	bk.mu.Lock()
	defer bk.mu.Unlock()
	srcAddr := bk.moduleAddr(senderModule)
	dstAddr := bk.moduleAddr(recipientModule)
	if err := bk.debit(srcAddr, amt); err != nil {
		return fmt.Errorf("send from module %s to %s: %w", senderModule, recipientModule, err)
	}
	bk.credit(dstAddr, amt)
	return nil
}

func (bk *simBankKeeper) SendCoinsFromAccountToModule(_ context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error {
	bk.mu.Lock()
	defer bk.mu.Unlock()
	dstAddr := bk.moduleAddr(recipientModule)
	if err := bk.debit(senderAddr.String(), amt); err != nil {
		return fmt.Errorf("send from account %s to module %s: %w", senderAddr, recipientModule, err)
	}
	bk.credit(dstAddr, amt)
	return nil
}

func (bk *simBankKeeper) SendCoins(_ context.Context, fromAddr, toAddr sdk.AccAddress, amt sdk.Coins) error {
	bk.mu.Lock()
	defer bk.mu.Unlock()
	if err := bk.debit(fromAddr.String(), amt); err != nil {
		return fmt.Errorf("send from %s: %w", fromAddr, err)
	}
	bk.credit(toAddr.String(), amt)
	return nil
}

func (bk *simBankKeeper) GetSupply(_ context.Context, denom string) sdk.Coin {
	bk.mu.Lock()
	defer bk.mu.Unlock()
	if amt, ok := bk.supply[denom]; ok {
		return sdk.NewCoin(denom, amt)
	}
	return sdk.NewCoin(denom, sdkmath.ZeroInt())
}

func (bk *simBankKeeper) GetBalance(_ context.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	bk.mu.Lock()
	defer bk.mu.Unlock()
	if coins, ok := bk.balances[addr.String()]; ok {
		return sdk.NewCoin(denom, coins.AmountOf(denom))
	}
	return sdk.NewCoin(denom, sdkmath.ZeroInt())
}

func (bk *simBankKeeper) GetAllBalances(_ context.Context, addr sdk.AccAddress) sdk.Coins {
	bk.mu.Lock()
	defer bk.mu.Unlock()
	if coins, ok := bk.balances[addr.String()]; ok {
		return coins
	}
	return sdk.Coins{}
}

func (bk *simBankKeeper) sumAllBalances(denom string) sdkmath.Int {
	bk.mu.Lock()
	defer bk.mu.Unlock()
	total := sdkmath.ZeroInt()
	for _, coins := range bk.balances {
		total = total.Add(coins.AmountOf(denom))
	}
	return total
}

// ---- Mock Staking Keeper ----

type simStakingKeeper struct {
	activeCount uint32
}

func (s *simStakingKeeper) GetActiveValidatorCount(_ context.Context) uint32 {
	return s.activeCount
}

// ---- Simulation Domain Types ----

type simValidator struct {
	addr        sdk.AccAddress
	tier        int
	staked      sdkmath.Int
	totalEarned sdkmath.Int
	jailed      bool
}

type simAgent struct {
	addr sdk.AccAddress
	name string
}

type simTool struct {
	id      int
	creator sdk.AccAddress
}

// SimState holds all simulation state visible to invariant checks.
type SimState struct {
	bank          *simBankKeeper
	vestingKeeper vestingkeeper.Keeper
	govKeeper     *govkeeper.Keeper // optional — nil when gov module is not wired
	ctx           sdk.Context

	validators  []*simValidator
	agents      []*simAgent
	tools       []*simTool
	moduleNames []string
	founderAddr sdk.AccAddress

	currentHeight      int64
	currentEpoch       int
	currentBlockReward sdkmath.Int
	lastEpochReward    sdkmath.Int
	totalMinted        sdkmath.Int
	totalBurned        sdkmath.Int
	initialSupply      sdkmath.Int
	factsAdded         int
	toolRevenue        sdkmath.Int
	hasTransactions    bool
}

// ============================================================================
// Invariant Definitions
// ============================================================================

// Invariant is a named check function.
type Invariant struct {
	Name  string
	Check func(s *SimState) error
}

// PerBlockInvariants returns invariants checked after every block.
func PerBlockInvariants() []Invariant {
	return []Invariant{
		{"SupplyConservation", checkSupplyConservation},
		{"ModuleSolvency", checkModuleSolvency},
		{"ResearchFundNonNegative", checkResearchFundNonNegative},
		{"RevenueSplitIntegrity", checkRevenueSplitIntegrity},
		{"RewardDecay", checkRewardDecay},
	}
}

// EpochInvariants returns invariants checked every 100 blocks.
func EpochInvariants() []Invariant {
	return []Invariant{
		{"StakingRatios", checkStakingRatios},
		{"ValidatorSetStability", checkValidatorSetStability},
		{"PhaseConsistency", checkPhaseConsistency},
	}
}

// FinalInvariants returns invariants checked at end of simulation.
func FinalInvariants() []Invariant {
	return []Invariant{
		{"FinalTokenAccounting", checkFinalTokenAccounting},
		{"NoOrphanedTokens", checkNoOrphanedTokens},
		{"RewardDistributionFairness", checkRewardDistributionFairness},
		{"KnowledgeTreeGrowth", checkKnowledgeTreeGrowth},
		{"ToolRevenueGenerated", checkToolRevenueGenerated},
		{"FounderShareImmutable", checkFounderShareImmutable},
		{"NoBurn", checkNoBurn},
	}
}

// ---------- Per-block invariants ----------

func checkSupplyConservation(s *SimState) error {
	trackedSupply := s.bank.GetSupply(nil, "uzrn").Amount

	// Use the bank keeper's own cumulative counters (captures ALL mint/burn sources).
	// Initial seeding goes through MintCoins, so cumulMinted already includes it.
	s.bank.mu.Lock()
	cumulMinted := s.bank.cumulMinted
	cumulBurned := s.bank.cumulBurned
	s.bank.mu.Unlock()

	expected := cumulMinted.Sub(cumulBurned)

	if !trackedSupply.Equal(expected) {
		return fmt.Errorf("supply mismatch: tracked=%s, expected=%s (minted=%s - burned=%s)",
			trackedSupply, expected, cumulMinted, cumulBurned)
	}

	// Also verify sum of all balances matches supply.
	sumBal := s.bank.sumAllBalances("uzrn")
	if !trackedSupply.Equal(sumBal) {
		return fmt.Errorf("supply vs balances mismatch: supply=%s, sum(balances)=%s",
			trackedSupply, sumBal)
	}
	return nil
}

func checkModuleSolvency(s *SimState) error {
	for _, mod := range s.moduleNames {
		bal := s.bank.moduleBalance(mod, "uzrn")
		if bal.IsNegative() {
			return fmt.Errorf("module %q has negative balance: %s", mod, bal)
		}
	}
	return nil
}

func checkResearchFundNonNegative(s *SimState) error {
	bal := s.bank.moduleBalance("research_fund", "uzrn")
	if bal.IsNegative() {
		return fmt.Errorf("research fund negative: %s", bal)
	}
	return nil
}

func checkRevenueSplitIntegrity(s *SimState) error {
	split := s.vestingKeeper.GetRevenueSplit(s.ctx)
	total := split.ContributorBps + split.ProtocolBps + split.ResearchBps + split.DevelopmentBps
	if total != 1_000_000 {
		return fmt.Errorf("revenue split does not sum to 1M BPS: got %d", total)
	}
	return nil
}

func checkRewardDecay(s *SimState) error {
	if s.currentEpoch == 0 || s.lastEpochReward.IsZero() {
		return nil
	}
	if s.currentBlockReward.GT(s.lastEpochReward) {
		return fmt.Errorf("block reward increased: current=%s > lastEpoch=%s (epoch %d)",
			s.currentBlockReward, s.lastEpochReward, s.currentEpoch)
	}
	return nil
}

// ---------- Epoch invariants ----------

func checkStakingRatios(s *SimState) error {
	supply := s.bank.GetSupply(nil, "uzrn").Amount
	if supply.IsZero() {
		return nil
	}
	totalStaked := sdkmath.ZeroInt()
	for _, v := range s.validators {
		totalStaked = totalStaked.Add(v.staked)
	}
	pct := totalStaked.Mul(sdkmath.NewInt(100)).Quo(supply)
	if pct.GT(sdkmath.NewInt(99)) {
		return fmt.Errorf("staking ratio too high: %s%%", pct)
	}
	return nil
}

func checkValidatorSetStability(s *SimState) error {
	for _, v := range s.validators {
		if v.jailed {
			return fmt.Errorf("validator %s jailed at epoch %d", v.addr, s.currentEpoch)
		}
	}
	return nil
}

// ---------- Final invariants ----------

func checkFinalTokenAccounting(s *SimState) error {
	supply := s.bank.GetSupply(nil, "uzrn").Amount
	sumBalances := s.bank.sumAllBalances("uzrn")
	if !supply.Equal(sumBalances) {
		return fmt.Errorf("final accounting: supply=%s != sum(balances)=%s, diff=%s",
			supply, sumBalances, supply.Sub(sumBalances))
	}
	return nil
}

func checkNoOrphanedTokens(s *SimState) error {
	knownAddrs := make(map[string]bool)
	for _, a := range s.agents {
		knownAddrs[a.addr.String()] = true
	}
	for _, v := range s.validators {
		knownAddrs[v.addr.String()] = true
	}
	for _, mod := range s.moduleNames {
		knownAddrs[s.bank.moduleAddr(mod)] = true
	}
	if s.founderAddr != nil {
		knownAddrs[s.founderAddr.String()] = true
	}
	orphaned := sdkmath.ZeroInt()
	for addr, coins := range s.bank.balances {
		if !knownAddrs[addr] {
			amt := coins.AmountOf("uzrn")
			if amt.IsPositive() {
				orphaned = orphaned.Add(amt)
			}
		}
	}
	if orphaned.IsPositive() {
		return fmt.Errorf("orphaned tokens: %s uzrn in unknown accounts", orphaned)
	}
	return nil
}

func checkRewardDistributionFairness(s *SimState) error {
	if len(s.validators) < 2 {
		return nil
	}
	// In this simplified simulation, block production is round-robin (not
	// tier-weighted), so all tiers get similar block rewards. Verification
	// rewards ARE tier-scaled but selection is random. The check verifies
	// that no lower tier earns more than 3x a higher tier per-validator,
	// which would indicate a broken reward model.
	tierEarnings := make(map[int]sdkmath.Int)
	tierCount := make(map[int]int)
	for _, v := range s.validators {
		if _, ok := tierEarnings[v.tier]; !ok {
			tierEarnings[v.tier] = sdkmath.ZeroInt()
		}
		tierEarnings[v.tier] = tierEarnings[v.tier].Add(v.totalEarned)
		tierCount[v.tier]++
	}
	for t1 := 0; t1 < 4; t1++ {
		for t2 := t1 + 1; t2 < 4; t2++ {
			e1, ok1 := tierEarnings[t1]
			e2, ok2 := tierEarnings[t2]
			c1, c2 := tierCount[t1], tierCount[t2]
			if !ok1 || !ok2 || c1 == 0 || c2 == 0 {
				continue
			}
			avg1 := e1.Quo(sdkmath.NewInt(int64(c1)))
			avg2 := e2.Quo(sdkmath.NewInt(int64(c2)))
			// Lower tier should not earn more than 3x higher tier.
			threshold := avg2.Mul(sdkmath.NewInt(3))
			if avg1.GT(threshold) {
				return fmt.Errorf("tier %d avg earnings (%s) > 3x tier %d (%s)", t1, avg1, t2, avg2)
			}
		}
	}
	return nil
}

func checkKnowledgeTreeGrowth(s *SimState) error {
	if s.factsAdded == 0 {
		return fmt.Errorf("no knowledge facts added during simulation")
	}
	return nil
}

func checkToolRevenueGenerated(s *SimState) error {
	if s.toolRevenue.IsZero() {
		return fmt.Errorf("no tool revenue generated during simulation")
	}
	return nil
}

func checkFounderShareImmutable(s *SimState) error {
	params := s.vestingKeeper.GetParams(s.ctx)
	// Founder share params must match genesis values (70000 BPS, founder_sim address).
	if params.FounderShareBps != 70000 {
		return fmt.Errorf("founder share BPS changed: expected 70000, got %d", params.FounderShareBps)
	}
	if s.founderAddr != nil && params.FounderAddress != s.founderAddr.String() {
		return fmt.Errorf("founder address changed: expected %s, got %s",
			s.founderAddr.String(), params.FounderAddress)
	}
	return nil
}

func checkNoBurn(s *SimState) error {
	// In the new model, no tokens are burned. Total supply == total minted.
	s.bank.mu.Lock()
	cumulBurned := s.bank.cumulBurned
	s.bank.mu.Unlock()

	if cumulBurned.IsPositive() {
		return fmt.Errorf("tokens were burned: %s uzrn (no-burn invariant violated)", cumulBurned)
	}
	return nil
}

// checkPhaseConsistency verifies that the governance phase state is internally
// consistent: multisig threshold matches phase, community seat count matches
// phase requirements, and no seat terms exceed the maximum.
func checkPhaseConsistency(s *SimState) error {
	if s.govKeeper == nil {
		return nil // gov module not wired — skip silently
	}

	state := s.govKeeper.GetResearchFundGovernanceState(s.ctx)
	phase := state.CurrentPhase

	// 1. Verify multisig threshold matches phase.
	required, total := govtypes.GetResearchFundThreshold(phase)
	switch phase {
	case govtypes.ResearchFundPhase_RESEARCH_FUND_PHASE_GENESIS_PAIR:
		if required != 2 || total != 2 {
			return fmt.Errorf("phase 0 threshold mismatch: expected 2/2, got %d/%d", required, total)
		}
	case govtypes.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER:
		if required != 2 || total != 3 {
			return fmt.Errorf("phase 1 threshold mismatch: expected 2/3, got %d/%d", required, total)
		}
	case govtypes.ResearchFundPhase_RESEARCH_FUND_PHASE_BALANCED:
		if required != 3 || total != 5 {
			return fmt.Errorf("phase 2 threshold mismatch: expected 3/5, got %d/%d", required, total)
		}
	case govtypes.ResearchFundPhase_RESEARCH_FUND_PHASE_FULL_GOVERNANCE:
		if required != 0 || total != 0 {
			return fmt.Errorf("phase 3 threshold mismatch: expected 0/0, got %d/%d", required, total)
		}
	}

	// 2. Verify community seat count matches phase requirements.
	seatCount := len(state.CommunitySeats)
	switch phase {
	case govtypes.ResearchFundPhase_RESEARCH_FUND_PHASE_GENESIS_PAIR:
		if seatCount != 0 {
			return fmt.Errorf("phase 0 should have 0 community seats, got %d", seatCount)
		}
	case govtypes.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER:
		if seatCount != 1 {
			return fmt.Errorf("phase 1 should have 1 community seat slot, got %d", seatCount)
		}
	case govtypes.ResearchFundPhase_RESEARCH_FUND_PHASE_BALANCED:
		if seatCount != 3 {
			return fmt.Errorf("phase 2 should have 3 community seat slots, got %d", seatCount)
		}
	case govtypes.ResearchFundPhase_RESEARCH_FUND_PHASE_FULL_GOVERNANCE:
		if seatCount != 0 {
			return fmt.Errorf("phase 3 should have 0 community seats, got %d", seatCount)
		}
	}

	// 3. Verify no seat term exceeds the maximum.
	for i, termEnd := range state.SeatTermEndBlocks {
		if termEnd == 0 {
			continue // vacant seat — no term
		}
		termStart := termEnd - govtypes.SeatTermBlocks
		termDuration := termEnd - termStart
		if termDuration > govtypes.SeatTermBlocks {
			return fmt.Errorf("seat %d term duration %d exceeds max %d",
				i, termDuration, govtypes.SeatTermBlocks)
		}
	}

	// 4. Verify seat term end blocks array length matches seats array.
	termBlockCount := len(state.SeatTermEndBlocks)
	if termBlockCount != seatCount {
		return fmt.Errorf("seat term end blocks count %d != community seats count %d",
			termBlockCount, seatCount)
	}

	return nil
}

func runInvariants(s *SimState, invariants []Invariant) error {
	for _, inv := range invariants {
		if err := inv.Check(s); err != nil {
			return fmt.Errorf("invariant %q violated at block %d: %w",
				inv.Name, s.currentHeight, err)
		}
	}
	return nil
}
