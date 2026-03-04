# R36-2 — Proto TX & Query: Training Data API Surface

## Objective

Rewrite `proto/zerone/knowledge/v1/tx.proto` and `query.proto` to reflect the training data protocol. The Msg service pivots from truth verification to quality validation and data access.

## Context

- **Design doc:** `docs/DESIGN-training-data-protocol.md`
- **New types:** From R36-1 (`types.proto` already rewritten)
- **Key principle:** The commit-reveal mechanism is identical — only the business semantics change

## Tasks

### 1. Rewrite tx.proto — Msg Service

**Remove all existing RPCs.** Replace with:

```protobuf
service Msg {
  option (cosmos.msg.v1.service) = true;

  // ─── Submission lifecycle ─────────────────────────────────────────────

  // SubmitData submits discourse for quality validation.
  rpc SubmitData(MsgSubmitData) returns (MsgSubmitDataResponse);

  // SubmitThread submits a complete conversation thread (batch).
  rpc SubmitThread(MsgSubmitThread) returns (MsgSubmitThreadResponse);

  // ─── Quality validation (commit-reveal, same as before) ───────────────

  // SubmitCommitment submits a blinded quality score during commit phase.
  rpc SubmitCommitment(MsgSubmitCommitment) returns (MsgSubmitCommitmentResponse);

  // SubmitReveal reveals a validator's quality scores during reveal phase.
  rpc SubmitReveal(MsgSubmitReveal) returns (MsgSubmitRevealResponse);

  // ─── Data integrity ──────────────────────────────────────────────────

  // ContestSample disputes a validated sample (consent, quality, duplication).
  rpc ContestSample(MsgContestSample) returns (MsgContestSampleResponse);

  // ─── Data preservation ───────────────────────────────────────────────

  // SponsorSample funds a sample to prevent it from being pruned.
  rpc SponsorSample(MsgSponsorSample) returns (MsgSponsorSampleResponse);

  // ─── Domain management (kept, adapted) ────────────────────────────────

  rpc ProposeDomain(MsgProposeDomain) returns (MsgProposeDomainResponse);
  rpc EndorseDomainProposal(MsgEndorseDomainProposal) returns (MsgEndorseDomainProposalResponse);

  // ─── Dataset curation ─────────────────────────────────────────────────

  // CreateDataset creates a curated dataset definition.
  rpc CreateDataset(MsgCreateDataset) returns (MsgCreateDatasetResponse);

  // AccessDataset records a consumer accessing a dataset (triggers payment).
  rpc AccessDataset(MsgAccessDataset) returns (MsgAccessDatasetResponse);

  // AccessSample records individual sample access (triggers micro-payment).
  rpc AccessSample(MsgAccessSample) returns (MsgAccessSampleResponse);

  // ─── Demand & bounties ────────────────────────────────────────────────

  // ReportDemand reports training data demand from AI labs.
  rpc ReportDemand(MsgReportDemand) returns (MsgReportDemandResponse);

  // FundBounty creates or adds to a data collection bounty.
  rpc FundBounty(MsgFundBounty) returns (MsgFundBountyResponse);

  // ─── Feedback ─────────────────────────────────────────────────────────

  // RateSample provides downstream quality feedback on a sample.
  rpc RateSample(MsgRateSample) returns (MsgRateSampleResponse);

  // ─── Governance ───────────────────────────────────────────────────────

  rpc UpdateParams(MsgUpdateParams) returns (MsgUpdateParamsResponse);

  // ─── Scraped source registry ──────────────────────────────────────────

  rpc AddScrapedSource(MsgAddScrapedSource) returns (MsgAddScrapedSourceResponse);
  rpc RemoveScrapedSource(MsgRemoveScrapedSource) returns (MsgRemoveScrapedSourceResponse);

  // ─── Research fund (kept) ─────────────────────────────────────────────

  rpc ProposeResearchFund(MsgProposeResearchFund) returns (MsgProposeResearchFundResponse);
  rpc VoteResearchProposal(MsgVoteResearchProposal) returns (MsgVoteResearchProposalResponse);
  rpc ExecuteResearchProposal(MsgExecuteResearchProposal) returns (MsgExecuteResearchProposalResponse);

  // ─── Authority ────────────────────────────────────────────────────────

  // AddSample governance-creates a sample (authority-gated, for seed data).
  rpc AddSample(MsgAddSample) returns (MsgAddSampleResponse);
}
```

