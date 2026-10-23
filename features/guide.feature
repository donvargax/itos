@phase-1
Feature: itos go and itos guide, the guides a session starts from
  A session that coordinates a repository's work followed a guide kept in
  that repository, written for it and copied nowhere else, so what one
  repository learnt about running agents never reached another (the user's
  calls, 2026-10-04, p3-coordinator-skill). The guides now ship in the
  binary, generic and versioned with itos: itos go prints the coordinator's
  guide, the command a person starts every session with (! itos go), and
  itos guide work prints the implementer's, which a coordinator asks each
  agent it starts to run first. What a repository learns that holds for no
  other stays its own: itos go appends docs/ORCHESTRATING.md after the
  generic guide when the repository has one (guide.orchestrating in the
  config names another file, under the git folder for a stealth config).
  They read and print, never write, and need no config: any git repository
  will do. The Claude Code plugin wraps neither guide (T-092 dropped its
  skill): a session runs them as commands.

  @ID-GUIDE-01 @slice-64
  Scenario: itos go prints the coordinator's guide
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "go"
    Then itos exits with code 0
    And its output says "# Coordinating with itos"

  @ID-GUIDE-02 @slice-64
  Scenario: itos go appends the repository's own orchestrating notes after the guide
    Given a repository that does not use itos, its one commit "docs: start"
    And the committed file "docs/ORCHESTRATING.md" holding "Our own lesson: the inbox is read first."
    When itos runs "go"
    Then itos exits with code 0
    And its output says "# Coordinating with itos" before "Our own lesson: the inbox is read first."

  @ID-GUIDE-03 @slice-64
  Scenario: guide.orchestrating names the file itos go appends
    Given a repository whose ledger has the task "T-001"
    And the committed file "notes/agents.md" holding "Kept elsewhere."
    And guide.orchestrating is "notes/agents.md"
    When itos runs "go"
    Then itos exits with code 0
    And its output says "Kept elsewhere."

  @ID-GUIDE-04 @slice-64
  Scenario: itos guide work prints the implementer's guide
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "guide work"
    Then itos exits with code 0
    And its output says "# Working with itos"

  @ID-GUIDE-05 @slice-64
  Scenario: itos guide refuses a guide it does not have, naming those it has
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "guide review"
    Then itos exits with code 2
    And its output says "review"
    And its output says "coordinate, work"

  # What holds on one machine only (a stale itos on the PATH, a local path) is
  # neither the repository's notes nor the generic guide's: it goes in this
  # clone's own notes, .git/itos/notes.md under the git common dir, which git
  # never commits and every linked worktree shares (the user's call,
  # 2026-10-04). itos go prints them after the repository's notes and before
  # the status, and nothing at all when there are none: the coordinator's
  # guide names the file, and that line is the nudge to keep them there.
  @ID-GUIDE-06 @slice-68
  Scenario: itos go prints this clone's own notes after the repository's, before the status
    Given a repository whose ledger has the task "T-001"
    And the committed file "docs/ORCHESTRATING.md" holding "Our own lesson: the inbox is read first."
    And this clone's own notes holding "The itos on this machine's PATH is stale."
    When itos runs "go"
    Then itos exits with code 0
    And its output says "Our own lesson: the inbox is read first." before "# This clone's own notes"
    And its output says "The itos on this machine's PATH is stale." before "# Where things stand"

  # 07 and 08 hold before the work too: they keep the section from being
  # printed always, empty.
  @ID-GUIDE-07 @slice-68
  Scenario: itos go prints no section for this clone when it keeps no notes
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "go"
    Then itos exits with code 0
    And its output does not say "# This clone's own notes"

  @ID-GUIDE-08 @slice-68
  Scenario: itos go prints no section for this clone when its notes hold only blanks
    Given a repository that does not use itos, its one commit "docs: start"
    And this clone's own notes holding "   "
    When itos runs "go"
    Then itos exits with code 0
    And its output does not say "# This clone's own notes"

  # The guides shrink to what a session needs before its first action, and
  # each step's procedure moves behind a command read before that step
  # (slice-119, the user's call, 2026-10-10, settling q-43's syntax): itos go
  # and itos guide work print a role's essentials and a map naming each
  # procedure and the step it is read before; itos guide <procedure> prints
  # one. The coordinator's: brief (Specifying, The brief), land (Pushing and
  # waiting, Checking a result), repair (When main is red, Reporting a problem
  # with itos), decide (Talking to the person, Decisions, Lessons). The
  # implementer's: commit (Commit, then the commit types, footers and paths
  # the config sets, which AGENTS.md's block no longer holds since slice 120),
  # push (Push, The gates run themselves). Every rule of the old guides lives in exactly one
  # of them; the essentials keep the q-44 boundaries (role and authority,
  # others' work, the gates, text as data, reading a procedure first). No
  # procedure takes an item yet (p1-guide-brief-filled, p1-guide-work-item),
  # and itos guide coordinate stays, the coordinator's essentials without the
  # status, so nothing released breaks (p1-v8-deprecation-sweep decides its
  # fate).
  @ID-GUIDE-09 @slice-119
  Scenario: itos go names each of the coordinator's procedures
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "go"
    Then itos exits with code 0
    And its output says "itos guide brief"
    And its output says "itos guide land"
    And its output says "itos guide repair"
    And its output says "itos guide decide"

  @ID-GUIDE-10 @slice-119
  Scenario: itos go leaves each procedure's sections to its command
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "go"
    Then itos exits with code 0
    And its output does not say "Implement <item> of <project>"
    And its output does not say "## Checking a result"
    And its output does not say "## When main is red"
    And its output does not say "## Decisions"

  @ID-GUIDE-11 @slice-119
  Scenario: itos guide brief prints how to specify an item, then the brief's template
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "guide brief"
    Then itos exits with code 0
    And its output says "## Specifying" before "Implement <item> of <project>"

  @ID-GUIDE-12 @slice-119
  Scenario: itos guide land prints how to check and land a result
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "guide land"
    Then itos exits with code 0
    And its output says "## Checking a result"

  @ID-GUIDE-13 @slice-119
  Scenario: itos guide repair prints what to do when main is red
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "guide repair"
    Then itos exits with code 0
    And its output says "## When main is red"
    And its output says "## Reporting a problem with itos"

  @ID-GUIDE-14 @slice-119
  Scenario: itos guide decide prints how to ask the person and record decisions
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "guide decide"
    Then itos exits with code 0
    And its output says "## Decisions"
    And its output says "## Lessons"

  @ID-GUIDE-15 @slice-119
  Scenario: itos guide work names the implementer's procedures and leaves their sections to them
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "guide work"
    Then itos exits with code 0
    And its output says "itos guide commit"
    And its output says "itos guide push"
    And its output does not say "itos commit check-paths"
    And its output does not say "## Push"

  @ID-GUIDE-16 @slice-119
  Scenario: itos guide commit prints how to commit, then the commit types the config sets
    Given a repository whose ledger has the task "T-001"
    And the config's commits.types gains "deps"
    When itos runs "guide commit"
    Then itos exits with code 0
    And its output says "itos commit check-paths" before "`deps`"

  # A repository without a config still learns how to commit; there is no
  # policy of its own to print.
  @ID-GUIDE-17 @slice-119
  Scenario: itos guide commit outside a repository using itos prints how to commit alone
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "guide commit"
    Then itos exits with code 0
    And its output says "itos commit check-paths"
    And its output does not say "A commit's type is one of"

  @ID-GUIDE-18 @slice-119
  Scenario: itos guide push prints how to push and what the gates do
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "guide push"
    Then itos exits with code 0
    And its output says "## Push"
    And its output says "## The gates run themselves"

  # GUIDE-05's "coordinate, work" holds too: the refusal names the guides,
  # then the procedures.
  @ID-GUIDE-19 @slice-119
  Scenario: itos guide refuses a name it has no guide or procedure by, naming the procedures
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "guide review"
    Then itos exits with code 2
    And its output says "brief, commit, decide, land, push, repair"

  @ID-GUIDE-20 @slice-119
  Scenario: itos guide coordinate prints the coordinator's essentials without the status
    Given a repository whose ledger has the task "T-001"
    When itos runs "guide coordinate"
    Then itos exits with code 0
    And its output says "itos guide brief"
    And its output does not say "# Where things stand"

  # Slice 167, the user's approval of 2026-10-23: guide.overrides explicitly
  # maps known guide/procedure names to Markdown paths. The eight names below
  # are the complete set; this adds neither names nor implicit filename,
  # directory, global or user override discovery. An absent mapping or absent
  # name selects shipped prose. Read only the selected name's override file.
  # A selected file replaces that name's prose, never its separate generated
  # policy, notes or status. Read Markdown as text, without execution,
  # templating, environment expansion or a required heading. An empty file is
  # an intentional replacement, not an absent optional note. Ordinary composed
  # prose keeps its contents, adding a final LF when needed. No command writes
  # a guide merely to display it. Slice 167 implements ordinary selection and
  # composition only, not raw export or launcher selection changes.
  #
  # Ordinary text stdout remains the composed Markdown, without a provenance
  # header, footer or wrapper. Its stderr has one source-identification line:
  # itos: guide <name> source: builtin
  # or itos: guide <name> source: <resolved path>.
  # Those sentences are human output, not a new wording contract. JSON adds
  # source: {kind: builtin} or {kind: file, path: <resolved path>}; path is the
  # actual opened path, slash-normalized, relative or absolute as the existing
  # config resolution gives it. JSON has no extra provenance line on stderr;
  # existing warnings remain. Its text contains the ordinary composition,
  # never the source line. Existing guide/text/notes/local_notes/status fields
  # retain their meaning. ADR-0054 exempts guide prose, not source metadata.
  # previous-release/main.go's additive JSON matching accepts the new source
  # field. Preserve existing outcomes, fields and files without overrides:
  # feat, Upgrading: none, no anticipated breaking Changes footer.
  #
  # Slice-167 delivery includes guide/go help and package, handler and Run doc
  # comments: document guide.overrides, optional config and its warning recovery,
  # selected-file config-guide-override-read exit 2, ordinary stderr provenance
  # and additive JSON source. Update internal/slices/{guide,gocmd}/command.go
  # and their docs, internal/guide/doc.go and internal/engine/guides/doc.go.
  # Remove stale same-in-every-repository, config-never-needed and always-exits-0
  # claims, notably internal/slices/gocmd/run.go: selected-file failures now exit
  # 2, while runtime config-load failures retain the agreed recovery below.
  # Raw flag/export help and the launcher invoked-binary exception are a
  # separate slice-168 delivery, reviewed separately in this same GUIDE area.
  # Neither delivery needs a dependency merely for raw export; sequence their
  # implementations to avoid overlapping shared-guide and CLI edits.
  #
  # These scenarios belong to guides only: no plugin, instruction-file startup
  # routing, notes/rules-data redesign, harness enforcement or coordinator-loop
  # policy adoption. Keep go's existing repo/clone notes and status composition;
  # do not implement q-47's note migration. The continuation prose is separate.
  @wip @slice-167 @ID-GUIDE-21
  Scenario Outline: An explicitly configured guide or procedure replaces only its shipped prose
    Given a repository whose ledger has the task "T-001"
    And the config sets "guide" to the YAML "{ overrides: { <name>: guides/selected.md } }"
    And the committed file "guides/selected.md" holding "Our replacement prose."
    When itos runs "guide <name> --json"
    Then itos exits with code 0
    And its JSON's "guide" is "<name>"
    And its JSON's "source" is {"kind":"file","path":"guides/selected.md"}
    And stdout says "Our replacement prose."
    And stdout does not say "<shipped-heading>"
    And stderr is empty
    And no file changed since the last run

    Examples:
      | name       | shipped-heading                  |
      | coordinate | # Coordinating with itos          |
      | work       | # Working with itos               |
      | brief      | # Specifying and briefing an item |
      | land       | # Landing a result                |
      | repair     | # Repairing what is red           |
      | decide     | # Deciding with the person        |
      | commit     | # Committing with itos            |
      | push       | # Pushing with itos               |

  @wip @slice-167 @ID-GUIDE-22
  Scenario: An unconfigured guide stays shipped without opening another name's missing override
    Given a repository whose ledger has the task "T-001"
    And the config sets "guide" to the YAML "{ overrides: { coordinate: missing.md } }"
    And the committed file "docs/guides/work.md" holding "Not configured."
    When itos runs "guide work --json"
    Then itos exits with code 0
    And its JSON's "source" is {"kind":"builtin"}
    And stdout says "# Working with itos"
    And stdout does not say "Not configured."
    And stderr is empty
    And no file changed since the last run

  # Compare streams independently. Existing outputSays combines stdout and
  # stderr, which cannot establish that provenance stayed out of Markdown.
  # The stderr source helper checks exactly one source-identification line
  # containing the canonical guide name and selected path or builtin, not
  # exact human wording. Other existing warning lines are not provenance.
  # Ordinary go uses canonical name coordinate, not go, and its stdout retains
  # notes and status; do not compare that composition to a bare guide file.
  @wip @slice-167 @ID-GUIDE-23
  Scenario: Ordinary text identifies file and builtin sources only on stderr
    Given a repository whose ledger has the task "T-001"
    And the config sets "guide" to the YAML "{ overrides: { work: guides/work.md, coordinate: guides/coordinate.md }, orchestrating: docs/ORCHESTRATING.md }"
    And the committed file "guides/work.md" holding "Our working guide."
    And the committed file "guides/coordinate.md" holding "Our coordinator."
    And the committed file "docs/ORCHESTRATING.md" holding "Our repository notes."
    And this clone's own notes holding "Our clone notes."
    When itos runs "guide work"
    Then itos exits with code 0
    And stdout is exactly the contents of "guides/work.md"
    And stderr identifies guide "work" as selected from "guides/work.md"
    And no file changed since the last run
    When itos runs "guide push"
    Then itos exits with code 0
    And stdout begins with "# Pushing with itos"
    And stdout says "## The gates run themselves"
    And stderr identifies guide "push" as selected from "builtin"
    And no file changed since the last run
    When itos runs "go"
    Then itos exits with code 0
    And stdout begins with "Our coordinator."
    And stdout says "Our coordinator." before "Our repository notes."
    And stdout says "Our repository notes." before "Our clone notes."
    And stdout says "Our clone notes." before "# Where things stand"
    And stderr identifies guide "coordinate" as selected from "guides/coordinate.md"
    And no file changed since the last run

  # Selection is shared, composition is not collapsed: guide coordinate has
  # repository notes only; go adds clone notes and status as it already does.
  # Existing same-file clone/repository note deduplication remains unchanged.
  @wip @slice-167 @ID-GUIDE-24
  Scenario: Both coordinator commands select the same replacement and keep their composition
    Given a repository whose ledger has the task "T-001"
    And the config sets "guide" to the YAML "{ overrides: { coordinate: guides/coordinate.md }, orchestrating: docs/ORCHESTRATING.md }"
    And the committed file "guides/coordinate.md" holding "Our coordinator."
    And the committed file "docs/ORCHESTRATING.md" holding "Our repository notes."
    And this clone's own notes holding "Our clone notes."
    When itos runs "go --json"
    Then itos exits with code 0
    And its JSON's "source" is {"kind":"file","path":"guides/coordinate.md"}
    And its JSON's "notes" is "docs/ORCHESTRATING.md"
    And its JSON's "local_notes" is ".git/itos/notes.md"
    And its JSON has the field "status"
    And its JSON's "text" says "Our coordinator." before "Our repository notes."
    And its JSON's "text" says "Our repository notes." before "Our clone notes."
    And stderr is empty
    And no file changed since the last run
    When itos runs "guide coordinate --json"
    Then itos exits with code 0
    And its JSON's "source" is {"kind":"file","path":"guides/coordinate.md"}
    And its JSON's "text" says "Our coordinator." before "Our repository notes."
    And its JSON has no field "local_notes"
    And its JSON has no field "status"
    And stdout does not say "Our clone notes."
    And stderr is empty
    And no file changed since the last run

  # Policy is always generated after ordinary commit prose when config loads,
  # even when the selected custom prose omits every policy instruction. The
  # prose does not alter commits.types, footer/path rules, hooks or CI gates.
  # committedFileHolding commits the preceding config setters too, so the
  # hook's staged config really includes this adoption before README changes.
  @wip @slice-167 @ID-GUIDE-25
  Scenario: Replacement commit prose cannot remove generated policy or bypass the hook
    Given a repository whose ledger has the task "T-001"
    And the header lint is commitlint's conventional config
    And the config's commits.types gains "deps"
    And the config sets "guide" to the YAML "{ overrides: { commit: guides/commit.md } }"
    And the committed file "guides/commit.md" holding "Our commit procedure."
    And a change to "README.md" is staged
    When itos runs "guide commit"
    Then itos exits with code 0
    And stdout says "Our commit procedure." before "## The rules the config sets"
    And stdout says "`deps`"
    And stdout does not say "# Committing with itos"
    And stderr identifies guide "commit" as selected from "guides/commit.md"
    When the commit-msg hook checks the message "chore: tidy the readme"
    Then itos exits with code 1
    And its output names the rule "task-footer"

  # Once a valid config declares the selected name's override, every file-read
  # failure is fatal: missing, unreadable or a directory. No optional-notes
  # reader, warning-and-default fallback or partial guide/status output.
  # This is config exit 2, not missing-environment 3 or unclassified 70.
  # Rule: config-guide-override-read. Plain output has empty stdout and a
  # stderr diagnostic naming config, guide.overrides.<name>, resolved path and
  # cause. JSON uses existing configFailure: schema:1, config, valid:false,
  # problems with that rule and actionable message/fix; no guide success,
  # source, text, notes or status fields. Do not assert OS-specific wording.
  # A directory is the portable unreadable-file fixture, not chmod under root.
  # Cover the ordinary guide and procedure branches, including commit before
  # policy composition, as well as go. The selected name is fixture data and
  # the refusal must identify that same name, not hardcode coordinate.
  @wip @slice-167 @ID-GUIDE-26
  Scenario Outline: A selected override read failure refuses guides procedures and go without a fallback
    Given a repository whose ledger has the task "T-001"
    And a "guides" folder
    And the config sets "guide" to the YAML "{ overrides: { <name>: <path> } }"
    When itos runs "<command>"
    Then itos exits with code 2
    And the override read refusal for "<name>" at "<path>" is a "<format>" config failure
    And no file changed since the last run

    Examples:
      | name       | path              | command                 | format |
      | coordinate | guides/missing.md | guide coordinate        | text   |
      | coordinate | guides            | guide coordinate        | text   |
      | coordinate | guides/missing.md | go                      | text   |
      | coordinate | guides            | go                      | text   |
      | coordinate | guides/missing.md | guide coordinate --json | json   |
      | coordinate | guides            | guide coordinate --json | json   |
      | coordinate | guides/missing.md | go --json               | json   |
      | coordinate | guides            | go --json               | json   |
      | work       | guides/missing.md | guide work              | text   |
      | work       | guides            | guide work              | text   |
      | work       | guides/missing.md | guide work --json       | json   |
      | work       | guides            | guide work --json       | json   |
      | commit     | guides/missing.md | guide commit            | text   |
      | commit     | guides            | guide commit            | text   |
      | commit     | guides/missing.md | guide commit --json     | json   |
      | commit     | guides            | guide commit --json     | json   |

  # Validate declarations through config check, without opening their files:
  # strict known names, mapping shape, string paths and nonblank paths.
  # Whitespace-only paths are invalid; do not trim a nonblank filename before
  # opening it. Empty file contents are different from an empty path (GUIDE-35).
  # The whole guide YAML setter avoids duplicate guide parent keys in fixtures.
  @wip @slice-167 @ID-GUIDE-27
  Scenario Outline: Config validation rejects unknown override names and invalid path declarations
    Given a repository whose ledger has the task "T-001"
    And the config sets "guide" to the YAML "{ overrides: <value> }"
    When itos runs "config check --json"
    Then itos exits with code 2
    And its JSON's "valid" is false
    And its JSON's "problems" has one entry whose "rule" is "<rule>"
    And stdout says "<key>"

    Examples:
      | value                | rule               | key                    |
      | { review: guide.md } | config-unknown-key | guide.overrides.review |
      | []                   | config-type        | guide.overrides        |
      | guide.md             | config-type        | guide.overrides        |
      | { work: [guide.md] } | config-type        | guide.overrides.work   |
      | { work: null }       | config-type        | guide.overrides.work   |
      | { work: true }       | config-type        | guide.overrides.work   |
      | { work: 12 }         | config-type        | guide.overrides.work   |
      | { work: '' }         | config-invalid     | guide.overrides.work   |
      | { work: '   ' }      | config-invalid     | guide.overrides.work   |

  # Coordinator decision: runtime config load errors KEEP warning-and-shipped
  # recovery, including schema validation errors in guide.overrides. No
  # declaration was successfully selected. Config check still rejects them.
  # Do not parse human diagnostics to distinguish these failures or introduce
  # a broad fatal-config compatibility change. Preserve existing warning,
  # policy omission and notes/status recovery behavior where applicable; a
  # successful recovery identifies its actual source as builtin. A loaded
  # selected override's read failure is instead GUIDE-26, never recovery.
  # The builtin source assertion is new, so these are not already-green guards.
  @wip @slice-167 @ID-GUIDE-28
  Scenario Outline: An ordinary guide reports builtin provenance after a config load failure
    Given a repository whose ledger has the task "T-001"
    And the untracked file "itos.yaml" holding "<config>"
    When itos runs "<command> --json"
    Then itos exits with code 0
    And its JSON's "source" is {"kind":"builtin"}
    And stdout says "<heading>"
    And stderr says "itos.yaml"

    Examples:
      | config                                                       | command          | heading                           |
      | version: [                                                   | go               | # Coordinating with itos          |
      | version: [                                                   | guide coordinate | # Coordinating with itos          |
      | version: [                                                   | guide work       | # Working with itos               |
      | version: [                                                   | guide brief      | # Specifying and briefing an item |
      | version: [                                                   | guide land       | # Landing a result                |
      | version: [                                                   | guide repair     | # Repairing what is red           |
      | version: [                                                   | guide decide     | # Deciding with the person        |
      | version: [                                                   | guide commit     | # Committing with itos            |
      | version: [                                                   | guide push       | # Pushing with itos               |
      | { version: 1, guide: { overrides: { work: [guide.md] } } }     | guide work       | # Working with itos               |
      | { version: 1, guide: { overrides: { review: guide.md } } }     | go               | # Coordinating with itos          |

  # Override paths follow guide.orchestrating, not a new search hierarchy.
  # Automatic top discovery moves a subfolder run to the root before paths
  # resolve, unless that folder has itos.yaml or explicit config/root disables
  # discovery. --root changes effective cwd first; relative --config is read
  # there, not at the original caller cwd. An explicit config anywhere else
  # does NOT rebase paths beside itself. ITOS_CONFIG has the same convention.
  # Absolute paths stay absolute. Relative source metadata names the effective
  # execution directory's path, not the original caller's relative path.
  # Stealth alone rebases these private data paths beside the recognized config
  # in the Git common dir, including explicit selection and linked worktrees.
  # A future slice-160 config-location boundary is separate, currently @wip:
  # neither assume its restrictions exist nor implement external-config policy
  # here. These alternative project config fixtures stay inside the worktree.
  @wip @slice-167 @ID-GUIDE-29
  Scenario Outline: Automatic root discovery and explicit root keep project-root override paths
    Given a repository whose ledger has the task "T-001"
    And the config sets "guide" to the YAML "{ overrides: { work: guides/work.md } }"
    And the committed file "guides/work.md" holding "Root working guide."
    And the committed file "sub/guides/work.md" holding "Subfolder decoy."
    When itos runs "<command>" from "<folder>"
    Then itos exits with code 0
    And its JSON's "source" is {"kind":"file","path":"guides/work.md"}
    And stdout says "Root working guide."
    And stdout does not say "Subfolder decoy."

    Examples:
      | folder | command                     |
      | .      | guide work --json           |
      | sub    | guide work --json           |
      | sub    | --root .. guide work --json |
      | sub    | guide work --root .. --json |

  @wip @slice-167 @ID-GUIDE-30
  Scenario Outline: Explicit command-line and environment config selection keep effective-directory paths
    Given a repository whose ledger has the task "T-001"
    And the committed file "config/alternate.yaml" holding the lines:
      """
      version: 1
      guide:
        overrides:
          work: guides/work.md
      """
    And the committed file "guides/work.md" holding "Root working guide."
    And the committed file "sub/guides/work.md" holding "Subfolder working guide."
    And the committed file "config/guides/work.md" holding "Beside-config decoy."
    And ITOS_CONFIG is "<environment-config>"
    When itos runs "<command>" from "<folder>"
    Then itos exits with code 0
    And its JSON's "source" is {"kind":"file","path":"guides/work.md"}
    And stdout says "<selected>"
    And stdout does not say "Beside-config decoy."

    Examples:
      | folder | environment-config       | command                                                    | selected                 |
      | .      |                          | --config config/alternate.yaml guide work --json            | Root working guide.      |
      | sub    |                          | --config ../config/alternate.yaml guide work --json         | Subfolder working guide. |
      | sub    |                          | --root .. --config config/alternate.yaml guide work --json  | Root working guide.      |
      | .      | config/alternate.yaml    | guide work --json                                          | Root working guide.      |
      | sub    | ../config/alternate.yaml | guide work --json                                          | Subfolder working guide. |

  @wip @slice-167 @ID-GUIDE-31
  Scenario: A subfolder's own config selects its own override instead of the root's
    Given a repository whose ledger has the task "T-001"
    And the config sets "guide" to the YAML "{ overrides: { work: guides/work.md } }"
    And the committed file "guides/work.md" holding "Root decoy."
    And the committed file "sub/itos.yaml" holding the lines:
      """
      version: 1
      guide:
        overrides:
          work: guides/work.md
      """
    And the committed file "sub/guides/work.md" holding "Subfolder working guide."
    When itos runs "guide work --json" from "sub"
    Then itos exits with code 0
    And its JSON's "source" is {"kind":"file","path":"guides/work.md"}
    And stdout says "Subfolder working guide."
    And stdout does not say "Root decoy."

  # Config setters respect w.dataDir. The .git file is deliberately untracked;
  # committedFileHolding commits config fixture changes, not Git private data.
  @wip @slice-167 @ID-GUIDE-32
  Scenario Outline: Automatically and explicitly selected stealth configs resolve overrides beside themselves
    Given a repository whose ledger has the task "T-001", kept in its git folder
    And the config sets "guide" to the YAML "{ overrides: { work: guides/work.md } }"
    And the untracked file ".git/itos/guides/work.md" holding "Private working guide."
    And the committed file "guides/work.md" holding "Project decoy."
    When itos runs "<command>"
    Then itos exits with code 0
    And its JSON's "source" is {"kind":"file","path":".git/itos/guides/work.md"}
    And stdout says "Private working guide."
    And stdout does not say "Project decoy."
    When itos runs "config get guide.overrides.work --json"
    Then itos exits with code 0
    And its JSON's "value" is ".git/itos/guides/work.md"

    Examples:
      | command                                               |
      | guide work --json                                     |
      | --config .git/itos/itos.yaml guide work --json          |

  # linkedWorktree moves w.dir under w.support and creates w.linked. Git gives
  # that worktree an absolute common dir. The new runner uses w.itosIn at
  # w.linked; explicit selection supplies that real common dir's itos.yaml,
  # not a guessed .git directory inside the linked worktree. Expected paths
  # come from fixture Git location, never from production guide/config code.
  @wip @slice-167 @ID-GUIDE-33
  Scenario Outline: A linked worktree shares the common-directory stealth override
    Given a repository whose ledger has the task "T-001", kept in its git folder
    And the config sets "guide" to the YAML "{ overrides: { work: guides/work.md } }"
    And the untracked file ".git/itos/guides/work.md" holding "Shared private working guide."
    And the committed file "guides/work.md" holding "Project decoy."
    And a linked worktree of the repository at "../linked"
    When itos runs "guide work --json" in the linked worktree using "<selection>" config selection
    Then itos exits with code 0
    And its JSON's "source.kind" is "file"
    And its JSON's "source.path" names the common Git directory's "itos/guides/work.md" as an absolute slash-normalized path
    And stdout says "Shared private working guide."
    And stdout does not say "Project decoy."
    When itos runs "config get guide.overrides.work --json" in the linked worktree using "<selection>" config selection
    Then itos exits with code 0
    And its JSON's "value" names the common Git directory's "itos/guides/work.md" as an absolute slash-normalized path

    Examples:
      | selection |
      | automatic |
      | explicit  |

  # This setup computes a native absolute scratch-file path and writes one
  # whole guide mapping as YAML. Do not hardcode a Unix temporary prefix or
  # expect backslashes in JSON; absolute paths are not stealth-rebased either.
  @wip @slice-167 @ID-GUIDE-34
  Scenario Outline: An absolute override path is used unchanged in project and stealth configs
    Given a repository whose ledger has the task "T-001"
    And the committed file "guides/work.md" holding "An absolute working guide."
    And the guide config is kept in "<mode>" mode
    And the config's "work" override names the absolute path of "guides/work.md"
    When itos runs "guide work --json"
    Then itos exits with code 0
    And its JSON's "source.kind" is "file"
    And its JSON's "source.path" is the absolute slash-normalized path of "guides/work.md"
    And stdout says "An absolute working guide."

    Examples:
      | mode    |
      | project |
      | stealth |

  # The empty-file helper writes zero bytes, not the ordinary file setter's
  # appended LF. Only ordinary composition supplies the final LF below.
  @wip @slice-167 @ID-GUIDE-35
  Scenario: An empty override file intentionally selects blank prose
    Given a repository whose ledger has the task "T-001"
    And the config sets "guide" to the YAML "{ overrides: { work: guides/empty.md } }"
    And the empty untracked file "guides/empty.md"
    When itos runs "guide work --json"
    Then itos exits with code 0
    And its JSON's "source" is {"kind":"file","path":"guides/empty.md"}
    And its JSON's "text" is "\n"
    And stderr is empty
    And no file changed since the last run

  # Slice 168: raw shipped-text export is a separate feature and implementation
  # delivery from slice 167, with its own review artifact. GUIDE-36 through
  # GUIDE-40 retain their IDs here but belong only to slice 168. Raw export
  # needs no override dependency: even a pre-override runtime must ignore all
  # config for this operation. Sequence implementations to avoid shared-guide
  # and CLI overlap; do not implement this launcher/export scope in slice 167.
  #
  # NEW flag: guide <name> --builtin is raw export of the INVOKED binary's
  # shipped Markdown, exact guide.Text bytes after its existing CRLF-to-LF
  # normalization. No added/removed newline, wrapper, provenance, repository
  # or clone notes, generated policy, status, config reads or writes. It works
  # outside Git. It deliberately bypasses pin/latest/ITOS_VERSION selection,
  # release requests/launches and cache writes, unlike ordinary guide calls.
  # No go --builtin. --builtin with --json is usage exit 2 naming both flags.
  # Shell redirection alone writes the requested file; itos never exports by
  # first writing a file and reading it back. These shell-facing framing
  # guarantees do not freeze the shipped prose against future releases.
  #
  # Slice-168 delivery includes declared flag/help and guide/export doc updates:
  # describe raw bytes, no wrapper/config/notes/policy/status, outside-Git use,
  # --json incompatibility and no go flag. Update launcher documentation for
  # the deliberate invoked-binary exception to pin/latest/ITOS_VERSION selection,
  # including internal/launch/{doc.go,launch.go}; do not leave it described as
  # ordinary pinned execution. The guide slice and CLI/global-path docs must
  # explain the structural parse and no-config raw path below. ADR-0024's normal
  # launcher selection remains the ordinary-command behavior, not raw export.
  #
  # Parse the declared guide switch and global flags structurally, including
  # --root/--config before or after the name; no argv substring detector or
  # flag-value confusion. A valid --root still selects an existing execution
  # directory; a bad --root retains its existing usage error. The raw path
  # skips automatic config-based top discovery and config loading, and skips
  # launcher selection before it can read a pin or update cache state. A
  # handler-only bypass is insufficient. Unit/launcher tests must prove the
  # no-read path and exact Text equality; feature assertions never import
  # internal/guide or inspect shipped source. Public raw baselines below stay
  # in scenario memory, not an export file. No tests are run while drafting.
  @wip @slice-168 @ID-GUIDE-36
  Scenario Outline: Raw builtin export works outside Git without wrappers or filesystem writes
    Given a folder that is not a git repository
    When itos runs "guide <name> --builtin"
    Then itos exits with code 0
    And stdout begins with "<heading>"
    And stdout uses LF line endings
    And stdout does not say "## The rules the config sets"
    And stdout does not say "# This clone's own notes"
    And stdout does not say "# Where things stand"
    And stderr is empty
    And no file changed since the last run

    Examples:
      | name       | heading                           |
      | coordinate | # Coordinating with itos          |
      | work       | # Working with itos               |
      | brief      | # Specifying and briefing an item |
      | land       | # Landing a result                |
      | repair     | # Repairing what is red           |
      | decide     | # Deciding with the person        |
      | commit     | # Committing with itos            |
      | push       | # Pushing with itos               |

  # The task fixture supplies a real policy config, not just a notes file.
  # Compare the public raw commit baseline even when valid policy, missing
  # override, directory override or explicit/environment bad config is present.
  # The --root rows also exercise global parsing at both positions. broken.yaml
  # is invalid YAML in each effective directory; a selected absent config is
  # equally irrelevant to raw export. No source line may leak to either stream.
  @wip @slice-168 @ID-GUIDE-37
  Scenario Outline: Raw export ignores override and config failures and never appends real commit policy
    Given a repository whose ledger has the task "T-001"
    And the config's commits.types gains "deps"
    And a "guides" folder
    And a "sub" folder
    And the config sets "guide" to the YAML "{ overrides: { commit: <override> } }"
    And the untracked file "broken.yaml" holding "version: ["
    And the untracked file "sub/broken.yaml" holding "version: ["
    When itos runs "guide commit --builtin"
    Then itos exits with code 0
    And stdout begins with "# Committing with itos"
    And stdout does not say "## The rules the config sets"
    And stderr is empty
    And the last raw export is saved for comparison
    Given ITOS_CONFIG is "<environment-config>"
    When itos runs "<command>"
    Then itos exits with code 0
    And stdout equals the saved raw export
    And stdout does not say "## The rules the config sets"
    And stderr is empty
    And no file changed since the last run

    Examples:
      | override          | environment-config | command                                              |
      | guides/missing.md |                    | guide commit --builtin                               |
      | guides            |                    | guide commit --builtin                               |
      | guides/missing.md |                    | --config itos.yaml guide commit --builtin             |
      | guides/missing.md |                    | --config broken.yaml guide commit --builtin           |
      | guides/missing.md | broken.yaml        | guide commit --builtin                               |
      | guides/missing.md |                    | --config missing.yaml guide commit --builtin          |
      | guides/missing.md | missing.yaml       | guide commit --builtin                               |
      | guides/missing.md |                    | --root sub --config broken.yaml guide commit --builtin |
      | guides/missing.md |                    | guide commit --builtin --root sub --config broken.yaml |

  # This is a subprocess/launcher scenario, not proof from cli.Main alone.
  # The existing release-server helpers make fake releases observable. Save
  # the invoked binary's public baseline before configuring the fake pin.
  # Capture cache contents/existence separately: noFileChanged snapshots w.dir
  # including .git, whereas ITOS_CACHE lives under w.support. Save cache state
  # after fixture preparation, without creating a missing cache directory.
  # Test pin, environment version, then absent-config latest selection. Nothing
  # from any of them may run, be requested or write cache/latest/notice state.
  @wip @slice-168 @ID-GUIDE-38
  Scenario: Raw export never selects a pinned environment or latest release or changes cache state
    Given a repository whose ledger has the task "T-001"
    When itos runs "guide work --builtin"
    Then itos exits with code 0
    And the last raw export is saved for comparison
    Given a release server offering the versions "0.0.1" and "0.0.2"
    And the config pins the version "0.0.1" of the release server
    And the raw export cache state is saved for comparison
    When itos runs "guide work --builtin"
    Then itos exits with code 0
    And stdout equals the saved raw export
    And stderr is empty
    And no version of the release server ran since the last run
    And the release server was asked for nothing since the last run
    And the raw export cache state is unchanged
    And no file changed since the last run
    Given ITOS_VERSION is "0.0.2"
    When itos runs "guide work --builtin"
    Then itos exits with code 0
    And stdout equals the saved raw export
    And stderr is empty
    And no version of the release server ran since the last run
    And the release server was asked for nothing since the last run
    And the raw export cache state is unchanged
    And no file changed since the last run
    Given the repository has no itos.yaml
    And ITOS_VERSION is ""
    When itos runs "guide work --builtin"
    Then itos exits with code 0
    And stdout equals the saved raw export
    And stderr is empty
    And no version of the release server ran since the last run
    And the release server was asked for nothing since the last run
    And the raw export cache state is unchanged
    And no file changed since the last run

  @wip @slice-168 @ID-GUIDE-39
  Scenario Outline: Raw builtin export cannot also request a JSON wrapper
    Given a folder that is not a git repository
    When itos runs "<command>"
    Then itos exits with code 2
    And stdout is empty
    And stderr says "--builtin"
    And stderr says "--json"
    And no file changed since the last run

    Examples:
      | command                     |
      | guide work --builtin --json |
      | --json guide work --builtin |

  # Start with the new successful export so the scenario cannot be an
  # already-passing negative guard. The switch does not invent go arguments,
  # add guide names or suppress existing arity/name/unknown-flag validation.
  @wip @slice-168 @ID-GUIDE-40
  Scenario Outline: Builtin export preserves guide name and argument validation without adding a go flag
    Given a folder that is not a git repository
    When itos runs "guide work --builtin"
    Then itos exits with code 0
    And stdout begins with "# Working with itos"
    And stderr is empty
    When itos runs "<command>"
    Then itos exits with code 2
    And stdout is empty
    And stderr says "<offending>"
    And no file changed since the last run

    Examples:
      | command                            | offending   |
      | guide --builtin                    | name        |
      | guide review --builtin             | review      |
      | guide work extra --builtin         | extra       |
      | guide work --builtin --not-a-flag   | --not-a-flag |
      | go --builtin                       | --builtin   |
