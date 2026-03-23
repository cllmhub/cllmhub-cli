package daemon

import "github.com/cllmhub/cllmhub-cli/internal/paths"

// Re-export path functions for backward compatibility with daemon package consumers.
var (
	StateDir        = paths.StateDir
	PIDFile         = paths.PIDFile
	SocketPath      = paths.SocketPath
	LogDir          = paths.LogDir
	ModelsDir       = paths.ModelsDir
	BinDir          = paths.BinDir
	DaemonLogPath   = paths.DaemonLogPath
	DaemonTokenPath = paths.DaemonTokenPath
)
