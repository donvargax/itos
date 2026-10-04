// Package out is how a command reports under --json (itos --help): one object
// on stdout, "schema": 1 first and its keys in the order written, indented as
// tools/itos/problem.ts's emit prints it, and the problems a check reports,
// each a sentence with a stable rule id and, where one exists, a fix.
package out

import (
	"bytes"
	"encoding/json"
	"io"
)

// Problem is one thing a check found wrong: the sentence the text output
// prints, and for --json its rule id and the fix an agent can act on.
type Problem struct {
	Rule    string `json:"rule"`
	Message string `json:"message"`
	Fix     string `json:"fix,omitempty"`
}

// Field is one key of a JSON object, written in the order given: Go's maps
// would sort the keys, and the output's order is part of what is quoted.
type Field struct {
	Key   string
	Value any
}

// encode writes a value as JSON, leaving <, > and & as they are, as
// JavaScript's JSON.stringify does.
func encode(buf *bytes.Buffer, v any) error {
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return err
	}
	buf.Truncate(buf.Len() - 1) // Encode's newline
	return nil
}

// Emit prints one object, "schema": 1 first, then the fields in order,
// indented by two spaces and ended by a newline. A key given twice keeps its
// first place and takes its last value, as `{ schema: 1, ...value }` does,
// so an object printed as an adapter gave it cannot write a key twice.
func Emit(w io.Writer, fields ...Field) error {
	all := []Field{{"schema", 1}}
	at := map[string]int{"schema": 0}
	for _, f := range fields {
		if i, seen := at[f.Key]; seen {
			all[i].Value = f.Value
			continue
		}
		at[f.Key] = len(all)
		all = append(all, f)
	}
	var raw bytes.Buffer
	raw.WriteByte('{')
	for i, f := range all {
		if i > 0 {
			raw.WriteByte(',')
		}
		if err := encode(&raw, f.Key); err != nil {
			return err
		}
		raw.WriteByte(':')
		if err := encode(&raw, f.Value); err != nil {
			return err
		}
	}
	raw.WriteByte('}')
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, raw.Bytes(), "", "  "); err != nil {
		return err
	}
	pretty.WriteByte('\n')
	_, err := w.Write(pretty.Bytes())
	return err
}
