package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/donvargax/itos/v6/internal/config"
	"github.com/donvargax/itos/v6/internal/kind"
	"github.com/donvargax/itos/v6/internal/out"
	"github.com/donvargax/itos/v6/internal/tests"
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

// Out is how a command reports: its streams, and the global flags that shape
// what it prints.
type Out struct {
	JSON, Quiet    bool
	Stdout, Stderr io.Writer
}

// usageError is a command called wrongly: exit 2, naming the help.
type usageError struct{ message string }

func (e usageError) Error() string { return e.message }

func usage(format string, a ...any) error {
	return usageError{strings.TrimSpace(fmt.Sprintf(format, a...))}
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

// uneditable is a change to the work registry that cannot be made in place,
// its text not as itos edits it: a data file itos refuses, exit 2.
func uneditable(registry string, err error) error {
	return kind.Wrap(kind.Usage, fmt.Errorf("%s cannot be edited in place: %w", registry, err))
}

// failure reports an error as main.ts does and gives its exit code, by its
// kind (ExitCode): a usage error naming the help, a config error as the
// config check prints it, anything else as its message alone.
func failure(err error, o Out) int {
	var u usageError
	var c *config.Error
	switch {
	case errors.As(err, &u):
		fmt.Fprintf(o.Stderr, "itos: %s (itos --help)\n", u.message)
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

// origin is the folder the person stood in, relative to the repository's
// top, when a run from a subfolder moved there to read its config
// (config.Top); "" when it runs where it was started, or where --root says.
var origin string

// applyGlobals sets what the global flags set before any command runs, so
// --config and --root hold for everything a command loads. With neither, and
// no ITOS_CONFIG, a run from a subfolder of a repository whose config is at
// its top moves there first, as --root <top> would (slice 40).
func applyGlobals(g Globals) error {
	origin = ""
	if g.Config != "" {
		if err := os.Setenv("ITOS_CONFIG", g.Config); err != nil {
			return err
		}
	}
	if g.Root != "" {
		if err := os.Chdir(g.Root); err != nil {
			return kind.Wrap(kind.Usage, fmt.Errorf("--root %s: %w", g.Root, err))
		}
	} else if top := config.Top(""); top != "" {
		if err := moveTo(top); err != nil {
			return err
		}
	}
	if g.NoColor {
		return os.Setenv("NO_COLOR", "1")
	}
	return nil
}

// moveTo makes the repository's top the folder itos runs in, and keeps
// where the person stood in origin.
func moveTo(top string) error {
	here, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := os.Chdir(top); err != nil {
		return fmt.Errorf("%s: %w", top, err)
	}
	if top, err = os.Getwd(); err != nil {
		return err
	}
	if real, err := filepath.EvalSymlinks(here); err == nil {
		here = real
	}
	if real, err := filepath.EvalSymlinks(top); err == nil {
		top = real
	}
	rel, err := filepath.Rel(top, here)
	if err != nil || rel == "." {
		return nil
	}
	origin = rel
	return nil
}

// typed is a path the person typed, as itos reads it at the top it moved to
// from a subfolder: relative to the folder they stood in, as git reads a
// path typed there. An absolute path, "-" (stdin) and every path of a run
// that did not move stay as typed.
func typed(p string) string {
	if origin == "" || p == "" || p == "-" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(origin, p)
}

// Main runs itos with the arguments after its name and gives the exit code.
func Main(args []string, stdout, stderr io.Writer) int {
	g := Parse(args)
	o := Out{JSON: g.JSON, Quiet: g.Quiet, Stdout: stdout, Stderr: stderr}
	tests.Warnings = stderr
	if err := applyGlobals(g); err != nil {
		return failure(err, o)
	}
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
	command, ok := commands[name]
	if !ok {
		return failure(usage("unknown command: %s", name), o)
	}
	read, err := readLine(args)
	if err != nil {
		return failure(err, o)
	}
	code, err := command(read.Rest[1:], o)
	if err != nil {
		return failure(err, o)
	}
	return code
}
