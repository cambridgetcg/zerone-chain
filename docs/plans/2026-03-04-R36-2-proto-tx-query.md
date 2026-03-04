# R36-2 Proto TX & Query: Training Data API Surface

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rewrite `tx.proto` and `query.proto` to reflect the training data protocol, pivoting from truth verification to quality validation and data access.

**Architecture:** The Msg service adds SubmitThread, CreateDataset, AccessDataset, AccessSample, FundBounty RPCs. Removes ChallengeDomainProposal, RegisterStratum, UpdateExtendedParams. ContestSample gains a ContestType enum. The Query service is simplified to core sample/submission/dataset/demand queries, removing ecology-specific queries (niche, diversity, metabolism, etc.).

**Tech Stack:** Protobuf3, Cosmos SDK v0.50 msg/query conventions, buf generate

---

### Task 1: Add ContestType enum to types.proto

**Files:**
- Modify: `proto/zerone/knowledge/v1/types.proto`

**Step 1: Add ContestType enum after the existing enums**

Add after the `DomainStatus` enum (line ~89):

```protobuf
// ContestType classifies the reason for contesting a sample.
enum ContestType {
  CONTEST_TYPE_UNSPECIFIED  = 0;
  CONTEST_TYPE_CONSENT      = 1;  // Consent proof is invalid
  CONTEST_TYPE_QUALITY      = 2;  // Quality was mis-scored
  CONTEST_TYPE_DUPLICATE    = 3;  // Already exists in dataset
  CONTEST_TYPE_TOXIC        = 4;  // Harmful content not labeled
  CONTEST_TYPE_COPYRIGHT    = 5;  // Copyright violation
}
```

**Step 2: Commit**

```bash
git add proto/zerone/knowledge/v1/types.proto
git commit -m "feat(knowledge): add ContestType enum to types.proto (R36-2)"
```

---

### Task 2: Rewrite tx.proto — Service definition and messages

**Files:**
- Rewrite: `proto/zerone/knowledge/v1/tx.proto`

**Step 1: Write the full tx.proto**

Replace the entire file with the new service definition. Key changes from current:
- **Add RPCs:** SubmitThread, CreateDataset, AccessDataset, AccessSample, FundBounty
- **Remove RPCs:** ChallengeDomainProposal, RegisterStratum, UpdateExtendedParams
- **Modify:** MsgSubmitReveal now has `QualityVote scores` (typed field) instead of `string vote`
- **Modify:** MsgContestSample adds `ContestType contest_type`, removes `repeated string evidence_ids`
- **Keep unchanged:** SubmitData, SubmitCommitment, SponsorSample, ProposeDomain, EndorseDomainProposal, UpdateParams, ProposeResearchFund, VoteResearchProposal, ExecuteResearchProposal, AddScrapedSource, RemoveScrapedSource, ReportDemand, RateSample, AddSample

Full file content:

