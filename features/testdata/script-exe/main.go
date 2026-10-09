// script-exe is the program the features' steps write, on windows, where a
// scenario needs a program that is a shell script: an extension, a release's
// itos, the config's shell. Windows runs no script by its #! line, so a step
// writes this program's bytes, then the marker line, then the script, to a
// file named <name>.exe. Run, it reads the script after the last marker in its
// own file, writes it to a temporary file and runs it with sh (Git for
// Windows'), its arguments, streams and environment handed on, and exits with
// the script's exit code. It lives under testdata so that no ./... builds it;
// the steps build it once a run (features/program_test.go).
//
// The arguments go into the script rather than onto sh's command line, because
// sh reads that line its own way round and not the way it was written: MSYS
// eats a caret and reads a single quote as one, so an argument carrying git's
// refs/itos/fetched-ids^{commit}, which internal/idcounter asks git for,
// reached the script as ^commit, and a quote in any argument swallowed what
// followed it. Every windows scenario whose fake git forwards what itos passed
// it lost them, which is what made @ID-IDS-07 fail there and nowhere else.
// Quoted for sh and read as its own positional parameters, the script's "$@"
// is what the program was given, on every platform.
//
// The sh that runs it is the one on the PATH, else the one Git for Windows
// keeps beside its git (bin/sh.exe beside cmd/git.exe): the features' PATH is
// the caller's, and a windows caller's may hold only the folder git.exe is in,
// where no sh is, and every scenario writing a program failed with exit 127.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// marker is the line between the program and its script, as the steps write
// it (scriptMarker in features/program_test.go).
const marker = "\n#itos-features-script\n"

func main() { os.Exit(run()) }

func run() int {
	holdTree()
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
	if _, err := tmp.WriteString(withArgs(os.Args[1:], string(data[i+len(marker):]))); err != nil {
		tmp.Close()
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		return fail(err)
	}
	// Slashes: sh reads a backslash as an escape.
	cmd := exec.Command(shProgram(), filepath.ToSlash(tmp.Name()))
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

// withArgs is the script with the arguments the program was given above it, as
// one line of quoted words, so that "$@" in the script is those arguments. The
// #! line stays first, where a reader of the file looks for it.
func withArgs(args []string, script string) string {
	var b strings.Builder
	if after, found := strings.CutPrefix(script, "#!"); found {
		line, rest, _ := strings.Cut(after, "\n")
		fmt.Fprintf(&b, "#!%s\n", line)
		script = rest
	}
	b.WriteString("set --")
	for _, arg := range args {
		// Within single quotes every character is itself but the single
		// quote, which ends the quotes, stands for itself, and opens them.
		b.WriteString(` '`)
		b.WriteString(strings.ReplaceAll(arg, "'", `'\''`))
		b.WriteString("'")
	}
	b.WriteString("\n")
	b.WriteString(script)
	return b.String()
}

// shProgram is the sh to run a script with: the one on the PATH, else one
// beside a git the scripts run, which is how Git for Windows lays it out and
// what a PATH holding only the folder git.exe is in does not reach. "sh" when
// there is neither, so that the failure is the one sh itself gives.
func shProgram() string {
	if path, err := exec.LookPath("sh"); err == nil {
		return path
	}
	for _, git := range gits() {
		dir := filepath.Dir(git)
		for _, at := range []string{
			filepath.Join(dir, "sh.exe"),
			filepath.Join(dir, "..", "bin", "sh.exe"),
			filepath.Join(dir, "..", "usr", "bin", "sh.exe"),
		} {
			if info, err := os.Stat(filepath.Clean(at)); err == nil && info.Mode().IsRegular() {
				return at
			}
		}
	}
	return "sh"
}

// gits is the git ITOS_GIT names and the one on the PATH, once each: the two a
// script's own git is found beside.
func gits() []string {
	var found []string
	for _, name := range []string{os.Getenv("ITOS_GIT"), "git.exe", "git"} {
		path, err := exec.LookPath(name)
		if err != nil || slices.Contains(found, path) {
			continue
		}
		found = append(found, path)
	}
	return found
}

func fail(err error) int {
	fmt.Fprintln(os.Stderr, "script-exe:", err)
	return 127
}
