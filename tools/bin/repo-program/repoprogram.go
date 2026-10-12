// Package repoprogram says how to start one of this repository's programs, a shell script:
// tools/bin/itos builds and runs the Go binary, tools/bin/dev-version asks git for this checkout's
// version, tools/bin/pinned fetches a pinned tool. Linux and macOS start one by its #! line, which
// the kernel reads; windows starts no script that way, so a spawn of one there fails without an
// exit code, and the gate that ran it fails without saying why (T-135). Command gives the program
// and the arguments to start one with: sh and the script first on windows, the script alone
// elsewhere, so a program under tools/ runs the repository's programs on every platform and
// nothing differs where it already works.
//
// Every such spawn goes through here: the programs under tools/bin and the self-tests'
// scratch.Program. A program that runs something else (go, git, gh) is started as itself on
// every platform and does not come here. It was tools/bin/repo-program.ts until T-146.
package repoprogram

import (
	"runtime"
	"strings"
)

// Command is the program and the arguments that start script with args on this platform.
func Command(script string, args ...string) (string, []string) {
	return commandOn(runtime.GOOS, script, args)
}

// commandOn is Command on the platform goos names.
func commandOn(goos, script string, args []string) (string, []string) {
	if goos != "windows" {
		return script, args
	}
	// Slashes: sh reads a backslash in the path as an escape.
	return "sh", append([]string{strings.ReplaceAll(script, `\`, "/")}, args...)
}