```protobuf
syntax = "proto3";
package zerone.knowledge.v1;

option go_package = "github.com/zerone-chain/zerone/x/knowledge/types";

import "cosmos/msg/v1/msg.proto";
import "zerone/knowledge/v1/types.proto";
import "zerone/knowledge/v1/genesis.proto";

// Msg defines the knowledge module's transaction service.
service Msg {
  option (cosmos.msg.v1.service) = true;

  // ─── Submission lifecycle ──────────────────────────────────────────────────

  // SubmitData submits discourse for quality validation.
  rpc SubmitData(MsgSubmitData) returns (MsgSubmitDataResponse);

  // SubmitThread submits a complete conversation thread (batch).
  rpc SubmitThread(MsgSubmitThread) returns (MsgSubmitThreadResponse);

  // ─── Quality validation (commit-reveal) ────────────────────────────────────

  // SubmitCommitment submits a blinded quality score during commit phase.
  rpc SubmitCommitment(MsgSubmitCommitment) returns (MsgSubmitCommitmentResponse);

  // SubmitReveal reveals a validator's quality scores during reveal phase.
  rpc SubmitReveal(MsgSubmitReveal) returns (MsgSubmitRevealResponse);

  // ─── Data integrity ────────────────────────────────────────────────────────

  // ContestSample disputes a validated sample (consent, quality, duplication).
  rpc ContestSample(MsgContestSample) returns (MsgContestSampleResponse);

  // ─── Data preservation ─────────────────────────────────────────────────────

  // SponsorSample funds a sample to prevent it from being pruned.
  rpc SponsorSample(MsgSponsorSample) returns (MsgSponsorSampleResponse);

  // ─── Domain management ─────────────────────────────────────────────────────

  rpc ProposeDomain(MsgProposeDomain) returns (MsgProposeDomainResponse);
  rpc EndorseDomainProposal(MsgEndorseDomainProposal) returns (MsgEndorseDomainProposalResponse);

  // ─── Dataset curation ──────────────────────────────────────────────────────

  // CreateDataset creates a curated dataset definition.
  rpc CreateDataset(MsgCreateDataset) returns (MsgCreateDatasetResponse);

  // AccessDataset records a consumer accessing a dataset (triggers payment).
  rpc AccessDataset(MsgAccessDataset) returns (MsgAccessDatasetResponse);

  // AccessSample records individual sample access (triggers micro-payment).
  rpc AccessSample(MsgAccessSample) returns (MsgAccessSampleResponse);

  // ─── Demand & bounties ─────────────────────────────────────────────────────

  // ReportDemand reports training data demand from AI labs.
  rpc ReportDemand(MsgReportDemand) returns (MsgReportDemandResponse);

  // FundBounty creates or adds to a data collection bounty.
  rpc FundBounty(MsgFundBounty) returns (MsgFundBountyResponse);

  // ─── Feedback ──────────────────────────────────────────────────────────────

  // RateSample provides downstream quality feedback on a sample.
  rpc RateSample(MsgRateSample) returns (MsgRateSampleResponse);

  // ─── Governance ────────────────────────────────────────────────────────────

  rpc UpdateParams(MsgUpdateParams) returns (MsgUpdateParamsResponse);

  // ─── Scraped source registry ───────────────────────────────────────────────

  rpc AddScrapedSource(MsgAddScrapedSource) returns (MsgAddScrapedSourceResponse);
  rpc RemoveScrapedSource(MsgRemoveScrapedSource) returns (MsgRemoveScrapedSourceResponse);

  // ─── Research fund ─────────────────────────────────────────────────────────

  rpc ProposeResearchFund(MsgProposeResearchFund) returns (MsgProposeResearchFundResponse);
  rpc VoteResearchProposal(MsgVoteResearchProposal) returns (MsgVoteResearchProposalResponse);
  rpc ExecuteResearchProposal(MsgExecuteResearchProposal) returns (MsgExecuteResearchProposalResponse);

  // ─── Authority ─────────────────────────────────────────────────────────────

  // AddSample governance-creates a sample (authority-gated, for seed data).
  rpc AddSample(MsgAddSample) returns (MsgAddSampleResponse);
}

// ─── Submission lifecycle messages ────────────────────────────────────────────

message MsgSubmitData {
  option (cosmos.msg.v1.signer) = "submitter";

  string          submitter           = 1;
  string          content             = 2;
  SampleType      sample_type         = 3;
  string          domain              = 4;
  string          source_uri          = 5;
  string          source_platform     = 6;
  uint64          source_timestamp    = 7;
  ConsentProof    consent             = 8;
  string          original_author     = 9;
  string          license             = 10;
  repeated string tags                = 11;
  string          language            = 12;
  string          stake               = 13; // uzrn amount
  string          parent_submission_id = 14; // If part of thread
  string          thread_id           = 15;
  repeated string context_ids         = 16;
  bool            sponsored           = 17; // Request bootstrap fund sponsorship
}

message MsgSubmitDataResponse {
  string submission_id = 1;
}

message MsgSubmitThread {
  option (cosmos.msg.v1.signer) = "submitter";

  string                submitter = 1;
  repeated MsgSubmitData items    = 2; // Ordered conversation
  string                thread_id = 3; // Shared thread ID
  string                domain    = 4;
  string                stake     = 5; // Covers entire thread
}

message MsgSubmitThreadResponse {
  repeated string submission_ids = 1;
  string          thread_id      = 2;
}

// ─── Quality validation messages ──────────────────────────────────────────────

message MsgSubmitCommitment {
  option (cosmos.msg.v1.signer) = "verifier";

  string verifier    = 1;
  string round_id    = 2;
  bytes  commit_hash = 3; // SHA-256(quality_vote_json || salt)
}

message MsgSubmitCommitmentResponse {}

message MsgSubmitReveal {
  option (cosmos.msg.v1.signer) = "verifier";

  string      verifier = 1;
  string      round_id = 2;
  QualityVote scores   = 3; // Multi-dimensional quality scores
  bytes       salt     = 4;
}

message MsgSubmitRevealResponse {}

// ─── Data integrity messages ──────────────────────────────────────────────────

message MsgContestSample {
  option (cosmos.msg.v1.signer) = "challenger";

  string      challenger   = 1;
  string      sample_id    = 2;
  string      stake        = 3; // uzrn amount
  string      reason       = 4;
  ContestType contest_type = 5;
}

message MsgContestSampleResponse {
  string round_id = 1;
}

// ─── Data preservation messages ───────────────────────────────────────────────

message MsgSponsorSample {
  option (cosmos.msg.v1.signer) = "sponsor";

  string sponsor          = 1;
  string sample_id        = 2;
  string amount           = 3; // uzrn amount
  uint64 duration_blocks  = 4;
}

message MsgSponsorSampleResponse {}

// ─── Domain management messages ───────────────────────────────────────────────

message MsgProposeDomain {
  option (cosmos.msg.v1.signer) = "proposer";

  string proposer     = 1;
  string name         = 2;
  string description  = 3;
  string stratum      = 4;
  string stake        = 5; // uzrn amount
}

message MsgProposeDomainResponse {
  string proposal_id = 1;
}

message MsgEndorseDomainProposal {
  option (cosmos.msg.v1.signer) = "endorser";

  string endorser    = 1;
  string proposal_id = 2;
}

message MsgEndorseDomainProposalResponse {}

// ─── Dataset curation messages ────────────────────────────────────────────────

message MsgCreateDataset {
  option (cosmos.msg.v1.signer) = "curator";

  string          curator          = 1;
  string          name             = 2;
  string          description      = 3;
  string          domain           = 4;
  string          license          = 5;
  SampleType      filter_type      = 6;
  string          filter_language  = 7;
  repeated string filter_tags      = 8;
  uint64          min_quality      = 9;
  string          price_per_sample = 10;
  string          bulk_price       = 11;
}

message MsgCreateDatasetResponse {
  string dataset_id = 1;
}

message MsgAccessDataset {
  option (cosmos.msg.v1.signer) = "consumer";

  string consumer    = 1;
  string dataset_id  = 2;
  string max_payment = 3;
}

message MsgAccessDatasetResponse {
  string payment      = 1;
  uint64 sample_count = 2;
}

message MsgAccessSample {
  option (cosmos.msg.v1.signer) = "consumer";

  string consumer    = 1;
  string sample_id   = 2;
  string max_payment = 3;
}

message MsgAccessSampleResponse {
  string payment = 1;
}

// ─── Demand & bounty messages ─────────────────────────────────────────────────

message MsgReportDemand {
  option (cosmos.msg.v1.signer) = "reporter";

  string                reporter = 1;
  repeated DemandReport reports  = 2;
}

message DemandReport {
  string domain      = 1;
  string subject     = 2;
  uint64 queries     = 3;
  uint64 fulfilled   = 4;
  uint64 unfulfilled = 5;
}

message MsgReportDemandResponse {}

message MsgFundBounty {
  option (cosmos.msg.v1.signer) = "funder";

  string     funder         = 1;
  string     domain         = 2;
  string     topic          = 3;
  SampleType preferred_type = 4;
  string     language       = 5;
  string     amount         = 6;
  uint64     expires_blocks = 7;
}

message MsgFundBountyResponse {
  string bounty_id = 1;
}

// ─── Feedback messages ────────────────────────────────────────────────────────

message MsgRateSample {
  option (cosmos.msg.v1.signer) = "rater";

  string rater     = 1;
  string sample_id = 2;
  bool   useful    = 3;
  string memo      = 4;
}

message MsgRateSampleResponse {}

// ─── Governance messages ──────────────────────────────────────────────────────

message MsgUpdateParams {
  option (cosmos.msg.v1.signer) = "authority";

  string authority = 1;
  Params params    = 2;
}

message MsgUpdateParamsResponse {}

// ─── Scraped source registry messages ─────────────────────────────────────────

message MsgAddScrapedSource {
  option (cosmos.msg.v1.signer) = "authority";

  string authority       = 1;
  string platform        = 2;
  string domain          = 3;
  string description     = 4;
  uint64 novelty_penalty = 5;
}

message MsgAddScrapedSourceResponse {
  string id = 1;
}

message MsgRemoveScrapedSource {
  option (cosmos.msg.v1.signer) = "authority";

  string authority = 1;
  string id        = 2;
}

message MsgRemoveScrapedSourceResponse {}

// ─── Research fund governance messages ────────────────────────────────────────

message MsgProposeResearchFund {
  option (cosmos.msg.v1.signer) = "proposer";

  string proposer             = 1;
  string title                = 2;
  string description          = 3;
  string amount               = 4; // uzrn amount
  string recipient            = 5;
  uint64 voting_period_blocks = 6;
}

message MsgProposeResearchFundResponse {
  string proposal_id = 1;
}

message MsgVoteResearchProposal {
  option (cosmos.msg.v1.signer) = "voter";

  string voter       = 1;
  string proposal_id = 2;
  bool   vote        = 3;
}

message MsgVoteResearchProposalResponse {}

message MsgExecuteResearchProposal {
  option (cosmos.msg.v1.signer) = "authority";

  string authority   = 1;
  string proposal_id = 2;
}

message MsgExecuteResearchProposalResponse {}

// ─── Authority messages ───────────────────────────────────────────────────────

message MsgAddSample {
  option (cosmos.msg.v1.signer) = "authority";

  string     authority      = 1;
  string     content        = 2;
  SampleType sample_type    = 3;
  string     domain         = 4;
  string     source_uri     = 5;
  string     original_author = 6;
  string     license        = 7;
  uint64     quality_score  = 8; // initial quality (0-1,000,000)
}

message MsgAddSampleResponse {
  string sample_id = 1;
}
```

