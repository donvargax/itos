package corpus

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// compare is what differs between a case's expectations and its outcome, as lines to print.
// With additive, json and files_after are judged by what they must still hold, a written file by
// its data.
func compare(c *Case, out *Outcome, s *subst, additive bool) []string {
	var problems []string
	if out.Exit == nil || float64(*out.Exit) != c.exit {
		got := "null"
		if out.Exit != nil {
			got = strconv.Itoa(*out.Exit)
		}
		problems = append(problems, fmt.Sprintf("exit: expected %s, got %s", number(c.exit), got))
	}
	if c.stdout != nil {
		problems = append(problems, textProblems("stdout", s.text(*c.stdout), out.Stdout)...)
	}
	if c.stderr != nil {
		problems = append(problems, textProblems("stderr", s.text(*c.stderr), out.Stderr)...)
	}
	if c.stdoutNonempty && strings.TrimSpace(out.Stdout) == "" {
		problems = append(problems, quoted("stdout is empty, expected something besides whitespace:", out.Stdout)...)
	}
	problems = append(problems, hasProblems("stdout", c.stdoutHas, out.Stdout, s)...)
	problems = append(problems, hasProblems("stderr", c.stderrHas, out.Stderr, s)...)
	problems = append(problems, lacksProblems("stdout", c.stdoutLacks, out.Stdout, s)...)
	problems = append(problems, lacksProblems("stderr", c.stderrLacks, out.Stderr, s)...)
	if c.hasJSON {
		problems = append(problems, jsonProblems(s.deep(c.json), out.Stdout, additive)...)
	}
	if c.hasJSONHas {
		problems = append(problems, jsonHasProblems(s.deep(c.jsonHas), out.Stdout)...)
	}
	for _, e := range c.filesAfter {
		problems = append(problems, fileProblems(e, out.files[e.path], s, additive)...)
	}
	for _, kv := range c.gitConfigAfter {
		var want *string
		if kv.value != nil {
			v := s.text(*kv.value)
			want = &v
		}
		got := out.gitConfig[kv.key]
		if (want == nil) != (got == nil) || (want != nil && *want != *got) {
			problems = append(problems, fmt.Sprintf("git config %s: expected %s, got %s", kv.key, configEntry(want), configEntry(got)))
		}
	}
	return problems
}

func configEntry(v *string) string {
	if v == nil {
		return "it unset"
	}
	return show(*v)
}

// number is a number as JavaScript prints it, for the exit codes a case expects.
func number(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

// diff is a line diff of expected and actual: "- " expected only, "+ " actual only, "  " both.
func diff(expected, actual string) []string {
	a, b := strings.Split(expected, "\n"), strings.Split(actual, "\n")
	lcs := make([][]int, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	var lines []string
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		drop := i < len(a) && (j >= len(b) || lcs[i+1][j] >= lcs[i][j+1])
		switch {
		case i < len(a) && j < len(b) && a[i] == b[j]:
			lines = append(lines, "  "+a[i])
			i++
			j++
		case drop:
			lines = append(lines, "- "+a[i])
			i++
		default:
			lines = append(lines, "+ "+b[j])
			j++
		}
	}
	// Both ending in a newline end in the same empty line: not worth a line.
	if len(lines) > 0 && lines[len(lines)-1] == "  " {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// quoted is a text set off under the line that says why.
func quoted(why, text string) []string {
	lines := []string{why}
	for _, l := range strings.Split(text, "\n") {
		lines = append(lines, "    | "+l)
	}
	return lines
}

func indented(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = "    " + l
	}
	return out
}

func textProblems(what, expected, actual string) []string {
	if expected == actual {
		return nil
	}
	return append([]string{what + " (- expected, + got):"}, indented(diff(expected, actual))...)
}

// hasProblems is the first part a stream lacks of the parts it must contain.
func hasProblems(stream string, wanted []string, got string, s *subst) []string {
	for _, part := range wanted {
		part = s.text(part)
		if !strings.Contains(got, part) {
			return quoted(stream+" lacks "+show(part)+":", got)
		}
	}
	return nil
}

// lacksProblems is the first part a stream holds of the parts it must not.
func lacksProblems(stream string, unwanted []string, got string, s *subst) []string {
	for _, part := range unwanted {
		part = s.text(part)
		if strings.Contains(got, part) {
			return quoted(stream+" holds "+show(part)+", which it must not:", got)
		}
	}
	return nil
}

// jsonProblems is where stdout's JSON differs from expected, both less every unjudged key: the
// whole of it, or with additive what expected holds (jsonLacks).
func jsonProblems(expected any, stdout string, additive bool) []string {
	actual, err := parseJSON(stdout)
	if err != nil {
		return quoted("stdout is not JSON:", stdout)
	}
	var left int
	expected, actual = machine(expected, &left, true), machine(actual, &left, true)
	if !additive {
		return textProblems("json", pretty(expected), pretty(actual))
	}
	if lacks := jsonLacks(expected, actual, ""); len(lacks) > 0 {
		return append([]string{"json lacks what it held (a key added is compatible):"}, indented(lacks)...)
	}
	return nil
}

// pretty is a value as two values compare line by line: as JavaScript's JSON.stringify writes it,
// two spaces, keys sorted at every level.
func pretty(v any) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(plain(v)); err != nil {
		return "nothing\n"
	}
	return b.String()
}

// show is a value on one line, as JavaScript's JSON.stringify writes it: a mapping's keys in the
// order they were written or read.
func show(v any) string {
	var b strings.Builder
	if !writeJSON(&b, v) {
		return "nothing"
	}
	return b.String()
}

func writeJSON(b *strings.Builder, v any) bool {
	switch t := v.(type) {
	case *mapping:
		b.WriteByte('{')
		for i, k := range t.keys {
			if i > 0 {
				b.WriteByte(',')
			}
			writeJSON(b, k)
			b.WriteByte(':')
			if !writeJSON(b, t.vals[k]) {
				return false
			}
		}
		b.WriteByte('}')
		return true
	case []any:
		b.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				b.WriteByte(',')
			}
			if !writeJSON(b, e) {
				return false
			}
		}
		b.WriteByte(']')
		return true
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return false
	}
	b.WriteString(strings.TrimSuffix(buf.String(), "\n"))
	return true
}

