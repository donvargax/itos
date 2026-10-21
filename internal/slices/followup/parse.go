package followup

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/slices/followup/domain"
)

// takes is what followup takes, as a usage error names it.
const takes = "it takes add, note, close, show or doc, else --all"

// CLI is followup's command line: the Query it asks, and how it answers,
// --json's one object or --quiet's silence on a change.
type CLI struct {
	Query
	JSON, Quiet bool
}

// Parse reads the arguments internal/cli's flag spec hands the followup
// command, origin being the folder itos was started in, which a relative
// doc path is read from. Every error is a usage error, which the command
// line reports as one.
func Parse(args []string, origin string, json, quiet bool) (CLI, error) {
	c := CLI{JSON: json, Quiet: quiet}
	sub, rest := "", []string(nil)
	if len(args) > 0 {
		sub, rest = args[0], args[1:]
	}
	var err error
	switch sub {
	case "add":
		c.Query, err = parseAdd(rest)
	case "note":
		c.Query, err = parseNote(rest)
	case "close":
		c.Query, err = parseClose(rest)
	case "show":
		c.Query, err = parseShow(rest)
	case "doc":
		c.Query, err = parseDoc(rest, origin)
	default:
		var pos []string
		var set map[string]bool
		pos, _, set, err = flags("", args, nil, []string{"--all"})
		if err == nil && len(pos) > 0 {
			err = fmt.Errorf("followup has no subcommand %s: %s", pos[0], takes)
		}
		c.Query = Query{All: set["--all"]}
	}
	return c, err
}

func parseAdd(args []string) (Query, error) {
	pos, values, _, err := flags("add", args, []string{"--with", "--title", "--note"}, nil)
	if err != nil {
		return Query{}, err
	}
	id, rest, err := threadID("add", pos, "<id> --with <who> --title <title> --note <text>")
	if err != nil {
		return Query{}, err
	}
	if len(rest) > 0 {
		return Query{}, fmt.Errorf("followup add takes one <id>, not %s", strings.Join(rest, " "))
	}
	if values["--with"] == "" || values["--title"] == "" || values["--note"] == "" {
		return Query{}, fmt.Errorf("followup add needs --with <who>, --title <title> and --note <text>")
	}
	return Query{Sub: "add", ID: id, With: values["--with"], Title: values["--title"], Note: values["--note"]}, nil
}

func parseNote(args []string) (Query, error) {
	pos, _, _, err := flags("note", args, nil, nil)
	if err != nil {
		return Query{}, err
	}
	id, rest, err := threadID("note", pos, "<id> <text>")
	if err != nil {
		return Query{}, err
	}
	text := strings.Join(rest, " ")
	if strings.TrimSpace(text) == "" {
		return Query{}, fmt.Errorf("followup note needs <id> <text>")
	}
	return Query{Sub: "note", ID: id, Note: text}, nil
}

func parseClose(args []string) (Query, error) {
	pos, values, _, err := flags("close", args, []string{"--note"}, nil)
	if err != nil {
		return Query{}, err
	}
	id, rest, err := threadID("close", pos, "<id>")
	if err != nil {
		return Query{}, err
	}
	if len(rest) > 0 {
		return Query{}, fmt.Errorf("followup close takes one <id>, not %s (a last note goes in --note)", strings.Join(rest, " "))
	}
	return Query{Sub: "close", ID: id, Note: values["--note"]}, nil
}

func parseShow(args []string) (Query, error) {
	pos, _, _, err := flags("show", args, nil, nil)
	if err != nil {
		return Query{}, err
	}
	id, rest, err := threadID("show", pos, "<id>")
	if err != nil {
		return Query{}, err
	}
	if len(rest) > 0 {
		return Query{}, fmt.Errorf("followup show takes one <id>, not %s", strings.Join(rest, " "))
	}
	return Query{Sub: "show", ID: id}, nil
}

func parseDoc(args []string, origin string) (Query, error) {
	pos, _, set, err := flags("doc", args, nil, []string{"--force"})
	if err != nil {
		return Query{}, err
	}
	id, rest, err := threadID("doc", pos, "<id> <path>")
	if err != nil {
		return Query{}, err
	}
	if len(rest) != 1 || rest[0] == "" {
		return Query{}, fmt.Errorf("followup doc needs <id> <path>")
	}
	return Query{Sub: "doc", ID: id, Path: rest[0], Typed: typed(origin, rest[0]), Force: set["--force"]}, nil
}

// typed is p as the person typed it from origin: joined to origin when
// relative, "-" and an absolute path as they are.
func typed(origin, p string) string {
	if origin == "" || p == "-" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(origin, p)
}

// threadID is the one id a subcommand names first, checked; the arguments
// after it are the rest.
func threadID(sub string, pos []string, what string) (string, []string, error) {
	if len(pos) == 0 {
		return "", nil, fmt.Errorf("followup %s needs %s", sub, what)
	}
	if !domain.ValidID(pos[0]) {
		return "", nil, fmt.Errorf("followup %s: %q is no thread id (letters, digits, '.', '_' and '-', a letter or digit first)", sub, pos[0])
	}
	return pos[0], pos[1:], nil
}

// flags reads a subcommand's arguments: its positional ones, the valued
// flags' values and the switches given. A flag it does not take, a valued
// flag with no value or an empty one, and a flag given twice are errors;
// after "--" every argument is positional.
func flags(sub string, args []string, valued, switches []string) ([]string, map[string]string, map[string]bool, error) {
	name := strings.TrimSpace("followup " + sub)
	var pos []string
	values := map[string]string{}
	set := map[string]bool{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			pos = append(pos, arg)
			continue
		}
		flag, value, joined := strings.Cut(arg, "=")
		switch {
		case slices.Contains(switches, flag) && !joined:
			if set[flag] {
				return nil, nil, nil, fmt.Errorf("%s takes one %s", name, flag)
			}
			set[flag] = true
		case slices.Contains(valued, flag):
			if !joined {
				if i+1 >= len(args) {
					return nil, nil, nil, fmt.Errorf("%s %s needs a value", name, flag)
				}
				i++
				value = args[i]
			}
			if _, twice := values[flag]; twice || strings.TrimSpace(value) == "" {
				return nil, nil, nil, fmt.Errorf("%s takes one %s with a value", name, flag)
			}
			values[flag] = value
		default:
			return nil, nil, nil, fmt.Errorf("%s does not take %s", name, flag)
		}
	}
	return pos, values, set, nil
}
