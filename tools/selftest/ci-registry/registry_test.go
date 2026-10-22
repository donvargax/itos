// Package registry proves the repository-local CI route over real Git ranges. The old CLI
// plan can be selected with ITOS_CI_REGISTRY_BASELINE=1 to demonstrate the wasted-suite failure.
package registry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/store/tree/adapter"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
	"github.com/donvargax/itos/v8/tools/selftest/scratch"
)

func TestSelftest(t *testing.T) {
	scratch.Gate(t)
	root := scratch.Root(t)
	helper := scratch.Build(t, "ci-repo")
	r := scratch.NewRepo(t, "ci-registry")
	base := r.Open()
	// The checkout's built binary and pinned formatter cache are shared, not rebuilt per range.
	scratch.Itos(t, root, "version")
	if err := os.Symlink(filepath.Join(root, ".tools"), filepath.Join(r.Dir, ".tools")); err != nil {
		t.Fatal(err)
	}
	it := filepath.Join(root, ".tools/bin/itos")
	invoke := func(args ...string) scratch.Result { return scratch.Run(r.Dir, r.Env, "", helper, args...) }
	readPlan := func(from, to string) scratch.Plan {
		t.Helper()
		result := invoke("plan", from, to)
		if os.Getenv("ITOS_CI_REGISTRY_BASELINE") != "" {
			result = scratch.Run(r.Dir, r.Env, "", it, "ci", "plan", from, to, "--json")
		}
		if result.Code != 0 {
			t.Fatalf("plan: %s", result.Output())
		}
		var p scratch.Plan
		if err := json.Unmarshal([]byte(result.Stdout), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	cfgBytes, err := os.ReadFile(filepath.Join(root, "itos.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.FromText(vcs.Git{}, adapter.Trees{}, adapter.Worktree, "itos.yaml", string(cfgBytes), false)
	if err != nil {
		t.Fatal(err)
	}
	indexEnv := scratch.With(r.Env, "GIT_INDEX_FILE="+filepath.Join(t.TempDir(), "index"))
	git := func(input string, args ...string) string {
		t.Helper()
		x := scratch.Run(r.Dir, indexEnv, input, scratch.Git(), args...)
		if x.Code != 0 {
			t.Fatalf("git %v: %s", args, x.Output())
		}
		return strings.TrimSpace(x.Stdout)
	}
	changing := func(message string, paths ...string) string {
		git("", "read-tree", base)
		for _, path := range paths {
			blob := git("scope fixture\n", "hash-object", "-w", "--stdin")
			git("", "update-index", "--add", "--cacheinfo", "100644,"+blob+","+path)
		}
		return git("", "commit-tree", git("", "write-tree"), "-p", base, "-m", message)
	}
	for _, c := range []struct {
		name    string
		paths   []string
		message string
		full    bool
	}{
		{"registry", []string{"tasks/work-items.yaml"}, "docs: take an item", false},
		{"registry and prose", []string{"tasks/work-items.yaml", "docs/ARCHITECTURE.md"}, "docs: record work", false},
		{"ordinary prose", []string{"README.md"}, "docs: correct a typo", false},
		{"ledger", []string{"tasks/phase-1.yaml"}, "docs: describe a task", true},
		{"other task data", []string{"tasks/other.yaml"}, "docs: add task data", true},
		{"code", []string{"internal/cli/ci.go"}, "fix: a code change", true},
		{"registry and code", []string{"tasks/work-items.yaml", "cmd/itos/main.go"}, "fix: a code change", true},
		{"feature", []string{"features/ci.feature"}, "docs: specify a case", true},
		{"config", []string{"itos.yaml"}, "ci: change configuration", true},
		{"workflow", []string{".github/workflows/ci.yml"}, "ci: change workflow", true},
		{"registry with task", []string{"tasks/work-items.yaml"}, "docs: record work\n\nTask: T-171", true},
		{"prose with task", []string{"README.md"}, "docs: record work\n\nTask: T-171", true},
		{"registry with scenario", []string{"tasks/work-items.yaml"}, "docs: record work\n\nScenarios: ID-CI-19", true},
		{"prose with scenario", []string{"README.md"}, "docs: record work\n\nScenarios: ID-CI-19", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			to := changing(c.message, c.paths...)
			p := readPlan(base, to)
			if c.full {
				if p.Prose || !slices.Contains(p.Steps, "go test ./cmd/... ./internal/...") {
					t.Fatalf("full checks disappeared: %+v", p)
				}
				if strings.Contains(c.message, "Scenarios:") && !strings.Contains(strings.Join(p.Steps, "\n"), "ID-CI-19") {
					t.Fatal("named scenario disappeared")
				}
			} else {
				if !p.Prose || !slices.Equal(p.Steps, cfg.CI.Prose.Steps) {
					t.Fatalf("metadata runs unnecessary suites instead of configured validation: %+v", p.Steps)
				}
			}
			var s struct{ Full bool }
			x := invoke("scope", base, to)
			if x.Code != 0 || json.Unmarshal([]byte(x.Stdout), &s) != nil || s.Full != c.full {
				t.Fatalf("scope disagrees: %s", x.Output())
			}
		})
	}
	if os.Getenv("ITOS_CI_REGISTRY_BASELINE") != "" {
		return
	}
	for _, from := range []string{"", "not-a-commit", base} {
		t.Run("unknown or empty "+from, func(t *testing.T) {
			if p := readPlan(from, base); p.Prose || !slices.Contains(p.Steps, "go test ./cmd/... ./internal/...") {
				t.Fatal("unknown/empty range reduced")
			}
		})
	}
	t.Run("rename source is retained", func(t *testing.T) {
		git("", "read-tree", base)
		blob := git("", "rev-parse", base+":internal/cli/ci.go")
		git("", "update-index", "--force-remove", "internal/cli/ci.go")
		git("", "update-index", "--add", "--cacheinfo", "100644,"+blob+",docs/moved.md")
		to := git("", "commit-tree", git("", "write-tree"), "-p", base, "-m", "docs: move code")
		if readPlan(base, to).Prose {
			t.Fatal("code renamed into prose was reduced")
		}
	})
	t.Run("named late check still runs", func(t *testing.T) {
		// Use a separate fixture policy so this test never executes the full real suites.
		policy := `version: 1
ledger: {files: "tasks/phase-{group}.yaml", id: '^T-[0-9]+$'}
commits:
  footers:
    Task: {source: ledger}
ci:
  steps: ['true']
  prose: {paths: ['**/*.md'], steps: ['true']}
`
		scratch.WriteFiles(t, r.Dir, map[string]string{"fixture.yaml": policy, "tasks/phase-99999.yaml": "- id: T-99999\n  type: ci\n  title: late check\n  done_when:\n    - run: 'exit 37'\n      cost: late\n"})
		defer os.Remove(filepath.Join(r.Dir, "fixture.yaml"))
		defer os.Remove(filepath.Join(r.Dir, "tasks/phase-99999.yaml"))
		to := changing("docs: named late check\n\nTask: T-99999", "tasks/work-items.yaml")
		x := scratch.Run(r.Dir, scratch.With(r.Env, "ITOS_CONFIG=fixture.yaml"), "", helper, "run", base, to)
		if x.Code == 0 || !strings.Contains(x.Output(), "exit 37") {
			t.Fatalf("late task lost: %s", x.Output())
		}
	})
	t.Run("real validation accepts sound data and refuses malformed registry", func(t *testing.T) {
		path := filepath.Join(r.Dir, "tasks/work-items.yaml")
		original, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		scratch.WriteFiles(t, r.Dir, map[string]string{"tasks/work-items.yaml": string(original) + "\n# Registry route fixture.\n"})
		r.Git("add", "--", "tasks/work-items.yaml")
		to := r.Git("commit-tree", r.Git("write-tree"), "-p", base, "-m", "docs: record work")
		r.Git("reset", "-q", "--hard", to)
		x := scratch.Run(r.Dir, r.Env, "", it, "verify", base, to)
		if x.Code != 0 {
			t.Fatalf("real verification: %s", x.Output())
		}
		x = invoke("run", base, to)
		if x.Code != 0 {
			t.Fatalf("real validation: %s", x.Output())
		}
		for _, proof := range []string{"dprint check", "config check", "plugin-version:", "doc-budget:"} {
			if !strings.Contains(x.Output(), proof) {
				t.Fatalf("validation %s did not execute: %s", proof, x.Output())
			}
		}
		scratch.WriteFiles(t, r.Dir, map[string]string{"tasks/work-items.yaml": "invalid_registry_key: true\n"})
		x = invoke("run", base, to)
		if x.Code == 0 || !strings.Contains(x.Output(), "config check") {
			t.Fatalf("malformed registry accepted or validation never reached: %s", x.Output())
		}
	})
}