// parseJSON reads one JSON value as JavaScript's JSON.parse does: an object a *mapping, its keys
// in the order they were read, a number a float64.
func parseJSON(text string) (any, error) {
	dec := json.NewDecoder(strings.NewReader(text))
	v, err := readJSON(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("more than one JSON value")
	}
	return v, nil
}

func readJSON(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch tok {
	case json.Delim('{'):
		m := newMapping()
		for dec.More() {
			key, err := dec.Token()
			if err != nil {
				return nil, err
			}
			v, err := readJSON(dec)
			if err != nil {
				return nil, err
			}
			m.set(key.(string), v)
		}
		_, err := dec.Token()
		return m, err
	case json.Delim('['):
		list := []any{}
		for dec.More() {
			v, err := readJSON(dec)
			if err != nil {
				return nil, err
			}
			list = append(list, v)
		}
		_, err := dec.Token()
		return list, err
	}
	return tok, nil
}

// jsonLacks is where actual does not hold expected: every key of an object still there with what
// expected gives it, an array element by element at the same length, any other value equal. Each
// line names its place, a path from the top ("." for the whole).
func jsonLacks(expected, actual any, at string) []string {
	place := at
	if place == "" {
		place = "."
	}
	differs := []string{fmt.Sprintf("%s: expected %s, got %s", place, show(expected), show(actual))}
	switch want := expected.(type) {
	case *mapping:
		got, ok := actual.(*mapping)
		if !ok {
			return differs
		}
		var lacks []string
		for _, k := range want.keys {
			v, ok := got.get(k)
			if !ok {
				lacks = append(lacks, fmt.Sprintf("%s.%s: expected %s, got none", at, k, show(want.vals[k])))
				continue
			}
			lacks = append(lacks, jsonLacks(want.vals[k], v, at+"."+k)...)
		}
		return lacks
	case []any:
		got, ok := actual.([]any)
		if !ok {
			return differs
		}
		if len(got) != len(want) {
			return []string{fmt.Sprintf("%s: expected %d elements, got %d: %s", place, len(want), len(got), show(actual))}
		}
		var lacks []string
		for i := range want {
			lacks = append(lacks, jsonLacks(want[i], got[i], fmt.Sprintf("%s[%d]", at, i))...)
		}
		return lacks
	}
	switch actual.(type) {
	case *mapping, []any:
		return differs
	}
	if expected != actual {
		return differs
	}
	return nil
}

// anyValue, as a value anywhere in a json_has, holds whatever value stands there: the key, or
// the array element, must be present, its value not judged.
const anyValue = "<any>"

