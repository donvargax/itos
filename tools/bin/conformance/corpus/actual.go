package corpus

import (
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Actual is what a case's run did, unjudged: the command line's --actual, which prints it so a
// case's expectations can be written from it.
type Actual struct {
	Fixture *Fixture
	Case    *Case
	Outcome *Outcome
	Err     error // the case could not run
	s       *subst
}

// Actuals runs every case of the fixtures, Jobs at a time, and judges none.
func (r *Runner) Actuals(fixtures []*Fixture) []Actual {
	var actuals []Actual
	for _, f := range fixtures {
		for _, c := range f.Cases {
			actuals = append(actuals, Actual{Fixture: f, Case: c})
		}
	}
	r.each(len(actuals), func(i int) {
		a := &actuals[i]
		a.Outcome, a.s, a.Err = r.RunCase(a.Case)
	})
	return actuals
}

// WriteActuals writes each case's outcome as a YAML list, its expectations ready to paste: file,
// name and argv, then exit, stdout (when it is not JSON), stderr, json (stdout's, when it is
// JSON, less the keys json: does not judge) and files_after (the files the case names there, as
// the run left them; null for one absent), or error, why it could not run. The case's folder, its
// labelled commits and the binary's version are written back as {{dir}}, {{sha.<label>}},
// {{short.<label>}} and {{version}}.
func WriteActuals(w io.Writer, actuals []Actual) error {
	list := &yaml.Node{Kind: yaml.SequenceNode}
	for _, a := range actuals {
		list.Content = append(list.Content, a.node())
	}
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(list); err != nil {
		return err
	}
	return enc.Close()
}

func (a Actual) node() *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode}
	add := func(key string, v *yaml.Node) { m.Content = append(m.Content, str(key), v) }
	add("file", str(a.Fixture.Base()))
	add("name", str(a.Case.Name))
	argv := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
	for _, arg := range a.Case.Argv {
		argv.Content = append(argv.Content, str(arg))
	}
	add("argv", argv)
	if a.Err != nil {
		add("error", str(a.Err.Error()))
		return m
	}
	back := a.unsubst()
	out := a.Outcome
	if out.Exit == nil {
		add("exit", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"})
	} else {
		add("exit", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(*out.Exit)})
	}
	v, err := parseJSON(out.Stdout)
	if err != nil {
		add("stdout", str(back(out.Stdout)))
	}
	add("stderr", str(back(out.Stderr)))
	if err == nil {
		var left int
		add("json", valueNode(mapStrings(machine(v, &left, true), back)))
	}
	if len(a.Case.filesAfter) > 0 {
		files := &yaml.Node{Kind: yaml.MappingNode}
		for _, e := range a.Case.filesAfter {
			value := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}
			if f := out.files[e.path]; f != nil {
				value = str(back(f.text))
			}
			files.Content = append(files.Content, str(e.path), value)
		}
		add("files_after", files)
	}
	return m
}

// unsubst is the case's substitution undone: its folder, its labelled commits (whole, then
// short) and the binary's version written as the placeholders a case names them by.
func (a Actual) unsubst() func(string) string {
	var pairs []string
	if a.s.dir != "" {
		pairs = append(pairs, a.s.dir, "{{dir}}")
		if real, err := filepath.EvalSymlinks(a.s.dir); err == nil && real != a.s.dir {
			pairs = append(pairs, real, "{{dir}}")
		}
	}
	labels := make([]string, 0, len(a.s.labels))
	for l := range a.s.labels {
		labels = append(labels, l)
	}
	sort.Strings(labels)
	for _, l := range labels {
		pairs = append(pairs, a.s.labels[l], "{{sha."+l+"}}")
	}
	for _, l := range labels {
		sha := a.s.labels[l]
		pairs = append(pairs, sha[:min(7, len(sha))], "{{short."+l+"}}")
	}
	if a.s.version != "" {
		pairs = append(pairs, a.s.version, "{{version}}")
	}
	replacer := strings.NewReplacer(pairs...)
	return replacer.Replace
}

// str is a string as YAML writes it: a text of lines in literal style.
func str(s string) *yaml.Node {
	n := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}
	if strings.Contains(strings.TrimSuffix(s, "\n"), "\n") || strings.HasSuffix(s, "\n") && len(s) > 1 {
		n.Style = yaml.LiteralStyle
	}
	return n
}

// valueNode is a JSON value as YAML, its keys in the order they were read: on one line when it
// is short enough (100 characters of JSON), else a block whose short parts are each on one line.
func valueNode(v any) *yaml.Node {
	n := &yaml.Node{}
	switch t := v.(type) {
	case *mapping:
		n.Kind = yaml.MappingNode
		for _, k := range t.keys {
			n.Content = append(n.Content, str(k), valueNode(t.vals[k]))
		}
	case []any:
		n.Kind = yaml.SequenceNode
		for _, e := range t {
			n.Content = append(n.Content, valueNode(e))
		}
	case string:
		return str(t)
	case nil:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}
	case bool:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(t)}
	case float64:
		tag := "!!float"
		if t == float64(int64(t)) {
			tag = "!!int"
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: number(t)}
	default:
		return str(show(t))
	}
	if one := show(v); len(one) <= 100 && !strings.Contains(one, `\n`) {
		n.Style = yaml.FlowStyle
	}
	return n
}

// mapStrings is a value with every string in it, at any depth, mapped.
func mapStrings(v any, f func(string) string) any {
	switch t := v.(type) {
	case string:
		return f(t)
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = mapStrings(e, f)
		}
		return out
	case *mapping:
		out := newMapping()
		for _, k := range t.keys {
			out.set(k, mapStrings(t.vals[k], f))
		}
		return out
	}
	return v
}
