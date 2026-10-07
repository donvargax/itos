// Package kind is the kind of an error itos meets, which alone gives the
// exit code it ends with (decision record 0036 in docs/decisions/,
// docs/CLI.md rule 31): a usage or config error exits 2, an environment that
// lacks something 3, a failure that may pass when run again unchanged 75
// (sysexits' EX_TEMPFAIL), and an error of no kind 70 (EX_SOFTWARE), which is
// itos's to fix. A policy failure (a check failed, a commit refused) is no
// error: the command returns 1 itself.
//
// An error takes its kind where what failed is known, wrapped with Wrap (the
// outermost kind wins, so a caller that knows better can reclassify) or Else
// (which keeps a kind already there): release.Get makes a server it cannot
// reach Temporary, and git.RemoteFailure reads a failed fetch's or push's
// kind from git's stderr. The command line reads it with Of, through any
// wrapping added after (cli.ExitCode); an error with no kind is the 70 that
// says nobody classified it.
package kind
