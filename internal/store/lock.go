package store

import (
	"os"
	"path/filepath"
	"syscall"
)

const lockFileName = ".lock"

// lockFile acquires an exclusive file lock on .cctask/.lock.
// Returns the lock file which must be passed to unlockFile when done.
func lockFile(projectRoot string) (*os.File, error) {
	lockPath := filepath.Join(CctaskDir(projectRoot), lockFileName)
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// unlockFile releases the file lock and closes the file.
func unlockFile(f *os.File) {
	if f == nil {
		return
	}
	syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	f.Close()
}
