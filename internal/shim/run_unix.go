//go:build unix

package shim

import (
	"os"
	"runtime/coverage"
	"syscall"
)

// run replaces this process by the real git with git's arguments and the
// environment as it is, so git has the terminal, the signals and the exit
// code to itself. It returns only when git cannot start. With GOCOVERDIR
// set, it first writes this process's coverage counters there, since an exec
// runs none of the exit hooks where a binary built with -cover writes them
// (T-124); a binary built without -cover refuses, and the error is ignored.
func run(real string, args []string) (int, error) {
	if dir := os.Getenv("GOCOVERDIR"); dir != "" {
		_ = coverage.WriteCountersDir(dir)
	}
	return 0, syscall.Exec(real, append([]string{real}, args...), os.Environ())
}
