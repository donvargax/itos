@phase-3
Feature: itos guard claude-code, Claude Code's guard against git commit and git push
  An agent that commits or pushes with git by hand skips what itos commit
  writes (the footers) and what itos push checks (no stash, no force, no
  stopped rebase). Claude Code asks a PreToolUse hook before each tool runs,
  sending the tool's name, its input and the session's folder as JSON on
  stdin; the itos plugin's hook is itos guard claude-code, so a
  repository's pin picks the version that answers (PLAN.md §10,
  "Adoption"). In a repository itos manages, found from the folder the input
  names as everywhere else, a Bash command that runs git commit or git push
  is denied, the reason naming the itos command to use instead; Claude Code
  shows the reason to the agent. Everything else gets no answer at all, so
  Claude Code's own permission rules decide as if no hook ran: the guard
  never allows anything. It reads the command as a shell would, word by
  word, so git's options before the subcommand and a command later in a
  chain are caught, and a git commit that is only text an argument carries
  is not. A guardrail for agents that follow it, not a fortress (the user's
  call, 2026-10-03): a command hidden in sh -c or a script is not looked
  into, and the commit-msg hook and CI's verify stay the gates.

  Background:
    Given a repository whose ledger has the task "T-001"

  @ID-GUARD-01 @slice-42
  Scenario: In an itos repository an agent's git commit is denied, the reason naming itos commit
    When Claude Code asks itos about the Bash command "git commit -m 'chore: tidy the readme'"
    Then itos exits with code 0
    And itos denies the command, its reason saying "itos commit --task"

  @ID-GUARD-02 @slice-42
  Scenario: In an itos repository an agent's git push is denied, the reason naming itos push
    When Claude Code asks itos about the Bash command "git push origin main"
    Then itos exits with code 0
    And itos denies the command, its reason saying "itos push"

  # A permission rule matches a command's prefix, which is why this is a
  # hook: git -C . commit and git -c key=value push slip past a rule.
  @ID-GUARD-03 @slice-42
  Scenario: git's options before the subcommand do not hide it
    When Claude Code asks itos about the Bash command "git -C . -c core.editor=true commit -m 'chore: tidy the readme'"
    Then itos exits with code 0
    And itos denies the command, its reason saying "itos commit"

  @ID-GUARD-04 @slice-42
  Scenario: A git push later in a chain of commands is denied
    When Claude Code asks itos about the Bash command "go vet ./... && git pull --rebase && git push"
    Then itos exits with code 0
    And itos denies the command, its reason saying "itos push"

  @ID-GUARD-05 @slice-42
  Scenario: From a subfolder of an itos repository the command is still denied
    Given a "sub" folder
    When Claude Code asks itos about the Bash command "git commit -m 'chore: tidy the readme'" run in "sub"
    Then itos exits with code 0
    And itos denies the command, its reason saying "itos commit"

  # No answer, not an allow: an allow would skip the person's own permission
  # rules for every command the guard lets through.
  @ID-GUARD-06 @slice-42
  Scenario: Every other git command gets no answer
    When Claude Code asks itos about the Bash command "git status --short && git log --oneline -3"
    Then itos exits with code 0
    And itos writes nothing to stdout

  @ID-GUARD-07 @slice-42
  Scenario: A command that only names git commit in its arguments gets no answer
    When Claude Code asks itos about the Bash command "grep -n 'git commit' AGENTS.md"
    Then itos exits with code 0
    And itos writes nothing to stdout

  @ID-GUARD-08 @slice-42
  Scenario: In a repository itos does not manage git commit gets no answer
    Given a repository with no itos config, its change to "notes.md" staged
    When Claude Code asks itos about the Bash command "git commit -m 'whatever I like'" in that repository
    Then itos exits with code 0
    And itos writes nothing to stdout

  @ID-GUARD-09 @slice-42
  Scenario: A tool other than Bash gets no answer
    When Claude Code asks itos about the tool "Edit" on the file "README.md"
    Then itos exits with code 0
    And itos writes nothing to stdout

  # Claude Code blocks the tool on exit code 2 and goes on after any other
  # failure, so a guard that cannot read its input must not exit 2.
  @ID-GUARD-10 @slice-42
  Scenario: An input itos cannot read blocks nothing
    When Claude Code sends itos "not json" as a PreToolUse input
    Then itos exits with code 1
    And itos writes nothing to stdout
    And its output says "PreToolUse"

  # Slice 42 left it as p3-guard-old-pin: the launcher hands the hook to the
  # itos a repository pins, and an itos older than the guard has no such
  # hook: its usage error exits 2, which Claude Code takes as a block, so
  # every Bash command there would be blocked once the plugin wires the hook. Where the version the hook would
  # go to predates the guard, the command gets no answer, as bug 7 did for
  # the git shim. Outside a project the launcher runs the newest release,
  # never one older than itself, so that case cannot arise for an itos that
  # has the guard (the scenario written for it, ID-GUARD-12, passed before
  # any work and was dropped).
  @ID-GUARD-11 @slice-44
  Scenario: In a repository pinned to an itos older than the guard, the command gets no answer
    Given a release server offering the versions "1.9.0" and "2.0.0"
    And the config pins the version "2.0.0" of the release server
    When Claude Code asks itos about the Bash command "git commit -m 'chore: tidy the readme'"
    Then itos exits with code 0
    And itos writes nothing to stdout
    And no version of the release server ran

  # Slice 89 (decision of q-16): Claude Code's PreToolUse entry point was
  # itos hook pre-tool-use, in the group that holds git's hooks, under an
  # exit-code contract that is not git's. It is itos guard claude-code, a
  # group named for what it does and the harness whose protocol it speaks,
  # so another harness's guard is itos guard <harness>. The steps that ask
  # itos as Claude Code call the new name; the plugin's guard.sh calls it,
  # and the plugin's version rises. The old name exits 2 naming the new one;
  # guard.sh turns a 2 into 1, so Claude Code is never blocked by it.
  @ID-GUARD-12 @slice-89
  Scenario: itos hook pre-tool-use exits 2, naming itos guard claude-code
    When itos runs "hook pre-tool-use"
    Then itos exits with code 2
    And its output says "itos guard claude-code"
