package ledger

import (
	"context"
	"fmt"
	"math"
	"path"
	"path/filepath"
	"regexp"
	"regexp/syntax"
	"slices"
	"sort"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/store/tree/port"
	"github.com/donvargax/itos/v8/internal/value"
)

// Layout is the ledger's folder and how its files are named:
// tasks/phase-{group}.yaml gives tasks and a pattern whose group is the
// phase. The folder and the files' paths have slashes on every platform, as
// git names the paths they are compared with (bug 9).
type Layout struct {
	Dir     string
	File    *regexp.Regexp
	Numeric bool
}

// LayoutOf is the config's ledger layout; an error when the config has no
// ledger section.
func LayoutOf(cfg *config.Loaded) (Layout, error) {
	if err := cfg.Section("ledger"); err != nil {
		return Layout{}, err
	}
	files := filepath.ToSlash(cfg.Ledger.Files)
	parts := strings.Split(path.Base(files), "{group}")
	before, after := parts[0], ""
	if len(parts) > 1 {
		after = parts[1]
	}
	return Layout{
		Dir:     path.Dir(files),
		File:    regexp.MustCompile("^" + regexp.QuoteMeta(before) + "(" + cfg.Ledger.Group.Pattern + ")" + regexp.QuoteMeta(after) + "$"),
		Numeric: cfg.Ledger.Group.Numeric,
	}, nil
}

// File is one ledger file and its group.
type File struct {
	Path  string
	Group string
}

// Files are the ledger's files in its folder, sorted, with their group. A
// project may configure a ledger before it makes the folder: the folder
// missing is one config error naming it (exit 2, a folder the config names
// that cannot be read), in every command that reads the ledger, at whatever
// tree the source reads. A working tree's folder that is there and cannot be
// listed keeps its own error.
func Files(cfg *config.Loaded) ([]File, error) {
	layout, err := LayoutOf(cfg)
	if err != nil {
		return nil, err
	}
	names, err := cfg.Source.List(context.Background(), layout.Dir)
	if err != nil {
		if cfg.Source.Tree(context.Background()) == "worktree" && cfg.Source.Has(context.Background(), layout.Dir) {
			return nil, err
		}
		return nil, &config.Error{File: config.Path(cfg.VCS, cfg.Source), Problems: []out.Problem{{
			Rule:    "ledger-folder-missing",
			Message: fmt.Sprintf("the ledger's folder %s does not exist", layout.Dir),
			Fix:     fmt.Sprintf("create %s with the ledger's files (ledger.files is %s), or point ledger.files at the folder that holds them", layout.Dir, cfg.Ledger.Files),
		}}}
	}
	// By name: the TypeScript's localeCompare, which for names of one pattern
	// (a prefix, a group, a suffix) is the order of their bytes; numeric
	// groups then by number (slice 130), phase-2 before phase-10.
	sort.Strings(names)
	var files []File
	for _, name := range names {
		if m := layout.File.FindStringSubmatch(name); m != nil {
			files = append(files, File{Path: path.Join(layout.Dir, name), Group: m[1]})
		}
	}
	if layout.Numeric {
		sort.SliceStable(files, func(i, j int) bool { return groupBefore(files[i].Group, files[j].Group) })
	}
	return files, nil
}

// groupBefore is whether numeric group a comes before b: by number, a group
// that is no number after every one that is, in the order of their names.
func groupBefore(a, b string) bool {
	x, y := value.ToNumber(a), value.ToNumber(b)
	if math.IsNaN(y) {
		return !math.IsNaN(x)
	}
	return x < y
}

