// Package config finds and reads itos.yaml, the one policy file: the file
// --config or ITOS_CONFIG names, else itos.yaml in the folder itos runs in
// (--root changes that folder first).
//
// What it reads so far is what the scaffold's one command needs, version and
// requires, with tools/itos/config.ts's wording for what is wrong with them: a
// file that cannot be read or parsed, one that is not a mapping, a version
// that is missing, not a number or not 1, a requires that is not a string.
// The rest of the schema (the unknown keys, every section), the defaults and
// the cross-checks are the config group's (PLAN.md, phase 2, step 2), which
// replaces Load's narrow reading with the whole one.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/donvargax/itos/internal/out"
	"go.yaml.in/yaml/v3"
)

// Error is a config that cannot be used: exit 2, each problem printed as
// "FAIL <file>: <message>".
type Error struct {
	File     string
	Problems []out.Problem
}

func (e *Error) Error() string {
	messages := make([]string, len(e.Problems))
	for i, p := range e.Problems {
		messages[i] = p.Message
	}
	return e.File + ": " + strings.Join(messages, "; ")
}

// Config is the part of the file read so far.
type Config struct {
	// Requires is the config's requires, "" when it has none.
	Requires    string
	HasRequires bool
}

// Path is the config's path: ITOS_CONFIG, which --config sets, else itos.yaml.
func Path() string {
	if p := os.Getenv("ITOS_CONFIG"); p != "" {
		return p
	}
	return "itos.yaml"
}

// kind names a value's type as the TypeScript's messages do (JavaScript's
// typeof, with a list and null told apart).
func kind(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case []any:
		return "a list"
	case string:
		return "string"
	case bool:
		return "boolean"
	case int, int64, uint64, float64:
		return "number"
	default:
		return "object"
	}
}

func number(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float64:
		return n, true
	}
	return 0, false
}

// mapping is the file's top level as a mapping of string keys, as JavaScript
// reads YAML's.
func mapping(v any) (map[string]any, bool) {
	switch m := v.(type) {
	case map[string]any:
		return m, true
	case map[any]any:
		keyed := make(map[string]any, len(m))
		for k, value := range m {
			keyed[fmt.Sprint(k)] = value
		}
		return keyed, true
	}
	return nil, false
}

func wrongType(at, want string, v any) out.Problem {
	return out.Problem{
		Rule:    "config-type",
		Message: fmt.Sprintf("%s should be a %s, not %s", at, want, kind(v)),
		Fix:     fmt.Sprintf("make %s a %s", at, want),
	}
}

// Load reads the config at file.
func Load(file string) (*Config, error) {
	text, err := os.ReadFile(file)
	var raw any
	if err == nil {
		err = yaml.Unmarshal(text, &raw)
	}
	if err != nil {
		return nil, &Error{File: file, Problems: []out.Problem{{
			Rule:    "config-unreadable",
			Message: "cannot be read: " + err.Error(),
			Fix:     fmt.Sprintf("create %s, or correct its YAML", file),
		}}}
	}
	top, ok := mapping(raw)
	if !ok {
		return nil, &Error{File: file, Problems: []out.Problem{{
			Rule:    "config-type",
			Message: "the file should be a mapping, not " + kind(raw),
			Fix:     "make the file a mapping",
		}}}
	}
	var found []out.Problem
	version, hasVersion := top["version"]
	n, isNumber := number(version)
	switch {
	case !hasVersion:
		found = append(found, out.Problem{Rule: "config-missing-key", Message: "version is missing", Fix: "add version"})
	case !isNumber:
		found = append(found, wrongType("version", "number", version))
	}
	requires, hasRequires := top["requires"]
	requiresText, isString := requires.(string)
	if hasRequires && !isString {
		found = append(found, wrongType("requires", "string", requires))
	}
	if len(found) > 0 {
		return nil, &Error{File: file, Problems: found}
	}
	if n != 1 {
		return nil, &Error{File: file, Problems: []out.Problem{{
			Rule:    "config-version",
			Message: fmt.Sprintf("version %s is not 1", strconv.FormatFloat(n, 'f', -1, 64)),
			Fix:     "set version: 1",
		}}}
	}
	return &Config{Requires: requiresText, HasRequires: hasRequires}, nil
}
