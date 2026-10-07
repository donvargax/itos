// Command plugin-calls is the plugin's calls gate (T-111): the built itos
// answers every itos command the Claude Code plugin's scripts call.
//
// The commands the plugin calls (guard claude-code, and work list --all and
// task list with --json for its titles) are part of itos's contract with the
// plugin, never renamed or removed, even in a major release (decision 41,
// docs/CLI.md's entry points). Without a gate that is only a promise: an
// installed plugin keeps calling the old name, and a rename would surface in
// someone's session after the release, not on the push that made it.
//
// So it reads the calls from the plugin's scripts themselves, never from a
// list kept beside them, runs each against the built itos, and refuses one
// that itos does not answer. The scripts are the files under the plugin's
// folder, test files (*.test.*, *.spec.*) and declaration files aside:
//
//   - A shell script (*.sh): a simple command whose program is itos (after
//     any NAME=value words, a reserved word such as if or !, or exec or
//     command) is a call, its words up to the next operator, redirections
//     left out. command -v, type, which and hash name itos to look it up.
//   - A TypeScript or JavaScript script (*.ts, *.js, *.mts, *.mjs, *.cjs): an
//     array literal whose first element is the string "itos", or a const
//     bound to it, is an argv. Its other elements are string literals or a
//     spread of a parameter of the function declaration (function NAME(…))
//     around it, which each call of NAME in the same file fills with an
//     array literal of strings: register.ts's itos($, root, ["task", "list"])
//     runs [ITOS, ...args, "--json"], so the call is itos task list --json.
//   - A JSON file (hooks.json, plugin.json): each hook's "command" string, as
//     a shell script.
//
// Any other mention of itos that names it as a program (the word itos in a
// shell command, the string or the const anywhere but at an argv's head, a
// string holding a command line that starts with itos), and any argv it
// cannot read word by word (a variable, a forwarded parameter), stops it with
// exit 2, naming the script and the line: a call it cannot read is a call it
// would let break. So does finding no call at all.
//
// Each call runs in the directory it runs in (the repository's top in CI),
// given on stdin a harmless PreToolUse input for a Bash true, as Claude Code
// gives the guard; a command that reads no stdin ignores it. A call is
// refused when:
//
//   - it exits 2, itos's usage error: an unknown command, group or flag. Its
//     other exits are its verdict, which this does not judge: the guard may
//     say yes or no, so long as it answers.
//
//   - it asks for --json and does not exit 0 with a JSON object holding, as
//     a list, a key the plugin reads; the plugin reads only an answer that
//     exits 0. The keys it reads are the string literals given as
//     the second argument of titles.ts's listed(answer, "items" | "tasks"),
//     read from the scripts too. Each key must be held by some answer, so a
//     key renamed in one command fails; which key a command must hold is not
//     traced through the code, so two commands trading keys would pass.
//
//     go run ./tools/bin/plugin-calls [-plugin <folder>] [-itos <program>]
//
// -plugin is the plugin's folder (integrations/claude-code), -itos the itos
// to run (tools/bin/itos, the binary built from this tree). It imports nothing
// but the standard library, as the other gate programs do, and asks no
// network. Exit status: 0 every call answered, 1 a call refused, each named
// with the scripts that call it, 2 a script it cannot read, no call found, or
// an itos it cannot start.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// keyReader is the plugin's function that reads a list out of an itos
// answer: titles.ts's listed(json, "items" | "tasks").
const keyReader = "listed"

// callTimeout bounds one call; tools/bin/itos may build itos first.
const callTimeout = 5 * time.Minute

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// call is one itos command the plugin runs: its words after itos, and where
// the scripts run it.
type call struct {
	words []string
	sites []string // script:line, in the order found
}

func (c *call) String() string { return "itos " + strings.Join(c.words, " ") }

func (c *call) json() bool {
	for _, w := range c.words {
		if w == "--json" {
			return true
		}
	}
	return false
}

// found is what the scripts hold: their calls, the keys they read from
// --json answers, and what they name itos by that cannot be read.
type found struct {
	calls      []*call
	keys       map[string][]string // key → sites
	unreadable []string            // "script:line: why"
}

func (f *found) addCall(words []string, site string) {
	for _, c := range f.calls {
		if slicesEqual(c.words, words) {
			c.sites = append(c.sites, site)
			return
		}
	}
	f.calls = append(f.calls, &call{words: words, sites: []string{site}})
}

