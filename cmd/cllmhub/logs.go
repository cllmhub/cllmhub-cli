package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/cllmhub/cllmhub-cli/internal/daemon"
	"github.com/spf13/cobra"
)

var (
	logsFollow bool
	logsLines  int
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show daemon logs",
	RunE:  runLogs,
}

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow log output")
	logsCmd.Flags().IntVarP(&logsLines, "lines", "n", 50, "Number of lines to show")
}

func runLogs(cmd *cobra.Command, args []string) error {
	logPath, err := daemon.DaemonLogPath()
	if err != nil {
		return err
	}

	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no daemon logs found — run 'cllmhub start' first")
		}
		return err
	}
	defer f.Close()

	// Read last N lines
	lines, err := tailFile(f, logsLines)
	if err != nil {
		return err
	}

	for _, line := range lines {
		fmt.Println(line)
	}

	if !logsFollow {
		return nil
	}

	// Follow mode: poll for new content
	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			return err
		}
		fmt.Print(line)
	}
}

// tailFile reads the last n lines from a file using a ring buffer
// to avoid loading the entire file into memory.
func tailFile(f *os.File, n int) ([]string, error) {
	scanner := bufio.NewScanner(f)
	ring := make([]string, n)
	total := 0
	for scanner.Scan() {
		ring[total%n] = scanner.Text()
		total++
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if total == 0 {
		return nil, nil
	}
	if total <= n {
		return ring[:total], nil
	}

	// Unwrap the ring buffer into order
	start := total % n
	result := make([]string, n)
	copy(result, ring[start:])
	copy(result[n-start:], ring[:start])
	return result, nil
}
