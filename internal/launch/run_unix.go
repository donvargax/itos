//go:build unix

package launch

import (
	"fmt"
	"syscall"
)

// run replaces this process by the binary with the same arguments and
// ITOS_VERSION set, so the version run has the terminal, the signals and
// the exit code to itself. It returns only when the binary cannot start.
func run(bin string, args []string, v string) (int, error) {
	err := syscall.Exec(bin, append([]string{bin}, args...), environ(v))
	return 0, fmt.Errorf("cannot run itos %s (%s): %w", v, bin, err)
}
