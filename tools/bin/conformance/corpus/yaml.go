package corpus

import (
	"bytes"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// mapping is a YAML mapping, its keys in the order they were written, as the TypeScript
// runner's objects kept them: a fixture's files are written in that order.
type mapping struct {
	keys []string
	vals map[string]any
}

func newMapping() *mapping { return &mapping{vals: map[string]any{}} }

func (m *mapping) get(key string) (any, bool) {
	v, ok := m.vals[key]
	return v, ok
}

// set gives key its value, where it stands if it is there already, else last.
func (m *mapping) set(key string, v any) {
	if _, ok := m.vals[key]; !ok {
		m.keys = append(m.keys, key)
	}
	m.vals[key] = v
}

func (m *mapping) del(key string) {
	if _, ok := m.vals[key]; !ok {
		return
	}
	delete(m.vals, key)
	for i, k := range m.keys {
		if k == key {
			m.keys = append(m.keys[:i:i], m.keys[i+1:]...)
			return
		}
	}
}

// parseYAML reads a YAML text as YAML 1.2's core schema does (decode). A fixture text that
// go.yaml.in/yaml/v3 refuses is read once more with its flow sequences' question marks quoted
// (quoteFlowQuestions), so a release's corpus is read as it stands; anything else it refuses is
// refused, with what it said of the text as written.
func parseYAML(text []byte, relax bool) (any, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(text, &doc); err != nil {
		if !relax {
			return nil, err
		}
		relaxed, changed := quoteFlowQuestions(text)
		if !changed || yaml.Unmarshal(relaxed, &doc) != nil {
			return nil, err
		}
	}
	return decode(&doc)
}

// flowSequence is a flow sequence on one line holding no other flow collection, no quote, no
// comment and no escape: [decision, add, Which way?].
var flowSequence = regexp.MustCompile(`\[[^\[\]{}"'#\\\n]*\]`)

// quoteFlowQuestions quotes each plain item of a one-line flow sequence that holds a question
// mark, and says whether it quoted one.
//
// YAML 1.2 reads `[decision, add, Which way?]` as three plain strings, the last "Which way?",
// and so did the yaml package the TypeScript runner parsed fixtures with; go.yaml.in/yaml/v3
// keeps libyaml's YAML 1.1 rule, under which a ? ends a plain scalar in a flow collection, and
// refuses the line. v8.0.0's corpus has three such lines (ask.yaml), quoted in this tree since
// (78504a74), and a release's tree is never rewritten, so previous-release reads them as they
// stand. Quoting an item that holds no quote, backslash or flow indicator gives it the value
// YAML 1.2 gives it plain, so this changes nothing a YAML 1.2 reader reads.
func quoteFlowQuestions(text []byte) ([]byte, bool) {
	changed := false
	out := flowSequence.ReplaceAllFunc(text, func(seq []byte) []byte {
		if !bytes.ContainsRune(seq, '?') {
			return seq
		}
		items := strings.Split(string(seq[1:len(seq)-1]), ",")
		for i, item := range items {
			trimmed := strings.TrimSpace(item)
			if !strings.Contains(trimmed, "?") || strings.HasPrefix(trimmed, "? ") || trimmed == "?" {
				continue
			}
			at := strings.Index(item, trimmed)
			items[i] = item[:at] + `"` + trimmed + `"` + item[at+len(trimmed):]
			changed = true
		}
		return []byte("[" + strings.Join(items, ",") + "]")
	})
	return out, changed
}

// decode is a YAML node's value as YAML 1.2's core schema reads it, as the TypeScript runner's
// parser did: a mapping a *mapping, a sequence a []any, a scalar a string, a float64 (every
// number, as JavaScript's), a bool or nil. A timestamp is a string, never a time.Time, and a
// merge key (<<) a key like any other.
func decode(n *yaml.Node) (any, error) {
	switch n.Kind {
	case 0:
		return nil, nil
	case yaml.DocumentNode:
		if len(n.Content) == 0 {
			return nil, nil
		}
		return decode(n.Content[0])
	case yaml.AliasNode:
		return decode(n.Alias)
	case yaml.SequenceNode:
		list := make([]any, 0, len(n.Content))
		for _, c := range n.Content {
			v, err := decode(c)
			if err != nil {
				return nil, err
			}
			list = append(list, v)
		}
		return list, nil
	case yaml.MappingNode:
		m := newMapping()
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			for k.Kind == yaml.AliasNode {
				k = k.Alias
			}
			if k.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("line %d: a mapping's key is not a scalar", k.Line)
			}
			if _, twice := m.get(k.Value); twice {
				return nil, fmt.Errorf("line %d: map keys must be unique (%s)", k.Line, k.Value)
			}
			val, err := decode(v)
			if err != nil {
				return nil, err
			}
			m.set(k.Value, val)
		}
		return m, nil
	case yaml.ScalarNode:
		return scalar(n), nil
	}
	return nil, fmt.Errorf("line %d: a YAML node of an unknown kind", n.Line)
}

