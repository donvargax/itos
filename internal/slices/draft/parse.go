package draft

import (
	"strings"

	domain "github.com/donvargax/itos/v8/internal/draft"
	argsengine "github.com/donvargax/itos/v8/internal/engine/args"
	"github.com/donvargax/itos/v8/internal/out"
)

// takes is what draft takes, as a usage error names it.
const takes = "it takes add, edit, promote or drop, else nothing"

// CLI is draft's command line: the Query it asks, and how it answers,
// --json's one object or --quiet's silence on a change.
type CLI struct {
	Query
	JSON, Quiet bool
}

// Parse reads the arguments internal/cli's flag spec hands the draft
// command, origin being the folder itos was started in, which a typed path
// is read from (internal/cli's Out.Origin). Every error is a usage error.
func Parse(args []string, origin string, json, quiet bool) (CLI, error) {
	c := CLI{JSON: json, Quiet: quiet}
	var err error
	sub, rest := argsengine.Split(args)
	switch sub {
	case "add":
		c.Query, err = parseAdd(rest)
	case "edit":
		c.Query, err = parseEdit(rest)
	case "promote":
		c.Query, err = parsePromote(rest)
	case "drop":
		c.Query, err = parseDrop(rest)
	case "":
	default:
		return c, out.Usage("draft has no subcommand %s: %s", sub, takes)
	}
	c.Query.Origin = origin
	return c, err
}

// parseAdd reads `draft add <id> -m <message> <path>…`, a change, or
// `draft add <id> -- <itos args>…`, a command line.
func parseAdd(args []string) (Query, error) {
	var pos, command []string
	message, messages := "", 0
	isCommand := false
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "--":
			isCommand, command = true, args[i+1:]
			i = len(args)
		case arg == "-m" || arg == "--message":
			if i+1 >= len(args) {
				return Query{}, out.Usage("draft add %s needs a value", arg)
			}
			i++
			message, messages = args[i], messages+1
		default:
			pos = append(pos, arg)
		}
	}
	if len(pos) == 0 {
		return Query{}, out.Usage("draft add needs <id> -m <message> <path>…, or <id> -- <itos args>…")
	}
	id, paths := pos[0], pos[1:]
	if !domain.ValidID(id) {
		return Query{}, out.Usage("draft add: %q is no draft id (letters, digits, '.', '_' and '-', a letter or digit first)", id)
	}
	d := domain.Draft{ID: id}
	switch {
	case messages > 1:
		return Query{}, out.Usage("draft add takes one -m <message>")
	case isCommand && (messages > 0 || len(paths) > 0):
		return Query{}, out.Usage("draft add takes -m <message> <path>… or -- <itos args>…, not both")
	case isCommand && len(command) == 0:
		return Query{}, out.Usage("draft add %s -- needs the itos arguments to run", id)
	case isCommand && command[0] == "draft":
		return Query{}, out.Usage("draft add: a draft cannot run itos draft")
	case isCommand:
		d.Command = command
	case strings.TrimSpace(message) == "" || len(paths) == 0:
		return Query{}, out.Usage("draft add %s needs -m <message> and the paths whose change it keeps", id)
	default:
		d.Message = message
	}
	return Query{Sub: "add", Draft: d, Paths: paths}, nil
}

// parseEdit reads `draft edit <id> -m <message> <path>…`.
func parseEdit(args []string) (Query, error) {
	var pos []string
	message, messages := "", 0
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; arg {
		case "-m", "--message":
			if i+1 >= len(args) {
				return Query{}, out.Usage("draft edit %s needs a value", arg)
			}
			i++
			message, messages = args[i], messages+1
		default:
			pos = append(pos, arg)
		}
	}
	if len(pos) == 0 {
		return Query{}, out.Usage("draft edit needs <id> -m <message> <path>…")
	}
	id, typedPaths := pos[0], pos[1:]
	switch {
	case !domain.ValidID(id):
		return Query{}, out.Usage("draft edit: %q is no draft id (letters, digits, '.', '_' and '-', a letter or digit first)", id)
	case messages > 1:
		return Query{}, out.Usage("draft edit takes one -m <message>")
	case strings.TrimSpace(message) == "" || len(typedPaths) == 0:
		return Query{}, out.Usage("draft edit %s needs -m <message> and the paths to edit", id)
	}
	return Query{Sub: "edit", Draft: domain.Draft{ID: id, Message: message}, Paths: typedPaths}, nil
}

// parsePromote reads `draft promote`, which takes nothing.
func parsePromote(args []string) (Query, error) {
	if len(args) > 0 {
		return Query{}, out.Usage("draft promote takes no arguments: %s", strings.Join(args, " "))
	}
	return Query{Sub: "promote"}, nil
}

// parseDrop reads `draft drop <id>`.
func parseDrop(args []string) (Query, error) {
	if len(args) != 1 {
		return Query{}, out.Usage("draft drop needs one <id>")
	}
	id := args[0]
	if !domain.ValidID(id) {
		return Query{}, out.Usage("draft drop: %q is no draft id", id)
	}
	return Query{Sub: "drop", Draft: domain.Draft{ID: id}}, nil
}
