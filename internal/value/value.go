// Package value is itos's data as the TypeScript reads it: YAML parsed by the
// YAML 1.2 core schema (the `yaml` package's default) into JavaScript's
// values, mappings keeping JavaScript's key order, and the renderings the
// messages quote: a value in a template literal (String), its typeof as the
// config's messages name it (TypeOf), JSON.stringify (JSON) and the YAML the
// `yaml` package writes (YAML).
//
// A YAML value is one of: nil (null), Undefined (a key that is not there),
// bool, float64 (every number, as JavaScript has one), string, []any and
// *Map. go.yaml.in/yaml/v3 parses the text; this package resolves each plain
// scalar itself, since yaml/v3 also reads YAML 1.1's forms (017 as octal,
// 1_000, 0b101) where the core schema reads a decimal or a string.
package value

import (
	"errors"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// undefined is JavaScript's undefined: what a mapping gives for a key it does
// not have.
type undefined struct{}

// Undefined is a key that is not there.
var Undefined any = undefined{}

// Map is a YAML mapping as JavaScript holds it: string keys, read in
// JavaScript's order (keys that are array indices first, ascending, then the
// rest as written).
type Map struct {
	order []string
	vals  map[string]any
}

// NewMap is an empty mapping; pairs are key, value, key, value….
func NewMap(pairs ...any) *Map {
	m := &Map{vals: map[string]any{}}
	for i := 0; i+1 < len(pairs); i += 2 {
		m.Set(pairs[i].(string), pairs[i+1])
	}
	return m
}

// Set gives a key its value, keeping its place if it has one.
func (m *Map) Set(key string, v any) {
	if _, ok := m.vals[key]; !ok {
		m.order = append(m.order, key)
	}
	m.vals[key] = v
}

// Delete removes a key.
func (m *Map) Delete(key string) {
	if _, ok := m.vals[key]; !ok {
		return
	}
	delete(m.vals, key)
	for i, k := range m.order {
		if k == key {
			m.order = append(m.order[:i:i], m.order[i+1:]...)
			break
		}
	}
}

// Has is whether the mapping has the key (JavaScript's `in`).
func (m *Map) Has(key string) bool {
	if m == nil {
		return false
	}
	_, ok := m.vals[key]
	return ok
}

// At is the key's value, Undefined when the mapping does not have it.
func (m *Map) At(key string) any {
	if m == nil {
		return Undefined
	}
	if v, ok := m.vals[key]; ok {
		return v
	}
	return Undefined
}

// Len is the number of keys.
func (m *Map) Len() int {
	if m == nil {
		return 0
	}
	return len(m.order)
}

// arrayIndex is whether a key is one JavaScript orders first: an array index.
func arrayIndex(key string) (uint64, bool) {
	if key == "" || (len(key) > 1 && key[0] == '0') {
		return 0, false
	}
	n, err := strconv.ParseUint(key, 10, 32)
	return n, err == nil && n < math.MaxUint32
}

// Keys are the keys in JavaScript's order (Object.keys).
func (m *Map) Keys() []string {
	if m == nil {
		return nil
	}
	var indices, rest []string
	for _, k := range m.order {
		if _, ok := arrayIndex(k); ok {
			indices = append(indices, k)
		} else {
			rest = append(rest, k)
		}
	}
	sort.SliceStable(indices, func(i, j int) bool {
		a, _ := arrayIndex(indices[i])
		b, _ := arrayIndex(indices[j])
		return a < b
	})
	return append(indices, rest...)
}

// Copy is a deep copy (structuredClone).
func Copy(v any) any {
	switch t := v.(type) {
	case *Map:
		c := NewMap()
		for _, k := range t.order {
			c.Set(k, Copy(t.vals[k]))
		}
		return c
	case []any:
		c := make([]any, len(t))
		for i, e := range t {
			c[i] = Copy(e)
		}
		return c
	}
	return v
}

// MarshalJSON writes the mapping as JSON.stringify does, keys in order.
func (m *Map) MarshalJSON() ([]byte, error) { return []byte(JSON(m)), nil }

// Prop is a property of a value, as JavaScript reads `v.key`: a mapping's
// value, Undefined for anything else.
func Prop(v any, key string) any {
	if m, ok := v.(*Map); ok {
		return m.At(key)
	}
	return Undefined
}

// ErrMultipleDocuments is the `yaml` package's refusal of a stream of
// several documents.
var ErrMultipleDocuments = errors.New("Source contains multiple documents; please use YAML.parseAllDocuments()")

// Parse reads YAML text as the `yaml` package's parse does: null for an empty
// document, an error for a syntax error, several documents or a key written
// twice in one mapping.
func Parse(text string) (any, error) {
	dec := yaml.NewDecoder(strings.NewReader(text))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, err
	}
	var more yaml.Node
	if err := dec.Decode(&more); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, err
		}
		return nil, ErrMultipleDocuments
	}
	return convert(&doc)
}

