package cli

import (
	"context"
	"io"
	"os"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/shell"
	testsslice "github.com/donvargax/itos/v8/internal/slices/tests"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// runTests is `tests`, the tests slice (internal/slices/tests) over its
// ports bound to the real code; smoke check's --features is read from
// where itos was started, before a run from a subfolder moved to the top.
func runTests(args []string, o Out) (int, error) {
	c, err := testsslice.Parse(args, o.Origin, o.JSON, o.Quiet)
	if err != nil {
		return 0, err
	}
	p := testsPorts{Git: vcs.Git{Dir: o.Dir}, OS: o.OS, Stdout: o.Stdout, Stderr: o.Stderr}
	return c.Run(context.Background(), o.Stdout, o.Stderr, testsslice.NewHandler(p, o.Tests))
}

// testsPorts are the tests slice's ports over the real code: git, the
// config the working tree has, the tracker on the OS goos names and the
// config's shell, with itos's streams.
type testsPorts struct {
	vcs.Git
	OS             string
	Stdout, Stderr io.Writer
}

// Config is loadConfig.
func (p testsPorts) Config(context.Context) (*config.Loaded, error) { return loadConfig(p.Dir) }

// Tracker is tracker.
func (p testsPorts) Tracker(_ context.Context, cfg *config.Loaded) trackerport.Tracker {
	return tracker(cfg, p.OS)
}

// Shell is shell.Run with itos's streams.
func (p testsPorts) Shell(_ context.Context, cfg *config.Loaded, command string) shell.Result {
	return shell.Run(cfg, command, shell.Options{Stdin: os.Stdin, Stdout: p.Stdout, Stderr: p.Stderr})
}