func (f *found) addKey(key, site string) { f.keys[key] = append(f.keys[key], site) }

func (f *found) refuse(site, format string, a ...any) {
	f.unreadable = append(f.unreadable, site+": "+fmt.Sprintf(format, a...))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("plugin-calls", flag.ContinueOnError)
	flags.SetOutput(stderr)
	plugin := flags.String("plugin", "integrations/claude-code", "the plugin's folder, whose scripts are read")
	itos := flags.String("itos", "tools/bin/itos", "the itos each call runs against")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "plugin-calls: unexpected argument %q\n", flags.Arg(0))
		return 2
	}

	f, scripts, err := read(*plugin)
	if err != nil {
		fmt.Fprintf(stderr, "plugin-calls: cannot read the plugin's scripts in %s: %v\n", *plugin, err)
		return 2
	}
	if len(f.unreadable) > 0 {
		for _, u := range f.unreadable {
			fmt.Fprintf(stderr, "plugin-calls: %s\n", u)
		}
		fmt.Fprintf(stderr, "\nplugin-calls: a call it cannot read word by word is a call it would let break.\n"+
			"  Write each itos call as plain words (a shell command, or an argv of string literals,\n"+
			"  at most through one function's spread parameter), or teach this program the new form.\n")
		return 2
	}
	if len(f.calls) == 0 {
		fmt.Fprintf(stderr, "plugin-calls: no itos call found in the %d script(s) under %s (%s):\n"+
			"  either the plugin calls itos no longer, or this program no longer reads how it does\n",
			len(scripts), *plugin, strings.Join(scripts, ", "))
		return 2
	}
	var jsonCalls int
	for _, c := range f.calls {
		if c.json() {
			jsonCalls++
		}
	}
	if jsonCalls > 0 && len(f.keys) == 0 {
		fmt.Fprintf(stderr, "plugin-calls: the plugin asks itos for --json, but no %s(answer, \"<key>\") in its scripts says which key it reads\n", keyReader)
		return 2
	}
	if jsonCalls == 0 && len(f.keys) > 0 {
		fmt.Fprintf(stderr, "plugin-calls: the plugin reads %s from an itos answer, but no call it makes asks for --json\n", strings.Join(sortedKeys(f.keys), ", "))
		return 2
	}

	input, err := hookInput()
	if err != nil {
		fmt.Fprintf(stderr, "plugin-calls: %v\n", err)
		return 2
	}
	held := map[string]bool{}
	var refused []string
	for _, c := range f.calls {
		res, err := runCall(*itos, c.words, input)
		if err != nil {
			fmt.Fprintf(stderr, "plugin-calls: cannot run %s with %s: %v\n", c, *itos, err)
			return 2
		}
		where := strings.Join(c.sites, ", ")
		switch {
		case res.timedOut:
			refused = append(refused, fmt.Sprintf("%s, called by %s, did not answer within %s", c, where, callTimeout))
		case res.code == 2:
			refused = append(refused, fmt.Sprintf("%s, called by %s, exits 2: itos does not take it as written%s", c, where, firstLine(res.stderr)))
		case !c.json():
			fmt.Fprintf(stdout, "plugin-calls: %s, called by %s, is answered (exit %d)\n", c, where, res.code)
		case res.code != 0:
			refused = append(refused, fmt.Sprintf("%s, called by %s, exits %d, and the plugin reads only an answer that exits 0%s", c, where, res.code, firstLine(res.stderr)))
		default:
			has, err := listsHeld(res.stdout, f.keys)
			if err != nil {
				refused = append(refused, fmt.Sprintf("%s, called by %s, exits 0 but %v", c, where, err))
				break
			}
			if len(has) == 0 {
				refused = append(refused, fmt.Sprintf("%s, called by %s, answers with none of the lists the plugin reads (%s)", c, where, strings.Join(sortedKeys(f.keys), ", ")))
				break
			}
			for _, k := range has {
				held[k] = true
			}
			fmt.Fprintf(stdout, "plugin-calls: %s, called by %s, is answered with %s\n", c, where, strings.Join(has, ", "))
		}
	}
	for _, k := range sortedKeys(f.keys) {
		if !held[k] {
			refused = append(refused, fmt.Sprintf("the plugin reads %q from an itos answer (%s), but no --json call answers with it as a list", k, strings.Join(f.keys[k], ", ")))
		}
	}
	if len(refused) == 0 {
		return 0
	}
	for _, r := range refused {
		fmt.Fprintf(stderr, "plugin-calls: %s\n", r)
	}
	fmt.Fprintf(stderr, "\nplugin-calls: the commands the Claude Code plugin calls are part of itos's contract with it, never\n"+
		"  renamed or removed, even in a major release (decision 41): an installed plugin keeps calling them.\n"+
		"  Put the command, the flag or the key back as the plugin calls it.\n")
	return 1
}

