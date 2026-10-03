@phase-3
Feature: A stealth mode, itos kept in the git folder of a repository that does not use it
  One person can hold themselves to itos in a repository whose team does not
  use it, with nothing of it showing to anyone else: not in the tree, not in
  CI, not in the history. With no --config, no ITOS_CONFIG and no itos.yaml
  in the root, itos reads <git common dir>/itos/itos.yaml (git rev-parse
  --git-common-dir), with no environment variable to set, so every linked
  worktree shares it. The ledger, the work registry and the smoke sets that
  config names are read beside it, in that folder, which git never commits,
  so a footer naming a task is checked against the ledger file there, at
  every commit, since no commit carries it; no people file is read, the
  person being the only one, and itos work asks nobody who the session is,
  every item being theirs. A project's own itos.yaml in the root always
  wins: that is the project's mode. Where nothing is pinned, a
  global itos runs the newest release for a stealth config, as where there
  is no config at all (update.feature).

  Background:
    Given a repository whose ledger has the task "T-001", kept in its git folder

  @ID-STEALTH-01 @slice-30
  Scenario: With no itos.yaml in the root, itos reads the config in the git folder, and the ledger beside it
    When itos runs the task "T-001"
    Then itos exits with code 0
    And its output says "T-001"

  # The footer comes from itos commit, since the hook refuses one typed into
  # the message (@ID-STEALTH-10).
  @ID-STEALTH-02 @slice-30
  Scenario: The commit-msg hook checks a footer against the ledger beside the config, which no commit carries
    Given the commit-msg hook is installed
    And a change to "README.md" is staged
    When itos commits with the arguments "--task T-999 -m 'chore: tidy the readme'"
    Then itos exits with code 1
    And its output says "T-999"

  # A status the config does not list, so only reading that file can fail it.
  @ID-STEALTH-03 @slice-30
  Scenario: The work registry is read beside the config in the git folder
    Given the work registry beside the config has the item "slice-1" with the status "bogus"
    When itos checks the work registry
    Then itos exits with code 1
    And its output says "bogus"

  @ID-STEALTH-04 @slice-30
  Scenario: A linked worktree reads the same config in the git folder
    Given a linked worktree of the repository at "../wt"
    When itos runs the task "T-001" in the linked worktree
    Then itos exits with code 0
    And its output says "T-001"

  # The project's mode: a repository that adopts itos commits its config,
  # and the person's own copy stops applying.
  @ID-STEALTH-05 @slice-30
  Scenario: An itos.yaml in the root wins over the config in the git folder
    Given an itos.yaml in the root whose ledger has no task "T-001"
    When itos runs the task "T-001"
    Then itos exits with code 1

  @ID-STEALTH-06 @slice-30
  Scenario: Nothing of the stealth mode is in the tree git sees
    When itos runs the task "T-001"
    Then itos exits with code 0
    And git status shows nothing to commit

  @ID-STEALTH-07 @slice-30
  Scenario: A global itos runs the newest release for a stealth config that pins nothing
    Given a release server offering the versions "9.1.0" and "9.2.0"
    When itos runs "version"
    Then the version "9.2.0" ran with the arguments "version"

  # Slice 32: the footers live in notes. A footer in the message is what
  # would show to everyone, so in stealth mode itos commit writes the footers
  # its flags give as a note on the new commit, in refs/notes/itos, which git
  # push does not send unless asked; the commit-msg hook reads them from what
  # itos commit hands it, refuses a commit that needs one and was made without
  # itos commit, and refuses a footer typed into the message. itos sets
  # notes.rewriteRef so an amend or a rebase carries the note to the new
  # commit, and verify reads each commit's footers from its note.
  @ID-STEALTH-08 @slice-32
  Scenario: itos commit writes the task as a note on the new commit, never in its message
    Given the commit-msg hook is installed
    And a change to "README.md" is staged
    When itos commits with the arguments "--task T-001 -m 'chore: tidy the readme'"
    Then itos exits with code 0
    And the message of HEAD does not say "T-001"
    And the itos note on HEAD says "Task: T-001"

  @ID-STEALTH-09 @slice-32
  Scenario: A commit that needs a task, made without itos commit, is refused, saying how to make it
    Given the commit-msg hook is installed
    And a change to "README.md" is staged
    When git commits with the message "chore: tidy the readme"
    Then the commit is refused
    And its output says "itos commit --task"

  @ID-STEALTH-10 @slice-32
  Scenario: A footer typed into the message is refused, since it would show in the history
    Given a change to "README.md" is staged
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 1
    And its output says "itos commit --task"

  @ID-STEALTH-11 @slice-32
  Scenario: An amended commit keeps its note
    Given the commit-msg hook is installed
    And a change to "README.md" is staged
    And itos has committed with the arguments "--task T-001 -m 'chore: tidy the readme'"
    When git amends HEAD with the message "chore: tidy the readme again"
    Then the itos note on HEAD says "Task: T-001"

  @ID-STEALTH-12 @slice-32
  Scenario: verify reads each commit's footers from its note
    Given the commit-msg hook is installed
    And a change to "README.md" is staged
    And itos has committed with the arguments "--task T-001 -m 'chore: tidy the readme'"
    And the commit "chore: tidy the docs" on top of it
    When itos verifies the commits after the first
    Then itos exits with code 1
    And its output says "tidy the docs"
    And its output does not say "tidy the readme"

  # Slice 33: the hooks beside the project's own. Git runs a hook declared in
  # its config (hook.<name>.event and hook.<name>.command, in .git/config,
  # which is never committed) as well as the one in core.hooksPath or
  # .git/hooks, so itos's hooks run without touching the project's hook
  # files or settings, and a hook manager resetting core.hooksPath cannot
  # remove them. hooks install --manager git-config writes them, and is what
  # hooks install picks in stealth mode.
  @ID-STEALTH-13 @slice-33
  Scenario: In stealth mode hooks install declares itos's hooks in the git config
    When itos installs the hooks
    Then itos exits with code 0
    And the git config declares a "commit-msg" hook that runs itos
    And git status shows nothing to commit

  @ID-STEALTH-14 @slice-33
  Scenario: itos's hook runs beside the project's own, whose files and settings are untouched
    Given the project's hooks are in ".husky" by core.hooksPath, with a commit-msg hook that records it ran
    And itos has installed the hooks
    And a change to "README.md" is staged
    When git commits with the message "chore: tidy the readme"
    Then the commit is refused
    And the project's commit-msg hook ran
    And core.hooksPath is still ".husky"

  # Slice 34: the person's own commits. Others' commits in such a repository
  # follow no rules of theirs, so verify and ci plan given no range judge
  # the commits on no remote branch (HEAD --not --remotes): the person's,
  # not yet pushed.
  @ID-STEALTH-15 @slice-34
  Scenario: verify with no range judges only the commits no remote has
    Given a remote that has the commit "tidy the readme" with no task
    And a change to "README.md" is staged
    And itos has committed with the arguments "--task T-001 -m 'chore: tidy the docs'"
    When itos verifies with no range
    Then itos exits with code 0
    And its output says "1/1 commits pass the commit rules"

  @ID-STEALTH-16 @slice-34
  Scenario: ci plan with no range plans the commits no remote has
    Given a remote that has the commit "tidy the readme" with no task
    And the task "T-001" has the check "exit 0"
    And a change to "README.md" is staged
    And itos has committed with the arguments "--task T-001 -m 'chore: tidy the docs'"
    When itos plans CI with no range
    Then itos exits with code 0
    And its output says "T-001"

  # Slice 35: a stealth config's defaults fit one person. hooks.bin is itos,
  # the global launcher, whatever the config says of it (a project's default
  # stays tools/bin/itos through v2), and no people file is read: the person
  # is the only one.
  @ID-STEALTH-17 @slice-35
  Scenario: A stealth config reads no people file
    Given the config in the git folder names no people file
    When itos runs "work"
    Then itos exits with code 0
    And its output does not say "CONTRIBUTORS"

  @ID-STEALTH-18 @slice-35
  Scenario: A stealth config's hooks call the global itos
    When itos installs the hooks
    Then itos exits with code 0
    And the git config declares a "commit-msg" hook whose command starts with "itos "

  # Slice 36: in stealth mode only the links move to the note; a footer of
  # free text is content, written into the message by its flag.
  @ID-STEALTH-19 @slice-36
  Scenario: In stealth mode a footer of free text stays in the message, the task in the note
    Given the config requires an "Upgrading" footer of free text for "chore"
    And the commit-msg hook is installed
    And a change to "README.md" is staged
    When itos commits with the arguments "--task T-001 --upgrading none -m 'chore: tidy the readme'"
    Then itos exits with code 0
    And the message of HEAD has the footer "Upgrading: none"
    And the message of HEAD does not say "T-001"
    And the itos note on HEAD says "Task: T-001"

  # Slice 37: the hook is told an amend, not left to guess. itos commit sees
  # --amend among the arguments it hands git, and tells the hook, which then
  # judges the amend by HEAD's note. Slice 32's guess from the author's name,
  # email and date matching HEAD's stays only for a commit made without itos
  # commit; an amend that sets another date defeats the guess, so only the
  # hook being told can let this one through. (prepare-commit-msg was weighed
  # and dropped: git tells it nothing of an amend whose message comes from -m
  # or -F.)
  @ID-STEALTH-20 @slice-37
  Scenario: An amend through itos commit that sets another author date is known for an amend, and keeps its note
    Given the commit-msg hook is installed
    And a change to "README.md" is staged
    And itos has committed with the arguments "--task T-001 -m 'chore: tidy the readme'"
    When itos commits with the arguments "--amend --date=2001-01-01T00:00:00 -m 'chore: tidy the readme again'"
    Then itos exits with code 0
    And the itos note on HEAD says "Task: T-001"

  # Slice 38: the person is the only one, so a stealth session owns every
  # item, whoever an item names and whoever gh says the session is.
  @ID-STEALTH-21 @slice-38
  Scenario: A stealth session is proposed an item another owner holds, with no identity to look up
    Given the work registry beside the config has the item "slice-1" owned by "someone-else" with the status "todo"
    And no identity can be looked up
    When itos runs "work"
    Then itos exits with code 0
    And itos proposes "slice-1" to start
