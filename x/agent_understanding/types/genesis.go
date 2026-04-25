package types

import "fmt"

// DefaultParams returns the default parameters.
func DefaultParams() *Params {
	return &Params{
		// 50 verified facts threshold for "sparse." Below this a
		// domain is considered frontier territory; agents working
		// there earn frontier_reach.
		SparseDomainFactThreshold: 50,

		// 100 domain profiles per query response. Bounds payload
		// size for queries on highly-active agents.
		MaxDomainsPerQuery: 100,
	}
}

func DefaultGenesis() *GenesisState {
	return &GenesisState{Params: DefaultParams()}
}

func (gs *GenesisState) Validate() error {
	if gs.Params == nil {
		return fmt.Errorf("params required")
	}
	return gs.Params.Validate()
}

func (p *Params) Validate() error {
	if p.MaxDomainsPerQuery == 0 {
		return fmt.Errorf("max_domains_per_query must be > 0")
	}
	return nil
}
