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

  The drafter's brief names the commit type its spec goes in under (slice-221).
  It said docs whatever the config, and a project whose commits.scopes.docs
  leaves out features/ refuses that (itos commit check-paths --type docs): a
  drafter following the brief stopped twice on it, and its driver had to tell
  its agents the right type. The type is docs when the config lets docs carry
  the spec's files; otherwise the first of commits.types that may carry them
  and that the work registry's footer (Item:) stands in for. A type may carry
  them when commits.types has it, when the path rules itos commit check-paths
  judges with let it touch them, and when the hook's footer rules call nothing
  missing from a commit naming the item. The spec's files are those of the
  scenarios tagged for the item, else a feature file under the root of the
  config's scenarios; a task's are its ledger file, else the one its phase
  would have. Where no type may, the brief says so and tells the drafter to
  stop and report, so that it never names a type its own config refuses. The
  config is the one the brief reads, HEAD's or --at's. The request stays a
  digest of the item, the role, the spec commit and the reads: a config
  changing the type changes the brief's text and not its request, as the
  repository's own brief lines do.

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

  # The drafter's usual state: nothing is tagged yet, so no file names itself.
  # The rule judges a feature file under the scenarios' root, here acceptance/.
  # A build that judges features/ whatever the config says docs, as the rules
  # here let docs touch that folder and no other.
  @ID-BRIEF-29 @wip @slice-221
  Scenario: An item nothing is tagged for is judged by a feature file under the scenarios' root
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
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
          docs: { only: ["**/*.md", "docs/**", "features/**"] }
      tests:
        scenario: { root: acceptance, id: "ID-[A-Z]+-\\d+" }
      """
    When itos runs "brief slice-9 --role drafter"
    Then itos exits with code 0
    And its output says "No scenario is tagged @slice-9 yet."
    And its output says "the header `test: specify slice-9, <short rule>`"
    And its output does not say "`docs: specify"

  # The first of commits.types that may, and each way of not being it: feat
  # may touch anything but Item: does not stand in for it; refactor's never
  # bars the file; build's must_touch is not met, which check-paths refuses and
  # a reading of only and never alone (scope's TypesFor) lets through. in_place_of
  # lists chore first and commits.scopes lists neither test nor chore, so a
  # build taking either list's order, or the first type of all, names another.
  @ID-BRIEF-30 @wip @slice-221
  Scenario: The type is the first of commits.types that may touch the files and that Item stands in for
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And the committed file "itos.yaml" holding the lines:
      """
      version: 1
      ledger:
        files: "tasks/phase-{group}.yaml"
        id: "T-\\d+"
      commits:
        types: [feat, refactor, build, test, chore, docs]
        footers:
          Task:
            source: ledger
            required_for: [refactor, build, test, chore]
            validate_for: all
            read_at: commit
          Item:
            source: registry
            validate_for: all
            read_at: commit
            in_place_of: { Task: [chore, refactor, build, test, docs] }
        scopes:
          refactor: { never: ["features/**/*.feature"] }
          build: { must_touch: ["src/**"] }
          docs: { only: ["**/*.md", "docs/**"] }
      tests:
        scenario: { root: features, id: "ID-[A-Z]+-\\d+" }
      """
    When itos runs "brief slice-9 --role drafter"
    Then itos exits with code 0
    And its output says "the header `test: specify slice-9, <short rule>`"
    And its output does not say "`docs: specify"

  # A brief never names a type its own config would refuse. check-paths passes
  # a type with no rule, docs here, so the path rule alone would name it, and
  # the header lint's type-enum refuses it: commits.types lists no docs. A
  # config listing no types at all takes the conventional ones (message.Types).
  @ID-BRIEF-31 @wip @slice-221
  Scenario: A docs that commits.types does not list is not named
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And the committed file "itos.yaml" holding the lines:
      """
      version: 1
      ledger:
        files: "tasks/phase-{group}.yaml"
        id: "T-\\d+"
      commits:
        types: [feat, fix, test, chore]
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
            in_place_of: { Task: [test, chore] }
      tests:
        scenario: { root: features, id: "ID-[A-Z]+-\\d+" }
      """
    When itos runs "brief slice-9 --role drafter"
    Then itos exits with code 0
    And its output says "the header `test: specify slice-9, <short rule>`"
    And its output does not say "`docs: specify"

  # The same for the footers: docs may touch the files, yet the hook wants its
  # Task: and Item: does not stand in for docs, so a commit naming the item is
  # refused. What the hook's footer rules call missing (message.Missing) rules
  # out docs as it does any other type.
  @ID-BRIEF-32 @wip @slice-221
  Scenario: A docs that needs a footer the item does not give is not named
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
            required_for: [test, chore, docs]
            validate_for: all
            read_at: commit
          Item:
            source: registry
            validate_for: all
            read_at: commit
            in_place_of: { Task: [test, chore] }
        scopes:
          docs: { only: ["**/*.md", "docs/**", "features/**/*.feature"] }
      tests:
        scenario: { root: features, id: "ID-[A-Z]+-\\d+" }
      """
    When itos runs "brief slice-9 --role drafter"
    Then itos exits with code 0
    And its output says "the header `test: specify slice-9, <short rule>`"
    And its output does not say "`docs: specify"

  # No type may: docs is barred and no footer of the registry stands in for
  # any other, the commits section itos init writes today. So the brief says
  # so, and the drafter stops before it writes a spec it could not commit. The
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
    And its output says "No commit type of this repository's config can carry this spec: none may touch the spec's files and be committed with `itos commit --item slice-9`."
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

  # A task's drafter touches the ledger, not features/, so its file is judged
  # by the same rule: test may not touch it here, and chore, next in
  # commits.types, may. This rests on slice-220, which gives a task's drafter
  # its own text and none of whose scenarios pin the commit sentence: that
  # text keeps the header `<type>: specify <item>, <short rule>`, and the name
  # of the type comes from here. It fails today at the same step, as main's
  # brief of a task says docs.
  @ID-BRIEF-35 @wip @slice-221
  Scenario: A task's drafter is told the type that may touch its ledger file
    Given the work registry has the task "T-001" owned by nobody with the status "todo" and no why
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
          docs: { only: ["**/*.md", "docs/**"] }
          test: { only: ["features/**"] }
      tests:
        scenario: { root: features, id: "ID-[A-Z]+-\\d+" }
      """
    When itos runs "brief T-001 --role drafter"
    Then itos exits with code 0
    And its output says "the header `chore: specify T-001, <short rule>`"
    And its output does not say "`docs: specify"
    And its output does not say "`test: specify"

  # One config, two files: docs may touch the ledger and not the feature
  # files, so the task's drafter keeps docs and the slice's gets test. A build
  # judging features/ for every item names test for both. Rests on slice-220
  # as ID-BRIEF-35 does. slice-9's half is what fails today.
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

  # T-002 has no entry to name its file, so it is judged by the one its phase
  # would have, as itos task add would write: chore may touch that one file
  # alone, so a build judging ledger.files with {group} still in it says none
  # may. Rests on slice-220 as ID-BRIEF-35 does.
  @ID-BRIEF-37 @wip @slice-221
  Scenario: A task with no ledger entry yet is judged by the ledger file of its phase
    Given the work registry has the task "T-002" owned by nobody with the status "todo" and no why
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
          test: { only: ["features/**"] }
          chore: { only: ["tasks/phase-1.yaml"] }
      tests:
        scenario: { root: features, id: "ID-[A-Z]+-\\d+" }
      """
    When itos runs "brief T-002 --role drafter"
    Then itos exits with code 0
    And its output says "T-002 has no ledger entry yet."
    And its output says "the header `chore: specify T-002, <short rule>`"

  # No type may touch the ledger file with the item named: the same stop in
  # a task's text, with no word of scenarios in it, which ID-BRIEF-21 holds a
  # task's drafter to (slice-220). Rests on slice-220 as ID-BRIEF-35 does.
  @ID-BRIEF-38 @wip @slice-221
  Scenario: Where no type may touch its ledger file, a task's drafter is told to stop and report
    Given the work registry has the task "T-001" owned by nobody with the status "todo" and no why
    And the task "T-001" has the committed check "go vet ./..."
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
        scopes:
          docs: { only: ["**/*.md", "docs/**"] }
      tests:
        scenario: { root: features, id: "ID-[A-Z]+-\\d+" }
      """
    When itos runs "brief T-001 --role drafter"
    Then itos exits with code 0
    And its output says "No commit type of this repository's config can carry this spec: none may touch the spec's files and be committed with `itos commit --item T-001`."
    And its output says "Stop and report that before writing the spec; do not commit it under another type."
    And its output does not say "the header `"

  # Passes before the work, on purpose: under this repository's own policy, cut
  # down to the globs that reach a feature file, a slice's brief stays as it is.
  # docs may touch the file and comes before test by the rule, not by
  # commits.types, which has test first, so a build taking the first type that
  # may in that order names test. And the probe must look like a feature file,
  # for docs here takes only features/**/*.feature. For the implementer:
  # TestSliceAndBugBriefsGolden, which slice-220 adds, changes only where its
  # config's scopes make it. Under a config that keeps docs, as this one does
  # and as the unit tests' itos.yaml with no commits section does, the drafter's
  # commit sentence stays byte for byte, so this slice edits no golden text.
  @ID-BRIEF-39 @wip @slice-221 @guard
  Scenario: Under this repository's policy a slice's drafter keeps docs, today's text
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the committed file "itos.yaml" holding the lines:
      """
      version: 1
      ledger:
        files: "tasks/phase-{group}.yaml"
        id: "T-\\d+"
      commits:
        types: [feat, fix, refactor, perf, test, build, ci, chore, docs, revert]
        path_sets:
          implementation: ["{cmd,internal}/**/*.go"]
          gates: [itos.yaml, ".github/**", "tools/bin/**", "tools/selftest/**", deps-check.json, dprint.json]
        footers:
          Task:
            source: ledger
            required_for: [refactor, perf, test, build, ci, chore, revert]
            validate_for: all
            read_at: commit
          Item:
            source: registry
            validate_for: all
            read_at: commit
            in_place_of: { Task: [test, docs, chore, refactor, build, ci] }
        scopes:
          feat: { must_touch: [$implementation, "features/**"] }
          fix: { must_touch: [$implementation, "features/**"] }
          refactor: { never: ["features/**/*.feature", $gates] }
          perf: { never: ["features/**/*.feature", $gates] }
          test:
            never: [$gates]
            only: ["features/**", "{cmd,internal}/**/*_test.go", "tools/itos/conformance/**", "tools/itos/fixtures/**"]
          build: { only: [".devcontainer/**"] }
          ci: { only: [".github/**"] }
          chore: { never: [$implementation, $gates] }
          docs: { only: ["**/*.md", "docs/**", "tasks/**", "features/**/*.feature"] }
      tests:
        scenario: { root: features, id: "ID-[A-Z]+-\\d+" }
      """
    When itos runs "brief slice-9 --role drafter"
    Then itos exits with code 0
    And its output says "Commit with `itos commit --item slice-9`: the header `docs: specify slice-9, <short rule>`, the body the problem, the rule and each scenario's failing step. Push with `itos push`."

  # Passes before the work, on purpose: the scenarios' default config lets docs
  # touch the feature files and has no Item footer. A build asking Item: to
  # stand in for docs too tells this drafter to stop, and one that takes test
  # by default names a type Item: cannot carry.
  @ID-BRIEF-40 @wip @slice-221 @guard
  Scenario: Under the default config a drafter keeps docs, with no Item footer to stand in
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    When itos runs "brief slice-9 --role drafter"
    Then itos exits with code 0
    And its output says "the header `docs: specify slice-9, <short rule>`"
    And its output does not say "No commit type"

  # Passes before the work, on purpose: a config with no commits section has no
  # rule to refuse a type (ID-BRIEF-18's project in a folder, and the corpus's
  # brief.yaml, are such), where the path rules' own loader calls that a config
  # error. A build passing it on fails the brief; one saying none may stops it.
  @ID-BRIEF-41 @wip @slice-221 @guard
  Scenario: A config with no commits section refuses no type, so a drafter keeps docs
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the committed file "itos.yaml" holding the lines:
      """
      version: 1
      tests:
        scenario: { root: features, id: "ID-[A-Z]+-\\d+" }
      """
    When itos runs "brief slice-9 --role drafter"
    Then itos exits with code 0
    And its output says "the header `docs: specify slice-9, <short rule>`"
    And its output does not say "No commit type"

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
