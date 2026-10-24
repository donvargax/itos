package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// Event is the hook event the guard answers.
const Event = "PreToolUse"

// Input is what the guard reads of Claude Code's PreToolUse input; every
// other key is ignored.
type Input struct {
	Tool string
	// Command is the Bash tool's command; "" for any other tool.
	Command string
	// Folder is the session's folder, "" when the input names none.
	Folder string
}

// Read reads the input: one JSON object, with tool_name a string and, for
// the Bash tool, tool_input.command a string. Anything else cannot be read.
func Read(r io.Reader) (Input, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return Input{}, err
	}
	var fields struct {
		Tool  *string         `json:"tool_name"`
		Input json.RawMessage `json:"tool_input"`
		Cwd   string          `json:"cwd"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := dec.Decode(&fields); err != nil {
		return Input{}, fmt.Errorf("not a JSON object: %w", err)
	}
	if dec.More() {
		return Input{}, errors.New("more than one JSON value")
	}
	if fields.Tool == nil {
		return Input{}, errors.New("no tool_name")
	}
	in := Input{Tool: *fields.Tool, Folder: fields.Cwd}
	if in.Tool != "Bash" {
		return in, nil
	}
	var bash struct {
		Command *string `json:"command"`
	}
	if err := json.Unmarshal(fields.Input, &bash); err != nil || bash.Command == nil {
		return Input{}, errors.New("the Bash tool's input has no command")
	}
	in.Command = *bash.Command
	return in, nil
}

// Guarded are the git subcommands the guard denies, each with the reason it
// gives, in the order a reason names them.
var Guarded = []struct{ Name, Reason string }{
	{"commit", "git commit is itos commit in this repository: run itos commit --task <id>, or itos commit " +
		"--scenarios <ids>, with the same arguments, so the commit carries the footers it needs " +
		"(itos commit --help)."},
	{"push", "git push is itos push in this repository: run itos push, with no arguments; it pulls the " +
		"upstream with a rebase, then pushes HEAD to it, never forced (itos push --help)."},
}

// Reason is the deny's reason for the input, run from the folder here (the
// one a relative input folder and -C are read from, and the folder judged
// when the input names none), and whether there is one: a Bash command
// running a guarded git subcommand, in a folder managed says itos manages.
func Reason(in Input, here string, managed func(dir string) bool) (string, bool) {
	if in.Tool != "Bash" {
		return "", false
	}
	folder := here
	if in.Folder != "" {
		folder = in.Folder
		if !filepath.IsAbs(folder) {
			folder = filepath.Join(here, folder)
		}
	}
	calls, err := GitCalls(in.Command)
	if err != nil {
		return "", false
	}
	denied := map[string]bool{}
	for _, c := range calls {
		dir := c.Dir
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(folder, dir)
		}
		if !denied[c.Sub] && managed(dir) {
			denied[c.Sub] = true
		}
	}
	var reasons []string
	for _, g := range Guarded {
		if denied[g.Name] {
			reasons = append(reasons, g.Reason)
		}
	}
	return strings.Join(reasons, " "), len(reasons) > 0
}

// Deny writes Claude Code's answer denying the tool call, with the reason.
func Deny(w io.Writer, reason string) error {
	type specific struct {
		Event    string `json:"hookEventName"`
		Decision string `json:"permissionDecision"`
		Reason   string `json:"permissionDecisionReason"`
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(struct {
		Output specific `json:"hookSpecificOutput"`
	}{specific{Event, "deny", reason}})
}

// GitCall is one git command the shell command runs: the folder its -C
// options name (relative to the one it runs in, "." for that one) and its
// subcommand.
type GitCall struct {
	Dir, Sub string
}

// GitCalls are the git commands the shell command runs with a subcommand
// written out, in the order bash meets them; an error when bash cannot parse
// it.
func GitCalls(command string) ([]GitCall, error) {
	file, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(command), "")
	if err != nil {
		return nil, err
	}
	var calls []GitCall
	syntax.Walk(file, func(n syntax.Node) bool {
		if c, ok := n.(*syntax.CallExpr); ok {
			if call, ok := gitCall(c.Args); ok {
				calls = append(calls, call)
			}
		}
		return true
	})
	return calls, nil
}

// precommands run the command after them: the guard reads past them to it.
var precommands = map[string]bool{"command": true, "exec": true, "nohup": true, "env": true}

// valued are git's global options that take the next word as their value
// when it is not written after an =.
var valued = map[string]bool{
	"-C": true, "-c": true, "--git-dir": true, "--work-tree": true, "--namespace": true,
	"--super-prefix": true, "--config-env": true, "--attr-source": true,
}

// gitCall reads one simple command's words (its assignments already apart)
// as git with its global options and a subcommand.
func gitCall(args []*syntax.Word) (GitCall, bool) {
	words := make([]string, 0, len(args))
	for _, a := range args {
		w, ok := literal(a)
		if !ok {
			// A word the shell builds when it runs (a variable, a command
			// substitution) ends what can be read; the commands it holds are
			// walked on their own.
			break
		}
		words = append(words, w)
	}
	i, ok := commandIndex(words)
	if !ok {
		return GitCall{}, false
	}
	if !isGit(words[i]) {
		return GitCall{}, false
	}
	call := GitCall{Dir: "."}
	for i++; i < len(words); i++ {
		w := words[i]
		switch {
		case w == "-C":
			if i+1 >= len(words) {
				return GitCall{}, false
			}
			i++
			if d := words[i]; d != "" {
				if filepath.IsAbs(d) {
					call.Dir = d
				} else {
					call.Dir = filepath.Join(call.Dir, d)
				}
			}
		case valued[w]:
			i++
		case strings.HasPrefix(w, "-"):
		default:
			call.Sub = w
			return call, true
		}
	}
	return GitCall{}, false
}

// isGit is whether a command's name runs git: git, or a path to it.
func isGit(name string) bool {
	return name == "git" || strings.HasSuffix(name, "/git")
}

// commandIndex is where the command the precommands run starts in words: past
// command, exec, nohup and env with what env skips. ok is false when there is
// no such command or env's words cannot be read.
func commandIndex(words []string) (int, bool) {
	for i := 0; i < len(words); {
		switch w := words[i]; {
		case w == "env":
			var ok bool
			if i, ok = skipEnv(words, i+1); !ok {
				return 0, false
			}
		case precommands[w]:
			i++
		default:
			return i, true
		}
	}
	return 0, false
}

// envFlags are env's options without a value that the guard reads past.
var envFlags = map[string]bool{"-": true, "-i": true, "-0": true, "--ignore-environment": true, "--null": true}

// skipEnv is the index of the first word after env's options and assignments
// (words[i:] is what follows env): -i, -0, -, --ignore-environment, --null,
// -u NAME in the forms env takes it, and -- ending the options. An option env
// has otherwise (-S, -C) cannot be read, so ok is false.
func skipEnv(words []string, i int) (int, bool) {
	options := true
	for i < len(words) {
		w := words[i]
		switch {
		case options && w == "--":
			options = false
		case options && (w == "-u" || w == "--unset"):
			i++
			if i >= len(words) {
				return 0, false
			}
		case options && strings.HasPrefix(w, "-"):
			if !envFlags[w] && !strings.HasPrefix(w, "-u") && !strings.HasPrefix(w, "--unset=") {
				return 0, false
			}
		case isAssignment(w):
			options = false
		default:
			return i, true
		}
		i++
	}
	return i, true
}

// isAssignment is whether a word is an assignment to env: any word with a
// name before an =, as env reads it. Bash is stricter (the name must be a shell
// identifier), but a bare assignment before a command is the parser's, which
// keeps bash's rule, and bash runs A-B=1 git as the command A-B=1.
func isAssignment(w string) bool {
	name, _, ok := strings.Cut(w, "=")
	return ok && name != ""
}

// literal is the word as bash passes it to the command, when it is fixed
// text: unquoted, single-quoted or double-quoted text with no expansion.
func literal(w *syntax.Word) (string, bool) {
	var b strings.Builder
	for _, part := range w.Parts {
		switch p := part.(type) {
		case *syntax.Lit:
			b.WriteString(unescape(p.Value, false))
		case *syntax.SglQuoted:
			if p.Dollar && strings.Contains(p.Value, `\`) {
				return "", false
			}
			b.WriteString(p.Value)
		case *syntax.DblQuoted:
			if p.Dollar {
				return "", false
			}
			for _, q := range p.Parts {
				l, ok := q.(*syntax.Lit)
				if !ok {
					return "", false
				}
				b.WriteString(unescape(l.Value, true))
			}
		default:
			return "", false
		}
	}
	return b.String(), true
}

// unescape takes bash's backslashes out of text: unquoted, a backslash keeps
// the next character and a backslash before a newline joins the lines; in
// double quotes, it does so only before $, `, ", \ and a newline.
func unescape(text string, quoted bool) string {
	if !strings.Contains(text, `\`) {
		return text
	}
	var b strings.Builder
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c != '\\' || i+1 >= len(text) {
			b.WriteByte(c)
			continue
		}
		next := text[i+1]
		if quoted && !strings.ContainsRune("$`\"\\\n", rune(next)) {
			b.WriteByte(c)
			continue
		}
		i++
		if next != '\n' {
			b.WriteByte(next)
		}
	}
	return b.String()
}
