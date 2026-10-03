//go:build !unix

package cli

import (
	"errors"
	"os"
	"os/exec"
)

// runProgram starts the extension's program on o's streams and gives its exit
// code, where a process cannot be replaced; the error is why it could not
// start.
func runProgram(path string, args, env []string, o Out) (int, error) {
	cmd := exec.Command(path, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, o.Stdout, o.Stderr
	cmd.Env = env
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	return 0, err
}
