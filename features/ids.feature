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
  # words. So the scenario exercises "cannot lock ref ... is at ... but
  # expected", which git prints whatever its version, and "incorrect old
  # value provided" only where the machine's git gives that reason (2.56
  # does; an older git gives "failed to update ref"), each wording a case of
  # the classifier's unit test too. A hook that only printed the words and
  # declined would give the reason "hook declined" and leave the ref
  # unmoved, as no real race does, and would refuse a fix that retries when
  # the ref moved, whatever git's words. The retry fetches 11 and claims 12
  # in one push: the remote's counter is 12, not 13. Before the fix it fails
  # at "itos exits with code 0" (it exits 3).
  #
  # Decided (the coordinator, 2026-10-25): a lost race is told by the remote's
  # state, not by git's words, which differ across versions (this bug is one
  # wording missed): after a refused push, itos fetches the counter again and
  # retries, within its attempts, when refs/itos/ids moved past the commit the
  # attempt built on, whatever git said; a refusal that leaves the ref where it
  # was is a real one (ID-IDS-10). So a lock another push held, and won, is a
  # lost race too, and the word list of nonFastForward need not grow. A real
  # refusal keeps exit 3.
  @ID-IDS-09 @bug-141 @wip
  Scenario: A counter push that loses the race as the remote updates the ref fetches the counter again and retries
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    And another clone of the remote has minted "slice-10"
    And another clone claims "slice-11" on the remote while the clone's next counter push is received
    When itos runs the command line "work add --kind slice --title 'Do the thing' --why 'Because.'"
    Then itos exits with code 0
    And its output says "slice-12"
    And its output does not say "cannot pushing"
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
  @ID-IDS-10 @bug-141 @wip
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
