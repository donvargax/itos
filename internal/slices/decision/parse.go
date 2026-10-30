package decision

import (
	"strconv"
	"strings"

	"github.com/donvargax/itos/v8/internal/adr"
	"github.com/donvargax/itos/v8/internal/ask"
	argsengine "github.com/donvargax/itos/v8/internal/engine/args"
	"github.com/donvargax/itos/v8/internal/out"
)

// askTakes is what ask takes, as a usage error names it.
const askTakes = "it takes add, answer, ratify, overturn, record or show, else --all"

// CLI is the decision command's line: the Query it asks, and whether it
// prints --json's one object, or under --quiet no line for a change.
type CLI struct {
	Query
	JSON, Quiet bool
}

// Parse reads the arguments internal/cli's flag spec hands the decision
// command, with the global flags: the list and its --all, or add, answer,
// ratify, overturn, record or show and theirs. Every error is a usage error
// (out.Usage), given before anything is read.
func Parse(args []string, json, quiet bool) (CLI, error) {
	c := CLI{JSON: json, Quiet: quiet}
	sub, rest := argsengine.Split(args)
	var err error
	switch sub {
	case "add":
		c.Query, err = parseAdd(rest)
	case "answer":
		c.Query, err = parseAnswer(rest)
	case "ratify", "show":
		c.Query, err = parseOne(sub, rest)
	case "overturn":
		c.Query, err = parseOverturn(rest)
	case "record":
		c.Query, err = parseRecord(rest)
	default:
		c.Query, err = parseList(args)
	}
	return c, err
}

// parseList is `decision [--all]`.
func parseList(args []string) (Query, error) {
	pos, _, set, err := argsengine.Sub("decision", "", args, nil, []string{"--all"})
	if err != nil {
		return Query{}, err
	}
	if len(pos) > 0 {
		return Query{}, out.Usage("decision has no subcommand %s: %s", pos[0], askTakes)
	}
	return Query{All: set["--all"]}, nil
}

// parseAdd is `decision add <text>… [--item <id>] [--class <class>]`.
func parseAdd(args []string) (Query, error) {
	pos, flags, _, err := argsengine.Sub("decision", "add", args, []string{"--item", "--class"}, nil)
	if err != nil {
		return Query{}, err
	}
	text := strings.TrimSpace(strings.Join(pos, " "))
	if text == "" {
		return Query{}, out.Usage("decision add needs <text>, the question")
	}
	return Query{Sub: "add", Text: text, Item: flags["--item"], Class: flags["--class"]}, nil
}

// parseAnswer is `decision answer <id> <text>… [--provisional --by
// <handle>]`: the two flags go together, so an answer never names who gave
// it without being provisional, nor is provisional without naming who.
func parseAnswer(args []string) (Query, error) {
	pos, flags, set, err := argsengine.Sub("decision", "answer", args, []string{"--by"}, []string{"--provisional"})
	if err != nil {
		return Query{}, err
	}
	id, rest, err := askID("answer", pos, "<id> <text>")
	if err != nil {
		return Query{}, err
	}
	text := strings.TrimSpace(strings.Join(rest, " "))
	if text == "" {
		return Query{}, out.Usage("decision answer needs <id> <text>, the answer")
	}
	by, provisional := flags["--by"], set["--provisional"]
	switch {
	case provisional && by == "":
		return Query{}, out.Usage("decision answer --provisional needs --by <handle>, who answers in the person's place")
	case by != "" && !provisional:
		return Query{}, out.Usage("decision answer --by needs --provisional: an answer by someone else is provisional until the person ratifies it")
	}
	return Query{Sub: "answer", ID: id, Text: text, Provisional: provisional, By: by}, nil
}

// parseOne is `decision <sub> <id>`, ratify's and show's.
func parseOne(sub string, args []string) (Query, error) {
	pos, _, _, err := argsengine.Sub("decision", sub, args, nil, nil)
	if err != nil {
		return Query{}, err
	}
	id, rest, err := askID(sub, pos, "<id>")
	if err != nil {
		return Query{}, err
	}
	if len(rest) > 0 {
		return Query{}, out.Usage("decision %s takes one <id>, not %s", sub, strings.Join(rest, " "))
	}
	return Query{Sub: sub, ID: id}, nil
}

