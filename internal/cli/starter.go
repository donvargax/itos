package cli

// The starter itos init writes where there is no config (slice 48): small and
// commented, for a person to read and grow. The Conventional Commits types
// under itos's own header lint; a Task footer from the ledger that every type
// but feat, fix and docs needs, since itos's own registry and ledger commands
// commit docs with none (bug 12); when features/ holds feature files, a
// scenario kind whose Scenarios footer feat and fix need, with a smoke set,
// the footer required of no type while no scenario carries an ID tag (slice
// 50: most Cucumber projects tag none, and every feat would be refused with
// nothing to name); commits.since at HEAD, so no commit written before itos
// is judged; hooks.bin itos, the global launcher, as a consumer has no
// wrapper of its own; the pin, when the release server answered. One path
// scope, docs's, so a change labelled docs to skip the Task footer is
// refused; the rest are each project's own. The people file is left to its
// default, which config check only warns of.

import (
	"fmt"
	"path"
	"strings"
)

// Where the starter's files go, relative to the top, or beside the stealth
// config; the feature files are always the top's.
const (
	starterLedger   = "tasks/phase-1.yaml"
	starterRegistry = "tasks/work-items.yaml"
	starterSmoke    = "features/smoke.yaml"
	starterFeatures = "features"
)

// starter is what the starter config says.
type starter struct {
	stealth bool
	// since is HEAD's full SHA, "" in a repository with no commit.
	since string
	// pin is the newest release's version and its checksums.txt's SHA-256,
	// nil when the release server could not be asked.
	pin *[2]string
	// scenarios is whether features/ holds feature files.
	scenarios bool
	// untagged is whether none of their scenarios carries an ID tag, so the
	// Scenarios footer is required of no type.
	untagged bool
}

