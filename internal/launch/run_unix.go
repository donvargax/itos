//go:build unix

package launch

import (
	"fmt"
	"os"
	"runtime/coverage"
	"syscall"
)

// run replaces this process by the binary with the same arguments and
// ITOS_VERSION set, so the version run has the terminal, the signals and
// the exit code to itself. It returns only when the binary cannot start.
// With GOCOVERDIR set, it first writes this process's coverage counters
// there, since an exec runs none of the exit hooks where a binary built with
// -cover writes them (T-124); a binary built without -cover refuses, and the
// error is ignored.
func run(bin string, args []string, v string) (int, error) {
	if dir := os.Getenv("GOCOVERDIR"); dir != "" {
		_ = coverage.WriteCountersDir(dir)
	}
	err := syscall.Exec(bin, append([]string{bin}, args...), environ(v))
	return 0, fmt.Errorf("cannot run itos %s (%s): %w", v, bin, err)
}
