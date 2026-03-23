package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	"github.com/cllmhub/cllmhub-cli/internal/auth"
	"github.com/cllmhub/cllmhub-cli/internal/backend"
	"github.com/cllmhub/cllmhub-cli/internal/daemon"
	"github.com/cllmhub/cllmhub-cli/internal/models"
	"github.com/cllmhub/cllmhub-cli/internal/provider"
	"github.com/cllmhub/cllmhub-cli/internal/tui"
	"github.com/spf13/cobra"
)

var (
	publishModel         string
	publishBackend       string
	publishBackendURL    string
	publishDescription   string
	publishMaxConcurrent int
	publishLogFile       string
	publishRateLimit     int
)

var publishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Publish a local LLM to the cLLMHub network",
	Long: `Publish models to the cLLMHub network.

When given model names as positional arguments, publishes downloaded GGUF models
via the daemon (engine + bridge). When using -m/-b flags, runs in foreground
mode connecting to an external inference backend.

Supported backends (foreground mode): ollama, llama.cpp, vllm, lmstudio, custom`,
	Example: `  # Publish downloaded models via daemon
  cllmhub publish llama3-8b mistral-7b

  # Publish a model using an external backend (foreground mode)
  cllmhub publish -m "llama3-70b" -b ollama

  # Publish a model using a different backend
  cllmhub publish -m "mixtral-8x7b" -b vllm`,
	RunE: runPublish,
}

func init() {
	publishCmd.Flags().StringVarP(&publishModel, "model", "m", "", "Model name to publish (required)")
	publishCmd.Flags().StringVarP(&publishBackend, "backend", "b", "ollama", "Backend type: ollama, llama.cpp, vllm, lmstudio, custom")
	publishCmd.Flags().StringVar(&publishBackendURL, "backend-url", "", "Backend endpoint URL (overrides default for the backend type)")
	publishCmd.Flags().StringVarP(&publishDescription, "description", "d", "", "Model description")
	publishCmd.Flags().IntVarP(&publishMaxConcurrent, "max-concurrent", "c", 1, "Maximum concurrent requests")
	publishCmd.Flags().StringVar(&publishLogFile, "log-file", "", "Path to audit log file (JSON lines)")
	publishCmd.Flags().IntVar(&publishRateLimit, "rate-limit", 0, "Max requests per minute (0 = unlimited)")

}

// publishableModel represents a model that can be published, from any source.
type publishableModel struct {
	name     string
	source   string // "gguf", "ollama", "vllm", "lmstudio"
	label    string // display label
}

func runPublish(cmd *cobra.Command, args []string) error {
	// If positional args are provided and match downloaded models, route through daemon
	if len(args) > 0 && !cmd.Flags().Changed("model") && !cmd.Flags().Changed("backend") {
		return publishViaDaemon(args)
	}

	if publishModel == "" {
		available := listAllPublishable()
		if len(available) == 0 {
			return fmt.Errorf("no models found\n  Download GGUF models: cllmhub download <repo>\n  Or start Ollama/vLLM/LM Studio")
		}

		labels := make([]string, len(available))
		for i, m := range available {
			labels[i] = m.label
		}

		for {
			idx := tui.Select("Select a model to publish:", labels)
			if idx < 0 {
				return fmt.Errorf("no model selected")
			}
			selected := available[idx]

			// Route GGUF models through daemon
			if selected.source == "gguf" {
				return publishViaDaemon([]string{selected.name})
			}

			// External backend model
			publishModel = selected.name
			if !cmd.Flags().Changed("backend") {
				publishBackend = selected.source
			}
			if !cmd.Flags().Changed("max-concurrent") {
				v := tui.InputInt(fmt.Sprintf("Max concurrent requests for %s:", publishModel), publishMaxConcurrent)
				if v < 0 {
					continue
				}
				if v > 0 {
					publishMaxConcurrent = v
				}
			}
			break
		}
		fmt.Println()
	}

	hubURL, err := auth.LoadHubURL()
	if err != nil {
		return fmt.Errorf("not authenticated: run 'cllmhub login' first")
	}

	token, tokenMgr, err := auth.ResolveTokenManager(hubURL)
	if err != nil {
		return err
	}

	if !regexp.MustCompile(`^[a-zA-Z0-9._:/-]+$`).MatchString(publishModel) {
		return fmt.Errorf("invalid model name %q: only alphanumerics, dots, underscores, colons, slashes, and hyphens are allowed", publishModel)
	}
	if len(publishDescription) > 500 {
		return fmt.Errorf("description too long (%d chars): maximum is 500", len(publishDescription))
	}

	fmt.Printf("Publishing model %q with backend %s\n", publishModel, publishBackend)
	fmt.Printf("  Hub:   %s\n", hubURL)
	if publishDescription != "" {
		fmt.Printf("  Description: %s\n", publishDescription)
	}
	fmt.Println()

	cfg := provider.Config{
		Model:         publishModel,
		Description:   publishDescription,
		MaxConcurrent: publishMaxConcurrent,
		Token:         token,
		Backend: backend.Config{
			Type:  publishBackend,
			URL:   publishBackendURL,
			Model: publishModel,
		},
		HubURL:       hubURL,
		LogFile:      publishLogFile,
		RateLimit:    publishRateLimit,
		TokenManager: tokenMgr,
	}

	p, err := provider.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize provider: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// SIGINT (Ctrl+C) = full shutdown.
	// SIGTERM (e.g. from system update) = close WebSocket so reconnect logic kicks in.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for sig := range sigChan {
			if sig == syscall.SIGINT {
				fmt.Println("\nShutting down provider...")
				p.Stop()
				cancel()
				return
			}
			// SIGTERM: close the WebSocket to trigger reconnect
			fmt.Println("\nReceived SIGTERM, closing connection for reconnect...")
			p.CloseConnection()
		}
	}()

	err = p.Start(ctx)
	if err != nil && err == context.Canceled {
		return nil // clean shutdown via signal
	}
	return err
}

