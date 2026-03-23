package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/cllmhub/cllmhub-cli/internal/models"
	"github.com/spf13/cobra"
)

var modelsSearch string

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "List models",
	Long: `List downloaded models, or search Hugging Face for GGUF models.

Results only include text-generation models (no vision/multimodal).`,
	Example: `  cllmhub models
  cllmhub models --search mistral
  cllmhub models --search "llama 7b"`,
	RunE: runModels,
}

func init() {
	modelsCmd.Flags().StringVarP(&modelsSearch, "search", "s", "", "Search Hugging Face for GGUF models")
}

func runModels(cmd *cobra.Command, args []string) error {
	if modelsSearch != "" {
		return searchHFModels(modelsSearch)
	}
	return showDownloadedModels()
}

func searchHFModels(query string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	hfToken, err := models.LoadHFToken()
	if err != nil {
		return err
	}

	fmt.Printf("Searching for %q (text-only)...\n\n", query)

	results, err := models.SearchHFModels(ctx, hfToken, query)
	if err != nil {
		return err
	}

	if len(results) == 0 {
		fmt.Printf("No GGUF models with Q4_K_M found for %q\n", query)
		return nil
	}

	fmt.Printf("%-45s %s\n", "REPO", "DOWNLOADS")
	for _, m := range results {
		fmt.Printf("%-45s %d\n", m.ID, m.Downloads)
	}

	fmt.Println()
	fmt.Println("Download with: cllmhub download <repo>")

	return nil
}

func showDownloadedModels() error {
	registry, err := models.LoadRegistry()
	if err != nil {
		return err
	}

	entries := registry.List()
	if len(entries) == 0 {
		fmt.Println("No downloaded models")
		fmt.Println("Search for models: cllmhub models --search <query>")
		return nil
	}

	fmt.Printf("%-6s %-25s %-10s %-10s %s\n", "ALIAS", "NAME", "SIZE", "STATUS", "REPO")
	for _, e := range entries {
		sizeStr := formatSize(e.SizeBytes)
		fmt.Printf("%-6s %-25s %-10s %-10s %s\n", e.Alias, e.Name, sizeStr, e.State, e.RepoID)
	}

	return nil
}

func formatSize(bytes int64) string {
	if bytes == 0 {
		return "-"
	}
	gb := float64(bytes) / (1024 * 1024 * 1024)
	if gb >= 1 {
		return fmt.Sprintf("%.1f GB", gb)
	}
	mb := float64(bytes) / (1024 * 1024)
	return fmt.Sprintf("%.0f MB", mb)
}
