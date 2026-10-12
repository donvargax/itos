//go:build windows

package corpus

import (
	"os/exec"
	"strconv"
	"syscall"
)

// ownGroup starts the command in a process group of its own.
func ownGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

// killGroup kills the process and its tree, as taskkill finds it; one already gone is no error.
func killGroup(pid int) {
	kill := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F")
	kill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = kill.Run()
}
