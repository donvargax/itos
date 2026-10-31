package attempt

import (
	"path/filepath"
	"strings"

	argsengine "github.com/donvargax/itos/v8/internal/engine/args"
	"github.com/donvargax/itos/v8/internal/out"
)

// CLI is the attempt command's line: the Query it asks, and whether it
// prints --json's one object.
type CLI struct {
	Query
	JSON bool
}

// Query is what attempt submit asks: the item's ID, and the result File as
// the person typed it, Path where itos reads it from.
type Query struct {
	ID, File, Path string
}

// usage is how the line is written, as a usage error names it.
const usage = "itos attempt submit <id> <result.json>"

// Parse reads the arguments internal/cli's flag spec hands the attempt
// command, with the global --json and origin, the folder the person stood
// in (relative to the top itos moved to, "" when it did not move), which
// the result's path is read from. Every error is a usage error
// (out.Usage): attempt takes submit and nothing else, with one id and one
// file.
func Parse(args []string, origin string, json bool) (CLI, error) {
	sub, rest := argsengine.Split(args)
	switch sub {
	case "":
		return CLI{}, out.Usage("attempt needs submit: %s", usage)
	case "submit":
	default:
		return CLI{}, out.Usage("attempt has no subcommand %s: it takes submit, %s", sub, usage)
	}
	pos, _, _, err := argsengine.Sub("attempt", "submit", rest, nil, nil)
	if err != nil {
		return CLI{}, err
	}
	if len(pos) != 2 {
		return CLI{}, out.Usage("attempt submit takes an item's id and a result file, not %q: %s", strings.Join(pos, " "), usage)
	}
	file := pos[1]
	path := file
	if origin != "" && !filepath.IsAbs(file) {
		path = filepath.Join(origin, file)
	}
	return CLI{Query: Query{ID: pos[0], File: file, Path: path}, JSON: json}, nil
}
