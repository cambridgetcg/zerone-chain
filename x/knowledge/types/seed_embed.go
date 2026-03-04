package types

import _ "embed"

// genesis_seeds.json is embedded at compile time for use by prepare-genesis.
// Contains 25 curated seed samples across 9 training-data domains.
//
//go:embed genesis_seeds.json
var GenesisSeedsJSON []byte
