package draft

import (
	"context"
	"io"
	"os"
	"slices"

	domain "github.com/donvargax/itos/v8/internal/draft"
	"github.com/donvargax/itos/v8/internal/engine/ids"
)

// mintingLines are the commands that mint an item's id, as their first two
// words.
var mintingLines = [][2]string{{"work", "add"}, {"work", "promote"}, {"task", "add"}}

// mintedKind is the kind of item whose id the itos arguments mint when they
// run, read through m, slice, bug or task; "" when they mint none, or when
// the command refuses the line, which mints nothing either and is kept to
// fail at promote, as any drafted line is.
func mintedKind(ctx context.Context, m Minting, command []string) string {
	g := m.Globals(ctx, command)
	if g.Help || len(g.Rest) < 2 || !slices.Contains(mintingLines, [2]string{g.Rest[0], g.Rest[1]}) {
		return ""
	}
	read, err := m.ReadLine(ctx, command)
	if err != nil {
		return ""
	}
	args := read[2:]
	switch g.Rest[0] {
	case "work":
		if g.Rest[1] == "promote" {
			kind, err := m.WorkPromoteKind(ctx, args)
			if err != nil {
				return ""
			}
			return kind
		}
		if kind, err := m.WorkAddKind(ctx, args); err == nil && kind != "idea" {
			return kind
		}
		return ""
	}
	if cfg, err := m.Config(ctx); err == nil {
		if err := m.TaskAdd(ctx, cfg, args); err == nil {
			return "task"
		}
	}
	return ""
}

// mint is the id a drafted command mints, minted now as the command would
// mint it, under its own global flags, in the folder they name; "" for one
// that mints none. A registry the command would refuse is reported, its
// code given.
func (h Handler) mint(ctx context.Context, command []string) (minted string, code int, err error) {
	err = inPlace(func() error {
		m, err := h.ports.ApplyGlobals(ctx, command)
		if err != nil {
			return nil
		}
		kind := mintedKind(ctx, m, command)
		if kind == "" {
			return nil
		}
		sound, err := m.Sound(ctx)
		defer sound.Release()
		if sound.Cfg == nil {
			code = sound.Code
			return err
		}
		minted, err = m.Mint(ctx, sound.Cfg, kind, ids.Items(sound.Registry))
		return err
	})
	return minted, code, err
}

// runReserved runs a minting draft's command in this process through
// RunLine, with the id it reserved, its output on w, and gives its exit
// code.
func (h Handler) runReserved(ctx context.Context, d domain.Draft, w io.Writer) int {
	var code int
	_ = inPlace(func() error {
		code = h.ports.RunLine(ctx, d.Command, w, d.Minted)
		return nil
	})
	return code
}

// inPlace runs f, then puts back what a command's global flags change in
// this process's environment: ITOS_CONFIG and NO_COLOR. The folder they
// move to is the run's, never the process's.
func inPlace(f func() error) error {
	env := map[string]*string{}
	for _, name := range []string{"ITOS_CONFIG", "NO_COLOR"} {
		if value, ok := os.LookupEnv(name); ok {
			env[name] = &value
		} else {
			env[name] = nil
		}
	}
	ran := f()
	for name, value := range env {
		if value == nil {
			os.Unsetenv(name)
		} else {
			os.Setenv(name, *value)
		}
	}
	return ran
}
