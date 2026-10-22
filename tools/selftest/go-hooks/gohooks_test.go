// Package gohooks holds the self-test that the hooks run the Go unit tests a change reaches
// (T-059, a Go test since T-143). Run against the real hooks in a scratch worktree of the current
// tree (uncommitted edits included), as the gates self-test runs the other gates, itos's pre-push
// hook as git runs it from the git config (scratch.Repo.HookRun):
//
//   - pre-commit runs the unit tests of the Go package a staged change touches and of the
//     packages that import it, and no other package's;
//   - a staged Go change whose unit test fails is refused at commit, and one whose tests pass is
//     committed;
//   - pre-push does the same against the remote commit the push builds on, after the commit
//     rules (itos verify), so each commit carries its task footer;
//   - a change to go.mod, which every package builds with, runs every package;
//   - a Markdown-only commit, and a prose-only push, run no Go test;
//   - pre-push runs the pinned golangci-lint, CI's lint step, when the push changes Go, and a
//     finding refuses the push; a prose-only push runs no lint (T-220).
//
// tools/bin/go-unit-tests picks the packages; both hooks call it. tools/bin/golangci-lint-changed
// decides whether pre-push lints.
package gohooks

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/tools/selftest/scratch"
)

const module = "github.com/donvargax/itos/v8/"

// tested is the packages a go test run reported, by the line it prints for each: ok, FAIL or ?
// (no test files), a tab, the import path.
func tested(output, verdict string) map[string]bool {
	re := regexp.MustCompile(`(?m)^(?:` + verdict + `)\s+` + regexp.QuoteMeta(module) + `(\S+)`)
	ran := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(output, -1) {
		ran[m[1]] = true
	}
	return ran
}

const anyVerdict = `ok|FAIL|\?`

