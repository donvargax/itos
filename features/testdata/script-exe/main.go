// script-exe is the program the features' steps write, on windows, where a
// scenario needs a program that is a shell script: an extension, a release's
// itos, the config's shell. Windows runs no script by its #! line, so a step
// writes this program's bytes, then the marker line, then the script, to a
// file named <name>.exe. Run, it reads the script after the last marker in its
// own file, writes it to a temporary file and runs it with sh (Git for
// Windows'), its arguments, streams and environment handed on, and exits with
// the script's exit code. It lives under testdata so that no ./... builds it;
// the steps build it once a run (features/program_test.go).
package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// marker is the line between the program and its script, as the steps write
// it (scriptMarker in features/program_test.go).
const marker = "\n#itos-features-script\n"

func main() { os.Exit(run()) }

func run() int {
	self, err := os.Executable()
	if err != nil {
		return fail(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		return fail(err)
	}
	i := bytes.LastIndex(data, []byte(marker))
	if i < 0 {
		return fail(fmt.Errorf("%s holds no script", self))
	}
	tmp, err := os.CreateTemp("", "itos-features-script-")
	if err != nil {
		return fail(err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data[i+len(marker):]); err != nil {
		tmp.Close()
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		return fail(err)
	}
	// Slashes: sh reads a backslash as an escape.
	cmd := exec.Command("sh", append([]string{filepath.ToSlash(tmp.Name())}, os.Args[1:]...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err = cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	if err != nil {
		return fail(err)
	}
	return 0
}

func fail(err error) int {
	fmt.Fprintln(os.Stderr, "script-exe:", err)
	return 127
}