### 2. Message Definitions

For each RPC, define request/response messages. Key new ones:

```protobuf
message MsgSubmitData {
  option (cosmos.msg.v1.signer) = "submitter";
  string submitter           = 1;
  string content             = 2;
  SampleType sample_type     = 3;
  string domain              = 4;
  string source_uri          = 5;
  string source_platform     = 6;
  uint64 source_timestamp    = 7;
  ConsentProof consent       = 8;
  string original_author     = 9;
  string license             = 10;
  repeated string tags       = 11;
  string language            = 12;
  string stake               = 13;
  string parent_submission_id = 14;  // If part of thread
  string thread_id           = 15;
  repeated string context_ids = 16;
  bool sponsored             = 17;
}
message MsgSubmitDataResponse { string submission_id = 1; }

message MsgSubmitThread {
  option (cosmos.msg.v1.signer) = "submitter";
  string submitter            = 1;
  repeated MsgSubmitData items = 2;  // Ordered conversation
  string thread_id            = 3;   // Shared thread ID
  string domain               = 4;
  string stake                = 5;   // Covers entire thread
}
message MsgSubmitThreadResponse {
  repeated string submission_ids = 1;
  string thread_id               = 2;
}

// SubmitCommitment — kept from old knowledge, same structure
message MsgSubmitCommitment {
  option (cosmos.msg.v1.signer) = "verifier";
  string verifier    = 1;
  string round_id    = 2;
  bytes  commit_hash = 3;  // SHA-256(quality_vote_json || salt)
}
message MsgSubmitCommitmentResponse {}

// SubmitReveal — adapted: vote is now a serialized QualityVote
message MsgSubmitReveal {
  option (cosmos.msg.v1.signer) = "verifier";
  string verifier        = 1;
  string round_id        = 2;
  QualityVote scores     = 3;  // Multi-dimensional quality scores
  bytes salt             = 4;
}
message MsgSubmitRevealResponse {}

message MsgContestSample {
  option (cosmos.msg.v1.signer) = "challenger";
  string challenger      = 1;
  string sample_id       = 2;
  string stake           = 3;
  string reason          = 4;
  ContestType contest_type = 5;  // CONSENT, QUALITY, DUPLICATE, TOXIC
}
message MsgContestSampleResponse { string round_id = 1; }

message MsgSponsorSample {
  option (cosmos.msg.v1.signer) = "sponsor";
  string sponsor          = 1;
  string sample_id        = 2;
  string amount           = 3;
  uint64 duration_blocks  = 4;
}
message MsgSponsorSampleResponse {}

message MsgCreateDataset {
  option (cosmos.msg.v1.signer) = "curator";
  string curator           = 1;
  string name              = 2;
  string description       = 3;
  string domain            = 4;
  string license           = 5;
  SampleType filter_type   = 6;
  string filter_language   = 7;
  repeated string filter_tags = 8;
  uint64 min_quality       = 9;
  string price_per_sample  = 10;
  string bulk_price        = 11;
}
message MsgCreateDatasetResponse { string dataset_id = 1; }

message MsgAccessDataset {
  option (cosmos.msg.v1.signer) = "consumer";
  string consumer    = 1;
  string dataset_id  = 2;
  string max_payment = 3;
}
message MsgAccessDatasetResponse {
  string payment    = 1;
  uint64 sample_count = 2;
}

message MsgAccessSample {
  option (cosmos.msg.v1.signer) = "consumer";
  string consumer   = 1;
  string sample_id  = 2;
  string max_payment = 3;
}
message MsgAccessSampleResponse {
  string payment = 1;
}

message MsgFundBounty {
  option (cosmos.msg.v1.signer) = "funder";
  string funder          = 1;
  string domain          = 2;
  string topic           = 3;
  SampleType preferred_type = 4;
  string language        = 5;
  string amount          = 6;
  uint64 expires_blocks  = 7;
}
message MsgFundBountyResponse { string bounty_id = 1; }

message MsgRateSample {
  option (cosmos.msg.v1.signer) = "rater";
  string rater    = 1;
  string sample_id = 2;
  bool useful     = 3;
  string memo     = 4;
}
message MsgRateSampleResponse {}
```

