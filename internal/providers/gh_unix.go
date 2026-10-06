//go:build unix

package providers

import (
	"os/exec"
	"syscall"
)

// killTree starts the command in a process group of its own and has its
// context kill the whole group, so a gh given up on takes whatever it started
// with it, and leaves no process holding its output open.
func killTree(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}
