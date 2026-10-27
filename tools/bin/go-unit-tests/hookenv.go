package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/git"
)

// hookNames are the hooks of a repository whose command git runs on its own: every one of them
// fails in the folder core.hooksPath names, so a commit, a checkout, a merge, a push or a ref
// update a test makes through the environment it was handed is refused, not recorded.
var hookNames = []string{
	"pre-commit", "prepare-commit-msg", "commit-msg", "post-commit", "pre-merge-commit",
	"pre-rebase", "post-checkout", "post-merge", "post-rewrite", "pre-push", "reference-transaction",
	"push-to-checkout", "pre-auto-gc",
}

// pointers are the variables the hook environment sets that the run replaces: the ones that
// point git at a repository, its index or its objects, or add to its configuration. ITOS_GIT stays
// as itos exports it to every program it starts, the real git's path.
func pointers() []string {
	return slices.DeleteFunc(slices.Clone(handed), func(n string) bool { return n == "ITOS_GIT" })
}

// hookEnv runs go test over packages in dir with the environment git hands a hook, in the worst
// case a test can meet it: GIT_DIR, GIT_WORK_TREE and GIT_INDEX_FILE point at a scratch
// repository of one commit, and GIT_CONFIG_PARAMETERS sets core.hooksPath to a folder whose hooks
// all fail. A test that writes through them, or whose own repository inherits a hook that fails,
// is a test that does not isolate itself from its caller, and it is red here: when a test fails,
// or when the scratch repository's HEAD, refs, index or config differ afterwards. The scratch
// repository and the hooks are in a folder this makes and removes.
func hookEnv(packages []string, dir string, stdout, stderr io.Writer) int {
	if len(packages) == 0 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	realGit := git.Bin(runtime.GOOS)
	clean := without(os.Environ(), pointers())

	scratch, err := os.MkdirTemp("", "go-unit-tests-hook-env-")
	if err != nil {
		fmt.Fprintf(stderr, "go-unit-tests: %v\n", err)
		return 2
	}
	defer os.RemoveAll(scratch)
	// The folder as the system names it: a /var that is /private/var on macOS.
	if real, err := filepath.EvalSymlinks(scratch); err == nil {
		scratch = real
	}
	repo, err := newScratch(scratch, realGit, clean)
	if err != nil {
		fmt.Fprintf(stderr, "go-unit-tests: the scratch repository: %v\n", err)
		return 2
	}
	before, err := repo.snapshot(realGit, clean)
	if err != nil {
		fmt.Fprintf(stderr, "go-unit-tests: the scratch repository: %v\n", err)
		return 2
	}

	var ran []string
	if listed, err := capture(dir, clean, "go", append([]string{"list", "-e"}, packages...)...); err == nil {
		ran = strings.Fields(listed)
	} else {
		ran = packages
	}
	env := append(slices.Clone(clean),
		"GIT_DIR="+repo.gitDir,
		"GIT_WORK_TREE="+repo.work,
		"GIT_INDEX_FILE="+repo.index(),
		"GIT_CONFIG_PARAMETERS="+configParameter("core.hooksPath", repo.hooks),
	)
	fmt.Fprintln(stdout, "$ go test -count=1 "+strings.Join(packages, " ")+"  # under a hook's git environment")
	cmd := exec.Command("go", append([]string{"test", "-count=1"}, packages...)...)
	cmd.Dir, cmd.Env, cmd.Stdout, cmd.Stderr = dir, env, stdout, stderr
	var failed []string
	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			fmt.Fprintf(stderr, "go-unit-tests: %v\n", err)
			return 2
		}
		failed = append(failed, "a test failed")
	}
	after, err := repo.snapshot(realGit, clean)
	if err != nil {
		fmt.Fprintf(stderr, "go-unit-tests: the scratch repository: %v\n", err)
		return 2
	}
	failed = append(failed, before.differences(after)...)
	if len(failed) > 0 {
		fmt.Fprintf(stderr, "go-unit-tests -hook-env: red under a hook's git environment: %s.\n", strings.Join(failed, "; "))
		fmt.Fprintf(stderr, "A test that writes through GIT_DIR, GIT_WORK_TREE or GIT_INDEX_FILE, or whose own repository inherits GIT_CONFIG_PARAMETERS, does not isolate itself: see alone(t) and the features' world.env().\n")
		fmt.Fprintf(stderr, "The packages that ran (%d):\n  %s\n", len(ran), strings.Join(ran, "\n  "))
		return 1
	}
	fmt.Fprintf(stdout, "go-unit-tests -hook-env: %d packages ran under a hook's git environment, and the scratch repository is as it was.\n", len(ran))
	return 0
}

