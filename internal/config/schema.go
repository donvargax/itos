package config

import (
	"fmt"
	"sort"
	"strings"

	"github.com/donvargax/itos/internal/out"
	"github.com/donvargax/itos/internal/value"
)

// spec is the schema of one value, strict: an object's keys are the ones it
// lists, a map's are free (config.ts's Spec).
type spec struct {
	scalar   string // "string", "number", "boolean" or "strings"
	enum     []string
	object   []field // in the order the schema writes them
	required []string
	isObject bool
	mapOf    *spec
	list     *spec
	either   []*spec
}

type field struct {
	key  string
	spec *spec
}

var (
	str  = &spec{scalar: "string"}
	strs = &spec{scalar: "strings"}
	num  = &spec{scalar: "number"}
	boo  = &spec{scalar: "boolean"}
)

func enum(values ...string) *spec { return &spec{enum: values} }
func mapOf(s *spec) *spec         { return &spec{mapOf: s} }
func list(s *spec) *spec          { return &spec{list: s} }
func either(s ...*spec) *spec     { return &spec{either: s} }

// obj is an object of the keys given as key, spec, key, spec…, with the
// required ones.
func obj(required []string, pairs ...any) *spec {
	s := &spec{isObject: true, required: required}
	for i := 0; i+1 < len(pairs); i += 2 {
		s.object = append(s.object, field{pairs[i].(string), pairs[i+1].(*spec)})
	}
	return s
}

func (s *spec) field(key string) *spec {
	for _, f := range s.object {
		if f.key == key {
			return f.spec
		}
	}
	return nil
}

func (s *spec) keys() []string {
	keys := make([]string, len(s.object))
	for i, f := range s.object {
		keys[i] = f.key
	}
	return keys
}

var cost = enum("static", "late")

var step = either(str, obj(nil, "run", str, "tests", str, "whole", boo, "cost", cost, "tasks", enum("done")))

var typesOrAll = either(str, strs)

var provider = enum("github", "command", "none")

// HookManagers are the hook managers `hooks install` writes or prints for.
var HookManagers = []string{"vp", "git", "husky", "lefthook", "pre-commit", "prek"}

// schema is every key the config accepts, each one a tool reads.
var schema = obj([]string{"version"},
	"version", num,
	"requires", str,
	"shell", strs,
	"ledger", obj([]string{"files"},
		"files", str,
		"group", obj(nil, "label", str, "pattern", str, "numeric", boo),
		"id", str,
		"check", obj(nil, "timeout", num),
	),
	"commits", obj(nil,
		"types", strs,
		"header_lint", obj(nil, "hook", str, "stdin", str),
		"footers", mapOf(obj([]string{"source"},
			"source", either(str, obj([]string{"tests"}, "tests", str)),
			"strip_prefix", str,
			"required_for", typesOrAll,
			"validate_for", typesOrAll,
			"must_be_live", boo,
			"read_at", enum("commit", "worktree"),
		)),
		"path_sets", mapOf(strs),
		"scopes", mapOf(obj(nil, "only", strs, "never", strs, "must_touch", strs)),
		"reject_message", str,
		"since", str,
	),
	"tests", mapOf(obj(nil,
		// Built in (gherkin) or a command that speaks the adapter protocol.
		"adapter", either(str, obj([]string{"command"}, "command", str, "supports_at", boo)),
		"root", str,
		"id", str,
		"tag_prefix", str,
		"wip_tag", str,
		"run", obj(nil,
			"whole", str,
			"select", str,
			"ids_pattern", str,
			"join", obj(nil, "each", str, "sep", str),
		),
		"recognize", list(obj([]string{"command", "as"}, "command", str, "as", str)),
		"smoke", obj(nil, "file", str, "every_file", boo, "add_hint", str),
		"range_checks", list(obj([]string{"name"},
			"name", str,
			"except_types", strs,
			"staged", str,
			"range", str,
			"builtin", enum("moves"),
			"allowed_renames", mapOf(str),
		)),
	)),
	"ci", obj([]string{"steps"},
		"env", mapOf(str),
		"steps", list(step),
		"prose", obj([]string{"paths", "steps"}, "paths", strs, "steps", strs),
		"cost", obj(nil, "static", strs, "keep_written_order", boo),
		"covers", list(obj([]string{"by", "matches"}, "by", str, "matches", str)),
		"nightly_only", strs,
		"nightly", obj([]string{"steps"}, "steps", list(step)),
		"wait_on_status", strs,
		"stop_at_first_failure", boo,
		"range", obj(nil,
			"provider", provider,
			"command", str,
			"github", obj(nil, "workflow", str, "branch", str, "repository_env", str, "token_env", strs),
		),
	),
	"work", obj(nil,
		"registry", str,
		"groups_key", str,
		"statuses", strs,
		"people", obj([]string{"source", "file"},
			"source", enum("all-contributors-md", "all-contributorsrc", "yaml"),
			"file", str,
			"login_from", str,
		),
		"identity", obj(nil, "provider", provider, "command", str, "hint", str),
	),
	"hooks", obj(nil,
		"manager", enum(HookManagers...),
		"bin", str,
		"pre_push", obj([]string{"per_base", "whole"}, "per_base", str, "whole", str),
		"commit_msg", obj(nil, "task_checks", boo, "check_timeout", num),
	),
)

