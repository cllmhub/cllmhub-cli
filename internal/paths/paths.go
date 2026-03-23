package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

const dirName = ".cllmhub"

// StateDir returns the path to ~/.cllmhub, creating it if needed.
func StateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	dir := filepath.Join(home, dirName)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("cannot create state directory: %w", err)
	}
	return dir, nil
}

// PIDFile returns the path to the daemon PID file.
func PIDFile() (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "daemon.pid"), nil
}

// SocketPath returns the path to the daemon Unix socket.
func SocketPath() (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "cllmhub.sock"), nil
}

// LogDir returns the path to the logs directory, creating it if needed.
func LogDir() (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	logDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return "", fmt.Errorf("cannot create logs directory: %w", err)
	}
	return logDir, nil
}

// ModelsDir returns the path to the models directory, creating it if needed.
func ModelsDir() (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	modelsDir := filepath.Join(dir, "models")
	if err := os.MkdirAll(modelsDir, 0700); err != nil {
		return "", fmt.Errorf("cannot create models directory: %w", err)
	}
	return modelsDir, nil
}

// BinDir returns the path to the bin directory, creating it if needed.
func BinDir() (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0700); err != nil {
		return "", fmt.Errorf("cannot create bin directory: %w", err)
	}
	return binDir, nil
}

// DaemonLogPath returns the path to the daemon log file.
func DaemonLogPath() (string, error) {
	logDir, err := LogDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(logDir, "daemon.log"), nil
}

// DaemonTokenPath returns the path to the daemon auth token file.
func DaemonTokenPath() (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "daemon.token"), nil
}
