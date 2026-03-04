# R36-4 — Code Generation & Types Package Update

## Objective

After R36-1/2/3 rewrote the protos, regenerate Go code and update the `x/knowledge/types/` package: keys, codec, params validation, message validation, proto registration.

## Tasks

### 1. Regenerate Protobuf

```bash
make proto-gen
```

Fix any generation errors. Ensure all `.pb.go`, `.pb.gw.go` files are clean.

### 2. Update keys.go

Replace all store key prefixes for the new types:

```go
var (
    SampleKey           = []byte{0x01}
    SubmissionKey       = []byte{0x02}
    QualityRoundKey     = []byte{0x03}
    DomainKey           = []byte{0x04}
    DatasetKey          = []byte{0x05}
    TrainingDemandKey   = []byte{0x06}
    DataBountyKey       = []byte{0x07}
    ScrapedSourceKey    = []byte{0x08}
    ValidatorInfoKey    = []byte{0x09}
    ThreadIndexKey      = []byte{0x0A}  // thread_id → []sample_id
    DomainSampleIndex   = []byte{0x0B}  // domain → []sample_id
    SubmitterIndex      = []byte{0x0C}  // submitter → []sample_id
    NicheIndex          = []byte{0x0D}  // niche_key → []sample_id
    ContentHashIndex    = []byte{0x0E}  // content_hash → submission_id (dedup)
    // Sequences
    SampleSeqKey        = []byte{0x80}
    SubmissionSeqKey    = []byte{0x81}
    RoundSeqKey         = []byte{0x82}
    DatasetSeqKey       = []byte{0x83}
)
```

Remove old keys (FactKey, ClaimKey, etc.)

### 3. Update codec.go / proto_register.go

Register new message types with both gogoproto and the interface registry:

```go
func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
    registry.RegisterImplementations((*sdk.Msg)(nil),
        &MsgSubmitData{},
        &MsgSubmitThread{},
        &MsgSubmitCommitment{},
        &MsgSubmitReveal{},
        &MsgContestSample{},
        &MsgSponsorSample{},
        &MsgCreateDataset{},
        &MsgAccessDataset{},
        &MsgAccessSample{},
        &MsgFundBounty{},
        &MsgRateSample{},
        // ... kept messages
    )
}
```

### 4. Update params.go

- `DefaultParams()` with values from R36-3
- `Validate()` checking all thresholds are in valid ranges
- Ensure `gold_threshold > silver_threshold > bronze_threshold`
- All BPS values ≤ 1,000,000
- Revenue shares sum correctly

### 5. Update message validation

For each new Msg type, implement `ValidateBasic()`:

- `MsgSubmitData`: content non-empty, domain valid, consent proof present, language ISO 639-1
- `MsgSubmitThread`: at least 2 items, all share thread_id, total content within limits
- `MsgSubmitReveal`: quality scores in valid BPS range
- `MsgContestSample`: valid contest_type, reason non-empty
- `MsgCreateDataset`: name non-empty, min_quality in valid range
- `MsgAccessDataset` / `MsgAccessSample`: valid IDs

### 6. New helper types

```go
// types.go — replace old types

type QualityTier string
const (
    TierGold   QualityTier = "gold"
    TierSilver QualityTier = "silver"
    TierBronze QualityTier = "bronze"
)

func QualityTierFromScore(score uint64, params Params) QualityTier {
    switch {
    case score >= params.GoldThreshold:
        return TierGold
    case score >= params.SilverThreshold:
        return TierSilver
    default:
        return TierBronze
    }
}
```

### 7. Remove old types

Delete or replace:
- `canonical.go` / `canonical_test.go` (canonical form logic — not needed)
- `axiom_embed.go` → `seed_embed.go` (from R36-3)
- Old `types.go` content (ProjectPhase etc. is in tree module, not here)

### 8. Update errors

```go
var (
    ErrInvalidSubmission    = errorsmod.Register(ModuleName, 2, "invalid submission")
    ErrDuplicateContent     = errorsmod.Register(ModuleName, 3, "duplicate content")
    ErrInvalidConsent       = errorsmod.Register(ModuleName, 4, "invalid consent proof")
    ErrSampleNotFound       = errorsmod.Register(ModuleName, 5, "sample not found")
    ErrSubmissionNotFound   = errorsmod.Register(ModuleName, 6, "submission not found")
    ErrDatasetNotFound      = errorsmod.Register(ModuleName, 7, "dataset not found")
    ErrInvalidQualityScore  = errorsmod.Register(ModuleName, 8, "invalid quality score")
    ErrRoundNotFound        = errorsmod.Register(ModuleName, 9, "quality round not found")
    ErrConsentRequired      = errorsmod.Register(ModuleName, 10, "consent proof required")
    ErrContentTooLarge      = errorsmod.Register(ModuleName, 11, "content exceeds max bytes")
    ErrThreadTooLarge       = errorsmod.Register(ModuleName, 12, "thread exceeds max size")
    ErrInsufficientPayment  = errorsmod.Register(ModuleName, 13, "insufficient payment")
    // ... keep existing error codes for shared concepts (domain, params, slashing)
)
```

## Verification

```bash
go build ./x/knowledge/types/...   # Must pass
go vet ./x/knowledge/types/...     # Must pass
go test ./x/knowledge/types/...    # Update tests — must pass
```
