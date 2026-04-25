package types

import "fmt"

// DefaultParams returns the default counterexamples module parameters.
//
// These values express commitment 15 (counterexamples are part of the
// corpus). The defaults are calibrated so the chain ECONOMICALLY
// ENCOURAGES counterexample contribution: validation reward exceeds
// bond at the margin, since alignment-by-structure is a public good.
func DefaultParams() *Params {
	return &Params{
		// 1 ZRN bond on proposal. Returned on VALIDATED, burned on
		// REJECTED. Meaningful but not gatekeeping.
		ProposalBond: "1000000",

		// 0.5 ZRN reward on validation. Combined with the returned
		// bond, a validated counterexample net-pays 0.5 ZRN — the
		// chain explicitly subsidizes alignment-by-structure work.
		ValidationReward: "500000",

		// 3 votes minimum before resolution. Single-vote resolution
		// would let any one validator decide; three is the smallest
		// quorum that doesn't reduce to one.
		MinVotes: 3,

		// 66.6% threshold. A counterexample is VALIDATED if at least
		// 66.6% of votes affirm it. This matches the chain's broader
		// supermajority convention (commitment 8: panel weights
		// skill, not bond — the threshold is meaningful agreement,
		// not bare majority).
		AffirmThresholdBps: 666_000,

		// 4 KB per text field. Counterexamples need room to explain
		// the error properly; truncating the reasoning would defeat
		// the purpose.
		MaxReasonBytes: 4096,

		// 1.2x TVW multiplier for facts with at least one validated
		// counterexample. Read by x/knowledge during
		// ComputeTrainingValueWeight. The 20% boost is large enough
		// to be a real incentive for fact authors to seek
		// counterexample coverage, small enough that bare facts
		// aren't economically excluded.
		TvwMultiplierBps: 1_200_000,

		ProposalsEnabled: true,
	}
}

// DefaultGenesis returns the default genesis state.
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:                 DefaultParams(),
		Counterexamples:        []*Counterexample{},
		Validations:            []*Validation{},
		NextCounterexampleSeq:  1,
		NextValidationId:       1,
	}
}

// Validate validates the genesis state.
func (gs *GenesisState) Validate() error {
	if gs.Params == nil {
		return fmt.Errorf("params required")
	}
	if err := gs.Params.Validate(); err != nil {
		return err
	}
	seenCE := map[string]bool{}
	for _, c := range gs.Counterexamples {
		if c == nil {
			continue
		}
		if c.Id == "" {
			return fmt.Errorf("counterexample with empty id")
		}
		if seenCE[c.Id] {
			return fmt.Errorf("duplicate counterexample id: %s", c.Id)
		}
		seenCE[c.Id] = true
	}
	highestVal := uint64(0)
	for _, v := range gs.Validations {
		if v == nil {
			continue
		}
		if !seenCE[v.CounterexampleId] {
			return fmt.Errorf("validation references unknown counterexample %s", v.CounterexampleId)
		}
		if v.Id > highestVal {
			highestVal = v.Id
		}
	}
	if gs.NextValidationId != 0 && gs.NextValidationId <= highestVal {
		return fmt.Errorf("next_validation_id (%d) must be > highest validation id (%d)", gs.NextValidationId, highestVal)
	}
	return nil
}

// Validate validates Params.
func (p *Params) Validate() error {
	if _, ok := ParseBondAmount(p.ProposalBond); !ok {
		return fmt.Errorf("invalid proposal_bond")
	}
	if _, ok := ParseBondAmount(p.ValidationReward); !ok {
		return fmt.Errorf("invalid validation_reward")
	}
	if p.MinVotes == 0 {
		return fmt.Errorf("min_votes must be > 0")
	}
	if p.AffirmThresholdBps == 0 || p.AffirmThresholdBps > 1_000_000 {
		return fmt.Errorf("affirm_threshold_bps must be in (0, 1_000_000]")
	}
	if p.MaxReasonBytes == 0 {
		return fmt.Errorf("max_reason_bytes must be > 0")
	}
	if p.TvwMultiplierBps == 0 {
		return fmt.Errorf("tvw_multiplier_bps must be > 0")
	}
	return nil
}
