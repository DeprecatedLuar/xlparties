package internal

import (
	"fmt"
	"os"
	"syscall"
)

// acquireSingleInstanceLock takes an exclusive, non-blocking flock on a lock
// file next to the database, so only one xlparties process can run against
// a given DB_PATH at a time. Unlike a PID file, the lock is released
// automatically by the kernel when the process exits or crashes, so there
// is no stale-lock cleanup to get wrong.
func acquireSingleInstanceLock(dbPath string) (*os.File, error) {
	lockPath := dbPath + ".lock"
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("open lock file %s: %w", lockPath, err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("another xlparties instance is already running against %s", dbPath)
	}
	return f, nil
}
