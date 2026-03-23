package engine

import (
	"strings"
	"testing"
)

func TestToArgs_Empty(t *testing.T) {
	cfg := EngineConfig{}
	args := cfg.ToArgs()
	if len(args) != 0 {
		t.Errorf("expected no args for zero config, got %v", args)
	}
}

func TestToArgs_AllFields(t *testing.T) {
	cfg := EngineConfig{
		CtxSize:    4096,
		FlashAttn:  true,
		Slots:      4,
		NGPULayers: 999,
		BatchSize:  512,
	}
	args := cfg.ToArgs()

	expected := map[string]string{
		"--ctx-size":     "4096",
		"--flash-attn":   "on",
		"--slots":        "4",
		"--n-gpu-layers": "999",
		"--batch-size":   "512",
	}

	for flag, val := range expected {
		found := false
		for i, a := range args {
			if a == flag && i+1 < len(args) && args[i+1] == val {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %s %s in args, got %v", flag, val, args)
		}
	}
}

func TestToArgs_PartialFields(t *testing.T) {
	cfg := EngineConfig{FlashAttn: true, NGPULayers: 32}
	args := cfg.ToArgs()

	// Should have flash-attn and n-gpu-layers, nothing else
	if len(args) != 4 {
		t.Errorf("expected 4 args, got %d: %v", len(args), args)
	}
}

func TestToArgs_NGPULayersZero(t *testing.T) {
	cfg := EngineConfig{NGPULayers: 0}
	args := cfg.ToArgs()

	for _, a := range args {
		if a == "--n-gpu-layers" {
			t.Error("n-gpu-layers=0 should be omitted")
		}
	}
}

func TestToArgs_NGPULayersNegative(t *testing.T) {
	cfg := EngineConfig{NGPULayers: -1}
	args := cfg.ToArgs()

	found := false
	for i, a := range args {
		if a == "--n-gpu-layers" && i+1 < len(args) && args[i+1] == "-1" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --n-gpu-layers -1, got %v", args)
	}
}

func TestSummary_Defaults(t *testing.T) {
	cfg := EngineConfig{}
	s := cfg.Summary()

	if !strings.Contains(s, "ctx-size=auto") {
		t.Errorf("expected ctx-size=auto in %q", s)
	}
	if !strings.Contains(s, "slots=auto") {
		t.Errorf("expected slots=auto in %q", s)
	}
	if !strings.Contains(s, "batch-size=auto") {
		t.Errorf("expected batch-size=auto in %q", s)
	}
	if !strings.Contains(s, "flash-attn=false") {
		t.Errorf("expected flash-attn=false in %q", s)
	}
	if !strings.Contains(s, "n-gpu-layers=0") {
		t.Errorf("expected n-gpu-layers=0 in %q", s)
	}
}

func TestSummary_WithValues(t *testing.T) {
	cfg := EngineConfig{
		CtxSize:    8192,
		FlashAttn:  true,
		Slots:      2,
		NGPULayers: 999,
		BatchSize:  256,
	}
	s := cfg.Summary()

	if !strings.Contains(s, "ctx-size=8192") {
		t.Errorf("expected ctx-size=8192 in %q", s)
	}
	if !strings.Contains(s, "flash-attn=true") {
		t.Errorf("expected flash-attn=true in %q", s)
	}
	if !strings.Contains(s, "slots=2") {
		t.Errorf("expected slots=2 in %q", s)
	}
	if !strings.Contains(s, "n-gpu-layers=999") {
		t.Errorf("expected n-gpu-layers=999 in %q", s)
	}
	if !strings.Contains(s, "batch-size=256") {
		t.Errorf("expected batch-size=256 in %q", s)
	}
}

func TestProfileToConfig_AppleSilicon(t *testing.T) {
	cfg := profileToConfig(profileAppleSilicon)
	if !cfg.FlashAttn {
		t.Error("expected FlashAttn=true for Apple Silicon")
	}
	if cfg.NGPULayers != 999 {
		t.Errorf("NGPULayers = %d, want 999", cfg.NGPULayers)
	}
	if cfg.CtxSize != 0 {
		t.Errorf("CtxSize = %d, want 0 (auto)", cfg.CtxSize)
	}
}

func TestProfileToConfig_CPU(t *testing.T) {
	cfg := profileToConfig(profileCPU)
	if cfg.FlashAttn {
		t.Error("expected FlashAttn=false for CPU")
	}
	if cfg.NGPULayers != 0 {
		t.Errorf("NGPULayers = %d, want 0", cfg.NGPULayers)
	}
}

func TestDetectDefaults_ReturnsValidConfig(t *testing.T) {
	cfg, name := DetectDefaults()
	if name == "" {
		t.Error("expected non-empty profile name")
	}
	// Just verify it returns a valid config without panicking
	_ = cfg.ToArgs()
	_ = cfg.Summary()
}
