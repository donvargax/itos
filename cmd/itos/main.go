// Command itos is the Go build of itos, the task tooling: the ledger and its
// checks, the commit rules, the named tests, the CI plan, the work routing and
// the hooks, behind one command line (PLAN.md §7). internal/cli holds the
// command line; internal/launch runs first, and hands the run to the version
// of itos the repository pins when that is not this binary.
package main

import (
	"os"

	"github.com/donvargax/itos/v2/internal/cli"
	"github.com/donvargax/itos/v2/internal/launch"
)

func main() {
	if code, launched := launch.Main(os.Args[1:], os.Stderr); launched {
		os.Exit(code)
	}
	os.Exit(cli.Main(os.Args[1:], os.Stdout, os.Stderr))
}
