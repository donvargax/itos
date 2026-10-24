@phase-3
Feature: itos push, the pull-rebase-push routine as one command
  Every session finished the same way by hand: commit, git pull --rebase
  --no-autostash, check that no rebase stopped and no conflict is left, then
  push in a separate command, never piped and never forced. itos push is
  that routine, built in since it is core (the user's call, 2026-10-03:
  extensions are for what is not). It rebases onto the branch's upstream with
  its own flags, so git's pull.rebase and rebase.autostash settings change
  nothing; it refuses to start with uncommitted changes to tracked files,
  which autostash would otherwise pocket; a rebase that stops leaves the
  rebase for the person to finish and pushes nothing; and it never forces:
  a push the remote refuses is reported, not retried with force. The hooks
  run as for any push. It is itos commit's other half.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs

  # pull.rebase false would make a plain git pull merge; itos push's own
  # flags are what keep the history a line.
  @ID-PUSH-01 @slice-39
  Scenario: The local commits are rebased onto the remote's and pushed, the history a line
    Given the clone's git config says "pull.rebase" is "false"
    And the remote has gained the commit "chore: tidy the docs" touching "docs.md"
    And the clone has the commit "chore: tidy the readme" touching "README.md"
    When itos runs "push"
    Then itos exits with code 0
    And the remote's branch ends with "chore: tidy the docs" then "chore: tidy the readme", with no merge commit

  @ID-PUSH-02 @slice-39
  Scenario: A rebase that stops on a conflict pushes nothing and says how to go on
    Given the remote has gained the commit "chore: reword the readme" touching "README.md"
    And the clone has the commit "chore: tidy the readme" touching "README.md"
    When itos runs "push"
    Then itos exits with code 1
    And its output says "git rebase --continue"
    And the remote's branch does not have "chore: tidy the readme"

  # rebase.autostash true would stash the change and could leave it stashed;
  # itos push refuses before pulling instead.
  @ID-PUSH-03 @slice-39
  Scenario: Uncommitted changes to tracked files are refused before anything is pulled
    Given the clone's git config says "rebase.autostash" is "true"
    And the remote has gained the commit "chore: tidy the docs" touching "docs.md"
    And the clone has the commit "chore: tidy the readme" touching "README.md"
    And the clone's "README.md" has an uncommitted change
    When itos runs "push"
    Then itos exits with code 1
    And its output says "commit"
    And the remote's branch does not have "chore: tidy the readme"
    And the clone's "README.md" still has its uncommitted change

  @ID-PUSH-04 @slice-39
  Scenario: itos push never forces
    When itos runs "push --force"
    Then itos exits with code 2
    And its output says "force"

  # Slice 66: two people taking the same item edit the same lines of the
  # registry, so the second push's rebase stops on the conflict and nothing
  # lands twice; the conflict is the lock. itos push names the registry and
  # says what happened in a person's words, rather than git's alone.
  @ID-PUSH-05 @slice-66
  Scenario: A take that meets another person's take of the same item stops, and says the item was taken
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the remote has gained the commit "docs: take slice-9" making "ana" the owner of "slice-9"
    And the clone has the commit "docs: take slice-9" making "bo" the owner of "slice-9"
    When itos runs "push"
    Then itos exits with code 1
    And its output says "tasks/work-items.yaml"
    And its output says "taken"
    And the remote's registry gives "slice-9" the owner "ana"

  # Bug 118 (#107): two sessions that each add a different item append at the
  # registry's end, so the rebase stops on a textual conflict though no item
  # clashes. The take-clash advice, git rebase --skip, would drop the whole
  # commit; with no clash the push says to keep both sides and continue.
  # ID-PUSH-05's wording and advice for a real clash stay as they are.
  @ID-PUSH-15 @bug-118
  Scenario: Two different items added on both sides are not called the same item, and nothing advises skipping
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the remote has gained the commit "docs: add p1-from-ana" adding the idea "p1-from-ana" to the registry
    And the clone has the commit "docs: add p1-from-bo" adding the idea "p1-from-bo" to the registry
    When itos runs "push"
    Then itos exits with code 1
    And its output says "tasks/work-items.yaml"
    And its output does not say "the same item"
    And its output does not say "rebase --skip"
    And its output says "rebase --continue"

  # git resolves HEAD before the pre-push hook runs, but itos push read HEAD
  # again after git push returned, so a commit made while the hook ran (its
  # unit tests take minutes) was named as pushed, and push waited on a CI run
  # that would never come (found by bug 20's agent, 2026-10-04,
  # p1-push-names-head-after-hook). itos push resolves the commit once,
  # before pushing, and pushes, names and waits on that one.
  @ID-PUSH-06 @bug-21
  Scenario: itos push names the commit git pushed, though a commit lands while its pre-push hook runs
    Given the pre-push hook is installed
    And hooks.pre_push's command commits "docs: late" touching "late.md" in the clone
    And the clone has the commit "docs: mine" touching "mine.md"
    When itos runs "push --no-wait"
    Then itos exits with code 0
    And the remote's branch has "docs: mine"
    And the remote's branch does not have "docs: late"
    And its output names the remote branch's head as the commit pushed

  # Bug 45 (the user's call, 2026-10-05). To keep from calling itself, itos
  # looked for the real git as the first git on the PATH that is not this
  # very binary. The git shim links git to the itos installed globally, and
  # the itos running is often another file: a repository's pinned release,
  # which the launcher runs from its cache, or a repository's own build. That
  # itos took the shim for the real git, so its own git push origin <branch>
  # went back into itos, which refused the arguments, and itos push failed
  # wherever both a pin and the shim were in use (found pushing this
  # repository, whose tools/bin/itos is not the shim's target). itos's own
  # git calls pass over any git that is an itos (a link to a file named itos,
  # or itos.exe), not only itself.
  @ID-PUSH-07 @bug-45
  Scenario: itos push reaches the real git when the git on the PATH is another itos's shim
    Given another itos is linked as git before the real git on the PATH
    And the clone has the commit "chore: tidy the readme" touching "README.md"
    When itos runs "push --no-wait"
    Then itos exits with code 0
    And the remote's branch ends with "chore: tidy the readme"

  # Slice 90 (decision 36's rule 31, found by slice 86): itos push exited
  # with git's own code when its fetch or its push failed, so a remote it
  # could not reach exited 128, none of itos's codes, and not 75. A remote
  # that cannot be reached (a connection refused or timed out, a host that
  # does not resolve) exits 75; a remote that is no repository, or that git
  # cannot find, exits 3; a push the remote refuses exits 1, as before. In
  # the v6.0.0 push with slices 86 to 89, since an exit code is the contract.
  @ID-PUSH-08 @slice-90
  Scenario: A push whose remote cannot be reached exits 75
    Given the clone's remote origin is "https://127.0.0.1:1/itos.git"
    And the clone has the commit "chore: tidy the readme" touching "README.md"
    When itos runs "push"
    Then itos exits with code 75

  @ID-PUSH-09 @slice-90
  Scenario: A push whose remote is no repository exits 3
    Given the clone's remote origin is a folder that is no repository
    And the clone has the commit "chore: tidy the readme" touching "README.md"
    When itos runs "push"
    Then itos exits with code 3

  # Slice 91 (q-2 and q-17): as itos commit's refusal, for the pre-push hook.
  @ID-PUSH-10 @slice-91
  Scenario: itos push refuses when its hooks are not declared in the clone's git config
    Given the clone's itos hooks are not installed
    And the clone has the commit "chore: tidy the readme" touching "README.md"
    When itos runs "push"
    Then itos exits with code 3
    And its output says "itos hook install"
    And the remote's branch does not have "chore: tidy the readme"

  # Slice 131 (p1-push-tag-and-ref, issue #37 and its comment of 2026-10-11;
  # the v8 plan). The guard, the shim and itos push refuse a ref push, so a
  # release tag or a branch for another OS's CI went out through a hard-coded
  # /usr/bin/git, past the shim's checks; and push.followtags decided
  # silently whether the next unrelated itos push published a tag.
  # itos push --tag <tag> (repeatable) pushes the named tags, and only them,
  # in one atomic push (git push --atomic): a release of several tags, some on
  # commits HEAD does not reach (itos-template's main/, stack/go/, go/cli/
  # tags), lands whole or not at all. A tag the remote already holds at
  # another commit is refused, never moved, and then no tag of the push
  # lands. itos push --ref <branch> pushes that local branch to the remote's
  # branch of the same name, never forced. A plain itos push pushes no tag,
  # whatever push.followtags says (--no-follow-tags), which is the breaking
  # part. The pre-push hook judges a pushed tag's commits as any ref's. The
  # shim mapping git push origin <tag> or -u origin <branch> onto these
  # (p3-shim-push-args, folded into the item) is not in this slice.
  @ID-PUSH-11 @slice-131
  Scenario: itos push --tag pushes the named tags in one push
    Given the clone has the annotated tag "v1.0.0" on its HEAD
    And the clone has the annotated tag "cli/v1.0.0" on its HEAD
    When itos runs "push --tag v1.0.0 --tag cli/v1.0.0"
    Then itos exits with code 0
    And the remote has the tag "v1.0.0"
    And the remote has the tag "cli/v1.0.0"

  @ID-PUSH-12 @slice-131
  Scenario: A plain itos push pushes no tag, whatever push.followTags says
    Given the clone's git config says "push.followTags" is "true"
    And the clone has the commit "docs: note the release" touching "README.md"
    And the clone has the annotated tag "v1.0.0" on its HEAD
    When itos runs "push"
    Then itos exits with code 0
    And the remote's branch has "docs: note the release"
    And the remote does not have the tag "v1.0.0"

  @ID-PUSH-13 @slice-131
  Scenario: A tag the remote holds at another commit is refused, and no tag of the push lands
    Given the remote's head is tagged "v1.0.0"
    And the clone has the commit "docs: note the release" touching "README.md"
    And the clone has the annotated tag "v1.0.0" on its HEAD
    And the clone has the annotated tag "v1.0.1" on its HEAD
    When itos runs "push --tag v1.0.0 --tag v1.0.1"
    Then itos exits with code 1
    And its output says "v1.0.0"
    And the remote does not have the tag "v1.0.1"

  @ID-PUSH-14 @slice-131
  Scenario: itos push --ref pushes another local branch to the remote's branch of the same name
    Given the clone has the branch "ci/windows" with the commit "docs: try windows" touching "README.md"
    When itos runs "push --ref ci/windows"
    Then itos exits with code 0
    And the remote has the branch "ci/windows" with "docs: try windows"

  # Bug 136 (was p1-push-no-upstream-worktree), met three times: Claude
  # Code's isolation worktrees put each agent on a new branch,
  # worktree-agent-<id>, with no upstream, and itos push pushed it to origin
  # and the branch's own name, a stray remote branch in a repository that
  # pushes only to main, so the agent's take never reached main; under
  # Claude Code's Bash sandbox .git/config cannot be written, so the
  # upstream cannot be set by hand either. Under trunk (push.mode trunk, no
  # push.mode, no config, or a config that cannot be read, as slice 174
  # counts it), a branch with no upstream (branch.<name>.remote or .merge
  # unset) goes to origin's default branch: the branch origin's HEAD names,
  # asked of the remote as slice 174 finds main (git ls-remote --symref
  # origin HEAD), after the no-remote check and before the fetch. It is
  # fetched and rebased onto as an upstream is, and every line naming the
  # upstream names it: Pushed <sha> to origin/main. One more line of the
  # push's report says that the branch has no upstream and that itos push
  # --ref <branch> pushes it to its own name; it goes where the push's
  # other report lines go (run.go): on stdout beside the Pushed line, on
  # stderr with a refusal (a rebase that stops), dropped by -q on a success,
  # and left out by --json, whose remote and branch are origin and the
  # default branch, no new key. itos push writes no upstream: each push
  # asks the remote again, so it works where git config cannot be written.
  # Unchanged: a branch with an upstream; push.mode pr, where an item's
  # branch goes to its own name whatever its upstream says (slice 174);
  # --tag and --ref, which ask for no default branch; and a remote with no
  # branch at all, whose first push makes the branch's own name, as the
  # corpus's no-upstream case does.
  #
  # The words the fix changes: the help's upstream rule (command.go, "else
  # origin and the branch's own name") becomes origin's default branch, the
  # empty remote's own name beside it; exit 3's list gains a remote whose
  # HEAD names no branch it has, and --json's outcomes no-default-branch,
  # unless slice 174 has added them; the corpus header's "origin and the
  # branch's own name when there is none" (tools/itos/conformance/push.yaml)
  # and its no-upstream case's name say the same; and doc.go's paragraph on
  # the fetch gains the ls-remote, its no-upstream refusal staying as it is.
  # Unless slice 174 has already changed them, push-mode.feature's too: its
  # header's promise that under trunk itos push is what it was "whatever the
  # branch", and @ID-PUSHMODE-15's title, comment and remote-branch line,
  # which say a branch named for an item with no upstream is pushed to its
  # own name as before; under trunk it now goes to the default branch.
  # Fails before the fix at the history step: today the commit goes to
  # origin/worktree-agent-1 and main lacks it.
  @ID-PUSH-16 @bug-136 @wip
  Scenario: A branch with no upstream is rebased onto the remote's default branch and pushed to it, making no branch of its name
    Given the remote has gained the commit "chore: tidy the docs" touching "docs.md"
    And the clone is on the new branch "worktree-agent-1"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push"
    Then itos exits with code 0
    And the remote's branch ends with "chore: tidy the docs" then "feat: add the widget", with no merge commit
    And the remote has no branch "worktree-agent-1"
    And its output names the remote branch's head as the commit pushed
    And its output says "itos push --ref worktree-agent-1"

  # The meeting itself: git worktree add -b with no start point makes the
  # branch at HEAD with no upstream, as Claude Code's isolation does. The
  # push leaves the branch without one: it never writes git config, which
  # the sandbox refuses. Fails before the fix at the main step.
  @ID-PUSH-17 @bug-136 @wip
  Scenario: A push from a linked worktree's new branch lands on the remote's default branch and records no upstream
    Given the clone has a linked worktree on the new branch "worktree-agent-1", where itos runs
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push --no-wait"
    Then itos exits with code 0
    And the remote's branch has "feat: add the widget"
    And the remote has no branch "worktree-agent-1"
    And the clone's branch "worktree-agent-1" has no upstream

  # --json is the contract (ADR-0035): branch is where the commit went, the
  # default branch, not the local branch's name. push.mode trunk written out
  # is the same as none. Fails before the fix at the branch step: today it
  # is "worktree-agent-1".
  @ID-PUSH-18 @bug-136 @wip
  Scenario: Under push.mode trunk, push --json from a branch with no upstream names the default branch it pushed to
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: trunk
      """
    And the clone is on the new branch "worktree-agent-1"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push --json"
    Then itos exits with code 0
    And its JSON's "outcome" is "pushed"
    And its JSON's "remote" is "origin"
    And its JSON's "branch" is "main"

  # The default branch is asked of the remote, never guessed as main. Fails
  # before the fix at the develop step: today develop does not get the
  # commit.
  @ID-PUSH-19 @bug-136 @wip
  Scenario: A branch with no upstream goes to the branch the remote's HEAD names, whatever its name
    Given the remote's HEAD names its new branch "develop", made at its main's head
    And the clone is on the new branch "worktree-agent-1"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push"
    Then itos exits with code 0
    And the remote has the branch "develop" with "feat: add the widget"
    And the remote's branch does not have "feat: add the widget"
    And the remote has no branch "worktree-agent-1"

  # A remote with branches whose HEAD names none of them gives no default
  # branch, and falling back to the branch's own name is the bug: refused
  # before anything is fetched or pushed, exit 3, as slice 174 refuses it
  # under pr. Fails before the fix at the exit code: today the branch is
  # pushed, exit 0.
  @ID-PUSH-20 @bug-136 @wip
  Scenario: A branch with no upstream and a remote whose HEAD names no branch it has is refused, exit 3
    Given the remote's HEAD names the branch "trunk", which it does not have
    And the clone is on the new branch "worktree-agent-1"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push --json"
    Then itos exits with code 3
    And its JSON's "outcome" is "no-default-branch"
    And the remote has no branch "worktree-agent-1"

  # What the bug left behind: a stray branch of the local branch's name on
  # the remote. With no upstream the push still goes to the default branch,
  # never to the stray one, so one stray push does not draw the next. Fails
  # before the fix at the first main step: today both commits go to the
  # stray branch.
  @ID-PUSH-21 @bug-136 @wip
  Scenario: A branch with no upstream goes to the default branch though the remote has a branch of its name
    Given the clone is on the new branch "worktree-agent-1"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "worktree-agent-1"
    And the clone has the commit "test: cover the widget" touching "widget-test.md"
    When itos runs "push"
    Then itos exits with code 0
    And the remote's branch has "feat: add the widget"
    And the remote's branch has "test: cover the widget"

  # The fix is for a branch with no upstream alone: one whose upstream is
  # its own name on the remote goes there, not to the default branch.
  # Passes today and must keep passing.
  @ID-PUSH-22 @bug-136 @guard @wip
  Scenario: A branch whose upstream is the remote's branch of its name is pushed there, not to the default branch
    Given the clone is on the new branch "topic"
    And git pushes HEAD to the remote's new branch "topic"
    And the clone's git config says "branch.topic.remote" is "origin"
    And the clone's git config says "branch.topic.merge" is "refs/heads/topic"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push"
    Then itos exits with code 0
    And the remote has the branch "topic" with "feat: add the widget"
    And the remote's branch does not have "feat: add the widget"

  # --ref is how a branch with no upstream is pushed to its own name under
  # trunk, the way the push's text names: it asks for no default branch.
  # Passes today and must keep passing.
  @ID-PUSH-23 @bug-136 @guard @wip
  Scenario: push --ref naming the current branch with no upstream pushes it to its own name
    Given the clone is on the new branch "worktree-agent-1"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push --ref worktree-agent-1"
    Then itos exits with code 0
    And the remote has the branch "worktree-agent-1" with "feat: add the widget"
    And the remote's branch does not have "feat: add the widget"
