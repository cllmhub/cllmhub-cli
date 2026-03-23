package main

import (
	"fmt"

	"github.com/cllmhub/cllmhub-cli/internal/models"
	"github.com/spf13/cobra"
)

var hfTokenCmd = &cobra.Command{
	Use:   "hf-token",
	Short: "Manage the Hugging Face API token",
	Long: `Manage the Hugging Face API token used to download models.

Get a token at https://huggingface.co/settings/tokens`,
}

var hfTokenSetCmd = &cobra.Command{
	Use:   "set <token>",
	Short: "Save a Hugging Face token",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := models.SaveHFToken(args[0]); err != nil {
			return fmt.Errorf("failed to save token: %w", err)
		}
		fmt.Println("HF token saved")
		return nil
	},
}

var hfTokenRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove the stored Hugging Face token",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := models.RemoveHFToken(); err != nil {
			return fmt.Errorf("failed to remove token: %w", err)
		}
		fmt.Println("HF token removed")
		return nil
	},
}

var hfTokenStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check if a Hugging Face token is configured",
	RunE: func(cmd *cobra.Command, args []string) error {
		if models.HasHFToken() {
			fmt.Println("HF token is configured")
		} else {
			fmt.Println("No HF token configured")
			fmt.Println("Get a token at https://huggingface.co/settings/tokens")
			fmt.Println("Then run: cllmhub hf-token set <token>")
		}
		return nil
	},
}

func init() {
	hfTokenCmd.AddCommand(hfTokenSetCmd)
	hfTokenCmd.AddCommand(hfTokenRemoveCmd)
	hfTokenCmd.AddCommand(hfTokenStatusCmd)
}
