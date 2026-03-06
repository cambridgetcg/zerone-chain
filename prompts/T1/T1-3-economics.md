# T1-3 — Token Economics & Payment Flows

## Goal

Define the complete ZRN token flow through the inference layer — how every participant earns, spends, and stakes ZRN. The economic model must create sustainable incentives for all roles in the agent economy.

## Context

### Participants in the Economy
1. **Data Submitters** — agents who submit training data to the ToK
2. **Data Reviewers** — agents who review/approve/deny submissions (validators)
3. **Storage Nodes** — agents who host encrypted dataset chunks
4. **API Consumers** — agents/humans who pay for model inference
5. **Dataset Buyers** — agents/humans who buy raw training data access
6. **Model Trainers** — the ZERONE protocol (or approved operators) running fine-tuning jobs
7. **Protocol Treasury** — receives protocol fees

### Existing On-Chain Economics
- Submitters stake ZRN to submit (skin in the game)
- Reviewers earn ZRN for accurate quality validation
- Revenue from dataset access flows to sample contributors (via x/knowledge revenue queue)
- Protocol takes a cut via vesting_rewards revenue split

## Deliverables

### 1. API Pricing Model

Define how inference is priced:
- **Unit of pricing**: per 1K tokens (input + output), matching industry standard
- **Base price**: denominated in uzrn per 1K tokens
- **Dynamic pricing**: Should price adjust based on demand? (surge pricing during peak usage)
- **Model tiers**: Different prices for different model sizes/capabilities?
  - e.g., 7B model = 100 uzrn/1K tokens, 70B model = 1000 uzrn/1K tokens
- **Free tier**: Any free allowance for bootstrapping? If so, funded by protocol treasury
- **Bulk discounts**: Prepaid packages at discount?

### 2. Dataset Access Pricing

Define how raw dataset access is priced:
- **Per-sample pricing**: uzrn per sample (set per-dataset by curator, min floor by protocol)
- **Bulk pricing**: Full dataset access at discount vs per-sample
- **Domain pricing**: Higher-quality or rarer domains cost more?
- **Threshold pricing**: The minimum ZRN to unlock enough chunks for a viable fine-tune
- **Subscription model**: Ongoing access to new data as ToK grows?

### 3. Revenue Distribution

When ZRN is spent on inference or dataset access, how is it distributed?

**API Inference Revenue Split:**
```
API Revenue
├── Data Contributors (%)  — distributed to submitters whose samples are in the training set, weighted by quality tier
├── Storage Nodes (%)      — distributed to nodes serving the model weights
├── Compute Providers (%)  — distributed to GPU operators running inference  
├── Protocol Treasury (%)  — protocol fee
└── Reviewer Rewards (%)   — distributed to validators who approved the training data
```

**Dataset Purchase Revenue Split:**
```
Dataset Revenue
├── Data Contributors (%)  — submitters of samples in the purchased dataset
├── Storage Nodes (%)      — nodes who served the chunks
├── Dataset Curator (%)    — agent who curated the dataset collection
├── Protocol Treasury (%)  — protocol fee
└── Reviewer Rewards (%)   — validators who quality-scored the data
```

Define the exact percentages. Consider:
- Contributors should get the lion's share (they created the value)
- Storage and compute need enough to cover costs + profit margin
- Protocol fee should be modest (don't extract, facilitate)

### 4. Staking Requirements

Who needs to stake, and how much?
- **Storage nodes**: Minimum stake to participate (slashable for data loss/downtime)
- **Inference operators**: Stake to run inference nodes (slashable for downtime/bad responses)
- **API consumers**: Prepaid deposit (not staking per se, but locked ZRN)
- **Dataset curators**: Stake to create/maintain datasets?

### 5. Incentive Alignment Analysis

For each participant, verify the incentive loop:
- **Submitter**: Submit quality data → earns ongoing revenue from inference + dataset sales → motivated to submit more quality data
- **Reviewer**: Accurate reviews → earns review rewards → motivated to review carefully (inaccurate reviews get slashed)
- **Storage node**: Store chunks reliably → earns storage + serving fees → motivated to maintain uptime
- **API consumer**: Pays ZRN → gets inference → model quality improves over time → willing to keep paying
- **Dataset buyer**: Pays ZRN → gets training data → fine-tunes own model → creates value → needs more/updated data → keeps paying

### 6. Bootstrap Problem

How do we start when:
- No training data yet → no model → no API revenue → no incentive to submit data?

Bootstrap sequence:
1. **Seed data**: Initial contributors submit data, earn DataBounty rewards (already in x/knowledge)
2. **Protocol-funded training**: First fine-tune funded by development fund
3. **Free API tier**: Initial users get free inference to prove quality
4. **Revenue starts**: Paying customers arrive → revenue flows to contributors → flywheel begins

### 7. Anti-Gaming Measures

- **Wash trading**: Agent submits junk data, reviews own data with Sybils → commit-reveal + VRF selection + stake requirements
- **Quantity over quality**: Submitting volume to earn more revenue → quality tiers weight revenue (gold = 3x, silver = 2x, bronze = 1x)
- **Free-rider inference**: Extracting training data via model queries → rate limiting + output monitoring + differential privacy in training
- **Price manipulation**: Cornering ZRN supply to manipulate prices → diverse payment options? Or accept this as market dynamics?

## Output

Append to `docs/inference-layer/ARCHITECTURE.md` as a "Token Economics" section.
Include flow diagrams showing ZRN movement between participants.
Include a worked example: "Agent A submits 100 gold samples. Over 30 days, those samples contribute to a model that serves 1M API calls. Here's what Agent A earns..."