// read is what the plugin's scripts hold, and the scripts it read.
func read(folder string) (*found, []string, error) {
	f := &found{keys: map[string][]string{}}
	var scripts []string
	root := filepath.FromSlash(folder)
	info, err := os.Stat(root)
	if err != nil {
		return nil, nil, err
	}
	if !info.IsDir() {
		return nil, nil, errors.New("not a folder")
	}
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if p != root && (name == "node_modules" || strings.HasPrefix(name, ".") && name != ".claude-plugin") {
				return filepath.SkipDir
			}
			return nil
		}
		kind := kindOf(name)
		if kind == "" {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		script := path.Join(folder, filepath.ToSlash(rel))
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		scripts = append(scripts, script)
		switch kind {
		case "sh":
			readShell(f, script, string(src), 1)
		case "js":
			readScript(f, script, string(src))
		case "json":
			readHooks(f, script, src)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	if len(scripts) == 0 {
		return nil, nil, errors.New("it holds no script (*.sh, *.ts, *.js, *.json)")
	}
	return f, scripts, nil
}

// kindOf is how a file is read: "sh", "js", "json", or "" when it is no
// script or is a test.
func kindOf(name string) string {
	if strings.Contains(name, ".test.") || strings.Contains(name, ".spec.") || strings.HasSuffix(name, ".d.ts") {
		return ""
	}
	switch path.Ext(name) {
	case ".sh":
		return "sh"
	case ".ts", ".mts", ".cts", ".js", ".mjs", ".cjs":
		return "js"
	}
	if name == "hooks.json" || name == "plugin.json" {
		return "json"
	}
	return ""
}

// --- shell -----------------------------------------------------------------

type shToken struct {
	text    string
	line    int
	op      bool // an operator: ; & | && || ;; ( ) or a line's end
	redir   bool // a redirection, its target attached or the next word
	target  bool // the redirection wants the next word as its target
	dynamic bool // a word whose value is known only when it runs
}

func lexShell(src string, firstLine int) []shToken {
	var toks []shToken
	line := firstLine
	i := 0
	n := len(src)
	for i < n {
		c := src[i]
		switch {
		case c == '\\' && i+1 < n && src[i+1] == '\n':
			i += 2
			line++
			continue
		case c == ' ' || c == '\t' || c == '\r':
			i++
			continue
		case c == '\n':
			toks = append(toks, shToken{text: "\n", line: line, op: true})
			line++
			i++
			continue
		case c == '#':
			for i < n && src[i] != '\n' {
				i++
			}
			continue
		case c == ';' || c == '&' || c == '|' || c == '(' || c == ')':
			op := string(c)
			if i+1 < n && (c == ';' || c == '&' || c == '|') && src[i+1] == c {
				op += string(c)
			}
			toks = append(toks, shToken{text: op, line: line, op: true})
			i += len(op)
			continue
		}
		// A word, perhaps a redirection.
		start := line
		var b strings.Builder
		dynamic := false
		redir := false
		for i < n {
			c := src[i]
			if c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == ';' || c == '&' && !redir || c == '|' || c == '(' || c == ')' {
				break
			}
			if c == '<' || c == '>' {
				digits := b.Len() > 0 && strings.Trim(b.String(), "0123456789") == ""
				if b.Len() > 0 && !digits && !redir {
					break // a redirection glued to a word starts a token of its own
				}
				redir = true
				b.WriteByte(c)
				i++
				continue
			}
			switch c {
			case '\\':
				if i+1 < n {
					if src[i+1] == '\n' {
						line++
						i += 2
						continue
					}
					b.WriteByte(src[i+1])
					i += 2
				} else {
					i++
				}
			case '\'':
				j := strings.IndexByte(src[i+1:], '\'')
				if j < 0 {
					j = n - i - 1
				}
				lit := src[i+1 : i+1+j]
				line += strings.Count(lit, "\n")
				b.WriteString(lit)
				i += j + 2
			case '"':
				i++
				for i < n && src[i] != '"' {
					switch src[i] {
					case '\\':
						if i+1 < n {
							b.WriteByte(src[i+1])
							i += 2
							continue
						}
					case '$', '`':
						dynamic = true
					case '\n':
						line++
					}
					b.WriteByte(src[i])
					i++
				}
				i++
			case '$':
				dynamic = true
				b.WriteByte(c)
				i++
				if i < n && (src[i] == '(' || src[i] == '{') {
					open, close := src[i], byte(')')
					if open == '{' {
						close = '}'
					}
					depth := 0
					for i < n {
						switch src[i] {
						case open:
							depth++
						case close:
							depth--
						case '\n':
							line++
						}
						b.WriteByte(src[i])
						i++
						if depth == 0 {
							break
						}
					}
				}
			case '`', '*', '?', '[':
				dynamic = true
				b.WriteByte(c)
				i++
			default:
				b.WriteByte(c)
				i++
			}
		}
		text := b.String()
		if redir {
			t := strings.TrimLeft(text, "0123456789")
			t = strings.TrimLeft(t, "<>&|")
			toks = append(toks, shToken{text: text, line: start, redir: true, target: t == ""})
			continue
		}
		toks = append(toks, shToken{text: text, line: start, dynamic: dynamic})
	}
	return toks
}

// prefixes are the words that may stand before a command's program.
var prefixes = map[string]bool{
	"if": true, "then": true, "else": true, "elif": true, "while": true, "until": true,
	"do": true, "!": true, "{": true, "time": true, "exec": true,
}

var lookups = map[string]bool{"type": true, "which": true, "hash": true}

var assignment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

// namesItos is whether a word names itos as a program: itos, or a path to it.
func namesItos(word string) bool { return word == "itos" || strings.HasSuffix(word, "/itos") }

// readShell adds a shell script's calls; firstLine is the line its text
// starts on in the script.
func readShell(f *found, script, src string, firstLine int) {
	toks := lexShell(src, firstLine)
	var words []shToken
	flush := func() {
		readCommand(f, script, words)
		words = words[:0]
	}
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		switch {
		case t.op:
			flush()
		case t.redir:
			if t.target {
				i++ // its target
			}
		default:
			words = append(words, t)
		}
	}
	flush()
}

