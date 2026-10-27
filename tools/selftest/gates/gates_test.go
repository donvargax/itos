// Package gates holds the gates' self-test (T-011, a Go test since T-143): the local gates leave
// to CI what they do not run, and CI catches it. Run against the real hooks in a scratch worktree
// of the current tree (uncommitted edits included), so nothing here touches the checkout it was
// started from; itos's commit-msg and pre-push hooks run as git runs them, from the git config,
// declared there as itos hook install declares them (decision 37, scratch.Repo.HookRun):
//
//   - CI runs the whole unit suite and the features, which the hooks leave to it;
//   - pre-commit formats a root .yml file, as it must format action.yml (T-127);
//   - pre-commit refuses a staged file over its doc budget, judging the staged size, and runs
//     doc-budget only when a file it caps is staged (T-167);
//   - pre-commit refuses a command table a staged declaration in internal/cli or a slice leaves
//     stale, and checks none when only a test file there is staged (T-242);
//   - pre-push passes a prose-only push;
//   - pre-push runs neither the scenarios a commit's Scenarios: footer names nor the checks of
//     the tasks its Task: footer names; CI reads those footers from the pushed range and runs
//     them;
//   - the commit-msg hook, itos hook commit-msg as the git config declares it, rejects a commit
//     whose type may not touch a staged path, a scenario renamed outside feat and fix, and a
//     header without a type, by the built-in header lint with no node_modules in the scratch
//     copy, so no commitlint (T-063), and lets a sound commit through there too;
//   - what the hooks leave out fails CI's own steps: a refactor that changes what itos prints, in
//     a Go package whose unit tests do not read that line, with the task footer pre-push's commit
//     rules (itos verify) want, passes both hooks and fails the push's features step, whose smoke
//     set reads that output from the scratch copy's tools/bin/itos, rebuilt from the changed
//     source.
//
// The plan and the steps are the scratch copy's tools/bin/itos ci plan --json. That the hooks run
// exactly the unit tests a change reaches, and fail on a change that breaks one, is the go-hooks
// self-test's (T-059), which the nightly runs beside this.
package gates

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/tools/selftest/scratch"
)

