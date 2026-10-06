//go:build !unix

package providers

import "os/exec"

// killTree leaves the command as it is where there are no process groups:
// its context kills the command alone, and WaitDelay stops waiting for any
// process it started that holds its output open.
func killTree(*exec.Cmd) {}
