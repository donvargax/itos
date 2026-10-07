// Command doc-budget is the docs' byte budget (T-110): each file every session
// or most briefs read is held to a cap in bytes, so the docs cannot grow by
// accretion.
//
// Every byte of AGENTS.md, the guides itos prints (internal/guide) and this
// repository's own notes (docs/ORCHESTRATING.md, docs/ARCHITECTURE.md) is read
// by every session, or by most of the briefs a coordinator hands out, so a
// paragraph added costs every reader from then on. A cut holds only if growth
// costs something: text past a cap means taking text out first, as the
// guide's cap of ten lessons does, and a cap rises only in a commit of its
// own that says why.
//
// The caps are tools/bin/doc-budget/caps.json, beside this program, so that
// raising one is a gate change, build's or ci's alone (commits.path_sets.gates
// holds tools/bin/**):
//
//	{"caps": [{"file": "AGENTS.md", "bytes": 26000}, …]}
//
// Each file is a slash-separated path relative to the directory it runs in,
// the repository's top in CI; bytes is its cap, the file's size on disk. A
// file the caps name that is gone fails the check as one over its cap does, so
// a renamed doc cannot slip its budget: move its cap with it.
//
// It imports nothing but the standard library, as the other gate programs do,
// and reads the files alone, with no git and no network: CI runs it in the
// prose plan too, since a docs-only push is where the docs grow.
//
//	go run ./tools/bin/doc-budget [-caps <file>]
//
// Exit status: 0 every file within its cap, 1 a file over its cap or gone,
// each named with its size and cap, 2 a caps file it cannot read.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const defaultCaps = "tools/bin/doc-budget/caps.json"

// caps is the caps file.
type caps struct {
	Caps []limit `json:"caps"`
}

// limit is one file's cap.
type limit struct {
	File  string `json:"file"`
	Bytes int64  `json:"bytes"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("doc-budget", flag.ContinueOnError)
	flags.SetOutput(stderr)
	capsFile := flags.String("caps", defaultCaps, "the caps file: {\"caps\": [{\"file\": …, \"bytes\": …}]}")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "doc-budget: unexpected argument %q\n", flags.Arg(0))
		return 2
	}

	limits, err := readCaps(*capsFile)
	if err != nil {
		fmt.Fprintf(stderr, "doc-budget: cannot read the caps in %s: %v\n", *capsFile, err)
		return 2
	}

	var over []string
	for _, l := range limits {
		info, err := os.Stat(filepath.FromSlash(l.File))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			over = append(over, fmt.Sprintf("%s is gone, but %s caps it at %d bytes: move its cap with it, or remove the cap",
				l.File, *capsFile, l.Bytes))
		case err != nil:
			fmt.Fprintf(stderr, "doc-budget: cannot read %s: %v\n", l.File, err)
			return 2
		case info.IsDir():
			fmt.Fprintf(stderr, "doc-budget: %s, which %s caps, is a folder, not a file\n", l.File, *capsFile)
			return 2
		case info.Size() > l.Bytes:
			over = append(over, fmt.Sprintf("%s is %d bytes, over its cap of %d by %d",
				l.File, info.Size(), l.Bytes, info.Size()-l.Bytes))
		default:
			fmt.Fprintf(stdout, "doc-budget: %s is %d bytes, within its cap of %d\n", l.File, info.Size(), l.Bytes)
		}
	}
	if len(over) == 0 {
		return 0
	}
	for _, line := range over {
		fmt.Fprintf(stderr, "doc-budget: %s\n", line)
	}
	fmt.Fprintf(stderr, "\ndoc-budget: every session or most briefs read these files, so each byte costs every reader.\n"+
		"  Take text out first, as the cap of ten lessons does: cut what a reader no longer needs, or move\n"+
		"  it where only its readers look (a package's doc comment, a decision record, a task's why).\n"+
		"  A cap rises only in a ci commit of its own to %s that says why.\n", *capsFile)
	return 1
}

// readCaps is the caps file's limits, refused when the file holds anything
// but a list of distinct relative paths, each with a positive cap.
func readCaps(name string) ([]limit, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var c caps
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("not the caps' JSON: %v", err)
	}
	if dec.More() {
		return nil, errors.New("not the caps' JSON: more than one value")
	}
	if len(c.Caps) == 0 {
		return nil, errors.New("it caps no file")
	}
	seen := map[string]bool{}
	for i, l := range c.Caps {
		switch {
		case l.File == "":
			return nil, fmt.Errorf("cap %d names no file", i+1)
		case path.IsAbs(l.File) || filepath.IsAbs(l.File) || path.Clean(l.File) != l.File ||
			l.File == ".." || strings.HasPrefix(l.File, "../"):
			return nil, fmt.Errorf("cap %d's file, %q, is not a clean slash-separated path inside the repository", i+1, l.File)
		case l.Bytes <= 0:
			return nil, fmt.Errorf("%s's cap is %d bytes; a cap is a positive number of bytes", l.File, l.Bytes)
		case seen[l.File]:
			return nil, fmt.Errorf("%s is capped twice", l.File)
		}
		seen[l.File] = true
	}
	return c.Caps, nil
}
