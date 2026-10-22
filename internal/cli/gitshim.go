package cli

import (
	"context"
	"os"

	"github.com/donvargax/itos/v8/internal/shim"
	gitshimslice "github.com/donvargax/itos/v8/internal/slices/gitshim"
)

// runGitShim is `git-shim install [--dir <folder>]`, `git-shim uninstall [--dir
// <folder>]` and `git-shim run <commit|push> [<git args>…]`, the git-shim
// slice (internal/slices/gitshim) over its ports bound to the real code.
func runGitShim(args []string, o Out) (int, error) {
	c, err := gitshimslice.Parse(args, o.Origin, o.JSON)
	if err != nil {
		return 0, err
	}
	return c.Run(context.Background(), o.Stdout, o.Stderr, gitshimslice.NewHandler(gitShimPorts{o}))
}

// gitShimPorts are the git-shim slice's ports over internal/shim on the OS
// itos runs on, and itos commit and itos push with o.
type gitShimPorts struct{ o Out }

// Place is shimPlace.
func (p gitShimPorts) Place(_ context.Context, dir string) (self, link string, err error) {
	return shimPlace(dir, p.o.OS)
}

// LinksItos is linksItos.
func (gitShimPorts) LinksItos(_ context.Context, link string) (self, other bool) {
	return linksItos(link)
}

// MakeLink is makeLink.
func (p gitShimPorts) MakeLink(_ context.Context, self, link string, replace bool) error {
	return makeLink(self, link, replace, p.o.OS)
}

// Standing is pathStanding.
func (p gitShimPorts) Standing(_ context.Context, dir string) (at int, realGit string, realAt int) {
	return pathStanding(dir, p.o.OS)
}

// Commit is gitCommit.
func (p gitShimPorts) Commit(_ context.Context, args []string) (int, error) {
	return gitCommit(args, p.o)
}

// Push is runPush.
func (p gitShimPorts) Push(_ context.Context, args []string) (int, error) { return runPush(args, p.o) }

// shimPlace is shim.Place, of os.Executable.
func shimPlace(dir, goos string) (self, link string, err error) {
	return shim.Place(os.Executable, dir, goos)
}

// linksItos is shim.LinksItos.
func linksItos(link string) (self, other bool) { return shim.LinksItos(link) }

// makeLink is shim.MakeLink.
func makeLink(self, link string, replace bool, goos string) error {
	return shim.MakeLink(self, link, replace, goos)
}

// pathStanding is shim.PathStanding.
func pathStanding(dir, goos string) (at int, realGit string, realAt int) {
	return shim.PathStanding(dir, goos)
}

// standingLine is shim.StandingLine.
func standingLine(dir string, at int, realGit string, realAt int) string {
	return shim.StandingLine(dir, at, realGit, realAt)
}
