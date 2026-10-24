// Package task is the task command, a slice (ADR-0053): the ledger's tasks,
// their checks run, listed, the next ID and a task added (features/task.feature,
// features/next-id.feature). Its CLI and Run are the command line, its Query
// and Handler the application.
//
// Command (command.go) declares its name, help and flag spec, by which
// internal/cli reads the line and hands Parse the arguments after task.
// Parse judges task next-id's alone; the rest are read in Handle after the
// config, by the group flags it names (ledger.GroupFlags). Handle reaches
// the world through Ports, which the slice owns and internal/cli binds: the
// config, and for task add the code of work's commands that write the
// registry (internal/cli's workwrite.go and idmint.go).
//
// itos task runs the tasks' checks through one check.Runner per
// invocation, so a check two tasks list runs once, verbose for one task, the
// checks' output on stderr under --json, each check that opted in with an
// evidence declaration looked up by the index tree through the Evidence port,
// a check that also says snapshot: false asking for none and running in the
// working folder (--working-tree runs every check in the working folder and asks for none;
// --json then gives the check its evidence and the run its count of unstaged
// paths); a task with no checks is done when
// its item is done (work done took its checks out, slice 101), review
// otherwise, reading the statuses with work.ItemStatuses. task list runs
// nothing, reading each task's item status the same way, so it works over a
// registry with problems. A group flag's value is read by value.ToNumber
// (--group 01 is group 1), the status table padded by UTF-16 units, and an
// ID on the line is one ledger.IDPattern matches. An after: push check is
// pending until a remote branch holds HEAD. task next-id is ledger.NextID,
// counting the registry's ids (ids.Registry).
//
// task add writes two files in one commit: the task (ledger.Add, its file
// read through the tracker store's Load) and its item, a task, todo and
// nobody's (work.Add), over the registry Sound read sound with its lock
// held, its id the one ItemID gives. Write writes both and commits them
// alone, through the record engine (internal/engine/record), refusing a file
// with changes no commit holds and putting both back when the commit is
// refused. A refusal is a problem, exit 1; a registry that cannot be edited
// in place is a usage error's exit 2. The lock is given back after
// everything is printed.
package task