// readCommand reads one simple command's words.
func readCommand(f *found, script string, words []shToken) {
	ws := words
	for len(ws) > 0 && !ws[0].dynamic && (assignment.MatchString(ws[0].text) || prefixes[ws[0].text]) {
		ws = ws[1:]
	}
	if len(ws) > 0 && !ws[0].dynamic && ws[0].text == "command" {
		ws = ws[1:]
		if len(ws) > 0 && (ws[0].text == "-v" || ws[0].text == "-V") {
			ws = nil // a lookup: the names after it are not run
		}
	}
	if len(ws) > 0 && lookups[ws[0].text] {
		ws = nil
	}
	call := len(ws) > 0 && !ws[0].dynamic && namesItos(ws[0].text)
	if call {
		site := fmt.Sprintf("%s:%d", script, ws[0].line)
		var argv []string
		for _, w := range ws[1:] {
			if w.dynamic {
				f.refuse(site, "itos is called with %q, a word known only when it runs", w.text)
				return
			}
			argv = append(argv, w.text)
		}
		if len(argv) == 0 {
			f.refuse(site, "itos is called alone, which is no command of the plugin's")
			return
		}
		f.addCall(argv, site)
		ws = ws[1:]
	}
	// itos anywhere else in a command that is not a lookup is a call this
	// cannot read: an argument to a wrapper, or a word built when it runs.
	lookup := len(ws) == 0 && !call
	if lookup {
		return
	}
	for _, w := range ws {
		if namesItos(w.text) || w.dynamic && strings.Contains(strings.ToLower(w.text), "itos") {
			f.refuse(fmt.Sprintf("%s:%d", script, w.line), "%q names itos where this cannot tell how it runs", w.text)
		}
	}
}

