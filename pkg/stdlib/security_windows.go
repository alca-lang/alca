//go:build windows

package stdlib

import (
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var (
	modShell32Sec     = syscall.NewLazyDLL("shell32.dll")
	procIsUserAnAdmin = modShell32Sec.NewProc("IsUserAnAdmin")
	modKernel32Sec    = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx    = modKernel32Sec.NewProc("LockFileEx")
	procUnlockFileEx  = modKernel32Sec.NewProc("UnlockFileEx")
)

const (
	lockfileExclusiveLock   = 0x00000002
	lockfileFailImmediately = 0x00000001
)

func isPlatformPermissionError(err error) bool {
	if err == nil {
		return false
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case 5: // ERROR_ACCESS_DENIED
			return true
		case 1314: // ERROR_PRIVILEGE_NOT_HELD
			return true
		case 32: // ERROR_SHARING_VIOLATION
			return true
		case 33: // ERROR_LOCK_VIOLATION
			return true
		}
	}
	return false
}

// CurrentUserName returns the active Windows account username.
func CurrentUserName() string {
	u, err := user.Current()
	if err == nil && u.Username != "" {
		parts := strings.Split(u.Username, "\\")
		return parts[len(parts)-1]
	}
	if name := os.Getenv("USERNAME"); name != "" {
		return name
	}
	return "unknown"
}

// IsAdmin returns true if the current process is running with administrative privileges.
func IsAdmin() bool {
	if procIsUserAnAdmin == nil {
		return false
	}
	ret, _, _ := procIsUserAnAdmin.Call()
	return ret != 0
}

func checkExecutableAccess(path string, info os.FileInfo) (bool, error) {
	if info.IsDir() {
		return false, nil
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".exe", ".cmd", ".bat", ".com", ".ps1", ".vbs", ".al":
		return true, nil
	default:
		return false, nil
	}
}

// LockFileHandle applies a mandatory byte-range lock to the entire file on Windows.
func LockFileHandle(f *os.File, exclusive bool) error {
	if f == nil {
		return os.ErrInvalid
	}
	var flags uint32 = lockfileFailImmediately
	if exclusive {
		flags |= lockfileExclusiveLock
	}

	var ol syscall.Overlapped
	r1, _, err := procLockFileEx.Call(
		f.Fd(),
		uintptr(flags),
		0,
		0xFFFFFFFF,
		0xFFFFFFFF,
		uintptr(unsafe.Pointer(&ol)),
	)
	if r1 == 0 {
		return err
	}
	return nil
}

// UnlockFileHandle removes the file lock on Windows.
func UnlockFileHandle(f *os.File) error {
	if f == nil {
		return os.ErrInvalid
	}
	var ol syscall.Overlapped
	r1, _, err := procUnlockFileEx.Call(
		f.Fd(),
		0,
		0xFFFFFFFF,
		0xFFFFFFFF,
		uintptr(unsafe.Pointer(&ol)),
	)
	if r1 == 0 {
		return err
	}
	return nil
}