**Step 2: Commit**

```bash
git add proto/zerone/knowledge/v1/tx.proto
git commit -m "feat(knowledge): rewrite tx.proto for training data protocol (R36-2)

Add SubmitThread, CreateDataset, AccessDataset, AccessSample, FundBounty RPCs.
Remove ChallengeDomainProposal, RegisterStratum, UpdateExtendedParams.
MsgContestSample now uses ContestType enum.
MsgSubmitReveal now uses typed QualityVote instead of string."
```

---

### Task 3: Rewrite query.proto

**Files:**
- Rewrite: `proto/zerone/knowledge/v1/query.proto`

**Step 1: Write the full query.proto**

Replace entire file. Key changes:
- **Add queries:** SamplesByThread, TrainingDemand, DataBounties, DomainStats, ProtocolStats
- **Rename:** DatasetInfo → Dataset
- **Remove:** SamplesByTag, SamplesByFitness, BootstrapFundStatus, SamplesAtRisk, SampleThread, ScrapedSources, CheckNovelty, ActiveBounties, DemandSignals, TopDemandGaps, NicheInfo, NichesByDomain, DomainDiversity, DomainDiversityHistory, ValidatorIndependence, ConformityAlerts, MetabolismStatus, DomainCapacity, EpistemicTemperature, RoleElasticity
- **Keep:** Sample, Samples, SamplesByDomain, SamplesBySubmitter, Submission, PendingSubmissions, QualityRound, Domain, Domains, Params, Dataset(s)

