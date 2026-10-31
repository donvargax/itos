package next

import (
	"strings"

	argsengine "github.com/donvargax/itos/v8/internal/engine/args"
	"github.com/donvargax/itos/v8/internal/out"
)

// CLI is the next command's line: the Query it asks, and whether it prints
// --json's one object.
type CLI struct {
	Query
	JSON bool
}

// Query is what the next command asks: the ID of the item.
type Query struct {
	ID string
}

// Parse reads the arguments internal/cli's flag spec hands the next
// command, with the global --json: one <id> and nothing else. Every error
// is a usage error (out.Usage), given before anything is read.
func Parse(args []string, json bool) (CLI, error) {
	pos, _, _, err := argsengine.Sub("next", "", args, nil, nil)
	if err != nil {
		return CLI{}, err
	}
	switch len(pos) {
	case 0:
		return CLI{}, out.Usage("next needs one <id>")
	case 1:
		return CLI{Query: Query{ID: pos[0]}, JSON: json}, nil
	}
	return CLI{}, out.Usage("next takes one <id>: %s", strings.Join(pos, " "))
}
