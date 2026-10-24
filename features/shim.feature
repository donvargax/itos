@phase-3
Feature: The git shim, git commit and git push as itos's in an itos repository
  With itos linked as git before the real git on the PATH, itos acts as the
  shim whenever it is started under the name git (the user's call,
  2026-10-03: one binary, busybox's trick, so the shim is the itos version
  in effect). In a repository itos manages, an itos.yaml at its top or a
  stealth config, from any folder of it, git commit runs as itos commit and
  git push as itos push; every other command (but a git add there that
  stages more than the files it names, slice 128), and every command in
  every other repository, runs the real git, the first git on the PATH that
  is not an itos, with the same arguments, stdin, terminal and exit code.
  itos git-shim install links it into a folder and says whether that folder
  comes before the real git on the PATH. Per machine and opt-in: the hooks and
  CI's verify stay the gates.

  Background:
    Given a repository whose ledger has the task "T-001"
    And itos is linked as git before the real git on the PATH
    And a change to "README.md" is staged

  @ID-SHIM-01 @slice-41
  Scenario: In an itos repository git commit is itos commit, refusing a commit its rules refuse before git runs
    When git runs "commit -m 'chore: tidy the readme'"
    Then git exits with code 1
    And its output says "--task"
    And no commit was made

  @ID-SHIM-02 @slice-41
  Scenario: git commit takes itos commit's flags
    When git runs "commit --task T-001 -m 'chore: tidy the readme'"
    Then git exits with code 0
    And the message of HEAD has the footer "Task: T-001"

  @ID-SHIM-03 @slice-41
  Scenario: From a subfolder of an itos repository git commit is still itos commit
    Given a "sub" folder
    When git runs "commit -m 'chore: tidy the readme'" from "sub"
    Then git exits with code 1
    And its output says "--task"

  @ID-SHIM-04 @slice-41
  Scenario: In an itos repository git push is itos push, which never forces
    When git runs "push --force"
    Then git exits with code 2
    And its output says "force"

  @ID-SHIM-05 @slice-41
  Scenario: Every other command runs the real git, its exit code handed back
    When git runs "rev-parse --verify no-such-ref"
    Then git exits with code 128

  @ID-SHIM-06 @slice-41
  Scenario: In a repository itos does not manage git commit is the real git's
    Given a repository with no itos config, its change to "notes.md" staged
    When git runs "commit -m 'whatever I like'" in that repository
    Then git exits with code 0
    And that repository's HEAD says "whatever I like"

  @ID-SHIM-07 @slice-41
  Scenario: itos git-shim install links itos as git in the folder named, and says where it stands on the PATH
    When itos runs "git-shim install --dir shim"
    Then itos exits with code 0
    And "shim/git" runs itos
    And its output says "PATH"

  # Bug 7 (slice 41 left it as p3-shim-old-pin): the shim hands commit and
  # push to the itos the repository pins, and an itos older than the shim
  # has no git-shim command, so every git commit there failed with unknown
  # command. Where the pin is older than the shim, git commit and git push
  # are the real git's, and the shim says why in one line.
  @ID-SHIM-08 @bug-7
  Scenario: In a repository pinned to an itos older than the shim, git commit is the real git's
    Given a release server offering the versions "1.9.0" and "2.0.0"
    And the config pins the version "2.0.0" of the release server
    When git runs "commit -m 'chore: tidy the readme'"
    Then git exits with code 0
    And no version of the release server ran
    And its output says "2.0.0"

  # Slice 128 (p1-shim-refuses-hook-skipping, merged with p1-agent-rules-as-checks,
  # the user's calls of 2026-10-06 and 2026-10-11): the shim exists so a
  # session cannot skip the gates, yet the devcontainer test of 2026-10-06
  # committed and pushed junk through it with git commit --no-verify, -n and
  # git -c core.hooksPath=<elsewhere>. itos commit and itos push, and so the
  # shim, now refuse --no-verify, -n (commit's short form) and a -c that sets
  # core.hooksPath, exit 2 naming what was refused; itos commit refuses -a and
  # --all, which stage every tracked change; and in an itos repository the
  # shim refuses the sweeping stages AGENTS.md's block warns of (git add -A,
  # --all, ., -u, --update, a folder), exit 2 naming git add <path>, while an
  # explicit file path runs the real git as before. A person at a terminal is
  # refused the same; no config switch until someone needs one. The forms
  # without a scenario (., -u, --update, --all, -n on push) are judged by
  # unit tests. /usr/bin/git with --no-verify stays out of reach: the server's
  # branch protection is that guarantee.
  @ID-SHIM-09 @slice-128
  Scenario: git commit --no-verify is refused, and nothing is committed
    When git runs "commit --no-verify --task T-001 -m 'chore: tidy the readme'"
    Then git exits with code 2
    And its output says "--no-verify"
    And no commit was made

  @ID-SHIM-10 @slice-128
  Scenario: git commit -a is refused, naming the explicit-path form
    When git runs "commit -a --task T-001 -m 'chore: tidy the readme'"
    Then git exits with code 2
    And its output says "-a"
    And no commit was made

  @ID-SHIM-11 @slice-128
  Scenario: A -c that moves core.hooksPath is refused for a commit
    When git runs "-c core.hooksPath=/nonexistent commit --task T-001 -m 'chore: tidy the readme'"
    Then git exits with code 2
    And its output says "core.hooksPath"
    And no commit was made

  @ID-SHIM-12 @slice-128
  Scenario: git push --no-verify is refused
    When git runs "push --no-verify"
    Then git exits with code 2
    And its output says "--no-verify"

  @ID-SHIM-13 @slice-128
  Scenario: git add -A is refused in an itos repository, naming git add with a path
    Given an untracked file "notes.md"
    When git runs "add -A"
    Then git exits with code 2
    And its output says "git add <path>"
    And the file "notes.md" is not staged

  @ID-SHIM-14 @slice-128
  Scenario: git add of a folder is refused in an itos repository
    Given a "sub" folder
    And an untracked file "sub/notes.md"
    When git runs "add sub"
    Then git exits with code 2
    And the file "sub/notes.md" is not staged

  @ID-SHIM-15 @slice-128
  Scenario: git add of a file by its path runs the real git
    Given an untracked file "notes.md"
    When git runs "add notes.md"
    Then git exits with code 0
    And the file "notes.md" is staged

  # Bug 128: git-shim --help says install replaces a link to another itos,
  # yet shim.LinksItos (internal/shim/install.go) read only symbolic links,
  # so a git that is a hard link to another itos (what install makes on
  # Windows when symbolic links are refused, and what an upgraded itos.exe
  # leaves behind as a different file) was refused by install, uninstall and
  # init --git-shim-dir alike. LinksItos's other becomes git.IsItos (bug 45's
  # check: a symbolic link named itos, or a Go binary built as itos, read and
  # never run), so a hard link or a copy of another itos is another itos;
  # anything else is still refused. The other itos is a stamped build
  # (stampedItos) copied into the scenario's folder first, so the hard link
  # never reaches the shared stamped cache and stays on one filesystem; the
  # program that is not itos is a script, which has no build information.
  # "shim/git" is git.exe on Windows: every step naming the link reads it
  # through programPath, as runsItos does (the file steps do not, so a
  # removed link is "no git is left in", not "the file … does not exist").
  @ID-SHIM-16 @bug-128 @wip
  Scenario: install replaces a git that is a hard link to another itos
    Given "shim/git" is a hard link to another itos
    When itos runs "git-shim install --dir shim --json"
    Then itos exits with code 0
    And the output's JSON field "action" is "replaced"
    And "shim/git" runs itos
    And the other itos is unchanged

  @ID-SHIM-17 @bug-128 @wip
  Scenario: uninstall removes a git that is a hard link to another itos
    Given "shim/git" is a hard link to another itos
    When itos runs "git-shim uninstall --dir shim --json"
    Then itos exits with code 0
    And the output's JSON field "action" is "removed"
    And no git is left in "shim"
    And the other itos is unchanged

  @ID-SHIM-18 @bug-128 @wip
  Scenario Outline: A git that is a hard link to a program that is not itos is still refused
    Given "shim/git" is a hard link to a program that is not itos
    When itos runs "git-shim <command> --dir shim --json"
    Then itos exits with code 1
    And the output's JSON field "action" is "refused"
    And "shim/git" is still that program

    Examples:
      | command   |
      | install   |
      | uninstall |
