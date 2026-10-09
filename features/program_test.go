// Programs that are shell scripts, on every platform. A scenario's fake
// programs (an extension, a release's itos, the config's shell) are shell
// scripts, which Linux and macOS run by their #! line and windows does not:
// there a program is found by its extension (PATHEXT) and must be one windows
// can start. So on windows a step writes <name>.exe instead, script-exe
// (testdata/script-exe) with the script after it, which writes the script with
// the program's arguments quoted into it and runs that with Git for Windows'
// sh, the one on the PATH or, failing that, the one beside git.exe; elsewhere
// it writes the script itself, whose #! line runs it with the arguments as they
// came. Either way the script's "$@" is what the program was run with, which
// the two tests at the end of this file hold it to.
package features

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/donvargax/itos/v7/internal/git"
)

// echoArgs writes each argument to a file of its own in the folder
// echoArgsDir names, and the number of them beside it: a line apiece could not
// carry an argument holding a newline, which is what git passes for a commit
// message. The tests below read the files back.
const echoArgs = `#!/bin/sh
i=0
for a in "$@"; do
  i=$((i + 1))
  printf '%s' "$a" > "$ITOS_FEATURES_ECHO_ARGS/arg$i"
done
printf '%s\n' "$i" > "$ITOS_FEATURES_ECHO_ARGS/count"
`

// echoArgsDir names the folder echoArgs writes its arguments to.
const echoArgsDir = "ITOS_FEATURES_ECHO_ARGS"

// scriptMarker is the line between script-exe and its script (marker in
// testdata/script-exe).
const scriptMarker = "\n#itos-features-script\n"

var (
	scriptExeOnce  sync.Once
	scriptExeBytes []byte
	scriptExeErr   error
)

// scriptExe is script-exe's bytes, built once a run from the module at root.
func scriptExe(root string) ([]byte, error) {
	scriptExeOnce.Do(func() {
		dir, err := os.MkdirTemp("", "itos-features-script-exe-")
		if err != nil {
			scriptExeErr = err
			return
		}
		defer os.RemoveAll(dir)
		out := filepath.Join(dir, "script-exe.exe")
		cmd := exec.Command("go", "build", "-o", out, "./features/testdata/script-exe")
		cmd.Dir = root
		if text, err := cmd.CombinedOutput(); err != nil {
			scriptExeErr = fmt.Errorf("building script-exe: %v\n%s", err, text)
			return
		}
		scriptExeBytes, scriptExeErr = os.ReadFile(out)
	})
	return scriptExeBytes, scriptExeErr
}

// program is a program that runs the script: the script itself, or on
// windows script-exe with the script after it.
func (w *world) program(script string) ([]byte, error) {
	if runtime.GOOS != "windows" {
		return []byte(script), nil
	}
	exe, err := scriptExe(w.root)
	if err != nil {
		return nil, err
	}
	return append(append(slices.Clone(exe), scriptMarker...), script...), nil
}

// programPath is the file a program at path is: path.exe on windows.
func programPath(path string) string {
	if runtime.GOOS == "windows" {
		return path + ".exe"
	}
	return path
}

// writeProgram writes, executable, a program at path (path.exe on windows)
// that runs the script.
func (w *world) writeProgram(path, script string) error {
	text, err := w.program(script)
	if err != nil {
		return err
	}
	return os.WriteFile(programPath(path), text, 0o755)
}

// runEchoArgs runs a program built from the echoArgs script with args, and
// gives back what its script was given, each argument read whole.
func runEchoArgs(t *testing.T, program string, env []string, args ...string) []string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command(programPath(program), args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Env = append(cmd.Env, echoArgsDir+"="+filepath.ToSlash(dir))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("running the program with %q: %v\n%s", args, err, out)
	}
	count, err := os.ReadFile(filepath.Join(dir, "count"))
	if err != nil {
		t.Fatalf("reading how many arguments the script was given: %v\n%s", err, out)
	}
	got := make([]string, 0, len(args))
	for i := 1; i <= len(args); i++ {
		arg, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("arg%d", i)))
		if err != nil {
			t.Fatalf("reading argument %d: %v\nthe script was given %s arguments\n%s", i, err, count, out)
		}
		got = append(got, string(arg))
	}
	if n := strings.TrimSpace(string(count)); n != fmt.Sprint(len(args)) {
		t.Fatalf("the script was given %s arguments, want %d", n, len(args))
	}
	return got
}

// A program's script is given the arguments the program was run with, as they
// came. The id counter asks git for refs/itos/fetched-ids^{commit}
// (internal/idcounter), and a windows program ran its script through sh, whose
// own reading of the command line it was started with ate the caret: the ref
// reached the script as ^commit, git resolved nothing, and @ID-IDS-07 failed
// on windows alone, linux and macos running the script by its #! line.
func TestProgramForwardsItsArguments(t *testing.T) {
	args := []string{
		"rev-parse", "--verify", "--quiet",
		"refs/itos/fetched-ids^{commit}",
		"two words", "", `back\slash`, "it's", "$HOME", "*", "a\nnewline", "last",
	}
	got := runEchoArgs(t, echoProgram(t), nil, args...)
	if !slices.Equal(got, args) {
		t.Errorf("the script was given %q, want %q", got, args)
	}
}

// A windows clone whose PATH holds no sh still runs a scenario's programs: sh
// is found beside the git the scripts run. The features' PATH is the caller's,
// and a caller's may hold only the folder git.exe is in, where sh is not.
func TestProgramFindsShBesideGit(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("elsewhere sh is on the PATH")
	}
	gitDir, sh := gitLayout(t)
	want := []string{"refs/itos/fetched-ids^{commit}", "two words"}
	got := runEchoArgs(t, echoProgram(t), []string{"PATH=" + gitDir, "ITOS_GIT="}, want...)
	if !slices.Equal(got, want) {
		t.Errorf("with only %s on the PATH, where sh is %s, the script was given %q, want %q", gitDir, sh, got, want)
	}
}

// gitLayout is the folder to put on a PATH that holds no sh, and the sh beside
// it: Git for Windows keeps git.exe in cmd and sh.exe in bin, so the folder
// holding git.exe is where a caller's PATH may reach and bin is not. A git
// laid out otherwise, or none found, skips the test that uses this.
func gitLayout(t *testing.T) (gitDir, sh string) {
	t.Helper()
	gitExe := git.Bin()
	if filepath.Base(gitExe) != "git.exe" {
		t.Skipf("no git.exe on this machine, only %s", gitExe)
	}
	root := filepath.Dir(filepath.Dir(gitExe))
	for _, at := range []string{filepath.Join(root, "cmd", "git.exe"), gitExe} {
		if _, err := os.Stat(at); err == nil {
			gitDir = filepath.Dir(at)
			break
		}
	}
	sh = filepath.Join(root, "bin", "sh.exe")
	if _, err := os.Stat(sh); err != nil {
		t.Skipf("no sh at %s: %v", sh, err)
	}
	if _, err := os.Stat(filepath.Join(gitDir, "sh.exe")); err == nil {
		t.Skipf("the folder holding git.exe, %s, holds sh too", gitDir)
	}
	return gitDir, sh
}

// echoProgram writes a program that writes its arguments one per line, and its
// path: what the two tests above run.
func echoProgram(t *testing.T) string {
	t.Helper()
	root, err := moduleRoot()
	if err != nil {
		t.Fatalf("module root: %v", err)
	}
	w := &world{root: root}
	program := filepath.Join(t.TempDir(), "echo-args")
	if err := w.writeProgram(program, echoArgs); err != nil {
		t.Fatalf("writing the program: %v", err)
	}
	return program
}
