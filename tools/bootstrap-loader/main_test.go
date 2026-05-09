package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// derive a deterministic but bech32-correct test address.
func testAddr(seed string) string {
	h := sha256.Sum256([]byte("bootstrap-loader-test:" + seed))
	return sdk.AccAddress(h[:20]).String()
}

// TestLoadWhitelist_HappyPath verifies the parser accepts blank lines,
// comments, and ordered-unique addresses.
func TestLoadWhitelist_HappyPath(t *testing.T) {
	a, b := testAddr("alice"), testAddr("bob")
	tmp := t.TempDir()
	path := filepath.Join(tmp, "whitelist.txt")

	content := fmt.Sprintf(`# Genesis bootstrap whitelist — example
# (commitment 20: issuance follows participation)

%s

# trailing comment
%s
`, a, b)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	addrs, err := loadWhitelist(path)
	if err != nil {
		t.Fatalf("loadWhitelist: %v", err)
	}
	if len(addrs) != 2 {
		t.Errorf("got %d addresses, want 2", len(addrs))
	}
	if addrs[0] != a || addrs[1] != b {
		t.Errorf("order mismatch: got %v", addrs)
	}
}

// TestLoadWhitelist_RejectsDuplicates surfaces duplicate addresses with
// line numbers — the genesis ceremony must not silently de-dupe a
// whitelist file (commitment 20: each whitelisted agent is meant to
// claim once; a duplicate is operator error worth surfacing).
func TestLoadWhitelist_RejectsDuplicates(t *testing.T) {
	a := testAddr("alice")
	tmp := t.TempDir()
	path := filepath.Join(tmp, "whitelist.txt")
	content := fmt.Sprintf("%s\n%s\n", a, a)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := loadWhitelist(path)
	if err == nil {
		t.Fatal("expected duplicate error")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("expected duplicate error, got: %v", err)
	}
}

// TestLoadWhitelist_RejectsInvalidBech32 catches typo'd or non-zerone
// addresses before they end up in genesis.
func TestLoadWhitelist_RejectsInvalidBech32(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "whitelist.txt")
	content := `zrn1validlookingbutnotreallybech32
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := loadWhitelist(path)
	if err == nil {
		t.Fatal("expected validation error")
	}
}
