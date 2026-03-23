package models

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cllmhub/cllmhub-cli/internal/paths"
)

const hfTokenFile = "hf-token"

// SaveHFToken stores the Hugging Face token to ~/.cllmhub/hf-token.
func SaveHFToken(token string) error {
	dir, err := paths.StateDir()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, hfTokenFile), []byte(strings.TrimSpace(token)), 0600)
}

// LoadHFToken reads the Hugging Face token from ~/.cllmhub/hf-token.
func LoadHFToken() (string, error) {
	dir, err := paths.StateDir()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(dir, hfTokenFile))
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no Hugging Face token configured — run 'cllmhub hf-token set <token>'")
		}
		return "", fmt.Errorf("cannot read HF token: %w", err)
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", fmt.Errorf("HF token file is empty — run 'cllmhub hf-token set <token>'")
	}
	return token, nil
}

// RemoveHFToken deletes the stored Hugging Face token.
func RemoveHFToken() error {
	dir, err := paths.StateDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, hfTokenFile)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot remove HF token: %w", err)
	}
	return nil
}

// HasHFToken checks if a Hugging Face token is saved.
func HasHFToken() bool {
	_, err := LoadHFToken()
	return err == nil
}
