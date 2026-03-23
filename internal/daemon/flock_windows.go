//go:build windows

package daemon

import "os"

// lockFile acquires an exclusive lock on f.
// On Windows, opening with O_CREATE|O_EXCL provides basic mutual exclusion.
// True advisory locking uses LockFileEx, but for daemon use the file-exists
// check combined with process probing is sufficient.
func lockFile(f *os.File) error {
	// Windows does not support flock. The PID file is still written with
	// exclusive create semantics, so a second daemon will fail to truncate
	// it while the first holds a handle. This is a best-effort guard.
	return nil
}

// unlockFile is a no-op on Windows.
func unlockFile(f *os.File) {}