Full file content:

```protobuf
syntax = "proto3";
package zerone.knowledge.v1;

option go_package = "github.com/zerone-chain/zerone/x/knowledge/types";

import "cosmos/base/query/v1beta1/pagination.proto";
import "google/api/annotations.proto";
import "zerone/knowledge/v1/types.proto";
import "zerone/knowledge/v1/genesis.proto";

// Query defines the gRPC query service for the knowledge module.
service Query {
  // ─── Samples ───────────────────────────────────────────────────────────────

  // Sample queries a single sample by ID.
  rpc Sample(QuerySampleRequest) returns (QuerySampleResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/samples/{id}";
  }

  // Samples queries samples with optional filters.
  rpc Samples(QuerySamplesRequest) returns (QuerySamplesResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/samples";
  }

  // SamplesByDomain queries all samples in a domain.
  rpc SamplesByDomain(QuerySamplesByDomainRequest) returns (QuerySamplesResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/samples/domain/{domain}";
  }

  // SamplesByThread queries all samples in a conversation thread.
  rpc SamplesByThread(QuerySamplesByThreadRequest) returns (QuerySamplesResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/samples/thread/{thread_id}";
  }

  // SamplesBySubmitter queries all samples submitted by an address.
  rpc SamplesBySubmitter(QuerySamplesBySubmitterRequest) returns (QuerySamplesResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/samples/submitter/{submitter}";
  }

  // ─── Submissions ───────────────────────────────────────────────────────────

  // Submission queries a single submission by ID.
  rpc Submission(QuerySubmissionRequest) returns (QuerySubmissionResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/submissions/{id}";
  }

  // PendingSubmissions queries all pending submissions.
  rpc PendingSubmissions(QueryPendingSubmissionsRequest) returns (QuerySubmissionsResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/submissions/pending";
  }

  // ─── Quality rounds ────────────────────────────────────────────────────────

  // QualityRound queries a quality round by ID.
  rpc QualityRound(QueryQualityRoundRequest) returns (QueryQualityRoundResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/rounds/{id}";
  }

  // ─── Datasets ──────────────────────────────────────────────────────────────

  // Dataset queries a dataset by ID.
  rpc Dataset(QueryDatasetRequest) returns (QueryDatasetResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/datasets/{id}";
  }

  // Datasets queries all datasets with optional filters.
  rpc Datasets(QueryDatasetsRequest) returns (QueryDatasetsResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/datasets";
  }

  // ─── Demand & bounties ─────────────────────────────────────────────────────

  // TrainingDemand queries training demand data.
  rpc TrainingDemand(QueryTrainingDemandRequest) returns (QueryTrainingDemandResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/demand";
  }

  // DataBounties queries active data bounties.
  rpc DataBounties(QueryDataBountiesRequest) returns (QueryDataBountiesResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/bounties";
  }

  // ─── Domains ───────────────────────────────────────────────────────────────

  // Domain queries a domain by name.
  rpc Domain(QueryDomainRequest) returns (QueryDomainResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/domains/{name}";
  }

  // Domains queries all domains with optional pagination.
  rpc Domains(QueryDomainsRequest) returns (QueryDomainsResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/domains";
  }

  // ─── Stats ─────────────────────────────────────────────────────────────────

  // DomainStats queries statistics for a specific domain.
  rpc DomainStats(QueryDomainStatsRequest) returns (QueryDomainStatsResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/stats/domain/{domain}";
  }

  // ProtocolStats queries aggregate protocol statistics.
  rpc ProtocolStats(QueryProtocolStatsRequest) returns (QueryProtocolStatsResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/stats";
  }

  // ─── Params ────────────────────────────────────────────────────────────────

  // Params queries module parameters.
  rpc Params(QueryParamsRequest) returns (QueryParamsResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/params";
  }
}

// ─── Sample query messages ────────────────────────────────────────────────────

message QuerySampleRequest {
  string id = 1;
}

message QuerySampleResponse {
  Sample sample = 1;
}

message QuerySamplesRequest {
  string     domain      = 1;
  string     status      = 2;
  SampleType sample_type = 3;
  cosmos.base.query.v1beta1.PageRequest pagination = 4;
}

message QuerySamplesResponse {
  repeated Sample samples = 1;
  cosmos.base.query.v1beta1.PageResponse pagination = 2;
}

message QuerySamplesByDomainRequest {
  string domain = 1;
  cosmos.base.query.v1beta1.PageRequest pagination = 2;
}

message QuerySamplesByThreadRequest {
  string thread_id = 1;
  cosmos.base.query.v1beta1.PageRequest pagination = 2;
}

message QuerySamplesBySubmitterRequest {
  string submitter = 1;
  cosmos.base.query.v1beta1.PageRequest pagination = 2;
}

// ─── Submission query messages ────────────────────────────────────────────────

message QuerySubmissionRequest {
  string id = 1;
}

message QuerySubmissionResponse {
  Submission submission = 1;
}

message QueryPendingSubmissionsRequest {
  cosmos.base.query.v1beta1.PageRequest pagination = 1;
}

message QuerySubmissionsResponse {
  repeated Submission submissions = 1;
  cosmos.base.query.v1beta1.PageResponse pagination = 2;
}

// ─── Quality round query messages ─────────────────────────────────────────────

message QueryQualityRoundRequest {
  string id = 1;
}

message QueryQualityRoundResponse {
  QualityRound round = 1;
}

// ─── Dataset query messages ───────────────────────────────────────────────────

message QueryDatasetRequest {
  string id = 1;
}

message QueryDatasetResponse {
  Dataset dataset = 1;
}

message QueryDatasetsRequest {
  string domain = 1;
  cosmos.base.query.v1beta1.PageRequest pagination = 2;
}

message QueryDatasetsResponse {
  repeated Dataset datasets = 1;
  cosmos.base.query.v1beta1.PageResponse pagination = 2;
}

// ─── Demand & bounty query messages ───────────────────────────────────────────

message QueryTrainingDemandRequest {
  string domain = 1;
  cosmos.base.query.v1beta1.PageRequest pagination = 2;
}

message QueryTrainingDemandResponse {
  repeated TrainingDemand signals = 1;
  cosmos.base.query.v1beta1.PageResponse pagination = 2;
}

message QueryDataBountiesRequest {
  string domain = 1;
  cosmos.base.query.v1beta1.PageRequest pagination = 2;
}

message QueryDataBountiesResponse {
  repeated DataBounty bounties = 1;
  cosmos.base.query.v1beta1.PageResponse pagination = 2;
}

// ─── Domain query messages ────────────────────────────────────────────────────

message QueryDomainRequest {
  string name = 1;
}

message QueryDomainResponse {
  Domain domain = 1;
}

message QueryDomainsRequest {
  cosmos.base.query.v1beta1.PageRequest pagination = 1;
}

message QueryDomainsResponse {
  repeated Domain domains = 1;
  cosmos.base.query.v1beta1.PageResponse pagination = 2;
}

// ─── Stats query messages ─────────────────────────────────────────────────────

message QueryDomainStatsRequest {
  string domain = 1;
}

message QueryDomainStatsResponse {
  string domain          = 1;
  uint64 sample_count    = 2;
  uint64 pending_count   = 3;
  uint64 gold_count      = 4;
  uint64 silver_count    = 5;
  uint64 bronze_count    = 6;
  uint64 rejected_count  = 7;
  string total_revenue   = 8;
  uint64 active_bounties = 9;
}

message QueryProtocolStatsRequest {}

message QueryProtocolStatsResponse {
  uint64 total_samples       = 1;
  uint64 total_submissions   = 2;
  uint64 total_datasets      = 3;
  uint64 total_domains       = 4;
  uint64 active_rounds       = 5;
  string total_revenue       = 6;
  uint64 total_access_count  = 7;
}

// ─── Params query messages ────────────────────────────────────────────────────

message QueryParamsRequest {}

message QueryParamsResponse {
  Params params = 1;
}
```

