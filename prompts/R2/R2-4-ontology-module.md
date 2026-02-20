# R2-4 — Ontology Module: Full Port

## Goal

Port the ontology module — domain taxonomy, knowledge strata, and relation
types that organize the knowledge tree. Knowledge module depends on ontology
for domain validation.

## Dependencies

- R1-2 (core proto) must be complete. Independent of R2-1/R2-2.

## Working Directory

`/Users/yuai/Desktop/Zerone`

## Reference

- `/Users/yuai/Desktop/legible_money/x/ontology/` — full module
- `/Users/yuai/Desktop/legible_money/proto/legible/ontology/` — proto definitions
- `/Users/yuai/Desktop/legible_money/x/ontology/keeper/keeper_test.go` — 81 tests

## Proto Files

### `proto/zerone/ontology/v1/types.proto`
```protobuf
message OntologyDomain {
  string name = 1;
  string description = 2;
  string parent = 3;             // hierarchical domains
  uint64 depth = 4;
  string status = 5;             // "active", "deprecated"
  uint64 created_at_block = 6;
}

message Stratum {
  string name = 1;
  string description = 2;
  uint32 level = 3;              // hierarchy level
  uint64 fact_count = 4;
}

message RelationType {
  string name = 1;               // "cites", "contradicts", "extends", "specializes"
  string description = 2;
  bool is_symmetric = 3;
  bool is_transitive = 4;
}

message Relation {
  string id = 1;
  string from_fact = 2;
  string to_fact = 3;
  string relation_type = 4;
  string creator = 5;
  uint64 created_at_block = 6;
}
```

### `proto/zerone/ontology/v1/tx.proto`
- MsgCreateDomain
- MsgUpdateDomain
- MsgDeprecateDomain
- MsgCreateStratum
- MsgCreateRelationType
- MsgCreateRelation
- MsgDeleteRelation
- MsgBulkCreateRelations
- MsgUpdateOntologyParams
- MsgMigrateDomain
- MsgUpdateParams

### `proto/zerone/ontology/v1/query.proto`
- QueryDomain, QueryDomains, QueryStratum, QueryStrata
- QueryRelation, QueryRelationsByFact, QueryRelationTypes
- QueryDomainHierarchy, QueryParams

### `proto/zerone/ontology/v1/genesis.proto`
- Params (domain_max_depth, max_relations_per_fact, etc.)
- GenesisState { params, domains, strata, relation_types, relations }

## Module Implementation

### Keeper
- Domain CRUD with hierarchy validation
- Stratum management
- Relation CRUD with symmetry/transitivity enforcement
- Domain migration (move facts between domains)
- BankKeeper for any staking requirements

### Port all 81 tests

All tests from draft carry over with proto types.

### Migrator stub

```go
type Migrator struct { keeper Keeper }
```

### Wire into app.go

- Store key, keeper, ModuleManager
- BeginBlocker (if needed — check draft)
- InitGenesis with default domains (18 genesis domains from draft)

## Verification

```bash
make proto-gen
go build ./...
go test ./x/ontology/... -count=1 -v
```

## Commit

```
feat(ontology): domain taxonomy, strata, relations — organizing the knowledge tree
```

## Do NOT

- Skip the 18 default genesis domains (they're needed by knowledge module)
- Allow circular domain hierarchies
- Skip relation symmetry enforcement
- Use hand-written types