// --- JSON hooks -------------------------------------------------------------

// readHooks reads each hook's command in a JSON file as a shell script.
func readHooks(f *found, script string, src []byte) {
	var doc any
	if err := json.Unmarshal(src, &doc); err != nil {
		f.refuse(script, "not JSON: %v", err)
		return
	}
	var walk func(v any)
	walk = func(v any) {
		switch v := v.(type) {
		case map[string]any:
			if cmd, ok := v["command"].(string); ok && v["type"] == "command" {
				readShell(f, script+" (a hook's command)", cmd, 1)
			}
			for _, k := range sortedMapKeys(v) {
				walk(v[k])
			}
		case []any:
			for _, e := range v {
				walk(e)
			}
		}
	}
	walk(doc)
}

// --- TypeScript and JavaScript ---------------------------------------------

type jsKind int

const (
	jsIdent jsKind = iota
	jsString
	jsNumber
	jsRegex
	jsPunct
)

type jsToken struct {
	kind    jsKind
	text    string // an identifier, a punctuator, or a string's value
	line    int
	dynamic bool // a template literal with substitutions
}

func (t jsToken) is(p string) bool { return t.kind == jsPunct && t.text == p }

// regexAfter are the keywords after which a slash starts a regular
// expression, not a division.
var regexAfter = map[string]bool{
	"return": true, "typeof": true, "instanceof": true, "in": true, "of": true, "new": true,
	"delete": true, "void": true, "throw": true, "case": true, "do": true, "else": true,
	"yield": true, "await": true,
}

func identByte(c byte) bool {
	return c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c >= 0x80
}

type jsLexer struct {
	src  string
	i    int
	line int
	toks []jsToken
}

func lexScript(src string) []jsToken {
	l := &jsLexer{src: src, line: 1}
	l.code(false)
	return l.toks
}

// code lexes code up to the end, or, inside a template's substitution, up to
// its closing brace.
func (l *jsLexer) code(substitution bool) {
	depth := 0
	for l.i < len(l.src) {
		c := l.src[l.i]
		switch {
		case c == '\n':
			l.line++
			l.i++
		case c == ' ' || c == '\t' || c == '\r':
			l.i++
		case strings.HasPrefix(l.src[l.i:], "//"):
			for l.i < len(l.src) && l.src[l.i] != '\n' {
				l.i++
			}
		case strings.HasPrefix(l.src[l.i:], "/*"):
			end := strings.Index(l.src[l.i+2:], "*/")
			if end < 0 {
				end = len(l.src) - l.i - 2
			}
			l.line += strings.Count(l.src[l.i:l.i+2+end], "\n")
			l.i += end + 4
		case c == '"' || c == '\'':
			l.quoted(c)
		case c == '`':
			l.template()
		case c == '/' && l.regexHere():
			l.regex()
		case c >= '0' && c <= '9':
			start := l.i
			for l.i < len(l.src) && (identByte(l.src[l.i]) || l.src[l.i] == '.') {
				l.i++
			}
			l.toks = append(l.toks, jsToken{kind: jsNumber, text: l.src[start:l.i], line: l.line})
		case identByte(c):
			start := l.i
			for l.i < len(l.src) && identByte(l.src[l.i]) {
				l.i++
			}
			l.toks = append(l.toks, jsToken{kind: jsIdent, text: l.src[start:l.i], line: l.line})
		default:
			p := string(c)
			for _, long := range []string{"...", "=>"} {
				if strings.HasPrefix(l.src[l.i:], long) {
					p = long
				}
			}
			if p == "{" {
				depth++
			}
			if p == "}" {
				if substitution && depth == 0 {
					l.i++
					return
				}
				depth--
			}
			l.toks = append(l.toks, jsToken{kind: jsPunct, text: p, line: l.line})
			l.i += len(p)
		}
	}
}

func (l *jsLexer) regexHere() bool {
	if len(l.toks) == 0 {
		return true
	}
	prev := l.toks[len(l.toks)-1]
	switch prev.kind {
	case jsIdent:
		return regexAfter[prev.text]
	case jsPunct:
		return prev.text != ")" && prev.text != "]"
	}
	return false
}

