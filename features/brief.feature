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
  implementer or reviewer, filled from the item: for a slice or a bug, the
  implementer's is itos guide brief's template (which stays as it is), the
  drafter's and the reviewer's are templates shipped beside it, not guide
  names; a task's text for each role is its own (below). Filled in: the
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

  The drafter's brief names the commit type its spec goes in under, and the
  flag of itos commit that goes with it (slice-221). It said docs and --item
  whatever the config, and a project whose commits.scopes.docs leaves out
  features/ refuses docs (itos commit check-paths --type docs): a drafter
  following the brief stopped twice on it, and its driver had to tell its
  agents the right type. The flag is the one the config has: --item where it
  has a footer of the work registry, else --task for a task where it has a
  ledger footer, else plain itos commit, as itos commit refuses a flag whose
  footer the config lacks. On a project with a ci: section, --task also makes
  CI run the task's checks once the spec's commit is pushed, as it does for any
  commit naming a task. The type is docs when the config lets docs carry the
  spec's files; otherwise the first of commits.types that may carry them and
  that the registry's footer (Item:) stands in for, or, where the config has no
  such footer, for a task, that its ledger footer is required for. So a slice
  or a bug with no such footer, one whose in_place_of names nothing, or a task
  with neither footer, has docs alone. A type may carry them when commits.types
  has it, when the path
  rules itos commit check-paths judges with let it touch them, and when the
  hook's footer rules call nothing missing from a commit made with that flag,
  a required footer of free text among them, as the brief's sentence writes
  none. The spec's files are those of the scenarios tagged for the item, else
  a feature file under the root of the first Gherkin kind of tests, as the
  scan for tagged scenarios reads them; a task's are its ledger file, else the
  one its phase would have. Where no type may, or a slice or a bug has no such
  kind, the brief says so and tells the drafter to stop and report, so that it
  never names a type its config refuses by its types, path rules and footers.
  It does not judge the hook's staged range commands, which need the staged
  files a brief does not have. With no commits section there is nothing to
  judge by, and the brief keeps docs with plain itos commit, even with no
  Gherkin kind, where docs wins over the stop; the hook's path rules call such
  a config an error whatever the type, so this promises nothing of that
  commit. The config is the one the brief reads, HEAD's or
  --at's. The request stays a digest of the item, the role, the spec commit
  and the reads: a config changing the type changes the brief's text and not
  its request, as the repository's own brief lines do.

  The choice is a function, so the scenarios hold what reaches it from the
  command line (ID-BRIEF-28, 33, 34, 36 and 40) and the rest are the rows of
  its table test, TestSpecCommitType in internal/slices/brief, each giving a
  config and an item and holding the type and flag chosen, or the stop.
  ID-BRIEF-29 to 32, 35, 37 to 39 and 41 name rows here and are not scenarios,
  and their IDs stay spent:
  - ID-BRIEF-29: nothing tagged, the kind's root acceptance/, docs may touch
    features/** alone: test, so a build judging features/ fails.
  - ID-BRIEF-30: types [feat, refactor, build, test, chore, docs], in_place_of
    listing chore first, refactor's never barring the file, build's must_touch
    unmet (only and never alone, TypesFor, let it through), feat not stood in
    for: test.
  - ID-BRIEF-31: commits.types without docs, and no path rule on it: test.
  - ID-BRIEF-32: docs may touch the file and needs a Task: footer Item: does
    not stand in for: test.
  - ID-BRIEF-35: a task whose ledger file docs and test may not touch: chore.
  - ID-BRIEF-37: a task with no ledger entry, chore's only naming the file of
    its phase, so ledger.files with {group} still in it fails: chore.
  - ID-BRIEF-38: over the task's text, no registry footer, docs barred and the
    types its ledger footer is required for barred too: the stop, with no word
    of scenarios in it, naming `itos commit --task T-001`.
  - ID-BRIEF-39: this repository's itos.yaml, read from the tree so that it
    cannot drift: a slice's sentence stays whole, docs and --item, though
    commits.types has test before docs.
  - ID-BRIEF-41: no commits section: docs, plain itos commit, with a Gherkin
    kind and with none, where docs wins over the stop.
  - A task with no registry footer: docs barred, a type its ledger footer is
    not required for passed over: the first it is, with --task.
  - A task with neither a registry nor a ledger footer, docs let touch its file:
    docs, plain itos commit, never --task.
  - An Item footer with no in_place_of, docs barred: the stop.
  - A type that requires a footer of free text: passed over for the next.
  - No Gherkin kind (no tests, or a command adapter alone), under a commits
    section: a slice's stop.
  - A command kind listed before the Gherkin one, docs may touch the command
    kind's root alone: test, judged under the Gherkin kind's root.
  A unit assertion holds two configs choosing different types to the same
  request and to different texts, so it cannot pass with the type left out of
  the brief. TestSliceAndBugBriefsGolden, which slice-220 adds, changes only where
  its config makes it: run it under a config with a registry footer that lets
  docs touch the file, and its drafter's commit sentence stays byte for byte.

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

  A task has no scenarios: its spec is its ledger entry's why and done_when
  checks. Its brief gave it the scenario workflow all the same (red first
  through @wip scenarios, step definitions, "scenarios turned green"), so an
  implementer had to be told to read the template differently, and a
  consumer reported the same of the guides (slice-220, #237). The item's
  kind, as next reads it (its kind key, else its id), chooses the role's
  text: a slice's and a bug's stay byte for byte as they are, and only a
  task's change. What a person must add stays a marked placeholder in a
  task's brief too. itos guide work and itos guide brief say what red first
  means for a task in one same line, outside the brief procedure's quoted
  block, which is the implementer's template. This closes the idea
  p2-guide-red-first-tasks.

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

  # A task's why settles its design and its checks are the proof. Red first
  # is running them before the work; a check that cannot fail before it is
  # said so, as a post-push check cannot. The commit flag is said because the
  # scenario workflow's red-first paragraph, which held --item, is gone. The
  # neighbours' placeholder keeps its one mention of scenarios: placeholders
  # stay, as they are, and a task's change can reach a scenario. The last
  # step is case-insensitive, so any other mention of them fails, "Scenarios"
  # as "scenario".
  @ID-BRIEF-20 @slice-220
  Scenario: A task's implementer is told its why settles the design and red first is running its checks
    Given the work registry has the task "T-001" owned by nobody with the status "todo" and no why
    And the task "T-001" has the committed check "go vet ./..."
    When itos runs "brief T-001 --role implementer"
    Then itos exits with code 0
    And its output says "Implement T-001 (a task) of"
    And its output says "The task is T-001, whose checks are `go vet ./...`. Take it with `itos work take T-001`"
    And its output says "read AGENTS.md and tasks/phase-1.yaml"
    And its output says "The task's why in its ledger entry settles the design, and its checks are the proof."
    And its output says "Red first: run the task's checks with `itos task T-001` before the work and see each fail; where one cannot fail before the work, say so."
    And its output says "A new program's own tests fail against a stub."
    And its output says "itos commit --task T-001"
    And its output says "each check and how it failed before the work"
    And its output says "<settled decisions, filled by the coordinator>"
    And its output says "<neighbouring scenarios and checks, filled by the coordinator>"
    And its output says "<the item's own questions, filled by the coordinator>"
    And its output does not say "<item>"
    And its output does not say "@wip"
    And its output does not say "step definition"
    And its output does not say "scenario" outside "<neighbouring scenarios and checks, filled by the coordinator>"

  # The spec is the entry's why and its checks, so nothing is minted or
  # tagged. No command edits an entry: itos task add mints a new id and item,
  # which is wrong for an item the registry already has, and itos task edit
  # is only the idea p1-task-edit. So the brief says by hand, until it is
  # built. The settled decisions go in the entry's why, where work edit --note
  # refuses a task. The one mention of scenarios is the sentence saying there
  # are none to write: a leftover such as "Tag each scenario" fails.
  @ID-BRIEF-21 @slice-220
  Scenario: A task's drafter is told its spec is the ledger entry, written by hand, with nothing to mint
    Given the work registry has the task "T-001" owned by nobody with the status "todo" and no why
    And the task "T-001" has the committed check "go vet ./..."
    When itos runs "brief T-001 --role drafter"
    Then itos exits with code 0
    And its output says "Specify T-001 (a task) of"
    And its output says "The task is T-001, whose checks are `go vet ./...`. Your job"
    And its output says "read AGENTS.md and tasks/phase-1.yaml"
    And its output says "The spec is the task's ledger entry: its why and its done_when checks, each able to fail before the work. There are no scenarios to write and no IDs to mint."
    And its output says "No command edits a ledger entry, and `itos task add` mints another task: write this one's by hand in its group's ledger file."
    And its output says "Every settled decision goes into the entry's why"
    And its output says "the header `docs: specify T-001, <short rule>`, the body the problem, the rule and how each check fails today."
    And its output says "each check and how it fails before the work"
    And its output says "<settled decisions, filled by the coordinator>"
    And its output says "<the item's own questions, filled by the coordinator>"
    And its output does not say "<item>"
    And its output does not say "@wip"
    And its output does not say "step definition"
    And its output does not say "itos tests next-id"
    And its output does not say "feature file"
    And its output does not say "scenario" outside "There are no scenarios to write"

  # The drafter is the one role briefed on a task with no entry: T-002 is in
  # the registry and the ledger has only the Background's T-001. So it is
  # told to write the entry, in the file its reads cannot name yet.
  @ID-BRIEF-22 @slice-220
  Scenario: A task's drafter is told the same when the task has no ledger entry yet
    Given the work registry has the task "T-002" owned by nobody with the status "todo" and no why
    When itos runs "brief T-002 --role drafter"
    Then itos exits with code 0
    And its output says "T-002 has no ledger entry yet. Your job"
    And its output says "No command edits a ledger entry, and `itos task add` mints another task: write this one's by hand in its group's ledger file."
    And its output does not say "itos tests next-id"
    And its output does not say "scenario" outside "There are no scenarios to write"

  # A task can have an entry and no check: T-001 here has the Background's
  # entry alone, a state ID-BRIEF-22's task with no entry cannot show. Its
  # drafter still gets the task's text, for it is the one to write the checks.
  @ID-BRIEF-27 @slice-220
  Scenario: A task's drafter is told the same when the task has no checks yet
    Given the work registry has the task "T-001" owned by nobody with the status "todo" and no why
    When itos runs "brief T-001 --role drafter"
    Then itos exits with code 0
    And its output says "which has no checks yet. Your job"
    And its output says "There are no scenarios to write"
    And its output does not say "scenario" outside "There are no scenarios to write"

  # A task has checks to judge, not scenarios: a verdict on each, whether it
  # can fail before the work and pass after. This template has no neighbours'
  # placeholder, so the word is gone altogether: the text the last step
  # excepts holds no such word, and the step's case-insensitive reading is
  # what the plain step lacks.
  @ID-BRIEF-23 @slice-220
  Scenario: A task's reviewer judges each check, not scenarios
    Given the work registry has the task "T-001" owned by nobody with the status "todo" and no why
    And the task "T-001" has the committed check "go vet ./..."
    When itos runs "brief T-001 --role reviewer"
    Then itos exits with code 0
    And its output says "Review T-001 (a task) of"
    And its output says "The task is T-001, whose checks are `go vet ./...`. Your job"
    And its output says "read AGENTS.md and tasks/phase-1.yaml"
    And its output says "For each check, say whether it can fail before the work and pass after a correct one"
    And its output says "for each check, whether it can fail before the work and pass after a correct one"
    And its output says "<settled decisions, filled by the coordinator>"
    And its output says "<the item's own questions, filled by the coordinator>"
    And its output does not say "<item>"
    And its output does not say "scenario" outside "<settled decisions, filled by the coordinator>"

  # The kind is read as next reads it: an item of no kind by its id, so
  # T-001 under ledger.id is a task, and a kind key over the id, so T-002 is
  # a slice whatever its id looks like. A build reading only the key fails
  # the first, one reading only the id the second.
  @ID-BRIEF-24 @slice-220
  Scenario: The item's kind, as next reads it, chooses the text, not its id alone
    Given the committed file "tasks/work-items.yaml" holding the lines:
      """
      phases: { 1: null }
      items:
        - { id: T-001, title: Tidy, phase: 1, owner: null, status: todo, depends_on: [] }
        - { id: T-002, title: Sliced, phase: 1, owner: null, status: todo, kind: slice, depends_on: [] }
      """
    And the task "T-001" has the committed check "go vet ./..."
    When itos runs "brief T-001 --role drafter"
    Then itos exits with code 0
    And its output says "Specify T-001 (a task) of"
    And its output says "There are no scenarios to write and no IDs to mint."
    And its output does not say "Mint every scenario ID"
    When itos runs "brief T-002 --role drafter"
    Then itos exits with code 0
    And its output says "Specify T-002 (a slice) of"
    And its output says "Mint every scenario ID with `itos tests next-id`"
    And its output does not say "There are no scenarios to write"

  # Passes before the work, on purpose: a slice's and a bug's briefs do not
  # change. One clause of each paragraph a task's text replaces, for every
  # role, so a build that sends a bug the task's text (a kind check on
  # "slice"), or puts the guides' new line inside the brief procedure's
  # quoted block, the implementer's template, fails. Whole paragraphs would
  # become released cases, though ADR-0054 exempts guide text, so byte
  # equality is a unit test's: TestSliceAndBugBriefsGolden in
  # internal/slices/brief, each role's brief of a slice and of a bug against
  # the text before this slice, which catches what no clause here does, such
  # as removing "start no agents" or "close the item with `itos work done`".
  # slice-221 will run that golden under a config with a registry footer that
  # lets docs touch the file.
  # The drafter's commit paragraph is left to slice-221, which changes it.
  @ID-BRIEF-25 @slice-220 @guard
  Scenario Outline: A slice's and a bug's briefs keep the scenario workflow
    Given the work registry has the item "<item>" owned by nobody with the status "todo"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @<item>"
    When itos runs "brief <item> --role implementer"
    Then itos exits with code 0
    And its output says "Implement <item> (<kind>) of"
    And its output says "Red first: commit the step definitions the scenarios need alone"
    And its output says "each scenario's failing step before the work, the scenarios turned green"
    And its output does not say "red first is running"
    And its output does not say "The task's why"
    When itos runs "brief <item> --role drafter"
    Then itos exits with code 0
    And its output says "Specify <item> (<kind>) of"
    And its output says "Your job is the spec alone: the `@wip` scenarios the item needs"
    And its output says "Read in full the feature file the scenarios go in"
    And its output says "Every settled decision goes into the scenarios or the comments above them, never re-decided"
    And its output says "Mint every scenario ID with `itos tests next-id`"
    And its output says "each scenario's ID, title and failing step before the work"
    And its output does not say "There are no scenarios to write"
    When itos runs "brief <item> --role reviewer"
    Then itos exits with code 0
    And its output says "Review <item> (<kind>) of"
    And its output says "Read what the change must do before what it does: the scenarios"
    And its output says "For each scenario, say whether it can fail before the work"
    And its output says "for each scenario, whether it can fail before the work"
    And its output does not say "For each check"

    Examples:
      | item    | kind    |
      | slice-9 | a slice |
      | bug-7   | a bug   |

  # The guides are read before a brief and are the same in every repository,
  # so the line says nothing a repository could change. Both are asked for it
  # (#237); neither scenario-driven line goes. These live here, not in
  # guide.feature, with the rest of what this slice says of red first for a
  # task.
  @ID-BRIEF-26 @slice-220
  Scenario Outline: itos guide <guide> says what red first means for a task
    When itos runs "guide <guide>"
    Then itos exits with code 0
    And its output says "For a task, red first is running its checks before the work and seeing each fail, saying so where one cannot; a new program's own tests fail against a stub.", whatever its line breaks
    And its output says "**Red first.**"

    Examples:
      | guide |
      | work  |
      | brief |

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

  # The case that stopped a drafter: docs may not touch the feature files, so
  # the spec goes in as test, with Item: in place of Task:. The header is
  # <type>: specify <item>, <short rule>, the rest of the sentence as it was.
  # The two check-paths runs hold the brief to the judgement it shares: test
  # taken, docs refused, by the very config it read. A slice and a bug alike:
  # both use the one drafter's text, and slice-220 gives a task its own.
  @ID-BRIEF-28 @wip @slice-221
  Scenario Outline: Where docs may not touch the feature files, a <kind>'s drafter is told test
    Given the work registry has the item "<item>" owned by nobody with the status "todo"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @<item>"
    And the committed file "itos.yaml" holding the lines:
      """
      version: 1
      ledger:
        files: "tasks/phase-{group}.yaml"
        id: "T-\\d+"
      commits:
        types: [feat, fix, refactor, perf, test, build, ci, chore, docs, style, revert]
        footers:
          Task:
            source: ledger
            required_for: [refactor, perf, test, build, ci, chore, style, revert]
            validate_for: all
            read_at: commit
          Item:
            source: registry
            validate_for: all
            read_at: commit
            in_place_of: { Task: [test, docs, chore] }
        scopes:
          docs: { only: ["**/*.md", "docs/**", "tasks/**"] }
      tests:
        scenario: { root: features, id: "ID-[A-Z]+-\\d+" }
      """
    When itos runs "brief <item> --role drafter"
    Then itos exits with code 0
    And its output says "Commit with `itos commit --item <item>`: the header `test: specify <item>, <short rule>`, the body the problem, the rule and each scenario's failing step. Push with `itos push`."
    And its output does not say "`docs: specify"
    When itos runs "commit check-paths --type test features/tagged.feature"
    Then itos exits with code 0
    When itos runs "commit check-paths --type docs features/tagged.feature"
    Then itos exits with code 1

    Examples:
      | kind  | item    |
      | slice | slice-9 |
      | bug   | bug-7   |

  # No type may: docs is barred and no footer of the registry stands in for
  # any other, the commits section itos init writes today (the idea
  # p2-starter-item-footer would change that). So the brief says so, and the
  # drafter stops before it writes a spec it could not commit. With no such
  # footer, --item is a usage error, so the sentence says plain itos commit. The
  # commit and push sentences go with the type they would have named; the rest
  # of the paragraph stays. It is a brief, not a refusal: exit 0, as a drafter
  # is the one to report it.
  @ID-BRIEF-33 @wip @slice-221
  Scenario: Where no type may carry the spec, the drafter is told to stop and report
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the committed file "itos.yaml" holding the lines:
      """
      version: 1
      ledger:
        files: "tasks/phase-{group}.yaml"
        id: "T-\\d+"
      commits:
        types: [feat, fix, refactor, perf, test, build, ci, chore, docs, style, revert]
        footers:
          Task:
            source: ledger
            required_for: [refactor, perf, test, build, ci, chore, style, revert]
            validate_for: all
            read_at: commit
        scopes:
          docs: { only: ["**/*.md", "docs/**", "tasks/**"] }
      tests:
        scenario: { root: features, id: "ID-[A-Z]+-\\d+" }
      """
    When itos runs "brief slice-9 --role drafter"
    Then itos exits with code 0
    And its output says "No commit type of this repository's config can carry this spec: none may touch the spec's files and be committed with `itos commit`."
    And its output says "Stop and report that before writing the spec; do not commit it under another type."
    And its output says "Do the work yourself; start no agents."
    And its output does not say "the header `"
    And its output does not say "specify slice-9,"
    And its output does not say "Push with `itos push`"

  # The config the brief reads, never the working tree's. HEAD bars docs and
  # the staged file lets it, so a build loading the working tree's config for
  # the rule alone names docs; HEAD~1 is the config the feature file's step
  # committed, which lets it too, so a build ignoring --at for the rule names
  # test there. The staged file is the working tree's and the index's both.
  @ID-BRIEF-34 @wip @slice-221
  Scenario: The type comes from HEAD's config or --at's, never the working tree's
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And the committed file "itos.yaml" holding the lines:
      """
      version: 1
      ledger:
        files: "tasks/phase-{group}.yaml"
        id: "T-\\d+"
      commits:
        types: [feat, fix, test, chore, docs]
        footers:
          Task:
            source: ledger
            required_for: [test, chore]
            validate_for: all
            read_at: commit
          Item:
            source: registry
            validate_for: all
            read_at: commit
            in_place_of: { Task: [test, docs, chore] }
        scopes:
          docs: { only: ["**/*.md", "docs/**"] }
      tests:
        scenario: { root: features, id: "ID-[A-Z]+-\\d+" }
      """
    And the staged file "itos.yaml" holding the lines:
      """
      version: 1
      ledger:
        files: "tasks/phase-{group}.yaml"
        id: "T-\\d+"
      commits:
        types: [feat, fix, test, chore, docs]
        footers:
          Task:
            source: ledger
            required_for: [test, chore]
            validate_for: all
            read_at: commit
          Item:
            source: registry
            validate_for: all
            read_at: commit
            in_place_of: { Task: [test, docs, chore] }
        scopes:
          docs: { only: ["**/*.md", "docs/**", "features/**"] }
      tests:
        scenario: { root: features, id: "ID-[A-Z]+-\\d+" }
      """
    When itos runs "brief slice-9 --role drafter"
    Then itos exits with code 0
    And its output says "the header `test: specify slice-9, <short rule>`"
    And its output does not say "`docs: specify"
    When itos runs "brief slice-9 --role drafter --at HEAD~1"
    Then itos exits with code 0
    And its output says "the header `docs: specify slice-9, <short rule>`"
    And its output does not say "`test: specify"

  # One config, two files: docs may touch the ledger and not the feature
  # files, so the task's drafter keeps docs and the slice's gets test. A build
  # judging features/ for every item names test for both. The task's half rests
  # on slice-220, which gives a task's drafter its own text and whose ID-BRIEF-21
  # will pin the header sentence. slice-9's half is what fails today.
  @ID-BRIEF-36 @wip @slice-221
  Scenario: Under one config a slice's drafter and a task's get the type that fits each one's files
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the work registry has the task "T-001" owned by nobody with the status "todo" and no why
    And the task "T-001" has the committed check "go vet ./..."
    And the committed file "itos.yaml" holding the lines:
      """
      version: 1
      ledger:
        files: "tasks/phase-{group}.yaml"
        id: "T-\\d+"
      commits:
        types: [feat, fix, refactor, perf, test, build, ci, chore, docs, style, revert]
        footers:
          Task:
            source: ledger
            required_for: [refactor, perf, test, build, ci, chore, style, revert]
            validate_for: all
            read_at: commit
          Item:
            source: registry
            validate_for: all
            read_at: commit
            in_place_of: { Task: [test, docs, chore] }
        scopes:
          docs: { only: ["**/*.md", "docs/**", "tasks/**"] }
      tests:
        scenario: { root: features, id: "ID-[A-Z]+-\\d+" }
      """
    When itos runs "brief slice-9 --role drafter"
    Then itos exits with code 0
    And its output says "the header `test: specify slice-9, <short rule>`"
    When itos runs "brief T-001 --role drafter"
    Then itos exits with code 0
    And its output says "the header `docs: specify T-001, <short rule>`"

  # The scenarios' default config lets docs touch the feature files and the
  # ledger, and has no footer of the registry, so itos commit refuses --item
  # there as a usage error: the brief names the flag the config has, plain itos
  # commit for a slice and --task for a task. A build asking Item: to stand in
  # for docs too tells this drafter to stop, and one that takes test by default
  # names a type no flag here can carry. The task's half rests on slice-220 as
  # ID-BRIEF-36 does. The implementer's brief is not this slice's: guide brief's
  # template stays, and released ID-BRIEF-01 pins its --item.
  @ID-BRIEF-40 @wip @slice-221
  Scenario: Under the default config a drafter keeps docs and is told the flag the config has
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the work registry has the task "T-001" owned by nobody with the status "todo" and no why
    And the task "T-001" has the committed check "go vet ./..."
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    When itos runs "brief slice-9 --role drafter"
    Then itos exits with code 0
    And its output says "Commit with `itos commit`: the header `docs: specify slice-9, <short rule>`, the body the problem, the rule and each scenario's failing step. Push with `itos push`."
    And its output does not say "itos commit --item"
    And its output does not say "No commit type"
    When itos runs "brief T-001 --role drafter"
    Then itos exits with code 0
    And its output says "`itos commit --task T-001`"
    And its output says "the header `docs: specify T-001, <short rule>`"
    And its output does not say "itos commit --item"

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
  @ID-BRIEF-13 @slice-204
  Scenario: The brief is a function of HEAD, an uncommitted change to the config aside
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And an unrelated change to "itos.yaml" is staged
    When itos runs "brief slice-9 --role implementer --json"
    Then itos exits with code 0
    And its JSON's "item" is "slice-9"
    And the "path" of each entry of its JSON's "reads", in order, is "AGENTS.md, features/tagged.feature"

  # A ref written with a leading ./ or a trailing / names the same file or
  # folder (the review of the build, round 2); the reason keeps its spelling.
  @ID-BRIEF-14 @slice-204
  Scenario: A ref's leading ./ and trailing / name the same file or folder
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
          refs: [./internal/foo/, ./docs/plan.md]
      """
    And the committed file "internal/foo/doc.go" holding "// Package foo says the next step."
    And the committed file "docs/plan.md" holding "# The plan"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    When itos runs "brief slice-9 --role implementer --json"
    Then itos exits with code 0
    And the "path" of each entry of its JSON's "reads", in order, is "AGENTS.md, features/tagged.feature, internal/foo/doc.go, docs/plan.md"
    And its JSON's "reads" entry whose "path" is "internal/foo/doc.go" has "reason" "the item's ref ./internal/foo/"
    And its JSON's "omitted" is []

  # A ref naming a path outside the repository, which git refuses to list,
  # is left out as not in the tree, and the brief is still written (the
  # review of the build, round 3).
  @ID-BRIEF-16 @slice-204
  Scenario: A ref outside the repository is left out, the brief still written
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
          refs: [../notes.md, .., docs/plan.md]
      """
    And the committed file "docs/plan.md" holding "# The plan"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    When itos runs "brief slice-9 --role implementer --json"
    Then itos exits with code 0
    And the "path" of each entry of its JSON's "reads", in order, is "AGENTS.md, features/tagged.feature, docs/plan.md"
    And the "path" of each entry of its JSON's "omitted", in order, is "../notes.md, .."
    And its JSON's "omitted" entry whose "path" is "../notes.md" has "reason" "not in the tree at HEAD"

  # Without ledger.id an item of no kind is a task by its id never, as work
  # show reads it, so an untyped bug-<n> is still a bug and an untyped idea
  # still refused (the review of the build, round 4).
  @ID-BRIEF-17 @slice-204
  Scenario: Without ledger.id, an item of no kind is a bug or an idea by its id
    Given the config has no ledger.id
    And the committed feature file "features/fixes.feature" with the live scenario "@ID-F-01" tagged "@bug-7"
    And the committed file "tasks/work-items.yaml" holding the lines:
      """
      phases: { 1: null }
      items:
        - { id: bug-7, title: A bug, phase: 1, owner: null, status: todo, depends_on: [] }
        - { id: p1-next, title: Later, phase: 1, owner: null, status: todo, depends_on: [], why: Later. }
      """
    When itos runs "brief bug-7 --role implementer"
    Then itos exits with code 0
    And its output says "Implement bug-7 (a bug) of"
    And its output says "The scenarios are @ID-F-01 in features/fixes.feature, all live."
    When itos runs "brief p1-next --role drafter --json"
    Then itos exits with code 1
    And its JSON's problem "brief-idea"'s "message" says "p1-next"

  # A project in a folder of its own, with its own itos.yaml, is briefed
  # from that folder: AGENTS.md and a ref are the folder's, never the
  # repository top's, whose AGENTS.md the Background commits (the review of
  # the build, round 5). Nothing is tagged, so the drafter is briefed.
  @ID-BRIEF-18 @slice-204
  Scenario: A project in a subfolder is briefed from its own files
    Given the committed file "app/itos.yaml" holding the lines:
      """
      version: 1
      work:
        registry: work-items.yaml
      tests:
        scenario: { root: features, id: "ID-[A-Z]+-\\d+" }
      """
    And the committed file "app/work-items.yaml" holding the lines:
      """
      phases: { 1: null }
      items:
        - { id: slice-9, title: Nine, phase: 1, owner: null, status: todo, depends_on: [], refs: [docs/plan.md] }
      """
    And the committed file "app/AGENTS.md" holding "The app's rules."
    And the committed file "app/docs/plan.md" holding "# The plan"
    When itos runs "brief slice-9 --role drafter --json" from "app"
    Then itos exits with code 0
    And the "path" of each entry of its JSON's "reads", in order, is "AGENTS.md, docs/plan.md"
    And its JSON's "reads" entry whose "path" is "AGENTS.md" has "blob" the git blob id of "app/AGENTS.md" at HEAD
    And its JSON's "omitted" is []

  # The scenarios tagged for an item of such a project are found in its
  # own feature files at HEAD, so its implementer is briefed (the review of
  # the build, round 6).
  @ID-BRIEF-19 @slice-204
  Scenario: A project in a subfolder briefs its implementer from its own scenarios
    Given the committed file "app/itos.yaml" holding the lines:
      """
      version: 1
      work:
        registry: work-items.yaml
      tests:
        scenario: { root: features, id: "ID-[A-Z]+-\\d+" }
      """
    And the committed file "app/work-items.yaml" holding the lines:
      """
      phases: { 1: null }
      items:
        - { id: slice-9, title: Nine, phase: 1, owner: null, status: todo, depends_on: [] }
      """
    And the committed file "app/AGENTS.md" holding "The app's rules."
    And the committed file "app/features/one.feature" holding the lines:
      """
      Feature: One

        @ID-A-01 @wip @slice-9
        Scenario: one
          When it runs
      """
    When itos runs "brief slice-9 --role implementer --json" from "app"
    Then itos exits with code 0
    And the "path" of each entry of its JSON's "reads", in order, is "AGENTS.md, features/one.feature"
    And its JSON's "reads" entry whose "path" is "features/one.feature" has "blob" the git blob id of "app/features/one.feature" at HEAD

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
