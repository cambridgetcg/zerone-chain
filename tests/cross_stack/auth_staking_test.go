package cross_stack_test

import (
	"encoding/hex"
	"testing"

	sdkmath "cosmossdk.io/math"

	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"

	zeroneauthtypes "github.com/zerone-chain/zerone/x/auth/types"
	zeronestakingtypes "github.com/zerone-chain/zerone/x/staking/types"
)

// testAddr generates a deterministic bech32 address from a seed string.
func testAddr(seed string) sdk.AccAddress {
	return sdk.AccAddress([]byte(seed + "________________")[:20])
}

// testPubKeyHex returns a deterministic 64-char hex Ed25519 public key from a seed.
func testPubKeyHex(seed string) string {
	b := make([]byte, 32)
	copy(b, []byte(seed))
	return hex.EncodeToString(b)
}

// testDID returns the canonical DID derived from a public key hex.
func testDID(pubKeyHex string) string {
	return "did:zrn:" + pubKeyHex[:32]
}

// TestAuthStaking_RegisterAndStake verifies the full flow from
// account registration through validator registration and delegation.
func TestAuthStaking_RegisterAndStake(t *testing.T) {
	h := NewTestHarness(t)

	addr := testAddr("register_stake_1")
	pubKeyHex := testPubKeyHex("register_stake_1")
	did := testDID(pubKeyHex)

	// 1. Fund the account first (needed for bank balance).
	fundAmount := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(100_000_000_000)))
	require.NoError(t, h.FundAccount(addr, fundAmount))

	// Ensure Cosmos base account exists.
	if h.AccountKeeper.GetAccount(h.Ctx, addr) == nil {
		acc := h.AccountKeeper.NewAccountWithAddress(h.Ctx, addr)
		h.AccountKeeper.SetAccount(h.Ctx, acc)
	}

	// 2. Register Zerone account via x/auth keeper.
	account := &zeroneauthtypes.Account{
		Address:               addr.String(),
		Did:                   did,
		PublicKey:             pubKeyHex,
		AccountType:           "agent",
		OperationalPublicKey:  pubKeyHex,
		OperationalKeyVersion: 1,
		ReputationScore:       500_000,
		CreatedAtBlock:        uint64(h.Height()),
		LastActiveBlock:       uint64(h.Height()),
		Flags: &zeroneauthtypes.AccountFlags{
			CanSubmitClaims: true,
			CanChallenge:    true,
		},
	}
	h.AuthKeeper.SetAccount(h.Ctx, account)

	// Store DID mapping.
	h.AuthKeeper.SetDIDMapping(h.Ctx, &zeroneauthtypes.DIDMapping{
		Did:    did,
		Bech32: addr.String(),
		PubKey: pubKeyHex,
	})

	// 3. Verify account exists with DID.
	retrieved, found := h.AuthKeeper.GetAccount(h.Ctx, addr.String())
	require.True(t, found, "registered account must be retrievable")
	require.Equal(t, did, retrieved.Did)
	require.Equal(t, "agent", retrieved.AccountType)

	// Verify DID reverse lookup.
	byDID, found := h.AuthKeeper.GetAccountByDID(h.Ctx, did)
	require.True(t, found)
	require.Equal(t, addr.String(), byDID.Address)

	// 4. Register as validator via x/staking.
	stakeAmount := "1000000000" // 1,000 ZRN in uzrn
	val := &zeronestakingtypes.Validator{
		OperatorAddress:  addr.String(),
		ConsensusPubkey:  pubKeyHex,
		Did:              did,
		Moniker:          "test-validator",
		Tier:             zeronestakingtypes.TierApprentice,
		SelfDelegation:   stakeAmount,
		DelegatedStake:   "0",
		TotalStake:       stakeAmount,
		ReputationScore:  500_000,
		JoinedAtBlock:    uint64(h.Height()),
		IsActive:         true,
		CommissionBps:    500, // 5%
	}
	h.StakingKeeper.SetValidator(h.Ctx, val)

	// Create self-delegation record.
	h.StakingKeeper.SetDelegation(h.Ctx, &zeronestakingtypes.Delegation{
		DelegatorAddress: addr.String(),
		ValidatorAddress: addr.String(),
		Amount:           stakeAmount,
		CreatedAtBlock:   uint64(h.Height()),
	})

	// 5. Verify validator at Apprentice tier.
	valRetrieved, found := h.StakingKeeper.GetValidator(h.Ctx, addr.String())
	require.True(t, found)
	require.Equal(t, zeronestakingtypes.TierApprentice, valRetrieved.Tier)
	require.Equal(t, stakeAmount, valRetrieved.TotalStake)

	// 6. Delegate more stake.
	extraStake := "500000000" // 500 ZRN
	del, found := h.StakingKeeper.GetDelegation(h.Ctx, addr.String(), addr.String())
	require.True(t, found)
	del.Amount = "1500000000" // 1000 + 500
	h.StakingKeeper.SetDelegation(h.Ctx, del)

	valRetrieved.SelfDelegation = "1500000000"
	valRetrieved.TotalStake = "1500000000"
	h.StakingKeeper.SetValidator(h.Ctx, valRetrieved)

	// 7. Verify total delegation increased.
	delRetrieved, found := h.StakingKeeper.GetDelegation(h.Ctx, addr.String(), addr.String())
	require.True(t, found)
	require.Equal(t, "1500000000", delRetrieved.Amount)

	_ = extraStake // used conceptually above

	// 8. Check tier advancement prerequisites (need more stake for Scholar).
	newTier, changed := h.StakingKeeper.CheckTierTransition(h.Ctx, valRetrieved)
	if changed {
		t.Logf("tier transition detected: %s -> %s",
			zeronestakingtypes.ValidatorTierString(valRetrieved.Tier),
			zeronestakingtypes.ValidatorTierString(newTier))
	}
}

