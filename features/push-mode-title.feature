@phase-1
Feature: Under push.mode pr, itos push titles an item's pull request after the item and describes it by the item, not by its first commit
  Under push.mode pr (slices 174 and 179, push-mode.feature and
  push-mode-draft.feature) itos push opens an item's pull request with gh pr
  create --fill, which takes the title and the description from the commits.
  With one pull request per item the first commit is the item's spec, a docs
  commit, so every pull request was titled "docs: specify ..." and described
  by the spec commit's body (#130, #137, #139), and the coordinator retitled
  them by hand (the person, 2026-10-25, issue 140). Wanted: a pull request
  titled "<item>: <the item's title in the registry>", whose description
  says what the item is, with "Closes" for the issue the item is tracked in.

  The item's title. The title is the registry's, read at the head the push
  sends (after the rebase; with nothing to push, the item branch's head),
  from the registry work.registry names, as itos work show prints it: on a
  first push that is main's registry as the push fetched it, plus the
  branch's own commits. Each run of whitespace in it (a line break of a
  folded YAML text included) is one space, with none at either end. An item
  whose title is no text, or empty once so trimmed (a registry problem work
  check reports), is titled by its id alone, "<item>" in both the title and
  the description: the push never falls back to --fill and never fails for
  it. So is an item the registry at that head lacks, or a registry there
  that cannot be read or parsed: the guard that the branch names an item
  reads the working tree, so the push can reach gh pr create with either,
  the branch already pushed, and failing then would leave a branch with no
  pull request. Neither has a scenario of its own: both take the same path
  as the empty title.

  The pull request's title is "<item>: <the item's title>". GitHub keeps 256
  characters of a pull request's title, so one longer than that is cut: its
  first 255 characters (Unicode code points, not bytes), then "…" (U+2026),
  256 in all. The description always holds the whole title.

  The issues it closes are read from the item's title alone: the issue
  numbers of a parenthesised list that ends the title, "(#<n>)" or
  "(#<n>, #<m>, ...)", each "#" then digits, separated by a comma and
  spaces, nothing after the closing parenthesis but whitespace. That is how
  the registry pilot's items are titled (slice-184's title ends "(#140)",
  p1-ci-e2e-workflow's "(#18, #84)"). A "#<n>" anywhere else in the title
  is a mention, not the item's issue, and an entry that is not "#" and digits
  (another repository's issue, itos-cc#27) makes the whole list none: neither
  closes anything. The why is never read: whys cite issues as context,
  other repositories' among them ("Issue #24 (itos-template, ...)"), and a
  "Closes" there would close the wrong issue when the pull request merges.
  The list is read from the whole title, before any cut, and each number is
  closed once, in the list's order.

  The description (the body) is the item's title, whole, as above; then, when
  the title closes issues, an empty line and one line "Closes #<n>" for each
  (GitHub needs the keyword before each number). So for slice-184 it is
    itos push titles an item's pull request after the item, not after its first commit (#140)

    Closes #140
  GitHub closes the issue when the pull request merges into main, the
  repository's default branch, as T-306 has the pilot's other items close
  theirs with a Closes line.

  The call. gh pr create --repo <repository> --base <main> --head <item>
  --title=<title> --body=<body>, then --draft as before for a draft (slice
  179), in that order. --fill goes, and neither --fill-first nor --fill is
  kept for anything: both read the commits, which is what this slice stops,
  and with --title and --body gh needs neither. The title and the body are
  passed as --title=<value> and --body=<value>, one argument each, so a
  value that begins with "-" is never read as a flag. itos runs gh with its
  arguments (exec, no shell line between), so "$", backticks, quotes, ";",
  "&", "|" and "*" in a title reach GitHub as written, and the line breaks
  of the body are the argument's own. The address gh prints is read as
  before.

  A pull request already open keeps its title and its description, with one
  exception: one whose title is still the one --fill gave it, the subject of
  the first commit of its branch, is retitled. The first commit is the oldest
  of the commits the pushed head adds to main as the push fetched it, the
  registry's commits that go to main left out (the first that goes to the
  item's branch). gh pr list asks --json number,url,isDraft,title (title is
  new) under every setting, and when the open pull request's title is that
  subject, and is not the item's title already, itos push calls gh pr edit
  <item> --repo <repository> --title=<title>, after gh pr list and before any
  gh pr merge, gh pr ready or gh pr update-branch; a plain push under
  push.pr.draft makes it too, since it changes no draft state. It sets the
  title alone: the description is left as it is, since a person may have
  written it, and telling --fill's description from theirs needs the
  commit's body as gh trimmed it; adding a Closes line to a pull request
  opened before this slice is the person's. A title a person edited (any
  title that is not that subject) is never touched, nor is any title on a
  later push once retitled. A pull request opened by hand with gh pr create
  --fill is retitled the same way. A gh pr edit that fails is any gh call
  that fails: exit 3, outcome pull-request-failed, naming gh pr edit. The
  push prints nothing new for it, and --json's pull_request is unchanged:
  url, number, created, updated, draft and auto_merge.

  What it lets through that --fill did not, and why each is accepted: a
  person's title identical to the first commit's subject is retitled (no
  word of theirs is lost: it was the subject); the description is Markdown,
  so an "@login" in an item's title notifies that person and a "#<n>"
  mention links that issue, as a commit body under --fill did; a title cut
  at 255 code points may split a character GitHub counts as one (an emoji
  of several code points), which GitHub shows as it can. --fill gave a pull
  request opened with several commits the branch's name as its title; that
  title is not the first commit's subject, so it is left as it is (none of
  this repository's: the spec opens each alone).

  Trunk is unaffected: under trunk, with no config or one that cannot be
  read, itos push calls no gh (push-mode.feature's guards, ID-PUSHMODE-15
  and ID-PUSHMODE-16), and a branch of the registry's commits alone still
  goes to main with no pull request.

  Old cases. No scenario of the last release quotes --fill: its gh pr create
  checks name --base, --head and --draft, which stay, and the recording gh
  of those scenarios titles every item by its id, which cites no issue, so
  each body is one line. previous-release keeps every promise of v8.19.0
  before this slice, and this slice changes none: a feat with no
  BREAKING-CHANGE and no Changes:, and Upgrading: none. The unit tests in
  internal/slices/push quote --fill and change with the code. The
  implementing commit rewords push-mode.feature's and
  push-mode-draft.feature's descriptions where they say --fill, to point
  here, and command.go's help (gh pr create --fill) and the package's
  doc.go; it adds this file's smoke entry to features/smoke.yaml.

  Not in this slice: updating the description as commits land; retitling a
  pull request when its item is retitled; "Closes" for another repository's
  issue; --fill's title for a pull request opened with several commits.

  The steps. The recording gh is push-mode.feature's, and learns four
  things. It writes a line break inside an argument as the two characters
  backslash and n, so that a call stays one line in its record (a body has
  line breaks), and the steps below read the body that way. pr create holds
  the pull request with the --title=<value> it was given ("" when it was
  given none), and pr list answers each entry's "title" beside its number,
  url, state, isDraft and mergedAt. pr edit <branch> --title=<value> gives
  that branch's open pull request the title, answering nothing. New steps:
  "the work registry has the item <id> owned by nobody with the status
  <status> and the title <title>", which writes the item as "the work
  registry has the item ... owned by nobody with the status ..." does, its
  title the text given, quoted as YAML so that any character but a double
  quote is the title's, and commits it as that step does; "the recording gh
  has the pull request <address> open for the branch <branch>, titled
  <title>", and the same for a draft pull request, which hold it as "the
  recording gh has the pull request ... open for the branch ..." and "the
  recording gh has the draft pull request ... open ..." do, with that title
  (those two steps keep holding theirs untitled, ""); "the recording gh's
  <command> call has a --title of <n> characters ending with <text>", which
  takes a call of the command, the text between "--title=" and the next "
  --body=", and checks that it is n Unicode code points long and ends with
  the text. The rest are push-mode.feature's.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the work registry has the item "slice-12" owned by nobody with the status "todo" and the title "Add the widget (#12)"

  # The slice's point: the first push opens the pull request titled after
  # the item and described by it, closing its issue on merge, with no
  # --fill. Fails before the work at the --title step: today pr create is
  # called with --fill and no --title.
  @ID-PRTITLE-01 @slice-184
  Scenario: Under pr, the first push from an item's branch titles its pull request after the item and closes the item's issue in its description
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-12"
    And the clone has the commit "docs: specify slice-12, the widget" touching "spec.md"
    When itos runs "push"
    Then itos exits with code 0
    And the recording gh's "pr create" call has "--title=slice-12: Add the widget (#12) --body="
    And the recording gh's "pr create" call has "--body=Add the widget (#12)\n\nCloses #12"
    And the recording gh's "pr create" call does not have "--fill"
    And every call of the recording gh has "--repo owner/scratch"
    And its output says "https://github.example/o/r/pull/1"

  # An item may answer several issues: each number of the list that ends
  # its title gets its own Closes line, since GitHub needs the keyword
  # before each. Fails before the work at the --body step: today the body
  # is --fill's.
  @ID-PRTITLE-02 @slice-184
  Scenario: Under pr, an item whose title ends with a list of issues closes each of them
    Given the work registry has the item "slice-13" owned by nobody with the status "todo" and the title "Watch two workflows (#12, #15)"
    And the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-13"
    And the clone has the commit "docs: specify slice-13, two workflows" touching "spec.md"
    When itos runs "push"
    Then itos exits with code 0
    And the recording gh's "pr create" call has "--body=Watch two workflows (#12, #15)\n\nCloses #12\nCloses #15"

  # Being adversarial about the rule: a number mentioned inside the title is
  # context, not the item's issue, and itos-cc#27 is another repository's;
  # closing either on merge would close the wrong issue. The body is the
  # title alone. Fails before the work at the --title step: today pr create
  # has no --title.
  @ID-PRTITLE-03 @slice-184
  Scenario: Under pr, an issue mentioned inside an item's title, or another repository's at its end, closes nothing
    Given the work registry has the item "slice-14" owned by nobody with the status "todo" and the title "Follow #9 into the docs (itos-cc#27)"
    And the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-14"
    And the clone has the commit "docs: specify slice-14, the docs" touching "spec.md"
    When itos runs "push"
    Then itos exits with code 0
    And the recording gh's "pr create" call has "--title=slice-14: Follow #9 into the docs (itos-cc#27) --body=Follow #9 into the docs (itos-cc#27)"
    And the recording gh's "pr create" call does not have "Closes"

  # This repository's own setting: a draft is opened with the same title
  # and description, --draft after them. Fails before the work at the
  # --title step: today the call is --fill --draft.
  @ID-PRTITLE-04 @slice-184
  Scenario: Under push.pr.draft, the first push opens the draft titled after the item and described by it
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-12"
    And the clone has the commit "docs: specify slice-12, the widget" touching "spec.md"
    When itos runs "push"
    Then itos exits with code 0
    And the recording gh's "pr create" call has "--title=slice-12: Add the widget (#12) --body=Add the widget (#12)\n\nCloses #12 --draft"
    And the recording gh was never called as "pr merge"

  # GitHub keeps 256 characters of a title: a longer one is cut to 255 and
  # an ellipsis, while the description keeps it whole and its Closes, read
  # from the whole title, survives the cut. Fails before the work at the
  # --title step: today pr create has no --title.
  @ID-PRTITLE-05 @slice-184
  Scenario: Under pr, an item's title too long for GitHub is cut in the pull request's title and kept whole in its description
    Given the work registry has the item "slice-15" owned by nobody with the status "todo" and the title "Make the long title of an item, one that runs well past what GitHub keeps for the title of a pull request, reach that pull request whole in its description, while the title itself is cut to the length GitHub allows and ends with an ellipsis, so that nothing a person wrote is lost on the way, and the rest is in the description (#12)"
    And the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-15"
    And the clone has the commit "docs: specify slice-15, the long title" touching "spec.md"
    When itos runs "push"
    Then itos exits with code 0
    And the recording gh's "pr create" call has a --title of 256 characters ending with "so …"
    And the recording gh's "pr create" call has "the rest is in the description (#12)\n\nCloses #12"

  # itos passes arguments, never a shell line: what a shell would expand or
  # split reaches GitHub as the registry wrote it. Fails before the work at
  # the --title step: today pr create has no --title.
  @ID-PRTITLE-06 @slice-184
  Scenario: Under pr, an item's title with characters a shell would read reaches the pull request as written
    Given the work registry has the item "slice-16" owned by nobody with the status "todo" and the title "Keep $HOME, `pwd`, 'quotes', a & b; c | d and *.md as written"
    And the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-16"
    And the clone has the commit "docs: specify slice-16, the characters" touching "spec.md"
    When itos runs "push"
    Then itos exits with code 0
    And the recording gh's "pr create" call has "--title=slice-16: Keep $HOME, `pwd`, 'quotes', a & b; c | d and *.md as written --body=Keep $HOME, `pwd`, 'quotes', a & b; c | d and *.md as written"

  # A pull request opened before this slice, or by hand with --fill, still
  # has its first commit's subject as its title: the next push retitles it,
  # the title alone, and opens nothing. Under push.pr.draft, since a plain
  # draft push stops early and must retitle all the same. Fails before the
  # work at the --json step: today pr list asks number,url,isDraft.
  @ID-PRTITLE-07 @slice-184
  Scenario: Under pr, an open pull request still titled by --fill after its first commit is retitled after the item, its description left as it is
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-12"
    And the clone has the commit "docs: specify slice-12, the widget" touching "spec.md"
    And git pushes HEAD to the remote's new branch "slice-12"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-12", titled "docs: specify slice-12, the widget"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push"
    Then itos exits with code 0
    And the recording gh's "pr list" call has "--json number,url,isDraft,title"
    And the recording gh's "pr edit" call has "slice-12 --repo owner/scratch --title=slice-12: Add the widget (#12)"
    And the recording gh's "pr edit" call does not have "--body"
    And the recording gh was never called as "pr create"
    And its output says "https://github.example/o/r/pull/7"

  # A title a person wrote is theirs: itos push never edits it. A guard: it
  # passes before the work, which calls no gh pr edit, and must after it.
  @ID-PRTITLE-08 @slice-184 @guard
  Scenario: Under pr, an open pull request whose title a person edited keeps it
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-12"
    And the clone has the commit "docs: specify slice-12, the widget" touching "spec.md"
    And git pushes HEAD to the remote's new branch "slice-12"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-12", titled "The widget, at last"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push"
    Then itos exits with code 0
    And the recording gh was never called as "pr edit"
    And the recording gh was never called as "pr create"
    And the recording gh's "pr merge" call has "slice-12"
