//go:build !windows

package corpus

import (
	"os/exec"
	"syscall"
)

// ownGroup starts the command as the leader of a process group of its own.
func ownGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killGroup kills the process group the process leads, every descendant still in it; one already
// gone is no error.
func killGroup(pid int) {
	_ = syscall.Kill(-pid, syscall.SIGKILL)
}