func show(packages map[string]bool) string {
	var names []string
	for p := range packages {
		names = append(names, p)
	}
	slices.Sort(names)
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

// A change to internal/version reaches it, internal/cli (which imports it) and cmd/itos (which
// imports internal/cli and has no tests), so those must be tested; internal/glob imports none of
// them, so it must not be. The check asks for at least those, not for an exact list: the
// importers of internal/version grow as itos does, and a new importer is reached correctly, so it
// must not turn this self-test red. The one package left out still proves the hooks test what a
// change reaches, not everything.
const (
	pkg       = "internal/version"
	file      = pkg + "/version.go"
	unreached = "internal/glob"
)

var reached = []string{"internal/version", "internal/cli", "cmd/itos"}

// linted is golangci-lint-changed's line when it runs the linter.
var linted = regexp.MustCompile(`(?m)^\$ tools/bin/pinned golangci-lint run `)

func testsReached(t *testing.T, hook, output string) {
	t.Helper()
	ran := tested(output, anyVerdict)
	for _, p := range reached {
		if !ran[p] {
			t.Errorf("%s should test %s for a change to %s, tested %s", hook, p, pkg, show(ran))
		}
	}
	if ran[unreached] {
		t.Errorf("%s should not test %s for a change to %s", hook, unreached, pkg)
	}
}

func TestSelftest(t *testing.T) {
	scratch.Gate(t)
	repo := scratch.NewRepo(t, "go-hooks-selftest")
	hooks := scratch.Hooks{R: repo}
	base := repo.Open()
	repo.SharePinned("golangci-lint")
	var every []string
	for _, line := range strings.Split(repo.Sh("go list ./cmd/... ./internal/...", "", "").Stdout, "\n") {
		if line != "" {
			every = append(every, line)
		}
	}
	reset := func() { repo.Git("reset", "-q", "--hard", base) }

	// A harmless change to one package runs its tests and its importers', passes, and is
	// committed; the push of it does the same.
	var harmless string
	t.Run("pre-commit tests what a harmless change reaches, and passes it", func(t *testing.T) {
		reset()
		repo.Edit(file, "// stamp is", "// go-hooks self-test: a harmless change\n// stamp is")
		r := hooks.PreCommit(t, "a harmless change to "+pkg)
		if r.Code != 0 {
			t.Errorf("pre-commit refused a harmless change to %s:\n%s", pkg, r.Output())
		}
		testsReached(t, "pre-commit", r.Output())
		harmless = repo.Commit("refactor: touch the version package\n\nTask: T-059")
	})
	t.Run("pre-push tests what that change reaches, and passes it", func(t *testing.T) {
		if harmless == "" {
			t.Fatal("the harmless change was not committed")
		}
		r := hooks.PrePush(t, "that change, committed", base, harmless)
		if r.Code != 0 {
			t.Errorf("pre-push refused a harmless change to %s:\n%s", pkg, r.Output())
		}
		if !linted.MatchString(r.Output()) {
			t.Errorf("pre-push did not lint a push that changes Go:\n%s", r.Output())
		}
		testsReached(t, "pre-push", r.Output())
	})

	// A context held in a struct, which ADR-0053 refuses and only golangci-lint's containedctx
	// catches: it builds and breaks no unit test, so the lint alone refuses the push.
	t.Run("pre-push refuses a change golangci-lint finds fault with", func(t *testing.T) {
		reset()
		scratch.WriteFiles(t, repo.Dir, map[string]string{
			"internal/glob/held.go": "package glob\n\nimport \"context\"\n\n// Held holds a context.\ntype Held struct{ Ctx context.Context }\n",
		})
		finding := repo.Commit("refactor: hold a context\n\nTask: T-220")
		r := hooks.PrePush(t, "a context held in a struct", base, finding)
		if r.Code == 0 {
			t.Errorf("pre-push let through a finding of golangci-lint:\n%s", r.Output())
		}
		if !strings.Contains(r.Output(), "(containedctx)") {
			t.Errorf("pre-push did not name containedctx's finding:\n%s", r.Output())
		}
	})

	// The same package, broken so its unit test fails: refused at commit, in that package's
	// test, and at push.
	var broken string
	t.Run("pre-commit refuses a change that breaks a unit test", func(t *testing.T) {
		reset()
		repo.Edit(file, `return strings.TrimPrefix(v, "v")`, "return v")
		r := hooks.PreCommit(t, "a change that breaks "+pkg+"'s test")
		if r.Code == 0 {
			t.Errorf("pre-commit let through a change that breaks %s's unit test", pkg)
		}
		if failed := tested(r.Output(), "FAIL"); !failed[pkg] {
			t.Errorf("pre-commit should fail in %s, failed in %s", pkg, show(failed))
		}
		broken = repo.Commit("refactor: break the version package\n\nTask: T-059")
	})
	t.Run("pre-push refuses that change", func(t *testing.T) {
		if broken == "" {
			t.Fatal("the breaking change was not committed")
		}
		r := hooks.PrePush(t, "that change, committed", base, broken)
		if r.Code == 0 {
			t.Errorf("pre-push let through a change that breaks %s's unit test", pkg)
		}
		if !tested(r.Output(), "FAIL")[pkg] {
			t.Errorf("pre-push did not fail in %s", pkg)
		}
	})

	// go.mod is read by every package's build: a change to it tests all.
	t.Run("a change to go.mod tests every package", func(t *testing.T) {
		reset()
		repo.Edit("go.mod", "module ", "// go-hooks self-test: a harmless change\nmodule ")
		r := hooks.PreCommit(t, "a change to go.mod")
		if r.Code != 0 {
			t.Errorf("pre-commit refused a comment in go.mod:\n%s", r.Output())
		}
		if n := len(tested(r.Output(), anyVerdict)); n != len(every) {
			t.Errorf("a change to go.mod should test all %d packages, tested %d", len(every), n)
		}
	})

	// Prose runs no Go test, at commit or at push.
	var prose string
	t.Run("a Markdown-only commit runs no Go test", func(t *testing.T) {
		reset()
		repo.Edit("README.md", "# ", "A prose edit.\n\n# ")
		r := hooks.PreCommit(t, "a Markdown-only change")
		if r.Code != 0 {
			t.Errorf("pre-commit refused a Markdown-only change:\n%s", r.Output())
		}
		if strings.Contains(r.Output(), "go test") {
			t.Errorf("a Markdown-only commit ran Go tests:\n%s", r.Output())
		}
		prose = repo.Commit("docs: edit the readme")
	})
	t.Run("a prose-only push runs no Go test and no lint", func(t *testing.T) {
		if prose == "" {
			t.Fatal("the prose change was not committed")
		}
		r := hooks.PrePush(t, "a prose-only push", base, prose)
		if r.Code != 0 {
			t.Errorf("pre-push refused a prose-only push:\n%s", r.Output())
		}
		if ran := tested(r.Output(), anyVerdict); len(ran) != 0 {
			t.Errorf("a prose-only push ran Go tests: %s", show(ran))
		}
		if linted.MatchString(r.Output()) {
			t.Errorf("a prose-only push ran golangci-lint:\n%s", r.Output())
		}
	})
}
