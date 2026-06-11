//go:build windows

package engine

import (
	"os"
	"syscall"
)

// detachSysProcAttr starts the child in a new process group so it is not tied
// to the parent CLI's console.
func detachSysProcAttr() *syscall.SysProcAttr {
	const createNewProcessGroup = 0x00000200
	return &syscall.SysProcAttr{CreationFlags: createNewProcessGroup}
}

// killPID terminates the process (Windows has no SIGTERM).
func killPID(pid int) error {
	if pid <= 0 {
		return nil
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}