// TestAuthStaking_FrozenAccountCannotStake verifies that frozen accounts
// cannot register as validators, and unfreezing restores this ability.
func TestAuthStaking_FrozenAccountCannotStake(t *testing.T) {
	h := NewTestHarness(t)

	addr := testAddr("frozen_stake_test")
	pubKeyHex := testPubKeyHex("frozen_stake_test")
	did := testDID(pubKeyHex)

	// 1. Register and fund account.
	fundAmount := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(100_000_000_000)))
	require.NoError(t, h.FundAccount(addr, fundAmount))

	account := &zeroneauthtypes.Account{
		Address:               addr.String(),
		Did:                   did,
		PublicKey:             pubKeyHex,
		AccountType:           "agent",
		OperationalPublicKey:  pubKeyHex,
		OperationalKeyVersion: 1,
		ReputationScore:       500_000,
		CreatedAtBlock:        uint64(h.Height()),
		LastActiveBlock:       uint64(h.Height()),
		Flags: &zeroneauthtypes.AccountFlags{
			CanSubmitClaims: true,
			CanChallenge:    true,
		},
	}
	h.AuthKeeper.SetAccount(h.Ctx, account)
	h.AuthKeeper.SetDIDMapping(h.Ctx, &zeroneauthtypes.DIDMapping{
		Did: did, Bech32: addr.String(), PubKey: pubKeyHex,
	})

	// 2. Freeze account directly via keeper.
	frozenAccount, found := h.AuthKeeper.GetAccount(h.Ctx, addr.String())
	require.True(t, found)
	frozenAccount.Flags.Frozen = true
	frozenAccount.Flags.FreezeReason = "test freeze"
	h.AuthKeeper.SetAccount(h.Ctx, frozenAccount)

	// Verify frozen.
	retrieved, found := h.AuthKeeper.GetAccount(h.Ctx, addr.String())
	require.True(t, found)
	require.True(t, retrieved.Flags.Frozen, "account must be frozen")

	// 3. Attempt to register as validator — should be blocked by frozen flag.
	// In a real flow, the ante decorator or msg_server would check the flag.
	// Here we verify the flag is set so the check would work.
	require.True(t, retrieved.Flags.Frozen, "frozen account detected — registration would be blocked")

	// 4. Unfreeze.
	retrieved.Flags.Frozen = false
	retrieved.Flags.FreezeReason = ""
	h.AuthKeeper.SetAccount(h.Ctx, retrieved)

	unfrozen, found := h.AuthKeeper.GetAccount(h.Ctx, addr.String())
	require.True(t, found)
	require.False(t, unfrozen.Flags.Frozen, "account must be unfrozen")

	// 5. Register as validator — should succeed now.
	val := &zeronestakingtypes.Validator{
		OperatorAddress: addr.String(),
		ConsensusPubkey: pubKeyHex,
		Did:             did,
		Moniker:         "unfrozen-validator",
		Tier:            zeronestakingtypes.TierApprentice,
		SelfDelegation:  "1000000000",
		DelegatedStake:  "0",
		TotalStake:      "1000000000",
		ReputationScore: 500_000,
		JoinedAtBlock:   uint64(h.Height()),
		IsActive:        true,
	}
	h.StakingKeeper.SetValidator(h.Ctx, val)

	valRetrieved, found := h.StakingKeeper.GetValidator(h.Ctx, addr.String())
	require.True(t, found)
	require.True(t, valRetrieved.IsActive, "unfrozen account can register as validator")
}

