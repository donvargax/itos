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
	// The checkout's built binary is shared, not rebuilt per range.
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
	cfg, err := config.FromText(vcs.Git{}, adapter.Trees{VCS: vcs.Git{}}, adapter.Worktree, "itos.yaml", string(cfgBytes), false)
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
	// onto is a commit on top of parent whose tree holds each file with the text given, or
	// without it for the text deleted.
	const deleted = "\x00deleted"
	onto := func(parent, message string, files map[string]string) string {
		git("", "read-tree", parent)
		for path, text := range files {
			if text == deleted {
				git("", "update-index", "--force-remove", path)
				continue
			}
			blob := git(text, "hash-object", "-w", "--stdin")
			git("", "update-index", "--add", "--cacheinfo", "100644,"+blob+","+path)
		}
		return git("", "commit-tree", git("", "write-tree"), "-p", parent, "-m", message)
	}
	changing := func(message string, paths ...string) string {
		files := map[string]string{}
		for _, path := range paths {
			files[path] = "scope fixture\n"
		}
		return onto(base, message, files)
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
	// A ledger task's entry is compared at both ends by its id: whole entries gained or lost
	// are bookkeeping, a change inside one is not. The tasks live in a fixture commit, so the
	// cases hold whatever the real ledger ends with.
	entry := func(id, kind, check string) string {
		return "- id: " + id + "\n  type: " + kind + "\n  title: a fixture task\n  why: >\n    Held by the scope self-test.\n  done_when:\n    - run: " + check + "\n"
	}
	shown := func(ref, path string) string { return r.Git("show", ref+":"+path) + "\n" }
	ledgerAt := func(ref string) string { return shown(ref, "tasks/phase-1.yaml") }
	registryAt := func(ref string) string { return shown(ref, "tasks/work-items.yaml") + "# bookkeeping\n" }
	kept := entry("T-99997", "ci", "'true'")
	start := onto(base, "docs: add a fixture task", map[string]string{"tasks/phase-1.yaml": ledgerAt(base) + kept})
	ledger := func(text string) map[string]string { return map[string]string{"tasks/phase-1.yaml": text} }
	grown := ledgerAt(start) + entry("T-99998", "chore", "'true'")
	for _, c := range []struct {
		name    string
		files   map[string]string
		message string
		full    bool
	}{
		{"task added", ledger(grown), "docs: add T-99998", false},
		{"task added with the registry", map[string]string{"tasks/phase-1.yaml": grown, "tasks/work-items.yaml": registryAt(start)}, "docs: add T-99998", false},
		{"task added with prose", map[string]string{"tasks/phase-1.yaml": grown, "docs/ARCHITECTURE.md": "prose\n"}, "docs: add T-99998", false},
		{"task added in a new ledger file", map[string]string{"tasks/phase-99998.yaml": entry("T-99998", "chore", "'true'")}, "docs: add T-99998", false},
		{"task removed", ledger(ledgerAt(base)), "docs: drop T-99997", false},
		{"task added and another removed", ledger(ledgerAt(base) + entry("T-99998", "chore", "'true'")), "docs: swap tasks", false},
		{"done_when edited", ledger(strings.Replace(ledgerAt(start), "run: 'true'", "run: 'false'", 1)), "docs: change a check", true},
		{"type edited", ledger(strings.Replace(ledgerAt(start), "type: ci\n  title: a fixture", "type: docs\n  title: a fixture", 1)), "docs: change a type", true},
		{"checks taken out when a task closes", ledger(strings.Replace(ledgerAt(start), "  done_when:\n    - run: 'true'\n", "", 1)), "docs: close T-99997", true},
		{"task added beside code", map[string]string{"tasks/phase-1.yaml": grown, "internal/cli/ci.go": "package cli\n"}, "fix: add T-99998 and code", true},
		{"task added beside a feature", map[string]string{"tasks/phase-1.yaml": grown, "features/ci.feature": "Feature: x\n"}, "docs: add T-99998", true},
		{"task added beside configuration", map[string]string{"tasks/phase-1.yaml": grown, "itos.yaml": "version: 1\n"}, "docs: add T-99998", true},
		{"task added naming it", ledger(grown), "docs: add T-99998\n\nTask: T-99998", true},
		{"task added naming a scenario", ledger(grown), "docs: add T-99998\n\nScenarios: ID-CI-19", true},
		{"task moved to another file", map[string]string{"tasks/phase-1.yaml": ledgerAt(base), "tasks/phase-99998.yaml": kept}, "docs: move T-99997", true},
		{"ledger file deleted", map[string]string{"tasks/phase-2.yaml": deleted}, "docs: delete a ledger file", true},
		{"ledger file not a list", ledger("id: T-99997\n"), "docs: break the ledger", true},
		{"ledger file with a repeated id", ledger(ledgerAt(start) + kept), "docs: repeat a task", true},
		{"ledger entry without an id", ledger(ledgerAt(start) + "- type: ci\n"), "docs: add a task without an id", true},
		{"ledger file that is not YAML", ledger("- id: [\n"), "docs: break the ledger", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			to := onto(start, c.message, c.files)
			p := readPlan(start, to)
			if c.full {
				if p.Prose || !slices.Contains(p.Steps, "go test ./cmd/... ./internal/...") {
					t.Fatalf("full checks disappeared: %+v", p)
				}
			} else if !p.Prose || !slices.Equal(p.Steps, cfg.CI.Prose.Steps) {
				t.Fatalf("a ledger range of whole entries did not take the metadata route: %+v", p.Steps)
			}
			var s struct{ Full bool }
			x := invoke("scope", start, to)
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
