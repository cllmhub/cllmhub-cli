package main

import (
	"fmt"
	"os"

	"github.com/cllmhub/cllmhub-cli/internal/versioncheck"
	"github.com/spf13/cobra"
)

var (
	hubURL       string
	useLocalhost bool
	Version      = "dev"
	verChecker   *versioncheck.Checker
)

var rootCmd = &cobra.Command{
	Use:     "cllmhub",
	Short:   "cLLMHub CLI - Turn your local LLM into a production API",
	Long:    `cLLMHub turns your local LLM into a production API.
Publish models, create tokens, and share access with anyone.`,
	Version: Version,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if useLocalhost {
			hubURL = "http://localhost:8080"
		}
		if cmd.Name() != "update" {
			verChecker = versioncheck.New(Version)
		}
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if verChecker == nil {
			return
		}
		if r := verChecker.Result(); r != nil && r.Available {
			fmt.Printf("\nA new version of cllmhub is available: %s (current: %s)\n", r.LatestVersion, r.CurrentVersion)
			fmt.Println("Run \"cllmhub update\" to upgrade.")
		}
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&hubURL, "hub-url", "https://cllmhub.com", "LLMHub gateway URL")
	rootCmd.PersistentFlags().MarkHidden("hub-url")
	rootCmd.PersistentFlags().BoolVarP(&useLocalhost, "local", "l", false, "Use localhost hub (http://localhost:8080)")
	rootCmd.PersistentFlags().MarkHidden("local")

	rootCmd.CompletionOptions.DisableDefaultCmd = true

	rootCmd.AddCommand(publishCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
	rootCmd.AddCommand(whoamiCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
