//go:build !windows

package stdlib

import (
	"errors"
	"os"
	"os/user"
	"syscall"
)

func isPlatformPermissionError(err error) bool {
	if err == nil {
		return false
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case syscall.EACCES, syscall.EPERM:
			return true
		}
	}
	return false
}

// CurrentUserName returns the active POSIX username.
func CurrentUserName() string {
	u, err := user.Current()
	if err == nil && u.Username != "" {
		return u.Username
	}
	if name := os.Getenv("USER"); name != "" {
		return name
	}
	return "unknown"
}

// IsAdmin returns true if the current process is running with root privileges (EUID == 0).
func IsAdmin() bool {
	return os.Geteuid() == 0
}

func checkExecutableAccess(path string, info os.FileInfo) (bool, error) {
	if info.IsDir() {
		return false, nil
	}
	// Check execute bits on Unix
	if info.Mode()&0111 != 0 {
		return true, nil
	}
	return false, nil
}

// LockFileHandle locks a file on Unix using flock.
func LockFileHandle(f *os.File, exclusive bool) error {
	if f == nil {
		return os.ErrInvalid
	}
	how := syscall.LOCK_SH | syscall.LOCK_NB
	if exclusive {
		how = syscall.LOCK_EX | syscall.LOCK_NB
	}
	return syscall.Flock(int(f.Fd()), how)
}

// UnlockFileHandle unlocks a file on Unix using flock.
func UnlockFileHandle(f *os.File) error {
	if f == nil {
		return os.ErrInvalid
	}
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
