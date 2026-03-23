package models

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func setupTestHome(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
}

func TestLoadRegistry_EmptyDir(t *testing.T) {
	setupTestHome(t)

	r, err := LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if len(r.Entries) != 0 {
		t.Errorf("expected empty registry, got %d entries", len(r.Entries))
	}
}

func TestLoadRegistry_ExistingFile(t *testing.T) {
	setupTestHome(t)

	// Create a registry file manually
	home, _ := os.UserHomeDir()
	modelsDir := filepath.Join(home, ".cllmhub", "models")
	os.MkdirAll(modelsDir, 0700)

	entries := map[string]ModelEntry{
		"test-model": {
			Name:     "test-model",
			FileName: "test.gguf",
			RepoID:   "user/repo",
			State:    "ready",
		},
	}
	data, _ := json.Marshal(entries)
	os.WriteFile(filepath.Join(modelsDir, "registry.json"), data, 0600)

	r, err := LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if len(r.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(r.Entries))
	}
	if r.Entries["test-model"].FileName != "test.gguf" {
		t.Errorf("FileName = %q, want %q", r.Entries["test-model"].FileName, "test.gguf")
	}
}

func TestLoadRegistry_InvalidJSON(t *testing.T) {
	setupTestHome(t)

	home, _ := os.UserHomeDir()
	modelsDir := filepath.Join(home, ".cllmhub", "models")
	os.MkdirAll(modelsDir, 0700)
	os.WriteFile(filepath.Join(modelsDir, "registry.json"), []byte("{invalid"), 0600)

	_, err := LoadRegistry()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestRegistry_AddAndGet(t *testing.T) {
	setupTestHome(t)

	r, err := LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}

	entry := ModelEntry{
		Name:         "mistral-7b",
		FileName:     "mistral-7b-v0.1.Q4_K_M.gguf",
		RepoID:       "TheBloke/Mistral-7B-v0.1-GGUF",
		SizeBytes:    4_000_000_000,
		SHA256:       "abc123",
		DownloadedAt: time.Now().Truncate(time.Second),
		State:        "ready",
	}

	if err := r.Add(entry); err != nil {
		t.Fatalf("Add: %v", err)
	}

	got, ok := r.Get("mistral-7b")
	if !ok {
		t.Fatal("expected to find mistral-7b")
	}
	if got.FileName != entry.FileName {
		t.Errorf("FileName = %q, want %q", got.FileName, entry.FileName)
	}
	if got.SizeBytes != entry.SizeBytes {
		t.Errorf("SizeBytes = %d, want %d", got.SizeBytes, entry.SizeBytes)
	}
}

func TestRegistry_GetMissing(t *testing.T) {
	setupTestHome(t)

	r, _ := LoadRegistry()
	_, ok := r.Get("nonexistent")
	if ok {
		t.Error("expected Get to return false for nonexistent model")
	}
}

func TestRegistry_Has(t *testing.T) {
	setupTestHome(t)

	r, _ := LoadRegistry()
	r.Add(ModelEntry{Name: "test", FileName: "test.gguf", State: "ready"})

	if !r.Has("test") {
		t.Error("expected Has to return true")
	}
	if r.Has("missing") {
		t.Error("expected Has to return false for missing model")
	}
}

func TestRegistry_Remove(t *testing.T) {
	setupTestHome(t)

	r, _ := LoadRegistry()
	r.Add(ModelEntry{Name: "to-remove", FileName: "rm.gguf", State: "ready"})

	if err := r.Remove("to-remove"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if r.Has("to-remove") {
		t.Error("expected model to be removed")
	}
}

func TestRegistry_List(t *testing.T) {
	setupTestHome(t)

	r, _ := LoadRegistry()
	r.Add(ModelEntry{Name: "model-a", FileName: "a.gguf", State: "ready"})
	r.Add(ModelEntry{Name: "model-b", FileName: "b.gguf", State: "ready"})

	list := r.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(list))
	}

	names := map[string]bool{}
	for _, e := range list {
		names[e.Name] = true
	}
	if !names["model-a"] || !names["model-b"] {
		t.Errorf("unexpected entries: %v", list)
	}
}

func TestRegistry_AddOverwrites(t *testing.T) {
	setupTestHome(t)

	r, _ := LoadRegistry()
	r.Add(ModelEntry{Name: "m", FileName: "old.gguf", State: "downloading"})
	r.Add(ModelEntry{Name: "m", FileName: "new.gguf", State: "ready"})

	got, _ := r.Get("m")
	if got.FileName != "new.gguf" {
		t.Errorf("FileName = %q, want %q", got.FileName, "new.gguf")
	}
	if got.State != "ready" {
		t.Errorf("State = %q, want %q", got.State, "ready")
	}
}

func TestRegistry_SavePersists(t *testing.T) {
	setupTestHome(t)

	r, _ := LoadRegistry()
	r.Add(ModelEntry{Name: "persistent", FileName: "p.gguf", State: "ready"})

	// Load a fresh registry and verify
	r2, err := LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if !r2.Has("persistent") {
		t.Error("expected entry to persist after reload")
	}
}

func TestRegistry_ModelFilePath(t *testing.T) {
	setupTestHome(t)

	r, _ := LoadRegistry()
	r.Add(ModelEntry{Name: "fp-model", FileName: "model.gguf", State: "ready"})

	path, err := r.ModelFilePath("fp-model")
	if err != nil {
		t.Fatalf("ModelFilePath: %v", err)
	}
	if filepath.Base(path) != "model.gguf" {
		t.Errorf("expected path to end with model.gguf, got %q", path)
	}
}