func (l *jsLexer) quoted(q byte) {
	line := l.line
	l.i++
	var b strings.Builder
	for l.i < len(l.src) && l.src[l.i] != q && l.src[l.i] != '\n' {
		if l.src[l.i] == '\\' && l.i+1 < len(l.src) {
			b.WriteByte(l.src[l.i+1])
			l.i += 2
			continue
		}
		b.WriteByte(l.src[l.i])
		l.i++
	}
	l.i++
	l.toks = append(l.toks, jsToken{kind: jsString, text: b.String(), line: line})
}

// template lexes a template literal: one string token, then the tokens of its
// substitutions, so a name used inside one is still seen.
func (l *jsLexer) template() {
	at := len(l.toks)
	l.toks = append(l.toks, jsToken{kind: jsString, line: l.line})
	l.i++
	var b strings.Builder
	for l.i < len(l.src) && l.src[l.i] != '`' {
		switch {
		case l.src[l.i] == '\\' && l.i+1 < len(l.src):
			b.WriteByte(l.src[l.i+1])
			l.i += 2
		case strings.HasPrefix(l.src[l.i:], "${"):
			l.toks[at].dynamic = true
			l.i += 2
			l.code(true)
		default:
			if l.src[l.i] == '\n' {
				l.line++
			}
			b.WriteByte(l.src[l.i])
			l.i++
		}
	}
	l.i++
	l.toks[at].text = b.String()
}

func (l *jsLexer) regex() {
	start := l.i
	l.i++
	class := false
	for l.i < len(l.src) && l.src[l.i] != '\n' {
		c := l.src[l.i]
		if c == '\\' {
			l.i += 2
			continue
		}
		l.i++
		if c == '[' {
			class = true
		} else if c == ']' {
			class = false
		} else if c == '/' && !class {
			break
		}
	}
	for l.i < len(l.src) && identByte(l.src[l.i]) {
		l.i++
	}
	l.toks = append(l.toks, jsToken{kind: jsRegex, text: l.src[start:l.i], line: l.line})
}

// function is a function declaration: its name, its parameters by position,
// and its body's tokens, braces included.
type function struct {
	name       string
	params     []string
	start, end int
}

// readScript adds a TypeScript or JavaScript script's calls and keys.
func readScript(f *found, script, src string) {
	toks := lexScript(src)
	site := func(i int) string { return fmt.Sprintf("%s:%d", script, toks[i].line) }

	// The names itos goes by: the string "itos", and a const bound to it.
	names := map[string]bool{}
	declared := map[int]bool{} // the tokens of each such declaration
	for i := 0; i+3 < len(toks); i++ {
		t := toks[i]
		if t.kind == jsIdent && (t.text == "const" || t.text == "let" || t.text == "var") &&
			toks[i+1].kind == jsIdent && toks[i+2].is("=") &&
			toks[i+3].kind == jsString && !toks[i+3].dynamic && namesItos(toks[i+3].text) {
			names[toks[i+1].text] = true
			declared[i+1], declared[i+3] = true, true
		}
	}
	isItos := func(t jsToken) bool {
		return t.kind == jsString && !t.dynamic && namesItos(t.text) || t.kind == jsIdent && names[t.text]
	}

	functions := declarations(toks)
	for i, t := range toks {
		switch {
		case declared[i]:
		case isItos(t):
			if i == 0 || !toks[i-1].is("[") {
				f.refuse(site(i), "%s names itos, but not at the head of an argv this can read", describe(t))
				continue
			}
			readArgv(f, script, toks, i-1, functions)
		case t.kind == jsString && len(strings.Fields(t.text)) > 1 && namesItos(strings.Fields(t.text)[0]):
			f.refuse(site(i), "the string %q holds an itos command line, which this cannot tell how it runs", t.text)
		case t.kind == jsIdent && t.text == keyReader && i+1 < len(toks) && toks[i+1].is("(") &&
			(i == 0 || !toks[i-1].is(".") && !(toks[i-1].kind == jsIdent && toks[i-1].text == "function")):
			args, _ := arguments(toks, i+1)
			if len(args) < 2 || len(args[1]) != 1 || args[1][0].kind != jsString || args[1][0].dynamic {
				f.refuse(site(i), "%s(…) is called without a string literal as its second argument, the key it reads", keyReader)
				continue
			}
			f.addKey(args[1][0].text, site(i))
		}
	}
}

