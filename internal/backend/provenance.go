package backend

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// hashFile computes the SHA256 hex digest of the file at path.
// Returns an empty string if the file cannot be read.
func hashFile(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return fmt.Sprintf("sha256:%x", h.Sum(nil))
}

// hfModelRevision returns the raw git revision hash of the "main" ref for a
// HuggingFace model in the local cache. Returns "" if the model is not cached.
//
// Cache layout:
//
//	$HF_HOME/hub/models--org--name/refs/main   → contains commit hash
//
// Falls back to ~/.cache/huggingface/hub/ if HF_HOME is unset.
func hfModelRevision(modelID string) string {
	cacheDir := hfCacheDir()
	if cacheDir == "" {
		return ""
	}
	sanitized := "models--" + strings.ReplaceAll(modelID, "/", "--")
	data, err := os.ReadFile(filepath.Join(cacheDir, sanitized, "refs", "main"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// huggingFaceRevision returns the revision hash prefixed with "hf:" for use as
// an opaque digest in ModelIdentity.
func huggingFaceRevision(modelID string) string {
	rev := hfModelRevision(modelID)
	if rev == "" {
		return ""
	}
	return "hf:" + rev
}

// huggingFaceContextLength reads max_position_embeddings from the cached
// config.json for a HuggingFace model. Returns 0 if the model is not cached
// or the field is missing.
func huggingFaceContextLength(modelID string) int {
	rev := hfModelRevision(modelID)
	if rev == "" {
		return 0
	}
	sanitized := "models--" + strings.ReplaceAll(modelID, "/", "--")
	path := filepath.Join(hfCacheDir(), sanitized, "snapshots", rev, "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var cfg struct {
		MaxPositionEmbeddings int `json:"max_position_embeddings"`
		TextConfig            struct {
			MaxPositionEmbeddings int `json:"max_position_embeddings"`
		} `json:"text_config"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return 0
	}
	if cfg.MaxPositionEmbeddings > 0 {
		return cfg.MaxPositionEmbeddings
	}
	// Multimodal models (e.g., Gemma-4, LLaVA) nest the text context under text_config.
	return cfg.TextConfig.MaxPositionEmbeddings
}

// hfCacheDir returns the HuggingFace hub cache directory.
func hfCacheDir() string {
	// HUGGINGFACE_HUB_CACHE takes priority (points directly to the hub dir).
	if dir := os.Getenv("HUGGINGFACE_HUB_CACHE"); dir != "" {
		return dir
	}
	// HF_HOME is the root; hub cache is underneath.
	if dir := os.Getenv("HF_HOME"); dir != "" {
		return filepath.Join(dir, "hub")
	}
	// Default location.
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cache", "huggingface", "hub")
}
