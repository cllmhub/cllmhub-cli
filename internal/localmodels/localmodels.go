// Package localmodels discovers models served by locally-running inference
// backends (Ollama, vLLM, LM Studio, MLX) so the CLI can offer them for
// publishing without the user typing model names.
package localmodels

import (
	"context"
	"time"

	"github.com/cllmhub/cllmhub-cli/internal/backend"
)

// Entry is a model discovered on a local backend.
type Entry struct {
	Name    string
	Backend string
}

// List queries every supported local backend and returns the models found.
// Backends that are unreachable or error out are silently skipped.
func List() []Entry {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type probe struct {
		name    string
		newFunc func(backend.Config) (backend.Backend, error)
	}
	probes := []probe{
		{"ollama", func(c backend.Config) (backend.Backend, error) { return backend.NewOllama(c) }},
		{"vllm", func(c backend.Config) (backend.Backend, error) { return backend.NewVLLM(c) }},
		{"lmstudio", func(c backend.Config) (backend.Backend, error) { return backend.NewLMStudio(c) }},
		{"mlx", func(c backend.Config) (backend.Backend, error) { return backend.NewMLX(c) }},
	}

	var entries []Entry
	for _, p := range probes {
		b, err := p.newFunc(backend.Config{})
		if err != nil {
			continue
		}
		models, err := b.ListModels(ctx)
		if err != nil {
			continue
		}
		for _, m := range models {
			entries = append(entries, Entry{Name: m, Backend: p.name})
		}
	}

	return entries
}
