package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/cllmhub/cllmhub-cli/internal/daemon"
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

	hfToken := models.LoadHFTokenOptional()

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

	// Build a map of published models with their provider ID from the daemon.
	type publishedInfo struct {
		providerID string
	}
	published := make(map[string]publishedInfo) // model name -> info
	running, _ := daemon.IsRunning()
	if running {
		if client, err := daemon.NewClient(); err == nil {
			if status, err := client.Status(); err == nil {
				for _, m := range status.Models {
					published[m.Name] = publishedInfo{providerID: m.ProviderID}
				}
			}
		}
	}

	// Collect published external models (not in local registry).
	type externalModel struct {
		name       string
		providerID string
	}
	var externals []externalModel
	for name, info := range published {
		if _, ok := registry.Get(name); !ok {
			externals = append(externals, externalModel{name, info.providerID})
		}
	}

	if len(entries) == 0 && len(externals) == 0 {
		fmt.Println("No downloaded models")
		fmt.Println("Search for models: cllmhub models --search <query>")
		return nil
	}

	fmt.Printf("%-6s %-25s %-10s %-12s %-10s %s\n", "ALIAS", "NAME", "SIZE", "PROVIDER", "STATUS", "REPO")
	for _, e := range entries {
		sizeStr := formatSize(e.SizeBytes)
		providerID := "-"
		if info, ok := published[e.Name]; ok {
			providerID = info.providerID
		}
		fmt.Printf("%-6s %-25s %-10s %-12s %-10s %s\n", e.Alias, e.Name, sizeStr, providerID, e.State, e.RepoID)
	}
	for _, ext := range externals {
		fmt.Printf("%-6s %-25s %-10s %-12s %-10s %s\n", "-", ext.name, "-", ext.providerID, "published", "-")
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
