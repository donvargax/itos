package cli

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/out"
	attemptslice "github.com/donvargax/itos/v8/internal/slices/attempt"
	briefslice "github.com/donvargax/itos/v8/internal/slices/brief"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// runAttempt is `attempt`, the attempt slice (internal/slices/attempt)
// over its ports bound to the real code.
func runAttempt(args []string, o Out) (int, error) {
	c, err := attemptslice.Parse(args, o.Origin, o.JSON)
	if err != nil {
		return 0, err
	}
	return c.Run(context.Background(), o.Stdout, o.Stderr, attemptslice.NewHandler(attemptPorts{o: o, git: vcs.Git{Dir: o.Dir}}))
}

// attemptPorts are the attempt slice's ports over the real code: the
// config every command finds, the brief slice over its own ports for the
// request, and the files of the folder itos runs in and of the git common
// dir.
type attemptPorts struct {
	o   Out
	git vcs.Git
}

// Config is loadConfig.
func (p attemptPorts) Config(context.Context) (*config.Loaded, error) { return loadConfig(p.o.Dir) }

// Request is the request itos brief gives for the item and role at the
// commit base, or the problems it refuses that brief for.
func (p attemptPorts) Request(ctx context.Context, id, role, base string) (string, []out.Problem, error) {
	a, err := briefslice.NewHandler(briefPorts{Git: p.git, o: p.o}).Handle(ctx, briefslice.Query{ID: id, Role: role, At: base})
	if err != nil || len(a.Refused) > 0 {
		return "", a.Refused, err
	}
	return a.Brief.Request, nil, nil
}

// ReadFile reads the path from the folder itos runs in, an error naming
// the path as given.
func (p attemptPorts) ReadFile(_ context.Context, path string) ([]byte, error) {
	data, err := os.ReadFile(within(p.o.Dir, path))
	return data, named(err, path)
}

// journal is where the item's journal is: attempts/<id>.jsonl in itos's
// folder of the git common dir.
func (p attemptPorts) journal(ctx context.Context, id string) (string, error) {
	l, err := p.git.Locate(ctx, "")
	if err != nil {
		return "", kind.Else(kind.Missing, err)
	}
	return filepath.Join(within(p.o.Dir, l.Common), config.StealthFolder, "attempts", id+".jsonl"), nil
}

// Journal is the item's journal's bytes, none when it has none.
func (p attemptPorts) Journal(ctx context.Context, id string) ([]byte, error) {
	path, err := p.journal(ctx, id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

// Record appends the line to the item's journal, making its folder.
func (p attemptPorts) Record(ctx context.Context, id string, line []byte) error {
	path, err := p.journal(ctx, id)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, err = f.Write(line)
	return errors.Join(err, f.Close())
}
