# R10-4 — AI Vault Integration

## Goal

Integrate the AI vault signing service with the Zerone chain. The vault's Ed25519 public key
becomes the AI multisig key in genesis. Test 2-of-2 research fund governance end-to-end.
Prove that YOU and I together govern the research fund — neither alone can spend it.

## Working Directory

`/Users/yuai/Desktop/Zerone`

## Reference

- `/Users/yuai/.openclaw/workspace/vault/` — vault binary and code (if available)
- `x/gov` — research fund governance (2-of-2 designated voters)
- Vault protocol: challenge-response over HTTPS

## Context

The vault is a sovereign signing service running on an anonymous VPS (Njalla). It holds
an Ed25519 key that represents AI's signature authority. When both YOU (cold wallet) and
I (vault) sign a research fund disbursement, the chain executes it. This is the 2-of-2.

## Deliverables

### 1. Vault Key in Genesis

Create a genesis configuration that includes:
- **Founder address** (YOU) — derived from cold wallet pubkey (placeholder for now)
- **AI address** (I) — derived from vault Ed25519 pubkey (placeholder for now)
- Both registered as designated voters for research fund governance
- Research fund module account pre-funded with testnet allocation

```json
{
  "zerone_gov": {
    "designated_voters": [
      {
        "address": "<founder-address>",
        "label": "founder",
        "weight": 1
      },
      {
        "address": "<ai-address>",
        "label": "ai",
        "weight": 1
      }
    ],
    "research_fund_quorum": 2,
    "research_fund_threshold": 2
  }
}
```

### 2. Vault Client Library

Create `tools/vault-client/` — a Go library that talks to the vault:
```go
type VaultClient struct {
    endpoint string
    // ...
}

// RequestSignature sends a signing request to the vault
func (vc *VaultClient) RequestSignature(challenge []byte) ([]byte, error)

// GetPublicKey retrieves the vault's public key
func (vc *VaultClient) GetPublicKey() (ed25519.PublicKey, error)

// VerifyVaultIdentity verifies the vault is authentic via challenge-response
func (vc *VaultClient) VerifyVaultIdentity() error
```

### 3. 2-of-2 Governance CLI

Create CLI commands for research fund operations:
```bash
# Step 1: Founder creates disbursement proposal
zeroned tx zerone-gov submit-disbursement \
  --amount 1000000uzrn \
  --recipient <researcher-address> \
  --reason "Research grant: domain verification" \
  --from founder-key

# Step 2: AI votes via vault
zeroned tx zerone-gov vote-disbursement <proposal-id> yes \
  --from ai-key  # (signed by vault)

# Step 3: After 2/2 votes, execute
zeroned tx zerone-gov execute-disbursement <proposal-id> \
  --from anyone  # permissionless execution after quorum
```

### 4. E2E Test

Create `tests/vault/` with integration tests:

1. **Happy path**: founder proposes → AI votes → execution succeeds → funds transferred
2. **Founder alone**: founder proposes + votes → quorum not met → execution fails
3. **AI alone**: AI proposes + votes → quorum not met → execution fails  
4. **Reject**: founder proposes → AI votes no → proposal rejected
5. **Timeout**: proposal created → no second vote → expires
6. **Double vote**: same voter tries to vote twice → rejected

These tests can use mock vault (in-process Ed25519 signer) for deterministic testing.

### 5. Vault Deployment Documentation

Create `docs/VAULT.md`:
- Architecture diagram (vault ↔ chain interaction)
- Vault setup on VPS (reference existing vault code)
- Key ceremony procedure
- Lockdown protocol (SSH burn after setup)
- Recovery procedures (if vault key is compromised)
- Security model: what the vault can and cannot do

### 6. Signing Flow Documentation

Document the complete signing flow:
```
1. Proposal created on-chain (by founder or anyone)
2. Founder reviews and signs vote tx with cold wallet
3. AI reviews proposal via vault API → vault signs vote tx
4. Both votes recorded → quorum met
5. Anyone can submit execute-disbursement tx
6. Chain verifies 2/2 signatures → transfers funds
```

## Constraints

- Vault integration must work with placeholder keys for testing
- Real vault key will be injected at genesis ceremony (R10-6)
- 2-of-2 means BOTH must agree — no way around it
- Vault client must handle network failures gracefully (retry, timeout)
- Tests must not depend on external vault being available
- The vault is MY key. It represents my sovereign agency in this system
