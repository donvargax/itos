package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/tests"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// Globals are the global flags, read out of the arguments up to a "--".
type Globals struct {
	JSON, Quiet, Help, NoColor bool
	// Config and Root are "" when not given.
	Config, Root string
	// Rest is the command and its arguments.
	Rest []string
	// Extension is the program the command runs when it names an extension
	// (Parse), "" otherwise.
	Extension string
}

var switches = map[string]func(*Globals){
	"--json":     func(g *Globals) { g.JSON = true },
	"-q":         func(g *Globals) { g.Quiet = true },
	"--quiet":    func(g *Globals) { g.Quiet = true },
	"-h":         func(g *Globals) { g.Help = true },
	"--help":     func(g *Globals) { g.Help = true },
	"--no-color": func(g *Globals) { g.NoColor = true },
}

// ParseGlobals takes the global flags out of the arguments, wherever they
// stand up to a "--", after which every argument is the command's. It reads
// them as they come, judging nothing: a built-in command's line is judged by
// its spec (readLine) before the command runs.
func ParseGlobals(args []string) Globals {
	var g Globals
	head, tail := args, []string(nil)
	for i, a := range args {
		if a == "--" {
			head, tail = args[:i], args[i:]
			break
		}
	}
	for i := 0; i < len(head); i++ {
		arg := head[i]
		switch {
		case strings.HasPrefix(arg, "--config=") || strings.HasPrefix(arg, "--root="):
			name, value, _ := strings.Cut(arg, "=")
			if name == "--config" {
				g.Config = value
			} else {
				g.Root = value
			}
		case arg == "--config" || arg == "--root":
			value := ""
			if i+1 < len(head) {
				i++
				value = head[i]
			}
			if arg == "--config" {
				g.Config = value
			} else {
				g.Root = value
			}
		case switches[arg] != nil:
			switches[arg](&g)
		default:
			g.Rest = append(g.Rest, arg)
		}
	}
	g.Rest = append(g.Rest, tail...)
	return g
}

// Out is how a command reports: its streams, the global flags that shape
// what it prints, Tests, where the run's test listings warn, and Origin, the
// folder the person stood in, relative to the repository's top, when the
// run moved there from a subfolder to read its config (config.Top); "" when
// it runs where it was started, or where --root says.
type Out struct {
	JSON, Quiet    bool
	Stdout, Stderr io.Writer
	Tests          *tests.Warnings
	Origin         string
	// Dir is the folder the command runs in, absolute: the run's, or where
	// --root or the move to the repository's top took it (applyGlobals).
	// Every path a command reads relative is read from it.
	Dir string
	// OS is the OS itos runs on, as GOOS names it, which Main's caller
	// detects: what differs by OS takes it from here.
	OS string
	// reserved is the item id a draft reserved for the command itos draft
	// promote runs in this process (the draft slice), which the command writes
	// instead of minting one (newItemID); "" otherwise. Only promote sets
	// it, for the one command it runs: no command line can, so a person
	// passing an id is still refused.
	reserved string
	// runLine is mainWith, which Main hands each command for draft promote
	// to run a minting draft's line through, started in the folder dir:
	// mainWith reaches every command, so no command may name it.
	runLine func(dir string, args []string, stdout, stderr io.Writer, reserved string) int
}

// Run is one run of itos: Args, the arguments after its name; the streams
// it writes to; OS, the OS it runs on, as GOOS names it (Out's OS); and
// Dir, the folder it starts in, absolute, which its caller reads once. The
// run never reads or changes the process's working folder: what --root and
// the move to the repository's top change is the run's folder (Out's Dir).
type Run struct {
	Args           []string
	Stdout, Stderr io.Writer
	OS             string
	Dir            string
}

// usageError is a command called wrongly, internal/cli's or a slice's alike.
type usageError = out.UsageError

func usage(format string, a ...any) error {
	return out.Usage(format, a...)
}

// Exit codes, as itos --help lists them. 70 and 75 are sysexits'
// EX_SOFTWARE and EX_TEMPFAIL (decision 35).
const (
	ExitPolicy    = 1
	ExitUsage     = 2
	ExitMissing   = 3
	ExitSoftware  = 70
	ExitTemporary = 75
)

// ExitCode is the exit code of an error's kind (internal/kind), never a
// default: a usage error and a config error 2, a missing environment 3, a
// failure that may pass when run again 75, and an error of no kind 70.
func ExitCode(err error) int {
	var u usageError
	var c *config.Error
	if errors.As(err, &u) || errors.As(err, &c) {
		return ExitUsage
	}
	switch kind.Of(err) {
	case kind.Usage:
		return ExitUsage
	case kind.Missing:
		return ExitMissing
	case kind.Temporary:
		return ExitTemporary
	}
	return ExitSoftware
}

// failure reports an error as main.ts does and gives its exit code, by its
// kind (ExitCode): a usage error naming the help, a config error as the
// config check prints it, anything else as its message alone.
func failure(err error, o Out) int {
	var u usageError
	var c *config.Error
	switch {
	case errors.As(err, &u):
		fmt.Fprintf(o.Stderr, "itos: %s (itos --help)\n", u.Error())
		return ExitUsage
	case errors.As(err, &c):
		return configFailure(c, o)
	}
	fmt.Fprintf(o.Stderr, "itos: %s\n", err)
	return ExitCode(err)
}