// LayoutProblems are the ledger layouts that read nothing, or read what is
// not a ledger (slice 130), judged before any file is listed: a
// ledger.group.pattern with anchors of its own, which never match, since the
// pattern stands inside a file's name, between what ledger.files writes
// around {group}; {group} in ledger.files' folder, which Files would look for
// as a folder of that name; and a ledger.files that matches work.registry,
// which every reader of the ledger would read as a list of tasks. None when
// the config has no ledger.
func LayoutProblems(cfg *config.Loaded) []out.Problem {
	layout, err := LayoutOf(cfg)
	if err != nil {
		return nil
	}
	var found []out.Problem
	pattern := cfg.Ledger.Group.Pattern
	if anchored(pattern) {
		found = append(found, problem("config-ledger-group-anchors",
			"ledger.group.pattern has anchors of its own: "+pattern+"; it is matched inside a file's name, "+
				"between what ledger.files writes around {group}, so it matches no file",
			"remove the anchors (^, $, \\A, \\z) from ledger.group.pattern"))
	}
	files := filepath.ToSlash(cfg.Ledger.Files)
	if strings.Contains(path.Dir(files), "{group}") {
		found = append(found, problem("config-ledger-files-folder",
			"ledger.files has {group} in its folder: "+cfg.Ledger.Files+"; the ledger is one folder, and a group is "+
				"a part of a file's name",
			"write {group} in the file's name, as in tasks/phase-{group}.yaml"))
	}
	registry := filepath.ToSlash(cfg.Work.Registry)
	if path.Dir(registry) == path.Clean(layout.Dir) && layout.File.MatchString(path.Base(registry)) {
		found = append(found, problem("config-ledger-registry",
			"ledger.files ("+cfg.Ledger.Files+") matches work.registry ("+cfg.Work.Registry+
				"), so the registry would be read as a ledger file",
			"change ledger.files or ledger.group.pattern so that no ledger file is work.registry, or move work.registry"))
	}
	return found
}

// anchored is whether a pattern that compiles holds an anchor: ^, $, \A or
// \z, anywhere in it.
func anchored(pattern string) bool {
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return false
	}
	var has func(*syntax.Regexp) bool
	has = func(r *syntax.Regexp) bool {
		switch r.Op {
		case syntax.OpBeginLine, syntax.OpEndLine, syntax.OpBeginText, syntax.OpEndText:
			return true
		}
		return slices.ContainsFunc(r.Sub, has)
	}
	return has(re)
}

var (
	taskKeys  = []string{"id", "type", "title", "why", "done_when"}
	checkKeys = []string{"run", "fails", "after", "timeout", "prose", "cost", "evidence", "snapshot"}
)

func problem(rule, message, fix string) out.Problem {
	return out.Problem{Rule: rule, Message: message, Fix: fix}
}

// checkValue is what one optional key of a check may hold, and the fix.
type checkValue struct {
	key   string
	valid func(any) bool
	rule  string
	fix   string
}

var checkValues = []checkValue{
	{"after", func(v any) bool { return v == "push" }, "after: push is its only value", "write after: push, or remove it"},
	{"timeout", func(v any) bool { return value.TypeOf(v) == "number" }, "its timeout is a number of seconds", "write the timeout as a number of seconds"},
	// A timeout of 0 is none at all to the shell, and the commit-msg hook takes
	// the lesser of it and its cap, so one of 0 or less, or NaN, lifted the cap
	// (slice 129); a number is judged here, anything else by the line above.
	{"timeout", func(v any) bool {
		seconds, ok := v.(float64)
		return !ok || seconds > 0 && !math.IsInf(seconds, 1)
	}, "its timeout is a finite number of seconds above 0", "write the timeout as the seconds the check may run, above 0"},
	{"prose", func(v any) bool { return value.TypeOf(v) == "boolean" }, "prose is true or false", "write prose: true or false"},
	{"cost", func(v any) bool { return v == "static" || v == "late" }, "cost is static or late", "write cost: static or cost: late"},
	{"snapshot", func(v any) bool { return value.TypeOf(v) == "boolean" }, "snapshot is true or false", "write snapshot: true or snapshot: false"},
}

// checkProblems are one check's problems.
func checkProblems(check any, where string) []out.Problem {
	c, ok := check.(*value.Map)
	if !ok {
		return []out.Problem{problem("ledger-check-shape", where+" is not a mapping", "write "+where+" as run: <command> or fails: <command>")}
	}
	var found []out.Problem
	for _, k := range c.Keys() {
		if !value.Includes(checkKeys, k) {
			found = append(found, problem("ledger-check-unknown-key", where+" has an unknown key "+k,
				"remove "+k+"; a check's keys are "+strings.Join(checkKeys, ", ")))
		}
	}
	command := c.At("run")
	if command == nil || command == value.Undefined {
		command = c.At("fails")
	}
	switch {
	case c.Has("run") == c.Has("fails"):
		found = append(found, problem("ledger-check-run-or-fails", where+" needs exactly one of run and fails",
			"give "+where+" either run: or fails:, not both"))
	case value.TypeOf(command) != "string":
		found = append(found, problem("ledger-check-command", where+"'s command is not text",
			"quote "+where+"'s command as one string"))
	}
	if v := c.At("evidence"); v != value.Undefined {
		found = append(found, evidenceProblems(v, where)...)
		if c.At("snapshot") == false {
			found = append(found, problem("ledger-check-evidence", where+" says snapshot: false and declares evidence",
				"remove evidence, which keys a run by the tree it ran on and a run in the working folder has none, or remove snapshot: false from "+where))
		}
	}
	for _, cv := range checkValues {
		if v := c.At(cv.key); v != value.Undefined && !cv.valid(v) {
			found = append(found, problem("ledger-check-value",
				fmt.Sprintf("%s says %s: %s; %s", where, cv.key, shown(v), cv.rule), cv.fix+" in "+where))
		}
	}
	return found
}

