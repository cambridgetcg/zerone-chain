package types

import (
	"context"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// StakingKeeper defines the expected staking module interface for tier checks.
type StakingKeeper interface {
	GetValidatorTier(ctx context.Context, addr string) (uint32, error)
}

// AuthKeeper defines the expected auth module interface for registration age.
type AuthKeeper interface {
	GetRegistrationBlock(ctx context.Context, addr string) (uint64, error)
}

// BankKeeper defines the expected bank module interface for fund transfers.
// The bootstrap pathway forwards minted coins from the claiming_pot module
// account to the claimer in the same transaction; the module account never
// holds funds across blocks.
type BankKeeper interface {
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
}

// VestingRewardsKeeper is the chain's single cap-gated mint entry point.
// x/claiming_pot uses MintWithCap to mint bootstrap claims into its own
// module account; the cap is enforced once, regardless of which emission
// pathway calls. See docs/tokenomics/SUPPLY.md (emission pathways) and
// docs/tokenomics/GENESIS.md (zero team allocation, two participation-
// gated emission pathways).
type VestingRewardsKeeper interface {
	MintWithCap(ctx sdk.Context, recipientModule string, amount *big.Int) (*big.Int, error)
}
