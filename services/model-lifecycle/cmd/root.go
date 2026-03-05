package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/zerone-chain/zerone/services/model-lifecycle/internal/deployer"
	"github.com/zerone-chain/zerone/services/model-lifecycle/internal/rollback"
	"github.com/zerone-chain/zerone/services/model-lifecycle/internal/router"
)

var (
	flagModelsDir    string
	flagInferenceURL string
)

var rootCmd = &cobra.Command{
	Use:   "model",
	Short: "ZERONE Model Lifecycle — deploy, A/B test, and rollback models",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagModelsDir, "models-dir", "/models", "Models directory")
	rootCmd.PersistentFlags().StringVar(&flagInferenceURL, "inference-url", "http://localhost:8000", "Inference server URL")

	rootCmd.AddCommand(deployCmd, promoteCmd, rollbackCmd, listCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var deployCmd = &cobra.Command{
	Use:   "deploy --adapter <name>",
	Short: "Deploy a new adapter with optional canary",
	RunE: func(cmd *cobra.Command, args []string) error {
		adapter, _ := cmd.Flags().GetString("adapter")
		canary, _ := cmd.Flags().GetInt("canary")

		if adapter == "" {
			return fmt.Errorf("--adapter required")
		}

		d := deployer.New(deployer.Config{
			ModelsDir:    flagModelsDir,
			InferenceURL: flagInferenceURL,
		})

		return d.Deploy(context.Background(), adapter, canary)
	},
}

var promoteCmd = &cobra.Command{
	Use:   "promote --adapter <name>",
	Short: "Promote an adapter to 100% traffic",
	RunE: func(cmd *cobra.Command, args []string) error {
		adapter, _ := cmd.Flags().GetString("adapter")
		if adapter == "" {
			return fmt.Errorf("--adapter required")
		}

		d := deployer.New(deployer.Config{
			ModelsDir:    flagModelsDir,
			InferenceURL: flagInferenceURL,
		})

		return d.Promote(adapter)
	},
}

var rollbackCmd = &cobra.Command{
	Use:   "rollback --to <version>",
	Short: "Rollback to a previous adapter version",
	RunE: func(cmd *cobra.Command, args []string) error {
		target, _ := cmd.Flags().GetString("to")
		if target == "" {
			return fmt.Errorf("--to required")
		}

		mgr := rollback.NewManager(flagModelsDir, 5)
		if err := mgr.Rollback(target); err != nil {
			return err
		}

		d := deployer.New(deployer.Config{
			ModelsDir:    flagModelsDir,
			InferenceURL: flagInferenceURL,
		})
		return d.Deploy(context.Background(), target, 0)
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available and active adapters",
	RunE: func(cmd *cobra.Command, args []string) error {
		d := deployer.New(deployer.Config{ModelsDir: flagModelsDir})

		active, err := d.ActiveAdapter()
		if err != nil {
			active = "(none)"
		}

		adapters, err := d.ListAvailable()
		if err != nil {
			return err
		}

		fmt.Printf("Active adapter: %s\n\n", active)
		fmt.Println("Available adapters:")
		for _, a := range adapters {
			marker := "  "
			if a == active {
				marker = "* "
			}
			fmt.Printf("  %s%s\n", marker, a)
		}
		return nil
	},
}

func init() {
	deployCmd.Flags().String("adapter", "", "Adapter name to deploy")
	deployCmd.Flags().Int("canary", 0, "Initial canary traffic percentage")
	promoteCmd.Flags().String("adapter", "", "Adapter name to promote")
	rollbackCmd.Flags().String("to", "", "Version to rollback to")

	// Create a dummy router for package import (prevents unused import in future)
	_ = router.NewRouter("default")
	_ = time.Second
}
