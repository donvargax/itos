//go:build unix

package cli

import (
	"errors"
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
// refuses, and the error is ignored. The program stands in the folder itos
// runs in (o's Dir), which only the exec makes the process's: one that
// cannot start leaves the process where it stood.
func runProgram(path string, args, env []string, o Out) (int, error) {
	if dir := os.Getenv("GOCOVERDIR"); dir != "" {
		_ = coverage.WriteCountersDir(dir)
	}
	back, err := enter(o.Dir)
	if err != nil {
		return 0, err
	}
	err = syscall.Exec(path, append([]string{path}, args...), env)
	return 0, errors.Join(err, back())
}

// enter makes the folder dir the process's, "" leaving it as it is, and
// gives what puts the process back where it stood.
func enter(dir string) (back func() error, err error) {
	if dir == "" {
		return func() error { return nil }, nil
	}
	here, err := os.Open(".")
	if err != nil {
		return nil, err
	}
	there, err := os.Open(dir)
	if err == nil {
		err = errors.Join(there.Chdir(), there.Close())
	}
	if err != nil {
		return nil, errors.Join(err, here.Close())
	}
	return func() error { return errors.Join(here.Chdir(), here.Close()) }, nil
}
