// Package cli is itos's one command line, tools/itos/main.ts ported: the
// global flags wherever they stand, the command table with every command's
// argument errors, and how a failure is reported and which exit code it
// takes (PLAN.md §7): 0 success, 1 a policy failure, 2 a usage or config
// error, 3 a missing environment.
//
// The Go port landed one command group at a time (PLAN.md, phase 2), and
// every command of the table is ported now. Each takes its arguments as the
// TypeScript does, so a usage error reads the same in both. A command added
// to the TypeScript alone cannot land: every push runs the whole corpus and
// every feature against the Go build (tools/selftest/go-port.ts).
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/donvargax/itos/internal/config"
	"github.com/donvargax/itos/internal/out"
	"github.com/donvargax/itos/internal/tests"
)

// Globals are the global flags, read out of the arguments up to a "--".
type Globals struct {
	JSON, Quiet, Help, NoColor bool
	// Config and Root are "" when not given.
	Config, Root string
	// Rest is the command and its arguments.
	Rest []string
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
// stand up to a "--", after which every argument is the command's.
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

// Exit codes (PLAN.md §7).
const (
	ExitPolicy  = 1
	ExitUsage   = 2
	ExitMissing = 3
)

// failure reports an error as main.ts does and gives its exit code: a usage
// error naming the help, a config error as the config check prints it,
// anything else as its message alone.
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
	return ExitUsage
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
// --config and --root hold for everything a command loads.
func applyGlobals(g Globals) error {
	if g.Config != "" {
		if err := os.Setenv("ITOS_CONFIG", g.Config); err != nil {
			return err
		}
	}
	if g.Root != "" {
		if err := os.Chdir(g.Root); err != nil {
			return fmt.Errorf("--root %s: %w", g.Root, err)
		}
	}
	if g.NoColor {
		return os.Setenv("NO_COLOR", "1")
	}
	return nil
}

// Main runs itos with the arguments after its name and gives the exit code.
func Main(args []string, stdout, stderr io.Writer) int {
	g := ParseGlobals(args)
	o := Out{JSON: g.JSON, Quiet: g.Quiet, Stdout: stdout, Stderr: stderr}
	tests.Warnings = stderr
	if err := applyGlobals(g); err != nil {
		return failure(err, o)
	}
	name, rest := "", []string(nil)
	if len(g.Rest) > 0 {
		name, rest = g.Rest[0], g.Rest[1:]
	}
	// `itos`, `itos help …` and any --help print the help, before any config.
	if g.Help || name == "" || name == "help" {
		return help(g, o)
	}
	command, ok := commands[name]
	if !ok {
		return failure(usage("unknown command: %s", name), o)
	}
	code, err := command(rest, o)
	if err != nil {
		return failure(err, o)
	}
	return code
}
