package config

import (
	"fmt"
	"sort"
	"strings"

	"github.com/donvargax/itos/v5/internal/out"
	"github.com/donvargax/itos/v5/internal/value"
)

// spec is the schema of one value, strict: an object's keys are the ones it
// lists, a map's are free (config.ts's Spec). desc is what the value is for,
// in words: the description the config's JSON Schema gives it (Schema), which
// config check never reads.
type spec struct {
	scalar   string // "string", "number", "boolean" or "strings"
	enum     []string
	object   []field // in the order the schema writes them
	required []string
	isObject bool
	mapOf    *spec
	list     *spec
	either   []*spec
	desc     string
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

// about is a spec with words for what it is for; the spec it copies, which
// may be shared (str, num…), is left as it was.
func about(desc string, s *spec) *spec {
	described := *s
	described.desc = desc
	return &described
}

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

var cost = about("static (seconds, run first) or late.", enum("static", "late"))

var step = about(
	"A command, or a mapping that runs a command (run), a kind's tests (tests) or, in the nightly "+
		"alone, the checks of every done task (tasks: done).",
	either(str, obj(nil,
		"run", about("The command the step runs.", str),
		"tests", about("The kind of named tests the step runs, by its run.whole.", str),
		"whole", about("Whether the step runs every test of its kind, rather than the ones a range names.", boo),
		"cost", cost,
		"tasks", about("done: the checks of every task whose work item is done (a nightly step).", enum("done")),
	)),
)

var typesOrAll = either(str, strs)

var provider = enum("github", "none")

// removedKeys are the keys v5.0.0 removed (slice 85), each with what to do
// instead: config check refuses one as removed, not as a key it never knew,
// so a config written for v4 is told what changed. The command providers ran
// a repository's own commands without anyone choosing to (the git shim's push
// ran ci.watch.command, itos go and itos status the rest), and nothing has
// read ci.range.github.branch since bug 29 counts a green run on any branch.
var removedKeys = map[string]string{
	"ci.range.command":         "remove ci.range.command, and set ci.range.provider to github or none",
	"ci.range.github.branch":   "remove ci.range.github.branch: a commit's green run counts on any branch",
	"ci.watch.command":         "remove ci.watch.command, and set ci.watch.provider to github or none",
	"ci.watch.nightly_command": "remove ci.watch.nightly_command; ci.watch.github.nightly_workflow names the nightly",
	"work.identity.command":    "remove work.identity.command, set work.identity.provider to github or none, and pass --as <handle>",
}

// RemovedKey is what to do instead of a key v5.0.0 removed, and whether it
// is one, for config get, which refuses it as removed.
func RemovedKey(key string) (string, bool) {
	fix, ok := removedKeys[key]
	return fix, ok
}

// removedValues are the values v5.0.0 removed from a key, by the key, each
// with what to do instead.
var removedValues = map[string]map[string]string{
	"ci.range.provider":      {"command": "set ci.range.provider to github or none"},
	"ci.watch.provider":      {"command": "set ci.watch.provider to github or none"},
	"work.identity.provider": {"command": "set work.identity.provider to github or none, and pass --as <handle> where it ran"},
}

// removed is the problem of a key or a value v5.0.0 removed, its message
// saying what to do instead, since config check prints the message alone.
func removed(what, fix string) out.Problem {
	return out.Problem{Rule: "config-removed", Message: what + " was removed in v5; " + fix, Fix: fix}
}

// schema is every key the config accepts, each one a tool reads.
var schema = about("itos's policy: the ledger, the commit rules, the named tests, CI's plan, the work routing and the hooks.", obj([]string{"version"},
	"version", about("The config's format: 1.", num),
	"requires", about("The oldest itos that reads this file, as a version range (>=0.6.0); itos version --check holds the running itos to it.", str),
	"pin", about("The one itos release this repository runs: a global itos fetches it into its cache, checks it and runs it "+
		"in its own place (ITOS_VERSION overrides it).", obj([]string{"version", "checksums"},
		"version", about("The release's version, x.y.z, without its v.", str),
		"checksums", about("The SHA-256 of that release's checksums.txt, 64 hex digits, which the launcher checks it against "+
			"before trusting the archive it lists.", str),
	)),
	"shell", about("The argv prefix every command runs under.", strs),
	"ledger", about("The ledger: the task files, the group in their names, the ID pattern and the checks' timeout.", obj([]string{"files"},
		"files", about("The ledger's files, {group} standing where a file's group is (tasks/phase-{group}.yaml).", str),
		"group", about("The group in the files' names.", obj(nil,
			"label", about("What a group is called in what the tools print, and the flag itos task takes one by beside --group.", str),
			"pattern", about("A regular expression a group matches.", str),
			"numeric", about("Whether groups sort as numbers.", boo),
		)),
		"id", about("A regular expression every task ID matches.", str),
		"check", about("How a task's checks run.", obj(nil, "timeout", about("The seconds a check may run.", num))),
	)),
	"commits", about("The commit rules: the types, the header lint, the footers, the paths each type may touch.", obj(nil,
		"types", about("The commit types a header may have.", strs),
		"header_lint", about("The header lint, itos's own or a delegate, which itos runs the footer rules beside.", obj(nil,
			"use", about("builtin: itos lints the header itself, by config-conventional's rules with commits.types; "+
				"command: the delegate hook and stdin name, as with no use.", enum("builtin", "command")),
			"hook", about("The delegate's command that lints the message file, {file} standing for its path.", str),
			"stdin", about("The delegate's command that lints a message on its stdin.", str),
		)),
		"footers", about("The footers, by key: where the IDs each names come from, or free text, and which types need it.", mapOf(obj([]string{"source"},
			"source", about("Where the footer's IDs come from: ledger, registry (the work registry's items), or { tests: <kind> }; or "+
				"text, free text that says what a consumer must do, or none.", either(str, obj([]string{"tests"}, "tests", about("The kind of named tests.", str)))),
			"strip_prefix", about("A prefix the IDs are written with and read without (not for source: text).", str),
			"required_for", about("The commit types that must carry the footer: a list, or all.", typesOrAll),
			"validate_for", about("The commit types whose footer IDs must exist, or whose free text must not be empty: a list, or all.", typesOrAll),
			"must_be_live", about("Whether every scenario the footer names must be live, not wip (not for source: text).", boo),
			"read_at", about("Where the IDs that exist are read: at the commit being checked, or in the working tree (not for source: text).",
				enum("commit", "worktree")),
			"since", about("The full SHA of the commit after which the footer is required: verify leaves it and its ancestors out of "+
				"required_for; the commit-msg hook always requires it.", str),
			"in_place_of", about("The other footers this one stands in for, by key, each with the commit types it does so for (a list, "+
				"or all): a commit of such a type that carries this footer needs none of that key (not for source: text).", mapOf(typesOrAll)),
		))),
		"path_sets", about("Named path lists; $<name> in a path list stands for one.", mapOf(strs)),
		"scopes", about("The paths each commit type may touch, by type.", mapOf(obj(nil,
			"only", about("The globs the commit's paths must all match.", strs),
			"never", about("The globs no path of the commit may match, unless it matches except.", strs),
			"except", about("The globs that take a path back out of never (only never; a scope with except needs a never).", strs),
			"must_touch", about("The globs one path of the commit at least must match.", strs),
		))),
		"reject_message", about("The line a rejected commit's problems are printed under.", str),
		"since", about("The full SHA of the commit where verification starts: verify and the range checks leave it and its ancestors out.", str),
	)),
	"tests", about("The kinds of named tests, by name, each behind an adapter.", mapOf(obj(nil,
		"adapter", about("gherkin, built in, or { command } that speaks the adapter protocol.", either(str, obj([]string{"command"},
			"command", about("The command that lists the tests (<command> list --at <tree>).", str),
			"supports_at", about("Whether the command lists the tests of a historical tree; when not, the working tree's are read.", boo),
		))),
		"root", about("The folder the tests are under.", str),
		"id", about("A regular expression every test ID matches.", str),
		"tag_prefix", about("What a Gherkin tag line writes before an ID.", str),
		"wip_tag", about("The tag that makes a scenario, or a file's scenarios, not live.", str),
		"run", about("How a selection of the tests runs.", obj(nil,
			"whole", about("The command that runs every test of the kind.", str),
			"select", about("The command that runs a selection, {pattern} standing for it.", str),
			"ids_pattern", about("How IDs become a pattern, {ids} standing for them joined with |.", str),
			"join", about("How several patterns join into one: each in each's {p}, joined by sep.", obj(nil, "each", str, "sep", str)),
		)),
		"recognize", about("Templates that read a task check back as a selection, so CI runs it in the kind's one run.", list(obj([]string{"command", "as"}, "command", str, "as", str))),
		"smoke", about("The smoke set: the tests every push runs.", obj(nil,
			"file", about("The smoke set's file.", str),
			"every_file", about("Whether every test file needs a smoke test.", boo),
			"add_hint", about("What a missing smoke test's fix says.", str),
		)),
		"range_checks", about("Rules on how the tests may change between two trees: commands, or builtin: moves.", list(obj([]string{"name"},
			"name", str,
			"except_types", about("The commit types the rule skips.", strs),
			"staged", about("The command the commit-msg hook runs on the staged tree.", str),
			"range", about("The command verify runs over a range, {from} and {to} standing for its ends.", str),
			"builtin", about("moves: itos's rule that, outside the types excepted, live scenarios only move, unchanged; on a kind "+
				"whose adapter is a command (with supports_at: true), no live test is added, lost, retitled or switched between live and wip.", enum("moves")),
			"allowed_renames", about("Renames builtin: moves allows, the new scenario name, or a command kind's new test title, by ID.", mapOf(str)),
		))),
	))),
	"ci", about("CI's plan: its steps, the prose shortcut, the cost patterns, what covers a check, the nightly, the range provider and the watch.", obj(nil,
		"env", about("Variables every step runs with.", mapOf(str)),
		"steps", about("Every step of CI, in order, given the range's ends as ITOS_FROM and ITOS_TO, and as {from} and {to} in its command; ci plan and ci run need it, while the range and the watch are read without it.", list(step)),
		"prose", about("A range touching only paths is prose, and runs only steps.", obj([]string{"paths", "steps"}, "paths", strs, "steps", strs)),
		"cost", about("Which checks with no cost of their own are static.", obj(nil,
			"static", about("Regular expressions over a command: static when one matches, else late.", strs),
			"keep_written_order", about("Whether a task's checks keep their written order, so no static check runs before a late one above it.", boo),
		)),
		"covers", about("A check matching matches is not run again after the step by.", list(obj([]string{"by", "matches"}, "by", str, "matches", str))),
		"nightly_only", about("Checks left to the nightly.", strs),
		"nightly", about("The nightly's steps.", obj([]string{"steps"}, "steps", list(step))),
		"wait_on_status", about("The work item statuses whose named tasks wait rather than run.", strs),
		"stop_at_first_failure", about("Whether CI stops at its first failure, or runs every step and check.", boo),
		"keep_step_order", about("Whether ci run runs ci.steps as written, then the named tasks' checks in their ledger order, instead of in cost order; "+
			"the cost class still decides what the commit-msg hook and the nightly's static step run.", boo),
		"range", about("Where a push's range starts.", obj(nil,
			"provider", about("github (the head's nearest first parent with a green run) or none.", provider),
			"github", about("The GitHub provider's workflow and environment.", obj(nil, "workflow", str, "repository_env", str, "token_env", strs)),
		)),
		"watch", about("How itos push and itos ci watch wait for a commit's CI run, and how itos status reads it and the last nightly's.", obj(nil,
			"provider", about("none (push does not wait) or github (the workflow's run, through GitHub's API).", provider),
			"github", about("The GitHub provider's workflow, and the nightly's, whose newest run on the watched branch itos status reads (left out, it prints no nightly); its token is ci.range.github.token_env's, else gh auth token.", obj(nil,
				"workflow", str,
				"nightly_workflow", str,
			)),
			"interval", about("The seconds between two looks at the run, 0 or more.", num),
			"timeout", about("The seconds a watch waits for the run to finish, above 0, before it exits 3.", num),
		)),
	)),
	"work", about("The work registry, its statuses, the people and the identity.", obj(nil,
		"registry", about("The work registry's file; by default work-items.yaml beside the ledger's files.", str),
		"asks", about("itos question's file; by default asks.yaml beside the work registry.", str),
		"decisions", about("The folder itos question record writes its MADR decision records and their index in; by default docs/decisions.", str),
		"groups_key", about("The registry's key whose entries name each group's owner.", str),
		"statuses", about("The statuses a work item may have; dropped, which itos work drop gives, is known whatever this lists.", strs),
		"people", about("Who may own work: the source and the file it reads.", obj([]string{"source", "file"},
			"source", enum("all-contributors-md", "all-contributorsrc", "yaml"),
			"file", str,
			"login_from", about("The link a login is read out of, {login} standing for it (all-contributors-md).", str),
		)),
		"identity", about("Who itos works for: github (gh's account) or none (only --as).", obj(nil, "provider", provider, "hint", str)),
	)),
	"hooks", about("The binary itos's hooks call, the pre-push commands and the commit-msg hook's task checks.", obj(nil,
		"bin", about("How the project calls itos: what the hooks itos hook install declares call; by default itos, the global launcher. Internal and unsupported, for a repository that must run its own build.", str),
		"pre_push", about("The pre-push hook's commands.", obj([]string{"per_base", "whole"},
			"per_base", about("Run once per remote base the clone has, {base} standing for it.", str),
			"whole", about("Run when there is no base.", str),
		)),
		"commit_msg", about("The commit-msg hook's task checks.", obj(nil,
			"task_checks", about("Whether the hook runs the static checks of the tasks a commit names.", boo),
			"check_timeout", about("The seconds a check may hold a commit, above 0.", num),
		)),
	)),
	"guide", about("What itos go prints beside the guide shipped in itos.", obj(nil,
		"orchestrating", about("The repository's own notes for the session that coordinates its work, which itos go appends to "+
			"the coordinator's guide when the file exists; in the git folder's itos/ for a stealth config.", str),
	)),
))

// KnownKey is whether the schema has a dotted key path, each part a key of
// the object above it or any key of a map (tests.scenario, a footer's name);
// a list's items and a scalar have no keys under them. When it has not, keys
// are the keys of the object the path last stood on, for the message that
// names the key, nil where there was none.
func KnownKey(key string) (bool, []string) {
	return knownIn(schema, strings.Split(key, "."))
}

func knownIn(s *spec, parts []string) (bool, []string) {
	if len(parts) == 0 {
		return true, nil
	}
	switch {
	case parts[0] == "":
		return false, s.keys()
	case s.either != nil:
		var keys []string
		for _, e := range s.either {
			ok, under := knownIn(e, parts)
			if ok {
				return true, nil
			}
			keys = append(keys, under...)
		}
		return false, keys
	case s.mapOf != nil:
		return knownIn(s.mapOf, parts[1:])
	case s.isObject:
		if f := s.field(parts[0]); f != nil {
			return knownIn(f, parts[1:])
		}
		return false, s.keys()
	}
	return false, nil
}

// Node is one value of the config's schema as data, for what describes the
// config outside itos: tools/bin/config-schema writes the config's JSON
// Schema from it. Kind is string, number, boolean, strings (a list of
// strings), enum, object, map, list or either.
type Node struct {
	Kind        string
	Description string
	Enum        []string // an enum's values
	Fields      []Field  // an object's keys, in the order the schema writes them
	Required    []string // an object's keys it cannot lack
	Of          *Node    // a map's values, a list's items
	Either      []*Node  // the shapes a value may take
}

// Field is one key of an object Node.
type Field struct {
	Key  string
	Node *Node
}

// Schema is the schema config check holds a file to, as data.
func Schema() *Node { return node(schema) }

func node(s *spec) *Node {
	n := &Node{Description: s.desc}
	switch {
	case s.scalar != "":
		n.Kind = s.scalar
	case s.enum != nil:
		n.Kind, n.Enum = "enum", s.enum
	case s.either != nil:
		n.Kind = "either"
		for _, e := range s.either {
			n.Either = append(n.Either, node(e))
		}
	case s.list != nil:
		n.Kind, n.Of = "list", node(s.list)
	case s.mapOf != nil:
		n.Kind, n.Of = "map", node(s.mapOf)
	default:
		n.Kind, n.Required = "object", s.required
		for _, f := range s.object {
			n.Fields = append(n.Fields, Field{f.key, node(f.spec)})
		}
	}
	return n
}

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
		if text, ok := v.(string); ok {
			if fix, ok := removedValues[path][text]; ok {
				return []out.Problem{removed(path+": "+text, fix)}
			}
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
		case removedKeys[key(k)] != "":
			found = append(found, removed(key(k), removedKeys[key(k)]))
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
