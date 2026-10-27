@phase-3
Feature: itos commit, a commit whose footers itos writes
  itos commit runs git commit with the arguments it does not read itself,
  and writes the footers from its own flags: --task <id> for the ledger
  footer (Task: by default, or what the config calls it) and --scenarios
  <ids> for the scenario footer, each as git's own --trailer, so they land
  in the message whether it comes from -m, -F or the editor. The hooks run
  as for any commit and judge the footers the same way, so a footer itos
  writes is held to the rules a typed one is; a refused commit leaves no
  commit. In stealth mode (stealth.feature) the same flags write a note
  instead, so one habit serves both modes.

  Background:
    Given a repository whose ledger has the task "T-001"
    And the commit-msg hook is installed
    And a change to "README.md" is staged

  @ID-COMMITCMD-01 @slice-31
  Scenario: --task writes the ledger footer into the commit's message
    When itos commits with the arguments "--task T-001 -m 'chore: tidy the readme'"
    Then itos exits with code 0
    And the message of HEAD has the footer "Task: T-001"

  @ID-COMMITCMD-02 @slice-31
  Scenario: A footer itos writes is judged by the hook like a typed one, and a refused commit leaves none
    When itos commits with the arguments "--task T-999 -m 'chore: tidy the readme'"
    Then itos exits with code 1
    And its output says "T-999"
    And no commit was made

  @ID-COMMITCMD-03 @slice-31
  Scenario: --scenarios writes the scenario footer
    Given a feature file "a.feature" with the live scenario "@ID-A-01"
    When itos commits with the arguments "--scenarios @ID-A-01 -m 'feat: greet'"
    Then itos exits with code 0
    And the message of HEAD has the footer "Scenarios: @ID-A-01"

  @ID-COMMITCMD-04 @slice-31
  Scenario: The arguments itos does not read are git commit's
    When itos commits with the arguments "--task T-001 -m 'chore: tidy the readme' -m 'Why it changed.'"
    Then itos exits with code 0
    And the message of HEAD says "Why it changed."

  # Slice 31 passed every argument that is not a subcommand to git commit, so
  # a mistyped subcommand became a pathspec: git's confusing error, or, where
  # the word names a file, a commit of that file. A bare first word is
  # itos's: the subcommand, or the usage error it was before slice 31. Paths
  # for git still go after a flag or --.
  @ID-COMMITCMD-05 @bug-5
  Scenario: A first word that is not a subcommand is a usage error, and commits nothing
    Given a change to "check-mesage" is staged
    When itos commits with the arguments "check-mesage -m 'chore: tidy the readme'"
    Then itos exits with code 2
    And its output says "unknown command: commit check-mesage"
    And no commit was made

  @ID-COMMITCMD-06 @bug-5
  Scenario: Paths after -- are git commit's
    When itos commits with the arguments "--task T-001 -m 'chore: tidy the readme' -- README.md"
    Then itos exits with code 0
    And the message of HEAD has the footer "Task: T-001"

  # Slice 36: every footer a commit needs (p3-commit-content-flags). Links
  # (the ledger and scenario footers) and content (a footer of free text,
  # BREAKING-CHANGE) share git's trailer block, and the config's source tells
  # them apart: each footer of free text the config declares gets a flag of
  # its name in lower case, --breaking writes BREAKING-CHANGE:, the form git
  # reads as a trailer ("BREAKING CHANGE:", with its space, makes git see no
  # trailers in the block at all), and a commit missing a footer its type
  # requires is refused before git runs, naming the flag. The hook still
  # judges commits made any other way.
  @ID-COMMITCMD-07 @slice-36
  Scenario: A footer of free text the config declares has a flag of its name
    Given the config requires an "Upgrading" footer of free text for "chore"
    When itos commits with the arguments "--task T-001 --upgrading none -m 'chore: tidy the readme'"
    Then itos exits with code 0
    And the message of HEAD has the footer "Upgrading: none"

  @ID-COMMITCMD-08 @slice-36
  Scenario: A commit missing a footer its type requires is refused before git runs, naming the flag
    Given the config requires an "Upgrading" footer of free text for "chore"
    When itos commits with the arguments "--task T-001 -m 'chore: tidy the readme'"
    Then itos exits with code 1
    And its output says "--upgrading"
    And no commit was made

  @ID-COMMITCMD-09 @slice-36
  Scenario: A commit missing its task is refused before git runs, naming --task
    When itos commits with the arguments "-m 'chore: tidy the readme'"
    Then itos exits with code 1
    And its output says "--task"
    And no commit was made

  @ID-COMMITCMD-10 @slice-36
  Scenario: --breaking writes a BREAKING-CHANGE footer git reads as a trailer
    When itos commits with the arguments "--task T-001 --breaking 'the readme moved' -m 'chore!: move the readme'"
    Then itos exits with code 0
    And the message of HEAD has the footer "BREAKING-CHANGE: the readme moved"

  # Another trailer follows the task line: git already skips a trailer that
  # would land beside an identical one, so only this shape can double it.
  @ID-COMMITCMD-11 @slice-36
  Scenario: An amend through itos commit keeps one footer, not two
    Given itos has committed with the arguments "--task T-001 --trailer 'Reviewed-by: someone' -m 'chore: tidy the readme'"
    When itos commits with the arguments "--amend --no-edit --task T-001"
    Then itos exits with code 0
    And the message of HEAD has the footer "Task: T-001" once

  # Bug 6: the hook judged an amend's staged paths against HEAD, the commit
  # being replaced, rather than against its parent, so an amend changing only
  # the message of a feat was refused for touching nothing (slice 32 and 34
  # met it). An amend is judged as the commit it makes: its parent's tree
  # against the new one. The scratch config gives feat no path rule of its
  # own, so the scenario sets one, or the amend would pass either way.
  @ID-COMMITCMD-12 @bug-6
  Scenario: An amend that changes only the message of a feat is judged by the paths of the commit it makes
    Given a feature file "a.feature" with the live scenario "@ID-A-01"
    And the config's feat commits must touch "features/**"
    And itos has committed with the arguments "--scenarios @ID-A-01 -m 'feat: greet'"
    When itos commits with the arguments "--amend --scenarios @ID-A-01 -m 'feat: greet everyone'"
    Then itos exits with code 0

  # Slice 58 (p1-commit-wrap): a body line over the header lint's
  # body-max-line-length (100 under config-conventional) refused the commit,
  # and every message after one such refusal was checked by hand. itos
  # commit now breaks each body line longer than the limit at word
  # boundaries before git sees the message, from -m and -F alike (the editor's
  # message is left to the hook). The limit is the built-in lint's; a config
  # with no header lint, or one that delegates to a command whose limit itos
  # cannot read, wraps nothing (the coordinator's call, 2026-10-04). A list
  # item's continuation lines are indented to its text, and a line within the
  # limit, an indented line, the header and the footers are left as written.
  # It never joins lines, so a message already wrapped is unchanged. A word
  # longer than the limit stays whole, for the lint to judge.
  @ID-COMMITCMD-13 @slice-58
  Scenario: A body line longer than the lint's limit is wrapped at word boundaries, and the commit is accepted
    Given the message file "msg.txt" with the header "chore: tidy the readme" and a body line of 150 characters
    And the header lint is itos's built-in one
    When itos commits with the arguments "--task T-001 -F msg.txt"
    Then itos exits with code 0
    And no line of HEAD's message is longer than 100 characters
    And HEAD's message body has the same words as the file's, in order

  @ID-COMMITCMD-14 @slice-58
  Scenario: A list item longer than the limit wraps under its own text
    Given the message file "msg.txt" with the header "chore: tidy the readme" and a body list item of 150 characters
    And the header lint is itos's built-in one
    When itos commits with the arguments "--task T-001 -F msg.txt"
    Then itos exits with code 0
    And no line of HEAD's message is longer than 100 characters
    And every line of HEAD's message body after the item's first starts with two spaces

  # Passes before the slice; it guards the wrapping against joining lines.
  @ID-COMMITCMD-15 @slice-58
  Scenario: Body lines within the limit are left as written, never joined
    Given the message file "msg.txt" with the header "chore: tidy the readme" and the body lines "Line one." and "Line two."
    And the header lint is itos's built-in one
    When itos commits with the arguments "--task T-001 -F msg.txt"
    Then itos exits with code 0
    And the message of HEAD has the line "Line one."
    And the message of HEAD has the line "Line two."

  # Bug 15, found by slice 58's post-landing review: the wrap broke a body
  # line at any space, so a continuation line could begin with a footer
  # token or a breaking-change note (the lint, the footer rules and a
  # release then read the rest of the body as footers, and an ID footer's
  # words as IDs), or with git's comment char (git's strip cleanup then
  # drops the line). A break that would start a line so moves back a word,
  # or the line runs over the limit when no earlier break is left.
  @ID-COMMITCMD-16 @bug-15
  Scenario: A wrap never starts a line with a breaking-change note
    Given the message file "msg.txt" with the header "chore: tidy the readme" and a body line whose wrap would start a line with "BREAKING CHANGE: the keys stay."
    And the header lint is itos's built-in one
    When itos commits with the arguments "--task T-001 -F msg.txt"
    Then itos exits with code 0
    And no line of HEAD's message starts with "BREAKING CHANGE:"

  @ID-COMMITCMD-17 @bug-15
  Scenario: A wrap never starts a line with git's comment char, so strip cleanup keeps every word
    Given the message file "msg.txt" with the header "chore: tidy the readme" and a body line whose wrap would start a line with "#123 for the rest."
    And the header lint is itos's built-in one
    When itos commits with the arguments "--task T-001 --cleanup=strip -F msg.txt"
    Then itos exits with code 0
    And the message of HEAD says "#123 for the rest."

  # Slice 63: --item writes the Item footer, as --task writes the Task one.
  @ID-COMMITCMD-18 @slice-63
  Scenario: --item writes the Item footer into the commit's message
    Given the config has an Item footer of registry items, taken in place of Task for test, docs and chore
    And the work registry has the item "slice-9" owned by nobody with the status "doing"
    When itos commits with the arguments "--item slice-9 -m 'test: add slice 9 steps'"
    Then itos exits with code 0
    And the message of HEAD has the footer "Item: slice-9"

  # Slice 81 (issue #10; the user's call, 2026-10-05). A scope could say
  # only, never and must_touch, so "nothing under src/ but a game's themes"
  # could not be written, and a consumer enforced it with a range check of
  # its own, which itos commit check-paths does not run: planning a split
  # with it approved paths the hook then refused. A scope's except list,
  # beside never, takes paths back out of it: a path matching never is
  # refused unless it matches except. except applies to never alone, and
  # config check refuses an except in a scope that has no never. The hook,
  # verify and check-paths read the same scopes.
  @ID-COMMITCMD-19 @slice-81
  Scenario: check-paths lets a path through that a scope's except takes out of its never
    Given the config's chore commits may never touch "src/**" except "src/games/*/themes/**"
    When itos runs "commit check-paths --type chore src/games/cielo/themes/colours.ts"
    Then itos exits with code 0

  @ID-COMMITCMD-20 @slice-81
  Scenario: check-paths refuses a path under never that except does not name
    Given the config's chore commits may never touch "src/**" except "src/games/*/themes/**"
    When itos runs "commit check-paths --type chore src/games/cielo/sim/nudo.ts"
    Then itos exits with code 1
    And its output says "src/games/cielo/sim/nudo.ts"

  @ID-COMMITCMD-21 @slice-81
  Scenario: The commit-msg hook takes a scope's except as check-paths does
    Given the config's chore commits may never touch "src/**" except "src/games/*/themes/**"
    And a change to "src/games/cielo/themes/colours.ts" is staged
    When the commit-msg hook checks the message:
      """
      chore: tune the colours

      Task: T-001
      """
    Then itos exits with code 0

  # Slice 91 (q-2 and q-17, the user's calls, 2026-10-06): itos commit and
  # itos push ran git with whatever hooks git would run, so where none ran
  # (issue #16: a fresh worktree of a project whose hooks folder is
  # generated and was missing) every check was skipped in silence. itos now
  # installs its hooks only in the git config (hook.<name>.event and
  # hook.<name>.command, in .git/config, shared by every worktree of the
  # clone), which git runs beside any hooks folder, whatever core.hooksPath
  # says; it knows no hook manager. Every itos command that commits or
  # pushes refuses first, exit 3, when its hooks are not declared there or
  # the git cannot run hooks its config declares, naming itos hook install
  # or the git to upgrade. Plain git stays the deliberate way around it, and
  # CI's verify the backstop. No config key turns it off (decision 36). In
  # the v6.0.0 push. The scenarios' repositories install the hooks, so the
  # existing commit and push scenarios hold.
  @ID-COMMITCMD-22 @slice-91
  Scenario: itos's commit-msg hook runs where core.hooksPath names a folder that does not exist
    Given itos has installed the hooks
    And core.hooksPath is "missing/hooks"
    And a change to "README.md" is staged
    When git commits with the message "chore: tidy the readme"
    Then itos exits with code 1
    And its output says "Task"
    And nothing was committed

  @ID-COMMITCMD-23 @slice-91
  Scenario: itos commit refuses when its hooks are not declared in the git config
    Given itos's hooks are not installed
    And a change to "README.md" is staged
    When itos runs the command line "commit --task T-001 -m 'chore: tidy the readme'"
    Then itos exits with code 3
    And its output says "itos hook install"
    And nothing was committed

  @ID-COMMITCMD-24 @slice-91
  Scenario: itos commit refuses with a git that cannot run the hooks its config declares
    Given itos has installed the hooks
    And the git itos runs cannot run the hooks its config declares
    And a change to "README.md" is staged
    When itos runs the command line "commit --task T-001 -m 'chore: tidy the readme'"
    Then itos exits with code 3
    And its output says "git"
    And nothing was committed

  # itos commit runs without an itos config: it hands the command to git. In a
  # folder with no config it panicked on the nil config (gitCommit read its
  # comment character after checking the config was there), inside a
  # repository or not. The Background writes itos.yaml, so each scenario
  # removes it first. Outside a repository git fails before reading its
  # options, exit 128; in a repository it refuses the unknown option, 129.
  @ID-COMMITCMD-25 @bug-120
  Scenario: itos commit outside a repository with no itos config hands the command to git and never panics
    Given a folder that is not a git repository
    And the folder has no itos config
    When itos runs "commit --zz-bogus"
    Then itos exits with code 128
    And its output says "not a git repository"
    And its output does not say "panic"

  @ID-COMMITCMD-26 @bug-120
  Scenario: itos commit in a repository with no itos config hands the command to git and never panics
    Given the folder has no itos config
    When itos runs "commit --zz-bogus"
    Then itos exits with code 129
    And its output says "unknown option"
    And its output does not say "panic"

  # Bug 138 (issue #142, met on slice-181's PR #137): itos commit wrote its
  # footers as git's --trailer under trailer.ifExists=addIfDifferent, which
  # compares whole lines, so a message that already gave Scenarios: with the
  # same IDs laid out on other lines got a second Scenarios: footer, and the
  # duplicate had to be amended out.
  #
  # The rule. Before git runs, each footer a flag would write is compared
  # with the same footer in the message, both read by the reader the
  # commit-msg hook reads footers with (internal/message: the trailer block,
  # IDs and Texts), never by a second parser:
  # - a footer of IDs (--task's ledger footer, --item's registry footer,
  #   --scenarios' named tests' footer, so Task: and Item: are the same case
  #   as Scenarios:): the set of IDs every line of its key in the trailer
  #   block gives, continuation lines included, commas or whitespace between
  #   them, the footer's strip_prefix taken off, against the set of the
  #   flag's IDs read the same way. Order, repeats and how the lines are
  #   wrapped do not count; case does;
  # - a footer of free text (its flag --<key in lower case>): the set of the
  #   texts every line of its key gives, each unfolded onto one line and
  #   trimmed, against the flag's texts read the same way;
  # - --breaking: a footer of free text whose key is BREAKING-CHANGE. The
  #   message's BREAKING-CHANGE and BREAKING CHANGE lines are read together
  #   for the refusal alone: when their texts are not the flag's, it is
  #   refused. It is the same, and nothing is written, only when the message
  #   gives BREAKING-CHANGE itself with the flag's texts; texts given only
  #   under BREAKING CHANGE still get the flag's BREAKING-CHANGE line, as the
  #   hook reads that key alone.
  # Equal sets: nothing of that footer is written; the other flags' footers
  # still are. Sets that differ, one holding more or fewer included, since
  # itos cannot tell which the person meant: refused before git runs, exit
  # 1, as a commit missing a required footer is, in the same rejection, the
  # differing footers listed first: the config's reject message, then a line
  # per footer, "<Key>: the message gives <value>, <flag> gives <value>",
  # IDs as written, each once in the order given, joined by ", "; texts in
  # double quotes, joined by ", "; "nothing" for a key with an empty value.
  # Under --json each is a problem whose rule is the key in lower case and
  # "-footer-differs" (scenarios-footer-differs, and for --breaking
  # breaking-change-footer-differs), whose message is that line and whose
  # fix is "give <Key>: once, in the message or with <flag>".
  # git does not run, so nothing is committed; the hooks check (exit 3)
  # comes after it, as it does after the required-footer refusal.
  #
  # Which message: the one the required-footer check already reads before
  # git runs, -m (the -m values joined as git joins them), -F <file>, or
  # HEAD's for --amend --no-edit. An amend through --no-edit keeps one
  # footer when the value is the same (ID-COMMITCMD-11 holds), and is now
  # refused when it is another: adding a second item or more scenarios to
  # HEAD is an amend with -m or -F giving the whole footer, or with the
  # editor. The editor's message (an amend's without --no-edit too) and -F -
  # are not read, as the required-footer check does not read them: each line
  # is still added unless the message has that very line, as before. The
  # person sees and edits the editor's message after git writes the
  # trailers, and refusing there would block an amend meant to replace a
  # footer in the editor; -F - is read only by the wrap, later.
  #
  # Only a footer counts: a line that starts with "Scenarios:" in the body
  # (in a paragraph with prose, or not the message's last) is no trailer to
  # git or to the hook, so it is neither compared nor refused, and the
  # flag's footer is written as before. Under a stealth config the links
  # (Task, Item, Scenarios) never go into the message, so only the content
  # (free text, BREAKING-CHANGE) is compared. trailer.ifExists stays
  # addIfDifferent; a --trailer the person hands git among its own
  # arguments is git's and is not compared.
  #
  # No case of v8.19.0's scenarios or corpus changes, so no Changes: footer.
  # itos commit --help's "a footer the message already has is not written
  # again (trailer.ifExists addIfDifferent), so an amend keeps one" becomes:
  # where itos reads the message (-m, -F <file>, HEAD's for --amend
  # --no-edit), a footer it already gives with the same value, read as the
  # hook reads it, is not written again, so an amend keeps one, and one it
  # gives with another value is refused, naming the footer and both values,
  # nothing committed; a message it does not read (the editor's, -F -) gets
  # each line unless it has that very line (trailer.ifExists
  # addIfDifferent). Its Exit line's 1 adds "or a footer the message gives
  # differs from its flag's"; the package's doc.go says the same.
  #
  # Steps: "HEAD's <key> footers name exactly <ids>, each once" checks that
  # a new HEAD was made and reads git log -1
  # --format=%(trailers:key=<key>,valueonly,unfold), git's own reading of
  # the trailers, splits each value on commas and whitespace, and requires
  # the IDs found to be exactly those listed, each once, in any order. The
  # message files are written by the existing doc-string step "the scratch
  # file <path> contains exactly:", in the repository's root, where itos
  # runs.
  #
  # Fails before the fix: git adds "Scenarios: @ID-A-01 @ID-A-02" beside the
  # message's two lines, so each ID is named twice.
  @ID-COMMITCMD-27 @bug-138
  Scenario: A footer the message already gives with the same IDs on other lines is not written again
    Given a feature file "a.feature" with the live scenario "@ID-A-01"
    And a feature file "b.feature" with the live scenario "@ID-A-02"
    And the scratch file "msg.txt" contains exactly:
      """
      feat: greet

      Scenarios: @ID-A-01
      Scenarios: @ID-A-02
      """
    When itos commits with the arguments "--scenarios @ID-A-01,@ID-A-02 -F msg.txt"
    Then itos exits with code 0
    And HEAD's "Scenarios" footers name exactly "@ID-A-01, @ID-A-02", each once

  # Both scenarios are live, so before the fix the hook takes the two
  # footers and the commit is made: the scenario fails at the exit code.
  @ID-COMMITCMD-28 @bug-138
  Scenario: A footer the message gives with other IDs than its flag's is refused before git runs, naming both
    Given a feature file "a.feature" with the live scenario "@ID-A-01"
    And a feature file "b.feature" with the live scenario "@ID-A-02"
    And the scratch file "msg.txt" contains exactly:
      """
      feat: greet

      Scenarios: @ID-A-01
      """
    When itos commits with the arguments "--scenarios @ID-A-02 -F msg.txt"
    Then itos exits with code 1
    And its output says "the message gives @ID-A-01"
    And its output says "--scenarios gives @ID-A-02"
    And no commit was made

  # Passes before the fix: it guards the case addIfDifferent already caught.
  @ID-COMMITCMD-29 @bug-138
  Scenario: A footer the message gives on the very line its flag would write stays one footer
    Given a feature file "a.feature" with the live scenario "@ID-A-01"
    And the scratch file "msg.txt" contains exactly:
      """
      feat: greet

      Scenarios: @ID-A-01
      """
    When itos commits with the arguments "--scenarios @ID-A-01 -F msg.txt"
    Then itos exits with code 0
    And the message of HEAD has the footer "Scenarios: @ID-A-01" once

  # Passes before the fix: it guards against reading every line that starts
  # with a footer's key, which would refuse this commit for a footer it has
  # not got.
  @ID-COMMITCMD-30 @bug-138
  Scenario: A body line that starts with a footer's key is no footer, so the flag's footer is written
    Given a feature file "a.feature" with the live scenario "@ID-A-01"
    And the scratch file "msg.txt" contains exactly:
      """
      feat: greet

      The old message carried its footer in the body,
      Scenarios: @ID-A-02
      where neither git nor the hook reads it as one.
      """
    When itos commits with the arguments "--scenarios @ID-A-01 -F msg.txt"
    Then itos exits with code 0
    And the message of HEAD has the footer "Scenarios: @ID-A-01"

  # Bug 139 (issue #133): `itos commit --task none` was told "unknown tasks:
  # none". The rule and its words are written above ID-CMSG-16 in
  # commit-msg.feature: itos commit makes no ID check of its own, so what it
  # prints is the hook's, run by git, and these scenarios hold the flag path
  # the bug was met on. The config's Item: footer stands in for Task: for
  # test, docs and chore.
  #
  # A chore, which Item: serves: the sentence names --item. Before the fix
  # the hook says "unknown tasks: none" and the commit is refused, exit 1, so
  # the scenario fails at the step that its output says "has no none".
  @ID-COMMITCMD-31 @bug-139
  Scenario: --task none on a type Item: stands in for says Task: has no none and names --item
    Given the config has an Item footer of registry items, taken in place of Task for test, docs and chore
    When itos commits with the arguments "--task none -m 'chore: tidy the readme'"
    Then itos exits with code 1
    And its output says "the Task: footer takes a ledger task's id, and has no none"
    And its output says "a chore commit names its registry item in Item: instead, with itos commit --item <id>"
    And its output does not say "unknown tasks"
    And no commit was made

  # A refactor, which Item: does not serve (ID-FOOT-09): the first sentence
  # alone, no --item. Before the fix it fails at the step that its output
  # says "has no none".
  @ID-COMMITCMD-32 @bug-139
  Scenario: --task none on a type no footer stands in for says Task: has no none without naming --item
    Given the config has an Item footer of registry items, taken in place of Task for test, docs and chore
    When itos commits with the arguments "--task none -m 'refactor: split the parser'"
    Then itos exits with code 1
    And its output says "has no none"
    And its output does not say "--item"
    And no commit was made

  # Passes before the fix: it guards that only none is told anew, and that
  # an unknown task on a type Item: serves keeps today's words, with no
  # --item hint.
  @ID-COMMITCMD-33 @bug-139
  Scenario: --task naming an unknown task on a type Item: stands in for is still told the task is unknown
    Given the config has an Item footer of registry items, taken in place of Task for test, docs and chore
    When itos commits with the arguments "--task T-999 -m 'chore: tidy the readme'"
    Then itos exits with code 1
    And its output says "unknown tasks: T-999"
    And its output does not say "has no none"
    And its output does not say "--item"
    And no commit was made

  # check-message, the path verify shares (message.Check), under --json: one
  # problem of the rule task-footer, its message the new sentence and its
  # fix the new one. Before the fix the message is "unknown tasks: none", so
  # the scenario fails at the step that the problem's message says "has no
  # none".
  @ID-COMMITCMD-34 @bug-139
  Scenario: commit check-message --json gives Task: none's problem its rule, sentence and fix
    Given the scratch file "msg.txt" contains exactly:
      """
      chore: tidy the readme

      Task: none
      """
    When itos runs "commit check-message msg.txt --json"
    Then itos exits with code 1
    And its JSON's "ok" is false
    And its JSON's problem "task-footer"'s "message" says "has no none"
    And its JSON's problem "task-footer"'s "fix" says "name the task the commit belongs to by an id"

  # git commit <path> commits from a temporary index, so a pre-commit hook
  # that rewrites the file and stages it again (a formatter) puts its version
  # in the commit and the working tree, while git keeps the real index as it
  # read it before the hook ran: git status says MM and the next commit may
  # sweep the stale copy in (found 2026-10-26, recording slice 187 and a
  # plan). Before the fix the scenario fails at the step that git reports no
  # change. The hook stands in for dprint, which the features cannot run.
  @ID-COMMITCMD-35 @bug-145
  Scenario: A commit by path leaves the index as the commit has the file when a pre-commit hook rewrites it
    Given a pre-commit hook that appends a line to each staged Markdown file and stages it again
    When itos commits with the arguments "--task T-001 -m 'chore: tidy the readme' -- README.md"
    Then itos exits with code 0
    And git reports no change to the working tree or the index
    And the file "README.md" has the same contents as its committed version
