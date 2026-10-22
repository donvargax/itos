package cli

// A drafted command that mints an item's id (slice 103,
// features/draft.feature): work add --kind slice|task|bug, task add and work
// promote. itos draft add mints its id at once, through the id counter
// (idmint.go), prints it and keeps it in the draft (draft.Draft.Minted), so a
// later draft, a spec's tags and a brief can write it before the item exists.
// itos draft promote then runs the command in this process, not as a child
// itos, through the run Main handed it, with the id in the command's Out
// (reserved), which the command writes instead of minting another: no flag
// or argument hands an id over, so the commands still refuse one a person
// passes. A draft dropped leaves its id a
// gap, as the counter never hands a number out twice.

import (
	"io"
	"os"
	"slices"

	"github.com/donvargax/itos/v8/internal/draft"
)

// mintingLines are the commands that mint an item's id, as their first two
// words.
var mintingLines = [][2]string{{"work", "add"}, {"work", "promote"}, {"task", "add"}}

// mintedKind is the kind of item whose id the itos arguments mint when they
// run, slice, bug or task; "" when they mint none, or when the command refuses
// the line, which mints nothing either and is kept to fail at promote, as
// any drafted line is.
func mintedKind(command []string) string {
	g := ParseGlobals(command)
	if g.Help || len(g.Rest) < 2 || !slices.Contains(mintingLines, [2]string{g.Rest[0], g.Rest[1]}) {
		return ""
	}
	read, err := readLine(command)
	if err != nil {
		return ""
	}
	args := read.Rest[2:]
	switch g.Rest[0] {
	case "work":
		if g.Rest[1] == "promote" {
			p, err := workPromoteLine(args)
			if err != nil {
				return ""
			}
			return p.kind
		}
		if _, kind, _, err := workAddLine(args); err == nil && kind != "idea" {
			return kind
		}
		return ""
	}
	if cfg, err := loadConfig(); err == nil {
		if _, err := taskAddArgs(cfg, args); err == nil {
			return "task"
		}
	}
	return ""
}

// draftMint is the id a drafted command mints, minted now as the command
// would mint it, under its own global flags; "" for one that mints none.
// A registry the command would refuse is reported, its code given.
func draftMint(command []string, o Out) (minted string, code int, err error) {
	err = inPlace(func() error {
		if _, err := applyGlobals(ParseGlobals(command)); err != nil {
			return nil
		}
		kind := mintedKind(command)
		if kind == "" {
			return nil
		}
		cfg, registry, _, release, refused, err := soundRegistry(o)
		defer release()
		if cfg == nil {
			code = refused
			return err
		}
		minted, err = mintItemID(cfg, kind, itemIDs(registry), o.OS)
		return err
	})
	return minted, code, err
}

// runReserved runs a minting draft's command in this process through
// runLine (mainWith, handed in Out's runLine), with the id it reserved, its output
// on w, and gives its exit code; one that leaves this process where it
// cannot be put back is reported on w, as a failure.
func runReserved(d draft.Draft, w io.Writer, runLine func(args []string, stdout, stderr io.Writer, reserved string) int) int {
	code := 0
	if err := inPlace(func() error {
		code = runLine(d.Command, w, w, d.Minted)
		return nil
	}); err != nil {
		return failure(err, Out{Stdout: w, Stderr: w})
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
