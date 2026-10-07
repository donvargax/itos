// Command itos is the task tooling: the ledger and its checks, the commit
// rules, the named tests, the CI plan, the work routing and the hooks, behind
// one command line (itos --help).
//
// A run passes through four packages in this order, each of which may end it:
//
//  1. Started under the name git, through the link itos git-shim install
//     makes, it is the git shim first (internal/shim, shim.Named and
//     shim.Main): the real git for everything but git commit and git push in
//     a repository itos manages, which go on as itos's own git-shim run.
//  2. launch.Args reads a first argument --version as the command version.
//  3. git.Export sets ITOS_GIT to the real git for everything the run
//     starts, the version the launcher runs included, so no git itos starts
//     is ever the shim again.
//  4. internal/launch picks the version of itos to run, and runs it when it
//     is not this binary; only when it hands the run back does internal/cli,
//     the command line, run.
package main
