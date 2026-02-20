package app

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	ibcante "github.com/cosmos/ibc-go/v8/modules/core/ante"
)

// NewAnteHandler returns the standard Cosmos SDK ante handler chain.
// Custom Zerone decorators will be added here in later batches:
//   - R1-3: ZeroneAccount decorator (frozen check, LastActiveBlock)
//   - R1-3: ZeroneCapability decorator (session key enforcement)
//   - R1-3: ZeroneDID decorator (DID resolution from memo)
//   - R6-1: EmergencyHalt decorator (block non-emergency txs when halted)
func NewAnteHandler(app *ZeroneApp) sdk.AnteHandler {
	return sdk.ChainAnteDecorators(
		// IBC redundant relay prevention (must be before gas meter init)
		ibcante.NewRedundantRelayDecorator(app.IBCKeeper),

		// Gas meter init (must be before any gas consumption)
		ante.NewSetUpContextDecorator(),

		// Standard Cosmos SDK decorators
		ante.NewExtensionOptionsDecorator(nil),
		ante.NewValidateBasicDecorator(),
		ante.NewTxTimeoutHeightDecorator(),
		ante.NewValidateMemoDecorator(app.AccountKeeper),
		ante.NewConsumeGasForTxSizeDecorator(app.AccountKeeper),
		ante.NewDeductFeeDecorator(app.AccountKeeper, app.BankKeeper, app.FeeGrantKeeper, nil),
		ante.NewSetPubKeyDecorator(app.AccountKeeper),
		ante.NewValidateSigCountDecorator(app.AccountKeeper),
		ante.NewSigGasConsumeDecorator(app.AccountKeeper, ante.DefaultSigVerificationGasConsumer),
		ante.NewSigVerificationDecorator(app.AccountKeeper, app.txConfig.SignModeHandler()),
		ante.NewIncrementSequenceDecorator(app.AccountKeeper),
	)
}
