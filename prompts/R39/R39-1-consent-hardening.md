# R39-1 — Consent Hardening

## Objective

Strengthen consent verification beyond the basic validation in R37-1. Add consent revocation (authors can withdraw consent), consent audit trails, and on-chain consent attestation.

## Tasks

### 1. Consent Revocation

Authors must be able to withdraw consent after the fact. This is a legal requirement in many jurisdictions (GDPR right to erasure).

```go
// MsgRevokeConsent — new Msg type (add to tx.proto)
message MsgRevokeConsent {
  option (cosmos.msg.v1.signer) = "requester";
  string requester    = 1;  // Must be original_author or submitter
  string sample_id    = 2;
  string reason       = 3;
}

func (k Keeper) RevokeConsent(ctx context.Context, msg *types.MsgRevokeConsent) error {
    sample := k.GetSample(ctx, msg.SampleId)

    // 1. Verify requester is original_author or submitter
    if msg.Requester != sample.OriginalAuthor && msg.Requester != sample.Submitter {
        return ErrUnauthorized
    }

    // 2. Remove content (keep provenance metadata)
    sample.Content = "[consent revoked]"
    sample.Status = types.SAMPLE_STATUS_PRUNED

    // 3. Stop all revenue to this sample
    k.DeletePendingRevenue(ctx, sample.Id)

    // 4. Record revocation in audit trail
    k.RecordConsentEvent(ctx, ConsentEvent{
        SampleId:  sample.Id,
        EventType: "revocation",
        Actor:     msg.Requester,
        Reason:    msg.Reason,
        Block:     currentBlock,
    })

    // 5. Remove from active indexes (no longer discoverable)
    k.removeFromActiveIndexes(ctx, sample)
    k.SetSample(ctx, sample)

    return nil
}
```

### 2. Consent Audit Trail

Every consent-related event is logged on-chain:

```go
type ConsentEvent struct {
    SampleId    string
    EventType   string  // "granted", "revocation", "challenged", "verified"
    Actor       string
    Reason      string
    Block       uint64
}
```

Store: `consent_audit/{sample_id}/{seq}` → ConsentEvent

Queryable: `QueryConsentHistory(sample_id)` returns full audit trail.

### 3. On-Chain Consent Attestation

For `CONSENT_TYPE_OPT_IN`, support cryptographic consent:

```go
func (k Keeper) verifyOptInSignature(consent *types.ConsentProof, content string) error {
    if consent.AuthorSignature == "" {
        return nil  // Signature optional, proof_uri is fallback
    }
    // Verify Ed25519 or secp256k1 signature over content_hash
    // Author signs: SHA-256(content) to prove they consented to THIS content
    contentHash := sha256.Sum256([]byte(content))
    return verifySignature(consent.AuthorSignature, contentHash[:])
}
```

### 4. Consent Upgrade Path

Submitters can upgrade consent level after initial submission (e.g., get the original author to sign):

```go
message MsgUpgradeConsent {
  option (cosmos.msg.v1.signer) = "submitter";
  string submitter       = 1;
  string sample_id       = 2;
  ConsentProof new_consent = 3;
}
```

Upgrading consent increases the sample's revenue multiplier retroactively.

### 5. Tests

- Consent revocation by original author
- Consent revocation by submitter
- Unauthorized revocation rejected
- Revoked sample removed from search
- Revoked sample stops earning revenue
- Consent audit trail recorded
- Cryptographic consent verification (valid signature)
- Cryptographic consent verification (invalid signature)
- Consent upgrade from fair_use → opt_in
- Consent upgrade increases multiplier
- Cannot upgrade to self_authored if not the author

Target: ≥ 20 tests.
