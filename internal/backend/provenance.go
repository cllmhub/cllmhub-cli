package backend

import (
	"crypto/sha256"
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

// huggingFaceRevision looks up the git revision hash for a HuggingFace model
// in the local cache. The model ID should be in "org/name" format.
//
// Cache layout:
//
//	$HF_HOME/hub/models--org--name/refs/main   → contains commit hash
//
// Falls back to ~/.cache/huggingface/hub/ if HF_HOME is unset.
func huggingFaceRevision(modelID string) string {
	cacheDir := hfCacheDir()
	if cacheDir == "" {
		return ""
	}

	// Convert "org/name" to "models--org--name"
	sanitized := "models--" + strings.ReplaceAll(modelID, "/", "--")
	refPath := filepath.Join(cacheDir, sanitized, "refs", "main")

	data, err := os.ReadFile(refPath)
	if err != nil {
		return ""
	}

	rev := strings.TrimSpace(string(data))
	if rev == "" {
		return ""
	}
	return "hf:" + rev
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
