// Command conformance runs itos's conformance corpus (tools/itos/conformance/*.yaml) through a
// binary: what itos did when each case was written, run through its command line in scratch
// repositories. CI's corpus step and the nightly run it against tools/bin/itos; the format of a
// case, and how one is run and judged, is its package's doc (tools/bin/conformance/corpus).
//
//	go run ./tools/bin/conformance --bin <command> [--git <path>] [--additive] [--json] [--actual] [--case <regexp>] [--only <file>…]
//
// --bin is the command to run: a path if it holds a slash, else a name on the PATH. --only runs
// the fixture files given, else every one of tools/itos/conformance, found above the working
// folder (the nearest folder holding go.mod). --git names native Git by an absolute path, else
// it is the first git on the PATH that is not an itos. --additive judges each case by what its
// output must still hold, as previous-release judges the last release's corpus. --case runs only
// the cases whose name the regular expression matches.
//
// --actual judges nothing: it writes to stdout, as a YAML list, what each case's run did (its
// exit code, stdout, stderr, stdout's JSON less the keys json: does not judge, the message and
// fix it left out, to read, and the files its files_after names), the case's folder, commits and
// the binary's version written back as their placeholders, in the corpus's style, so a case's
// expectations can be written from it (corpus.WriteActuals). It exits
// 0, or 1 when a case could not run.
//
// Each case that fails is printed to stderr, with its command and what differs, and a line on
// stdout says how many pass of each file's, and how long they took. --json writes instead a JSON
// line a case to stdout, {"name": "<file>.yaml: <case>", "file", "case", "verdict": "pass" or
// "fail", "problems"}, the line saying how many pass going to stderr, for a program to read.
// Exit status: 0 every case passed, 1 one failed, 2 a usage error, a fixture that cannot be read
// or a runner that cannot be readied.
//
// It was tools/itos/conformance/run.ts until T-144 (the user's call, 2026-10-10: this repository
// drops Node); it lives with the other programs CI's steps run, under tools/bin, as a gate, which
// only build and ci commits may change, while the corpus it runs stays the test commits'.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	itosgit "github.com/donvargax/itos/v8/internal/git"
	"github.com/donvargax/itos/v8/tools/bin/conformance/corpus"
)

const usage = "usage: go run ./tools/bin/conformance --bin <command> [--git <path>] [--additive] [--json] [--actual] [--case <regexp>] [--only <file>…]"

func main() {
	itosgit.Platform = runtime.GOOS
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// options are the command line's.
type options struct {
	bin, git               string
	additive, json, actual bool
	cases                  *regexp.Regexp
	only                   []string
}

// parse reads the command line as run.ts did: --only takes every argument up to the next flag.
func parse(args []string) (options, error) {
	var o options
	listing := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--bin" || a == "--git" || a == "--case":
			if i+1 >= len(args) {
				return o, fmt.Errorf("%s needs a value", a)
			}
			i++
			switch a {
			case "--bin":
				o.bin = args[i]
			case "--git":
				o.git = args[i]
			default:
				re, err := regexp.Compile(args[i])
				if err != nil {
					return o, fmt.Errorf("--case: %v", err)
				}
				o.cases = re
			}
			listing = false
		case a == "--additive":
			o.additive, listing = true, false
		case a == "--json":
			o.json, listing = true, false
		case a == "--actual":
			o.actual, listing = true, false
		case a == "--only":
			listing = true
		case listing && !strings.HasPrefix(a, "--"):
			o.only = append(o.only, a)
		default:
			return o, fmt.Errorf("unexpected argument %q", a)
		}
	}
	if o.bin == "" {
		return o, errors.New("--bin is required")
	}
	return o, nil
}

func run(args []string, stdout, stderr io.Writer) int {
	o, err := parse(args)
	if err != nil {
		fmt.Fprintf(stderr, "conformance: %v\n%s\n", err, usage)
		return 2
	}
	started := time.Now()
	fixtures, runner, err := ready(o)
	if err != nil {
		fmt.Fprintf(stderr, "FAIL conformance runner setup: %v\n", err)
		return 2
	}
	defer runner.Close()
	if o.actual {
		actuals := runner.Actuals(fixtures)
		if err := corpus.WriteActuals(stdout, actuals); err != nil {
			fmt.Fprintf(stderr, "conformance: %v\n", err)
			return 2
		}
		for _, a := range actuals {
			if a.Err != nil {
				return 1
			}
		}
		return 0
	}
	results := runner.Run(fixtures)
	corpus.Failures(stderr, results)
	summary := "\n" + corpus.Summary(fixtures, results, time.Since(started)) + "\n"
	if o.json {
		if err := corpus.Verdicts(stdout, results); err != nil {
			fmt.Fprintf(stderr, "conformance: %v\n", err)
			return 2
		}
		fmt.Fprint(stderr, summary)
	} else {
		fmt.Fprint(stdout, summary)
	}
	for _, r := range results {
		if len(r.Problems) > 0 {
			return 1
		}
	}
	return 0
}

// ready reads the fixtures and readies the runner: the PATH, native Git and the binary's
// version.
func ready(o options) ([]*corpus.Fixture, *corpus.Runner, error) {
	paths := o.only
	if len(paths) == 0 {
		top, err := top()
		if err != nil {
			return nil, nil, err
		}
		if paths, err = corpus.Files(filepath.Join(top, "tools", "itos", "conformance")); err != nil {
			return nil, nil, err
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, nil, err
	}
	var fixtures []*corpus.Fixture
	for _, path := range paths {
		f, err := corpus.ReadFixture(path)
		if err != nil {
			return nil, nil, err
		}
		if o.cases != nil {
			f.Cases = slices.DeleteFunc(f.Cases, func(c *corpus.Case) bool { return !o.cases.MatchString(c.Name) })
		}
		f.Shown = filepath.Base(path)
		if abs, err := filepath.Abs(path); err == nil {
			if rel, err := filepath.Rel(cwd, abs); err == nil && rel != "" {
				f.Shown = filepath.ToSlash(rel)
			}
		}
		fixtures = append(fixtures, f)
	}
	runner, err := corpus.New(corpus.Options{Bin: o.bin, Git: o.git, Additive: o.additive})
	if err != nil {
		return nil, nil, err
	}
	if err := runner.LearnVersion(); err != nil {
		runner.Close()
		return nil, nil, err
	}
	return fixtures, runner, nil
}

// top is the repository's top folder: the nearest folder above the working one that holds
// go.mod.
func top() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no go.mod above the working folder, so no tools/itos/conformance to run")
		}
		dir = parent
	}
}
