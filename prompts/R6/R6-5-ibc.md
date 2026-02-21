# R6-5 — IBC Rate Limiting + ICA Auth Module

## Goal

Port IBC rate limiting and the interchain accounts auth module. Apply all
security fixes from the draft audit (B17-1 P0 fixes baked in).

## Working Directory

`/Users/yuai/Desktop/Zerone`

## Reference

- `/Users/yuai/Desktop/legible_money/x/icaauth/` — ICA auth module
- Cosmos SDK IBC middleware patterns for rate limiting
- Draft audit findings (B17-1): rate limits were bypassable via ICA, timeout handling was incomplete

## Module 1: IBC Rate Limiting Middleware

### `proto/zerone/ibc_ratelimit/v1/types.proto`
```protobuf
syntax = "proto3";
package zerone.ibc_ratelimit.v1;
option go_package = "github.com/zerone-chain/zerone/x/ibc_ratelimit/types";

message RateLimit {
  string channel_id = 1;
  string denom = 2;
  string max_send_amount = 3;        // uzrn per window
  string max_recv_amount = 4;        // uzrn per window
  uint64 window_blocks = 5;          // sliding window size
  string current_send = 6;           // uzrn sent in current window
  string current_recv = 7;           // uzrn received in current window
  uint64 window_start_block = 8;
}
```

### `proto/zerone/ibc_ratelimit/v1/genesis.proto`
```protobuf
message Params {
  repeated RateLimitConfig rate_limits = 1;
  bool enabled = 2;                   // default: true
  uint64 default_window_blocks = 3;   // default: 10000
}

message RateLimitConfig {
  string channel_id = 1;             // "*" for all channels
  string denom = 2;                  // "*" for all denoms
  string max_send_amount = 3;        // uzrn
  string max_recv_amount = 4;        // uzrn
  uint64 window_blocks = 5;
}
```

### Messages
```protobuf
service Msg {
  rpc AddRateLimit(MsgAddRateLimit) returns (MsgAddRateLimitResponse);
  rpc RemoveRateLimit(MsgRemoveRateLimit) returns (MsgRemoveRateLimitResponse);
  rpc UpdateParams(MsgUpdateParams) returns (MsgUpdateParamsResponse);
}
```

### Implementation
IBC middleware that wraps the transfer module:

```go
type IBCMiddleware struct {
    app    porttypes.IBCModule
    keeper keeper.Keeper
}
```

**OnRecvPacket:** Check recv rate limit before forwarding. If exceeded, return error ack.
**OnSendPacket (via SendPacket wrapper):** Check send rate limit. If exceeded, reject.
**OnTimeoutPacket:** Reverse the send tracking (refund the rate limit quota).
**OnAcknowledgementPacket:** If error ack, reverse the send tracking.

**P0 fix from audit:** ICA-initiated transfers MUST also go through rate limiting.
Ensure the middleware is placed ABOVE the ICA module in the stack so all
transfers regardless of origin are rate-checked.

**BeginBlocker:** Reset window counters when block > window_start + window_blocks.

---

## Module 2: ICA Auth

### `proto/zerone/icaauth/v1/types.proto`
```protobuf
syntax = "proto3";
package zerone.icaauth.v1;
option go_package = "github.com/zerone-chain/zerone/x/icaauth/types";

message InterchainAccount {
  string owner = 1;                  // bech32 local owner
  string connection_id = 2;
  string port_id = 3;
  string account_address = 4;       // remote chain address
  string status = 5;                // "active", "closed"
  uint64 registered_at_block = 6;
}
```

### `proto/zerone/icaauth/v1/genesis.proto`
```protobuf
message Params {
  repeated string allowed_messages = 1;  // list of allowed msg type URLs
  uint64 max_messages_per_tx = 2;        // default: 10
  bool enabled = 3;                      // default: true
}
```

### Messages
```protobuf
service Msg {
  rpc RegisterAccount(MsgRegisterAccount) returns (MsgRegisterAccountResponse);
  rpc SubmitTx(MsgSubmitTx) returns (MsgSubmitTxResponse);
}

message MsgRegisterAccount {
  option (cosmos.msg.v1.signer) = "owner";
  string owner = 1; string connection_id = 2;
}
message MsgRegisterAccountResponse { string port_id = 1; }

message MsgSubmitTx {
  option (cosmos.msg.v1.signer) = "owner";
  string owner = 1; string connection_id = 2;
  repeated google.protobuf.Any msgs = 3;
  uint64 timeout_seconds = 4;
}
message MsgSubmitTxResponse {}
```

### Implementation
- `RegisterAccount` — register ICA on remote chain via IBC
- `SubmitTx` — validate allowed_messages whitelist, check max_messages_per_tx, submit via ICA controller
- **P0 fix:** Validate EVERY message type URL against the allowed list (draft had a bypass where unknown types were silently allowed)
- **P0 fix:** Timeout handling — properly handle ICA timeouts with channel closure cleanup

### Expected Keepers
```go
type ICAControllerKeeper interface {
    RegisterInterchainAccount(ctx context.Context, connectionID, owner, version string) error
    SendTx(ctx context.Context, chanCap *capabilitytypes.Capability, connectionID, portID string, icaPacketData icatypes.InterchainAccountPacketData, timeoutTimestamp uint64) (uint64, error)
}
```

### Queries
```protobuf
service Query {
  rpc Account(QueryAccountRequest) returns (QueryAccountResponse);
  rpc Accounts(QueryAccountsRequest) returns (QueryAccountsResponse);
  rpc Params(QueryParamsRequest) returns (QueryParamsResponse);
}
```

## Tests
- **Rate limiting:** Send within limit → OK, exceeds limit → rejected, window reset → OK again
- **Rate limiting timeout:** Send → timeout → quota restored
- **Rate limiting recv:** Receive within limit → OK, exceeds → error ack
- **ICA auth:** Register account, submit allowed tx → OK, submit disallowed msg type → rejected
- **ICA + rate limit integration:** ICA transfer counts against rate limit

## Conventions
- Token: uzrn. Module path: github.com/zerone-chain/zerone
- BPS: 1,000,000 scale
- IBC v8.8.0, ICA from ibc-go
- Run `go build ./...` and `go test ./x/ibc_ratelimit/... ./x/icaauth/...` before finishing
