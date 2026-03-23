package engine

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// MemoryInfo holds system memory information.
type MemoryInfo struct {
	TotalBytes     uint64
	AvailableBytes uint64
}

// CheckMemory returns available system memory.
func CheckMemory() (*MemoryInfo, error) {
	switch runtime.GOOS {
	case "darwin":
		return checkMemoryDarwin()
	case "linux":
		return checkMemoryLinux()
	default:
		return nil, fmt.Errorf("memory checking not supported on %s", runtime.GOOS)
	}
}

// CanFit checks if a model of the given size can fit in available memory.
// Uses a 500MB safety margin.
func CanFit(modelSizeBytes int64, currentUsageBytes int64) (bool, string) {
	mem, err := CheckMemory()
	if err != nil {
		// If we can't check, allow it
		return true, ""
	}

	const safetyMargin = 500 * 1024 * 1024 // 500MB
	needed := uint64(modelSizeBytes) + uint64(safetyMargin)
	available := mem.AvailableBytes

	if needed > available {
		return false, fmt.Sprintf(
			"not enough memory (needs ~%.1f GB, available: %.1f GB, in use by models: %.1f GB)",
			float64(modelSizeBytes)/(1024*1024*1024),
			float64(available)/(1024*1024*1024),
			float64(currentUsageBytes)/(1024*1024*1024),
		)
	}

	return true, ""
}

func checkMemoryDarwin() (*MemoryInfo, error) {
	// Get total memory
	out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return nil, fmt.Errorf("sysctl failed: %w", err)
	}
	total, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("cannot parse total memory: %w", err)
	}

	// Get page size and free/inactive pages from vm_stat
	out, err = exec.Command("vm_stat").Output()
	if err != nil {
		return nil, fmt.Errorf("vm_stat failed: %w", err)
	}

	lines := strings.Split(string(out), "\n")
	var pageSize uint64 = 4096
	var freePages, inactivePages uint64

	for _, line := range lines {
		if strings.HasPrefix(line, "Mach Virtual Memory Statistics") {
			// Parse page size from header: "...page size of 16384 bytes)"
			parts := strings.Fields(line)
			for i, p := range parts {
				if p == "size" && i+2 < len(parts) {
					if ps, err := strconv.ParseUint(parts[i+2], 10, 64); err == nil {
						pageSize = ps
					}
				}
			}
		}
		if strings.HasPrefix(line, "Pages free:") {
			freePages = parseVMStatValue(line)
		}
		if strings.HasPrefix(line, "Pages inactive:") {
			inactivePages = parseVMStatValue(line)
		}
	}

	available := (freePages + inactivePages) * pageSize

	return &MemoryInfo{
		TotalBytes:     total,
		AvailableBytes: available,
	}, nil
}

func checkMemoryLinux() (*MemoryInfo, error) {
	out, err := exec.Command("cat", "/proc/meminfo").Output()
	if err != nil {
		return nil, fmt.Errorf("cannot read /proc/meminfo: %w", err)
	}

	var total, available uint64
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			total = parseMeminfoKB(line) * 1024
		}
		if strings.HasPrefix(line, "MemAvailable:") {
			available = parseMeminfoKB(line) * 1024
		}
	}

	return &MemoryInfo{
		TotalBytes:     total,
		AvailableBytes: available,
	}, nil
}

func parseVMStatValue(line string) uint64 {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return 0
	}
	valStr := strings.TrimSuffix(parts[len(parts)-1], ".")
	val, _ := strconv.ParseUint(valStr, 10, 64)
	return val
}

func parseMeminfoKB(line string) uint64 {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return 0
	}
	val, _ := strconv.ParseUint(parts[1], 10, 64)
	return val
}
