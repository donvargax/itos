//go:build !unix

package shim

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
)

// run starts the real git with git's arguments, the terminal's streams and
// the environment as it is, and gives its exit code, where a process cannot
// be replaced; an interrupt is git's to act on while it runs.
func run(real string, args []string) (int, error) {
	cmd := exec.Command(real, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	defer signal.Stop(interrupts)
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	return 0, err
}
