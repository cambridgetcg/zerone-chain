package types

import "cosmossdk.io/errors"

var (
	ErrInvalidVaultID         = errors.Register(ModuleName, 2, "invalid vault id")
	ErrVaultExists            = errors.Register(ModuleName, 3, "vault already registered")
	ErrVaultNotFound          = errors.Register(ModuleName, 4, "vault not found")
	ErrNotVaultOperator       = errors.Register(ModuleName, 5, "caller is not the vault operator")
	ErrVaultNotActive         = errors.Register(ModuleName, 6, "vault is not active")
	ErrInvalidManifestID      = errors.Register(ModuleName, 7, "invalid manifest id")
	ErrManifestExists         = errors.Register(ModuleName, 8, "manifest already published")
	ErrManifestNotFound       = errors.Register(ModuleName, 9, "manifest not found")
	ErrManifestAlreadyWithdrawn = errors.Register(ModuleName, 10, "manifest already withdrawn")
	ErrInvalidContentHash     = errors.Register(ModuleName, 11, "invalid content hash")
	ErrInvalidPubkey          = errors.Register(ModuleName, 12, "invalid operator public key")
	ErrInvalidEndpoint        = errors.Register(ModuleName, 13, "invalid server endpoint")
	ErrInvalidPolicyURL       = errors.Register(ModuleName, 14, "invalid access policy URL")
	ErrDescriptionTooLong     = errors.Register(ModuleName, 15, "description exceeds maximum length")
	ErrNoteTooLong            = errors.Register(ModuleName, 16, "note exceeds maximum length")
	ErrRegistrationDisabled   = errors.Register(ModuleName, 17, "vault registration is currently disabled")
	ErrManifestVaultMismatch  = errors.Register(ModuleName, 18, "manifest does not belong to the named vault")
	ErrInvalidAuthority       = errors.Register(ModuleName, 19, "invalid authority")
)
