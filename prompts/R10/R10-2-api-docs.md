# R10-2 — API Documentation

## Goal

Auto-generate OpenAPI/Swagger documentation from proto files. Enable gRPC reflection.
Serve API docs from the running node. Document all endpoints.

## Working Directory

`/Users/yuai/Desktop/Zerone`

## Deliverables

### 1. OpenAPI Spec Generation

Generate OpenAPI v2 (Swagger) spec from proto files using `grpc-gateway`:
- All query endpoints for all 32 custom modules
- All tx endpoints
- Standard Cosmos SDK endpoints (bank, staking, gov, etc.)

Create or update `Makefile` target:
```makefile
proto-swagger:
	@echo "Generating Swagger..."
	buf generate --template buf.gen.swagger.yaml
	# Merge all per-module swagger files into one
	swagger-combine docs/swagger-ui/swagger.yaml \
	  $(wildcard proto/zerone/*/v1/*.swagger.json)
```

If `swagger-combine` is too complex, use `protoc-gen-openapiv2` to generate a single spec,
or manually create a combined spec.

### 2. Swagger UI

Embed Swagger UI in the node's API server:
- Serve at `http://localhost:1317/swagger/` (standard Cosmos SDK location)
- The app already has a placeholder (`app.go` line 1363) — wire it up
- Use the embedded Swagger UI from `github.com/cosmos/cosmos-sdk/client/docs`

### 3. gRPC Reflection

Enable gRPC reflection so tools like `grpcurl` and `grpc-ui` work:
```go
// In app.go or server setup
import "google.golang.org/grpc/reflection"
reflection.Register(grpcServer)
```

Verify with:
```bash
grpcurl -plaintext localhost:9090 list
grpcurl -plaintext localhost:9090 describe zerone.knowledge.v1.Query
```

### 4. API Reference (`docs/API.md`)

Human-readable API reference documenting:
- REST endpoints (with curl examples)
- gRPC endpoints (with grpcurl examples)
- WebSocket subscription endpoints
- Authentication (none for testnet, explain future plans)

Group by module:
```markdown
## Knowledge Module

### Query Facts
GET /zerone/knowledge/v1/facts/{id}

### Submit Claim
POST /zerone/knowledge/v1/tx/submit-claim
```

### 5. Client Examples

Create `docs/examples/` with:
- `query_facts.sh` — curl examples for querying knowledge
- `submit_claim.sh` — example of submitting a knowledge claim
- `check_balance.sh` — check ZRN balance
- `delegate.sh` — delegate to a validator

## Constraints

- OpenAPI spec must be generated from proto (not hand-written)
- Every Query service endpoint must appear in the spec
- Swagger UI must be servable from the running node
- Examples must work against a running testnet
- Use the standard Cosmos SDK API server port (1317 for REST, 9090 for gRPC)
