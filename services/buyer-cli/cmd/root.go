package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zerone-chain/zerone/services/buyer-cli/internal/client"
)

var (
	flagGateway   string
	flagAPIKey    string
	flagBuyerAddr string
)

var rootCmd = &cobra.Command{
	Use:   "zerone-data",
	Short: "ZERONE Dataset Marketplace CLI — browse, purchase, and download training datasets",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagGateway, "gateway", "http://localhost:8080", "API gateway URL")
	rootCmd.PersistentFlags().StringVar(&flagAPIKey, "api-key", "", "API key for authentication")
	rootCmd.PersistentFlags().StringVar(&flagBuyerAddr, "buyer", "", "Buyer wallet address")

	rootCmd.AddCommand(browseCmd, previewCmd, purchaseCmd, statusCmd, downloadCmd, rateCmd, pricingCmd)
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func newClient() *client.Client {
	return client.New(flagGateway, flagAPIKey)
}

var browseCmd = &cobra.Command{
	Use:   "browse",
	Short: "Browse available datasets",
	RunE: func(cmd *cobra.Command, args []string) error {
		domain, _ := cmd.Flags().GetString("domain")
		c := newClient()

		datasets, err := c.Browse(domain)
		if err != nil {
			return fmt.Errorf("browse: %w", err)
		}

		if len(datasets) == 0 {
			fmt.Println("No datasets found.")
			return nil
		}

		fmt.Printf("%-36s %-20s %-12s %10s %8s %s\n", "ID", "NAME", "DOMAIN", "SAMPLES", "RATING", "VERSION")
		fmt.Println(strings.Repeat("-", 100))
		for _, ds := range datasets {
			rating := "N/A"
			if ds.RatingCount > 0 {
				rating = fmt.Sprintf("%.1f/5", ds.AvgRating)
			}
			fmt.Printf("%-36s %-20s %-12s %10d %8s %s\n",
				ds.ID, truncate(ds.Name, 20), ds.Domain, ds.SampleCount, rating, ds.Version)
		}
		fmt.Printf("\n%d dataset(s) found.\n", len(datasets))
		return nil
	},
}

var previewCmd = &cobra.Command{
	Use:   "preview",
	Short: "Preview a dataset (free)",
	RunE: func(cmd *cobra.Command, args []string) error {
		dataset, _ := cmd.Flags().GetString("dataset")
		if dataset == "" {
			return fmt.Errorf("--dataset required")
		}

		c := newClient()
		preview, err := c.Preview(dataset)
		if err != nil {
			return fmt.Errorf("preview: %w", err)
		}

		fmt.Printf("Dataset: %s (%s)\n\n", preview.Name, preview.DatasetID)
		for _, s := range preview.Previews {
			fmt.Printf("  [%d] %s/%s: %s\n", s.Index, s.Domain, s.Category, s.Snippet)
		}
		fmt.Println("\nPurchase dataset for full access.")
		return nil
	},
}

var purchaseCmd = &cobra.Command{
	Use:   "purchase",
	Short: "Purchase a dataset",
	RunE: func(cmd *cobra.Command, args []string) error {
		dataset, _ := cmd.Flags().GetString("dataset")
		tier, _ := cmd.Flags().GetString("tier")

		if dataset == "" {
			return fmt.Errorf("--dataset required")
		}
		if flagBuyerAddr == "" {
			return fmt.Errorf("--buyer required")
		}

		// Map tier to amount
		amount := tierToAmount(tier)

		c := newClient()
		purchase, err := c.Purchase(dataset, flagBuyerAddr, amount)
		if err != nil {
			return fmt.Errorf("purchase: %w", err)
		}

		fmt.Printf("Purchase initiated!\n")
		fmt.Printf("  Purchase ID: %s\n", purchase.ID)
		fmt.Printf("  Dataset:     %s\n", purchase.DatasetID)
		fmt.Printf("  Tier:        %s\n", purchase.Tier)
		fmt.Printf("  Amount:      %d uzrn\n", purchase.AmountUZRN)
		fmt.Printf("  Status:      %s\n", purchase.Status)
		fmt.Printf("  Shares:      %d\n", purchase.ShamirShares)
		fmt.Printf("  Expires:     %s\n", purchase.ExpiresAt)

		if len(purchase.AccessTickets) > 0 {
			fmt.Printf("  Tickets:     %d access tickets issued\n", len(purchase.AccessTickets))
			fmt.Println("\nUse 'zerone-data download' to retrieve chunks.")
		}
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check purchase status",
	RunE: func(cmd *cobra.Command, args []string) error {
		dataset, _ := cmd.Flags().GetString("dataset")
		purchaseID, _ := cmd.Flags().GetString("purchase-id")

		if dataset == "" || purchaseID == "" {
			return fmt.Errorf("--dataset and --purchase-id required")
		}

		c := newClient()
		purchase, err := c.PurchaseStatus(dataset, purchaseID)
		if err != nil {
			return fmt.Errorf("status: %w", err)
		}

		data, _ := json.MarshalIndent(purchase, "", "  ")
		fmt.Println(string(data))
		return nil
	},
}

var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download purchased dataset chunks",
	RunE: func(cmd *cobra.Command, args []string) error {
		dataset, _ := cmd.Flags().GetString("dataset")
		purchaseID, _ := cmd.Flags().GetString("purchase-id")
		output, _ := cmd.Flags().GetString("output")

		if dataset == "" || purchaseID == "" {
			return fmt.Errorf("--dataset and --purchase-id required")
		}
		if output == "" {
			output = "./" + dataset
		}

		c := newClient()
		info, err := c.Download(dataset, purchaseID)
		if err != nil {
			return fmt.Errorf("download: %w", err)
		}

		fmt.Printf("Download info for %s:\n", info.DatasetID)
		fmt.Printf("  Chunks to download: %d / %d total\n", len(info.ChunkOrder), info.TotalChunks)
		fmt.Printf("  Access tickets:     %d\n", len(info.AccessTickets))
		fmt.Printf("  Watermark ID:       %s\n", info.WatermarkID)
		fmt.Printf("  Expires:            %s\n", info.ExpiresAt)
		fmt.Printf("  Output directory:   %s\n", output)

		// Create output directory
		if err := os.MkdirAll(output, 0o755); err != nil {
			return fmt.Errorf("create output dir: %w", err)
		}

		// Save download manifest
		manifest, _ := json.MarshalIndent(info, "", "  ")
		manifestPath := output + "/download-manifest.json"
		if err := os.WriteFile(manifestPath, manifest, 0o644); err != nil {
			return fmt.Errorf("save manifest: %w", err)
		}

		fmt.Printf("\nManifest saved to %s\n", manifestPath)
		fmt.Println("Use storage node endpoints with access tickets to retrieve chunks.")
		fmt.Println("After downloading, use 'zerone-data reconstruct' to rebuild the dataset.")
		return nil
	},
}

