package draft

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"

	domain "github.com/donvargax/itos/v8/internal/draft"
	"github.com/donvargax/itos/v8/internal/engine/ids"
)

// software is the exit code of an error of no kind, sysexits' EX_SOFTWARE
// (itos --help).
const software = 70

// mintingLines are the commands that mint an item's id, as their first two
// words.
var mintingLines = [][2]string{{"work", "add"}, {"work", "promote"}, {"task", "add"}}

// mintedKind is the kind of item whose id the itos arguments mint when they
// run, slice, bug or task; "" when they mint none, or when the command refuses
// the line, which mints nothing either and is kept to fail at promote, as
// any drafted line is.
func (h Handler) mintedKind(ctx context.Context, command []string) string {
	g := h.ports.Globals(ctx, command)
	if g.Help || len(g.Rest) < 2 || !slices.Contains(mintingLines, [2]string{g.Rest[0], g.Rest[1]}) {
		return ""
	}
	read, err := h.ports.ReadLine(ctx, command)
	if err != nil {
		return ""
	}
	args := read[2:]
	switch g.Rest[0] {
	case "work":
		if g.Rest[1] == "promote" {
			kind, err := h.ports.WorkPromoteKind(ctx, args)
			if err != nil {
				return ""
			}
			return kind
		}
		if kind, err := h.ports.WorkAddKind(ctx, args); err == nil && kind != "idea" {
			return kind
		}
		return ""
	}
	if cfg, err := h.ports.Config(ctx); err == nil {
		if err := h.ports.TaskAdd(ctx, cfg, args); err == nil {
			return "task"
		}
	}
	return ""
}

// mint is the id a drafted command mints, minted now as the command would
// mint it, under its own global flags; "" for one that mints none. A
// registry the command would refuse is reported, its code given.
func (h Handler) mint(ctx context.Context, command []string) (minted string, code int, err error) {
	err = inPlace(func() error {
		if err := h.ports.ApplyGlobals(ctx, command); err != nil {
			return nil
		}
		kind := h.mintedKind(ctx, command)
		if kind == "" {
			return nil
		}
		sound, err := h.ports.Sound(ctx)
		defer sound.Release()
		if sound.Cfg == nil {
			code = sound.Code
			return err
		}
		minted, err = h.ports.Mint(ctx, sound.Cfg, kind, ids.Items(sound.Registry))
		return err
	})
	return minted, code, err
}

// runReserved runs a minting draft's command in this process through
// RunLine, with the id it reserved, its output on w, and gives its exit
// code; one that leaves this process where it cannot be put back is
// reported on w as an error of no kind is, as a failure.
func (h Handler) runReserved(ctx context.Context, d domain.Draft, w io.Writer) int {
	var code int
	if err := inPlace(func() error {
		code = h.ports.RunLine(ctx, d.Command, w, d.Minted)
		return nil
	}); err != nil {
		fmt.Fprintf(w, "itos: %s\n", err)
		return software
	}
	return code
}

// inPlace runs f, then puts back what a command's global flags change in
// this process: the folder it runs in, ITOS_CONFIG and NO_COLOR.
func inPlace(f func() error) error {
	here, err := os.Getwd()
	if err != nil {
		return err
	}
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
	if err := os.Chdir(here); err != nil {
		return err
	}
	return ran
}
