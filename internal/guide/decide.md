# Deciding with the person

Read before you ask the person anything, change the queue, record a decision
or add a lesson.

## Talking to the person

- **A decision needed from the person goes through
  `itos decision add '…' --item <id>`**, so it outlives the session; the
  person answers with `itos decision answer`. Anything to take up with
  someone else, a question to ask them included, is `itos followup`, a
  private thread, never `itos decision`.
- **Ask with a recommendation**: the recommended option first, then the
  others. Say in a line what the item is for before asking about its design.
- **Say why each proposed item matters**: what it fixes or saves, and what it
  waits on. Mention the cost when proposing many more agents.
- **Be wary of scope creep.** A fix that grows is a new item. Record a gap as
  an idea (`itos work add <idea-id> --title '…' --why '…'`), not as work folded
  into the current one. The person's review is final.
- **The queue is the order the person wants.** An item the person puts off is
  deferred with `itos work defer <id> --why '…'`, and lifted with
  `itos work resume <id>`, each committing the registry alone; one already in
  progress is given back first, `itos work give-back <id> --why '…'`. The order is
  kept by `itos work queue <id> --top`, `--before <id>`, `--after <id>` or
  `--remove`, each committing the registry alone; `itos work done` takes a
  closed item out. An item no longer wanted is dropped, once the person
  agrees, with `itos work drop <id> --why '…'`, the reason its commit's body;
  it stays in the registry, never proposed again. Defer, resume, give-back,
  queue, drop and edit take several ids: one change for each, one commit.

## Decisions

- **An answered question is a decision.** `itos decision` names the answered
  ones recorded nowhere else. Offer the person
  `itos decision record <id> --title '…'` for an answer that sets a
  direction beyond its item, and `--none` for one that only settled its
  item; write a record only when they agree.
- **Records live in `docs/decisions/`** (or the folder `work.decisions`
  names), one MADR file each, and its README.md lists the live ones. Read
  that index, not the superseded records.
- **A repository whose docs already hold decisions** (a plan, a design
  file) moves them into records as one task, the person agreeing first:
  - Only what was decided becomes a record, one per decision; what the docs
    describe (a command's options, how a mode works) stays in the docs or
    the help. A row holding several calls stays one record while it fits a
    screen.
  - Seed each one through `itos decision add`, `decision answer` and
    `decision record`, so every record keeps the question it answers and the format stays itos's.
  - `git grep` the old doc's name for its citations: point one at a section
    that left to its record, and keep one at a section that stayed. Code
    comments change in a `refactor` commit, the rest in `docs` commits; a
    feature file's description changes only in a feat or a fix.
  - Aim the task's checks at what it changes: a grep over whole folders also
    hits fixtures that keep old text on purpose.

## Lessons

A mistake an agent or you made, and that a rule in the repository's own
notes would prevent, becomes a lesson there:

- **It names its exit:** the item that makes it unnecessary (a check, a
  gate, a fix), or "permanent" when it needs judgement. When the exit item
  is done, delete the lesson in the same commit.
- **It gives two dates:** the day it was recorded and the day the mistake
  was last seen. Update the second when the mistake comes back.
- **Keep at most ten.** To add one, turn one into an item or delete it:
  every lesson is read by every session, so a long list costs each one.