func TestSelftest(t *testing.T) {
	scratch.Gate(t)
	repo := scratch.NewRepo(t, "gates-selftest")
	hooks := scratch.Hooks{R: repo}
	messages := t.TempDir()
	commitMsg := func(t *testing.T, label, message string) scratch.Result {
		t.Helper()
		repo.Git("add", "-A")
		file := filepath.Join(messages, "COMMIT_EDITMSG")
		if err := os.WriteFile(file, []byte(message), 0o644); err != nil {
			t.Fatal(err)
		}
		return hooks.CommitMsg(t, label, file)
	}
	reset := func(base string) { repo.Git("reset", "-q", "--hard", base) }

	base := repo.Open()
	const features = "go test ./features -count=1"
	if _, err := os.Stat(filepath.Join(repo.Dir, "node_modules")); err == nil {
		t.Fatal("the scratch copy holds node_modules")
	}

	t.Run("CI runs the whole unit suite and the features", func(t *testing.T) {
		steps := scratch.CIPlan(t, repo.Dir, "--whole").Steps
		for _, step := range []string{features, "go test ./cmd/... ./internal/..."} {
			if !slices.Contains(steps, step) {
				t.Errorf("CI no longer runs `%s`, which the hooks leave to it", step)
			}
		}
	})

	// A root .yml file is formatted by the actual staged hook, not left for CI to reject (T-127).
	// A scratch fixture, not a policy file read as text: its missing final newline is the failure
	// action.yml reached CI with, since the staged root glob accepted .yaml but not .yml.
	t.Run("pre-commit formats a root .yml file", func(t *testing.T) {
		reset(base)
		yml := filepath.Join(repo.Dir, "launcher-format.yml")
		if err := os.WriteFile(yml, []byte("name: itos"), 0o644); err != nil {
			t.Fatal(err)
		}
		r := hooks.PreCommit(t, "a root .yml file without its final newline")
		if r.Code != 0 {
			t.Errorf("pre-commit failed on a root .yml file:\n%s", r.Output())
		}
		if got, _ := os.ReadFile(yml); string(got) != "name: itos\n" {
			t.Errorf("pre-commit left a root .yml file unformatted: %q", got)
		}
	})

	// A declaration re-ranked ahead of every other one moves its command to the top of the table
	// tools/bin/assemble writes, so the committed table is stale (T-242).
	const stale = "assemble: internal/cli/commands_gen.go is not what the command declarations make"
	for file, rank := range map[string]string{
		"internal/slices/config/command.go":  "Rank: 200,",
		"internal/slices/version/command.go": "Rank: 210,",
	} {
		t.Run("pre-commit refuses a stale command table, "+file, func(t *testing.T) {
			reset(base)
			repo.Edit(file, rank, "Rank: 1,")
			r := hooks.PreCommit(t, "a re-ranked declaration in "+file)
			if r.Code != 1 || !strings.Contains(r.Output(), stale) {
				t.Errorf("pre-commit did not refuse the command table %s leaves stale:\n%s", file, r.Output())
			}
		})
	}
	t.Run("pre-commit checks no command table when only a test file is staged", func(t *testing.T) {
		reset(base)
		repo.Edit("internal/cli/commands_gen.go", "package cli", "package cli\n\n// stale")
		repo.Edit("internal/cli/cli_test.go", "package cli", "package cli\n\n// staged")
		repo.Git("add", "internal/cli/cli_test.go")
		r := hooks.Gate(t, "pre-commit, a test file staged, the command table stale", "sh tools/bin/pre-commit")
		if r.Code != 0 || strings.Contains(r.Output(), "assemble") {
			t.Errorf("pre-commit checked the command table with only a test file staged:\n%s", r.Output())
		}
	})

	// The doc budget judges a capped file by its staged size, and runs only when one is staged
	// (T-167). The scratch copy's caps.json caps one file, budget.md, at 100 bytes.
	reset(base)
	budget := filepath.Join(repo.Dir, "budget.md")
	write := func(file string, size int) {
		t.Helper()
		if err := os.WriteFile(file, []byte(strings.Repeat("x", size-1)+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(budget, 50)
	if err := os.WriteFile(filepath.Join(repo.Dir, "tools/bin/doc-budget/caps.json"),
		[]byte(`{"caps": [{"file": "budget.md", "bytes": 100}]}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	capped := repo.Commit("ci: cap budget.md")
	t.Run("pre-commit refuses a staged file over its doc budget", func(t *testing.T) {
		reset(capped)
		write(budget, 101)
		r := hooks.PreCommit(t, "a capped file over its cap")
		if r.Code != 1 || !strings.Contains(r.Output(), "doc-budget: budget.md is 101 bytes, over its cap of 100 by 1") {
			t.Errorf("pre-commit did not refuse a capped file over its cap with doc-budget's message:\n%s", r.Output())
		}
	})
	t.Run("pre-commit judges a capped file by its staged size", func(t *testing.T) {
		reset(capped)
		write(budget, 100)
		repo.Git("add", "budget.md")
		write(budget, 500)
		r := hooks.Gate(t, "pre-commit, a capped file within its cap, over it unstaged", "sh tools/bin/pre-commit")
		if r.Code != 0 || !strings.Contains(r.Output(), "doc-budget: budget.md is 100 bytes, within its cap of 100") {
			t.Errorf("pre-commit did not pass a capped file staged within its cap:\n%s", r.Output())
		}
	})
	t.Run("pre-commit runs no doc budget when no capped file is staged", func(t *testing.T) {
		reset(capped)
		write(budget, 500)
		repo.Edit("README.md", "# ", "A prose edit.\n\n# ")
		repo.Git("add", "README.md")
		r := hooks.Gate(t, "pre-commit, a capped file over its cap, unstaged", "sh tools/bin/pre-commit")
		if r.Code != 0 || strings.Contains(r.Output(), "doc-budget") {
			t.Errorf("pre-commit ran the doc budget with no capped file staged:\n%s", r.Output())
		}
	})

	t.Run("pre-push passes a prose-only push", func(t *testing.T) {
		reset(base)
		repo.Edit("README.md", "# ", "A prose edit.\n\n# ")
		sha := repo.Commit("docs: edit the readme")
		if r := hooks.PrePush(t, "a prose-only push", base, sha); r.Code != 0 {
			t.Errorf("pre-push failed on a prose-only push:\n%s", r.Output())
		}
	})

	// A commit that names a scenario and a task leaves both to CI...
	reset(base)
	repo.Edit("README.md", "# ", "A footed edit.\n\n# ")
	footed := repo.Commit("chore: name a scenario and a task\n\nScenarios: @ID-SINCE-01\nTask: T-007")
	t.Run("pre-push leaves the scenarios and the tasks a commit names to CI", func(t *testing.T) {
		r := hooks.PrePush(t, "a push naming a scenario and a task", base, footed)
		if r.Code != 0 {
			t.Errorf("pre-push failed on a footed push:\n%s", r.Output())
		}
		if strings.Contains(r.Output(), features) {
			t.Error("pre-push ran the scenarios a footer names")
		}
		if strings.Contains(r.Output(), "$ tools/bin/itos task") {
			t.Error("pre-push ran the checks of a task a footer names")
		}
	})
	// ...and CI finds the task in the pushed range.
	t.Run("CI finds the task a pushed commit names", func(t *testing.T) {
		if named := scratch.CIPlan(t, repo.Dir, base, footed).Tasks; !slices.Contains(named, "T-007") {
			t.Errorf("CI did not find T-007 in the pushed range: %v", named)
		}
	})

	// The commit-msg hook, as the git config declares it. A docs commit may not touch itos; a
	// test commit may not rename a live scenario; the header lint rejects a header without a
	// type; a docs commit with its footer passes.
	t.Run("commit-msg rejects a docs commit touching itos", func(t *testing.T) {
		reset(base)
		// A line appended to a Go source under internal/: anchored on nothing in the file, so no
		// move of its comments can break it (T-114).
		const goSource = "internal/version/version.go"
		f, err := os.OpenFile(filepath.Join(repo.Dir, goSource), os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		f.WriteString("\n// gates self-test: a docs commit\n")
		f.Close()
		r := commitMsg(t, "a docs commit touching itos", "docs: touch the package\n\nTask: T-007\n")
		if r.Code != 1 || !strings.Contains(r.Output(), "docs commits may not touch "+goSource) {
			t.Errorf("commit-msg did not reject a docs commit touching %s:\n%s", goSource, r.Output())
		}
	})
	t.Run("commit-msg rejects a test commit renaming a live scenario", func(t *testing.T) {
		reset(base)
		repo.Edit("features/since.feature",
			"Scenario: verify skips the commit commits.since names, and its ancestors",
			"Scenario: verify skips commits.since, renamed")
		r := commitMsg(t, "a test commit renaming a scenario", "test: rename a scenario\n\nTask: T-007\n")
		if r.Code != 1 || !strings.Contains(r.Output(), "  - a test commit ") {
			t.Errorf("commit-msg did not reject a test commit renaming a live scenario:\n%s", r.Output())
		}
	})
	// The header lint is the built-in one: with no node_modules in the scratch copy, nothing of
	// commitlint's can run, and the header is still judged, by commitlint's rule and words.
	t.Run("commit-msg rejects a header without a type, with no node_modules", func(t *testing.T) {
		reset(base)
		repo.Edit("README.md", "# ", "A message check.\n\n# ")
		r := commitMsg(t, "a header without a type, no node_modules", "update things\n")
		if r.Code != 1 || !strings.Contains(r.Output(), "✖   type may not be empty [type-empty]") {
			t.Errorf("commit-msg did not reject a header without a type with no node_modules:\n%s", r.Output())
		}
	})
	t.Run("commit-msg lets a sound docs commit through, with no node_modules", func(t *testing.T) {
		reset(base)
		repo.Edit("README.md", "# ", "A message check.\n\n# ")
		if r := commitMsg(t, "a sound docs commit, no node_modules", "docs: edit the readme\n\nTask: T-007\n"); r.Code != 0 {
			t.Errorf("commit-msg rejected a sound docs commit:\n%s", r.Output())
		}
	})

	// What the hooks leave out, CI's steps catch. A refactor that changes what itos prints, in
	// a Go package whose unit tests do not read that line, passes both hooks, pre-push's commit
	// rules too, since it carries its task footer... The line is ci run's for a task check a step
	// covers, internal/ci's: no unit test of internal/ci or of a package importing it reads
	// "ran above as", and @ID-BIN-01, in the smoke set, does (T-277; the registry's unknown status
	// problem, T-268's line, is read by internal/slices/work's unit tests since T-265).
	reset(base)
	repo.Edit("internal/ci/ci.go", "(ran above as `%s`)", "(covered by `%s`)")
	var refactor string
	t.Run("the hooks let through a refactor that changes what itos prints", func(t *testing.T) {
		if r := hooks.PreCommit(t, "a refactor that changes what itos prints"); r.Code != 0 {
			t.Errorf("pre-commit should not see the changed output:\n%s", r.Output())
		}
		refactor = repo.Commit("refactor: shorten the covered check line\n\nTask: T-007")
		if r := hooks.PrePush(t, "that refactor", base, refactor); r.Code != 0 {
			t.Errorf("pre-push should leave the features to CI:\n%s", r.Output())
		}
	})
	// ...and fails a push's features step, whose smoke set reads that line.
	t.Run("CI's features step catches that refactor", func(t *testing.T) {
		if refactor == "" {
			t.Fatal("the refactor was not committed")
		}
		step := scratch.FeaturesStep(scratch.CIPlan(t, repo.Dir, base, refactor), features)
		if !strings.HasPrefix(step, features+" -scenarios=") {
			t.Fatalf("a push's features step is not a selection: %s", step)
		}
		if r := hooks.Gate(t, "CI's features step for a push, same refactor", step); r.Code == 0 {
			t.Error("a push's features step passed a refactor that changes what itos prints")
		}
	})
}
