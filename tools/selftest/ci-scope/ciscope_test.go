// Package ciscope holds the self-test of this repository's prose shortcut (T-008's check, kept
// as a step in T-125, a Go test since T-143).
//
// The prose shortcut must cover prose and nothing else. A path that reaches any gate (a task, a
// scenario, a config, a source file) must send the push down the full pipeline. This holds the
// project's own prose paths (itos.yaml's ci.prose.paths) to that, through itos ci scope over
// ranges made with git commit-tree on top of HEAD, each changing the paths named (objects only:
// no branch, index or file of the checkout moves; the index is a private one).
//
// What a prose-only range plans (the prose steps, the named tasks' static and prose: true
// checks, nothing late) and what waits for a task not started are the tool's behaviour, whatever
// the config: they are tools/itos/conformance/plans.yaml's and ci-run.yaml's.
package ciscope

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/tools/selftest/scratch"
)

func TestSelftest(t *testing.T) {
	scratch.Gate(t)
	root := scratch.Root(t)
	env := scratch.With(scratch.OutsideEnv(), "GIT_INDEX_FILE="+filepath.Join(t.TempDir(), "index"))
	git := func(t *testing.T, input string, args ...string) string {
		t.Helper()
		r := scratch.Run(root, env, input, scratch.Git(), args...)
		if r.Code != 0 {
			t.Fatalf("git %s failed:\n%s", strings.Join(args, " "), r.Output())
		}
		return strings.TrimSpace(r.Stdout)
	}
	// A commit on top of HEAD whose tree changes each path, added if it is new.
	changing := func(t *testing.T, paths ...string) string {
		git(t, "", "read-tree", "HEAD")
		for _, path := range paths {
			blob := git(t, "ci scope self-test: "+path+"\n", "hash-object", "-w", "--stdin")
			git(t, "", "update-index", "--add", "--cacheinfo", "100644,"+blob+","+path)
		}
		tree := git(t, "", "write-tree")
		return git(t, "", "-c", "user.name=ci-scope", "-c", "user.email=selftest@localhost",
			"commit-tree", tree, "-p", "HEAD", "-m", "docs: a ci scope self-test range")
	}
	scope := func(t *testing.T, from, to string) string {
		t.Helper()
		said := strings.TrimSpace(scratch.Itos(t, root, "ci", "scope", from, to))
		if said != "docs_only=true" && said != "docs_only=false" {
			t.Fatalf("itos ci scope said %q", said)
		}
		return said
	}

	for _, c := range []struct {
		name  string
		paths []string
		prose bool
	}{
		{"the docs and the root prose are prose", []string{"docs/HANDOFF.md", "AGENTS.md", "PLAN.md"}, true},
		{"a decision record is prose", []string{"docs/decisions/format.md"}, true},
		{"prose beside a Go source is not prose", []string{"AGENTS.md", "internal/cli/cli.go"}, false},
		{"the ledger is not prose", []string{"tasks/phase-0.yaml"}, false},
		{"a feature file is not prose", []string{"features/since.feature"}, false},
		{"package.json is not prose", []string{"package.json"}, false},
		{"go.mod is not prose", []string{"go.mod"}, false},
		{"a workflow is not prose", []string{".github/workflows/ci.yml"}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			want := "docs_only=false"
			if c.prose {
				want = "docs_only=true"
			}
			if got := scope(t, "HEAD", changing(t, c.paths...)); got != want {
				t.Errorf("a range changing %s: itos ci scope said %s, want %s", strings.Join(c.paths, ", "), got, want)
			}
		})
	}

	// Nothing known changed (a first push, a shallow clone): run everything.
	t.Run("a range with no start runs everything", func(t *testing.T) {
		if got := scope(t, "", "HEAD"); got != "docs_only=false" {
			t.Errorf("itos ci scope \"\" HEAD said %s, want docs_only=false", got)
		}
	})
}
