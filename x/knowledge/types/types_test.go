package types_test

import (
	"os"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

func TestMain(m *testing.M) {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount("zrn", "zrnpub")
	config.SetBech32PrefixForValidator("zrnvaloper", "zrnvaloperpub")
	config.SetBech32PrefixForConsensusNode("zrnvalcons", "zrnvalconspub")
	os.Exit(m.Run())
}

// ─── Params & Genesis ────────────────────────────────────────────────────────

func TestDefaultParams_AllSlashParamsNonZero(t *testing.T) {
	p := types.DefaultParams()

	require.Greater(t, p.WrongValidationSlashBps, uint64(0),
		"wrong_validation_slash_bps must be > 0")
	require.Greater(t, p.MissedRevealSlashBps, uint64(0),
		"missed_reveal_slash_bps must be > 0")
	require.Greater(t, p.EquivocationSlashBps, uint64(0),
		"equivocation_slash_bps must be > 0")
}

func TestDefaultParams_Validate(t *testing.T) {
	p := types.DefaultParams()
	require.NoError(t, p.Validate())
}

func TestDefaultParams_ThresholdOrdering(t *testing.T) {
	p := types.DefaultParams()
	require.Greater(t, p.GoldThreshold, p.SilverThreshold, "gold > silver")
	require.Greater(t, p.SilverThreshold, p.BronzeThreshold, "silver > bronze")
}

func TestDefaultGenesis_Validate(t *testing.T) {
	gs := types.DefaultGenesis()
	require.NotNil(t, gs)
	require.NoError(t, gs.Validate())
}

func TestDefaultGenesis_Domains(t *testing.T) {
	gs := types.DefaultGenesis()
	require.NotNil(t, gs)

	require.Len(t, gs.Domains, 9, "expected 9 genesis domains")

	for _, d := range gs.Domains {
		require.Equal(t, types.DomainStatus_DOMAIN_STATUS_ACTIVE, d.Status,
			"genesis domain %q must be active", d.Name)
	}

	require.NotNil(t, gs.Samples)
	require.NotNil(t, gs.Submissions)
	require.NotNil(t, gs.QualityRounds)
}

// ─── QualityTier ─────────────────────────────────────────────────────────────

func TestQualityTierFromScore(t *testing.T) {
	p := types.DefaultParams()

	require.Equal(t, types.TierGold, types.QualityTierFromScore(p.GoldThreshold, &p))
	require.Equal(t, types.TierGold, types.QualityTierFromScore(p.GoldThreshold+1, &p))
	require.Equal(t, types.TierSilver, types.QualityTierFromScore(p.SilverThreshold, &p))
	require.Equal(t, types.TierSilver, types.QualityTierFromScore(p.GoldThreshold-1, &p))
	require.Equal(t, types.TierBronze, types.QualityTierFromScore(p.BronzeThreshold, &p))
	require.Equal(t, types.TierBronze, types.QualityTierFromScore(0, &p))
}

func TestQualityVerdictToTier(t *testing.T) {
	require.Equal(t, types.TierGold, types.QualityVerdictToTier(types.QualityVerdict_QUALITY_VERDICT_GOLD))
	require.Equal(t, types.TierSilver, types.QualityVerdictToTier(types.QualityVerdict_QUALITY_VERDICT_SILVER))
	require.Equal(t, types.TierBronze, types.QualityVerdictToTier(types.QualityVerdict_QUALITY_VERDICT_BRONZE))
	require.Equal(t, types.QualityTier(""), types.QualityVerdictToTier(types.QualityVerdict_QUALITY_VERDICT_REJECT))
}

// ─── Keys ────────────────────────────────────────────────────────────────────

func TestKeyPrefixes_NoDuplicates(t *testing.T) {
	seen := map[byte]string{}
	prefixes := map[string][]byte{
		"SampleKeyPrefix":     types.SampleKeyPrefix,
		"SubmissionKeyPrefix": types.SubmissionKeyPrefix,
		"QualityRoundPrefix":  types.QualityRoundPrefix,
		"DomainKeyPrefix":     types.DomainKeyPrefix,
		"DatasetKeyPrefix":    types.DatasetKeyPrefix,
		"TrainingDemandKey":   types.TrainingDemandKey,
		"DataBountyKeyPrefix": types.DataBountyKeyPrefix,
		"ScrapedSourceKey":    types.ScrapedSourceKey,
		"ValidatorInfoKey":    types.ValidatorInfoKey,
		"ThreadIndexPrefix":   types.ThreadIndexPrefix,
		"DomainSampleIndex":   types.DomainSampleIndexPrefix,
		"SubmitterIndex":      types.SubmitterIndexPrefix,
		"NicheIndexPrefix":    types.NicheIndexPrefix,
		"ContentHashIndex":    types.ContentHashIndexPrefix,
		"ParamsKey":           types.ParamsKey,
		"SampleSeqKey":        types.SampleSeqKey,
		"SubmissionSeqKey":    types.SubmissionSeqKey,
		"RoundSeqKey":         types.RoundSeqKey,
		"DatasetSeqKey":       types.DatasetSeqKey,
	}
	for name, pfx := range prefixes {
		b := pfx[0]
		if existing, ok := seen[b]; ok {
			t.Errorf("prefix collision: %s and %s both use 0x%02x", name, existing, b)
		}
		seen[b] = name
	}
}

func TestNewKeyConstructors(t *testing.T) {
	require.Equal(t, []byte{0x01, 's', '1'}, types.SampleKey("s1"))
	require.Equal(t, []byte{0x02, 'x', '1'}, types.SubmissionKey("x1"))
	require.Equal(t, []byte{0x03, 'r', '1'}, types.QualityRoundKey("r1"))
	require.Equal(t, []byte{0x05, 'd', '1'}, types.DatasetKey("d1"))

	tk := types.ThreadIndexKey("t1", "s1")
	require.Equal(t, byte(0x0A), tk[0])

	sk := types.SubmitterIndexKey("alice", "s1")
	require.Equal(t, byte(0x0C), sk[0])
}

func TestDeprecatedKeyAliases(t *testing.T) {
	// Deprecated aliases must point to same byte prefix as new keys.
	require.Equal(t, types.SampleKeyPrefix, types.FactKeyPrefix)
	require.Equal(t, types.SubmissionKeyPrefix, types.ClaimKeyPrefix)
	require.Equal(t, types.QualityRoundPrefix, types.VerificationRoundKeyPrefix)
}

// ─── Message Validation ──────────────────────────────────────────────────────

const testAddr = "zrn1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqulc3kt" // valid bech32

func TestMsgSubmitData_ValidateBasic(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		msg := &types.MsgSubmitData{
			Submitter:  testAddr,
			Content:    "test content",
			SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION,
			Domain:     "technology",
			Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
			Language:   "en",
		}
		require.NoError(t, msg.ValidateBasic())
	})

	t.Run("empty content", func(t *testing.T) {
		msg := &types.MsgSubmitData{
			Submitter:  testAddr,
			Content:    "",
			SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION,
			Domain:     "technology",
			Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		}
		require.Error(t, msg.ValidateBasic())
	})

	t.Run("missing consent", func(t *testing.T) {
		msg := &types.MsgSubmitData{
			Submitter:  testAddr,
			Content:    "content",
			SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION,
			Domain:     "technology",
		}
		require.Error(t, msg.ValidateBasic())
	})

	t.Run("invalid language", func(t *testing.T) {
		msg := &types.MsgSubmitData{
			Submitter:  testAddr,
			Content:    "content",
			SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION,
			Domain:     "technology",
			Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
			Language:   "eng",
		}
		require.Error(t, msg.ValidateBasic())
	})

	t.Run("unspecified sample type", func(t *testing.T) {
		msg := &types.MsgSubmitData{
			Submitter:  testAddr,
			Content:    "content",
			SampleType: types.SampleType_SAMPLE_TYPE_UNSPECIFIED,
			Domain:     "technology",
			Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		}
		require.Error(t, msg.ValidateBasic())
	})
}