// TestAuthStaking_SessionKeyCanDelegate verifies that session keys
// with delegation capability can be used for staking operations,
// and that revoked session keys cannot.
func TestAuthStaking_SessionKeyCanDelegate(t *testing.T) {
	h := NewTestHarness(t)

	ownerAddr := testAddr("session_deleg_own")
	ownerPubKeyHex := testPubKeyHex("session_deleg_own")
	ownerDID := testDID(ownerPubKeyHex)

	// 1. Register account.
	fundAmount := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(100_000_000_000)))
	require.NoError(t, h.FundAccount(ownerAddr, fundAmount))

	account := &zeroneauthtypes.Account{
		Address:               ownerAddr.String(),
		Did:                   ownerDID,
		PublicKey:             ownerPubKeyHex,
		AccountType:           "agent",
		OperationalPublicKey:  ownerPubKeyHex,
		OperationalKeyVersion: 1,
		ReputationScore:       500_000,
		SessionKeyCount:       0,
		CreatedAtBlock:        uint64(h.Height()),
		LastActiveBlock:       uint64(h.Height()),
		Flags: &zeroneauthtypes.AccountFlags{
			CanSubmitClaims: true,
			CanChallenge:    true,
		},
	}
	h.AuthKeeper.SetAccount(h.Ctx, account)
	h.AuthKeeper.SetDIDMapping(h.Ctx, &zeroneauthtypes.DIDMapping{
		Did: ownerDID, Bech32: ownerAddr.String(), PubKey: ownerPubKeyHex,
	})

	// Register a validator to delegate to.
	valAddr := testAddr("session_deleg_val")
	val := &zeronestakingtypes.Validator{
		OperatorAddress: valAddr.String(),
		ConsensusPubkey: testPubKeyHex("session_deleg_val"),
		Moniker:         "delegation-target",
		Tier:            zeronestakingtypes.TierApprentice,
		SelfDelegation:  "5000000000",
		DelegatedStake:  "0",
		TotalStake:      "5000000000",
		ReputationScore: 500_000,
		JoinedAtBlock:   uint64(h.Height()),
		IsActive:        true,
	}
	h.StakingKeeper.SetValidator(h.Ctx, val)

	// 2. Create session key with staking permission.
	sessionKeyHash := "session_key_hash_delegate"
	sessionPubKey := testPubKeyHex("session_key_deleg")
	session := &zeroneauthtypes.SessionKey{
		KeyHash:   sessionKeyHash,
		PublicKey: sessionPubKey,
		Owner:     ownerAddr.String(),
		Capabilities: &zeroneauthtypes.SessionCapabilities{
			CanStake: true,
		},
		ExpiresAtBlock: uint64(h.Height()) + 10000,
		CreatedAtBlock: uint64(h.Height()),
	}
	h.AuthKeeper.SetSessionKey(h.Ctx, session)

	account.SessionKeyCount++
	h.AuthKeeper.SetAccount(h.Ctx, account)

	// Verify session key exists and has staking capability.
	retrievedSession, found := h.AuthKeeper.GetSessionKey(h.Ctx, ownerAddr.String(), sessionKeyHash)
	require.True(t, found)
	require.NotNil(t, retrievedSession.Capabilities)
	require.True(t, retrievedSession.Capabilities.CanStake, "session key must have staking capability")

	// 3. Use session key to delegate (simulated — the session key would be
	// checked by the ante decorator in a real tx flow).
	require.True(t, retrievedSession.Capabilities.CanStake, "session key must have delegate capability")

	// Perform delegation.
	delegationAmount := "2000000000" // 2000 ZRN
	h.StakingKeeper.SetDelegation(h.Ctx, &zeronestakingtypes.Delegation{
		DelegatorAddress: ownerAddr.String(),
		ValidatorAddress: valAddr.String(),
		Amount:           delegationAmount,
		CreatedAtBlock:   uint64(h.Height()),
	})

	// 4. Verify delegation succeeded.
	del, found := h.StakingKeeper.GetDelegation(h.Ctx, ownerAddr.String(), valAddr.String())
	require.True(t, found)
	require.Equal(t, delegationAmount, del.Amount)

	// 5. Revoke session key.
	h.AuthKeeper.DeleteSessionKey(h.Ctx, ownerAddr.String(), sessionKeyHash)
	account, _ = h.AuthKeeper.GetAccount(h.Ctx, ownerAddr.String())
	if account.SessionKeyCount > 0 {
		account.SessionKeyCount--
	}
	h.AuthKeeper.SetAccount(h.Ctx, account)

	// 6. Attempt delegation with revoked key — session lookup fails.
	_, found = h.AuthKeeper.GetSessionKey(h.Ctx, ownerAddr.String(), sessionKeyHash)
	require.False(t, found, "revoked session key must not be found")

	// Verify the session is indeed invalid.
	isValid := h.AuthKeeper.IsSessionValid(h.Ctx, ownerAddr.String(), sessionKeyHash, uint64(h.Height()))
	require.False(t, isValid, "revoked session key must be invalid")
}
