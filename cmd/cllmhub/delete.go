package main

import (
	"fmt"

	"github.com/cllmhub/cllmhub-cli/internal/daemon"
	"github.com/cllmhub/cllmhub-cli/internal/models"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <model...>",
	Short: "Delete one or more downloaded models",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runDelete,
}

func runDelete(cmd *cobra.Command, args []string) error {
	// Resolve aliases to full model names.
	if registry, err := models.LoadRegistry(); err == nil {
		for i, name := range args {
			if resolved, ok := registry.ResolveAlias(name); ok {
				args[i] = resolved
			}
		}
	}

	// Check if any models are currently published
	if running, _ := daemon.IsRunning(); running {
		client, err := daemon.NewClient()
		if err == nil {
			if status, err := client.Status(); err == nil {
				published := make(map[string]bool)
				for _, m := range status.Models {
					published[m.Name] = true
				}
				for _, name := range args {
					if published[name] {
						fmt.Printf("Error: %s is currently published. Run 'cllmhub unpublish %s' first.\n", name, name)
						return fmt.Errorf("cannot delete published models")
					}
				}
			}
		}
	}

	var failures int
	var totalFreed int64

	for _, name := range args {
		fmt.Printf("Deleting %s...\n", name)
		freed, err := models.DeleteModel(name)
		if err != nil {
			fmt.Printf("%-20s error: %v\n", name, err)
			failures++
			continue
		}

		totalFreed += freed
		fmt.Printf("Deleted %-14s (%.1f GB freed)\n", name, float64(freed)/(1024*1024*1024))
	}

	if len(args) > 1 && failures < len(args) {
		fmt.Printf("Total: %.1f GB freed\n", float64(totalFreed)/(1024*1024*1024))
	}

	if failures > 0 {
		return fmt.Errorf("%d of %d deletes failed", failures, len(args))
	}
	return nil
}
