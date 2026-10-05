// Command itos is the Go build of itos, the task tooling: the ledger and its
// checks, the commit rules, the named tests, the CI plan, the work routing and
// the hooks, behind one command line (itos --help). internal/cli holds the
// command line; internal/launch runs first, and hands the run to the version
// of itos the repository pins when that is not this binary. Started under the
// name git, through the link itos git-shim install makes, it is the git shim
// first (internal/shim): the real git for everything but git commit and git
// push in a repository itos manages, which run as itos's.
package main

import (
	"os"

	"github.com/donvargax/itos/v4/internal/cli"
	"github.com/donvargax/itos/v4/internal/git"
	"github.com/donvargax/itos/v4/internal/launch"
	"github.com/donvargax/itos/v4/internal/shim"
)

func main() {
	args := os.Args[1:]
	if shim.Named(os.Args[0]) {
		var code int
		var done bool
		if args, code, done = shim.Main(args, os.Stderr); done {
			os.Exit(code)
		}
	}
	git.Export()
	if code, launched := launch.Main(args, os.Stderr); launched {
		os.Exit(code)
	}
	os.Exit(cli.Main(args, os.Stdout, os.Stderr))
}