Also add `ContestType` enum:

```protobuf
enum ContestType {
  CONTEST_TYPE_UNSPECIFIED  = 0;
  CONTEST_TYPE_CONSENT      = 1;  // Consent proof is invalid
  CONTEST_TYPE_QUALITY      = 2;  // Quality was mis-scored
  CONTEST_TYPE_DUPLICATE    = 3;  // Already exists in dataset
  CONTEST_TYPE_TOXIC        = 4;  // Harmful content not labeled
  CONTEST_TYPE_COPYRIGHT    = 5;  // Copyright violation
}
```

### 3. Rewrite query.proto

Replace all fact/claim queries with sample/submission queries:

```protobuf
service Query {
  // Samples
  rpc Sample(QuerySampleRequest) returns (QuerySampleResponse);
  rpc Samples(QuerySamplesRequest) returns (QuerySamplesResponse);
  rpc SamplesByDomain(QuerySamplesByDomainRequest) returns (QuerySamplesResponse);
  rpc SamplesByThread(QuerySamplesByThreadRequest) returns (QuerySamplesResponse);
  rpc SamplesBySubmitter(QuerySamplesBySubmitterRequest) returns (QuerySamplesResponse);

  // Submissions
  rpc Submission(QuerySubmissionRequest) returns (QuerySubmissionResponse);
  rpc PendingSubmissions(QueryPendingSubmissionsRequest) returns (QuerySubmissionsResponse);

  // Quality rounds
  rpc QualityRound(QueryQualityRoundRequest) returns (QueryQualityRoundResponse);

  // Datasets
  rpc Dataset(QueryDatasetRequest) returns (QueryDatasetResponse);
  rpc Datasets(QueryDatasetsRequest) returns (QueryDatasetsResponse);

  // Demand & bounties
  rpc TrainingDemand(QueryTrainingDemandRequest) returns (QueryTrainingDemandResponse);
  rpc DataBounties(QueryDataBountiesRequest) returns (QueryDataBountiesResponse);

  // Domains (kept)
  rpc Domain(QueryDomainRequest) returns (QueryDomainResponse);
  rpc Domains(QueryDomainsRequest) returns (QueryDomainsResponse);

  // Stats
  rpc DomainStats(QueryDomainStatsRequest) returns (QueryDomainStatsResponse);
  rpc ProtocolStats(QueryProtocolStatsRequest) returns (QueryProtocolStatsResponse);

  // Params
  rpc Params(QueryParamsRequest) returns (QueryParamsResponse);
}
```

Define request/response messages with standard pagination for list queries. Include REST annotations (grpc-gateway) matching existing patterns.

### 4. Keep Research Fund Messages

Copy `MsgProposeResearchFund`, `MsgVoteResearchProposal`, `MsgExecuteResearchProposal` and their responses from the old tx.proto — these are unchanged.

### 5. Keep Domain Messages

Copy `MsgProposeDomain`, `MsgEndorseDomainProposal` — these work the same way.

## Constraints

- Import `zerone/knowledge/v1/types.proto` and `genesis.proto` (R36-3)
- All REST paths under `/zerone/knowledge/v1/`
- Pagination uses `cosmos.base.query.v1beta1.PageRequest`
- All coin amounts as string (uzrn)

## Verification

```bash
make proto-gen  # Must succeed
grep -r "MsgSubmitData" x/knowledge/types/tx.pb.go  # New type exists
```