func describe(t jsToken) string {
	if t.kind == jsString {
		return fmt.Sprintf("the string %q", t.text)
	}
	return t.text
}

// declarations are a script's function declarations.
func declarations(toks []jsToken) []function {
	var fns []function
	for i := 0; i+2 < len(toks); i++ {
		if toks[i].kind != jsIdent || toks[i].text != "function" {
			continue
		}
		j := i + 1
		if toks[j].is("*") {
			j++
		}
		if j+1 >= len(toks) || toks[j].kind != jsIdent || !toks[j+1].is("(") {
			continue
		}
		fn := function{name: toks[j].text}
		// Parameters: the name at the start of each comma-separated part of the
		// list, generics' and nested brackets' commas aside.
		depth := 0
		k := j + 1
		expect := true
	params:
		for ; k < len(toks); k++ {
			t := toks[k]
			if t.kind == jsPunct {
				switch t.text {
				case "(", "[", "{", "<":
					depth++
					if depth > 1 && expect {
						fn.params = append(fn.params, "") // a destructured parameter
						expect = false
					}
				case ")", "]", "}", ">":
					depth--
					if depth == 0 {
						k++
						break params
					}
				case ",":
					if depth == 1 {
						expect = true
					}
				}
				continue
			}
			if expect && depth == 1 && t.kind == jsIdent {
				fn.params = append(fn.params, t.text)
				expect = false
			}
		}
		// The body: the first brace after the parameters, to its match.
		for k < len(toks) && !toks[k].is("{") {
			k++
		}
		fn.start = k
		depth = 0
		for ; k < len(toks); k++ {
			if toks[k].is("{") {
				depth++
			} else if toks[k].is("}") {
				depth--
				if depth == 0 {
					break
				}
			}
		}
		fn.end = k
		fns = append(fns, fn)
	}
	return fns
}

// arguments are the comma-separated parts between the parenthesis or bracket
// at open and its match, and the index of the match.
func arguments(toks []jsToken, open int) ([][]jsToken, int) {
	var args [][]jsToken
	var cur []jsToken
	depth := 0
	for k := open; k < len(toks); k++ {
		t := toks[k]
		if t.kind == jsPunct {
			switch t.text {
			case "(", "[", "{":
				depth++
				if depth == 1 {
					continue
				}
			case ")", "]", "}":
				depth--
				if depth == 0 {
					if len(cur) > 0 {
						args = append(args, cur)
					}
					return args, k
				}
			case ",":
				if depth == 1 {
					args = append(args, cur)
					cur = nil
					continue
				}
			}
		}
		cur = append(cur, t)
	}
	return args, len(toks)
}

