// Command go-unit-tests runs the Go unit tests a change reaches (T-059), itos's unit tests since
// the TypeScript and its Vitest suite left (T-062). Both hooks call it: pre-commit with --cached
// (what is staged), pre-push through itos.yaml's hooks.pre_push with the remote commit the push
// builds on (that commit against the working tree), or --all when there is none.
//
//	go run ./tools/bin/go-unit-tests --cached | <base> | --all
//	go run ./tools/bin/go-unit-tests -hook-env <package>...
//
// A changed file belongs to the package whose folder holds it, the deepest one, so a package's
// testdata/ and embedded files count as the package. The run is those packages and every package
// whose imports, its tests' included, reach one of them, transitively: `go list -deps -test`
// gives each package's Deps, in well under a second and without building anything. go.mod and
// go.sum, which every package builds with, run them all. Results stay in Go's test cache, so a
// package whose inputs did not change answers at once.
//
// The packages are ./cmd/... and ./internal/..., CI's step: features/ is not a unit suite but the
// named tests, run by CI and the nightly.
//
// What git hands a hook stops here, once the change is read from it: the index the commit is made
// from (GIT_INDEX_FILE, an index.lock under git commit -a), in a linked worktree GIT_DIR, and the
// rest a git command or itos may set. A test's git inherits them through go test, so a scratch
// repository's commands would read and write this repository instead, its index and its hooks
// included (p1-hook-env-unset-tracked; tools/selftest/hookenv, T-142).
//
// -hook-env is the nightly's (T-326, ADR-0051's tiers): it runs go test over the packages it is
// given with the git environment a hook hands, in the worst case a test can meet it (hookenv.go),
// so a test that writes through GIT_DIR or GIT_INDEX_FILE, or whose own repository inherits a
// failing hook, is red every time and not when a reviewer happens to think of it. The hooks never
// pass it; they drop the variables above.
//
// It was a shell script until T-326, which needed a Go test package beside it; the selection is
// the script's, folders compared as slash paths so a native Windows run selects too.
//
// Exit status: 0 the tests passed or none was reached, 1 a test failed (with -hook-env, or the
// scratch repository changed), 2 a usage error or a
// command this could not run.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"

	"github.com/donvargax/itos/v8/internal/git"
)

// patterns are the packages a run covers: CI's unit step.
var patterns = []string{"./cmd/...", "./internal/..."}

// handed are the variables git, itos or a hook set for the commands they start: the ones that
// point git at a repository, its index or its objects, or add to its configuration.
var handed = []string{
	"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_PREFIX", "GIT_COMMON_DIR",
	"GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES",
	"GIT_CONFIG_COUNT", "GIT_CONFIG_KEY_0", "GIT_CONFIG_VALUE_0", "GIT_CONFIG_PARAMETERS", "ITOS_GIT",
}

const usage = "usage: go run ./tools/bin/go-unit-tests --cached | <base> | --all | -hook-env <package>..."

func main() {
	os.Exit(run(os.Args[1:], ".", os.Stdout, os.Stderr))
}

// run is the program in dir, which every command it starts runs in.
func run(args []string, dir string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "-hook-env" {
		return hookEnv(args[1:], dir, stdout, stderr)
	}
	if len(args) != 1 || args[0] == "" || (strings.HasPrefix(args[0], "-") && args[0] != "--all" && args[0] != "--cached") {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	realGit := git.Bin(runtime.GOOS)
	var changed []string
	switch args[0] {
	case "--all":
		changed = []string{"go.mod"}
	case "--cached":
		out, err := capture(dir, os.Environ(), realGit, "diff", "--cached", "--name-only", "--no-renames", "-z")
		if err != nil {
			fmt.Fprintf(stderr, "go-unit-tests: %v\n", err)
			return 2
		}
		changed = splitNUL(out)
	default:
		out, err := capture(dir, os.Environ(), realGit, "diff", "--name-only", "--no-renames", "-z", args[0])
		if err != nil {
			fmt.Fprintf(stderr, "go-unit-tests: %v\n", err)
			return 2
		}
		changed = splitNUL(out)
	}
	root, err := capture(dir, os.Environ(), realGit, "rev-parse", "--show-toplevel")
	if err != nil {
		fmt.Fprintf(stderr, "go-unit-tests: %v\n", err)
		return 2
	}
	root = strings.TrimSpace(root)

	// Everything from here on starts its commands without what the hook handed.
	env := without(os.Environ(), handed)
	if !anyGoBuilt(changed) {
		return 0
	}
	if slices.Contains(changed, "go.mod") || slices.Contains(changed, "go.sum") {
		return goTest(dir, env, patterns, stdout, stderr)
	}
	listing, err := capture(dir, env, "go", append([]string{"list", "-e", "-test", "-f", "{{.ImportPath}}|{{.Dir}}|{{join .Deps \" \"}}"}, patterns...)...)
	if err != nil {
		fmt.Fprintf(stderr, "go-unit-tests: %v\n", err)
		return 2
	}
	packages := affected(changed, root, listing)
	if len(packages) == 0 {
		return 0
	}
	return goTest(dir, env, packages, stdout, stderr)
}

// goTest runs go test over packages, echoing the command first as a shell trace does.
func goTest(dir string, env, packages []string, stdout, stderr io.Writer) int {
	fmt.Fprintln(stdout, "$ go test "+strings.Join(packages, " "))
	cmd := exec.Command("go", append([]string{"test"}, packages...)...)
	cmd.Dir, cmd.Env, cmd.Stdout, cmd.Stderr = dir, env, stdout, stderr
	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return 1
		}
		fmt.Fprintf(stderr, "go-unit-tests: %v\n", err)
		return 2
	}
	return 0
}

