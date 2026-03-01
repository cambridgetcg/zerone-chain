# R27-2 — Complete Evidence Management CLI + Minor CLI Fixes

## Context

The evidence_mgmt module has **4 message types** (SubmitEvidence, TransferCustody, VerifyEvidence, ChallengeEvidence) but only **1 tx CLI command** (submit). Three operations are inaccessible from the command line. The R25 assessment flagged "8 evidence CLI commands missing" and inconsistent hash formats between knowledge and disputes modules.

Additionally, the knowledge module has **5 satisfaction query RPCs without CLI** (from R25 assessment: "5 query RPCs without CLI").

## Task

### 1. Evidence Management Tx Commands

**Existing:**
- `submit [evidence-type] [content-hash] [metadata]`

**Missing (3):**
- `transfer-custody [evidence-id] [new-custodian]` — MsgTransferCustody
- `verify [evidence-id] [verification-hash]` — MsgVerifyEvidence  
- `challenge [evidence-id] [reason]` — MsgChallengeEvidence

### 2. Evidence Management Query Commands

**Existing:**
- `evidence [id]`
- `params`

**Check for missing queries:**
```bash
grep "rpc Query" proto/zerone/evidence_mgmt/v1/query.proto
```

Likely missing: list-by-custodian, list-by-type, chain-of-custody history, evidence-by-hash.

### 3. Fix Evidence Hash Format Inconsistency

R25 assessment noted: "hash format inconsistent between knowledge and disputes modules."

Investigate:
```bash
grep -rn "ContentHash\|content_hash\|EvidenceHash\|evidence_hash" --include="*.go" x/knowledge/ x/disputes/ x/evidence_mgmt/ | head -20
```

If knowledge uses hex and disputes uses base64 (or similar), standardize on one format. Recommend hex (consistent with the vault signing flow and standard Cosmos patterns).

### 4. Knowledge Satisfaction Query CLI

From R25: "Satisfaction: 4/10 — 5 query RPCs without CLI"

```bash
grep "rpc Query" proto/zerone/knowledge/v1/query.proto | grep -i "satisf\|demand\|relevance"
```

Add CLI commands for any satisfaction/demand/relevance queries that lack them.

### 5. Disputes CLI Completeness Check

Disputes has 6 tx commands + 5 queries. Verify against proto:
```bash
grep "rpc " proto/zerone/disputes/v1/tx.proto | grep -v Response
grep "rpc Query" proto/zerone/disputes/v1/query.proto
```

The disputes CLI looked complete in R25, but verify no query commands are missing.

### 6. Test

For each new command:
- `--help` works and shows usage
- Valid tx broadcasts on localnet
- Evidence chain: submit → transfer-custody → verify (3-step lifecycle test)

## Files to Modify

- `x/evidence_mgmt/client/cli/tx.go` — Add 3 tx commands
- `x/evidence_mgmt/client/cli/query.go` — Add missing query commands
- `x/knowledge/client/cli/query.go` — Add satisfaction query commands
- Possibly `x/disputes/client/cli/query.go` — If missing queries found
- Possibly `x/knowledge/types/` or `x/disputes/types/` — Hash format standardization

## Success Criteria

- [ ] All 4 evidence_mgmt message types have CLI commands
- [ ] Evidence queries complete
- [ ] Knowledge satisfaction queries accessible via CLI
- [ ] Hash format consistent across knowledge, disputes, and evidence_mgmt
- [ ] Evidence lifecycle testable from CLI (submit → transfer → verify)
- [ ] All existing tests pass