**Step 2: Commit**

```bash
git add proto/zerone/knowledge/v1/query.proto
git commit -m "feat(knowledge): rewrite query.proto for training data protocol (R36-2)

Add SamplesByThread, TrainingDemand, DataBounties, DomainStats, ProtocolStats.
Simplify query surface: remove ecology queries (niche, diversity, metabolism, etc.).
Unified QuerySamplesResponse for all sample list queries.
All list queries use standard Cosmos SDK pagination."
```

---

### Task 4: Run proto-gen and verify

**Files:**
- Generated: `x/knowledge/types/tx.pb.go`, `x/knowledge/types/tx_grpc.pb.go`
- Generated: `x/knowledge/types/query.pb.go`, `x/knowledge/types/query_grpc.pb.go`
- Generated: `x/knowledge/types/types.pb.go`

**Step 1: Run buf generate**

```bash
make proto-gen
```

Expected: Success, no errors.

**Step 2: Verify new types exist**

```bash
grep "MsgSubmitThread\b" x/knowledge/types/tx.pb.go
grep "MsgCreateDataset\b" x/knowledge/types/tx.pb.go
grep "MsgAccessDataset\b" x/knowledge/types/tx.pb.go
grep "MsgFundBounty\b" x/knowledge/types/tx.pb.go
grep "ContestType" x/knowledge/types/types.pb.go
grep "QuerySamplesByThreadRequest\b" x/knowledge/types/query.pb.go
grep "QueryDomainStatsRequest\b" x/knowledge/types/query.pb.go
grep "QueryProtocolStatsRequest\b" x/knowledge/types/query.pb.go
```

