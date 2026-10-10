// Package tests is named tests behind an adapter: which tests a kind has at
// a tree (the working tree, the index or a commit) and which are live, as
// the adapter protocol's list, the kind's smoke set with its rule, the run
// templates, and the rule on how a kind's tests may change. A kind (here
// one, scenario) says how its tests are listed and run; nothing else in itos
// knows Gherkin or the runner.
//
// # Adapters
//
// The built-in Gherkin adapter (gherkin.go, ParseFeature) is the only reader
// of a feature file; a scenario is live when neither its tag line nor its
// file's header holds @wip. A command adapter is `<command> list --at
// <tree>` through the config's shell, its output read by value.ParseJSON
// (keys in JavaScript's order) and held to the protocol, its failures
// carrying its stderr and its exit as shell.Result.Status; supports_at:
// false warns once a run on Warnings, which cli.Main points at its stderr. A
// command adapter's listing keeps each object as printed in Raw, which itos
// tests list --json prints key for key, extra keys included. ListTests is a
// kind's listing at a tree, ListTestsUnder the same with tests smoke check
// --features <dir> standing in for a Gherkin kind's root. Tagged gives the
// Gherkin kinds' scenarios whose tag line, or their file's, holds a tag
// (@slice-<n> for the item slice-<n>), for itos work done and itos work
// show; a command adapter's list carries no tags and is left out.
//
// # Run templates
//
// CommandFor(cfg, kind, selections) is the one function that turns
// Selections (Whole(), IDs(…), Pattern(p)) into one command through the
// kind's run templates (run.go): whole if any selection is whole, else a
// pattern per IDs selection by ids_pattern (its {ids} the bare IDs,
// deduplicated, joined by |), the patterns deduplicated in order and, when
// more than one, each in join.each's {p} joined by join.sep, the result one
// shell word (ShellWord) in select's {pattern}; each placeholder is replaced
// once. It says false when nothing is selected (an IDs selection with no IDs
// adds no pattern), and a template the selections need and the kind lacks is
// a config error. itos compiles none of the templates' patterns: the runner
// reads them in its own dialect. itos tests smoke run is it over the smoke
// IDs, its exit the runner's or 1; CI's plan calls the same function for its
// one merged run. Recognize (recognize.go) reads a task check back as a
// selection, each recognize template a pattern whose {pattern} is one shell
// word, the bare word bounded by JavaScript's whitespace rather than RE2's,
// tried on the command as config.Readings reads it.
//
// # The smoke set
//
// A kind's smoke set (tests.<kind>.smoke.file) has one loader (LoadSmoke,
// LoadSmokeAt) and one rule (SmokeIssues), which itos tests smoke check runs
// and CI runs as a step.
//
// # Range checks and the moves rule
//
// tests.<kind>.range_checks is a kind's rule on how its tests may change
// between two trees: commands (staged, run by the commit-msg hook on the
// index; range, run once by verify over the range with {from} and {to},
// RangeCommands, {from} at RangeFrom) or the built-in moves rule (builtin:
// moves). Both hook and verify skip a type except_types names, and one that
// is not in commits.types when the config lists them (a merge's "Merge …");
// the command ranges wait for the type to have a path rule, the built-in
// does not. FillRange fills {from} and {to} in a CI step's command, each one
// shell word, so itos ci plan prints the command itos ci run runs.
//
// The moves rule (moves.go) is one comparison, MoveProblems over two
// FeatureSets that ReadFeatures builds through ParseFeature with comment
// lines dropped (so a scenario's reason may be written beside it in any
// commit), its IDs and files kept in the order JavaScript's Maps keep them
// (files by path, as git lists them; an ID written twice keeps its first
// place and its last block), since the problems are printed in that order.
// On a kind whose adapter is a command the rule compares two adapter
// listings instead (ListedProblems, moves_command.go), each read once per
// tree at the index or a commit, a tree that names no commit (the empty
// tree, an unborn HEAD) listing nothing, so it needs supports_at: true: no
// live test added or lost, none switched between live and wip, and a live
// test's title, when the adapter gives one at both ends, kept unless
// allowed_renames lists the new one; bodies are not compared. config check
// refuses builtin beside a command, on a command kind without supports_at:
// true or a kind whose named adapter is not gherkin, and allowed_renames
// without it.
//
// NewMoves(cfg) reads each kind's feature files once per tree and gives the
// callers: Commit(sha, type) for verify (the commit against git.Parent, the
// empty tree for a root commit, nothing read when no check judges the type),
// Between for the commit-msg hook (HEAD, or for an amend HEAD's parent,
// against the index), Merge for a merge commit in both (its own paths, the
// kind's root joined to each test's file, as its first parent lists them),
// and Index(kind) for itos tests moves. Between and Merge go through one
// judge, which picks the comparison by the kind's adapter.
//
// NextTag (nextid.go) is itos tests next-id's tag (internal/nextid), and
// Area whether a stem is an area of a kind's ID- pattern, whose tags tests
// next-id claims through the id counter.
package tests
