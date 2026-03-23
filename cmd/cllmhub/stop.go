package main

import (
	"fmt"
	"os"
	"time"

	"github.com/cllmhub/cllmhub-cli/internal/daemon"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the cLLMHub daemon",
	RunE:  runStop,
}

func runStop(cmd *cobra.Command, args []string) error {
	running, pid := daemon.IsRunning()
	if !running {
		fmt.Println("Daemon is not running")
		return nil
	}

	client, err := daemon.NewClient()
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}

	fmt.Println("Stopping daemon...")

	if err := client.Stop(); err != nil {
		return fmt.Errorf("failed to stop daemon: %w", err)
	}

	// Wait for PID file to be removed (up to 10 seconds)
	pidPath, err := daemon.PIDFile()
	if err != nil {
		return err
	}

	for i := 0; i < 100; i++ {
		if _, err := os.Stat(pidPath); os.IsNotExist(err) {
			fmt.Printf("Daemon stopped (was PID: %d)\n", pid)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Printf("Daemon may still be shutting down (PID: %d)\n", pid)
	return nil
}
