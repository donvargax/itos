@phase-1
Feature: The pre-push hook judges a new ref from the nearest commit the remote has, and never lets a tag move
  A ref the remote does not have yet (git's line gives its remote SHA as
  zeros) had no remote commit to compare with, so the hook ran
  hooks.pre_push's whole command for every new branch and tag: here
  golangci-lint, staticcheck and the unit tests over everything. Under
  push.mode pr every item's first push makes a new branch, so a docs-only
  spec ran the whole suite, and a release tag on a commit already on main
  ran it again for nothing. And slice 128's rewrite check refuses only a
  ref whose remote commit is not an ancestor of the one pushed, so a forced
  move of a release tag onto a later commit passed it, through the real git
  past the shim and the guard. Slice 182 (p1-prepush-new-ref-base, with
  p1-prepush-refuses-tag-moves merged in by the user's grooming of
  2026-10-24) judges each ref git hands the hook by its kind.

  A tag is a line whose remote ref is under refs/tags/, whatever the local
  ref (git push origin main:refs/tags/v1 makes a tag on the remote); every
  other ref, refs/heads/ and the rest, is judged as a branch is, as before.

  The tag rule comes first, over every line git gives, a deleted ref's
  included: a tag the remote already has (the remote SHA on its line is not
  zeros) is never updated. A move is refused whether or not the new commit
  builds on the old, and whether or not this clone has the old; a deletion
  (the local SHA zeros) is refused too, since deleting a tag and pushing it
  again is a move in two pushes, the second of which is a new tag the hook
  cannot tell from a first, and since what the remote's tag named is
  already someone's (a module proxy, a release page). git hands the hook a
  tag update whether or not it is forced, and rejects an unforced one
  itself only afterwards, so the hook's refusal is what a person pushing with
  git reads in both cases. itos push --tag over a tag the remote holds
  (ID-PUSH-13) never reaches the hook: it refuses first, itself, before git
  push runs (slice 131), and still exits 1 naming the tag. The refusal goes
  to stderr, as slice 128's
  does, its first line naming the ref and the remote's commit:
  "itos: the push would move the remote's tag refs/tags/<name> (at
  <short sha>) to <short sha>", or "would delete the remote's tag
  refs/tags/<name> (at <short sha>)", then "; a tag the remote has is never
  moved or deleted, so the push is refused, and hooks.pre_push's commands
  did not run."; a move adds the line "Tag the new commit with a new name
  instead." For an annotated tag the "(at <short sha>)" and "to <short sha>"
  are those of the tag objects, which git's line gives, as slice 131's
  refusal words them. It exits 1, before the config is read, so under every config
  as slice 128's refusal, and with no verify and no command run. itos
  offers no way round it: a tag that truly must go is deleted on the
  remote's host, by hand (the real git's --no-verify skips every hook, as
  slice 128 says; a tag protection rule on the host is that guarantee).
  A tag pushed again at the commit the remote
  already has never reaches the hook: git leaves an up-to-date ref out.

  Slice 128's rewrite check then judges the remaining refs as before. A
  push with a refused tag never reaches it, so a push makes one refusal,
  the tag's, and a tag never gets the rewrite's advice to run itos push,
  which rebases branches and has nothing to offer a tag. A deleted branch
  is still not refused.

  A new ref, branch or tag alike, adds the commits of the commit it names
  (an annotated tag's commit, not its tag object) that are on no
  remote-tracking branch: git rev-list <sha> --not --remotes, the very set
  the hook's verify already judges for a new ref (pre-push.feature,
  ID-PREPUSH-04). Its bases are that range's boundary, the commits git
  rev-list --boundary marks, less any that is an ancestor of another (git
  merge-base --independent): the remote commits the ref builds on.

    - It adds no commit (a release tag on main's commit, a new branch at a
      remote branch's head): it brings no base and asks for no whole run.
      A push whose refs all add nothing runs no pre-push command and prints
      nothing, as a deleted branch's push does.
    - It has bases: hooks.pre_push's per_base runs once for each, with
      {base} its full SHA, deduplicated with the other refs' bases, as an
      existing branch's remote commit is. A merge of two remote branches
      has two bases, and per_base runs for each: each is a remote commit
      the push builds on, and per_base takes one base, so the push is
      judged against each line it joins, as a push updating two branches
      already is; their merge base would be a commit the push does not
      build on, and would judge again what the remote already has.
    - It adds commits and has no base (no remote-tracking ref at all, as
      in a new project's first push, or history unrelated to every remote
      branch): whole, as before.

  Any ref that asks for whole makes the push run whole alone, as before.
  An existing branch is judged from its remote commit, as before. What the
  hook prints for a command it runs is unchanged: "$ <command>" before
  each.

  The remote-tracking refs are every remote's (refs/remotes/*), not only
  those of the remote pushed to: they are the set verify's range already
  uses, so per_base checks what verify judged, and the remote argument git
  passes may be a URL with no refs of its own. The hook fetches nothing
  (a fetch is slow, may need credentials, and a hook should not move the
  clone's refs); it trusts the refs as the clone last saw them. itos push
  fetches before it pushes, so there they are current. A stale ref behind
  its remote only makes a base older, so more is judged, never less; one
  left for a branch since deleted on the remote (no fetch --prune) gives
  a base whose commits were pushed, and judged, once. A clone with no
  remote-tracking ref has no base, and runs whole.

  itos help hook pre-push and the hook package's doc say this. No scenario
  or corpus case of v8.17.0 changes: its corpus's new-branch cases have no
  remote-tracking ref and still run whole, and no scenario tells per_base
  from whole or moves a tag. So the work is a feat, with no Changes: or
  BREAKING-CHANGE: footer.

  The steps. "hooks.pre_push's commands record which of them ran, and with
  which base" sets hooks.pre_push in the clone's config to per_base
  printf '%s\n' "per_base {base}" and whole printf '%s\n' whole, each
  appending to the file "none of hooks.pre_push's commands ran" reads,
  committed as "chore: record the pre-push commands" (Task: T-001) and
  pushed to the remote's main past the hooks, as the step that records
  that they ran does. "hooks.pre_push's per_base alone ran, once, with the
  base <subject>" checks the file holds that one line, its SHA the full
  SHA of the commit with the subject in the clone (git log --all);
  "hooks.pre_push's per_base alone ran, twice, with the bases <a> and <b>"
  the two lines in either order; "hooks.pre_push's whole alone ran, once"
  the one line whole. "the remote has gained the branch <name> with the
  commit <subject> touching <path>" makes the branch in the other clone
  the remote gains commits from, at its main's head, commits there as that
  step does and pushes the branch, leaving the other clone on its main.
  "the clone has merged the remote's branch <name>" is git merge --no-ff
  --no-edit --no-verify origin/<name>: a merge with no change of its own,
  which verify passes whatever its header (ID-CMSG-09). "the clone's
  remote origin is a new repository with no branches, of which the clone
  tracks none" points origin at a new empty bare repository and deletes
  the clone's refs/remotes/origin/ refs, as a new project's clone has none.
  "the remote's tag <name> names its main's head" checks the remote's
  refs/tags/<name>, peeled to its commit, is the remote's main.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the pre-push hook is installed
    And hooks.pre_push's commands record which of them ran, and with which base

  # The case the slice is for: an item's first push under push.mode pr.
  # The base is the remote's main as the clone fetched it, the commit the
  # branch was made from. Before the work, whole runs.
  @ID-PUSHREF-01 @slice-182
  Scenario: A new branch made from the remote's main runs per_base from main's head, not whole
    Given the remote has gained the commit "docs: the remote's work" touching "OTHER.md"
    And the clone has fetched the remote
    And the clone is on the new branch "topic", tracking the remote's main
    And the commit "chore: tidy the readme" naming the task "T-001"
    When git pushes HEAD to the remote's new branch "topic"
    Then git exits with code 0
    And hooks.pre_push's per_base alone ran, once, with the base "docs: the remote's work"

  # The side branch is made before main gains its commit, so neither base
  # is an ancestor of the other and both stay.
  @ID-PUSHREF-02 @slice-182
  Scenario: A new branch merging two remote branches runs per_base once from each
    Given the remote has gained the branch "side" with the commit "docs: the side's work" touching "SIDE.md"
    And the remote has gained the commit "docs: the remote's work" touching "OTHER.md"
    And the clone has fetched the remote
    And the clone is on the new branch "topic", tracking the remote's main
    And the clone has merged the remote's branch "side"
    When git pushes HEAD to the remote's new branch "topic"
    Then git exits with code 0
    And hooks.pre_push's per_base alone ran, twice, with the bases "docs: the remote's work" and "docs: the side's work"

  # A release: the tag names the clone's HEAD, which the Background pushed
  # to the remote's main. Before the work, whole runs.
  @ID-PUSHREF-03 @slice-182
  Scenario: A new tag on a commit the remote has runs no pre-push command
    Given the clone has the annotated tag "v1.0.0" on its HEAD
    When itos runs "push --tag v1.0.0"
    Then itos exits with code 0
    And the remote has the tag "v1.0.0"
    And none of hooks.pre_push's commands ran

  # A tag is judged as a new branch is, not skipped for its kind: the tagged
  # commit is the clone's own, built on the remote's main.
  @ID-PUSHREF-04 @slice-182
  Scenario: A new tag on a commit the remote lacks runs per_base from the nearest commit the remote has
    Given the remote has gained the commit "docs: the remote's work" touching "OTHER.md"
    And the clone has fetched the remote
    And the clone is on the new branch "topic", tracking the remote's main
    And the commit "chore: tidy the readme" naming the task "T-001"
    And the clone has the annotated tag "v1.0.0" on its HEAD
    When the real git runs "push origin refs/tags/v1.0.0"
    Then the real git exits with code 0
    And hooks.pre_push's per_base alone ran, once, with the base "docs: the remote's work"

  # A guard: with no remote-tracking ref there is no base, as before.
  @ID-PUSHREF-05 @slice-182
  Scenario: A new branch on a remote with no branches runs whole, as before
    Given the clone's remote origin is a new repository with no branches, of which the clone tracks none
    When git pushes HEAD to the remote's new branch "main"
    Then git exits with code 0
    And hooks.pre_push's whole alone ran, once

  # A guard: itos push rebases onto the remote's commit and pushes main,
  # whose remote commit is the base, as before.
  @ID-PUSHREF-06 @slice-182
  Scenario: An existing branch's push runs per_base from the remote's commit, as before
    Given the remote has gained the commit "docs: the remote's work" touching "OTHER.md"
    And the commit "chore: tidy the readme" naming the task "T-001"
    When itos runs "push"
    Then itos exits with code 0
    And hooks.pre_push's per_base alone ran, once, with the base "docs: the remote's work"

  # The hole: the remote's tag names an ancestor of the new commit, so
  # slice 128's rewrite check passes it, and before the work the forced
  # push moves the tag.
  @ID-PUSHREF-07 @slice-182
  Scenario: A forced move of a tag the remote has onto a later commit is refused, and the remote keeps it
    Given the remote's head is tagged "v1.0.0"
    And the commit "chore: tidy the readme" naming the task "T-001"
    And the clone has the annotated tag "v1.0.0" on its HEAD
    When the real git runs "push --force origin refs/tags/v1.0.0"
    Then the real git exits with code 1
    And its output says "a tag the remote has is never moved or deleted"
    And none of hooks.pre_push's commands ran
    And the remote's tag "v1.0.0" names its main's head

  # The clone has not fetched the remote's tagged commit, so before the work
  # slice 128's check refuses it, telling the person to run itos push; the
  # tag's refusal replaces that one, never adds to it.
  @ID-PUSHREF-08 @slice-182
  Scenario: A tag moved onto a commit not built on it gets the tag's refusal alone, not the rewrite's
    Given the remote has gained the commit "docs: the remote's work" touching "OTHER.md"
    And the remote's head is tagged "v1.0.0"
    And the commit "chore: tidy the readme" naming the task "T-001"
    And the clone has the annotated tag "v1.0.0" on its HEAD
    When the real git runs "push --force origin refs/tags/v1.0.0"
    Then the real git exits with code 1
    And its output says "a tag the remote has is never moved or deleted"
    And its output does not say "itos push"
    And the remote's tag "v1.0.0" names its main's head

  # A deletion and a new push of the name would move the tag in two steps.
  # Before the work the hook leaves a deleted ref out, and the tag goes.
  @ID-PUSHREF-09 @slice-182
  Scenario: Deleting a tag the remote has is refused
    Given the remote's head is tagged "v1.0.0"
    When the real git runs "push origin :refs/tags/v1.0.0"
    Then the real git exits with code 1
    And its output says "a tag the remote has is never moved or deleted"
    And the remote's tag "v1.0.0" names its main's head
