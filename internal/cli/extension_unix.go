//go:build unix

package cli

import (
	"os"
	"runtime/coverage"
	"syscall"
)

// runProgram replaces this process by the extension's program, as the
// launcher replaces it by the version it runs, so the extension has the
// terminal, the signals and the exit code to itself, and writes to the
// process's own streams whatever o holds. It returns only when the program
// cannot start. With GOCOVERDIR set, it first writes this process's
// coverage counters there, since an exec runs none of the exit hooks where a
// binary built with -cover writes them (T-124); a binary built without -cover
// refuses, and the error is ignored.
func runProgram(path string, args, env []string, _ Out) (int, error) {
	if dir := os.Getenv("GOCOVERDIR"); dir != "" {
		_ = coverage.WriteCountersDir(dir)
	}
	return 0, syscall.Exec(path, append([]string{path}, args...), env)
}
