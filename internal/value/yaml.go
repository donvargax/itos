package value

import (
	"regexp"
	"strings"
)

// YAML is a value as the `yaml` package's stringify writes it, in block
// style with two spaces of indent: what `config check --print-defaults`
// prints. It writes what the table of defaults holds (mappings, lists,
// strings, numbers, booleans); a string is quoted where the `yaml` package
// quotes it, and folding a line longer than 80 columns is left out, since no
// default is that long.
func YAML(v any) string {
	var b strings.Builder
	writeYAML(&b, v, "")
	return b.String()
}

func writeYAML(b *strings.Builder, v any, indent string) {
	switch t := v.(type) {
	case *Map:
		if t.Len() == 0 {
			b.WriteString(indent + "{}\n")
			return
		}
		for _, k := range t.Keys() {
			b.WriteString(indent + yamlString(k, true) + ":")
			writeNested(b, t.vals[k], indent)
		}
	case []any:
		if len(t) == 0 {
			b.WriteString(indent + "[]\n")
			return
		}
		for _, e := range t {
			b.WriteString(indent + "-")
			writeNested(b, e, indent)
		}
	default:
		b.WriteString(indent + yamlScalar(v) + "\n")
	}
}

// writeNested writes a value after its key or its dash: a scalar or an empty
// collection on the same line, a collection below it, indented.
func writeNested(b *strings.Builder, v any, indent string) {
	switch t := v.(type) {
	case *Map:
		if t.Len() > 0 {
			b.WriteString("\n")
			writeYAML(b, t, indent+"  ")
			return
		}
		b.WriteString(" {}\n")
	case []any:
		if len(t) > 0 {
			b.WriteString("\n")
			writeYAML(b, t, indent+"  ")
			return
		}
		b.WriteString(" []\n")
	default:
		b.WriteString(" " + yamlScalar(v) + "\n")
	}
}

func yamlScalar(v any) string {
	switch t := v.(type) {
	case string:
		return yamlString(t, false)
	case nil, undefined:
		return "null"
	case float64:
		switch s := Number(t); s {
		case "NaN":
			return ".nan"
		case "Infinity":
			return ".inf"
		case "-Infinity":
			return "-.inf"
		default:
			return s
		}
	}
	return String(v)
}

// notPlain is what keeps a string from being written plain (stringifyString.js).
var notPlain = regexp.MustCompile("^[\\n\\t ,\\[\\]{}#&*!|>'\"%@`]|^[?-]$|^[?-][ \\t]|[\\n:][ \\t]|[ \\t]\\n|[\\n\\t ]#|[\\n\\t :]$")

var documentMarker = regexp.MustCompile(`(?m)^(%|---|\.\.\.)`)

// yamlString is a string plain where the `yaml` package writes it plain, else
// quoted as it quotes it.
func yamlString(s string, key bool) string {
	quote := s == "" || notPlain.MatchString(s) || (key && strings.Contains(s, "\n")) ||
		(key && documentMarker.MatchString(s))
	if _, isString := Resolve(s).(string); !isString {
		quote = true
	}
	if !quote {
		return s
	}
	hasDouble, hasSingle := strings.Contains(s, `"`), strings.Contains(s, "'")
	if hasDouble && !hasSingle && !strings.Contains(s, "\n") {
		return "'" + strings.ReplaceAll(s, "'", "''") + "'"
	}
	return Quote(s)
}
