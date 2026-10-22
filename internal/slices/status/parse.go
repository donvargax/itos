package status

import (
	"strings"

	argsengine "github.com/donvargax/itos/v8/internal/engine/args"
	"github.com/donvargax/itos/v8/internal/out"
)

// CLI is the status command's line: the Query it asks, and whether it
// prints --json's one object.
type CLI struct {
	Query
	JSON bool
}

// Parse reads the arguments internal/cli's flag spec hands the status
// command, with the global --json: --as and no other argument. Every error
// is a usage error (out.Usage), given before anything is read.
func Parse(args []string, json bool) (CLI, error) {
	pos, flags, _, err := argsengine.Sub("status", "", args, []string{"--as"}, nil)
	if err != nil {
		return CLI{}, err
	}
	if len(pos) > 0 {
		return CLI{}, out.Usage("status takes no arguments: %s", strings.Join(pos, " "))
	}
	return CLI{Query: Query{As: flags["--as"]}, JSON: json}, nil
}
