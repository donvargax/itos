package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/draft"
	draftslice "github.com/donvargax/itos/v8/internal/slices/draft"
	private "github.com/donvargax/itos/v8/internal/store/private/adapter"
	privateport "github.com/donvargax/itos/v8/internal/store/private/port"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// runDraft is `draft`, the draft slice (internal/slices/draft) over its
// ports bound to the real code: git, the private store's lock, the hooks,
// this itos's binary, and for a line that mints an item's id the code of
// work add, work promote and task add.
func runDraft(args []string, o Out) (int, error) {
	c, err := draftslice.Parse(args, o.Origin, o.JSON, o.Quiet)
	if err != nil {
		return 0, err
	}
	return c.Run(context.Background(), o.Stdout, o.Stderr, draftslice.NewHandler(draftPorts{o: o}))
}

// draftPorts are the draft slice's ports over the real code, reporting as o
// says: the VCS port and git's patches are the git adapter's.
type draftPorts struct {
	vcs.Git
	o Out
}

// Hold is the private store's Hold.
func (p draftPorts) Hold(ctx context.Context, name string) (privateport.Held, error) {
	return private.Store{OS: p.o.OS, VCS: vcs.Git{}}.Hold(ctx, name)
}

// TreeEntries is the git adapter's.
func (p draftPorts) TreeEntries(ctx context.Context, commit string, paths []string) (map[string]draftslice.TreeEntry, error) {
	listed, err := p.Git.TreeEntries(ctx, commit, paths)
	if err != nil {
		return nil, err
	}
	entries := make(map[string]draftslice.TreeEntry, len(listed))
	for path, e := range listed {
		entries[path] = draftslice.TreeEntry{Mode: e.Mode, Kind: e.Kind, Object: e.Object}
	}
	return entries, nil
}

// Scratch is the git adapter's.
func (p draftPorts) Scratch(_ context.Context, index string) draftslice.Index {
	return p.Git.Scratch(index)
}

// HooksReady is hooksReady.
func (draftPorts) HooksReady(_ context.Context, event string) error { return hooksReady(event) }

// Self is executable.
func (draftPorts) Self(context.Context) (string, error) { return executable() }

// Exec runs the binary self with the arguments, never through a shell, its
// output on w, and gives its exit code.
func (draftPorts) Exec(_ context.Context, self string, args []string, w io.Writer) (int, error) {
	cmd := exec.Command(self, args...)
	cmd.Stdout, cmd.Stderr = w, w
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode(), nil
		}
		return 0, err
	}
	return 0, nil
}

// RunLine is the run Main handed the command (Out's runLine).
func (p draftPorts) RunLine(_ context.Context, args []string, w io.Writer, reserved string) int {
	return p.o.runLine(args, w, w, reserved)
}

// Getwd is os.Getwd.
func (draftPorts) Getwd(context.Context) (string, error) { return os.Getwd() }

// CreateTemp is os.CreateTemp.
func (draftPorts) CreateTemp(_ context.Context, dir, pattern string) (*os.File, error) {
	return os.CreateTemp(dir, pattern)
}

// Save is draft.Save.
func (draftPorts) Save(_ context.Context, file string, drafts draft.File) error {
	return draft.Save(file, drafts)
}

// Globals is ParseGlobals.
func (draftPorts) Globals(_ context.Context, command []string) draftslice.Line {
	g := ParseGlobals(command)
	return draftslice.Line{Help: g.Help, Rest: g.Rest}
}

// ApplyGlobals is applyGlobals of the line's global flags.
func (draftPorts) ApplyGlobals(_ context.Context, command []string) error {
	_, err := applyGlobals(ParseGlobals(command))
	return err
}

// ReadLine is readLine's Rest.
func (draftPorts) ReadLine(_ context.Context, command []string) ([]string, error) {
	read, err := readLine(command)
	return read.Rest, err
}

// WorkAddKind is workAddLine's kind.
func (draftPorts) WorkAddKind(_ context.Context, args []string) (string, error) {
	_, kind, _, err := workAddLine(args)
	return kind, err
}

// WorkPromoteKind is workPromoteLine's kind.
func (draftPorts) WorkPromoteKind(_ context.Context, args []string) (string, error) {
	p, err := workPromoteLine(args)
	return p.kind, err
}

// TaskAdd is taskAddArgs.
func (draftPorts) TaskAdd(_ context.Context, cfg *config.Loaded, args []string) error {
	_, err := taskAddArgs(cfg, args)
	return err
}

// Config is loadConfig.
func (draftPorts) Config(context.Context) (*config.Loaded, error) { return loadConfig() }

// Sound is soundRegistry.
func (p draftPorts) Sound(context.Context) (draftslice.Sound, error) {
	cfg, registry, _, release, code, err := soundRegistry(p.o)
	return draftslice.Sound{Cfg: cfg, Registry: registry, Release: release, Code: code}, err
}

// Mint is mintItemID.
func (p draftPorts) Mint(_ context.Context, cfg *config.Loaded, kind string, registryIDs []string) (string, error) {
	return mintItemID(cfg, kind, registryIDs, p.o.OS)
}

// executable is this itos's binary, which runs a command draft: os.Executable,
// another in a test.
var executable = os.Executable

// draftsFile is draft.Path over git.
func draftsFile() string {
	return draft.Path(context.Background(), vcs.Git{})
}

// wordsOf is what git said of a failure on stderr, when err is a command
// that failed, else err's own words.
func wordsOf(err error) string {
	var failed *vcsport.Failed
	if errors.As(err, &failed) {
		return strings.TrimSpace(failed.Words)
	}
	return err.Error()
}