// config is the starter config's text.
func (s starter) config() string {
	var b strings.Builder
	if s.stealth {
		b.WriteString(`# The stealth config, written by itos init --stealth: one person's itos in a
# repository whose team does not use it. It lives in the git folder, which git
# never commits, with its ledger, its registry and its smoke set beside it, and
# the hooks are declared in the git config, so nothing tracked changes.
`)
	} else {
		b.WriteString(`# itos.yaml, written by itos init: this repository's rules for its commits,
# kept small to grow. itos config check validates it; itos config check
# --print-defaults prints every key it leaves to its default.
`)
	}
	b.WriteString("version: 1\n\n")
	switch {
	case s.pin != nil:
		fmt.Fprintf(&b, `# The itos release this repository runs: a global itos fetches it, checks it
# against these checksums and runs it. itos pin moves it to the newest.
pin:
  version: %q
  checksums: %q # the SHA-256 of the release's checksums.txt

`, s.pin[0], s.pin[1])
	case s.stealth:
		b.WriteString("# No pin: a global itos runs the newest release. itos pin pins one.\n\n")
	default:
		b.WriteString("# No pin: the itos called runs. itos pin pins the newest release.\n\n")
	}
	b.WriteString(`# The tasks: every commit but a feat, a fix or a docs names one in its Task
# footer (itos commit --task T-1), and itos task T-1 runs its checks.
ledger:
  files: "tasks/phase-{group}.yaml" # one file per phase: tasks/phase-1.yaml, …
  id: "T-\\d+"

commits:
  # The Conventional Commits types, held to commitlint's config-conventional
  # rules by itos's own lint.
  types: [feat, fix, refactor, perf, test, build, ci, chore, docs, style, revert]
  header_lint: { use: builtin }
  footers:
    Task:
      source: ledger
      required_for: [refactor, perf, test, build, ci, chore, style, revert]
      validate_for: all # a task named must be in the ledger
      read_at: commit # judged against the ledger the commit carries
`)
	switch {
	case s.scenarios && s.untagged:
		b.WriteString(`    # The scenarios a feat or a fix turns green, by their ID tags
    # (itos commit --scenarios '@ID-PAGE-01'); a @wip one is not live. No
    # scenario carries an ID tag yet, so no commit needs the footer: tag one
    # (@ID-PAGE-01 on the line above its Scenario:), then set required_for
    # to [feat, fix].
    Scenarios:
      source: { tests: scenario }
      strip_prefix: "@"
      required_for: []
      validate_for: [feat, fix]
      must_be_live: true
      read_at: commit
`)
	case s.scenarios:
		b.WriteString(`    # The scenarios a feat or a fix turns green, by their ID tags
    # (itos commit --scenarios '@ID-PAGE-01'); a @wip one is not live.
    Scenarios:
      source: { tests: scenario }
      strip_prefix: "@"
      required_for: [feat, fix]
      validate_for: [feat, fix]
      must_be_live: true
      read_at: commit
`)
	}
	if s.since != "" {
		fmt.Fprintf(&b, `  # Where verification starts: this commit and the ones before it, written
  # before itos, are never judged.
  since: %q
`, s.since)
	} else {
		b.WriteString("  # No since: the repository had no commit, so every commit is judged.\n")
	}
	// docs may touch Markdown, docs/ and, in a project, itos's data beside the
	// ledger; the stealth data is in the git folder, which no commit touches.
	data := path.Dir(starterLedger)
	touch, only := "Markdown and docs/", `"**/*.md", "docs/**"`
	if !s.stealth {
		touch = "Markdown, docs/ and itos's data in " + data + "/"
		only += fmt.Sprintf(", %q", data+"/**")
	}
	fmt.Fprintf(&b, `  # Which paths each type may touch. docs needs no Task footer, so it may
  # touch only %s; the rest, anything.
  scopes:
    docs: { only: [%s] }
`, touch, only)
	if s.scenarios {
		b.WriteString(`
# The named tests: the scenarios of the feature files under features/, each
# with an ID tag such as @ID-PAGE-01, and a smoke set naming one of each file.
tests:
  scenario:
    root: features
    id: "ID-[A-Z]+-\\d+"
    smoke: { file: features/smoke.yaml }
`)
	}
	b.WriteString(`
hooks:
  bin: itos # the hooks call the global itos, which runs the release pinned
`)
	return b.String()
}

// ledger is the ledger's first file: a commented task, and in a project the
// task its adoption commit names.
func (s starter) ledger() string {
	text := `# The ledger's phase 1: the tasks that drive every commit but a feat, a fix or
# a docs, each named in its commit's Task footer (itos commit --task T-2).
# A task:
#
#   - id: T-2
#     type: chore # one of commits.types
#     title: What it is
#     why: Why it is worth doing. # optional
#     done_when: # the checks that prove it, which itos task T-2 runs
#       - run: a command that exits 0 once it is done
`
	if s.stealth {
		return text + "[]\n"
	}
	return text + `- id: T-1
  type: chore
  title: Adopt itos
  why: >
    The commit that adds itos.yaml, this ledger and the work registry names it:
    itos commit --task T-1.
  done_when:
    - run: itos config check
`
}

// starterRegistryText is the work registry: no item, and the ledger's one
// phase, owned by nobody, so the first itos work add or task add has a phase
// to put its item in (bug 12).
const starterRegistryText = `# The work registry: who owns each phase and each piece of work, and what
# waits on what. itos work shows what can start; itos work check validates it.
# An item:
#
#   - id: T-2 # a task's ID, or a name of its own
#     title: What it is
#     phase: 1
#     owner: null # a handle, or null
#     status: todo # todo, doing, done or blocked
#     depends_on: []
phases:
  1: null # the ledger's phase 1, owned by nobody: a handle, or null
items: []
`

// starterSmokeHeader opens the smoke set.
const starterSmokeHeader = `# The smoke set: one live scenario of each feature file, which a push's CI can
# run in place of every one; itos tests smoke check scenario holds it to that.
# itos init picked each file's first: pick a fast one, central to its file.
`
