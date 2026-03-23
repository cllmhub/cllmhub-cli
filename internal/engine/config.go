package engine

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// EngineConfig holds tunable llama-server flags.
// Zero values mean "let llama-server decide" (dynamic/auto).
type EngineConfig struct {
	CtxSize    int  // --ctx-size (0 = auto, llama-server sizes based on available memory)
	FlashAttn  bool // --flash-attn
	Slots      int  // --slots (0 = auto, llama-server picks based on memory)
	NGPULayers int  // --n-gpu-layers (-1 = auto, 0 = CPU only)
	BatchSize  int  // --batch-size (0 = auto, llama-server default)
}

// HardwareProfile defines what we can reliably determine from hardware alone.
// Only sets GPU-related flags. Memory-dependent values (ctx-size, slots,
// batch-size) are left at 0 so llama-server can auto-size them based on
// the actual model being loaded and available memory at runtime.
type HardwareProfile struct {
	Name       string
	FlashAttn  bool
	NGPULayers int
}

var (
	profileAppleSilicon = HardwareProfile{Name: "apple-silicon", FlashAttn: true, NGPULayers: 999}
	profileNvidiaGPU    = HardwareProfile{Name: "nvidia-gpu",    FlashAttn: true, NGPULayers: 999}
	profileCPU          = HardwareProfile{Name: "cpu",           FlashAttn: false, NGPULayers: 0}
)

// DetectDefaults returns an EngineConfig based on the detected hardware.
// Only sets what we can reliably determine (GPU offload, flash attention).
// Memory-dependent settings are left at 0 for llama-server to auto-size.
func DetectDefaults() (EngineConfig, string) {
	switch {
	case runtime.GOOS == "darwin" && runtime.GOARCH == "arm64":
		return profileToConfig(profileAppleSilicon), profileAppleSilicon.Name
	case runtime.GOOS == "linux":
		if hasNvidiaGPU() {
			return profileToConfig(profileNvidiaGPU), profileNvidiaGPU.Name
		}
		return profileToConfig(profileCPU), profileCPU.Name
	default:
		return profileToConfig(profileCPU), profileCPU.Name
	}
}

func profileToConfig(p HardwareProfile) EngineConfig {
	return EngineConfig{
		FlashAttn:  p.FlashAttn,
		NGPULayers: p.NGPULayers,
	}
}

// --- Hardware detection helpers ---

func hasNvidiaGPU() bool {
	err := exec.Command("nvidia-smi").Run()
	return err == nil
}

// --- Config serialization ---

// ToArgs converts the config into llama-server command-line arguments.
// Zero-value fields are omitted, letting llama-server use its own defaults.
func (c EngineConfig) ToArgs() []string {
	var args []string

	if c.CtxSize > 0 {
		args = append(args, "--ctx-size", strconv.Itoa(c.CtxSize))
	}
	if c.FlashAttn {
		args = append(args, "--flash-attn", "on")
	}
	if c.Slots > 0 {
		args = append(args, "--slots", strconv.Itoa(c.Slots))
	}
	if c.NGPULayers != 0 {
		args = append(args, "--n-gpu-layers", strconv.Itoa(c.NGPULayers))
	}
	if c.BatchSize > 0 {
		args = append(args, "--batch-size", strconv.Itoa(c.BatchSize))
	}

	return args
}

// Summary returns a human-readable summary of the config.
// Shows "auto" for zero-value fields that llama-server will determine.
func (c EngineConfig) Summary() string {
	ctxSize := "auto"
	if c.CtxSize > 0 {
		ctxSize = strconv.Itoa(c.CtxSize)
	}
	slots := "auto"
	if c.Slots > 0 {
		slots = strconv.Itoa(c.Slots)
	}
	batchSize := "auto"
	if c.BatchSize > 0 {
		batchSize = strconv.Itoa(c.BatchSize)
	}

	parts := []string{
		fmt.Sprintf("ctx-size=%s", ctxSize),
		fmt.Sprintf("flash-attn=%v", c.FlashAttn),
		fmt.Sprintf("slots=%s", slots),
		fmt.Sprintf("n-gpu-layers=%d", c.NGPULayers),
		fmt.Sprintf("batch-size=%s", batchSize),
	}
	return strings.Join(parts, ", ")
}