func convert(n *yaml.Node) (any, error) {
	switch n.Kind {
	case yaml.DocumentNode:
		if len(n.Content) == 0 {
			return nil, nil
		}
		return convert(n.Content[0])
	case yaml.AliasNode:
		return convert(n.Alias)
	case yaml.SequenceNode:
		list := make([]any, 0, len(n.Content))
		for _, c := range n.Content {
			v, err := convert(c)
			if err != nil {
				return nil, err
			}
			list = append(list, v)
		}
		return list, nil
	case yaml.MappingNode:
		m := NewMap()
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, err := convert(n.Content[i])
			if err != nil {
				return nil, err
			}
			key := mapKey(k)
			if m.Has(key) {
				return nil, errors.New("Map keys must be unique at line " + strconv.Itoa(n.Content[i].Line) + ", column " + strconv.Itoa(n.Content[i].Column))
			}
			v, err := convert(n.Content[i+1])
			if err != nil {
				return nil, err
			}
			m.Set(key, v)
		}
		return m, nil
	case yaml.ScalarNode:
		return scalar(n), nil
	}
	return nil, nil
}

// mapKey is a key as the `yaml` package makes it a property name: null is
// "", a scalar its String, anything else its JSON.
func mapKey(k any) string {
	switch k.(type) {
	case nil:
		return ""
	case *Map, []any:
		return JSON(k)
	}
	return String(k)
}

const quotedStyles = yaml.DoubleQuotedStyle | yaml.SingleQuotedStyle | yaml.LiteralStyle | yaml.FoldedStyle

func scalar(n *yaml.Node) any {
	if n.Style&yaml.TaggedStyle != 0 {
		switch n.ShortTag() {
		case "!!str", "!!binary", "!!timestamp":
			return n.Value
		case "!!null":
			return nil
		}
	}
	if n.Style&quotedStyles != 0 {
		return n.Value
	}
	return Resolve(n.Value)
}

var (
	coreInt   = regexp.MustCompile(`^[-+]?[0-9]+$`)
	coreOct   = regexp.MustCompile(`^0o[0-7]+$`)
	coreHex   = regexp.MustCompile(`^0x[0-9a-fA-F]+$`)
	coreInf   = regexp.MustCompile(`^[-+]?\.(?:inf|Inf|INF)$`)
	coreNaN   = regexp.MustCompile(`^\.(?:nan|NaN|NAN)$`)
	coreExp   = regexp.MustCompile(`^[-+]?(?:\.[0-9]+|[0-9]+(?:\.[0-9]*)?)[eE][-+]?[0-9]+$`)
	coreFloat = regexp.MustCompile(`^[-+]?(?:\.[0-9]+|[0-9]+\.[0-9]*)$`)
)

// Resolve is a plain scalar's value by the core schema: null, a boolean, a
// number, else the text.
func Resolve(s string) any {
	switch s {
	case "", "~", "null", "Null", "NULL":
		return nil
	case "true", "True", "TRUE":
		return true
	case "false", "False", "FALSE":
		return false
	}
	switch {
	case coreOct.MatchString(s):
		n, _ := strconv.ParseUint(s[2:], 8, 64)
		return float64(n)
	case coreHex.MatchString(s):
		if n, err := strconv.ParseUint(s[2:], 16, 64); err == nil {
			return float64(n)
		}
		f, _ := strconv.ParseFloat(s, 64)
		return f
	case coreInf.MatchString(s):
		if s[0] == '-' {
			return math.Inf(-1)
		}
		return math.Inf(1)
	case coreNaN.MatchString(s):
		return math.NaN()
	case coreInt.MatchString(s), coreExp.MatchString(s), coreFloat.MatchString(s):
		f, _ := strconv.ParseFloat(s, 64)
		return f
	}
	return s
}

