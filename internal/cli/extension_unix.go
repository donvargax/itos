//go:build unix

package cli

import "syscall"

// runProgram replaces this process by the extension's program, as the
// launcher replaces it by the version it runs, so the extension has the
// terminal, the signals and the exit code to itself, and writes to the
// process's own streams whatever o holds. It returns only when the program
// cannot start.
func runProgram(path string, args, env []string, _ Out) (int, error) {
	return 0, syscall.Exec(path, append([]string{path}, args...), env)
}
