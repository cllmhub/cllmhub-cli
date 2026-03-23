//go:build !windows

package daemon

import (
	"os"
	"syscall"
)

// lockFile acquires an exclusive non-blocking advisory lock on f.
func lockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

// unlockFile releases the advisory lock on f.
func unlockFile(f *os.File) {
	syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
