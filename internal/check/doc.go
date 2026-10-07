// Package check runs a task's checks and gives each its cost class, so a
// check means one thing wherever it runs: itos task, CI's plan and driver,
// and the commit-msg hook.
//
// A Runner is one invocation's runs, keyed by Key: the command, its
// whitespace collapsed (config.Normal), and its timeout. Its Run prints the
// verbose command line and runs each distinct check once, every later task
// that lists it reading the kept exit status by its own run: or fails:. The
// runs live for the invocation alone; the nightly shares one Runner across
// its done tasks, while a push runs each named task's checks as its own.
//
// CostOf, CostedChecks and ChecksBeforeLate are the cost classes and the
// written order: a check is static by its own cost: static, else by a
// pattern of ci.cost.static, else late, and a task's checks keep their
// written order, so the commit-msg hook runs a task's checks only up to its
// first late one. (config check's written-order rule stays in internal/ledger,
// since it reads the ledger before it is typed.)
//
// RunCaptured is how the commit-msg hook runs a check: quiet, stdout and
// stderr in one temporary file rather than a pipe, so a command it leaves
// running cannot hold the commit past its timeout, which
// hooks.commit_msg.check_timeout caps (Captured.Capped says the cap set it);
// a timeout is a failure whatever the code, and the check runs in the hook's
// environment less GIT_INDEX_FILE. An after: push check is Pending until a
// remote branch holds HEAD (Pushed).
package check
