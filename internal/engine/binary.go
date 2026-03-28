package engine

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cllmhub/cllmhub-cli/internal/paths"
)

const (
	llamaCppRepo       = "ggml-org/llama.cpp"
	maxExtractFileSize = 2 * 1024 * 1024 * 1024 // 2GB per extracted file
)

// isSharedLib returns true if the filename looks like a shared library.
func isSharedLib(name string) bool {
	return strings.HasSuffix(name, ".dylib") ||
		strings.HasSuffix(name, ".so") ||
		strings.Contains(name, ".so.") ||
		strings.HasSuffix(name, ".dll")
}

// EnsureBinary checks if llama-server exists and returns its path.
// If not found, it downloads the latest release from llama.cpp.
func EnsureBinary(logger *slog.Logger) (string, error) {
	binDir, err := paths.BinDir()
	if err != nil {
		return "", err
	}

	binaryName := "llama-server"
	if runtime.GOOS == "windows" {
		binaryName = "llama-server.exe"
	}

	binPath := filepath.Join(binDir, binaryName)

	if _, err := os.Stat(binPath); err == nil {
		return binPath, nil
	}

	logger.Info("llama-server not found, downloading latest release...")
	if err := downloadLlamaServer(logger, binDir, binaryName); err != nil {
		return "", fmt.Errorf("failed to download llama-server: %w", err)
	}

	if _, err := os.Stat(binPath); err != nil {
		return "", fmt.Errorf("llama-server binary not found after download at %s", binPath)
	}

	return binPath, nil
}

// BinaryPath returns the expected path for the llama-server binary.
func BinaryPath() (string, error) {
	binDir, err := paths.BinDir()
	if err != nil {
		return "", err
	}

	binaryName := "llama-server"
	if runtime.GOOS == "windows" {
		binaryName = "llama-server.exe"
	}

	return filepath.Join(binDir, binaryName), nil
}

// assetPattern returns the release asset name pattern for the current platform.
func assetPattern() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		switch runtime.GOARCH {
		case "arm64":
			return "bin-macos-arm64.tar.gz", nil
		case "amd64":
			return "bin-macos-x64.tar.gz", nil
		}
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return "bin-ubuntu-x64.tar.gz", nil
		case "arm64":
			// No official arm64 Linux build from llama.cpp releases
			return "", fmt.Errorf("no pre-built llama-server available for linux/arm64; build from source: https://github.com/ggml-org/llama.cpp")
		}
	case "windows":
		switch runtime.GOARCH {
		case "amd64":
			return "bin-win-cpu-x64.zip", nil
		case "arm64":
			return "bin-win-cpu-arm64.zip", nil
		}
	}
	return "", fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
}

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func downloadLlamaServer(logger *slog.Logger, binDir, binaryName string) error {
	pattern, err := assetPattern()
	if err != nil {
		return err
	}

	// Fetch latest release metadata
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", llamaCppRepo)
	resp, err := http.Get(apiURL)
	if err != nil {
		return fmt.Errorf("failed to fetch release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("failed to parse release info: %w", err)
	}

	// Find matching asset
	var downloadURL string
	for _, asset := range release.Assets {
		if strings.HasSuffix(asset.Name, pattern) {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		return fmt.Errorf("no matching asset found for pattern *%s in release %s", pattern, release.TagName)
	}

	logger.Info("downloading llama-server", "release", release.TagName, "asset", filepath.Base(downloadURL))

	// Download the archive
	archiveResp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer archiveResp.Body.Close()

	if archiveResp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", archiveResp.StatusCode)
	}

	// Extract llama-server binary
	if strings.HasSuffix(pattern, ".tar.gz") {
		return extractFromTarGz(archiveResp.Body, binDir, binaryName)
	}
	return extractFromZip(archiveResp.Body, binDir, binaryName)
}

func extractFromTarGz(r io.Reader, binDir, binaryName string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip error: %w", err)
	}
	defer gz.Close()

	var foundBinary bool
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar error: %w", err)
		}

		base := filepath.Base(hdr.Name)
		extract := base == binaryName || isSharedLib(base)
		if !extract {
			continue
		}

		dst := filepath.Join(binDir, base)

		// Handle symlinks (e.g. libmtmd.0.dylib -> libmtmd.0.0.8472.dylib)
		if hdr.Typeflag == tar.TypeSymlink {
			// Validate symlink target stays within binDir
			target := hdr.Linkname
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(dst), target)
			}
			resolved, err := filepath.Abs(target)
			if err != nil || !strings.HasPrefix(resolved, binDir+string(filepath.Separator)) {
				return fmt.Errorf("symlink %s -> %s escapes extraction directory", base, hdr.Linkname)
			}
			os.Remove(dst)
			if err := os.Symlink(hdr.Linkname, dst); err != nil {
				return fmt.Errorf("symlink %s -> %s: %w", base, hdr.Linkname, err)
			}
			continue
		}

		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, io.LimitReader(tr, maxExtractFileSize)); err != nil {
			f.Close()
			return err
		}
		f.Close()

		if base == binaryName {
			foundBinary = true
		}
	}
	if !foundBinary {
		return fmt.Errorf("%s not found in archive", binaryName)
	}
	return nil
}

func extractFromZip(r io.Reader, binDir, binaryName string) error {
	// zip needs random access, so write to temp file first
	tmp, err := os.CreateTemp("", "llama-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	if _, err := io.Copy(tmp, r); err != nil {
		return fmt.Errorf("failed to save archive: %w", err)
	}

	stat, err := tmp.Stat()
	if err != nil {
		return err
	}

	zr, err := zip.NewReader(tmp, stat.Size())
	if err != nil {
		return fmt.Errorf("zip error: %w", err)
	}

	var foundBinary bool
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		base := filepath.Base(f.Name)
		extract := base == binaryName || isSharedLib(base)
		if !extract {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		dst := filepath.Join(binDir, base)
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, io.LimitReader(rc, maxExtractFileSize)); err != nil {
			out.Close()
			rc.Close()
			return err
		}
		out.Close()
		rc.Close()

		if base == binaryName {
			foundBinary = true
		}
	}
	if !foundBinary {
		return fmt.Errorf("%s not found in archive", binaryName)
	}
	return nil
}
