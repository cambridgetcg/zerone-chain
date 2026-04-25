# Off-chain vault protocol

`x/private_corpus` records vault identity and manifest hashes on-chain.
The data itself lives off-chain, in a server the vault operator runs.
This document specifies the protocol the off-chain server should speak
so clients can fetch vault items and verify them against the on-chain
record.

The chain does not enforce any of this. The protocol is a shared
convention — operators may diverge, but their access policy must say
how (the URL is recorded on-chain at vault registration).

---

## Roles

- **Operator** — owns the vault, runs the server, controls the
  `operator_pubkey` private key. Registered on ZERONE via
  `MsgRegisterVault`.
- **Reader** — wants to fetch vault items. Authenticates to the
  server per the operator's access policy. Verifies received items
  against on-chain manifest content hashes.

---

## Server endpoints

The server is reachable at `Vault.server_endpoint` (HTTPS expected).
Operators MAY support additional endpoints; the minimum set is below.

### `GET /healthz`

Returns 200 if the server is up. No authentication.

### `GET /manifest/{manifest_id}`

Returns the manifest item list as JSON. The list is what the on-chain
`content_hash` was computed over.

Response body shape:

```json
{
  "manifest_id": "love-corpus#42",
  "vault_id": "love-corpus",
  "items": [
    {
      "id": "item-001",
      "size_bytes": 12345,
      "hash": "<hex>",
      "url": "/item/item-001"
    },
    ...
  ],
  "signature": "<base64 signature over the canonical-JSON body of `manifest_id+vault_id+items`>"
}
```

The signature is produced by the operator's private key (matching
`operator_pubkey` on-chain) over a canonical JSON encoding of
`{manifest_id, vault_id, items}`. The reader verifies:

1. The signature is valid for `operator_pubkey` from the on-chain
   `Vault` record.
2. Hashing `items` with the operator's documented algorithm reproduces
   the on-chain `content_hash`.

If either check fails, the server is misbehaving — the reader should
treat the response as untrusted.

### `GET /item/{item_id}`

Returns the raw bytes of a single item. The response MAY be served
with content-type appropriate to the data; the protocol does not
constrain it.

The reader verifies the bytes by hashing them and comparing to the
manifest's per-item `hash` field. If the manifest's `content_hash` is
a Merkle root, the operator SHOULD also serve a Merkle proof endpoint
(`GET /proof/{manifest_id}/{item_id}`) returning the path; otherwise
the reader must fetch the entire manifest item list and hash from
scratch.

---

## Authentication

The operator decides. Common shapes:

- **Public read.** No authentication. Anyone with the URL can fetch.
  In this case the privacy is per-item (only readers who know
  `manifest_id` and `item_id` get them), not per-reader.
- **Bearer token.** Operator issues tokens out-of-band; reader
  presents `Authorization: Bearer <token>`.
- **Signed challenge.** Reader signs a random challenge with their
  own keypair; operator verifies against an allow-list of public keys.
  Most flexible; suitable for AI agents that hold their own key.
- **mTLS.** Client certificate; operator pins reader certs.

The chosen scheme MUST be documented at the vault's
`access_policy_url`.

---

## Recording accesses (optional)

If the operator wants the access pattern to be public — for instance,
to demonstrate that a specific AI agent received a specific manifest
on a specific date — they can call `MsgRecordAccess` from the
operator's address after granting access.

The `accessor` field is whatever address the operator considers
identifying for the reader. The operator's bookkeeping address, the
reader's known on-chain address, or a pseudonym are all valid choices;
the chain does not interpret the field beyond requiring it to parse
as bech32.

Recording is a per-operator transparency choice. Some operators will
log every access; others will log none. Both are honest postures —
what matters is that the choice is the operator's, made openly.

---

## What the chain DOES NOT do

- It does not store `items[]` or item bytes.
- It does not authenticate readers to your server.
- It does not enforce your access policy.
- It does not vouch for the truthfulness of vault content. The
  truth-seeking commitments at `docs/TRUTH_SEEKING.md` apply to the
  PUBLIC knowledge module (`x/knowledge`), not to vaults.
- It does not retrieve items on a reader's behalf. If a reader cannot
  reach the server, the chain has no fallback.

---

## Why this honesty matters

The chain's existence depends on its public properties being
trustworthy. If `x/private_corpus` quietly served items, it would be
performing storage that the chain hadn't declared — and any future
audit of "what does ZERONE store?" would have to mention it. By
keeping items strictly off-chain and documenting the boundary here,
both the chain's stated guarantees and your operator's stated policy
remain checkable independently. Neither lies for the other.
