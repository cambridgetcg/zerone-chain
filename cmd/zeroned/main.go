package main

import (
	"os"

	"cosmossdk.io/log"
	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"

	"github.com/zerone-chain/zerone/app"
	"github.com/zerone-chain/zerone/cmd/zeroned/cmd"
)

func main() {
	rootCmd := cmd.NewRootCmd()
	if err := svrcmd.Execute(rootCmd, "", app.DefaultNodeHome); err != nil {
		log.NewNopLogger().Error("failure when running app", "err", err)
		os.Exit(1)
	}
}
