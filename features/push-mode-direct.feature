@phase-1
Feature: Under push.mode pr, the commit types push.pr.direct names go from main straight to main with the registry's commits
  Under push.mode pr (slice 174, push-mode.feature) a push from main sends
  only the registry's commits to main (slice 56's rule, widened by slice-177
  to the ledger edits made beside the registry, push-mode-ledger.feature);
  any other commit is refused once rebased, exit 1, nothing pushed (outcome
  work-on-main). So a coordinator's docs commit on main (a note, a guide, a
  decision record) has to ride a branch and a pull request of its own and
  wait for its checks, though nothing there judges it more than main's run
  does after: on 2026-10-25 a decision record rode a placeholder branch and
  its own pull request, since itos push refused it on main and the guard
  hook refuses a raw git push (issue 131).

  The key. push.pr.direct is a new section of push.pr, read under push.mode
  pr alone as push.pr's other keys are (config.PushPR):

    push:
      mode: pr
      pr:
        direct:
          types: [docs]
          except: ['features/**']

  push.pr.direct.types is a list of commit types, empty by default;
  push.pr.direct.except a list of path globs, empty by default. Left out,
  or with types empty, nothing changes: a push from main sends the
  registry's commits alone, and the refusal keeps its words. config check
  accepts both, refusing a types or an except that is not a list of strings
  as it refuses any value of the wrong type (the schema; the implementing
  commit adds corpus cases for both to tools/itos/conformance/config.yaml,
  as slice 179 did for review_by), and an except naming a $name that
  commits.path_sets lacks (expandSets, rule config-path-set), since except
  reads $name as every other path list of the config does. A type that
  commits.types does not list is not refused: no commit the hooks pass has
  it, so it admits nothing and a push of such a commit is refused, naming
  push.pr.direct. config get push.pr.direct.types and
  push.pr.direct.except print [] when the config leaves them out. The
  schema (internal/config/schema.go) lists both, with their words: types
  "The commit types that a push from main, or itos push --ref main, sends
  straight to main beside the registry's commits under push.mode pr; empty,
  none." and except "Globs of paths that keep a commit of those types off
  main, so it goes through its item's pull request: a commit touching any
  is refused on main." push.mode's own words, "only the registry's commits
  go straight to main", gain "and those push.pr.direct names". Two optional
  keys with a default each are added keys, which tools/bin/schema-contract
  passes as compatible; no key changes.

  The rule. A commit is admitted when its type, read as every rule reads it
  (message.Type: the header's first word, a scope or a "!" not mattering,
  a revert's header revert), is one push.pr.direct.types lists, and no path
  it touches matches a glob of push.pr.direct.except, the globs matched as
  commits.scopes' are (internal/glob, commit check-paths' rules: * does not
  cross /, ** does). Its paths are judged as registryCommit judges them:
  the commit by itself against its first parent, renames off, so a feature
  file moved to docs/ touches both and is excepted. A glob internal/glob
  cannot read counts as matching: the commit is not admitted. So the
  admitted commits and the registry's are the two kinds a push from main
  may add; a commit that is both (a docs commit touching only the registry)
  is the registry's as before. This repository's specs are docs commits
  touching features/, so except: ['features/**'] keeps each spec on its
  item's pull request, where the reviewer reads it (ADR-0061).

  Where it applies: the judgement of a push to main alone, addsWork, which
  both a push from main and itos push --ref main use (judgeRefs). Every
  commit is judged by itself and all of them go to main, so their order
  does not matter: an admitted commit may sit below, above or between the
  registry's. A range that mixes admitted commits with any other commit is
  refused whole, as today, exit 1, outcome work-on-main, nothing pushed:
  main has no item branch to send the rest to, and splitting the range
  would mean reordering commits, a rewrite itos push never makes. The
  person carries the work with git switch -c <item>, as the refusal says.
  The other judgement, which commits above main an item's branch sends to
  main (registryPrefix, slice-177), does not change: on an item's branch
  every commit but the registry's is the item's work, which its pull
  request's checks and reviewers read, so a docs commit there rides the
  pull request even when push.pr.direct admits its type. Sending it ahead
  would land a docs item's whole branch on main with no pull request, and
  the issue's case is the coordinator's commit on main.

  What judges an admitted commit. itos push sends it as the routine sends
  any push to main (r.push): the same git push, so main's pre-push hook
  judges it (its commit rules, hooks.pre_push's commands) as it judges a
  trunk push, and a commit the hook refuses is refused, nothing pushed.
  The wait does not change either (slice 56, touchesOnlyRegistry): a push
  whose commits touch more than the registry waits, with ci.watch, for
  main's run, as a trunk push and a task's close (slice-177) do, since a
  docs commit changes what a run judges (the doc budget, the formatter);
  --no-wait returns at once. Main's run judges the commit after it lands.

  The refusal. With push.pr.direct.types empty its words are today's:
  "itos push: push.mode is pr, and this push adds commits to <remote>/<main>
  that touch more than <registry>; nothing was pushed." With types set it
  names the oldest commit the push adds that is neither the registry's nor
  admitted, by its seven-character SHA and its subject: "itos push:
  push.mode is pr, and this push adds <sha> <subject> to <remote>/<main>,
  which is neither the registry's nor a commit push.pr.direct sends
  straight to main; nothing was pushed." The hint is today's for both:
  "Carry the work to its item's branch with git switch -c <item>, then itos
  push again from there." The outcome stays work-on-main, exit 1. No live
  scenario or corpus case asserts the old words (ID-PUSHMODE-08,
  ID-PUSHMODE-17 and the ID-PUSHLEDGER guards assert "git switch -c";
  push.yaml's case the outcome), and they stay as they are.

  Unchanged: trunk, and with no push.mode, push as before; push.pr is not
  read there. A stealth config's push.pr.direct is read as a tracked one's:
  the rule reads the loaded config and the commits, nothing of where the
  config lives. itos push --help's paragraph on main, and the push slice's
  doc.go, say the new rule.

  Left to the neighbouring ideas: p1-pr-registry-to-main, registry commands
  run on an item's branch committing to main; p1-pr-take-after-spec, a take
  made above an item's spec commit reaching main before the merge (an
  item's branch that starts with its spec sends nothing to main, admitted
  types or not); p1-pr-conflict-path, an item's branch that conflicts with
  main. This repository turns push.pr.direct on in a build commit of its
  own once the feat lands, with the words in AGENTS.md's Finishing ("only
  registry and ledger bookkeeping goes straight to main") in the same push.

  The steps are push-mode.feature's and push.feature's, and the recording
  gh is push-mode.feature's; none is new. The Background installs the
  pre-push hook, as a clone set up by itos has it, since main's hook
  judging what goes to main is part of the rule (ID-PUSHDIRECT-05). The
  scratch config's commits.scopes gives docs only Markdown, docs/**,
  tasks/** and feature files, which the hook's commit rules judge.

  Why most scenarios fail before the work where they do: push.pr.direct is
  an unknown key until then, so the config cannot be read. itos push then
  pushes as trunk (prMode), and the pre-push hook refuses every push with
  "FAIL itos.yaml: unknown key push.pr.direct", exit 1, nothing pushed.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the pre-push hook is installed
    And the work registry has the item "slice-9" owned by nobody with the status "todo"

  # The slice's point: a coordinator's decision record, committed on main,
  # goes straight to main with no pull request. Fails before the work at
  # the exit code: the hook refuses the unreadable config, exit 1.
  @ID-PUSHDIRECT-01 @slice-186
  Scenario: Under pr with push.pr.direct naming docs, a docs commit pushed from main goes straight to main, opening no pull request
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          direct:
            types: [docs]
            except: ['features/**']
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "docs: record the widget's decision" touching "docs/decisions/0001-widget.md"
    When itos runs "push"
    Then itos exits with code 0
    And the remote's branch has "docs: record the widget's decision"
    And the recording gh was never called

  # Admitted commits go with the registry's, each judged by itself, so the
  # docs commit below the take is no reason to refuse it and both land in
  # one push, in their order. Fails before the work at the exit code: the
  # hook refuses the unreadable config, exit 1.
  @ID-PUSHDIRECT-02 @slice-186
  Scenario: Under pr with push.pr.direct naming docs, a docs commit below a take, pushed from main, goes to main with it
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          direct:
            types: [docs]
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "docs: record the widget's decision" touching "docs/decisions/0001-widget.md"
    And the clone has the commit "docs: take slice-9" making "bo" the owner of "slice-9"
    When itos runs "push"
    Then itos exits with code 0
    And the remote's branch ends with "docs: record the widget's decision" then "docs: take slice-9", with no merge commit
    And the recording gh was never called

  # A spec is a docs commit touching a feature file: except keeps it off
  # main, so it goes through its item's pull request, where the reviewer
  # reads every spec (ADR-0061). The refusal names the commit and the key.
  # Fails before the work at the words: the push is refused, exit 1, but by
  # the hook, which names the unknown key, not the commit.
  @ID-PUSHDIRECT-03 @slice-186
  Scenario: Under pr, a docs commit pushed from main that touches a path push.pr.direct.except names is refused, naming the commit and push.pr.direct
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          direct:
            types: [docs]
            except: ['features/**']
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "docs: specify the widget" touching "features/widget.feature"
    When itos runs "push"
    Then itos exits with code 1
    And its output says "docs: specify the widget to origin/main, which is neither the registry's nor a commit push.pr.direct sends straight to main"
    And its output says "git switch -c"
    And the remote's branch does not have "docs: specify the widget"
    And the recording gh was never called

  # A range from main that mixes an admitted commit with work is refused
  # whole: main has no item branch to send the work to, and itos push never
  # reorders commits. The refusal names the oldest commit not admitted, the
  # feat, whose type push.pr.direct does not list. Fails before the work at
  # the words: the hook refuses the unreadable config, naming neither.
  @ID-PUSHDIRECT-04 @slice-186
  Scenario: Under pr, a push from main of an admitted docs commit and a commit of a type push.pr.direct does not name is refused whole
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          direct:
            types: [docs]
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "docs: record the widget's decision" touching "docs/decisions/0001-widget.md"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push"
    Then itos exits with code 1
    And its output says "feat: add the widget to origin/main, which is neither the registry's nor a commit push.pr.direct sends straight to main"
    And the remote's branch does not have "docs: record the widget's decision"
    And the remote's branch does not have "feat: add the widget"
    And the recording gh was never called

  # Bug 149: the description above promises that a type commits.types does
  # not list "admits nothing", since no commit the hooks pass has it. Only
  # the built-in header lint judges a type against commits.types (its rule
  # type-enum), and it is optional: a config that leaves
  # commits.header_lint out, as the scratch config does, turns it off, the
  # commit-msg hook and the pre-push hook's commit rules then running the
  # footer rules alone (message.Check, LintFile), which read no type against
  # the list. So with push.pr.direct.types naming custom, which
  # commits.types lacks, direct() admits a custom: commit, and a push from
  # main sends it to main past the pull request, its checks and its review
  # (found by Astra and Codex's security review in the reviewer replay).
  # A type no scope names has no path limit, so such a commit can carry
  # code, as here: widget.go reaches main unreviewed.
  #
  # The fix: direct() admits a commit only when its type is also one of the
  # commit types (message.Typed: commits.types, or config-conventional's
  # own when the config lists none, as the header lint takes them), so the
  # type check is the push's own and not left to a lint the config may turn
  # off. Such a commit is then judged from main as any commit that is
  # neither the registry's nor admitted: the push is refused whole, exit 1,
  # outcome work-on-main, nothing pushed, in the words that name the oldest
  # such commit and push.pr.direct, with the hint to carry it to its item's
  # branch. No new words: they are the ones the description promised for
  # this case, the commit is named, and main has no pull request for it to
  # stay on. On an item's branch nothing changes (registryPrefix reads no
  # push.pr.direct): it rides the pull request as any commit but the
  # registry's does (ID-PUSHDIRECT-07). config check still accepts a type
  # commits.types lacks, as the description settles: it admits nothing.
  #
  # Before the fix it fails at "itos exits with code 1": direct() reads only
  # push.pr.direct.types, which lists custom, so addsWork finds no work, the
  # pre-push hook passes the commit (no scope names custom, no footer is
  # required for it), and the push sends it to main, exit 0.
  @ID-PUSHDIRECT-11 @bug-149 @wip
  Scenario: Under pr with the header lint off, a commit pushed from main whose type push.pr.direct names but commits.types lacks is refused, naming the commit and push.pr.direct
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          direct:
            types: [docs, custom]
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "custom: tune the widget" touching "widget.go"
    When itos runs "push"
    Then itos exits with code 1
    And its output says "custom: tune the widget to origin/main, which is neither the registry's nor a commit push.pr.direct sends straight to main"
    And its output says "git switch -c"
    And the remote's branch does not have "custom: tune the widget"
    And the recording gh was never called

  # A header with no conventional form is judged by the same type: the one
  # every rule reads (message.Type, the header's first word), not the
  # header's form. With the header lint off nothing refuses "custom the
  # widget", and its type is custom, so it is refused as ID-PUSHDIRECT-11's
  # commit is. Likewise "docs the widget" is a docs commit here, as it is to
  # the footer rules and to commits.scopes, which the pre-push hook applies
  # to it, and goes to main as "docs: the widget" would: requiring a
  # conventional header would make the push stricter than the commit rules
  # beside it, and the type, not the colon, is what push.pr.direct names. A
  # header with no word to read (message.Type "") is no commit type and is
  # never admitted. Before the fix it fails at "itos exits with code 1", as
  # ID-PUSHDIRECT-11 does: direct() admits custom, and the push exits 0.
  @ID-PUSHDIRECT-12 @bug-149 @wip
  Scenario: Under pr with the header lint off, a commit pushed from main with no conventional header, whose first word is a type commits.types lacks, is refused
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          direct:
            types: [docs, custom]
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "custom the widget" touching "widget.go"
    When itos runs "push"
    Then itos exits with code 1
    And its output says "custom the widget to origin/main, which is neither the registry's nor a commit push.pr.direct sends straight to main"
    And the remote's branch does not have "custom the widget"
    And the recording gh was never called

  # push.pr.direct admits a commit past itos push's own refusal, not past
  # main's pre-push hook: the push is the routine's, so the hook's commit
  # rules judge it as they judge a trunk push, and the scratch config's
  # docs scope refuses a Go file. (Under push.mode trunk this scenario's
  # push gives that output today.) Fails before the work at the commit
  # rules' words: the hook refuses the unreadable config before it judges
  # any commit.
  @ID-PUSHDIRECT-05 @slice-186
  Scenario: Under pr, an admitted docs commit pushed from main is still judged by the pre-push hook, which refuses one the commit rules refuse
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          direct:
            types: [docs]
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "docs: describe the widget" touching "widget.go"
    When itos runs "push"
    Then itos exits with code 1
    And its output says "0/1 commits pass the commit rules"
    And its output does not say "push.pr.direct"
    And the remote's branch does not have "docs: describe the widget"

  # --ref main is judged as a push from main (judgeRefs, addsWork), so it
  # admits what a push from main admits, as it refuses what one refuses
  # (ID-PUSHMODE-17). Fails before the work at the exit code: the hook
  # refuses the unreadable config, exit 1.
  @ID-PUSHDIRECT-06 @slice-186
  Scenario: Under pr with push.pr.direct naming docs, push --ref main sends a docs commit to main
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          direct:
            types: [docs]
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "docs: record the widget's decision" touching "docs/decisions/0001-widget.md"
    When itos runs "push --ref main"
    Then itos exits with code 0
    And the remote's branch has "docs: record the widget's decision"
    And the recording gh was never called

  # The second judgement, which commits above main an item's branch sends to
  # main (registryPrefix), is unchanged: a docs commit there is the item's
  # work and rides its pull request, though push.pr.direct admits its type
  # and it sits directly above main. Fails before the work at the exit code:
  # the hook refuses the unreadable config, exit 1; once the key is read it
  # is a guard of what the work must not change.
  @ID-PUSHDIRECT-07 @slice-186
  Scenario: Under pr with push.pr.direct naming docs, a docs commit directly above main on an item's branch still rides its pull request
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          direct:
            types: [docs]
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "docs: describe the widget" touching "docs/widget.md"
    When itos runs "push"
    Then itos exits with code 0
    And the remote's branch does not have "docs: describe the widget"
    And the remote has the branch "slice-9" with "docs: describe the widget"
    And the recording gh's "pr create" call has "--head slice-9"

  # The keys are push.pr's, read only under pr. Fails before the work at the
  # exit code: today config check refuses the unknown key push.pr.direct,
  # exit 2.
  @ID-PUSHDIRECT-08 @slice-186
  Scenario: config check accepts push.pr.direct's types and except
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          direct:
            types: [docs]
            except: ['features/**']
      """
    When itos checks the config
    Then itos exits with code 0

  # Off by default: with no types, a push from main sends the registry's
  # commits alone, as before. Fails before the work at the exit code: today
  # config get refuses the unknown key, exit 2.
  @ID-PUSHDIRECT-09 @slice-186
  Scenario: config get prints push.pr.direct.types' default, the empty list, when the config leaves it out
    When itos runs "config get push.pr.direct.types"
    Then itos exits with code 0
    And its output says "[]"

  # Off changes nothing: without push.pr.direct a docs commit from main is
  # refused as any work is (ID-PUSHMODE-08), in today's words, which name
  # no key the repository did not set. A guard: it passes before the work
  # and must after it.
  @ID-PUSHDIRECT-10 @slice-186 @guard
  Scenario: Under pr without push.pr.direct, a docs commit pushed from main is still refused in today's words
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "docs: record the widget's decision" touching "docs/decisions/0001-widget.md"
    When itos runs "push"
    Then itos exits with code 1
    And its output says "that touch more than"
    And its output does not say "push.pr.direct"
    And the remote's branch does not have "docs: record the widget's decision"
    And the recording gh was never called
