package main

import (
	"fmt"

	"github.com/cllmhub/cllmhub-cli/internal/daemon"
	"github.com/cllmhub/cllmhub-cli/internal/tui"
	"github.com/spf13/cobra"
)

var unpublishCmd = &cobra.Command{
	Use:   "unpublish [model...]",
	Short: "Stop serving one or more published models",
	RunE:  runUnpublish,
}

func runUnpublish(cmd *cobra.Command, args []string) error {
	running, _ := daemon.IsRunning()
	if !running {
		return fmt.Errorf("daemon is not running — no models are published")
	}

	client, err := daemon.NewClient()
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}

	// If no args, show interactive selection of published models.
	if len(args) == 0 {
		status, err := client.Status()
		if err != nil {
			return fmt.Errorf("failed to get daemon status: %w", err)
		}

		if len(status.Models) == 0 {
			return fmt.Errorf("no models are currently published")
		}

		// Single model — unpublish directly without listing.
		if len(status.Models) == 1 {
			args = []string{status.Models[0].Name}
		} else {
			labels := make([]string, len(status.Models))
			for i, m := range status.Models {
				labels[i] = fmt.Sprintf("%s (%s)", m.Name, m.Backend)
			}

			idx := tui.Select("Select a model to unpublish:", labels)
			if idx < 0 {
				return fmt.Errorf("no model selected")
			}
			args = []string{status.Models[idx].Name}
		}
	}

	for _, name := range args {
		fmt.Printf("Unpublishing %s...\n", name)
	}

	resp, err := client.Unpublish(args)
	if err != nil {
		return err
	}

	var failures int
	for _, r := range resp.Results {
		if r.Success {
			fmt.Printf("%-20s unpublished\n", r.Model)
		} else {
			fmt.Printf("%-20s error: %s\n", r.Model, r.Error)
			failures++
		}
	}

	if failures > 0 {
		return fmt.Errorf("%d of %d models failed to unpublish", failures, len(args))
	}
	return nil
}