// evidenceProblems are what is wrong with a check's evidence: its shape,
// then, when it has one, the texts in it.
func evidenceProblems(v any, where string) []out.Problem {
	if found := config.EvidenceShape(v, where, "ledger-check-evidence"); len(found) > 0 {
		return found
	}
	return config.EvidenceOf(v).Problems(where, "ledger-check-evidence")
}

// shown is a check's value as its problem names it: as JSON, but a number
// as itos writes one, so a NaN reads as NaN, not as JSON's null.
func shown(v any) string {
	if n, ok := v.(float64); ok {
		return value.Number(n)
	}
	return value.JSON(v)
}

// commandOf is a check's command, run's else fails', and whether it is text.
func commandOf(check any) (string, bool) {
	command := value.Prop(check, "run")
	if command == nil || command == value.Undefined {
		command = value.Prop(check, "fails")
	}
	text, ok := command.(string)
	return text, ok
}

// lateByItself is whether a check is late by itself: its own cost, else the
// patterns.
func lateByItself(cfg *config.Loaded, check any) bool {
	switch cost := value.Prop(check, "cost"); cost {
	case "late", "static":
		return cost == "late"
	}
	text, ok := commandOf(check)
	return !(ok && cfg.MatchesStatic(text))
}

// orderFindings are what written order does to a task's checks under
// ci.cost.keep_written_order: a cost: static written below a late check,
// which it runs late, is a problem; a check static by a ci.cost.static
// pattern written there, run late just the same, is a warning (bug 27). The
// explicit mark asks for what the order cannot give, while a pattern match
// is the cost rule's own reading, and failing ledgers that passed before
// would break every project holding one.
func orderFindings(cfg *config.Loaded, checks []any) (problems, warnings []out.Problem) {
	firstLate := -1
	for i, check := range checks {
		if lateByItself(cfg, check) {
			firstLate = i
			break
		}
	}
	if firstLate < 0 || !cfg.CI.Cost.KeepWrittenOrder {
		return nil, nil
	}
	for n := firstLate + 1; n < len(checks); n++ {
		switch cost := value.Prop(checks[n], "cost"); {
		case cost == "static":
			problems = append(problems, problem("ledger-static-after-late",
				fmt.Sprintf("check %d says cost: static below check %d, which is late: written order runs it late", n, firstLate),
				fmt.Sprintf("move check %d above check %d, or remove its cost: static", n, firstLate)))
		case cost == value.Undefined || cost == nil:
			if text, ok := commandOf(checks[n]); ok && cfg.MatchesStatic(text) {
				warnings = append(warnings, problem("ledger-pattern-static-after-late",
					fmt.Sprintf("check %d is static by ci.cost.static below check %d, which is late: written order runs it late", n, firstLate),
					fmt.Sprintf("move check %d above check %d, or write cost: late on check %d if it is meant to run late", n, firstLate, n)))
			}
		}
	}
	return problems, warnings
}

// IDPattern is the pattern a task's whole ID must match: ledger.id, or any
// ID when the config has none. It is ledger.id's one reader (slice 130):
// itos task, the ledger check, work show's task items (work.SpecKind) and
// minting all take it, anchored as ^(?:id)$, so an alternation anchors every
// branch, where ^T-\d+|OPS-\d+$ anchored only the first's start and the
// last's end.
func IDPattern(cfg *config.Loaded) *regexp.Regexp {
	id := ".+"
	if cfg.Ledger.ID != nil {
		id = *cfg.Ledger.ID
	}
	return regexp.MustCompile("^(?:" + id + ")$")
}

