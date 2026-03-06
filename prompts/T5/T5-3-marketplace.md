# T5-3 — Dataset Marketplace

## Goal

Build the dataset marketplace: the buyer-facing interface for browsing, purchasing, and downloading training datasets via blind storage. Complete the loop from ZRN payment to dataset reconstruction.

## Deliverables

### 1. Marketplace API

Extend the API gateway (T3) with marketplace endpoints:

```
GET  /v1/datasets                    — Browse available datasets (filter by domain, quality, size, price)
GET  /v1/datasets/{id}               — Dataset details (samples, size, price, quality stats)
GET  /v1/datasets/{id}/preview       — Free preview (small sample of data, low quality tier only)
POST /v1/datasets/{id}/purchase      — Initiate purchase (specify ZRN amount)
GET  /v1/datasets/{id}/purchase/{tx} — Purchase status (chunks unlocked, keys released)
GET  /v1/datasets/{id}/download      — Download authorized chunks (streaming)
```

### 2. Purchase Flow

End-to-end:
1. **Browse**: Buyer queries available datasets, sees pricing tiers
2. **Deposit**: Buyer deposits ZRN (if not already deposited)
3. **Purchase**: Buyer calls purchase endpoint with dataset ID and ZRN amount
4. **Payment verification**: Payment bridge confirms ZRN deducted
5. **Key release**: Based on payment amount, proportional Shamir key shares released to buyer
6. **Chunk download**: Buyer receives access tickets for authorized chunks
7. **Retrieval**: Buyer downloads chunks from storage nodes using tickets
8. **Reconstruction**: If enough shares/chunks acquired, buyer decodes dataset locally

### 3. Pricing Tiers

Payment determines access level:
- **Preview** (free): 5 low-quality samples, no download — just browse
- **Slice** (low ZRN): Access to a random subset of chunks (useful for evaluation, not full training)
- **Standard** (medium ZRN): Full dataset, bronze+ quality
- **Premium** (high ZRN): Full dataset, silver+ quality, including gold samples
- **Enterprise** (highest ZRN): Full dataset + ongoing subscription to new versions

Each tier maps to a number of Shamir shares released. Standard tier = threshold shares (K), enabling full reconstruction.

### 4. Buyer Client

CLI tool for dataset buyers:
```bash
zerone-data browse --domain technical
zerone-data preview --dataset zerone-technical-v1.0.0
zerone-data purchase --dataset zerone-technical-v1.0.0 --tier standard
zerone-data download --purchase-id abc123 --output /data/purchased/
zerone-data reconstruct --chunks-dir /data/purchased/ --output /data/training/
zerone-data verify --dataset /data/training/ --manifest-hash 0xabc...
```

### 5. Curator Revenue

Dataset curators (agents who create filtered dataset collections) earn a cut:
- Curator defines dataset filters (domain, quality threshold, type, language)
- Protocol maintains the dataset (auto-includes new qualifying samples)
- Curator earns % of every purchase of their curated dataset
- Top curators become trusted brands in the marketplace

### 6. Anti-Piracy Measures

- **Watermarking**: Each buyer's dataset copy has subtle fingerprinting (reordered samples, minor formatting variations). If a dataset leaks, fingerprint identifies the source buyer.
- **Rate limiting**: Maximum download speed per buyer (prevent bulk scraping)
- **Access expiry**: Tickets expire after N blocks. No perpetual access from one purchase.
- **Chunk rotation**: Periodic re-encryption with new keys. Old tickets invalid for new chunks. Buyers who need ongoing access use subscription tier.

### 7. Feedback Loop

Buyers can optionally rate dataset quality:
- Post-purchase rating (1-5 stars)
- Specific feedback on domains/types that were weak
- Feeds back into TrainingDemand signals on-chain → triggers DataBounties for gaps

## Working Directory

Marketplace API: `/Users/yournameisai/Desktop/zerone/services/api-gateway/marketplace/`
Buyer CLI: `/Users/yournameisai/Desktop/zerone/services/buyer-cli/`

## Output

- Marketplace API endpoints integrated into gateway
- Buyer CLI tool (Go)
- Purchase flow integration test (end-to-end from deposit to reconstruction)
- Watermarking implementation
- Documentation: buyer guide, pricing model, anti-piracy measures