// Number is a number as JavaScript writes it.
func Number(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	case f == 0:
		return "0"
	}
	if a := math.Abs(f); a >= 1e-6 && a < 1e21 {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	s := strconv.FormatFloat(f, 'e', -1, 64)
	mantissa, exp, _ := strings.Cut(s, "e")
	sign := exp[:1]
	exp = strings.TrimLeft(exp[1:], "0")
	return mantissa + "e" + sign + exp
}

// String is a value as a template literal writes it.
func String(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case undefined:
		return "undefined"
	case bool:
		return strconv.FormatBool(t)
	case float64:
		return Number(t)
	case string:
		return t
	case []any:
		parts := make([]string, len(t))
		for i, e := range t {
			if e != nil && e != Undefined {
				parts[i] = String(e)
			}
		}
		return strings.Join(parts, ",")
	}
	return "[object Object]"
}

// TypeOf is a value's type as the config's messages name it: JavaScript's
// typeof, with a list and null told apart.
func TypeOf(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case undefined:
		return "undefined"
	case []any:
		return "a list"
	case string:
		return "string"
	case bool:
		return "boolean"
	case float64:
		return "number"
	}
	return "object"
}

// IsMapping is whether a value is a mapping (an object that is not a list).
func IsMapping(v any) bool {
	_, ok := v.(*Map)
	return ok
}

// Truthy is whether JavaScript reads a value as true.
func Truthy(v any) bool {
	switch t := v.(type) {
	case nil, undefined:
		return false
	case bool:
		return t
	case float64:
		return t != 0 && !math.IsNaN(t)
	case string:
		return t != ""
	}
	return true
}

// Includes is whether a list of strings holds the value (Array.includes).
func Includes(list []string, v any) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	for _, e := range list {
		if e == s {
			return true
		}
	}
	return false
}

// Keys are a value's own keys as Object.keys gives them: a mapping's, a
// list's or a string's indices, none for anything else.
func Keys(v any) []string {
	switch t := v.(type) {
	case *Map:
		return t.Keys()
	case []any:
		keys := make([]string, len(t))
		for i := range t {
			keys[i] = strconv.Itoa(i)
		}
		return keys
	case string:
		n := 0
		for _, r := range t {
			n++
			if r > 0xFFFF {
				n++
			}
		}
		keys := make([]string, n)
		for i := range keys {
			keys[i] = strconv.Itoa(i)
		}
		return keys
	}
	return nil
}

// JSON is a value as JSON.stringify writes it, on one line.
func JSON(v any) string {
	var b strings.Builder
	writeJSON(&b, v)
	return b.String()
}

func writeJSON(b *strings.Builder, v any) {
	switch t := v.(type) {
	case nil, undefined:
		b.WriteString("null")
	case bool:
		b.WriteString(strconv.FormatBool(t))
	case float64:
		if math.IsNaN(t) || math.IsInf(t, 0) {
			b.WriteString("null")
		} else {
			b.WriteString(Number(t))
		}
	case string:
		b.WriteString(Quote(t))
	case []any:
		b.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				b.WriteByte(',')
			}
			writeJSON(b, e)
		}
		b.WriteByte(']')
	case *Map:
		b.WriteByte('{')
		first := true
		for _, k := range t.Keys() {
			e := t.vals[k]
			if e == Undefined {
				continue
			}
			if !first {
				b.WriteByte(',')
			}
			first = false
			b.WriteString(Quote(k))
			b.WriteByte(':')
			writeJSON(b, e)
		}
		b.WriteByte('}')
	default:
		b.WriteString("null")
	}
}

// Quote is a string as JSON.stringify quotes it.
func Quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				b.WriteString(`\u00`)
				b.WriteString(strconv.FormatInt(int64(r)>>4, 16))
				b.WriteString(strconv.FormatInt(int64(r)&15, 16))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// SpaceChars are JavaScript's \s, written to go inside a character class:
// RE2's \s is ASCII alone.
const SpaceChars = `\t\n\v\f\r \x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}`

// Space is JavaScript's \s as a character class.
const Space = "[" + SpaceChars + "]"

var spaces = regexp.MustCompile(Space + `+`)

func isSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

// Trim is a string without the space at either end (String.trim).
func Trim(s string) string { return strings.TrimFunc(s, isSpace) }

// Fields is a string split at each run of space (split(/\s+/)): a space at
// either end gives an empty field there.
func Fields(s string) []string { return spaces.Split(s, -1) }

// Collapse is a string with each run of space made one space.
func Collapse(s string) string { return spaces.ReplaceAllString(s, " ") }
