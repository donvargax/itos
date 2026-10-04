@phase-1
Feature: itos follow, private threads with people
  Following up with people is a thread rather than a work item: a teammate
  covered one thing and missed another, the next time brings new questions,
  something works when tested and breaks two days later. itos follow keeps
  such threads with little ceremony (the user's calls, 2026-10-04,
  p1-follow-ups): a thread has an id, the person it is with, a title, a
  status (open or closed) and its notes, each a dated entry appended, never
  edited. They are the person's own: kept in follow-ups.yaml under itos's
  folder of the git common dir, never committed, shared by every worktree
  of the clone, and readable by no one else. A thread can be written out
  whole as Markdown (follow doc), the raw material another agent refines
  into a published document: an architecture, a test plan. Questions to the
  user are not threads but itos ask, which is public. No config is needed:
  any git repository will do.

  Background:
    Given a repository whose ledger has the task "T-001"

  @ID-FOLLOW-01 @slice-61
  Scenario: follow add opens a thread kept under the git common dir, and git sees nothing
    When itos runs the command line "follow add sync-ana --with ana --title 'The sync design' --note 'She covered the retries, not the backoff.'"
    Then itos exits with code 0
    And the file ".git/itos/follow-ups.yaml" exists
    And git status shows nothing to commit

  @ID-FOLLOW-02 @slice-61
  Scenario: follow note appends a dated entry, and follow show prints the notes in order
    Given itos has run the command line "follow add sync-ana --with ana --title 'The sync design' --note 'She covered the retries, not the backoff.'"
    When itos runs the command line "follow note sync-ana 'Backoff agreed: exponential, capped at a minute.'"
    Then itos exits with code 0
    When itos runs "follow show sync-ana"
    Then itos exits with code 0
    And its output says "She covered the retries, not the backoff." before "Backoff agreed: exponential, capped at a minute."

  @ID-FOLLOW-03 @slice-61
  Scenario: follow lists the open threads, and a closed one is not among them
    Given itos has run the command line "follow add sync-ana --with ana --title 'The sync design' --note 'Started.'"
    And itos has run the command line "follow add flaky-bo --with bo --title 'The flaky login test' --note 'Started.'"
    And itos has run the command line "follow close flaky-bo --note 'Fixed by the retry.'"
    When itos runs "follow"
    Then itos exits with code 0
    And its output says "sync-ana"
    And its output does not say "flaky-bo"

  @ID-FOLLOW-04 @slice-61
  Scenario: follow doc writes the whole thread as Markdown at the path given
    Given itos has run the command line "follow add sync-ana --with ana --title 'The sync design' --note 'She covered the retries, not the backoff.'"
    And itos has run the command line "follow note sync-ana 'Backoff agreed: exponential, capped at a minute.'"
    When itos runs "follow doc sync-ana notes/sync.md"
    Then itos exits with code 0
    And the file "notes/sync.md" says "The sync design"
    And the file "notes/sync.md" says "Backoff agreed: exponential, capped at a minute."

  @ID-FOLLOW-05 @slice-61
  Scenario: follow add refuses an id a thread already has
    Given itos has run the command line "follow add sync-ana --with ana --title 'The sync design' --note 'Started.'"
    When itos runs the command line "follow add sync-ana --with bo --title 'Another' --note 'Again.'"
    Then itos exits with code 1
    And its output says "sync-ana"

  @ID-FOLLOW-06 @slice-61
  Scenario: follow note refuses a thread that does not exist
    When itos runs the command line "follow note nobody 'Hello.'"
    Then itos exits with code 1
    And its output says "nobody"

  # Threads live in the git common dir, so a linked worktree (an agent's)
  # reads and writes the same ones as the main checkout.
  @ID-FOLLOW-07 @slice-61
  Scenario: A thread opened in a linked worktree is listed from the main checkout
    Given a linked worktree of the repository at "../wt"
    And itos has run the command line "follow add sync-ana --with ana --title 'The sync design' --note 'Started.'" in the linked worktree
    When itos runs "follow"
    Then itos exits with code 0
    And its output says "sync-ana"

  # Bug 16, found by slice 61's post-landing review: every command read the
  # whole file, changed it and saved it whole with no lock, so two writers at
  # once (a person and an agent's worktree) lost notes and both exited 0; a
  # file holding a second YAML document was cut to its first on the next
  # save; each time printed in its own zone without its offset, so notes
  # from a UTC container and a local shell read out of order; follow doc
  # wrote the private thread world-readable and untracked inside the work
  # tree with no word, one git add -A from a commit, and took - for a file
  # name. Now a lock is held across load, change and save; a file itos did
  # not write whole is refused, untouched; times print in the reader's zone;
  # follow doc writes 0600 and warns of a target inside the work tree that
  # git does not ignore; and follow doc <id> - prints the Markdown. The same
  # lock guards a stealth config's registry and ledger (@ID-STEALTH-23).
  @ID-FOLLOW-08 @bug-16 @wip
  Scenario: Notes written at the same moment are all kept
    Given itos has run the command line "follow add sync-ana --with ana --title 'The sync design' --note 'Started.'"
    When itos runs "follow note sync-ana" with the notes "one" to "twenty" all at once
    Then every run exited 0
    And itos follow show sync-ana lists every one of those notes

  @ID-FOLLOW-09 @bug-16 @wip
  Scenario: A threads file holding a second document is refused, and left as it was
    Given the threads file holds a second YAML document after the thread "sync-ana"
    When itos runs the command line "follow note sync-ana 'Hello.'"
    Then itos exits with code 2
    And the threads file is as it was

  @ID-FOLLOW-10 @bug-16 @wip
  Scenario: A note's time prints in the reader's time zone
    Given the threads file holds the thread "sync-ana" with a note stamped "2026-10-04T14:02:00+09:00"
    When itos runs "follow show sync-ana" with TZ "UTC"
    Then itos exits with code 0
    And its output says "05:02"

  @ID-FOLLOW-11 @bug-16 @wip
  Scenario: follow doc into the work tree, where git does not ignore it, says so
    Given itos has run the command line "follow add sync-ana --with ana --title 'The sync design' --note 'Started.'"
    When itos runs "follow doc sync-ana notes/sync.md"
    Then itos exits with code 0
    And its output says "not ignored"

  @ID-FOLLOW-12 @bug-16 @wip
  Scenario: follow doc to - prints the Markdown and writes no file
    Given itos has run the command line "follow add sync-ana --with ana --title 'The sync design' --note 'Started.'"
    When itos runs "follow doc sync-ana -"
    Then itos exits with code 0
    And its output says "The sync design"
    And the file "-" does not exist