// readArgv reads the argv whose bracket is at open, its head an itos name.
func readArgv(f *found, script string, toks []jsToken, open int, functions []function) {
	site := fmt.Sprintf("%s:%d", script, toks[open].line)
	elems, end := arguments(toks, open)
	if len(elems[0]) != 1 {
		f.refuse(site, "the argv's head is %s, not itos alone", text(elems[0]))
		return
	}
	if end+1 < len(toks) && toks[end+1].is(".") {
		f.refuse(site, "the argv is changed after it is written (.%s), so this cannot read what it runs", toks[min(end+2, len(toks)-1)].text)
		return
	}
	if len(elems) == 1 {
		f.refuse(site, "the argv is itos alone, which is no command of the plugin's")
		return
	}
	// Each element after the head: a word, or a parameter spread in.
	type part struct {
		word  string
		param int // the spread parameter's position, or -1
	}
	var parts []part
	var fn *function
	for _, e := range elems[1:] {
		switch {
		case len(e) == 1 && e[0].kind == jsString && !e[0].dynamic:
			parts = append(parts, part{word: e[0].text, param: -1})
		case len(e) == 2 && e[0].is("...") && e[1].kind == jsIdent:
			enclosing := innermost(functions, open)
			p := -1
			if enclosing != nil {
				for i, name := range enclosing.params {
					if name == e[1].text {
						p = i
					}
				}
			}
			if p < 0 {
				f.refuse(site, "the argv spreads %s, which is not a parameter of a function declaration around it", e[1].text)
				return
			}
			fn = enclosing
			parts = append(parts, part{param: p})
		default:
			f.refuse(site, "the argv holds %s, which is neither a string literal nor a spread parameter", text(e))
			return
		}
	}
	expand := func(fill func(int) ([]string, bool), at string) {
		var words []string
		for _, p := range parts {
			if p.param < 0 {
				words = append(words, p.word)
				continue
			}
			ws, ok := fill(p.param)
			if !ok {
				return
			}
			words = append(words, ws...)
		}
		f.addCall(words, at)
	}
	if fn == nil {
		expand(nil, site)
		return
	}
	// Each call of the function fills its spread parameters.
	calls := 0
	for i, t := range toks {
		if t.kind != jsIdent || t.text != fn.name || i > 0 && toks[i-1].is(".") {
			continue
		}
		if i > 0 && toks[i-1].kind == jsIdent && toks[i-1].text == "function" {
			continue // its declaration
		}
		at := fmt.Sprintf("%s:%d", script, t.line)
		if i+1 >= len(toks) || !toks[i+1].is("(") {
			f.refuse(at, "%s, which runs itos, is used as a value, so this cannot read what it runs", fn.name)
			continue
		}
		calls++
		args, _ := arguments(toks, i+1)
		expand(func(p int) ([]string, bool) {
			if p >= len(args) || len(args[p]) < 2 || !args[p][0].is("[") {
				f.refuse(at, "%s is called without an array literal of strings for its parameter %s", fn.name, fn.params[p])
				return nil, false
			}
			elems, _ := arguments(args[p], 0)
			var words []string
			for _, e := range elems {
				if len(e) != 1 || e[0].kind != jsString || e[0].dynamic {
					f.refuse(at, "%s is called with %s in its parameter %s, not a string literal", fn.name, text(e), fn.params[p])
					return nil, false
				}
				words = append(words, e[0].text)
			}
			return words, true
		}, at)
	}
	if calls == 0 {
		f.refuse(site, "%s runs itos, but nothing in %s calls it, so this cannot read what it runs", fn.name, script)
	}
}

// innermost is the function declaration whose body holds token i.
func innermost(fns []function, i int) *function {
	var best *function
	for k := range fns {
		fn := &fns[k]
		if fn.start < i && i < fn.end && (best == nil || fn.start > best.start) {
			best = fn
		}
	}
	return best
}

func text(ts []jsToken) string {
	var parts []string
	for _, t := range ts {
		if t.kind == jsString {
			parts = append(parts, fmt.Sprintf("%q", t.text))
		} else {
			parts = append(parts, t.text)
		}
	}
	return strings.Join(parts, " ")
}

// --- running the calls ------------------------------------------------------

// hookInput is a harmless PreToolUse input, as Claude Code hands the guard: a
// Bash true, in the directory the calls run in.
func hookInput() ([]byte, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"session_id":      "plugin-calls",
		"transcript_path": "",
		"cwd":             cwd,
		"permission_mode": "default",
		"hook_event_name": "PreToolUse",
		"tool_name":       "Bash",
		"tool_input":      map[string]any{"command": "true", "description": "plugin-calls: a harmless command"},
	})
}

type result struct {
	code           int
	stdout, stderr []byte
	timedOut       bool
}

func runCall(itos string, words []string, input []byte) (result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, itos, words...)
	cmd.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	res := result{stdout: stdout.Bytes(), stderr: stderr.Bytes()}
	if ctx.Err() != nil {
		res.timedOut = true
		return res, nil
	}
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit) && exit.ExitCode() >= 0:
		res.code = exit.ExitCode()
	default:
		return res, err
	}
	return res, nil
}

// listsHeld are the keys the plugin reads that an answer holds as a list.
func listsHeld(stdout []byte, keys map[string][]string) ([]string, error) {
	var answer map[string]json.RawMessage
	if err := json.Unmarshal(stdout, &answer); err != nil {
		return nil, fmt.Errorf("its answer is not a JSON object: %v", err)
	}
	var has []string
	for _, k := range sortedKeys(keys) {
		var list []json.RawMessage
		if raw, ok := answer[k]; ok && json.Unmarshal(raw, &list) == nil && list != nil {
			has = append(has, k)
		}
	}
	return has, nil
}

func firstLine(b []byte) string {
	s := strings.TrimSpace(string(b))
	if s == "" {
		return ""
	}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return ": " + s
}

func sortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
