package types

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Validation primitives. The chain enforces the SHAPE of inputs, not
// the meaning. Operators are responsible for the off-chain semantics
// of vault content; the chain only ensures that what gets stored is
// well-formed enough to be queried.

var (
	vaultIDPattern    = regexp.MustCompile(`^[A-Za-z0-9._/\-]{3,128}$`)
	manifestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._/\-#]{3,192}$`)
)

// ValidateVaultID checks that an id matches the chain's syntactic
// constraint: alphanumeric plus . _ / - characters, 3..128 bytes.
func ValidateVaultID(id string) error {
	if !vaultIDPattern.MatchString(id) {
		return fmt.Errorf("%w: %q", ErrInvalidVaultID, id)
	}
	return nil
}

// ValidateManifestID — same shape as vault, plus '#' for vault#n
// convention. 3..192 bytes.
func ValidateManifestID(id string) error {
	if !manifestIDPattern.MatchString(id) {
		return fmt.Errorf("%w: %q", ErrInvalidManifestID, id)
	}
	return nil
}

// ValidateContentHash accepts a hex-encoded hash of 32..128 bytes
// (decoded). 32 bytes is SHA-256 / BLAKE2s; 64 bytes is SHA-512 /
// BLAKE2b. Operators choose the hash; the chain only enforces shape.
func ValidateContentHash(h string) error {
	if h == "" {
		return ErrInvalidContentHash
	}
	raw, err := hex.DecodeString(h)
	if err != nil {
		return fmt.Errorf("%w: not hex-encoded", ErrInvalidContentHash)
	}
	if len(raw) < 16 || len(raw) > 128 {
		return fmt.Errorf("%w: decoded length %d outside [16,128]", ErrInvalidContentHash, len(raw))
	}
	return nil
}

// ValidatePolicyURL accepts http/https URLs only.
func ValidatePolicyURL(u string) error {
	if u == "" {
		return nil // optional field
	}
	parsed, err := url.Parse(u)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%w: %q", ErrInvalidPolicyURL, u)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%w: scheme must be http or https", ErrInvalidPolicyURL)
	}
	return nil
}

// ValidateEndpoint — same constraint as policy URL.
func ValidateEndpoint(u string) error {
	if u == "" {
		return nil
	}
	parsed, err := url.Parse(u)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%w: %q", ErrInvalidEndpoint, u)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%w: scheme must be http or https", ErrInvalidEndpoint)
	}
	return nil
}

// ValidatePubkey ensures the field is non-empty and within a reasonable
// length window. The exact format is operator-chosen.
func ValidatePubkey(p string) error {
	if p == "" {
		return ErrInvalidPubkey
	}
	if len(p) > 1024 {
		return fmt.Errorf("%w: pubkey too long", ErrInvalidPubkey)
	}
	return nil
}

// ─── Msg validation ────────────────────────────────────────────────

func (msg *MsgRegisterVault) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Operator); err != nil {
		return fmt.Errorf("invalid operator address: %w", err)
	}
	if err := ValidateVaultID(msg.Id); err != nil {
		return err
	}
	if err := ValidatePubkey(msg.OperatorPubkey); err != nil {
		return err
	}
	if err := ValidatePolicyURL(msg.AccessPolicyUrl); err != nil {
		return err
	}
	if err := ValidateEndpoint(msg.ServerEndpoint); err != nil {
		return err
	}
	return nil
}

func (msg *MsgRegisterVault) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Operator)
	return []sdk.AccAddress{addr}
}

func (msg *MsgUpdateVault) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Operator); err != nil {
		return fmt.Errorf("invalid operator address: %w", err)
	}
	if err := ValidateVaultID(msg.VaultId); err != nil {
		return err
	}
	if err := ValidatePolicyURL(msg.AccessPolicyUrl); err != nil {
		return err
	}
	if err := ValidateEndpoint(msg.ServerEndpoint); err != nil {
		return err
	}
	if msg.OperatorPubkey != "" {
		if err := ValidatePubkey(msg.OperatorPubkey); err != nil {
			return err
		}
	}
	return nil
}

func (msg *MsgUpdateVault) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Operator)
	return []sdk.AccAddress{addr}
}

func (msg *MsgPublishManifest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Operator); err != nil {
		return fmt.Errorf("invalid operator address: %w", err)
	}
	if err := ValidateVaultID(msg.VaultId); err != nil {
		return err
	}
	if err := ValidateManifestID(msg.ManifestId); err != nil {
		return err
	}
	if err := ValidateContentHash(msg.ContentHash); err != nil {
		return err
	}
	if msg.Version == "" {
		return fmt.Errorf("version cannot be empty")
	}
	if len(msg.Version) > 64 {
		return fmt.Errorf("version too long")
	}
	return nil
}

func (msg *MsgPublishManifest) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Operator)
	return []sdk.AccAddress{addr}
}

func (msg *MsgWithdrawManifest) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Operator); err != nil {
		return fmt.Errorf("invalid operator address: %w", err)
	}
	if err := ValidateManifestID(msg.ManifestId); err != nil {
		return err
	}
	return nil
}

func (msg *MsgWithdrawManifest) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Operator)
	return []sdk.AccAddress{addr}
}

func (msg *MsgRecordAccess) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Operator); err != nil {
		return fmt.Errorf("invalid operator address: %w", err)
	}
	if err := ValidateVaultID(msg.VaultId); err != nil {
		return err
	}
	if msg.ManifestId != "" {
		if err := ValidateManifestID(msg.ManifestId); err != nil {
			return err
		}
	}
	if msg.Accessor != "" {
		if _, err := sdk.AccAddressFromBech32(msg.Accessor); err != nil {
			return fmt.Errorf("invalid accessor address: %w", err)
		}
	}
	return nil
}

func (msg *MsgRecordAccess) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Operator)
	return []sdk.AccAddress{addr}
}

func (msg *MsgUpdateParams) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %w", err)
	}
	if msg.Params != nil {
		if err := msg.Params.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (msg *MsgUpdateParams) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Authority)
	return []sdk.AccAddress{addr}
}
