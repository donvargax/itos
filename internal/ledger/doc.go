// Package ledger is the ledger: the YAML files of tasks ledger.files names
// (tasks/phase-{group}.yaml here), where they are (Layout, Files), what is
// wrong with them (Issues, Findings), and the tasks and checks typed
// (Tasks). A group is called by ledger.group.label (phase by default) in
// what the tools print; the rule IDs and the registry's item key phase keep
// their names whatever the label.
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
// staying and git's history keeping them.
package ledger
