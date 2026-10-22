package gitshim

import (
	"path/filepath"

	argsengine "github.com/donvargax/itos/v8/internal/engine/args"
	"github.com/donvargax/itos/v8/internal/out"
)

// CLI is git-shim's command line: the Query it asks, and whether install
// and uninstall print the one object --json asks for.
type CLI struct {
	Query
	JSON bool
}

// Parse reads the arguments after git-shim, with the folder itos was started
// in, origin, and the global --json: `install [--dir <folder>]`, `uninstall
// [--dir <folder>]` or `run <commit|push> [<git args>…]`, a "--" before
// commit or push dropped. Every error is a usage error (out.Usage).
func Parse(args []string, origin string, json bool) (CLI, error) {
	sub, rest := argsengine.Split(args)
	switch sub {
	case install, uninstall:
		dir := ""
		for i := 0; i < len(rest); i++ {
			if rest[i] != "--dir" {
				return CLI{}, out.Usage("git-shim %s takes only --dir <folder> (%s)", sub, rest[i])
			}
			if i+1 >= len(rest) || rest[i+1] == "" {
				return CLI{}, out.Usage("git-shim %s --dir needs <folder>", sub)
			}
			i++
			dir = typed(origin, rest[i])
		}
		return CLI{Query: Query{Sub: sub, Dir: dir}, JSON: json}, nil
	case run:
		if len(rest) > 0 && rest[0] == "--" {
			rest = rest[1:]
		}
		name, gitArgs := argsengine.Split(rest)
		switch name {
		case commit, push:
			return CLI{Query: Query{Sub: sub, Run: name, Args: gitArgs}, JSON: json}, nil
		}
		return CLI{}, out.Usage("git-shim run needs commit or push, then git's arguments")
	}
	return CLI{}, out.Usage("git-shim needs install, uninstall or run")
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