// listAllPublishable returns all models that can be published:
// downloaded GGUF models + models from external backends (Ollama, vLLM, LM Studio).
func listAllPublishable() []publishableModel {
	var all []publishableModel

	// Downloaded GGUF models
	if registry, err := models.LoadRegistry(); err == nil {
		for _, entry := range registry.List() {
			if entry.State == "ready" {
				all = append(all, publishableModel{
					name:   entry.Name,
					source: "gguf",
					label:  fmt.Sprintf("%s (downloaded, %.1f GB)", entry.Name, float64(entry.SizeBytes)/(1024*1024*1024)),
				})
			}
		}
	}

	// External backend models
	for _, e := range listLocalModels() {
		all = append(all, publishableModel{
			name:   e.name,
			source: e.backend,
			label:  fmt.Sprintf("%s (%s)", e.name, e.backend),
		})
	}

	return all
}

// publishViaDaemon publishes models through the daemon (for downloaded GGUF models).
func publishViaDaemon(modelNames []string) error {
	// Verify models exist in registry
	registry, err := models.LoadRegistry()
	if err != nil {
		return fmt.Errorf("failed to load model registry: %w", err)
	}

	for i, name := range modelNames {
		resolved, ok := registry.ResolveAlias(name)
		if !ok {
			return fmt.Errorf("model %q not found — run 'cllmhub models' to see available models", name)
		}
		modelNames[i] = resolved

		entry, _ := registry.Get(resolved)
		if entry.State != "ready" {
			return fmt.Errorf("model %q is not ready (state: %s)", resolved, entry.State)
		}
	}

	// Ensure daemon is running
	running, _ := daemon.IsRunning()
	if !running {
		fmt.Println("Starting daemon...")
		// Start daemon by running the start command logic
		if err := runStart(nil, nil); err != nil {
			return fmt.Errorf("failed to start daemon: %w", err)
		}
	}

	client, err := daemon.NewClient()
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}

	for _, name := range modelNames {
		fmt.Printf("Publishing %s...\n", name)
	}

	resp, err := client.Publish(modelNames)
	if err != nil {
		return err
	}

	var failures int
	for _, r := range resp.Results {
		if r.Already {
			fmt.Printf("%-20s already published\n", r.Model)
		} else if r.Success {
			fmt.Printf("%-20s published\n", r.Model)
		} else {
			fmt.Printf("%-20s error: %s\n", r.Model, r.Error)
			failures++
		}
	}

	if failures > 0 {
		return fmt.Errorf("%d of %d models failed to publish", failures, len(modelNames))
	}
	return nil
}
