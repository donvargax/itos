@phase-3
Feature: The git shim, git commit and git push as itos's in an itos repository
  With itos linked as git before the real git on the PATH, itos acts as the
  shim whenever it is started under the name git (the user's call,
  2026-10-03: one binary, busybox's trick, so the shim is the itos version
  in effect). In a repository itos manages, an itos.yaml at its top or a
  stealth config, from any folder of it, git commit runs as itos commit and
  git push as itos push; every other command, and every command in every
  other repository, runs the real git, the first git on the PATH that is not
  an itos, with the same arguments, stdin, terminal and exit code. itos
  git-shim install links it into a folder and says whether that folder comes
  before the real git on the PATH. Per machine and opt-in: the hooks and
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