func TestRegistry_ModelFilePath_NotFound(t *testing.T) {
	setupTestHome(t)

	r, _ := LoadRegistry()
	_, err := r.ModelFilePath("missing")
	if err == nil {
		t.Fatal("expected error for missing model")
	}
}

func TestRegistry_AliasAutoAssigned(t *testing.T) {
	setupTestHome(t)

	r, _ := LoadRegistry()
	r.Add(ModelEntry{Name: "first", FileName: "f.gguf", State: "ready"})
	r.Add(ModelEntry{Name: "second", FileName: "s.gguf", State: "ready"})

	e1, _ := r.Get("first")
	if e1.Alias != "m1" {
		t.Errorf("first alias = %q, want m1", e1.Alias)
	}

	e2, _ := r.Get("second")
	if e2.Alias != "m2" {
		t.Errorf("second alias = %q, want m2", e2.Alias)
	}
}

func TestRegistry_AliasPreservedOnUpdate(t *testing.T) {
	setupTestHome(t)

	r, _ := LoadRegistry()
	r.Add(ModelEntry{Name: "model", FileName: "old.gguf", State: "downloading"})

	e1, _ := r.Get("model")
	alias := e1.Alias

	r.Add(ModelEntry{Name: "model", FileName: "new.gguf", State: "ready"})
	e2, _ := r.Get("model")

	if e2.Alias != alias {
		t.Errorf("alias changed from %q to %q on update", alias, e2.Alias)
	}
}

func TestRegistry_AliasNotRecycled(t *testing.T) {
	setupTestHome(t)

	r, _ := LoadRegistry()
	r.Add(ModelEntry{Name: "a", FileName: "a.gguf", State: "ready"})
	r.Add(ModelEntry{Name: "b", FileName: "b.gguf", State: "ready"})
	r.Remove("a") // m1 removed
	r.Add(ModelEntry{Name: "c", FileName: "c.gguf", State: "ready"})

	e, _ := r.Get("c")
	if e.Alias != "m3" {
		t.Errorf("alias = %q, want m3 (should not recycle m1)", e.Alias)
	}
}

func TestRegistry_ResolveAlias_ByName(t *testing.T) {
	setupTestHome(t)

	r, _ := LoadRegistry()
	r.Add(ModelEntry{Name: "mistral-7b", FileName: "m.gguf", State: "ready"})

	resolved, ok := r.ResolveAlias("mistral-7b")
	if !ok {
		t.Fatal("expected to resolve by name")
	}
	if resolved != "mistral-7b" {
		t.Errorf("resolved = %q, want mistral-7b", resolved)
	}
}

func TestRegistry_ResolveAlias_ByAlias(t *testing.T) {
	setupTestHome(t)

	r, _ := LoadRegistry()
	r.Add(ModelEntry{Name: "mistral-7b", FileName: "m.gguf", State: "ready"})

	resolved, ok := r.ResolveAlias("m1")
	if !ok {
		t.Fatal("expected to resolve by alias")
	}
	if resolved != "mistral-7b" {
		t.Errorf("resolved = %q, want mistral-7b", resolved)
	}
}

func TestRegistry_ResolveAlias_NotFound(t *testing.T) {
	setupTestHome(t)

	r, _ := LoadRegistry()
	_, ok := r.ResolveAlias("m99")
	if ok {
		t.Error("expected false for nonexistent alias")
	}
}

func TestRegistry_LegacyMigration(t *testing.T) {
	setupTestHome(t)

	home, _ := os.UserHomeDir()
	modelsDir := filepath.Join(home, ".cllmhub", "models")
	os.MkdirAll(modelsDir, 0700)

	// Write legacy format (flat map, no aliases)
	legacy := map[string]ModelEntry{
		"model-a": {Name: "model-a", FileName: "a.gguf", State: "ready", DownloadedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		"model-b": {Name: "model-b", FileName: "b.gguf", State: "ready", DownloadedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)},
	}
	data, _ := json.Marshal(legacy)
	os.WriteFile(filepath.Join(modelsDir, "registry.json"), data, 0600)

	r, err := LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}

	ea, _ := r.Get("model-a")
	eb, _ := r.Get("model-b")

	// model-a downloaded first, should get m1
	if ea.Alias != "m1" {
		t.Errorf("model-a alias = %q, want m1", ea.Alias)
	}
	if eb.Alias != "m2" {
		t.Errorf("model-b alias = %q, want m2", eb.Alias)
	}

	// Verify new format persisted
	r2, _ := LoadRegistry()
	ea2, _ := r2.Get("model-a")
	if ea2.Alias != "m1" {
		t.Errorf("persisted alias = %q, want m1", ea2.Alias)
	}
}

func TestRegistry_AliasPersistsAcrossReloads(t *testing.T) {
	setupTestHome(t)

	r, _ := LoadRegistry()
	r.Add(ModelEntry{Name: "test", FileName: "t.gguf", State: "ready"})

	r2, _ := LoadRegistry()
	e, _ := r2.Get("test")
	if e.Alias != "m1" {
		t.Errorf("alias after reload = %q, want m1", e.Alias)
	}

	// Adding a new model should continue from m2
	r2.Add(ModelEntry{Name: "test2", FileName: "t2.gguf", State: "ready"})
	e2, _ := r2.Get("test2")
	if e2.Alias != "m2" {
		t.Errorf("alias = %q, want m2", e2.Alias)
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	setupTestHome(t)

	r, _ := LoadRegistry()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := "model-" + string(rune('a'+i))
			r.Add(ModelEntry{Name: name, FileName: name + ".gguf", State: "ready"})
			r.Has(name)
			r.Get(name)
			r.List()
		}(i)
	}
	wg.Wait()
}