var rateCmd = &cobra.Command{
	Use:   "rate",
	Short: "Rate a purchased dataset",
	RunE: func(cmd *cobra.Command, args []string) error {
		dataset, _ := cmd.Flags().GetString("dataset")
		purchaseID, _ := cmd.Flags().GetString("purchase-id")
		starsStr, _ := cmd.Flags().GetString("stars")
		comment, _ := cmd.Flags().GetString("comment")
		weakAreasStr, _ := cmd.Flags().GetString("weak-areas")

		if dataset == "" || purchaseID == "" || starsStr == "" {
			return fmt.Errorf("--dataset, --purchase-id, and --stars required")
		}
		if flagBuyerAddr == "" {
			return fmt.Errorf("--buyer required")
		}

		stars, err := strconv.Atoi(starsStr)
		if err != nil || stars < 1 || stars > 5 {
			return fmt.Errorf("--stars must be 1-5")
		}

		var weakAreas []string
		if weakAreasStr != "" {
			weakAreas = strings.Split(weakAreasStr, ",")
		}

		c := newClient()
		if err := c.SubmitRating(dataset, purchaseID, flagBuyerAddr, stars, comment, weakAreas); err != nil {
			return fmt.Errorf("rate: %w", err)
		}

		fmt.Printf("Rating submitted: %d/5 for dataset %s\n", stars, dataset)
		return nil
	},
}

var pricingCmd = &cobra.Command{
	Use:   "pricing",
	Short: "Show pricing tiers",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := newClient()
		tiers, err := c.Pricing()
		if err != nil {
			return fmt.Errorf("pricing: %w", err)
		}

		fmt.Printf("%-12s %15s  %s\n", "TIER", "PRICE (uzrn)", "DESCRIPTION")
		fmt.Println(strings.Repeat("-", 60))
		for _, t := range tiers {
			price := "FREE"
			if t.PriceUZRN > 0 {
				price = fmt.Sprintf("%d", t.PriceUZRN)
			}
			fmt.Printf("%-12s %15s  %s\n", t.Tier, price, t.Description)
		}
		return nil
	},
}

func init() {
	browseCmd.Flags().String("domain", "", "Filter by domain")

	previewCmd.Flags().String("dataset", "", "Dataset ID to preview")

	purchaseCmd.Flags().String("dataset", "", "Dataset ID to purchase")
	purchaseCmd.Flags().String("tier", "standard", "Pricing tier (preview/slice/standard/premium/enterprise)")

	statusCmd.Flags().String("dataset", "", "Dataset ID")
	statusCmd.Flags().String("purchase-id", "", "Purchase ID")

	downloadCmd.Flags().String("dataset", "", "Dataset ID")
	downloadCmd.Flags().String("purchase-id", "", "Purchase ID")
	downloadCmd.Flags().String("output", "", "Output directory")

	rateCmd.Flags().String("dataset", "", "Dataset ID")
	rateCmd.Flags().String("purchase-id", "", "Purchase ID")
	rateCmd.Flags().String("stars", "", "Rating (1-5)")
	rateCmd.Flags().String("comment", "", "Optional comment")
	rateCmd.Flags().String("weak-areas", "", "Comma-separated weak areas")
}

func tierToAmount(tier string) int64 {
	switch tier {
	case "preview":
		return 0
	case "slice":
		return 1_000_000
	case "standard":
		return 10_000_000
	case "premium":
		return 50_000_000
	case "enterprise":
		return 100_000_000
	default:
		return 10_000_000 // default to standard
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
