//go:build !windows

package engine

import (
	"os"
	"syscall"
)

// detachSysProcAttr starts the child in a new session (setsid) so it survives
// the parent CLI process exiting.
func detachSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}

// killPID sends SIGTERM so serve can shut Chrome down cleanly.
func killPID(pid int) error {
	if pid <= 0 {
		return nil
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Signal(syscall.SIGTERM)
}