var (
	coreNull  = regexp.MustCompile(`^(?:~|null|Null|NULL)?$`)
	coreTrue  = regexp.MustCompile(`^(?:true|True|TRUE)$`)
	coreFalse = regexp.MustCompile(`^(?:false|False|FALSE)$`)
	coreInt   = regexp.MustCompile(`^[-+]?[0-9]+$`)
	coreOct   = regexp.MustCompile(`^0o[0-7]+$`)
	coreHex   = regexp.MustCompile(`^0x[0-9a-fA-F]+$`)
	coreFloat = regexp.MustCompile(`^[-+]?(?:\.[0-9]+|[0-9]+(?:\.[0-9]*)?)(?:[eE][-+]?[0-9]+)?$`)
	coreInf   = regexp.MustCompile(`^([-+]?)\.(?:inf|Inf|INF)$`)
	coreNaN   = regexp.MustCompile(`^\.(?:nan|NaN|NAN)$`)
)

// scalar is a scalar's value: a quoted or block scalar, or one tagged !!str or with a tag the core
// schema does not know, is its text; a plain one, or one tagged as a core type, is resolved by
// the core schema's patterns.
func scalar(n *yaml.Node) any {
	if n.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle|yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		return n.Value
	}
	if n.Style&yaml.TaggedStyle != 0 {
		switch n.Tag {
		case "!!null":
			return nil
		case "!!bool", "!!int", "!!float":
		default:
			return n.Value
		}
	}
	return resolve(n.Value)
}

// resolve is a plain scalar's value under the core schema.
func resolve(s string) any {
	switch {
	case coreNull.MatchString(s):
		return nil
	case coreTrue.MatchString(s):
		return true
	case coreFalse.MatchString(s):
		return false
	case coreOct.MatchString(s):
		if v, err := strconv.ParseUint(s[2:], 8, 64); err == nil {
			return float64(v)
		}
	case coreHex.MatchString(s):
		if v, err := strconv.ParseUint(s[2:], 16, 64); err == nil {
			return float64(v)
		}
	case coreInt.MatchString(s), coreFloat.MatchString(s):
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			return v
		}
	case coreNaN.MatchString(s):
		return math.NaN()
	}
	if m := coreInf.FindStringSubmatch(s); m != nil {
		if m[1] == "-" {
			return math.Inf(-1)
		}
		return math.Inf(1)
	}
	return s
}

// plain is a value with its mappings as map[string]any, as encoding/json reads and writes them.
func plain(v any) any {
	switch t := v.(type) {
	case *mapping:
		m := make(map[string]any, len(t.keys))
		for _, k := range t.keys {
			m[k] = plain(t.vals[k])
		}
		return m
	case []any:
		list := make([]any, len(t))
		for i, e := range t {
			list[i] = plain(e)
		}
		return list
	}
	return v
}
