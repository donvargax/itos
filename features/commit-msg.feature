@phase-1 @slice-2
Feature: The commit-msg hook
  itos hook commit-msg judges a commit before it is made: the paths its type
  may touch, the footers its type needs and the IDs they name, and the header,
  through the header lint the config delegates to. Its exit code is the hook's,
  so a commit it rejects is not made, and its output says why.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a change to "README.md" is staged

  @ID-CMSG-01
  Scenario: A header without a type is rejected, naming the rule it breaks
    Given the header lint is commitlint's conventional config
    When the commit-msg hook checks the message "update things"
    Then itos exits with code 1
    And its output names the rule "type-empty"

  @ID-CMSG-02
  Scenario: A sound message with the footer its type needs is accepted
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 0

  @ID-CMSG-03
  Scenario: A footer naming a task the ledger does not have is rejected
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-999
      """
    Then itos exits with code 1
    And its output says "unknown tasks: T-999"
    And its output names the rule "task-footer"

  @ID-CMSG-04
  Scenario: A commit whose type may not touch a staged path is rejected
    Given a change to "tools/build.sh" is staged
    When the commit-msg hook checks the message "docs: describe the build"
    Then itos exits with code 1
    And its output says "docs commits may not touch tools/build.sh"

  # The hook's help called the header lint's delegate "commitlint here", this
  # repository's until it switched to the built-in lint (T-063). The help is
  # the binary's, read in every project, and which lint a project runs is its
  # config's to say.
  @ID-CMSG-05 @bug-3
  Scenario: The commit-msg hook's help names no project's header lint
    When itos prints the help of "hook commit-msg"
    Then itos exits with code 0
    And its output says "commits.header_lint"
    And its output does not say "commitlint"

  # Bug 30 (found by the code review of 07b0e6d; the user's calls,
  # 2026-10-05). itos read a commit's type as the first word of its header,
  # and the header lint, as commitlint does, ignores the headers git writes
  # itself: Revert "…", Merge …, fixup!, squash!, amend!, Reapply. Such a
  # header gave a type outside commits.types, so no path scope, no required
  # footer and no moves rule applied, in the hook and in verify, and verify's
  # --no-merges left a merge's own changes unjudged: any change could land
  # with no footer. Now Revert "…" (and Reapply "…") is judged as revert;
  # fixup!, squash! and amend! as the type of the header they name, after the
  # last such prefix; and a merge commit by its own changes, the ones no
  # parent brings: a merge with none passes whatever its header, as GitHub's
  # "Merge pull request" commits do, while one with changes of its own is
  # judged by its first line's type, and refused when that has none. Any
  # other header with no type in commits.types stays refused by the lint.
  @ID-CMSG-06 @bug-30
  Scenario: A Revert "…" header is judged as a revert, which needs its footer
    When the commit-msg hook checks the message "Revert \"chore: tidy the readme\""
    Then itos exits with code 1
    And its output says "Task"

  @ID-CMSG-07 @bug-30
  Scenario: A fixup! header is judged by the type of the header it names
    Given the config's chore commits may never touch "src/**" except "src/themes/**"
    And a change to "src/app.js" is staged
    When the commit-msg hook checks the message:
      """
      fixup! chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 1
    And its output says "chore commits may not touch src/app.js"

  @ID-CMSG-08 @bug-30
  Scenario: verify judges a merge commit's own changes by its first line's type
    Given a merge commit "Merge branch 'topic'" on top of it, with a change of its own to "src/app.js"
    When itos verifies every commit up to HEAD
    Then itos exits with code 1
    And its output says "Merge branch 'topic'"

  @ID-CMSG-09 @bug-30
  Scenario: verify passes a merge commit with no changes of its own, whatever its header
    Given a merge commit "Merge branch 'topic'" on top of it, with no change of its own
    When itos verifies every commit up to HEAD
    Then itos exits with code 0

  # Bug 158 (seen on slice-209, 2026-10-31). Bug 30 read a merge's own
  # changes as the paths of its dense combined diff (git diff-tree --cc),
  # which keeps a hunk whose lines in the merge are no single parent's. When
  # the two sides change neighbouring lines of one file (a line apart, inside
  # the diff's three lines of context), git's clean merge holds both changes
  # in one such hunk: the merge has a path of its own, and its untyped
  # "Merge …" first line is refused under merge-type. itos push under
  # push.pr.update merge writes that line (push-merge.feature), and the only
  # way out was a rewrite, which Claude Code refuses agents.
  #
  # The rule (the coordinator's, settled): a merge's own changes are the
  # paths where its tree differs from the tree git's automatic merge of its
  # two parents writes, git merge-tree --write-tree <first parent> <second
  # parent> (which exits 1 on a conflict and still writes the tree, conflict
  # markers in it), the two trees compared as diff-tree compares them, with
  # --no-renames. A merge whose tree is git's own has none, whatever --cc
  # shows. A merge that resolved a conflict, with new lines or by taking one
  # side's, or that added anything git's merge did not write, has some, and is
  # judged as its first line's type, refused under merge-type when that has
  # none. One reading, the VCS's OwnPaths (and StagedOwnPaths through it, for
  # the merge the commit-msg hook judges), is what verify, the pre-push hook,
  # the commit-msg hook, the path rules and the moves rule judge a merge by;
  # internal/git/merge.go's and doc.go's account of --cc and --remerge-diff,
  # the VCS port's, verify's help and push-mode-ready-merge.feature's
  # description say so. A merge of more than two parents is read by --cc as
  # today: merge-tree merges two, and git's octopus merge stops at any
  # conflict. So is a merge of two parents with no merge base (git merge-base
  # exits 1), which merge-tree refuses (exit 128): one made with
  # --allow-unrelated-histories, or one read in a shallow clone whose base
  # was cut off, as a consumer's CI with a limited fetch depth reads it.
  #
  # What git's merge did not write is the merge's own whatever made it, so
  # the rule reverses bug 30's reason for refusing --remerge-diff, and these
  # all have changes of their own: a merge made with -s ours (it owns every
  # path its second parent changed; internal/git/merge_test.go's -s ours case
  # flips), one made with -X ours, -X theirs or another strategy option, and
  # one that relies on merge.renormalize, merge.renames or a custom
  # .gitattributes merge driver set in one clone, where the hook and CI may
  # then disagree.
  #
  # The steps. Each "a merge commit <message> on top of it, whose two sides
  # change …" is built as "a merge commit … with no change of its own" is
  # (git's plumbing in a scratch index, what the scenario staged left staged
  # and out of every commit): on HEAD, the commit "chore: write the notes"
  # adding <path> with the lines "one" to "five", one a line; on that, a main
  # line and a topic line of one chore commit each changing <path>; and the
  # merge of the two with the message, the main line its first parent, HEAD's
  # branch moved to it. Every commit but the merge has the footer Task: of the
  # ledger's first task.
  #   - "change neighbouring lines": the topic line makes "two" "two, the
  #     topic's" and the main line "four" "four, main's". "with no change of
  #     its own" makes the merge's tree git's merge of the two, both lines
  #     changed; "with a line of its own added to it", that tree with "six,
  #     the merge's" added at the file's end.
  #   - "change the same line": the topic line makes "two" "two, the topic's"
  #     and the main line "two, main's". "the conflict resolved by a line
  #     neither side has" leaves "two, both's" there; "the conflict resolved
  #     by taking the topic side's line" leaves the file as the topic line has
  #     it.
  # "a merge of the branch <branch> is in progress, whose two sides change
  # neighbouring lines of <path>, git's merge staged" first unstages what the
  # scenario staged, its change left in the work tree (git merges only onto
  # an index that matches HEAD); makes the same three commits, the base and
  # the main line's on HEAD's branch and the topic line's on the new branch
  # <branch>; then runs git merge --no-ff --no-commit <branch> on HEAD's
  # branch: MERGE_HEAD names <branch>'s head and the index holds git's merge.
  # "a merge commit <message> on top of it, of a root commit with no history
  # in common with it, with no change of its own" is built the same way, by
  # plumbing in a scratch index, so the Background's staged change to
  # README.md stays staged and out of every commit: a root commit "chore:
  # start the other line" adding "other.md" and a README.md of its own
  # ("# The other line"), with the footer Task: of the ledger's first task,
  # and the merge of HEAD and it with the message, HEAD its first parent,
  # its tree HEAD's (README.md as HEAD has it) with other.md added, HEAD's
  # branch moved to it. Both parents holding README.md is what makes the
  # path shared: a build that merges them on an empty base (merge-tree
  # --allow-unrelated-histories) gets an add/add conflict there and refuses
  # README.md as the merge's own, while the --cc fallback finds no hunk,
  # README.md being HEAD's.

  # Fails before the work at the exit code: --cc shows notes.md's one hunk,
  # holding both sides' lines, and the merge is refused under merge-type.
  @bug-158 @ID-CMSG-19
  Scenario: verify passes a clean merge whose two sides changed neighbouring lines of one file, whatever its header
    Given a merge commit "Merge branch 'topic'" on top of it, whose two sides change neighbouring lines of "notes.md", with no change of its own
    When itos verifies every commit up to HEAD
    Then itos exits with code 0

  # The merge being made, judged by the same reading. Fails before the work
  # at the exit code: the merge the hook commits from the index has --cc's
  # hunk in notes.md, merge-type.
  @bug-158 @ID-CMSG-20
  Scenario: The commit-msg hook accepts an untyped first line for a clean merge whose two sides changed neighbouring lines of one file
    Given a merge of the branch "topic" is in progress, whose two sides change neighbouring lines of "notes.md", git's merge staged
    When the commit-msg hook checks the message "Merge branch 'topic'"
    Then itos exits with code 0

  # A guard: --cc shows the line no parent has, and git's merge wrote
  # conflict markers there, so it passes before the work and must after it.
  @bug-158 @guard @ID-CMSG-21
  Scenario: verify refuses an untyped merge that resolved a conflict with a line of its own
    Given a merge commit "Merge branch 'topic'" on top of it, whose two sides change the same line of "notes.md", the conflict resolved by a line neither side has
    When itos verifies every commit up to HEAD
    Then itos exits with code 1
    And its output says "needs one of the commit types in its first line"
    And its output says "Merge branch 'topic'"

  # A conflict resolved by taking one side's line is resolved by hand all the
  # same: git's merge wrote conflict markers there, not that line. Bug 30
  # passed it, since every line of it is the topic's and --cc shows no hunk;
  # no scenario promised that. Fails before the work at the exit code, 0.
  @bug-158 @ID-CMSG-22
  Scenario: verify refuses an untyped merge that resolved a conflict by taking one side's line
    Given a merge commit "Merge branch 'topic'" on top of it, whose two sides change the same line of "notes.md", the conflict resolved by taking the topic side's line
    When itos verifies every commit up to HEAD
    Then itos exits with code 1
    And its output says "needs one of the commit types in its first line"
    And its output says "notes.md"

  # An evil merge: git's clean merge, then a line no side wrote, in the file
  # both sides changed. A guard: --cc shows the hunk, so it passes before the
  # work, and must after it, the tree being other than git's merge.
  @bug-158 @guard @ID-CMSG-23
  Scenario: verify refuses an untyped clean merge that added a line of its own to a file both sides changed
    Given a merge commit "Merge branch 'topic'" on top of it, whose two sides change neighbouring lines of "notes.md", with a line of its own added to it
    When itos verifies every commit up to HEAD
    Then itos exits with code 1
    And its output says "needs one of the commit types in its first line"
    And its output says "notes.md"

  # A change staged beside git's merge is the merge's own, and notes.md,
  # which git merged, is not. Fails before the work at the last step: today
  # the refusal names notes.md, whose hunk --cc shows, beside src/app.js.
  @bug-158 @ID-CMSG-24
  Scenario: The commit-msg hook refuses an untyped merge with a change of its own staged, naming that path alone
    Given a merge of the branch "topic" is in progress, whose two sides change neighbouring lines of "notes.md", git's merge staged
    And a change to "src/app.js" is staged
    When the commit-msg hook checks the message "Merge branch 'topic'"
    Then itos exits with code 1
    And its output says "needs one of the commit types in its first line"
    And its output says "src/app.js"
    And its output does not say "notes.md"

  # A merge whose parents share no merge base, as --allow-unrelated-histories
  # makes one, keeps the --cc reading, which finds no hunk: merge-tree would
  # refuse it, exit 128. Both parents hold README.md, the merge keeping
  # HEAD's: a build that merges on an empty base instead (merge-tree
  # --allow-unrelated-histories) meets an add/add conflict there and refuses
  # README.md under merge-type, while the fallback passes it. A guard: it
  # passes before the work and must after it, where a build that asks
  # merge-tree of every two-parent merge fails, and so does the empty-base
  # one.
  @bug-158 @guard @ID-CMSG-25
  Scenario: verify passes an untyped merge of a root commit with no history in common with its first parent
    Given a merge commit "Merge branch 'other'" on top of it, of a root commit with no history in common with it, with no change of its own
    When itos verifies every commit up to HEAD
    Then itos exits with code 0

  # Bug 159 (Codex's review of bug 158's PR #244, 2026-10-31). Where the two
  # parents conflict, merge-tree exits 1 and still writes a tree, conflict
  # markers in it, and bug 158 took that tree as git's merge: a merge
  # committed with exactly that tree, the raw markers included, had no
  # changes of its own and passed verify and the hooks under any first line,
  # skipping the message, footer, scope and moves checks. Bug 30's --cc
  # reading caught the markers' hunk. A conflict whose tree holds no markers
  # at all and that a merge keeps as merge-tree left it passed the same way,
  # and --cc never caught it either, a gap bug 30 had too: the modified side
  # of a modify/delete conflict, or the first parent's side of a binary one.
  #
  # The rule (the coordinator's, settled): every path merge-tree reports as
  # conflicted is the merge's own change, whatever the merge's tree holds,
  # beside the paths where its tree differs from merge-tree's. merge-tree
  # lists them after the tree: git merge-tree --write-tree --name-only
  # --no-messages prints the tree, then each conflicted path once, a line
  # each (-z separates them by NUL, so a path git would quote is read as it
  # is named, bug 31's rule); without --name-only each comes once per stage,
  # with its mode and object, and the messages are prose, an "Auto-merging"
  # line for a path that merged clean. The rest of bug 158's rule stands: a
  # clean merge's own changes are its tree's difference from merge-tree's,
  # and a merge of more than two parents or of two with no merge base is read
  # by --cc. A conflict merge-tree lists no path for (a directory rename split,
  # git's documentation says, exits 1 with an empty list) is beyond this
  # rule: its own paths are its tree's difference alone.
  #
  # ID-CMSG-19 (verify) and ID-CMSG-20 (the hook) are the guards that a clean
  # merge still has none: a build reading the messages' "Auto-merging
  # notes.md", or counting every path either side changed, refuses them.
  #
  # The steps. "the conflict left as git merge-tree writes it, markers and
  # all" builds the merge as the other "the same line" merges are built, its
  # tree the one git merge-tree --write-tree <main line> <topic line> writes,
  # the two commits named by their full SHAs, the first parent first: the
  # markers are labelled by the names merge-tree is given, and itos gives it
  # the parents' full SHAs, so any other names make another tree. "a merge of
  # the branch <branch> is in progress, whose two sides change the same line
  # of <path>, git merge-tree's conflict tree staged" makes the commits of
  # "… change neighbouring lines …, git's merge staged" with the same line's
  # texts, runs the same git merge --no-ff --no-commit <branch>, which stops
  # at the conflict with MERGE_HEAD written, then stages the tree merge-tree
  # writes for the full SHAs of HEAD and MERGE_HEAD (git read-tree of it),
  # whose markers git merge's own, labelled HEAD and <branch>, are not. "a
  # merge commit <message> on top of it, whose main line deletes <path> and
  # whose topic line changes it, keeping the topic's change" builds, in the
  # scratch index, the commit "chore: write the notes" adding <path> with the
  # lines "one" to "five"; on it a main line's chore commit removing <path>
  # and a topic line's making "two" "two, the topic's", each with the footer
  # Task: of the ledger's first task; and the merge of the two, its tree the
  # main line's with the topic's <path>, which is merge-tree's tree for them.

  # Fails before the work at the exit code, 0: the merge's tree is
  # merge-tree's, so it has no path of its own.
  @bug-159 @ID-CMSG-26
  Scenario: verify refuses an untyped merge that committed merge-tree's conflict markers as they are
    Given a merge commit "Merge branch 'topic'" on top of it, whose two sides change the same line of "notes.md", the conflict left as git merge-tree writes it, markers and all
    When itos verifies every commit up to HEAD
    Then itos exits with code 1
    And its output says "needs one of the commit types in its first line"
    And its output says "notes.md"

  # The merge being made, by the same reading. Fails before the work at the
  # exit code, 0: the index written as a tree is merge-tree's.
  @bug-159 @ID-CMSG-27
  Scenario: The commit-msg hook refuses an untyped merge with merge-tree's conflict markers staged as they are
    Given a merge of the branch "topic" is in progress, whose two sides change the same line of "notes.md", git merge-tree's conflict tree staged
    When the commit-msg hook checks the message "Merge branch 'topic'"
    Then itos exits with code 1
    And its output says "needs one of the commit types in its first line"
    And its output says "notes.md"

  # A conflict with no markers: merge-tree keeps the modified side of a
  # modify/delete conflict in its tree, so a merge that keeps it too has no
  # path of its own today; a build that looks for markers in the merge's
  # tree still passes it, and so does one that falls back to --cc on a
  # conflict, whose dense combined diff shows no hunk for a side kept whole.
  # Fails before the work at the exit code, 0.
  @bug-159 @ID-CMSG-28
  Scenario: verify refuses an untyped merge that kept the changed side of a modify/delete conflict
    Given a merge commit "Merge branch 'topic'" on top of it, whose main line deletes "notes.md" and whose topic line changes it, keeping the topic's change
    When itos verifies every commit up to HEAD
    Then itos exits with code 1
    And its output says "needs one of the commit types in its first line"
    And its output says "notes.md"

  # Bug 31 (found by the code review of 07b0e6d, by two reviewers apart).
  # itos read git's lists of paths (diff --name-only, ls-files, ls-tree)
  # without -z or core.quotePath=false, and git C-quotes a path holding a
  # letter outside ASCII, a quote, a backslash or a control character
  # ("src/caf\303\251.js"). A never rule missed such a path and an only rule
  # refused it, and a feature file so named vanished from what the moves
  # rule and the footers read at the index and at a commit. Every list of
  # paths itos reads from git is read NUL-separated (or unquoted), so a path
  # is matched as it is named.
  @ID-CMSG-10 @bug-31
  Scenario: A never rule refuses a path whose name git would quote
    Given the config's chore commits may never touch "src/**" except "src/themes/**"
    And a change to "src/café.js" is staged
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 1
    And its output says "src/café.js"

  @ID-CMSG-11 @bug-31
  Scenario: An only rule lets through a path whose name git would quote
    Given a change to "docs/é.md" is staged
    When the commit-msg hook checks the message "docs: say it in French"
    Then itos exits with code 0

  # Bug 157 (Codex's security review of pull request 236, 2026-10-31). The
  # VCS adapter read git log's output as one text split on the byte 0x1e,
  # the record separator its format ends each commit with, and git commit
  # stores that byte in a message as it is given. A message (or an itos
  # note) holding it split its commit's record into pieces too short to
  # read, and the reader dropped them: Log left the commit out without a
  # word, and Show, through which verify reads each commit, said "<sha>
  # names no commit", exit 70, so verify could judge no range holding one.
  # Log's readers lost the commit: CI's plan, which reads the tasks and the
  # named tests a range's footers name (message.IDsIn), so the checks of a
  # task only such a commit names did not run; a release's notes (itos
  # commit footers); work show and work done (message.History); status, the
  # pull request trust and the check for commits a push drops.
  #
  # The rule: the adapter frames each commit's record so that no byte a
  # message or a note may hold ends it, and reads every field whole, the
  # part of the message after the byte included. Of the item's two ways
  # (frame safely, or refuse the record), safe framing is decided here: a
  # commit that can be read is judged as any other, never refused for the
  # byte, since a refused commit on main cannot be fixed forward, and
  # verify, re-checking every commit since the last green run, would stay
  # red until commits.since passed it. A record the adapter still cannot
  # read is an error naming the commit, never a commit passed over (Show's
  # today, as slice 207 made attempt submit's). A note is a blob and may
  # hold NUL, so a framing on NUL alone still needs that error, checked
  # against the commits git names; no scenario can make such a record, so
  # the adapter's unit tests pin it.
  #
  # "the commit <text> on top of it, its backslash escapes read as Go's" is
  # new: the commit "the commit <message> on top of it" makes, its message
  # the text read as the body of a Go string literal, as the footer of "the
  # commit … naming the task … with the footer <text>, its backslash escapes
  # read as Go's" is read. git commit -m takes the byte as an argument on
  # every OS: it is a control character, not NUL.

  # The footer after the byte is read: a reader that kept the record but
  # cut the message at the byte would find no Task: footer, and say "chore
  # commits need …" instead. Fails before the work at the exit code: verify
  # exits 70, "<sha> names no commit".
  @bug-157 @ID-CMSG-29
  Scenario: verify judges a commit whose message holds the byte 0x1e by the whole message
    Given the commit "chore: tidy the readme\n\nA line with \x1e in it.\n\nTask: T-999" on top of it, its backslash escapes read as Go's
    When itos verifies every commit up to HEAD
    Then itos exits with code 1
    And its output says "unknown tasks: T-999"
    And its output says "chore: tidy the readme"

  # A sound commit holding the byte passes: it is read, not refused. Fails
  # before the work at the exit code: verify exits 70, "<sha> names no
  # commit".
  @bug-157 @ID-CMSG-30
  Scenario: verify passes a sound commit whose message holds the byte 0x1e
    Given the commit "chore: tidy the readme\n\nA line with \x1e in it.\n\nTask: T-001" on top of it, its backslash escapes read as Go's
    When itos verifies every commit up to HEAD
    Then itos exits with code 0
    And its output says "2/2 commits pass the commit rules"

  # Through Log: CI's plan reads the range's Task: footers from it, so a
  # commit Log drops names no task, and its task's failing check never runs.
  # Task: sits after the byte, so a reader that cut the message there would
  # name no task either. Fails before the work at the exit code: ci run
  # plans no task check and exits 0.
  @bug-157 @ID-CMSG-31
  Scenario: ci run runs the check of a task that only a commit whose message holds the byte 0x1e names
    Given the task "T-001" has the check "exit 4"
    And the commit "chore: tidy the readme\n\nA line with \x1e in it.\n\nTask: T-001" on top of it, its backslash escapes read as Go's
    When itos runs CI over the commits after the first
    Then itos exits with code 1
    And its output says "CI failed at T-001 (Tidy)"

  # Bug 32 (found by the code review of 07b0e6d). The commit-msg hook read
  # the staged paths with git's rename detection on, so a rename listed only
  # its new path: git mv of code into docs/ passed a docs commit. Its
  # --diff-filter=ACMRD also left out T, a path whose type changed (a file
  # made a symlink), which verify, reading diff-tree, saw. Every list of
  # changed paths is read with --no-renames and every change type, so a
  # rename is its old path's deletion and its new path's addition. The
  # symlink is staged in the index alone (git update-index), so no
  # filesystem link is needed on Windows.
  @ID-CMSG-12 @bug-32
  Scenario: A docs commit that renames code into docs/ is refused, naming the old path
    Given "src/app.js" is committed
    And "src/app.js" is staged renamed to "docs/app.md"
    When the commit-msg hook checks the message "docs: move the app into the docs"
    Then itos exits with code 1
    And its output says "src/app.js"

  @ID-CMSG-13 @bug-32
  Scenario: A never rule refuses a path whose type changed
    Given the config's chore commits may never touch "src/**" except "src/themes/**"
    And "src/app.js" is committed
    And "src/app.js" is staged as a symlink
    When the commit-msg hook checks the message:
      """
      chore: link the app

      Task: T-001
      """
    Then itos exits with code 1
    And its output says "src/app.js"

  # Bug 36 (found by the code review of 07b0e6d). The commit-msg hook read
  # the commit's type from the message file's first line, while the header
  # lint read the header git will store, after its cleanup: a message whose
  # first line was blank or a comment (as an editor leaves it) passed the
  # lint and hid its type from the path rules and the required footers. The
  # hook reads the type from the same cleaned header the lint reads. A
  # docstring may not keep a leading blank line, hence the second scenario's
  # own step.
  @ID-CMSG-14 @bug-36
  Scenario: A message whose first line is a comment is held to its type's path rules
    Given the config's chore commits may never touch "src/**" except "src/themes/**"
    And a change to "src/app.js" is staged
    When the commit-msg hook checks the message:
      """
      # Please enter the commit message for your changes.
      chore: tidy the app

      Task: T-001
      """
    Then itos exits with code 1
    And its output says "src/app.js"

  @ID-CMSG-15 @bug-36
  Scenario: A message whose first line is blank is held to its type's footers
    When the commit-msg hook checks a message whose first line is blank, then "chore: tidy the readme"
    Then itos exits with code 1
    And its output says "Task"

  # Bug 139 (issue #133, met on slice-179's PR #130): an implementer typed
  # `itos commit --task none` out of habit, since Upgrading: takes none, and
  # the commit-msg hook said "unknown tasks: none", which reads as if a task
  # named none were missing from the ledger.
  #
  # Where it is said. One rule says it: the ledger footer's check in
  # internal/message (CheckFooter, run by FooterProblems), which every path
  # that judges a message's footers runs: this hook (LintFile), itos commit,
  # which makes no ID check of its own and is refused by the hook git runs
  # (commit-command.feature), itos commit check-message and itos verify (both
  # message.Check). So the fix is in that rule, and every one of the four
  # says the same words; a fix in itos commit's flags alone would leave a
  # typed "Task: none" in the old trap. CI's plan is not one of them: it
  # takes only the footer IDs the ledger's id pattern matches (IDsIn), so
  # none never reaches its unknown tasks.
  #
  # The rule. When an ID the ledger footer names is unknown to the ledger and
  # is the word none, in any case (none, None, NONE: the same habit), the
  # footer's problem is no longer "unknown tasks: none" but
  #   the Task: footer takes a ledger task's id, and has no none
  # with the footer's key as the config names it in place of Task. Where
  # another footer of IDs stands in for the ledger footer in a commit of this
  # type (its in_place_of names the ledger footer for the type, as the
  # Item: footer does for test, docs and chore in the config of the
  # scenarios), the sentence goes on
  #   ; a chore commit names its registry item in Item: instead, with itos commit --item <id>
  # (the type, the stand-in's key and its flag as the config gives them; a
  # stand-in no itos commit flag writes is named without the "with …" part),
  # since the text output prints the sentence and not the fix. A type no
  # stand-in serves gets the first sentence alone.
  #
  # What stays. The rule (task-footer) and the exit code (1, the hook's and
  # git's refusal; 1 for check-message and verify) are today's. A ledger
  # that really has a task whose id is none is not judged: only an unknown
  # ID is. Every other unknown ID is reported in today's words, "unknown
  # tasks: T-999", with no --item hint (ID-CMSG-03, ID-FOOT-01 and
  # ID-COMMITCMD-33 hold that). A list holding none beside other IDs names
  # none once in the sentence above and still checks the others: an unknown
  # one is a second problem of the same rule after it, "unknown tasks:
  # T-999", with its own fix; a known one adds nothing, and the message is
  # still refused for the none. The required footer check is unchanged:
  # "Task: none" gives an ID, so it is never "chore commits need a …".
  #
  # --json: the problem keeps its shape, {"rule":"task-footer","message",
  # "fix"} (with "level":"error" under the built-in header lint or a
  # delegate). Its message is the sentence above; its fix is "name the task
  # the commit belongs to by an id <ledger folder>/ has" (ID-COMMITCMD-34),
  # and, where a stand-in serves, goes on ", or the registry item in Item:
  # with itos commit --item <id>". The fix of an unknown ID that is not none
  # is today's.
  #
  # Before the fix the hook says "unknown tasks: none" under the rule
  # task-footer and exits 1, so this scenario fails at the step that its
  # output says "has no none".
  @ID-CMSG-16 @bug-139
  Scenario: A ledger footer naming none says the footer takes a task's id, not that a task is unknown
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: none
      """
    Then itos exits with code 1
    And its output says "the Task: footer takes a ledger task's id, and has no none"
    And its output names the rule "task-footer"
    And its output does not say "unknown tasks"
    And its output does not say "--item"

  # none is matched whatever its case. Before the fix the hook says "unknown
  # tasks: None": the scenario fails at the step that its output says "has
  # no none".
  @ID-CMSG-17 @bug-139
  Scenario: A ledger footer naming None in another case is told the same
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: None
      """
    Then itos exits with code 1
    And its output says "has no none"
    And its output does not say "unknown tasks: None"

  # none beside an unknown task: none is named, and T-999 is still checked
  # and reported in today's words. Before the fix the hook says "unknown
  # tasks: none, T-999": the scenario fails at the step that its output says
  # "has no none".
  @ID-CMSG-18 @bug-139
  Scenario: A ledger footer naming none beside an unknown task names none and still reports the task
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: none, T-999
      """
    Then itos exits with code 1
    And its output says "has no none"
    And its output says "unknown tasks: T-999"
    And its output does not say "unknown tasks: none"