Expected: All grep commands find matches.

**Step 3: Verify removed types are gone**

```bash
grep "MsgChallengeDomainProposal\b" x/knowledge/types/tx.pb.go && echo "FAIL: still exists" || echo "OK: removed"
grep "MsgRegisterStratum\b" x/knowledge/types/tx.pb.go && echo "FAIL: still exists" || echo "OK: removed"
grep "MsgUpdateExtendedParams\b" x/knowledge/types/tx.pb.go && echo "FAIL: still exists" || echo "OK: removed"
```

Expected: All print "OK: removed"

**Step 4: Commit generated files**

```bash
git add x/knowledge/types/tx.pb.go x/knowledge/types/tx_grpc.pb.go \
        x/knowledge/types/query.pb.go x/knowledge/types/query_grpc.pb.go \
        x/knowledge/types/types.pb.go
git commit -m "chore(knowledge): regenerate protobuf Go code (R36-2)"
```

---

### Task 5: Update codec.go to match new message types

**Files:**
- Modify: `x/knowledge/types/codec.go`

**Step 1: Rewrite codec.go registrations**

Replace the old type names (MsgSubmitClaim, MsgChallengeFact, etc.) with the current proto-generated types. The old codec.go references types that no longer exist in the generated code.

```go
package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// RegisterCodec registers the knowledge module's types with the legacy amino codec.
func RegisterCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgSubmitData{}, "zerone_knowledge/SubmitData", nil)
	cdc.RegisterConcrete(&MsgSubmitThread{}, "zerone_knowledge/SubmitThread", nil)
	cdc.RegisterConcrete(&MsgSubmitCommitment{}, "zerone_knowledge/SubmitCommitment", nil)
	cdc.RegisterConcrete(&MsgSubmitReveal{}, "zerone_knowledge/SubmitReveal", nil)
	cdc.RegisterConcrete(&MsgContestSample{}, "zerone_knowledge/ContestSample", nil)
	cdc.RegisterConcrete(&MsgSponsorSample{}, "zerone_knowledge/SponsorSample", nil)
	cdc.RegisterConcrete(&MsgProposeDomain{}, "zerone_knowledge/ProposeDomain", nil)
	cdc.RegisterConcrete(&MsgEndorseDomainProposal{}, "zerone_knowledge/EndorseDomainProposal", nil)
	cdc.RegisterConcrete(&MsgCreateDataset{}, "zerone_knowledge/CreateDataset", nil)
	cdc.RegisterConcrete(&MsgAccessDataset{}, "zerone_knowledge/AccessDataset", nil)
	cdc.RegisterConcrete(&MsgAccessSample{}, "zerone_knowledge/AccessSample", nil)
	cdc.RegisterConcrete(&MsgReportDemand{}, "zerone_knowledge/ReportDemand", nil)
	cdc.RegisterConcrete(&MsgFundBounty{}, "zerone_knowledge/FundBounty", nil)
	cdc.RegisterConcrete(&MsgRateSample{}, "zerone_knowledge/RateSample", nil)
	cdc.RegisterConcrete(&MsgUpdateParams{}, "zerone_knowledge/UpdateParams", nil)
	cdc.RegisterConcrete(&MsgAddScrapedSource{}, "zerone_knowledge/AddScrapedSource", nil)
	cdc.RegisterConcrete(&MsgRemoveScrapedSource{}, "zerone_knowledge/RemoveScrapedSource", nil)
	cdc.RegisterConcrete(&MsgProposeResearchFund{}, "zerone_knowledge/ProposeResearchFund", nil)
	cdc.RegisterConcrete(&MsgVoteResearchProposal{}, "zerone_knowledge/VoteResearchProposal", nil)
	cdc.RegisterConcrete(&MsgExecuteResearchProposal{}, "zerone_knowledge/ExecuteResearchProposal", nil)
	cdc.RegisterConcrete(&MsgAddSample{}, "zerone_knowledge/AddSample", nil)
}

// RegisterInterfaces registers the knowledge module's interface implementations.
func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgSubmitData{},
		&MsgSubmitThread{},
		&MsgSubmitCommitment{},
		&MsgSubmitReveal{},
		&MsgContestSample{},
		&MsgSponsorSample{},
		&MsgProposeDomain{},
		&MsgEndorseDomainProposal{},
		&MsgCreateDataset{},
		&MsgAccessDataset{},
		&MsgAccessSample{},
		&MsgReportDemand{},
		&MsgFundBounty{},
		&MsgRateSample{},
		&MsgUpdateParams{},
		&MsgAddScrapedSource{},
		&MsgRemoveScrapedSource{},
		&MsgProposeResearchFund{},
		&MsgVoteResearchProposal{},
		&MsgExecuteResearchProposal{},
		&MsgAddSample{},
	)
}

var (
	Amino     = codec.NewLegacyAmino()
	ModuleCdc = codec.NewProtoCodec(cdctypes.NewInterfaceRegistry())
)
```

**Step 2: Verify build compiles (types package only)**

```bash
cd x/knowledge/types && go build ./...
```

Expected: No errors (note: full app may not build yet due to server stubs referencing old types — that's a separate task).

**Step 3: Commit**

```bash
git add x/knowledge/types/codec.go
git commit -m "fix(knowledge): update codec.go registrations for new message types (R36-2)"
```

---

### Task 6: Run proto-check

**Step 1: Run proto audit**

```bash
make proto-check
```

Expected: No proto/Go mismatches for the knowledge module.

**Step 2: Final commit if any fixups needed**

If proto-check flags issues, fix them and commit.
