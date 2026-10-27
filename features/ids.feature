@phase-1
Feature: The id counter, so an id itos mints is minted once
  itos mints every id it writes (q-26): an item's (work add --kind
  slice|task, task add, work promote, slice 102) and a scenario's (tests
  next-id, slice 105). Sessions on two machines may mint at the same
  moment, and whatever names an item before it reaches main (a feature
  file's tags, a footer, a draft, a brief) writes its id, so the id itself
  must not be minted twice (the user's calls, 2026-10-08, q-27). One
  counter holds the last number minted for each item kind and each
  scenario area. In a repository whose itos data is shared through a
  remote, it is the ref refs/itos/ids on the remote itos push pushes to: a
  commit whose tree holds the counters. Minting fetches it, takes one past
  the higher of its counter and the highest the repository holds, and
  pushes a child commit with that counter raised, fast-forward only, never
  forced; when another machine moved the ref first, the push is refused,
  and itos fetches again and retries. A number is never given back: one a
  dropped draft or a removed scenario held leaves a gap. Under a stealth
  config, or with no remote, the counter is a file in itos's folder of the
  git common dir instead, which no git command can push (a ref could leave
  with git push --mirror, and a stealth config shows nothing of itos to
  anyone); the same counters, written under a lock. A remote that cannot be
  reached refuses the mint with exit 3, writing nothing: a numbered item
  cannot land offline anyway, and an idea, which keeps its author's name,
  can still be added.

  # Each scenario makes its own repository: one shared through a remote,
  # one kept in its git folder (a stealth config).

  @ID-IDS-01 @slice-102
  Scenario: Minting with a remote claims the number in the remote's counter
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    When itos runs the command line "work add --kind slice --title 'Do the thing' --why 'Because.'"
    Then itos exits with code 0
    And its output says "slice-10"
    And the remote's id counter for "slice" is 10

  # Another machine's mint is made in a second clone of the remote, by the
  # itos under test, before this clone mints: this clone's registry still
  # tops out at slice-9.
  @ID-IDS-02 @slice-102
  Scenario: A number another clone claimed is not minted again
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    And another clone of the remote has minted "slice-10"
    When itos runs the command line "work add --kind slice --title 'Do the thing' --why 'Because.'"
    Then itos exits with code 0
    And its output says "slice-11"
    And the remote's id counter for "slice" is 11

  @ID-IDS-03 @slice-102
  Scenario: A counter behind what the repository holds mints past the repository's highest
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And another clone of the remote has minted "slice-10"
    And the work registry has the item "slice-12" owned by nobody with the status "todo"
    When itos runs the command line "work add --kind slice --title 'Do the thing' --why 'Because.'"
    Then itos exits with code 0
    And its output says "slice-13"

  @ID-IDS-04 @slice-102
  Scenario: A remote that cannot be reached refuses the mint with exit 3, and writes nothing
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the remote cannot be reached
    When itos runs the command line "work add --kind slice --title 'Do the thing' --why 'Because.'"
    Then itos exits with code 3
    And the registry has no item "slice-10"

  @ID-IDS-05 @slice-102
  Scenario: Under a stealth config the counter stays in the git folder, and no ref is made or pushed
    Given a repository whose ledger has the task "T-001", kept in its git folder
    And the stealth registry has the item "slice-9" owned by nobody with the status "todo"
    And the repository has a remote it pushes to
    When itos runs the command line "work add --kind slice --title 'Do the thing' --why 'Because.'"
    Then itos exits with code 0
    And its output says "slice-10"
    And the repository has no ref "refs/itos/ids"
    And the remote has no ref "refs/itos/ids"

  # The counter's push runs no hook (the user's call, 2026-10-08): its
  # commit has no conventional header and shares no history with the
  # branches, so a pre-push hook would refuse it or run every unit test, and
  # a project's own pre-push commands under any hook manager would run on
  # every mint. Hooks exist to stop bad commits reaching a branch, and this
  # push updates none: it names only refs/itos/ids, nothing lands on main
  # through it, and no CI workflow runs for it (GitHub's push events are for
  # branches and tags). So itos pushes it with git push --no-verify, the one
  # push that skips hooks, and only inside its minting code (work add, task
  # add, work promote, tests next-id, draft add): the ref is a constant, the
  # commit the counter commit it has just built, and no itos command takes a
  # refspec, a ref or a flag that reaches that push. itos runs git itself, so
  # the plugin's guard still refuses git push --no-verify typed anywhere.
  @ID-IDS-06 @slice-102
  Scenario: The counter's push runs no pre-push hook, none of hooks.pre_push's commands running
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the pre-push hook is installed
    And hooks.pre_push's commands record that they ran
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    When itos runs the command line "work add --kind slice --title 'Do the thing' --why 'Because.'"
    Then itos exits with code 0
    And the remote's id counter for "slice" is 10
    And none of hooks.pre_push's commands ran

  # The same parent, category and number must produce distinct counter commits:
  # concurrent clones can both read the old ref before either push is accepted.
  @ID-IDS-07 @bug-102 @slice-102
  Scenario: Concurrent clones claim distinct numbers from the same remote counter
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    When the clones concurrently add a slice
    Then every run exited 0
    And the concurrent clones minted "slice-10" and "slice-11"
    And the remote's id counter for "slice" is 11

  # A git that fails as itos reserves an ID is an error nobody classified:
  # exit 70, as v8.5.2 exits. T-218's refactor made it the locate's
  # missing-environment error, exit 3; the user restored 70 (2026-10-21).
  # The wording is not judged: prose is no contract (ADR-0052). The git that
  # fails is the one itos runs (ITOS_GIT), set up after the clone, so the
  # repository is real and only git's answers fail.
  @ID-IDS-08 @bug-119
  Scenario: A git that fails while itos reserves an ID exits 70, and writes nothing
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the git itos runs fails every command
    When itos runs the command line "work add --kind slice --title 'Do the thing' --why 'Because.'"
    Then itos exits with code 70
    And the registry has no item "slice-10"

  # Bug 141: two clones pushing refs/itos/ids at the same moment. A push is
  # checked against the ref twice: by git push, against the ref the remote
  # advertised ("fetch first", which itos retries), and by the remote's
  # receive-pack as it updates the ref under its lock. A push that passes the
  # first and loses the second is refused with "remote: error: cannot lock
  # ref 'refs/itos/ids': is at <theirs> but expected <ours>" and "! [remote
  # rejected] ... (incorrect old value provided)", which itos took for a real
  # refusal: exit 3, "cannot pushing id counter on remote" (T-310's
  # concurrent property found it). The race is made deterministic with git
  # itself. The new Given writes git's update hook in the remote (the bare
  # origin.git, its hooks folder; a #!/bin/sh script, run by Git's sh on
  # Windows as the clone's hooks are), which receive-pack runs before it
  # updates each ref and outside the push's quarantine (a pre-receive hook
  # may not move a ref there). On the first push of refs/itos/ids only (a
  # marker file in the remote's folder) it builds, with hash-object, mktree
  # and commit-tree -p on the old value git hands it, another clone's counter
  # commit whose counters/slice is 11, moves the ref to it with update-ref
  # <new> <old>, and exits 0; later pushes it lets through. receive-pack then
  # finds the ref moved under it and refuses the push in the race's own
  # words ("cannot lock ref ... is at ... but expected", and the reason
  # "incorrect old value provided" where the machine's git gives it; an
  # older git says "failed to update ref"). The retry fetches 11 and claims
  # 12 in one push: the remote's counter is 12, not 13. Before the fix it
  # fails at "itos exits with code 0" (it exits 3).
  #
  # Decided (the coordinator, 2026-10-25): a lost race is told by the remote's
  # state, not by git's words, which differ across versions (this bug is one
  # wording missed): after a refused push, itos fetches the counter again and
  # retries, within its attempts, when refs/itos/ids moved past the commit the
  # attempt built on, whatever git said; a refusal that leaves the ref where it
  # was is a real one (ID-IDS-10). So a lock another push held, and won, is a
  # lost race too, and the word list of nonFastForward need not grow. A real
  # refusal keeps exit 3. ID-IDS-09 alone cannot tell this from adding git's
  # words to that list; ID-IDS-11 can.
  @ID-IDS-09 @bug-141
  Scenario: A counter push that loses the race as the remote updates the ref fetches the counter again and retries
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    And another clone of the remote has minted "slice-10"
    And another clone claims "slice-11" on the remote while the clone's next counter push is received
    When itos runs the command line "work add --kind slice --title 'Do the thing' --why 'Because.'"
    Then itos exits with code 0
    And its output says "slice-12"
    And the remote's id counter for "slice" is 12

  # The decision's own check: the remote's update hook moves refs/itos/ids as
  # in ID-IDS-09, then declines the push itself (exit 1), so git's words are
  # "hook declined", which no word list names, while the ref has moved past
  # the commit the attempt built on. Judged by the ref, itos fetches and
  # retries, and the hook lets the retry through; judged by git's words, it
  # exits 3. The new Given is ID-IDS-09's with "and the remote declines that
  # push". Before the fix it fails at "itos exits with code 0" (it exits 3),
  # and a fix that only adds git's lost-race words to nonFastForward still
  # fails there.
  @ID-IDS-11 @bug-141
  Scenario: A counter push refused after the remote's ref moved is retried, whatever words the remote gave
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    And another clone of the remote has minted "slice-10"
    And another clone claims "slice-11" on the remote while the clone's next counter push is received, and the remote declines that push
    When itos runs the command line "work add --kind slice --title 'Do the thing' --why 'Because.'"
    Then itos exits with code 0
    And its output says "slice-12"
    And the remote's id counter for "slice" is 12

  # A refusal that is no lost race is not retried: exit 3, as today, and
  # nothing written, no counter on the remote. The new Given writes the
  # remote's update hook as above, one that prints its words on stderr and
  # exits 1 for every push of refs/itos/ids (git relays them as "remote:
  # <words>", the reason "hook declined"), the ref never moving. The message
  # was ungrammatical, as its fetching and reading siblings are: it becomes
  # "cannot push the id counter to the remote (nothing was written): <git's
  # error>", and the others "cannot fetch the id counter from the remote" and
  # "cannot read the id counter on the remote", with the same tail. The
  # output is checked loosely (ADR-0052): that it names the push, and relays
  # the remote's reason. Before the fix it fails at its output says "cannot
  # push the id counter to the remote".
  @ID-IDS-10 @bug-141
  Scenario: A counter push the remote refuses for another reason is not retried, exits 3, and writes nothing
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the remote refuses every push of "refs/itos/ids", its hook saying "refs/itos/ids is frozen here"
    When itos runs the command line "work add --kind slice --title 'Do the thing' --why 'Because.'"
    Then itos exits with code 3
    And its output says "cannot push the id counter to the remote"
    And its output says "refs/itos/ids is frozen here"
    And the registry has no item "slice-10"
    And the remote has no ref "refs/itos/ids"

  # Bug 153 (found 2026-10-27 by T-329's FuzzStrictYAML oracle): the counter
  # file (.git/itos/ids.yaml, the counter under a stealth config or with no
  # remote) was read with yaml.Unmarshal, not value.Parse, which reads every
  # other YAML file itos reads. So it read the first document alone, a second
  # one, broken or not, ignored; it missed a key written twice and an alias
  # that never ends under any key but counters; and it read YAML 1.1's
  # numbers: 017 as 15 (octal), 1_000 as 1000 and 0b11 as 3, where Parse
  # reads 17 and two strings, and 1.5 as 1.
  #
  # Decided (the coordinator, 2026-10-27): the counter file is read through
  # value.Parse, as upgrade's workflow reader does since K150b, and Parse's
  # value is held to the file's shape. Refused, writing nothing:
  # - whatever Parse refuses, anywhere in the file: a syntax error, a second
  #   document, a key written twice at any depth, an alias inside its own
  #   anchor or expanding past the bound Parse reads through (bug 39);
  # - a top that is no mapping, and a counters that is no mapping (an empty
  #   file, a file without counters and an empty counters are no counters,
  #   as before);
  # - a counter that, as Parse reads it, is no whole number from 0 to
  #   9007199254740991 (2^53 - 1, the largest whole number Parse's numbers,
  #   JavaScript's, hold exactly): 1_000 and 0b11, strings to Parse, a quoted
  #   number, null, 1.5, -3. Such a counter is refused rather than read as
  #   none, since the counter exists so that no number is minted twice, and
  #   one read lower than it was written would mint a number handed out.
  # A number is read as Parse reads it: 017 is 17, 0x10 16, 1e3 1000, and
  # the key 0x10 is the category "16". Keys beside counters are read by
  # Parse, and otherwise left alone, as before.
  #
  # A refusal is a data file itos refuses: exit 2 (kind.Usage, ADR-0036).
  # The key written twice that yaml/v3 refused too exited 70, the code of an
  # error that is itos's to fix, which a hand edit is not. The message keeps
  # its form, "read id counter <file>: <why>", the why Parse's error or the
  # counter's category and value, checked loosely (ADR-0052).
  #
  # itos writes the file with yaml.Marshal of a map of category to int: one
  # document, a block mapping, every counter a plain decimal and every
  # category letters, digits and dashes, quoted where YAML would read it as
  # something else. So no file itos wrote reads otherwise, and the change
  # reaches a hand-edited one alone (and a counter past 2^53 - 1, which only
  # nine quadrillion claims reach: refused, so no number is minted twice).
  # The remote's counter, one file of digits a category, read with
  # strconv.Atoi, is not this reader's and does not change. The fix lands
  # T-329's held FuzzStrictYAMLCounter in internal/idcounter, with seeds of
  # each form above and the one T-329 held, "&0,", a top Parse reads as a
  # string.
  #
  # Each scenario writes the counter file by hand, uncommitted, into a
  # repository with no remote, where the counter is that file.
  #
  # A second document: the first counters say task 1, the second task 20.
  # Before the fix the reader takes the first, task add mints T-002 and exits
  # 0, so it fails at "itos exits with code 2". The Given is new: it writes
  # the doc string to the file, as "the file ... is changed to hold" writes
  # one line (draft_test.go).
  @ID-IDS-12 @bug-153
  Scenario: A counter file with a second document is refused with exit 2, and nothing is minted
    Given a repository whose ledger has the task "T-001"
    And the work registry has the item "T-001" owned by nobody with the status "done"
    And the file ".git/itos/ids.yaml" holds the lines:
      """
      counters:
        task: 1
      ---
      counters:
        task: 20
      """
    When itos runs the command line "task add --group 1 --type chore --title 'Tidy the readme' --why 'It drifted.' --check 'true'"
    Then itos exits with code 2
    And its output says "more than one document"
    And the registry has no item "T-002"
    And the registry has no item "T-021"
    And the file ".git/itos/ids.yaml" says "task: 20"

  # 017 is 17, as Parse reads it, not YAML 1.1's octal 15. The area's files
  # hold @ID-A-01, so the counter alone sets the number. Before the fix the
  # reader reads 15 and tests next-id prints @ID-A-16, so it fails at its
  # output says "@ID-A-18".
  @ID-IDS-13 @bug-153
  Scenario: A counter written 017 is read as 17, as itos reads every other YAML file
    Given a repository whose ledger has the task "T-001"
    And a feature file "a.feature" with the live scenario "@ID-A-01"
    And the file ".git/itos/ids.yaml" is changed to hold "counters: {ID-A: 017}"
    When itos runs "tests next-id scenario ID-A"
    Then itos exits with code 0
    And its output says "@ID-A-18"
    And the file ".git/itos/ids.yaml" says "ID-A: 18"

  # The one-line forms, in flow style. Before the fix the key written twice
  # exits 70 and every other form mints (slice-10, or slice-1001 from 1_000)
  # and exits 0, so each fails at "itos exits with code 2".
  @ID-IDS-14 @bug-153
  Scenario Outline: A counter file Parse refuses, or a counter that is no whole number, is refused with exit 2, and nothing is minted
    Given a repository whose ledger has the task "T-001"
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the file ".git/itos/ids.yaml" is changed to hold "<file>"
    When itos runs the command line "work add --kind slice --title 'Do the thing' --why 'Because.'"
    Then itos exits with code 2
    And its output says "id counter"
    And the registry has no item "slice-10"
    And the file ".git/itos/ids.yaml" says "<file>"

    Examples:
      | form                                       | file                                   |
      | a key written twice                        | counters: {slice: 1, slice: 20}        |
      | an alias that never ends, beside counters  | {counters: {slice: 1}, notes: &n [*n]} |
      | 1_000, a string to Parse                   | counters: {slice: 1_000}               |
      | 0b11, a string to Parse                    | counters: {slice: 0b11}                |
      | 1.5, no whole number                       | counters: {slice: 1.5}                 |
      | a number below 0                           | counters: {slice: -3}                  |