// scratchRepo is the repository the hook environment points at, in a folder of its own.
type scratchRepo struct {
	work, gitDir, hooks string
}

func (r scratchRepo) index() string  { return filepath.Join(r.gitDir, "index") }
func (r scratchRepo) config() string { return filepath.Join(r.gitDir, "config") }

// newScratch makes the repository in root, with one commit made under a core.hooksPath of no hook
// and an identity of its own, and the folder of failing hooks beside it.
func newScratch(root, realGit string, env []string) (scratchRepo, error) {
	r := scratchRepo{work: filepath.Join(root, "work"), hooks: filepath.Join(root, "hooks")}
	r.gitDir = filepath.Join(r.work, ".git")
	quiet := filepath.Join(root, "no-hooks")
	for _, d := range []string{r.work, r.hooks, quiet} {
		if err := os.Mkdir(d, 0o755); err != nil {
			return r, err
		}
	}
	for _, name := range hookNames {
		if err := os.WriteFile(filepath.Join(r.hooks, name), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
			return r, err
		}
	}
	if err := os.WriteFile(filepath.Join(r.work, "scratch.txt"), []byte("scratch\n"), 0o644); err != nil {
		return r, err
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.name", "go-unit-tests"},
		{"config", "user.email", "go-unit-tests@example.com"},
		{"config", "commit.gpgsign", "false"},
		{"add", "scratch.txt"},
		{"-c", "core.hooksPath=" + slash(quiet), "commit", "-q", "-m", "scratch"},
	} {
		if _, err := capture(r.work, env, realGit, args...); err != nil {
			return r, err
		}
	}
	return r, nil
}

// snapshot is what a test must leave alone: HEAD, every ref, the index file and the config file.
type snapshot struct {
	head, refs, index, config []byte
}

func (r scratchRepo) snapshot(realGit string, env []string) (snapshot, error) {
	var s snapshot
	var err error
	if s.head, err = os.ReadFile(filepath.Join(r.gitDir, "HEAD")); err != nil {
		return s, err
	}
	refs, err := capture(r.work, env, realGit, "--git-dir", r.gitDir, "for-each-ref", "--format=%(refname) %(objectname)")
	if err != nil {
		return s, err
	}
	s.refs = []byte(refs)
	if s.index, err = os.ReadFile(r.index()); err != nil {
		return s, err
	}
	s.config, err = os.ReadFile(r.config())
	return s, err
}

// differences names what a test changed since before, in a sentence each.
func (before snapshot) differences(after snapshot) []string {
	var out []string
	for _, c := range []struct {
		name    string
		was, is []byte
	}{
		{"HEAD", before.head, after.head},
		{"refs", before.refs, after.refs},
		{"index file", before.index, after.index},
		{"config file", before.config, after.config},
	} {
		if !bytes.Equal(c.was, c.is) {
			out = append(out, "the scratch repository's "+c.name+" changed")
		}
	}
	return out
}

// configParameter is the entry git's own -c writes to GIT_CONFIG_PARAMETERS: the name and the
// value, each single-quoted as sh does, a quote and a ! escaped.
func configParameter(name, value string) string {
	return shQuote(name) + "=" + shQuote(slash(value))
}

func shQuote(s string) string {
	s = strings.ReplaceAll(s, "'", `'\''`)
	s = strings.ReplaceAll(s, "!", `'\!'`)
	return "'" + s + "'"
}
