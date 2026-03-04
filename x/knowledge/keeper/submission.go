package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ComputeContentHash returns the lowercase hex SHA-256 of content.
func (k Keeper) ComputeContentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

// CheckDuplicate returns ErrDuplicateContent if the hash already exists in state.
func (k Keeper) CheckDuplicate(ctx context.Context, contentHash string) error {
	if k.HasContentHash(ctx, contentHash) {
		return types.ErrDuplicateContent
	}
	return nil
}

// ValidateConsent checks that the consent proof is present and has the required fields.
func (k Keeper) ValidateConsent(consent *types.ConsentProof) error {
	if consent == nil {
		return types.ErrConsentRequired
	}
	switch consent.Type {
	case types.ConsentType_CONSENT_TYPE_SELF_AUTHORED:
		return nil
	case types.ConsentType_CONSENT_TYPE_OPT_IN:
		if consent.AuthorSignature == "" && consent.ProofUri == "" {
			return types.ErrInvalidConsent
		}
	case types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE:
		if consent.ProofUri == "" {
			return types.ErrInvalidConsent
		}
	case types.ConsentType_CONSENT_TYPE_PLATFORM_TOS:
		if consent.ProofUri == "" {
			return types.ErrInvalidConsent
		}
	case types.ConsentType_CONSENT_TYPE_FAIR_USE:
		return nil
	default:
		return types.ErrInvalidConsent
	}
	return nil
}