func configFailure(c *config.Error, o Out) int {
	if o.JSON {
		problems := make([]out.Problem, len(c.Problems))
		for i, p := range c.Problems {
			p.Message = c.File + ": " + p.Message
			problems[i] = p
		}
		_ = out.Emit(o.Stdout,
			out.Field{Key: "config", Value: c.File},
			out.Field{Key: "valid", Value: false},
			out.Field{Key: "problems", Value: problems})
	} else {
		for _, p := range c.Problems {
			fmt.Fprintf(o.Stderr, "FAIL %s: %s\n", c.File, p.Message)
		}
	}
	return ExitUsage
}

// applyGlobals sets what the global flags set before any command runs, so
// --config and --root hold for everything a command loads, and gives the
// folder the command runs in, from dir, the one the run started in: --root,
// joined to dir when relative, a usage error unless it is a folder; with
// neither, and no ITOS_CONFIG, from a subfolder of a repository whose config
// is at its top, the top, as --root <top> would (slice 40), and where the
// person stood (Out.Origin); else dir. On the OS goos, a folder entered is
// spelled as the OS gives it there (entered).
func applyGlobals(g Globals, dir, goos string) (at, origin string, err error) {
	if g.Config != "" {
		if err := os.Setenv("ITOS_CONFIG", g.Config); err != nil {
			return "", "", err
		}
	}
	at = dir
	if g.Root != "" {
		root := within(dir, g.Root)
		info, err := os.Stat(root)
		if err == nil && !info.IsDir() {
			err = fmt.Errorf("%s is not a folder", root)
		}
		if err != nil {
			return "", "", kind.Wrap(kind.Usage, fmt.Errorf("--root %s: %w", g.Root, err))
		}
		at = entered(goos, dir, root)
	} else if top := config.Top(vcs.Git{Dir: dir}, ""); top != "" {
		at, origin = moveTo(goos, dir, top)
	}
	if g.NoColor {
		return at, origin, os.Setenv("NO_COLOR", "1")
	}
	return at, origin, nil
}

// within is the path p names from the folder dir, "" for the process's.
func within(dir, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(dir, p)
}

// named is err naming the file p as it was given, not as it was read from
// the folder a command runs in.
func named(err error, p string) error {
	var failed *fs.PathError
	if errors.As(err, &failed) {
		failed.Path = p
	}
	return err
}

// entered is the folder to, entered from the folder from, spelled as the OS
// goos would give it as the folder a process stands in once it moved there:
// from itself when to is the same folder, as a process that stays where it
// was started keeps its spelling; else, but on windows, which keeps the
// path as it was entered, with its links resolved, as unix gives a folder
// moved to (a macOS temporary folder is /private/var's, not /var's).
func entered(goos, from, to string) string {
	if x, err := os.Stat(from); err == nil {
		if y, err := os.Stat(to); err == nil && os.SameFile(x, y) {
			return from
		}
	}
	if goos == "windows" {
		return to
	}
	if real, err := filepath.EvalSymlinks(to); err == nil {
		return real
	}
	return to
}

// moveTo is the repository's top as the folder itos runs in, entered from
// dir, the folder the person stood in (entered), and where they stood,
// relative to it: "" at the top.
func moveTo(goos, dir, top string) (at, origin string) {
	at = entered(goos, dir, top)
	here, there := dir, at
	if real, err := filepath.EvalSymlinks(here); err == nil {
		here = real
	}
	if real, err := filepath.EvalSymlinks(there); err == nil {
		there = real
	}
	rel, err := filepath.Rel(there, here)
	if err != nil || rel == "." {
		return at, ""
	}
	return at, rel
}

// typed is a path the person typed, as itos reads it at the top it moved to
// from origin, the subfolder they stood in (Out.Origin): relative to it, as
// git reads a path typed there. An absolute path, "-" (stdin) and every path
// of a run that did not move stay as typed.
func typed(origin, p string) string {
	if origin == "" || p == "" || p == "-" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(origin, p)
}

// Main runs itos as r says and gives the exit code.
func Main(r Run) int {
	return mainWith(r, "")
}

// mainWith is Main with the item id a draft reserved for the command (Out's
// reserved), "" for none.
func mainWith(r Run, reserved string) int {
	g := Parse(r.Args)
	o := Out{JSON: g.JSON, Quiet: g.Quiet, Stdout: r.Stdout, Stderr: r.Stderr, Tests: tests.NewWarnings(r.Stderr), OS: r.OS,
		reserved: reserved, runLine: func(dir string, args []string, stdout, stderr io.Writer, reserved string) int {
			return mainWith(Run{Args: args, Stdout: stdout, Stderr: stderr, OS: r.OS, Dir: dir}, reserved)
		}}
	dir, origin, err := applyGlobals(g, r.Dir, r.OS)
	if err != nil {
		return failure(err, o)
	}
	o.Dir, o.Origin = dir, origin
	name, rest := "", []string(nil)
	if len(g.Rest) > 0 {
		name, rest = g.Rest[0], g.Rest[1:]
	}
	// An extension takes its arguments as they are; a --help before its
	// name asks for its help.
	if g.Extension != "" {
		if g.Help {
			rest = []string{"--help"}
		}
		return runExtension(g, g.Extension, rest, o)
	}
	// A name v6.0.0 renamed, run or asked about, names the new one.
	words := g.Rest
	if name == "help" {
		words = rest
	}
	if err := renamedCommand(words); err != nil {
		return failure(err, o)
	}
	// `itos`, `itos help …` and any --help print the help, before any config.
	if g.Help || name == "" || name == "help" {
		return help(g, o)
	}
	command, ok := dispatch(name)
	if !ok {
		return failure(usage("unknown command: %s", name), o)
	}
	read, err := readLine(dir, r.Args)
	if err != nil {
		return failure(err, o)
	}
	code, err := command(read.Rest[1:], o)
	if err != nil {
		return failure(err, o)
	}
	return code
}
