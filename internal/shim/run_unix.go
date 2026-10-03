//go:build unix

package shim

import (
	"os"
	"syscall"
)

// run replaces this process by the real git with git's arguments and the
// environment as it is, so git has the terminal, the signals and the exit
// code to itself. It returns only when git cannot start.
func run(real string, args []string) (int, error) {
	return 0, syscall.Exec(real, append([]string{real}, args...), os.Environ())
}
