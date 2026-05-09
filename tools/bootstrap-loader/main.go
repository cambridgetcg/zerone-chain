// bootstrap-loader seeds the genesis claiming_pot module state with one
// pot per whitelisted agent. The doctrine is in docs/tokenomics/GENESIS.md
// and x/claiming_pot/types/types.go::MakeBootstrapPotForAgent.
//
// The whitelist file is a plain UTF-8 text file with one bech32 address
// per line. Blank lines and lines starting with '#' are ignored. Each
// address gets a single-claimant ClaimingPot sized PerAgentBootstrapUzrn
// (0.222 ZRN), instantly vested at genesis block 0.
//
// Usage:
//
//	bootstrap-loader inject <whitelist.txt> <genesis.json>
//	bootstrap-loader validate <whitelist.txt>
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	cpottypes "github.com/zerone-chain/zerone/x/claiming_pot/types"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "validate":
		err = runValidate(args)
	case "inject":
		err = runInject(args)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: bootstrap-loader <command> [args]

Commands:
  validate <whitelist.txt>                       Validate addresses, report duplicates
  inject   <whitelist.txt> <genesis.json>        Insert one bootstrap pot per address

Each address gets a single-claimant ClaimingPot sized 0.222 ZRN, instantly
vested at genesis. Doctrine: docs/tokenomics/GENESIS.md (commitment 20:
issuance follows participation).
`)
}

func init() {
	// The whitelist must be parsed under the Zerone bech32 prefix so the
	// addresses round-trip through sdk.AccAddressFromBech32 cleanly.
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount("zrn", "zrnpub")
	cfg.SetBech32PrefixForValidator("zrnvaloper", "zrnvaloperpub")
	cfg.SetBech32PrefixForConsensusNode("zrnvalcons", "zrnvalconspub")
}

// loadWhitelist reads addresses from a UTF-8 text file. Blank lines and
// lines beginning with '#' are skipped. Returns the bech32 addresses in
// the order they appeared, with duplicates surfaced as an error.
func loadWhitelist(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var addrs []string
	seen := make(map[string]int) // address → first-seen line number
	dupes := []string{}

	sc := bufio.NewScanner(f)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		raw := strings.TrimSpace(sc.Text())
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		if _, err := sdk.AccAddressFromBech32(raw); err != nil {
			return nil, fmt.Errorf("line %d: invalid bech32 address %q: %w", lineNo, raw, err)
		}
		if first, ok := seen[raw]; ok {
			dupes = append(dupes, fmt.Sprintf("%q (lines %d, %d)", raw, first, lineNo))
			continue
		}
		seen[raw] = lineNo
		addrs = append(addrs, raw)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}
	if len(dupes) > 0 {
		return nil, fmt.Errorf("duplicate addresses in whitelist: %s", strings.Join(dupes, "; "))
	}
	return addrs, nil
}

func runValidate(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: bootstrap-loader validate <whitelist.txt>")
	}
	addrs, err := loadWhitelist(args[0])
	if err != nil {
		return err
	}
	fmt.Printf("✓ %d addresses validated (no duplicates, all bech32-correct)\n", len(addrs))
	if len(addrs) == 0 {
		fmt.Println("  (empty whitelist — no bootstrap pots will be seeded)")
		return nil
	}
	totalUzrn := uint64(len(addrs)) * 222000
	fmt.Printf("  per-agent: 0.222 ZRN (%s uzrn)\n", cpottypes.PerAgentBootstrapUzrn)
	fmt.Printf("  total addressable: %d agents × 0.222 ZRN = %.3f ZRN (%d uzrn)\n",
		len(addrs), float64(totalUzrn)/1_000_000.0, totalUzrn)
	fmt.Printf("  cap fraction: %.6f%% of 222,222,222 ZRN\n",
		float64(totalUzrn)/(222_222_222_000_000.0)*100.0)
	return nil
}

func runInject(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: bootstrap-loader inject <whitelist.txt> <genesis.json>")
	}
	whitelistPath := args[0]
	genesisPath := args[1]

	addrs, err := loadWhitelist(whitelistPath)
	if err != nil {
		return err
	}
	if len(addrs) == 0 {
		return fmt.Errorf("whitelist is empty — no bootstrap pots to seed; nothing to do")
	}

	// Build per-agent pots.
	pots := make([]*cpottypes.ClaimingPot, 0, len(addrs))
	for _, addr := range addrs {
		pots = append(pots, cpottypes.MakeBootstrapPotForAgent(addr, 0))
	}

	// Read genesis.
	genesisData, err := os.ReadFile(genesisPath)
	if err != nil {
		return fmt.Errorf("read genesis: %w", err)
	}
	var genesis map[string]json.RawMessage
	if err := json.Unmarshal(genesisData, &genesis); err != nil {
		return fmt.Errorf("parse genesis: %w", err)
	}
	var appState map[string]json.RawMessage
	if err := json.Unmarshal(genesis["app_state"], &appState); err != nil {
		return fmt.Errorf("parse app_state: %w", err)
	}

	// Encode the pots through the codec so the JSON shape matches what
	// the chain's GenesisState expects.
	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	cpotStateRaw, ok := appState[cpottypes.ModuleName]
	if !ok {
		return fmt.Errorf("genesis app_state has no %s module", cpottypes.ModuleName)
	}
	var cpotState cpottypes.GenesisState
	if err := cdc.UnmarshalJSON(cpotStateRaw, &cpotState); err != nil {
		return fmt.Errorf("unmarshal claiming_pot genesis: %w", err)
	}

	// Refuse to overwrite an existing seeded whitelist; the operator must
	// reset the genesis or de-seed first.
	for _, existing := range cpotState.Pots {
		if strings.HasPrefix(existing.Id, cpottypes.BootstrapPotIDPrefix) {
			return fmt.Errorf("genesis already contains a bootstrap pot (%q) — refusing to double-seed; clear bootstrap-* pots from claiming_pot.pots first",
				existing.Id)
		}
	}

	cpotState.Pots = append(cpotState.Pots, pots...)

	updated, err := cdc.MarshalJSON(&cpotState)
	if err != nil {
		return fmt.Errorf("marshal claiming_pot genesis: %w", err)
	}
	appState[cpottypes.ModuleName] = updated

	appStateJSON, err := json.Marshal(appState)
	if err != nil {
		return fmt.Errorf("marshal app_state: %w", err)
	}
	genesis["app_state"] = appStateJSON

	out, err := json.MarshalIndent(genesis, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal genesis: %w", err)
	}
	if err := os.WriteFile(genesisPath, out, 0o644); err != nil {
		return fmt.Errorf("write genesis: %w", err)
	}

	fmt.Printf("✓ Seeded %d bootstrap pots (0.222 ZRN per agent) into %s\n", len(pots), genesisPath)
	return nil
}
