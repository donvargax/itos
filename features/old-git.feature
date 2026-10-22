@phase-1
Feature: itos's hooks on a git that runs no hook its config declares
  itos declares its hooks in the clone's git config (config.feature), which
  git runs from 2.54.0 on. On an older git, down to 2.51, the oldest itos
  supports (the user, 2026-10-21), hook install writes them as hook files
  into the folder git runs hooks from (git rev-parse --git-path hooks, which
  follows core.hooksPath), each one running a hook the folder already held
  first and then itos's, and every command that commits or pushes refuses
  unless a hook file there calls itos. A hooks folder inside the work tree
  is the project's, whether git tracks its files or not: itos writes no file
  there, and says which line to add to each hook file instead. The git config stays the default and the only place itos writes
  on a git that runs its config's hooks (config.feature's ID-CONFIG-38):
  hook files are a compatibility path for older gits, kept a few months
  until most people's git runs config hooks (the user, 2026-10-22), and
  removed then.

  Background:
    Given a repository whose ledger has the task "T-001"
    And the git itos runs runs no hook its config declares, as git 2.51 does

  # Slice 165 (the user, 2026-10-21: git older than 2.54 is supported, 2.51
  # the oldest): hook install exited 3 on such a git, and so did every
  # command that commits or pushes, so itos could not be used there at all.
  @ID-OLDGIT-01 @slice-165
  Scenario: hook install writes itos's hooks into the hooks folder, declaring none in the git config
    When itos runs "hook install"
    Then itos exits with code 0
    And the file ".git/hooks/commit-msg" names "hook commit-msg"
    And the file ".git/hooks/pre-push" names "hook pre-push"
    And the git config declares no "commit-msg" hook
    And the git config declares no "pre-push" hook

  @ID-OLDGIT-02 @slice-165
  Scenario: The hook file hook install wrote runs itos's commit-msg check
    Given itos has installed the hooks
    And a change to "README.md" is staged
    When git commits with the message "chore: tidy the readme"
    Then itos exits with code 1
    And its output says "Task"
    And nothing was committed

  # A hook the folder already held is kept and runs first; when it refuses,
  # itos's check does not run and the commit stops, as git would stop at it.
  @ID-OLDGIT-03 @slice-165
  Scenario: A commit-msg hook the hooks folder held runs first, then itos's
    Given the hooks folder holds a commit-msg hook that records it ran
    And itos has installed the hooks
    And a change to "README.md" is staged
    When git commits with the message "chore: tidy the readme"
    Then itos exits with code 1
    And the project's commit-msg hook ran
    And its output says "Task"

  # A second install must not take itos's own hook file for the project's
  # and chain it again.
  @ID-OLDGIT-04 @slice-165
  Scenario: hook install twice changes nothing, the project's hook still running once
    Given the hooks folder holds a commit-msg hook that records it ran
    And itos has installed the hooks
    When itos runs "hook install"
    Then itos exits with code 0
    And its output says "unchanged"
    When a change to "README.md" is staged
    And git commits with the message "chore: tidy the readme"
    Then the project's commit-msg hook ran once

  # A hooks folder inside the work tree (a hook manager's, committed with the
  # project) is the project's: writing a file there would put a change in the
  # tree that some commit would carry. hook install writes nothing and names
  # each hook file and the line to add; the other hook is not written either,
  # so the clone is never half installed.
  @ID-OLDGIT-05 @slice-165
  Scenario: hook install refuses a hooks folder git tracks, naming the line to add
    Given the project's hooks are in ".husky" by core.hooksPath, with a commit-msg hook that records it ran
    When itos runs "hook install"
    Then itos exits with code 1
    And its output says ".husky/commit-msg"
    And its output says "hook commit-msg"
    And the file ".husky/commit-msg" has the same contents as its committed version
    And the file ".husky/pre-push" does not exist

  # The project adds the line itself: a hook file that calls itos is enough,
  # whoever wrote it.
  @ID-OLDGIT-06 @slice-165
  Scenario: itos commit accepts a project's hook file that calls itos's hook
    Given the project's hooks are in ".husky" by core.hooksPath, with a commit-msg hook that calls itos's commit-msg hook
    And a change to "README.md" is staged
    When itos runs the command line "commit --task T-001 -m 'chore: tidy the readme'"
    Then itos exits with code 0

  @ID-OLDGIT-07 @slice-165
  Scenario: itos commit refuses when no hook file in the hooks folder calls itos
    Given the hooks folder holds a commit-msg hook that records it ran
    And a change to "README.md" is staged
    When itos runs the command line "commit --task T-001 -m 'chore: tidy the readme'"
    Then itos exits with code 3
    And its output says "itos hook install"
    And nothing was committed

  # issue #16's case: core.hooksPath names a folder that does not exist (a
  # generated one, missing in a fresh worktree). hook install made it and its
  # hooks; a later reset of core.hooksPath to another folder leaves itos's
  # hooks out of git's reach, and itos push says so rather than pushing
  # unchecked.
  @ID-OLDGIT-08 @slice-165
  Scenario: itos push refuses when core.hooksPath moved away from the folder itos's hooks are in
    Given itos has installed the hooks
    And core.hooksPath is "missing/hooks"
    When itos runs "push --no-wait"
    Then itos exits with code 3
    And its output says "itos hook install"

  # A hooks folder inside the work tree is the project's even where git
  # tracks none of its files: a file itos added there would show in every
  # git status, for some commit to carry (the user, 2026-10-22).
  @ID-OLDGIT-09 @slice-165
  Scenario: hook install refuses a hooks folder inside the work tree that git does not track
    Given core.hooksPath is "githooks", a folder git does not track
    When itos runs "hook install"
    Then itos exits with code 1
    And its output says "githooks/commit-msg"
    And its output says "hook commit-msg"
    And the file "githooks/commit-msg" does not exist
