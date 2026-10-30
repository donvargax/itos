@phase-1
Feature: itos brief, the filled brief an agent starts from and its request
  The coordinator fills the same template for every brief by hand, from
  itos work show: the item, its title, its scenarios and their feature file,
  the repository's path (slice-204, was p1-guide-brief-filled). A gpt-6-luna
  agent read the template's red-first line as "the steps must already exist"
  and stopped on undefined steps (bug-111). And the deterministic coordinator
  (docs/plans/v9-coordinator.md, the LLM steps' contracts) wants the brief as
  data: a request id that names exactly what the agent was given, the spec
  commit it was made from, a manifest of what it reads and what was left out.

  itos brief <id> --role <role> prints the brief for one role, drafter,
  implementer or reviewer, filled from the item: the implementer's is itos
  guide brief's template (which stays as it is), the drafter's and the
  reviewer's are templates shipped beside it, not guide names. Filled in: the
  item's id, its kind and title, the repository's top as git prints it and
  its folder's name as the project, the ids of the scenarios tagged for a
  slice or a bug and their feature file, a task's checks from its ledger
  entry, its refs and its dependencies with their statuses, and the files
  the agent reads: AGENTS.md, the item's feature file (a task's ledger file),
  and for each ref the file it names, or the doc.go of the package folder it
  names. A ref naming neither is left out, and said so. A repository with no
  AGENTS.md is briefed all the same, that read going to omitted with the
  reason not in the tree at HEAD. Scenarios in several files, or live and
  wip mixed, are one sentence listing each file's ids, then the live ones:
  "The scenarios are @A-1 and @A-2 in a.feature and @B-1 in b.feature; @A-2
  is live." (all of one kind: "all `@wip`", "all live"). The repository's
  own brief lines, the body of the "## What a brief adds here" section of
  the file guide.orchestrating names (docs/ORCHESTRATING.md by default),
  follow the filled template as written; no file or no section adds nothing.
  Today that section of this repository's docs/ORCHESTRATING.md speaks to
  the coordinator who fills the brief; the slice includes a docs commit, the
  implementer's on this item's pull request, that rewrites it to address the
  agent and drops the line saying the handoff replaces the template's
  wait-for-CI line. What a person must add stays a marked placeholder, never
  invented: <settled decisions, filled by the coordinator>, <neighbouring
  scenarios and checks, filled by the coordinator> and <the item's own
  questions, filled by the coordinator>, in place of the template's own
  angle-bracket placeholders. The red-first paragraph says that writing the
  missing step definitions is the test commit's job, and that the only
  reasons to stop are a scenario that passes before the work and one that
  fails in its setup where no step definition can fix it. Each paragraph of
  the filled brief prints as one line, so a sentence of it is one substring;
  the new templates go through guide.Text's normalisation, CRLF to LF, as
  the shipped guides do.

  Everything is read from HEAD's tree: the registry, the tagged-scenario
  scan, the ledger, the ORCHESTRATING section and the reads' blobs, never
  the working tree or the index.

  --json gives the request:
  {"schema":1,"ok":true,"item","role","request","spec":{"commit"},"reads":[{"path","blob","reason","required"}],"omitted":[{"path","reason"}],"brief"},
  brief the text output, byte for byte; spec.commit the last commit on
  HEAD's first-parent history touching a feature file holding a scenario
  tagged for the item (a task's ledger file), null when nothing is tagged
  yet; each read's blob the file's git blob id at HEAD, required true for
  AGENTS.md and the spec's file, false for a ref; request "sha256:" and the
  hex digest of these LF-terminated lines: the item, the role, the spec
  commit's full SHA (or "-" when null), then one "<path> <blob>" line per
  read, in the reads' order, so another tool can recompute it, the same
  inputs give the same request and any change a new one. The attempt number
  and the base revision are not in it: itos attempt submit binds them. It
  reads, writes nothing, commits nothing and asks no forge.

  Refused, exit 1, {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}:
  an id the registry does not have (work-unknown-item, as work show), a role
  it does not know (brief-unknown-role, the fix naming the three), an idea
  (brief-idea, the fix naming its promotion), and for the implementer and
  the reviewer an item nothing is tagged for (brief-no-spec, the fix naming
  the drafter's brief); the drafter briefs it, its spec commit null. A
  missing --role is a usage error, exit 2.

  Background:
    Given a repository whose ledger has the task "T-001"
    And the committed file "AGENTS.md" holding "Stage only your own files."

  # The registry is written whole, for a title, a dependency and refs at
  # once. The title and the dependency's status are asserted exactly, so a
  # build that printed the id for the title, or left the placeholders in,
  # fails. The repository has no docs/ORCHESTRATING.md, so nothing follows
  # the template.
  @ID-BRIEF-01 @slice-204
  Scenario: The implementer's brief is the template filled from the item
    Given the committed file "tasks/work-items.yaml" holding the lines:
      """
      phases: { 1: null }
      items:
        - { id: slice-8, title: Earlier, phase: 1, owner: null, status: todo, depends_on: [] }
        - id: slice-9
          title: Say the next step
          phase: 1
          owner: null
          status: todo
          depends_on: [slice-8]
          refs: [internal/foo]
      """
    And the committed file "internal/foo/doc.go" holding "// Package foo says the next step."
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    When itos runs "brief slice-9 --role implementer"
    Then itos exits with code 0
    And its output says "Implement slice-9 (a slice) of <folder>, in the repository at <top>: Say the next step.", the repository's folder name and top filled in
    And its output says "The scenarios are @ID-A-01 in features/tagged.feature, all `@wip`."
    And its output says "It depends on slice-8 (todo)."
    And its output says "itos work show slice-9"
    And its output says "read AGENTS.md, features/tagged.feature and internal/foo/doc.go"
    And its output says "<settled decisions, filled by the coordinator>"
    And its output says "<neighbouring scenarios and checks, filled by the coordinator>"
    And its output says "<the item's own questions, filled by the coordinator>"
    And its output says "itos commit --item slice-9"
    And its output does not say "## Specifying"
    And its output does not say "<item>"
    And its output does not say "<absolute path>"
    And its output does not say "<the neighbouring scenarios and checks;"
    And its output does not say "What a brief adds here"

  # The sentence the gpt-6-luna agent needed (bug-111): an undefined step is
  # the test commit's work, not a reason to stop. The template's old line
  # alone does not say it.
  @ID-BRIEF-02 @slice-204
  Scenario: The red-first paragraph says the missing step definitions are the test commit's job
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    When itos runs "brief slice-9 --role implementer"
    Then itos exits with code 0
    And its output says "Writing the missing step definitions is the test commit's job: an undefined step is work, not a reason to stop."
    And its output says "Stop only for a scenario that passes before the work, or one that fails in its setup where no step definition can fix it, and propose the change."

  # The checks are quoted so a build that listed the ledger entry's title or
  # why instead fails; the ledger file is the task's spec, so it is read.
  @ID-BRIEF-03 @slice-204
  Scenario: A task's brief names its ledger checks and reads its ledger file
    Given the work registry has the task "T-001" owned by nobody with the status "todo" and no why
    And the task "T-001" has the committed check "go vet ./..."
    When itos runs "brief T-001 --role implementer"
    Then itos exits with code 0
    And its output says "Implement T-001 (a task) of"
    And its output says "The task is T-001, whose checks are `go vet ./...`."
    And its output says "read AGENTS.md and tasks/phase-1.yaml"
    When itos runs "brief T-001 --role implementer --json"
    Then itos exits with code 0
    And its JSON's "reads" entry whose "path" is "tasks/phase-1.yaml" has "reason" "the task's ledger entry"
    And its JSON's "reads" entry whose "path" is "tasks/phase-1.yaml" has "required" true

  # A bug's scenario is tagged @bug-<n>; one live scenario is said to be
  # live, not `@wip`.
  @ID-BRIEF-04 @slice-204
  Scenario: A bug's brief names the scenarios tagged for it
    Given the work registry has the bug "bug-7" owned by "someone" with the status "doing"
    And the committed feature file "features/fixes.feature" with the live scenario "@ID-F-01" tagged "@bug-7"
    When itos runs "brief bug-7 --role implementer"
    Then itos exits with code 0
    And its output says "Implement bug-7 (a bug) of"
    And its output says "The scenarios are @ID-F-01 in features/fixes.feature, all live."

  # The section's body follows the filled template as written; the other
  # sections of the notes stay out, and the heading itself is not printed.
  @ID-BRIEF-05 @slice-204
  Scenario: The repository's own brief lines follow the template as written
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And the committed file "docs/ORCHESTRATING.md" holding the lines:
      """
      # Our notes

      ## Independent work

      Repairs go first.

      ## What a brief adds here

      - **Reads:** `PLAN.md` and the decision records.
      - **The neighbours:** run the item's areas before pushing.

      ## Lessons

      The inbox is read first.
      """
    When itos runs "brief slice-9 --role implementer"
    Then itos exits with code 0
    And its output says "propose the change." before "- **Reads:** `PLAN.md` and the decision records."
    And its output says "- **The neighbours:** run the item's areas before pushing."
    And its output does not say "Repairs go first."
    And its output does not say "The inbox is read first."
    And its output does not say "## What a brief adds here"

  # The drafter's and the reviewer's templates ship beside the implementer's
  # and are not guide names: GUIDE-19's list of procedures holds. A slice
  # with no scenario yet is the drafter's to specify, so its brief says so
  # and its spec commit is null; the implementer and the reviewer have
  # nothing to work from and are refused.
  @ID-BRIEF-06 @slice-204
  Scenario: Each role has its template, and only the drafter is briefed on an item with no spec
    Given the work registry has the item "slice-9" owned by nobody with the status "todo" and the title "Say the next step"
    When itos runs "brief slice-9 --role drafter"
    Then itos exits with code 0
    And its output says "Specify slice-9 (a slice) of <folder>, in the repository at <top>: Say the next step.", the repository's folder name and top filled in
    And its output says "No scenario is tagged @slice-9 yet."
    And its output says "<settled decisions, filled by the coordinator>"
    When itos runs "brief slice-9 --role drafter --json"
    Then itos exits with code 0
    And its JSON's "role" is "drafter"
    And its JSON's "spec" is {"commit": null}
    And the "path" of each entry of its JSON's "reads", in order, is "AGENTS.md"
    When itos runs "brief slice-9 --role implementer --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "brief-no-spec"'s "message" says "slice-9"
    And its JSON's problem "brief-no-spec"'s "fix" says "itos brief slice-9 --role drafter"
    When itos runs "brief slice-9 --role reviewer --json"
    Then itos exits with code 1
    And its JSON's problem "brief-no-spec"'s "message" says "slice-9"
    Given a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    When itos runs "brief slice-9 --role reviewer"
    Then itos exits with code 0
    And its output says "Review slice-9 (a slice) of <folder>, in the repository at <top>: Say the next step.", the repository's folder name and top filled in
    And its output says "The scenarios are @ID-A-01 in features/tagged.feature, all `@wip`."
    When itos runs "guide review"
    Then itos exits with code 2
    And its output says "brief, commit, decide, land, push, repair"

  # The README commit on top is none of the item's facts: the spec commit
  # stays the scenario's, not HEAD. The blob of each read is checked against
  # git's, so a hash of the content under another scheme fails. A ref that
  # is a file (docs/plan.md) is read as it is; only internal/foo has a
  # doc.go: internal/bar's other file does not stand in for one, and
  # docs/missing.md is no file at all.
  @ID-BRIEF-07 @slice-204
  Scenario: The request names the spec commit, the reads with their blobs and what was left out
    Given the committed file "tasks/work-items.yaml" holding the lines:
      """
      phases: { 1: null }
      items:
        - id: slice-9
          title: Say the next step
          phase: 1
          owner: null
          status: todo
          depends_on: []
          refs: [internal/foo, docs/plan.md, internal/bar, docs/missing.md]
      """
    And the committed file "internal/foo/doc.go" holding "// Package foo says the next step."
    And the committed file "docs/plan.md" holding "# The plan"
    And the committed file "internal/bar/bar.go" holding "package bar"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And the commit "chore: tidy the readme" on top of it
    When itos runs "brief slice-9 --role implementer"
    Then itos exits with code 0
    And its output says "read AGENTS.md, features/tagged.feature, internal/foo/doc.go and docs/plan.md"
    And its output says "internal/bar has no doc.go at HEAD"
    And its output says "docs/missing.md is not in the tree at HEAD"
    And stdout is saved as "text"
    When itos runs "brief slice-9 --role implementer --json"
    Then itos exits with code 0
    And its JSON's "ok" is true
    And its JSON's "item" is "slice-9"
    And its JSON's "role" is "implementer"
    And its JSON's "spec.commit" names the commit "test: a tagged scenario"
    And the "path" of each entry of its JSON's "reads", in order, is "AGENTS.md, features/tagged.feature, internal/foo/doc.go, docs/plan.md"
    And its JSON's "reads" entry whose "path" is "AGENTS.md" has "blob" the git blob id of "AGENTS.md" at HEAD
    And its JSON's "reads" entry whose "path" is "AGENTS.md" has "reason" "the repository's agent rules"
    And its JSON's "reads" entry whose "path" is "AGENTS.md" has "required" true
    And its JSON's "reads" entry whose "path" is "features/tagged.feature" has "blob" the git blob id of "features/tagged.feature" at HEAD
    And its JSON's "reads" entry whose "path" is "features/tagged.feature" has "reason" "the item's scenarios"
    And its JSON's "reads" entry whose "path" is "features/tagged.feature" has "required" true
    And its JSON's "reads" entry whose "path" is "internal/foo/doc.go" has "blob" the git blob id of "internal/foo/doc.go" at HEAD
    And its JSON's "reads" entry whose "path" is "internal/foo/doc.go" has "reason" "the item's ref internal/foo"
    And its JSON's "reads" entry whose "path" is "internal/foo/doc.go" has "required" false
    And its JSON's "reads" entry whose "path" is "docs/plan.md" has "blob" the git blob id of "docs/plan.md" at HEAD
    And its JSON's "reads" entry whose "path" is "docs/plan.md" has "reason" "the item's ref docs/plan.md"
    And its JSON's "reads" entry whose "path" is "docs/plan.md" has "required" false
    And the "path" of each entry of its JSON's "omitted", in order, is "internal/bar, docs/missing.md"
    And its JSON's "omitted" entry whose "path" is "internal/bar" has "reason" "no doc.go at HEAD"
    And its JSON's "omitted" entry whose "path" is "docs/missing.md" has "reason" "not in the tree at HEAD"
    And its JSON's "brief" equals the stdout saved as "text"

  # The README commit changes HEAD and nothing the brief is made from: a
  # request of HEAD, or of the whole tree, differs.
  @ID-BRIEF-08 @slice-204
  Scenario: The same inputs give the same request
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    When itos runs "brief slice-9 --role implementer --json"
    Then itos exits with code 0
    And its JSON's "request" starts with "sha256:" followed by 64 lowercase hex digits
    And the JSON's "request" is saved as "first"
    When the commit "chore: tidy the readme" on top of it
    And itos runs "brief slice-9 --role implementer --json"
    Then itos exits with code 0
    And its JSON's "request" matches the one saved as "first"

  # Three changes, each to one input: the role, the spec (a second feature
  # file tagged for the item, whose commit becomes the spec commit) and a
  # read's blob (AGENTS.md reworded). A request of the item and role alone,
  # or of the spec commit alone, passes one of them and fails another.
  @ID-BRIEF-09 @slice-204
  Scenario: A change to the role, the spec or a read gives a new request
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    When itos runs "brief slice-9 --role implementer --json"
    Then itos exits with code 0
    And the JSON's "request" is saved as "implementer"
    When itos runs "brief slice-9 --role reviewer --json"
    Then itos exits with code 0
    And its JSON's "request" differs from the one saved as "implementer"
    When the committed feature file "features/b.feature" with the live scenario "@ID-B-01" tagged "@slice-9"
    And itos runs "brief slice-9 --role implementer --json"
    Then itos exits with code 0
    And its JSON's "spec.commit" names the commit "test: specify features/b.feature"
    And its output says "The scenarios are @ID-B-01 in features/b.feature and @ID-A-01 in features/tagged.feature; @ID-B-01 is live."
    And its JSON's "request" differs from the one saved as "implementer"
    And the JSON's "request" is saved as "two files"
    When the committed file "AGENTS.md" holding "Stage only your own files, by path."
    And itos runs "brief slice-9 --role implementer --json"
    Then itos exits with code 0
    And its JSON's "spec.commit" names the commit "test: specify features/b.feature"
    And its JSON's "request" differs from the one saved as "two files"

  @ID-BRIEF-10 @slice-204
  Scenario: An unknown id, an unknown role and a missing role are refused
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    When itos runs "brief slice-7 --role implementer --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "work-unknown-item"'s "message" says "slice-7"
    When itos runs "brief slice-9 --role tester --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "brief-unknown-role"'s "message" says "tester"
    And its JSON's problem "brief-unknown-role"'s "fix" says "drafter, implementer or reviewer"
    When itos runs "brief slice-9 --role tester"
    Then itos exits with code 1
    And its output says "tester"
    And its output says "drafter, implementer or reviewer"
    When itos runs "brief slice-9"
    Then itos exits with code 2
    And its output says "--role"

  # An idea has no spec and no kind to brief for; what it becomes is the
  # person's call, so the fix names the promotion, not a role.
  @ID-BRIEF-11 @slice-204
  Scenario: An idea is refused, its fix naming the promotion
    Given the work registry has the idea "p1-next" owned by nobody
    When itos runs "brief p1-next --role drafter --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "brief-idea"'s "message" says "p1-next"
    And its JSON's problem "brief-idea"'s "fix" says "itos work promote p1-next --kind slice"
    When itos runs "brief p1-next --role drafter"
    Then itos exits with code 1
    And its output says "itos work promote p1-next --kind slice"

  # The config is HEAD's too (the review of the build, round 1): the working
  # tree's itos.yaml, staged and no longer a config at all, changes no
  # brief, where a config read from the working tree would refuse the run.
  @ID-BRIEF-13 @wip @slice-204
  Scenario: The brief is a function of HEAD, an uncommitted change to the config aside
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And an unrelated change to "itos.yaml" is staged
    When itos runs "brief slice-9 --role implementer --json"
    Then itos exits with code 0
    And its JSON's "item" is "slice-9"
    And the "path" of each entry of its JSON's "reads", in order, is "AGENTS.md, features/tagged.feature"

  # The saved state is every file of the clone, its git folder included,
  # so HEAD, the index and anything itos might keep in .git/itos are
  # compared. A gh on the PATH records any call, so asking the forge shows.
  @ID-BRIEF-12 @slice-204
  Scenario: itos brief writes nothing, commits nothing and asks no forge
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And a recording gh on the PATH, which opens pull requests as "https://github.com/o/r/pull/1"
    And the repository state is saved for comparison
    When itos runs "brief slice-9 --role implementer"
    Then itos exits with code 0
    When itos runs "brief slice-9 --role implementer --json"
    Then itos exits with code 0
    And the repository files index and HEAD match the saved state
    And git reports no change to the working tree or the index
    And the recording gh was never called
