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
// of a feature file's scenarios, by their tag lines, but for their
// fingerprints (below); a scenario is live when neither its tag line nor its
// file's header holds @wip. A command adapter is `<command> list --at
// <tree>` through the config's shell, its output read by value.ParseJSON
// (keys in JavaScript's order) and held to the protocol, its failures
// carrying its stderr and its exit as shell.Result.Status; supports_at:
// false warns once a run on the run's Warnings, passed in (cli.Main makes
// one on its stderr, carried in cli.Out and message.Reading). A command
// adapter's listing keeps each object as printed in Raw, which itos tests
// list --json prints key for key, extra keys included. ListTests is a
// kind's listing at a tree, ListTestsUnder the same with tests smoke check
// --features <dir> standing in for a Gherkin kind's root. Tagged gives the
// Gherkin kinds' scenarios whose tag line, or their file's, holds a tag
// (@slice-<n> for the item slice-<n>), for itos work done and itos work
// show; a command adapter's list carries no tags and is left out.
// TaggedExamples adds the Examples blocks tagged @wip under the live ones,
// read through Gherkin's parser, for itos work done (bug 129); tests list
// and the other checks still count an outline live by its own tag line alone.
//
// # Fingerprints
//
// The Gherkin adapter gives each scenario it lists a fingerprint
// (fingerprint.go, slice 166), which itos-cc dates a kill by in place of the
// scenario's file: the SHA-256, in lowercase hex, of its canonical text
// (Canonical), what the scenario runs as Gherkin parses it
// (github.com/cucumber/gherkin, godog's parser), never its name, a comment,
// a description or another scenario. Its lines, each ending in LF:
//
//	itos scenario fingerprint v1
//	tag <name>                   each effective tag, sorted bytewise, once
//	step <text>                  each step, the Backgrounds' before it first
//	doc <media type>\t<content>  a step's doc string
//	row <cell>\t<cell>…          each row of a step's table
//	examples                     each of an outline's Examples, then
//	tag <name>                   its own tags, sorted, once each,
//	header <cell>\t<cell>…       its header row, when it has one,
//	row <cell>\t<cell>…          and each row of its body
//
// The effective tags are the scenario's own, its rule's and its feature's;
// the Backgrounds are its feature's, then its rule's. A step's keyword is
// left out, since godog matches a step's text alone, and a cell is its value
// as Gherkin reads it (trimmed, \| \n \\ unescaped). Every field is escaped,
// a backslash as \\, a newline as \n, a carriage return as \r and a tab as
// \t, so no field holds a separator and two scenarios that run differently
// never write one text. A change of what the text holds is a new version on
// its first line. A text Gherkin cannot parse gives its scenarios none, and
// a command adapter's tests carry none: tests list --json leaves the key
// out.
//
// # The TSV projection
//
// TSV (tsv.go, slice 154) is tests list --tsv: a listing's live tests as
// Rows, in its order, each a line of its ID, a tab and its file from the
// project's top (the kind's root joined to it, slash separated; a command
// kind with no root lists the project's own), then a tab and its
// fingerprint when it has one, the form itos-cc's mutation.tests.list reads.
// It is pure: the listing comes in, and the folder itos runs in reads an
// absolute file. A live test no row can hold (an empty ID or file, a tab, a
// carriage return, a newline or a NUL in one, an ID another live test has, a
// file outside the project) refuses every row, so a listing is never printed
// in part; a test that is not live is not judged.
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
// and CI runs as a step, and the commit-msg hook runs over the index when a
// commit stages one of the kind's test files (StagesTestFile: on a Gherkin
// kind a .feature file under its root, on a command kind a file its adapter
// lists at the index, or at the commit before for a deletion).
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
// A live scenario turned wip, moved or not, is refused as one changed is
// (bug 108); a wip one may be added, changed or removed.
// On a kind whose adapter is a command the rule compares two adapter
// listings instead (ListedProblems, moves_command.go), each read once per
// tree at the index or a commit, a tree that names no commit (the empty
// tree, an unborn HEAD) listing nothing, so it needs supports_at: true: no
// live test added or lost, none switched between live and wip, and a live
// test's title, when the adapter gives one at both ends, kept unless
// allowed_renames lists the new one; bodies are not compared. On either, a
// test going live (a new live one, or a wip one made live) is Live's to
// judge (slice 134): allowed when the check sets code, the commit's footer
// for the kind names it and the commit changes none of that code, so the
// callers hand each commit's named tests and paths in a Change. config check
// refuses builtin beside a command, on a command kind without supports_at:
// true or a kind whose named adapter is not gherkin, and allowed_renames or
// code without it; code's $path_sets are expanded as commits.scopes' are.
//
// NewMoves(cfg, ws) reads each kind's feature files once per tree and gives the
// callers: Commit(sha, type, change) for verify (the commit against the VCS's Parent, the
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