// jsonHasProblems is where stdout's JSON does not hold what a case's json_has names (jsonHolds).
func jsonHasProblems(expected any, stdout string) []string {
	actual, err := parseJSON(stdout)
	if err != nil {
		return quoted("stdout is not JSON:", stdout)
	}
	if lacks := jsonHolds(expected, actual, ""); len(lacks) > 0 {
		return append([]string{"json lacks what json_has names:"}, indented(lacks)...)
	}
	return nil
}

// jsonHolds is where actual does not hold expected, a json_has's subset: every key of an object
// present with what expected gives it, keys it does not name passing; every element of an array
// held by some element of actual's, in any order, whatever else actual's holds; anyValue holding
// any value; any other value equal. Unlike json:, it judges every key it names, message, fix and
// text included. Each line names its place, a path from the top ("." for the whole).
func jsonHolds(expected, actual any, at string) []string {
	if expected == anyValue {
		return nil
	}
	place := at
	if place == "" {
		place = "."
	}
	differs := []string{fmt.Sprintf("%s: expected %s, got %s", place, show(expected), show(actual))}
	switch want := expected.(type) {
	case *mapping:
		got, ok := actual.(*mapping)
		if !ok {
			return differs
		}
		var lacks []string
		for _, k := range want.keys {
			v, ok := got.get(k)
			if !ok {
				lacks = append(lacks, fmt.Sprintf("%s.%s: expected %s, got none", at, k, show(want.vals[k])))
				continue
			}
			lacks = append(lacks, jsonHolds(want.vals[k], v, at+"."+k)...)
		}
		return lacks
	case []any:
		got, ok := actual.([]any)
		if !ok {
			return differs
		}
		var lacks []string
		for i, w := range want {
			if !slices.ContainsFunc(got, func(g any) bool { return len(jsonHolds(w, g, "")) == 0 }) {
				lacks = append(lacks, fmt.Sprintf("%s[%d]: expected %s among %s", at, i, show(w), show(actual)))
			}
		}
		return lacks
	}
	switch actual.(type) {
	case *mapping, []any:
		return differs
	}
	if expected != actual {
		return differs
	}
	return nil
}

var (
	commentLines = regexp.MustCompile(`(?m)^[ \t]*<!--[\s\S]*?-->[ \t]*(?:\n|$)`)
	comments     = regexp.MustCompile(`<!--[\s\S]*?-->`)
	yamlFile     = regexp.MustCompile(`\.ya?ml$`)
)

// markdownContent is a Markdown text without its HTML comments: a comment standing alone on its
// lines goes with them, one inside a line leaves the rest of the line.
func markdownContent(text string) string {
	return comments.ReplaceAllString(commentLines.ReplaceAllString(text, ""), "")
}

// yamlData is a YAML text's data, comments dropped; false when it is not YAML.
func yamlData(text string) (any, bool) {
	v, err := parseYAML([]byte(text), false)
	if err != nil {
		return nil, false
	}
	return v, true
}

// dataProblems is a written file's data against what it held, comment lines aside (decision
// 40): YAML by what it parses to, judged as json is, a key added passing; Markdown by its text
// less its HTML comments; any other file whole.
func dataProblems(path, expected, actual string) []string {
	if yamlFile.MatchString(path) {
		if want, ok := yamlData(expected); ok {
			got, ok := yamlData(actual)
			if !ok {
				return quoted(path+" is not YAML:", actual)
			}
			if lacks := jsonLacks(want, got, ""); len(lacks) > 0 {
				return append([]string{path + "'s data lacks what it held (a key added or a comment changed is compatible):"}, indented(lacks)...)
			}
			return nil
		}
	}
	if strings.HasSuffix(path, ".md") {
		return textProblems(path+", its HTML comments aside", markdownContent(expected), markdownContent(actual))
	}
	return textProblems(path, expected, actual)
}

func fileProblems(e entry, actual *written, s *subst, additive bool) []string {
	if e.null {
		if actual == nil {
			return nil
		}
		return []string{e.path + ": expected no file, found one"}
	}
	if actual == nil {
		return []string{e.path + ": expected a file, found none"}
	}
	judge := textProblems
	if additive {
		judge = dataProblems
	}
	problems := judge(e.path, s.text(e.text), actual.text)
	if e.executable != nil && *e.executable != actual.executable {
		not := "not "
		if *e.executable {
			not = ""
		}
		problems = append(problems, e.path+": expected "+not+"executable")
	}
	return problems
}
