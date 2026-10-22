package task

import (
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	argsengine "github.com/donvargax/itos/v8/internal/engine/args"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/out"
)

// CLI is task's command line: the Query it asks, and how it answers,
// --json's one object or, for task add, --quiet's silence.
type CLI struct {
	Query
	JSON, Quiet bool
}

// Parse reads the arguments internal/cli's flag spec hands the task
// command, with the global --json and --quiet. Only task next-id's are
// judged here, a word being a usage error before the config is read; the
// others are read in Handle, by the group flags the config names.
func Parse(args []string, json, quiet bool) (CLI, error) {
	c := CLI{Query: Query{Args: args, ToStderr: json}, JSON: json, Quiet: quiet}
	switch sub, rest := argsengine.Split(args); sub {
	case "next-id":
		if words := argsengine.Positional(rest); len(words) > 0 {
			return CLI{}, out.Usage("task next-id takes no arguments")
		}
		fallthrough
	case "list", "add":
		c.Sub, c.Args = sub, rest
	}
	return c, nil
}

// addUsage is what task add needs, as a usage error says it.
const addUsage = "task add needs --group <g>, --type <type>, --title <title>, --why <why> and a --check <command>"

// AddArgs reads task add's arguments: --group (or --phase, or
// --<ledger.group.label>), --type, --title and --why, each once; --check,
// once per check, in order, and --timeout after a --check, that check's
// seconds. "--flag value" or "--flag=value". Anything else is a usage
// error.
func AddArgs(cfg *config.Loaded, args []string) (ledger.NewTask, error) {
	var n ledger.NewTask
	var ids []string
	once := map[string]*string{"--type": &n.Type, "--title": &n.Title, "--why": &n.Why}
	for _, f := range ledger.GroupFlags(cfg) {
		once[f] = &n.Group
	}
	seen := map[*string]bool{}
	timed := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			ids = append(ids, arg)
			continue
		}
		name, v, joined := strings.Cut(arg, "=")
		field, single := once[name]
		if !single && name != "--check" && name != "--timeout" {
			return n, out.Usage("task add does not take %s", name)
		}
		if !joined {
			if i+1 >= len(args) {
				return n, out.Usage("task add %s needs a value", name)
			}
			i++
			v = args[i]
		}
		switch {
		case single:
			if seen[field] || strings.TrimSpace(v) == "" {
				return n, out.Usage("task add takes one %s with a value", name)
			}
			seen[field], *field = true, v
		default:
			if err := ledger.CheckArg("task add", &n.Checks, &timed, name, v); err != nil {
				return n, err
			}
		}
	}
	if len(ids) > 0 {
		return n, out.Usage("task add mints the task id; do not pass one")
	}
	if n.Group == "" || n.Type == "" || n.Title == "" || n.Why == "" || len(n.Checks) == 0 {
		return n, out.Usage(addUsage)
	}
	return n, nil
}
