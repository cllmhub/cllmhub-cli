package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/cllmhub/cllmhub-cli/internal/auth"
	"github.com/cllmhub/cllmhub-cli/internal/backend"
	"github.com/cllmhub/cllmhub-cli/internal/models"
	"github.com/cllmhub/cllmhub-cli/internal/provider"
)

// Bridge wraps a Provider to run inside the daemon.
type Bridge struct {
	model    string
	provider *provider.Provider
	cancel   context.CancelFunc
	done     chan struct{}
}

// BridgeManager manages all active bridges.
type BridgeManager struct {
	mu       sync.RWMutex
	bridges  map[string]*Bridge
	logger   *slog.Logger
}

// NewBridgeManager creates a new bridge manager.
func NewBridgeManager(logger *slog.Logger) *BridgeManager {
	return &BridgeManager{
		bridges: make(map[string]*Bridge),
		logger:  logger,
	}
}

// StartBridge creates and starts a bridge for a model.
func (bm *BridgeManager) StartBridge(model string, enginePort int, hubURL, token string, tokenMgr *auth.TokenManager) error {
	bm.mu.Lock()
	if _, exists := bm.bridges[model]; exists {
		bm.mu.Unlock()
		return fmt.Errorf("model %q is already published", model)
	}
	bm.mu.Unlock()

	// Resolve the engine model name from the GGUF filename
	// (llama-server router mode uses the filename stem as the model ID)
	engineModel := model
	if registry, err := models.LoadRegistry(); err == nil {
		if entry, ok := registry.Get(model); ok {
			engineModel = strings.TrimSuffix(entry.FileName, ".gguf")
		}
	}

	cfg := provider.Config{
		Model:         model,
		MaxConcurrent: 1,
		Token:         token,
		Backend: backend.Config{
			Type:  "llamacpp",
			URL:   fmt.Sprintf("http://127.0.0.1:%d", enginePort),
			Model: engineModel,
		},
		HubURL:       hubURL,
		TokenManager: tokenMgr,
		Logger:       bm.logger,
	}

	p, err := provider.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create provider for %q: %w", model, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	bridge := &Bridge{
		model:    model,
		provider: p,
		cancel:   cancel,
		done:     done,
	}

	bm.mu.Lock()
	bm.bridges[model] = bridge
	bm.mu.Unlock()

	// Run provider in background
	go func() {
		defer close(done)
		if err := p.Start(ctx); err != nil && ctx.Err() == nil {
			bm.logger.Error("bridge stopped with error", "model", model, "error", err)
		}
		bm.mu.Lock()
		delete(bm.bridges, model)
		bm.mu.Unlock()
	}()

	bm.logger.Info("bridge started", "model", model)
	return nil
}

// StopBridge stops the bridge for a model.
func (bm *BridgeManager) StopBridge(model string) error {
	bm.mu.RLock()
	bridge, exists := bm.bridges[model]
	bm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("model %q is not published", model)
	}

	bridge.provider.Stop()
	bridge.cancel()

	// Wait for the provider goroutine to exit, but don't block forever.
	// The provider may be stuck in a reconnect loop or a slow WebSocket close.
	select {
	case <-bridge.done:
	case <-time.After(5 * time.Second):
		bm.logger.Warn("bridge stop timed out, forcing cleanup", "model", model)
		bm.mu.Lock()
		delete(bm.bridges, model)
		bm.mu.Unlock()
	}

	bm.logger.Info("bridge stopped", "model", model)
	return nil
}

// StopAll stops all bridges.
func (bm *BridgeManager) StopAll() {
	bm.mu.RLock()
	bridges := make([]*Bridge, 0, len(bm.bridges))
	for _, b := range bm.bridges {
		bridges = append(bridges, b)
	}
	bm.mu.RUnlock()

	for _, b := range bridges {
		b.provider.Stop()
		b.cancel()

		select {
		case <-b.done:
		case <-time.After(5 * time.Second):
			bm.logger.Warn("bridge stop timed out during shutdown", "model", b.model)
		}
	}
}

// IsPublished checks if a model is currently published.
func (bm *BridgeManager) IsPublished(model string) bool {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	_, exists := bm.bridges[model]
	return exists
}

// PublishedModels returns the list of currently published model names.
func (bm *BridgeManager) PublishedModels() []string {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	models := make([]string, 0, len(bm.bridges))
	for name := range bm.bridges {
		models = append(models, name)
	}
	return models
}

// Count returns the number of active bridges.
func (bm *BridgeManager) Count() int {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	return len(bm.bridges)
}
