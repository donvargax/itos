// Command itos is the Go build of itos, the task tooling: the ledger and its
// checks, the commit rules, the named tests, the CI plan, the work routing and
// the hooks, behind one command line (PLAN.md §7). It is ported from the
// TypeScript in tools/itos/ one command group at a time; internal/cli holds
// the command line.
package main

import (
	"os"

	"github.com/donvargax/itos/v2/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:], os.Stdout, os.Stderr))
}
