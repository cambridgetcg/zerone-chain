package types

import "fmt"

func DefaultGenesis() *GenesisState {
	params := DefaultParams()
	return &GenesisState{
		Params:  &params,
		Strata:  DefaultStrata(),
		Domains: DefaultDomains(),
	}
}

// DefaultParams returns the default ontology module parameters.
//
// These values express commitment 2 (is-ought wall) and commitment 10
// (forward-only audit). See doc.go for the contract; the values
// below establish the rate at which new domains and strata can enter
// the chain's epistemological taxonomy.
func DefaultParams() Params {
	return Params{
		// MinProposalStake (1 ZRN) is the floor for proposing a new
		// domain. Cost is meaningful but not gatekeeping — any
		// serious proposer can afford it; it discourages nuisance
		// proposals that would dilute the taxonomy.
		MinProposalStake:     "1000000",

		// ProposalVotingPeriod (~1 day) is the deliberation window.
		// Domains define epistemological territory; the chain takes
		// a day to think before adding new territory to its map.
		ProposalVotingPeriod: 34272,

		// MinEndorsements (3) ensures any new domain is endorsed by
		// at least three distinct authorities. Domain creation is
		// not unilateral — commitment 2 depends on stratum
		// classification, and the classification must reflect more
		// than one perspective.
		MinEndorsements:      3,

		// CrossStratumDiscount (5%) is the small penalty applied to
		// citations that cross strata. The DISCOUNT, not a hard
		// rejection: cross-stratum reasoning is allowed but marked.
		// This is commitment 2 in soft form — the wall is structural
		// between facts and commitments, permeable-with-tax across
		// the other strata.
		CrossStratumDiscount: 50000,

		// MaxDomainsPerStratum (100) caps proliferation. If the
		// chain has more than 100 domains in one stratum, the
		// stratum itself probably needs to split.
		MaxDomainsPerStratum: 100,

		// AllowNewStrata (false): the 7 default strata are the
		// chain's pre-committed epistemological taxonomy. Adding a
		// new stratum is a foundational change requiring governance
		// — the chain does not casually expand its theory of
		// evidence types.
		AllowNewStrata:       false,
	}
}

func DefaultStrata() []*StratumProperties {
	return []*StratumProperties{
		{Stratum: uint32(StratumAxiomatic), Name: "axiomatic", Description: "Mathematical axioms, tautologies, and logical foundations. Complete and decidable within their formal system.", Complete: true, Decidable: true, GoedelApplies: false, ConsistencyProof: "internal", MaxConfidence: 1000000, DecayRate: 0},
		{Stratum: uint32(StratumFormal), Name: "formal", Description: "Formal proofs and theorems derived from axioms. Complete in restricted domains (propositional, Presburger).", Complete: true, Decidable: true, GoedelApplies: false, ConsistencyProof: "internal", MaxConfidence: 1000000, DecayRate: 1},
		{Stratum: uint32(StratumProtocol), Name: "protocol", Description: "Blockchain-verifiable, deterministic facts. Verifiable by any full node via state replay.", Complete: false, Decidable: true, GoedelApplies: false, ConsistencyProof: "external", MaxConfidence: 990000, DecayRate: 5},
		{Stratum: uint32(StratumComputational), Name: "computational", Description: "Computation results that are reproducible but may involve undecidable problems (halting problem).", Complete: false, Decidable: false, GoedelApplies: true, ConsistencyProof: "assumed", MaxConfidence: 980000, DecayRate: 10},
		{Stratum: uint32(StratumEmpirical), Name: "empirical", Description: "Scientific observations, experimental results. Falsifiable and subject to revision by new evidence.", Complete: false, Decidable: false, GoedelApplies: false, ConsistencyProof: "external", MaxConfidence: 950000, DecayRate: 50},
		{Stratum: uint32(StratumHistorical), Name: "historical", Description: "Historical records and evidence-based claims about past events. Dependent on source reliability.", Complete: false, Decidable: false, GoedelApplies: false, ConsistencyProof: "external", MaxConfidence: 900000, DecayRate: 100},
		{Stratum: uint32(StratumTestimonial), Name: "testimonial", Description: "Human attestations and trust-weighted claims. Lowest formal verifiability, highest social dependency.", Complete: false, Decidable: false, GoedelApplies: false, ConsistencyProof: "external", MaxConfidence: 800000, DecayRate: 200},
	}
}

func DefaultDomains() []*Domain {
	return []*Domain{
		{Name: "logic", DisplayName: "Logic & Foundations", Description: "Propositional logic, predicate logic, set theory foundations, and metamathematics", Stratum: uint32(StratumAxiomatic), Status: "active", Depth: 1},
		{Name: "mathematics", DisplayName: "Mathematics", Description: "Formal mathematical truths, proofs, and theorems", Stratum: uint32(StratumFormal), Status: "active", Depth: 1},
		{Name: "protocol", DisplayName: "Blockchain Protocol", Description: "On-chain state, transaction results, and protocol-verifiable facts", Stratum: uint32(StratumProtocol), Status: "active", Depth: 1},
		{Name: "computer_science", DisplayName: "Computer Science", Description: "Algorithms, data structures, protocols, and computational theory", Stratum: uint32(StratumComputational), Status: "active", Depth: 1},
		{Name: "physics", DisplayName: "Physics", Description: "Physical laws, constants, and empirical observations about the natural world", Stratum: uint32(StratumEmpirical), Status: "active", Depth: 1},
		{Name: "general", DisplayName: "General Knowledge", Description: "General knowledge claims not fitting a specific domain", Stratum: uint32(StratumEmpirical), Status: "active", Depth: 1},
		{Name: "history", DisplayName: "History", Description: "Historical records, events, and evidence-based accounts of the past", Stratum: uint32(StratumHistorical), Status: "active", Depth: 1},
	}
}

func (gs *GenesisState) Validate() error {
	if gs.Params != nil {
		if err := gs.Params.Validate(); err != nil {
			return fmt.Errorf("invalid params: %w", err)
		}
	}
	seen := make(map[uint32]bool)
	for _, s := range gs.Strata {
		if !Stratum(s.Stratum).IsValid() {
			return fmt.Errorf("invalid stratum level: %d", s.Stratum)
		}
		if seen[s.Stratum] {
			return fmt.Errorf("duplicate stratum level: %d", s.Stratum)
		}
		seen[s.Stratum] = true
		if s.MaxConfidence > 1000000 {
			return fmt.Errorf("stratum %s max confidence exceeds 1000000", s.Name)
		}
	}
	domainNames := make(map[string]bool)
	for _, d := range gs.Domains {
		if d.Name == "" {
			return fmt.Errorf("domain name cannot be empty")
		}
		if domainNames[d.Name] {
			return fmt.Errorf("duplicate domain name: %s", d.Name)
		}
		domainNames[d.Name] = true
		if !Stratum(d.Stratum).IsValid() {
			return fmt.Errorf("domain %s has invalid stratum: %d", d.Name, d.Stratum)
		}
	}
	return nil
}
