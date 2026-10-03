//go:build !unix

package launch

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// run starts the binary with the same arguments, streams and ITOS_VERSION
// set, and gives its exit code, where a process cannot be replaced.
func run(bin string, args []string, v string) (int, error) {
	cmd := exec.Command(bin, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = environ(v)
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0, nil
	case errors.As(err, &exit):
		return exit.ExitCode(), nil
	}
	return 0, fmt.Errorf("cannot run itos %s (%s): %w", v, bin, err)
}
