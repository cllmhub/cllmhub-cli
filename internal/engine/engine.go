package engine

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/cllmhub/cllmhub-cli/internal/paths"
)

const (
	defaultPort       = 18080
	healthInterval    = 30 * time.Second
	maxRestarts       = 3
	startupTimeout    = 60 * time.Second
	shutdownGrace     = 10 * time.Second
)

// Engine manages a single llama-server process in router mode.
type Engine struct {
	mu           sync.RWMutex
	cmd          *exec.Cmd
	port         int
	config       EngineConfig
	state        string // "stopped", "starting", "running", "failed"
	restartCount int
	logger       *slog.Logger
	ctx          context.Context
	cancel       context.CancelFunc
	logFile      *os.File
}

// New creates a new Engine instance.
func New(logger *slog.Logger, cfg EngineConfig) *Engine {
	return &Engine{
		port:   defaultPort,
		config: cfg,
		state:  "stopped",
		logger: logger,
	}
}

// Start launches llama-server in router mode.
func (e *Engine) Start() error {
	e.mu.Lock()
	if e.state == "running" || e.state == "starting" {
		e.mu.Unlock()
		return nil
	}
	e.state = "starting"
	e.mu.Unlock()

	// Ensure llama-server binary exists
	binPath, err := EnsureBinary(e.logger)
	if err != nil {
		e.setState("failed")
		return fmt.Errorf("llama-server not available: %w", err)
	}

	modelsDir, err := paths.ModelsDir()
	if err != nil {
		e.setState("failed")
		return err
	}

	// Open log file for engine output
	logDir, err := paths.LogDir()
	if err != nil {
		e.setState("failed")
		return err
	}
	logPath := filepath.Join(logDir, "engine.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		e.setState("failed")
		return fmt.Errorf("cannot open engine log: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	args := []string{
		"--models-dir", modelsDir,
		"--port", fmt.Sprintf("%d", e.port),
		"--host", "127.0.0.1",
	}
	args = append(args, e.config.ToArgs()...)

	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		cancel()
		logFile.Close()
		e.setState("failed")
		return fmt.Errorf("failed to start llama-server: %w", err)
	}

	e.mu.Lock()
	e.cmd = cmd
	e.ctx = ctx
	e.cancel = cancel
	e.logFile = logFile
	e.mu.Unlock()

	e.logger.Info("llama-server starting", "pid", cmd.Process.Pid, "port", e.port, "models_dir", modelsDir, "config", e.config.Summary())

	// Wait for health check
	if err := e.waitForHealth(); err != nil {
		e.Stop()
		return fmt.Errorf("llama-server failed to become healthy: %w", err)
	}

	e.setState("running")
	e.logger.Info("llama-server ready", "port", e.port)

	// Start health monitoring
	go e.healthLoop()

	// Monitor process exit
	go e.watchProcess()

	return nil
}

// Stop gracefully stops the llama-server process.
func (e *Engine) Stop() {
	e.mu.Lock()
	if e.state == "stopped" {
		e.mu.Unlock()
		return
	}
	cmd := e.cmd
	cancel := e.cancel
	logFile := e.logFile
	e.state = "stopped"
	e.restartCount = 0
	e.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	if cmd != nil && cmd.Process != nil {
		// Wait for process to exit with grace period
		done := make(chan struct{})
		go func() {
			cmd.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(shutdownGrace):
			cmd.Process.Kill()
			<-done
		}
	}

	if logFile != nil {
		logFile.Close()
	}

	e.logger.Info("llama-server stopped")
}

// IsRunning returns whether the engine is running.
func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state == "running"
}

// State returns the current engine state.
func (e *Engine) State() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state
}

// Port returns the engine port.
func (e *Engine) Port() int {
	return e.port
}

// Health checks if llama-server is responding.
func (e *Engine) Health() error {
	url := fmt.Sprintf("http://127.0.0.1:%d/health", e.port)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned %d", resp.StatusCode)
	}
	return nil
}

func (e *Engine) waitForHealth() error {
	deadline := time.Now().Add(startupTimeout)
	for time.Now().Before(deadline) {
		if err := e.Health(); err == nil {
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("timeout after %s", startupTimeout)
}

func (e *Engine) healthLoop() {
	ticker := time.NewTicker(healthInterval)
	defer ticker.Stop()

	for {
		e.mu.RLock()
		ctx := e.ctx
		state := e.state
		e.mu.RUnlock()

		if state != "running" {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := e.Health(); err != nil {
				e.logger.Warn("engine health check failed", "error", err)
			}
		}
	}
}

func (e *Engine) watchProcess() {
	e.mu.RLock()
	cmd := e.cmd
	ctx := e.ctx
	e.mu.RUnlock()

	if cmd == nil {
		return
	}

	err := cmd.Wait()

	// Check if we were intentionally stopped
	select {
	case <-ctx.Done():
		return
	default:
	}

	e.logger.Error("llama-server exited unexpectedly", "error", err)

	e.mu.Lock()
	e.restartCount++
	restarts := e.restartCount
	e.mu.Unlock()

	if restarts > maxRestarts {
		e.logger.Error("llama-server exceeded max restarts", "max", maxRestarts)
		e.setState("failed")
		return
	}

	e.logger.Info("restarting llama-server", "attempt", restarts, "max", maxRestarts)
	e.setState("stopped")
	if err := e.Start(); err != nil {
		e.logger.Error("failed to restart llama-server", "error", err)
	}
}

func (e *Engine) setState(state string) {
	e.mu.Lock()
	e.state = state
	e.mu.Unlock()
}
