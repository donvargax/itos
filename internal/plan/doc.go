// Package plan is what a CI run runs, decided once, so that itos ci plan
// prints it and itos ci run carries it out from the same value.
//
//   - A push whose range CI can read runs one run of the kind of named tests
//     its tests: step names, over the kind's smoke set, the tests its footers
//     name (Scenarios:) and the subsets in the done_when of the tasks its
//     ledger footers (Task:) name; a range it cannot read runs every test. A
//     task check the kind's recognize templates read as a run of the kind is
//     merged into that run, unless the plan has no such run (nothing at all
//     selected: an empty smoke set, a range that names no test): then it
//     runs as itself. as: whole and as: pattern always select something, so
//     their run is always there; a check read as: smoke runs as itself.
//   - A named task's check that is one of the steps this run runs, or that a
//     ci.covers rule gives to one of them, is covered and not run again. A
//     check in ci.nightly_only is left out of the push's run, and a task
//     whose work item is still todo waits (ci.wait_on_status).
//   - A named task's check marked after: push is pending: it means something
//     only once the push has landed (a release, a tag, the run itself), so a
//     push's run lists it, neither runs it nor counts it, and leaves it to
//     itos task and itos work done. It is never merged or covered.
//   - The run is in cost order: the static steps, the named tasks' static
//     checks, the other steps, then the other checks (internal/check's cost
//     classes and written order), a task's checks keeping their written
//     order. With ci.keep_step_order it is the steps as written, then the
//     named tasks' checks in their ledger order (asWritten), for a
//     repository whose CI ran its steps in a chosen order; the cost class
//     still decides what the commit-msg hook and the nightly's static step
//     run, and what a prose range keeps.
//   - The nightly runs its own steps in the order written; its { tasks: done
//     } step stands for the checks of every task whose work item is done
//     (DoneTasks, by work.ItemStatuses), in cost order where it is written
//     (only the static ones with cost: static). config check refuses that
//     step in ci.steps, where the tasks are the ones the commits name.
//   - A prose-only range (only ci.prose.paths) runs the prose steps, then
//     the named tasks' static checks and those that say prose: true, and
//     lists what it leaves out.
//
// The Plan is a value the driver (internal/ci) walks: Order is every Item in
// the order the run takes it, a Step (its command, its cost, and Tests, the
// kind, when it is the one run of named tests) or a Check (a
// check.CostedCheck, so its task with the title a failure names, its place
// in done_when and its cost, plus its Action: run, merged into the kind's
// run, covered by a step, nightly, or pending); Steps, Tasks, LeftOut,
// Unknown and NotStarted sit beside it for the preamble. Make plans an
// Input; For(cfg, from, to, Data) plans a range, DataAt reading the ledger,
// the registry and the smoke set from the working tree or, for --data-at,
// from a commit under source.ReadingFrom (the config stays the working
// tree's); ForNightly plans the nightly. Fields and Print are the JSON and
// text itos ci plan prints, a contract: their keys, order and values are
// held by the corpus.
//
// A range's commits name tasks and tests through message.IDsIn (TasksIn,
// TestsNamedIn), and its paths are Changed. The range is from..to as given:
// commits.since does not narrow it, as it narrows verify's, and an empty
// start is unread (Readable), so every test runs, yet the footers of every
// commit up to the end are read, so the tasks and tests they name run too.
// A stealth config's itos ci plan with no range plans the unpushed commits
// (git.Unpushed; RangeOf gives the JSON's from as --remotes). The plan
// carries its range's Ends (ends.go): the start a range check's {from} gets
// (tests.RangeFrom: the unpushed commits' base, commits.since) and the end,
// as full SHAs, the start empty for a range that runs everything; the
// nightly's are those of every commit up to HEAD. Once the plan is made,
// {from} and {to} in each step's command, and in the step a covered check
// names, are filled in by tests.FillRange, so itos ci plan prints the
// command itos ci run runs, while the cost class and ci.covers read the step
// as ci.steps writes it.
package plan