func TestMsgSubmitThread_ValidateBasic(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		msg := &types.MsgSubmitThread{
			Submitter: testAddr,
			Items: []*types.MsgSubmitData{
				{Content: "msg1", ThreadId: "t1"},
				{Content: "msg2", ThreadId: "t1"},
			},
			ThreadId: "t1",
			Domain:   "science",
		}
		require.NoError(t, msg.ValidateBasic())
	})

	t.Run("too few items", func(t *testing.T) {
		msg := &types.MsgSubmitThread{
			Submitter: testAddr,
			Items:     []*types.MsgSubmitData{{Content: "msg1"}},
			ThreadId:  "t1",
			Domain:    "science",
		}
		require.Error(t, msg.ValidateBasic())
	})

	t.Run("thread ID mismatch", func(t *testing.T) {
		msg := &types.MsgSubmitThread{
			Submitter: testAddr,
			Items: []*types.MsgSubmitData{
				{Content: "a", ThreadId: "t1"},
				{Content: "b", ThreadId: "wrong"},
			},
			ThreadId: "t1",
			Domain:   "science",
		}
		require.Error(t, msg.ValidateBasic())
	})
}

func TestMsgSubmitReveal_ValidateBasic(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		msg := &types.MsgSubmitReveal{
			Verifier: testAddr,
			RoundId:  "r1",
			Scores:   &types.QualityVote{OverallQuality: 700000},
			Salt:     []byte("salt"),
		}
		require.NoError(t, msg.ValidateBasic())
	})

	t.Run("score too high", func(t *testing.T) {
		msg := &types.MsgSubmitReveal{
			Verifier: testAddr,
			RoundId:  "r1",
			Scores:   &types.QualityVote{OverallQuality: 2_000_000},
			Salt:     []byte("salt"),
		}
		require.Error(t, msg.ValidateBasic())
	})

	t.Run("nil scores", func(t *testing.T) {
		msg := &types.MsgSubmitReveal{
			Verifier: testAddr,
			RoundId:  "r1",
			Salt:     []byte("salt"),
		}
		require.Error(t, msg.ValidateBasic())
	})

	t.Run("empty salt", func(t *testing.T) {
		msg := &types.MsgSubmitReveal{
			Verifier: testAddr,
			RoundId:  "r1",
			Scores:   &types.QualityVote{OverallQuality: 700000},
		}
		require.Error(t, msg.ValidateBasic())
	})
}

