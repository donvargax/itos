// Package ledger is the ledger: the YAML files of tasks ledger.files names
// (tasks/phase-{group}.yaml here), where they are (Layout, Files), what is
// wrong with them (Issues, Findings), and the tasks and checks typed
// (Tasks). A group is called by ledger.group.label (phase by default) in
// what the tools print; the rule IDs and the registry's item key phase keep
// their names whatever the label. GroupFlags are the flags that name a
// group on a command line: GroupNames, --group and --phase, and the
// label's own. CheckArg reads one --check or --timeout of a command that
// writes a task's checks, task add or work promote.
//
// Files lists the ledger's files by name, numeric groups by number (phase-2
// before phase-10, slice 130). LayoutProblems are the layouts that read
// nothing or read the registry: a ledger.group.pattern with anchors of its
// own, {group} in ledger.files' folder, a ledger.files that matches
// work.registry; config check reports them, exit 1, before it lists the
// folder. IDPattern is ledger.id's one reader, anchored as ^(?:id)$, any ID
// with no ledger.id.
//
// A check may declare evidence (config.Evidence), which opts it in to
// evidence records: Check.Evidence is its declaration, nil for a check that
// does not, and the schema holds its shape and texts (a mapping of inputs,
// env and tools, each a list of texts, head true or false, an input that is no
// glob and no path outside the repository). A check may say snapshot: false
// (Check.NoSnapshot), which runs it in the working folder wherever it runs:
// config check refuses another value (ledger-check-value) and snapshot: false
// beside evidence (ledger-check-evidence), and a ledger read without it takes
// only the boolean false as the opt-out. Evidenced is whether the evidence
// engine looks the check up.
//
// The schema is strict: duplicate or malformed IDs, unknown types, unknown
// keys, both or neither of run and fails, and a cost: static written below a
// late check of the same task, which written order would run late anyway. A
// check static by a ci.cost.static pattern written there is run late just
// the same, and is a warning. The ledger's folder missing is one config
// error, ledger-folder-missing, exit 2, for every command that reads the
// ledger, and config check's one problem.
//
// Tasks reads every task; one whose checks cannot run (a done_when that is
// not a list, a check without a command) carries its error and fails only
// the command that runs it. IDs gives the task IDs at a tree, for a footer
// read at a commit; NextID the next free one (internal/nextid). Add writes a
// task at the end of its group's file for itos task add: the id judged
// against ledger.id, the type against commits.types, then a ledger without
// problems that lacks the id; the group's file is the one Files gives for it
// (numeric groups compared as numbers), else ledger.files with {group}
// filled in, made new when ledger.group.pattern matches. Its keys are
// written in the ledger's own order (id, type, title, why, done_when; a
// check's run, then timeout), appended with value.Doc's Append to the
// document's top list, its checks a block list below done_when (BlockItem
// writes a file's first task). WithoutChecks is the ledger with a task's
// done_when cut, in place with value.Doc's Drop, for itos work done (slice
// 101): a task's checks gate its close, then leave the ledger, its why
// staying and git's history keeping them. Both read the file they edit
// through the Load they are handed, the tracker store's (internal/cli binds
// it), the rest of the ledger through the config's source.
package ledger