// fieldProblems are a task's id, type, title and why.
func fieldProblems(cfg *config.Loaded, task any) []out.Problem {
	var found []out.Problem
	pattern := IDPattern(cfg)
	source := jsSource(pattern.String())
	switch id, ok := value.Prop(task, "id").(string); {
	case !ok:
		found = append(found, problem("ledger-no-id", "no id", "give the task an id: that matches ledger.id"))
	case !pattern.MatchString(id):
		found = append(found, problem("ledger-id-pattern", "the id does not match "+source,
			"rename the task to an id that matches "+source))
	}
	types := cfg.Commits.Types
	switch typ, ok := value.Prop(task, "type").(string); {
	case !ok:
		found = append(found, problem("ledger-no-type", "no type", "give the task a type: (a commit type)"))
	case types != nil && !value.Includes(types, typ):
		found = append(found, problem("ledger-type", "type "+typ+" is not a commit type",
			"set type: to one of "+strings.Join(types, ", ")))
	}
	if _, ok := value.Prop(task, "title").(string); !ok {
		found = append(found, problem("ledger-no-title", "no title", "give the task a title:"))
	}
	if why := value.Prop(task, "why"); why != value.Undefined && value.TypeOf(why) != "string" {
		found = append(found, problem("ledger-why", "its why is not text", "write the why as text"))
	}
	return found
}

// jsSource is a pattern as JavaScript's RegExp source writes it: each / not
// escaped and outside a character class escaped, and a line break written as
// \n.
func jsSource(pattern string) string {
	var b strings.Builder
	escaped, inClass := false, false
	for _, r := range pattern {
		switch {
		case escaped:
			escaped = false
		case r == '\\':
			escaped = true
		case r == '[':
			inClass = true
		case r == ']':
			inClass = false
		case r == '/' && !inClass:
			b.WriteByte('\\')
		case r == '\n':
			b.WriteString(`\n`)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// taskFindings are one task's own problems, its checks' included, and its
// warnings.
func taskFindings(cfg *config.Loaded, task any) (found, warnings []out.Problem) {
	for _, k := range value.Keys(task) {
		if !value.Includes(taskKeys, k) {
			found = append(found, problem("ledger-unknown-key", "unknown key "+k,
				"remove "+k+"; a task's keys are "+strings.Join(taskKeys, ", ")))
		}
	}
	found = append(found, fieldProblems(cfg, task)...)
	doneWhen := value.Prop(task, "done_when")
	if doneWhen == nil || doneWhen == value.Undefined {
		doneWhen = []any{}
	}
	checks, ok := doneWhen.([]any)
	if !ok {
		return append(found, problem("ledger-done-when", "done_when is not a list", "write done_when as a list of checks")), nil
	}
	for n, check := range checks {
		found = append(found, checkProblems(check, fmt.Sprintf("check %d", n))...)
	}
	order, warnings := orderFindings(cfg, checks)
	return append(found, order...), warnings
}

// read is a ledger file's tasks, or the problem reading it.
func read(src port.Source, file string) ([]any, *out.Problem) {
	text, err := src.Read(context.Background(), file)
	var tasks any
	if err == nil {
		tasks, err = value.Parse(text)
	}
	if err != nil {
		p := problem("ledger-unreadable", file+" cannot be read: "+err.Error(),
			"correct "+file+"'s YAML (a value holding \": \" must be quoted)")
		return nil, &p
	}
	if tasks == nil {
		return nil, nil
	}
	list, ok := tasks.([]any)
	if !ok {
		p := problem("ledger-not-a-list", file+" is not a list of tasks", "write "+file+" as a YAML list of tasks")
		return nil, &p
	}
	for i, t := range list {
		if t == nil {
			list[i] = value.NewMap()
		}
	}
	return list, nil
}

// Issues are the ledger's problems over the given files, each with its rule.
func Issues(cfg *config.Loaded, files []string) []out.Problem {
	found, _ := Findings(cfg, files)
	return found
}

// Findings are the ledger's problems over the given files and its warnings,
// which config check prints and never fails on, each with its rule and its
// message after the task's id.
func Findings(cfg *config.Loaded, files []string) (found, warned []out.Problem) {
	seen := map[string]string{}
	for _, file := range files {
		tasks, unreadable := read(cfg.Source, file)
		if unreadable != nil {
			found = append(found, *unreadable)
			continue
		}
		for i, task := range tasks {
			id, ok := value.Prop(task, "id").(string)
			if !ok {
				id = fmt.Sprintf("task %d of %s", i+1, file)
			}
			own, warnings := taskFindings(cfg, task)
			if other, twice := seen[id]; twice {
				own = append(own, problem("ledger-duplicate-id", "also in "+other, "give one of the two "+id+" tasks another id"))
			}
			seen[id] = file
			for _, p := range own {
				p.Message = id + ": " + p.Message
				found = append(found, p)
			}
			for _, p := range warnings {
				p.Message = id + ": " + p.Message
				warned = append(warned, p)
			}
		}
	}
	return found, warned
}
