package engine

import (
	"log/slog"
	"os"
	"testing"
)

func TestNew(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cfg := EngineConfig{FlashAttn: true, NGPULayers: 999}

	e := New(logger, cfg)

	if e.State() != "stopped" {
		t.Errorf("State = %q, want %q", e.State(), "stopped")
	}
	if e.IsRunning() {
		t.Error("expected IsRunning=false for new engine")
	}
	if e.Port() != defaultPort {
		t.Errorf("Port = %d, want %d", e.Port(), defaultPort)
	}
}

func TestEngine_StopWhenStopped(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	e := New(logger, EngineConfig{})

	// Should not panic
	e.Stop()

	if e.State() != "stopped" {
		t.Errorf("State = %q, want %q", e.State(), "stopped")
	}
}

func TestEngine_StateTransitions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	e := New(logger, EngineConfig{})

	if e.State() != "stopped" {
		t.Errorf("initial State = %q, want stopped", e.State())
	}

	e.setState("starting")
	if e.State() != "starting" {
		t.Errorf("State = %q, want starting", e.State())
	}

	e.setState("running")
	if e.State() != "running" {
		t.Errorf("State = %q, want running", e.State())
	}
	if !e.IsRunning() {
		t.Error("expected IsRunning=true when state is running")
	}

	e.setState("failed")
	if e.State() != "failed" {
		t.Errorf("State = %q, want failed", e.State())
	}
	if e.IsRunning() {
		t.Error("expected IsRunning=false when state is failed")
	}
}
