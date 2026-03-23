package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/cllmhub/cllmhub-cli/internal/models"
	"github.com/cllmhub/cllmhub-cli/internal/tui"
	"github.com/spf13/cobra"
)

var downloadCmd = &cobra.Command{
	Use:   "download <repo...>",
	Short: "Download one or more GGUF models from Hugging Face",
	Long: `Download GGUF model files from Hugging Face repositories.

Specify HF repo IDs. The CLI will list available GGUF files and let you pick
which quantization to download.

Requires a Hugging Face token — set one with 'cllmhub hf-token set <token>'.`,
	Example: `  cllmhub download TheBloke/Mistral-7B-v0.1-GGUF
  cllmhub download TheBloke/Mistral-7B-v0.1-GGUF TheBloke/Llama-2-7B-GGUF`,
	Args: cobra.MinimumNArgs(1),
	RunE: runDownload,
}

func runDownload(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	hfToken, err := models.LoadHFToken()
	if err != nil {
		return err
	}

	registry, err := models.LoadRegistry()
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	var failures int
	for _, repoID := range args {
		friendlyName := deriveFriendlyName(repoID)

		if entry, ok := registry.Get(friendlyName); ok && entry.State == "ready" {
			fmt.Printf("%-30s already downloaded\n", friendlyName)
			continue
		}

		// List all GGUF files in the repo
		fmt.Printf("Fetching %s...\n", repoID)
		files, err := models.ListRepoGGUFs(ctx, hfToken, repoID)
		if err != nil {
			fmt.Printf("%-30s error: %v\n", repoID, err)
			failures++
			continue
		}

		if len(files) == 0 {
			fmt.Printf("%-30s no GGUF files found\n", repoID)
			failures++
			continue
		}

		// Let user pick which file to download
		var chosen models.HFFile
		if len(files) == 1 {
			chosen = files[0]
		} else {
			labels := make([]string, len(files))
			for i, f := range files {
				labels[i] = fmt.Sprintf("%s (%s)", f.Filename, formatSize(f.Size))
			}
			idx := tui.Select(fmt.Sprintf("Select quantization for %s:", friendlyName), labels)
			if idx < 0 {
				fmt.Printf("%-30s skipped\n", friendlyName)
				continue
			}
			chosen = files[idx]
		}

		fmt.Printf("Downloading %s (%s)...", friendlyName, chosen.Filename)

		progressFn := func(downloaded, total int64) {
			if total > 0 {
				pct := float64(downloaded) / float64(total) * 100
				fmt.Printf("\rDownloading %s... %.1f%%", friendlyName, pct)
			}
		}

		var expectedSHA string
		if chosen.LFS != nil {
			expectedSHA = chosen.LFS.SHA256
		}

		if err := models.Download(ctx, hfToken, repoID, chosen.Filename, friendlyName, expectedSHA, progressFn); err != nil {
			fmt.Printf("\r%-30s error: %v\n", friendlyName, err)
			failures++
			continue
		}

		// Re-read entry for size and alias
		registry, _ = models.LoadRegistry()
		if entry, ok := registry.Get(friendlyName); ok {
			fmt.Printf("\r%-30s %.1f GB done (alias: %s)\n", friendlyName, float64(entry.SizeBytes)/(1024*1024*1024), entry.Alias)
		} else {
			fmt.Printf("\r%-30s done\n", friendlyName)
		}
	}

	if failures > 0 {
		return fmt.Errorf("%d of %d downloads failed", failures, len(args))
	}
	return nil
}

// deriveFriendlyName creates a short name from a HF repo ID.
// "TheBloke/Mistral-7B-v0.1-GGUF" -> "Mistral-7B-v0.1"
func deriveFriendlyName(repoID string) string {
	parts := strings.SplitN(repoID, "/", 2)
	name := repoID
	if len(parts) == 2 {
		name = parts[1]
	}
	name = strings.TrimSuffix(name, "-GGUF")
	name = strings.TrimSuffix(name, "-gguf")
	return name
}
