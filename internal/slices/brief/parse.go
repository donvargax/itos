package brief

import (
	"strings"

	argsengine "github.com/donvargax/itos/v8/internal/engine/args"
	"github.com/donvargax/itos/v8/internal/out"
)

// CLI is the brief command's line: the Query it asks, and whether it
// prints --json's one object.
type CLI struct {
	Query
	JSON bool
}

// Query is what brief asks: the item's ID, the Role briefed, and the
// revision At which it is computed, HEAD when "".
type Query struct {
	ID, Role, At string
}

// usage is how the line is written, as a usage error names it.
const usage = "itos brief <id> --role drafter|implementer|reviewer [--at <rev>]"

// Parse reads the arguments internal/cli's flag spec hands the brief
// command, with the global --json: one item's id and --role. Every error
// is a usage error (out.Usage), given before anything is read; whether the
// role is one itos knows, and --at a revision git resolves, is the
// handler's to refuse.
func Parse(args []string, json bool) (CLI, error) {
	role, hasRole := argsengine.FlagValue(args, "--role")
	at, _ := argsengine.FlagValue(args, "--at")
	ids := argsengine.Positional(args, "--role", "--at")
	switch {
	case len(ids) == 0:
		return CLI{}, out.Usage("brief needs an item's id: %s", usage)
	case len(ids) > 1:
		return CLI{}, out.Usage("brief takes one id, not %s: %s", strings.Join(ids, " "), usage)
	case !hasRole:
		return CLI{}, out.Usage("brief needs --role: %s", usage)
	}
	return CLI{Query: Query{ID: ids[0], Role: role, At: at}, JSON: json}, nil
}