func TestMsgContestSample_ValidateBasic(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		msg := &types.MsgContestSample{
			Challenger:  testAddr,
			SampleId:    "s1",
			Reason:      "copyright violation",
			ContestType: types.ContestType_CONTEST_TYPE_COPYRIGHT,
		}
		require.NoError(t, msg.ValidateBasic())
	})

	t.Run("empty reason", func(t *testing.T) {
		msg := &types.MsgContestSample{
			Challenger:  testAddr,
			SampleId:    "s1",
			Reason:      "",
			ContestType: types.ContestType_CONTEST_TYPE_COPYRIGHT,
		}
		require.Error(t, msg.ValidateBasic())
	})

	t.Run("unspecified contest type", func(t *testing.T) {
		msg := &types.MsgContestSample{
			Challenger:  testAddr,
			SampleId:    "s1",
			Reason:      "reason",
			ContestType: types.ContestType_CONTEST_TYPE_UNSPECIFIED,
		}
		require.Error(t, msg.ValidateBasic())
	})
}

func TestMsgCreateDataset_ValidateBasic(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		msg := &types.MsgCreateDataset{
			Curator:    testAddr,
			Name:       "My Dataset",
			MinQuality: 600000,
		}
		require.NoError(t, msg.ValidateBasic())
	})

	t.Run("empty name", func(t *testing.T) {
		msg := &types.MsgCreateDataset{
			Curator: testAddr,
			Name:    "",
		}
		require.Error(t, msg.ValidateBasic())
	})

	t.Run("min quality too high", func(t *testing.T) {
		msg := &types.MsgCreateDataset{
			Curator:    testAddr,
			Name:       "Dataset",
			MinQuality: 2_000_000,
		}
		require.Error(t, msg.ValidateBasic())
	})
}

func TestMsgAccessDataset_ValidateBasic(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		msg := &types.MsgAccessDataset{Consumer: testAddr, DatasetId: "d1"}
		require.NoError(t, msg.ValidateBasic())
	})

	t.Run("empty dataset ID", func(t *testing.T) {
		msg := &types.MsgAccessDataset{Consumer: testAddr, DatasetId: ""}
		require.Error(t, msg.ValidateBasic())
	})
}

// ─── Quality Commitment Hash ─────────────────────────────────────────────────

func TestComputeQualityCommitmentHash(t *testing.T) {
	vote := &types.QualityVote{
		OverallQuality:  800000,
		ReasoningDepth:  700000,
		Novelty:         600000,
		Toxicity:        10000,
		FactualAccuracy: 900000,
		ConsentValid:    true,
		Duplicate:       false,
	}
	salt := []byte("test-salt-1234")
	roundID := "r1"

	hash1 := types.ComputeQualityCommitHash(roundID, vote, salt)
	require.NotNil(t, hash1)
	require.Len(t, hash1, 32)

	// Determinism
	hash2 := types.ComputeQualityCommitHash(roundID, vote, salt)
	require.Equal(t, hash1, hash2)

	// Different salt → different hash
	hash3 := types.ComputeQualityCommitHash(roundID, vote, []byte("other-salt"))
	require.NotEqual(t, hash1, hash3)

	// Different round → different hash
	hash4 := types.ComputeQualityCommitHash("r2", vote, salt)
	require.NotEqual(t, hash1, hash4)

	// Different vote → different hash
	vote2 := &types.QualityVote{OverallQuality: 500000, ReasoningDepth: 700000, Novelty: 600000, Toxicity: 10000, FactualAccuracy: 900000, ConsentValid: true}
	hash5 := types.ComputeQualityCommitHash(roundID, vote2, salt)
	require.NotEqual(t, hash1, hash5)

	// Verify
	require.True(t, types.VerifyQualityCommitHash(hash1, roundID, vote, salt))
	require.False(t, types.VerifyQualityCommitHash(hash1, roundID, vote2, salt))
}

func TestMsgAccessSample_ValidateBasic(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		msg := &types.MsgAccessSample{Consumer: testAddr, SampleId: "s1"}
		require.NoError(t, msg.ValidateBasic())
	})

	t.Run("empty sample ID", func(t *testing.T) {
		msg := &types.MsgAccessSample{Consumer: testAddr, SampleId: ""}
		require.Error(t, msg.ValidateBasic())
	})
}
