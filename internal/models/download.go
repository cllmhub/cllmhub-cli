package models

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cllmhub/cllmhub-cli/internal/paths"
)

// safePath joins dir and fileName, then verifies the result is inside dir.
// This prevents path traversal attacks via file names containing "../".
func safePath(dir, fileName string) (string, error) {
	joined := filepath.Join(dir, fileName)
	abs, err := filepath.Abs(joined)
	if err != nil {
		return "", fmt.Errorf("cannot resolve path: %w", err)
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("cannot resolve directory: %w", err)
	}
	if !strings.HasPrefix(abs, absDir+string(filepath.Separator)) && abs != absDir {
		return "", fmt.Errorf("invalid file name %q: path traversal detected", fileName)
	}
	return abs, nil
}

// ProgressFunc is called during download with bytes downloaded and total size.
type ProgressFunc func(downloaded, total int64)

// Download downloads a GGUF file from Hugging Face and stores it locally.
// repoID is like "TheBloke/Mistral-7B-v0.1-GGUF", fileName is the specific GGUF file.
// expectedSHA256 is the LFS hash from the HF API; if non-empty the download is verified against it.
func Download(ctx context.Context, hfToken, repoID, fileName, friendlyName, expectedSHA256 string, progressFn ProgressFunc) error {
	modelsDir, err := paths.ModelsDir()
	if err != nil {
		return err
	}

	destPath, err := safePath(modelsDir, fileName)
	if err != nil {
		return err
	}

	// Update registry as downloading
	registry, err := LoadRegistry()
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	entry := ModelEntry{
		Name:     friendlyName,
		FileName: fileName,
		RepoID:   repoID,
		State:    "downloading",
	}
	if err := registry.Add(entry); err != nil {
		return fmt.Errorf("failed to update registry: %w", err)
	}

	// Download the file from HF
	downloadURL := fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", repoID, fileName)
	size, hash, err := downloadFile(ctx, downloadURL, hfToken, destPath, progressFn)
	if err != nil {
		entry.State = "error"
		registry.Add(entry)
		return fmt.Errorf("download failed: %w", err)
	}

	// Verify SHA256 against the hash provided by the HF API
	if expectedSHA256 != "" && hash != expectedSHA256 {
		os.Remove(destPath)
		entry.State = "error"
		registry.Add(entry)
		return fmt.Errorf("SHA256 mismatch: expected %s, got %s", expectedSHA256, hash)
	}

	// Update registry as ready
	entry.SizeBytes = size
	entry.SHA256 = hash
	entry.DownloadedAt = time.Now()
	entry.State = "ready"
	if err := registry.Add(entry); err != nil {
		return fmt.Errorf("failed to update registry: %w", err)
	}

	return nil
}

func downloadFile(ctx context.Context, url, hfToken, destPath string, progressFn ProgressFunc) (int64, string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, "", err
	}
	if hfToken != "" {
		req.Header.Set("Authorization", "Bearer "+hfToken)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return 0, "", fmt.Errorf("access denied — check your HF token and model permissions")
	}
	if resp.StatusCode != http.StatusOK {
		return 0, "", fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	totalSize := resp.ContentLength

	f, err := os.Create(destPath)
	if err != nil {
		return 0, "", fmt.Errorf("cannot create file: %w", err)
	}
	defer f.Close()

	hasher := sha256.New()
	writer := io.MultiWriter(f, hasher)

	var downloaded int64
	buf := make([]byte, 32*1024)
	for {
		select {
		case <-ctx.Done():
			os.Remove(destPath)
			return 0, "", ctx.Err()
		default:
		}

		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := writer.Write(buf[:n]); writeErr != nil {
				os.Remove(destPath)
				return 0, "", writeErr
			}
			downloaded += int64(n)
			if progressFn != nil {
				progressFn(downloaded, totalSize)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			os.Remove(destPath)
			return 0, "", err
		}
	}

	hash := hex.EncodeToString(hasher.Sum(nil))
	return downloaded, hash, nil
}

// DeleteModel removes a downloaded model from disk and the registry.
func DeleteModel(name string) (int64, error) {
	registry, err := LoadRegistry()
	if err != nil {
		return 0, fmt.Errorf("failed to load registry: %w", err)
	}

	entry, ok := registry.Get(name)
	if !ok {
		return 0, fmt.Errorf("model %q not found — run 'cllmhub models' to see downloaded models", name)
	}

	modelsDir, err := paths.ModelsDir()
	if err != nil {
		return 0, err
	}

	filePath, err := safePath(modelsDir, entry.FileName)
	if err != nil {
		return 0, err
	}
	os.Remove(filePath)

	freed := entry.SizeBytes
	if err := registry.Remove(name); err != nil {
		return 0, fmt.Errorf("failed to update registry: %w", err)
	}

	return freed, nil
}
