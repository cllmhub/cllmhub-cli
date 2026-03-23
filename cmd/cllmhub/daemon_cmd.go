package main

import (
	"fmt"
	"os"

	"github.com/cllmhub/cllmhub-cli/internal/daemon"
	"github.com/cllmhub/cllmhub-cli/internal/engine"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:    "__daemon",
	Hidden: true,
	Short:  "Run the daemon process (internal use only)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctxSize, _ := cmd.Flags().GetInt("ctx-size")
		flashAttn, _ := cmd.Flags().GetBool("flash-attn")
		slots, _ := cmd.Flags().GetInt("slots")
		nGPULayers, _ := cmd.Flags().GetInt("n-gpu-layers")
		batchSize, _ := cmd.Flags().GetInt("batch-size")

		cfg := engine.EngineConfig{
			CtxSize:    ctxSize,
			FlashAttn:  flashAttn,
			Slots:      slots,
			NGPULayers: nGPULayers,
			BatchSize:  batchSize,
		}

		d := daemon.New(cfg)
		if err := d.Run(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		return nil
	},
}

func init() {
	daemonCmd.Flags().Int("ctx-size", 0, "Context size")
	daemonCmd.Flags().Bool("flash-attn", false, "Enable flash attention")
	daemonCmd.Flags().Int("slots", 0, "Number of slots")
	daemonCmd.Flags().Int("n-gpu-layers", 0, "GPU layers")
	daemonCmd.Flags().Int("batch-size", 0, "Batch size")
}