// anyGoBuilt reports whether a changed file is something a Go package is built or tested from:
// a file under cmd/ or internal/, go.mod or go.sum.
func anyGoBuilt(changed []string) bool {
	return slices.ContainsFunc(changed, func(f string) bool {
		return strings.HasPrefix(f, "cmd/") || strings.HasPrefix(f, "internal/") || f == "go.mod" || f == "go.sum"
	})
}

// affected is the import paths of the packages the changed files reach, sorted: the package whose
// folder is the deepest one holding a file, and every package whose Deps, its test variants'
// included, hold one of those. listing is `go list -e -test` with one line for each package and
// each test variant, import path, folder and Deps joined by |: a test variant's path is `p
// [p.test]` and its test main's `p.test`, both p's tests; a Dep recompiled for a test reads `q
// [p.test]`, which the space splits into q and a word in brackets, dropped. A package only a test
// variant names, p's external tests `p_test [p.test]`, is not one go test can be given: it counts
// for the reach, never the result. Folders are compared as slash paths, as git names a file with
// slashes and go list a folder natively.
func affected(changed []string, root, listing string) []string {
	root = slash(root) + "/"
	folder := map[string]string{}
	deps := map[string][]string{}
	listed := map[string]bool{}
	for _, line := range strings.Split(listing, "\n") {
		fields := strings.Split(line, "|")
		if len(fields) < 3 {
			continue
		}
		path, _, variant := strings.Cut(fields[0], " [")
		if !variant && !strings.HasSuffix(path, ".test") {
			listed[path] = true
		}
		path = strings.TrimSuffix(path, ".test")
		folder[path] = strings.TrimPrefix(slash(fields[1]), root)
		deps[path] = append(deps[path], strings.Fields(fields[2])...)
	}
	paths := make([]string, 0, len(folder))
	for p := range folder {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	touched := map[string]bool{}
	for _, file := range changed {
		best, longest := "", 0
		for _, p := range paths {
			if strings.HasPrefix(file, folder[p]+"/") && len(folder[p]) > longest {
				best, longest = p, len(folder[p])
			}
		}
		if best != "" {
			touched[best] = true
		}
	}
	var out []string
	for _, p := range paths {
		if listed[p] && (touched[p] || slices.ContainsFunc(deps[p], func(d string) bool { return touched[d] })) {
			out = append(out, p)
		}
	}
	return out
}

// capture runs name with args in dir with env and returns its stdout, an error naming its stderr.
func capture(dir string, env []string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var stderr strings.Builder
	cmd.Dir, cmd.Env, cmd.Stderr = dir, env, &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %v: %s", filepath.Base(name), strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

// slash is path with backslashes as slashes: the folders go list gives on Windows, compared with
// the slash paths of the changed files. A backslash in a folder's name elsewhere is not worth a
// platform switch the tests could not reach.
func slash(path string) string {
	return strings.ReplaceAll(path, `\`, "/")
}

// splitNUL is the names git -z gives, in order.
func splitNUL(s string) []string {
	return slices.DeleteFunc(strings.Split(s, "\x00"), func(f string) bool { return f == "" })
}

// without is env less the variables named, Windows' case-blind names included.
func without(env, names []string) []string {
	var kept []string
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if !slices.ContainsFunc(names, func(n string) bool { return strings.EqualFold(n, name) }) {
			kept = append(kept, kv)
		}
	}
	return kept
}
