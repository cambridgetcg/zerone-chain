package agentsdk

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// defaultBaseStake is the fallback minimum submission stake (1 ZRN = 1,000,000 uzrn).
const defaultBaseStake = "1000000"

// parseTDUType maps SDK type strings to proto SampleType.
func parseTDUType(s string) (types.SampleType, error) {
	switch strings.ToLower(strings.ReplaceAll(s, "-", "_")) {
	case "instruction_response", "instruction-response":
		return types.SampleType_SAMPLE_TYPE_Q_AND_A, nil
	case "conversation":
		return types.SampleType_SAMPLE_TYPE_DISCUSSION, nil
	case "correction":
		return types.SampleType_SAMPLE_TYPE_CORRECTION, nil
	case "grounding_fact", "grounding-fact":
		return types.SampleType_SAMPLE_TYPE_ANNOTATION, nil
	case "reasoning_chain", "reasoning-chain":
		return types.SampleType_SAMPLE_TYPE_EXPLANATION, nil
	default:
		return 0, fmt.Errorf("unknown TDU type %q: must be instruction-response, conversation, correction, grounding-fact, or reasoning-chain", s)
	}
}

// parseConsentType maps SDK consent strings to proto ConsentType.
func parseConsentType(s string) (types.ConsentType, error) {
	switch strings.ToLower(strings.ReplaceAll(s, "-", "_")) {
	case "", "unspecified":
		return types.ConsentType_CONSENT_TYPE_UNSPECIFIED, nil
	case "original", "self":
		return types.ConsentType_CONSENT_TYPE_SELF_AUTHORED, nil
	case "public_domain", "public-domain":
		return types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE, nil
	case "licensed", "opt_in", "optin":
		return types.ConsentType_CONSENT_TYPE_OPT_IN, nil
	case "tos", "platform_tos":
		return types.ConsentType_CONSENT_TYPE_PLATFORM_TOS, nil
	case "fair_use", "fairuse":
		return types.ConsentType_CONSENT_TYPE_FAIR_USE, nil
	default:
		return 0, fmt.Errorf("unknown consent type %q", s)
	}
}

// parseContestType maps SDK contest strings to proto ContestType.
func parseContestType(s string) (types.ContestType, error) {
	switch strings.ToLower(s) {
	case "", "unspecified":
		return types.ContestType_CONTEST_TYPE_UNSPECIFIED, nil
	case "consent":
		return types.ContestType_CONTEST_TYPE_CONSENT, nil
	case "quality":
		return types.ContestType_CONTEST_TYPE_QUALITY, nil
	case "duplicate":
		return types.ContestType_CONTEST_TYPE_DUPLICATE, nil
	case "toxic":
		return types.ContestType_CONTEST_TYPE_TOXIC, nil
	case "copyright":
		return types.ContestType_CONTEST_TYPE_COPYRIGHT, nil
	default:
		return 0, fmt.Errorf("unknown contest type %q", s)
	}
}

// difficultyMultiplier returns a ×10 integer multiplier for the given difficulty level.
func difficultyMultiplier(s string) (int64, error) {
	switch strings.ToLower(s) {
	case "", "basic":
		return 10, nil
	case "standard":
		return 15, nil
	case "advanced":
		return 20, nil
	case "expert":
		return 30, nil
	case "frontier":
		return 50, nil
	default:
		return 0, fmt.Errorf("unknown difficulty %q: must be basic, standard, advanced, expert, or frontier", s)
	}
}

// calculateStake computes stake = (baseStake × multiplierX10) / 10.
func calculateStake(baseStake string, multiplierX10 int64) (string, error) {
	base := new(big.Int)
	if _, ok := base.SetString(baseStake, 10); !ok {
		return "", fmt.Errorf("invalid base stake: %s", baseStake)
	}
	result := new(big.Int).Mul(base, big.NewInt(multiplierX10))
	result.Div(result, big.NewInt(10))
	return result.String(), nil
}

// contentHashHex computes the SHA-256 hash of data and returns the hex string.
func contentHashHex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
