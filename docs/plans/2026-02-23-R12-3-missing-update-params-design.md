# R12-3: Add MsgUpdateParams to 4 Missing Modules

## Decision

Add standard `MsgUpdateParams` governance-gated parameter updates to: `capture_challenge`, `capture_defense`, `disputes`, `home`.

`claiming_pot` excluded — already has `MsgUpdatePotParams` which is functionally equivalent.

## Pattern (from billing reference)

Each module gets 4 additions:

1. **Proto** (`proto/zerone/<module>/v1/tx.proto`): `rpc UpdateParams`, `MsgUpdateParams` message (authority + params), `MsgUpdateParamsResponse`
2. **Handler** (`x/<module>/keeper/msg_server.go`): authority check, `Validate()`, `SetParams()`, emit `zerone.<module>.params_updated` event
3. **Codec** (`x/<module>/types/codec.go`): register `MsgUpdateParams` in amino + interface registry
4. **Tests** (`x/<module>/keeper/msg_server_update_params_test.go`): valid update, unauthorized, invalid params

## Prerequisites confirmed

All 4 modules already have:
- `Params` proto message in genesis.proto
- `Validate()` on Params type
- `authority` field on keeper struct
- `GetAuthority()` method
- `SetParams()` method

## Out of scope

- CLI `CmdUpdateParams` (optional per R12-3; even billing doesn't have one)
- Renaming claiming_pot's `MsgUpdatePotParams`