func wrongType(at, message, want string) out.Problem {
	return out.Problem{Rule: "config-type", Message: message, Fix: "make " + at + " " + want}
}

// problems is what is wrong with a value against a spec, each problem naming
// its key path.
func problems(v any, s *spec, path string) []out.Problem {
	switch {
	case s.scalar != "":
		at := path
		if at == "" {
			at = "the file"
		}
		return scalarProblems(v, s.scalar, at)
	case s.enum != nil:
		if value.Includes(s.enum, v) {
			return nil
		}
		values := strings.Join(s.enum, ", ")
		return []out.Problem{{
			Rule:    "config-enum",
			Message: fmt.Sprintf("%s should be one of %s, not %s", path, values, value.JSON(v)),
			Fix:     fmt.Sprintf("set %s to one of %s", path, values),
		}}
	case s.either != nil:
		// The specs of the value's shape are judged first, so a mapping with
		// a wrong value is told about that value, not that it is no string.
		var shaped []*spec
		for _, e := range s.either {
			if fits(v, e) {
				shaped = append(shaped, e)
			}
		}
		if len(shaped) == 0 {
			shaped = s.either
		}
		var best []out.Problem
		for i, e := range shaped {
			found := problems(v, e, path)
			if len(found) == 0 {
				return nil
			}
			if i == 0 || len(found) < len(best) {
				best = found
			}
		}
		return best
	case s.list != nil:
		items, ok := v.([]any)
		if !ok {
			return []out.Problem{wrongType(path, path+" should be a list", "a list")}
		}
		var found []out.Problem
		for i, item := range items {
			found = append(found, problems(item, s.list, fmt.Sprintf("%s[%d]", path, i))...)
		}
		return found
	}
	return mappingProblems(v, s, path)
}

// fits is whether a value has the shape a spec wants: a mapping for an object
// or a map, a list for a list, a scalar of the type for the rest.
func fits(v any, s *spec) bool {
	switch {
	case s.scalar == "strings" || s.list != nil:
		_, ok := v.([]any)
		return ok
	case s.scalar != "":
		return value.TypeOf(v) == s.scalar
	case s.enum != nil:
		_, ok := v.(string)
		return ok
	case s.either != nil:
		for _, e := range s.either {
			if fits(v, e) {
				return true
			}
		}
		return false
	}
	return value.IsMapping(v)
}

func scalarProblems(v any, kind, at string) []out.Problem {
	if kind == "strings" {
		items, ok := v.([]any)
		for _, item := range items {
			if _, isString := item.(string); !isString {
				ok = false
			}
		}
		if ok {
			return nil
		}
		return []out.Problem{wrongType(at, at+" should be a list of strings", "a list of strings")}
	}
	if value.TypeOf(v) == kind {
		return nil
	}
	return []out.Problem{wrongType(at, fmt.Sprintf("%s should be a %s, not %s", at, kind, value.TypeOf(v)), "a "+kind)}
}

// distance is the edit distance of two keys, for a misspelling's fix.
func distance(a, b string) int {
	ar, br := []rune(a), []rune(b)
	row := make([]int, len(br)+1)
	for j := range row {
		row[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		next := make([]int, len(br)+1)
		next[0] = i
		for j := 1; j <= len(br); j++ {
			change := 1
			if ar[i-1] == br[j-1] {
				change = 0
			}
			next[j] = min(row[j]+1, next[j-1]+1, row[j-1]+change)
		}
		row = next
	}
	return row[len(br)]
}

// unknownKey is an unknown key: renamed to the known key it misspells, else
// removed.
func unknownKey(at, key string, known []string) out.Problem {
	type near struct {
		name string
		d    int
	}
	var close []near
	for _, name := range known {
		if d := distance(key, name); d <= 2 {
			close = append(close, near{name, d})
		}
	}
	sort.SliceStable(close, func(i, j int) bool { return close[i].d < close[j].d })
	fix := fmt.Sprintf("remove %s; the keys here are %s", at, strings.Join(known, ", "))
	if len(close) > 0 {
		fix = fmt.Sprintf("rename %s to %s", at, close[0].name)
	}
	return out.Problem{Rule: "config-unknown-key", Message: "unknown key " + at, Fix: fix}
}

func mappingProblems(v any, s *spec, path string) []out.Problem {
	m, ok := v.(*value.Map)
	if !ok {
		at := path
		if at == "" {
			at = "the file"
		}
		return []out.Problem{wrongType(at, fmt.Sprintf("%s should be a mapping, not %s", at, value.TypeOf(v)), "a mapping")}
	}
	key := func(k string) string {
		if path == "" {
			return k
		}
		return path + "." + k
	}
	var found []out.Problem
	for _, k := range m.Keys() {
		switch {
		case s.mapOf != nil:
			found = append(found, problems(m.At(k), s.mapOf, key(k))...)
		case s.field(k) != nil:
			found = append(found, problems(m.At(k), s.field(k), key(k))...)
		default:
			found = append(found, unknownKey(key(k), k, s.keys()))
		}
	}
	for _, k := range s.required {
		if !m.Has(k) {
			found = append(found, out.Problem{Rule: "config-missing-key", Message: key(k) + " is missing", Fix: "add " + key(k)})
		}
	}
	return found
}
