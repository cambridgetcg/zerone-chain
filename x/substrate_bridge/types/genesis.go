package types

import "fmt"

func DefaultGenesis() *GenesisState {
	p := DefaultParams()
	return &GenesisState{
		Params:   &p,
		Adapters: nil,
	}
}

func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, a := range gs.Adapters {
		if seen[a.AdapterId] {
			return fmt.Errorf("duplicate adapter_id in genesis: %s", a.AdapterId)
		}
		seen[a.AdapterId] = true
	}
	return nil
}
