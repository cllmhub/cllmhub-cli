package models

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/cllmhub/cllmhub-cli/internal/paths"
)

// ModelEntry represents a downloaded model in the registry.
type ModelEntry struct {
	Name         string    `json:"name"`
	Alias        string    `json:"alias,omitempty"`
	FileName     string    `json:"file_name"`
	RepoID       string    `json:"repo_id,omitempty"` // HF repo e.g. "TheBloke/Mistral-7B-v0.1-GGUF"
	SizeBytes    int64     `json:"size_bytes"`
	SHA256       string    `json:"sha256,omitempty"`
	DownloadedAt time.Time `json:"downloaded_at"`
	State        string    `json:"state"` // "downloading", "ready", "error"
}

// registryFile is the on-disk JSON format.
type registryFile struct {
	Entries   map[string]ModelEntry `json:"entries"`
	NextAlias int                   `json:"next_alias"`
}

// Registry manages the model registry at ~/.cllmhub/models/registry.json.
type Registry struct {
	mu        sync.RWMutex
	path      string
	Entries   map[string]ModelEntry `json:"entries"`
	nextAlias int
}

// LoadRegistry loads or creates the model registry.
func LoadRegistry() (*Registry, error) {
	modelsDir, err := paths.ModelsDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(modelsDir, "registry.json")
	r := &Registry{
		path:    path,
		Entries: make(map[string]ModelEntry),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return r, nil
		}
		return nil, fmt.Errorf("cannot read registry: %w", err)
	}

	// Try new format first (wrapper with entries + next_alias).
	var rf registryFile
	if err := json.Unmarshal(data, &rf); err == nil && rf.Entries != nil {
		r.Entries = rf.Entries
		r.nextAlias = rf.NextAlias
		return r, nil
	}

	// Fall back to legacy format (flat map).
	var legacy map[string]ModelEntry
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, fmt.Errorf("invalid registry file: %w", err)
	}

	// Migrate: assign aliases sorted by download time for determinism.
	r.Entries = legacy
	sorted := make([]string, 0, len(legacy))
	for name := range legacy {
		sorted = append(sorted, name)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return legacy[sorted[i]].DownloadedAt.Before(legacy[sorted[j]].DownloadedAt)
	})
	for _, name := range sorted {
		r.nextAlias++
		entry := r.Entries[name]
		entry.Alias = fmt.Sprintf("m%d", r.nextAlias)
		r.Entries[name] = entry
	}

	// Persist the migration.
	if len(r.Entries) > 0 {
		r.save()
	}

	return r, nil
}

// Save writes the registry to disk.
func (r *Registry) Save() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.save()
}

// save writes the registry without acquiring a lock (caller must hold it).
func (r *Registry) save() error {
	rf := registryFile{
		Entries:   r.Entries,
		NextAlias: r.nextAlias,
	}
	data, err := json.MarshalIndent(rf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.path, data, 0600)
}

// Add adds or updates a model entry. New entries get an auto-assigned alias.
func (r *Registry) Add(entry ModelEntry) error {
	r.mu.Lock()
	if existing, ok := r.Entries[entry.Name]; ok {
		entry.Alias = existing.Alias
	} else {
		r.nextAlias++
		entry.Alias = fmt.Sprintf("m%d", r.nextAlias)
	}
	r.Entries[entry.Name] = entry
	r.mu.Unlock()
	return r.Save()
}

// Remove removes a model entry.
func (r *Registry) Remove(name string) error {
	r.mu.Lock()
	delete(r.Entries, name)
	r.mu.Unlock()
	return r.Save()
}

// Get returns a model entry by name.
func (r *Registry) Get(name string) (ModelEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.Entries[name]
	return entry, ok
}

// List returns all model entries.
func (r *Registry) List() []ModelEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entries := make([]ModelEntry, 0, len(r.Entries))
	for _, e := range r.Entries {
		entries = append(entries, e)
	}
	return entries
}

// Has checks if a model exists in the registry.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.Entries[name]
	return ok
}

// ResolveAlias resolves a name or alias to the full model name.
func (r *Registry) ResolveAlias(nameOrAlias string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, ok := r.Entries[nameOrAlias]; ok {
		return nameOrAlias, true
	}
	for _, e := range r.Entries {
		if e.Alias == nameOrAlias {
			return e.Name, true
		}
	}
	return "", false
}

// ModelFilePath returns the full path to a model's GGUF file.
func (r *Registry) ModelFilePath(name string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, ok := r.Entries[name]
	if !ok {
		return "", fmt.Errorf("model %q not found in registry", name)
	}

	modelsDir, err := paths.ModelsDir()
	if err != nil {
		return "", err
	}

	return safePath(modelsDir, entry.FileName)
}
