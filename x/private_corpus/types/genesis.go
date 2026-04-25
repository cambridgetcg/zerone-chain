package types

import "fmt"

// DefaultParams returns the default private_corpus module parameters.
//
// These values are sized so the module can hold genuinely useful
// metadata (paragraph descriptions, multi-line notes) without becoming
// a generic blob storage. The chain anchors hashes; it does not store
// content. The limits below are for HUMAN-READABLE METADATA only.
func DefaultParams() *Params {
	return &Params{
		MaxDescriptionBytes:        4096,  // 4 KB — paragraph-length descriptions
		MaxManifestDescriptionBytes: 1024, // 1 KB — short version notes
		MaxAccessNoteBytes:         512,   // 512 B — short access annotations
		MaxAccessRecordsPerQuery:   200,
		RegistrationEnabled:        true,
	}
}

// DefaultGenesis returns the default genesis state.
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:        DefaultParams(),
		Vaults:        []*Vault{},
		Manifests:     []*CorpusManifest{},
		AccessRecords: []*AccessRecord{},
		NextAccessSeq: 1,
	}
}

// Validate validates the genesis state.
func (gs *GenesisState) Validate() error {
	if gs.Params == nil {
		return fmt.Errorf("params cannot be nil")
	}
	if err := gs.Params.Validate(); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}
	seenVaults := make(map[string]bool)
	for _, v := range gs.Vaults {
		if v == nil {
			continue
		}
		if err := ValidateVaultID(v.Id); err != nil {
			return fmt.Errorf("invalid vault: %w", err)
		}
		if seenVaults[v.Id] {
			return fmt.Errorf("duplicate vault: %s", v.Id)
		}
		seenVaults[v.Id] = true
	}
	seenManifests := make(map[string]bool)
	for _, m := range gs.Manifests {
		if m == nil {
			continue
		}
		if err := ValidateManifestID(m.Id); err != nil {
			return fmt.Errorf("invalid manifest: %w", err)
		}
		if seenManifests[m.Id] {
			return fmt.Errorf("duplicate manifest: %s", m.Id)
		}
		seenManifests[m.Id] = true
		if !seenVaults[m.VaultId] {
			return fmt.Errorf("manifest %s references unknown vault %s", m.Id, m.VaultId)
		}
	}
	highestSeq := uint64(0)
	for _, r := range gs.AccessRecords {
		if r == nil {
			continue
		}
		if !seenVaults[r.VaultId] {
			return fmt.Errorf("access record %d references unknown vault %s", r.Seq, r.VaultId)
		}
		if r.Seq > highestSeq {
			highestSeq = r.Seq
		}
	}
	if gs.NextAccessSeq != 0 && gs.NextAccessSeq <= highestSeq {
		return fmt.Errorf("next_access_seq (%d) must be > highest existing seq (%d)", gs.NextAccessSeq, highestSeq)
	}
	return nil
}

// Validate validates the params.
func (p *Params) Validate() error {
	if p.MaxDescriptionBytes == 0 {
		return fmt.Errorf("max_description_bytes must be > 0")
	}
	if p.MaxManifestDescriptionBytes == 0 {
		return fmt.Errorf("max_manifest_description_bytes must be > 0")
	}
	if p.MaxAccessNoteBytes == 0 {
		return fmt.Errorf("max_access_note_bytes must be > 0")
	}
	if p.MaxAccessRecordsPerQuery == 0 {
		return fmt.Errorf("max_access_records_per_query must be > 0")
	}
	return nil
}
