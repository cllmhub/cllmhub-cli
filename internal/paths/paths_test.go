package paths

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestHome(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	return tmp
}

func TestStateDir(t *testing.T) {
	home := setupTestHome(t)

	dir, err := StateDir()
	if err != nil {
		t.Fatalf("StateDir: %v", err)
	}

	expected := filepath.Join(home, ".cllmhub")
	if dir != expected {
		t.Errorf("StateDir = %q, want %q", dir, expected)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}
	if perm := info.Mode().Perm(); perm != 0700 {
		t.Errorf("permissions = %o, want 0700", perm)
	}
}

func TestStateDir_Idempotent(t *testing.T) {
	setupTestHome(t)

	dir1, _ := StateDir()
	dir2, _ := StateDir()
	if dir1 != dir2 {
		t.Errorf("StateDir not idempotent: %q != %q", dir1, dir2)
	}
}

func TestPIDFile(t *testing.T) {
	setupTestHome(t)

	path, err := PIDFile()
	if err != nil {
		t.Fatalf("PIDFile: %v", err)
	}
	if !strings.HasSuffix(path, "daemon.pid") {
		t.Errorf("PIDFile = %q, want suffix daemon.pid", path)
	}
}

func TestSocketPath(t *testing.T) {
	setupTestHome(t)

	path, err := SocketPath()
	if err != nil {
		t.Fatalf("SocketPath: %v", err)
	}
	if !strings.HasSuffix(path, "cllmhub.sock") {
		t.Errorf("SocketPath = %q, want suffix cllmhub.sock", path)
	}
}

func TestLogDir(t *testing.T) {
	setupTestHome(t)

	dir, err := LogDir()
	if err != nil {
		t.Fatalf("LogDir: %v", err)
	}
	if !strings.HasSuffix(dir, "logs") {
		t.Errorf("LogDir = %q, want suffix logs", dir)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}
}

func TestModelsDir(t *testing.T) {
	setupTestHome(t)

	dir, err := ModelsDir()
	if err != nil {
		t.Fatalf("ModelsDir: %v", err)
	}
	if !strings.HasSuffix(dir, "models") {
		t.Errorf("ModelsDir = %q, want suffix models", dir)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}
}

func TestBinDir(t *testing.T) {
	setupTestHome(t)

	dir, err := BinDir()
	if err != nil {
		t.Fatalf("BinDir: %v", err)
	}
	if !strings.HasSuffix(dir, "bin") {
		t.Errorf("BinDir = %q, want suffix bin", dir)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}
}

func TestDaemonLogPath(t *testing.T) {
	setupTestHome(t)

	path, err := DaemonLogPath()
	if err != nil {
		t.Fatalf("DaemonLogPath: %v", err)
	}
	if !strings.HasSuffix(path, filepath.Join("logs", "daemon.log")) {
		t.Errorf("DaemonLogPath = %q, want suffix logs/daemon.log", path)
	}
}

func TestAllPathsUnderStateDir(t *testing.T) {
	setupTestHome(t)

	stateDir, _ := StateDir()

	paths := []func() (string, error){
		PIDFile,
		SocketPath,
		LogDir,
		ModelsDir,
		BinDir,
		DaemonLogPath,
	}

	for _, fn := range paths {
		p, err := fn()
		if err != nil {
			t.Fatalf("path function failed: %v", err)
		}
		if !strings.HasPrefix(p, stateDir) {
			t.Errorf("path %q is not under state dir %q", p, stateDir)
		}
	}
}