// parseOverturn is `decision overturn <id> <text>…`.
func parseOverturn(args []string) (Query, error) {
	pos, _, _, err := argsengine.Sub("decision", "overturn", args, nil, nil)
	if err != nil {
		return Query{}, err
	}
	id, rest, err := askID("overturn", pos, "<id> <text>")
	if err != nil {
		return Query{}, err
	}
	text := strings.TrimSpace(strings.Join(rest, " "))
	if text == "" {
		return Query{}, out.Usage("decision overturn needs <id> <text>, the person's answer")
	}
	return Query{Sub: "overturn", ID: id, Text: text}, nil
}

// parseRecord is `decision record <id> --title <title> [--option <text>]…
// [--consequences <text>] [--supersedes <n>]`, or `decision record <id>
// --none`.
func parseRecord(args []string) (Query, error) {
	options, args, err := repeated("decision record", "--option", args)
	if err != nil {
		return Query{}, err
	}
	pos, flags, set, err := argsengine.Sub("decision", "record", args, []string{"--title", "--consequences", "--supersedes"}, []string{"--none"})
	if err != nil {
		return Query{}, err
	}
	id, rest, err := askID("record", pos, "<id>")
	if err != nil {
		return Query{}, err
	}
	if len(rest) > 0 {
		return Query{}, out.Usage("decision record takes one <id>, not %s", strings.Join(rest, " "))
	}
	title, consequences := oneLine(flags["--title"]), strings.TrimSpace(flags["--consequences"])
	supersedes := 0
	if text, given := flags["--supersedes"]; given {
		if supersedes, err = strconv.Atoi(text); err != nil || supersedes < 1 {
			return Query{}, out.Usage("decision record --supersedes takes a decision record's number, not %q", text)
		}
	}
	none := set["--none"]
	switch {
	case none && (title != "" || consequences != "" || supersedes != 0 || len(options) > 0):
		return Query{}, out.Usage("decision record --none writes no record, so it takes no --title, --option, --consequences or --supersedes")
	case !none && title == "":
		return Query{}, out.Usage("decision record needs --title <title>, the decision's, or --none")
	case !none && adr.Slug(title) == "":
		return Query{}, out.Usage("decision record --title needs a letter or a digit, which the record's file is named by")
	}
	return Query{Sub: "record", ID: id, Title: title, Options: options, Consequences: consequences, Supersedes: supersedes, None: none}, nil
}

// askID is the question id a subcommand names first, checked; the arguments
// after it are the rest.
func askID(sub string, pos []string, what string) (string, []string, error) {
	if len(pos) == 0 {
		return "", nil, out.Usage("decision %s needs %s", sub, what)
	}
	if !ask.ValidID(pos[0]) {
		return "", nil, out.Usage("decision %s: %q is no question id (q-<n>)", sub, pos[0])
	}
	return pos[0], pos[1:], nil
}

// repeated takes every occurrence of the flag out of args, as `--flag
// <value>` or `--flag=<value>`, before the arguments' "--": the values in
// order and the arguments left. A flag with no value, or an empty one, is a
// usage error naming the command.
func repeated(command, flag string, args []string) (values, rest []string, err error) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			rest = append(rest, args[i:]...)
			break
		}
		name, value, joined := strings.Cut(arg, "=")
		if name != flag {
			rest = append(rest, arg)
			continue
		}
		if !joined {
			if i+1 >= len(args) {
				return nil, nil, out.Usage("%s %s needs a value", command, flag)
			}
			i++
			value = args[i]
		}
		if value = oneLine(value); value == "" {
			return nil, nil, out.Usage("%s %s needs a value", command, flag)
		}
		values = append(values, value)
	}
	return values, rest, nil
}

// oneLine is a text on one line, its whitespace each a space, as a list and
// a commit's body give it.
func oneLine(text string) string { return strings.Join(strings.Fields(text), " ") }
