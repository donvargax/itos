// Package ledger is the ledger: the YAML files of tasks ledger.files names
// (tasks/phase-{group}.yaml), where they are, and what is wrong with them
// (config.ts's ledgerLayout, ledgerFiles and ledgerIssues). The schema is
// strict: duplicate or malformed IDs, unknown types, unknown keys, both or
// neither of run and fails, and a cost: static written below a late check of
// the same task, which written order would run late anyway.
package ledger

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/donvargax/itos/v4/internal/config"
	"github.com/donvargax/itos/v4/internal/out"
	"github.com/donvargax/itos/v4/internal/source"
	"github.com/donvargax/itos/v4/internal/value"
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
	names, err := source.List(layout.Dir)
	if err != nil {
		if source.Current().Tree() == "worktree" && source.Has(layout.Dir) {
			return nil, err
		}
		return nil, &config.Error{File: config.Path(), Problems: []out.Problem{{
			Rule:    "ledger-folder-missing",
			Message: fmt.Sprintf("the ledger's folder %s does not exist", layout.Dir),
			Fix:     fmt.Sprintf("create %s with the ledger's files (ledger.files is %s), or point ledger.files at the folder that holds them", layout.Dir, cfg.Ledger.Files),
		}}}
	}
	// By name: the TypeScript's localeCompare, which for names of one pattern
	// (a prefix, a group, a suffix) is the order of their bytes.
	sort.Strings(names)
	var files []File
	for _, name := range names {
		if m := layout.File.FindStringSubmatch(name); m != nil {
			files = append(files, File{Path: path.Join(layout.Dir, name), Group: m[1]})
		}
	}
	return files, nil
}

var (
	taskKeys  = []string{"id", "type", "title", "why", "done_when"}
	checkKeys = []string{"run", "fails", "after", "timeout", "prose", "cost"}
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
	{"prose", func(v any) bool { return value.TypeOf(v) == "boolean" }, "prose is true or false", "write prose: true or false"},
	{"cost", func(v any) bool { return v == "static" || v == "late" }, "cost is static or late", "write cost: static or cost: late"},
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
	for _, cv := range checkValues {
		if v := c.At(cv.key); v != value.Undefined && !cv.valid(v) {
			found = append(found, problem("ledger-check-value",
				fmt.Sprintf("%s says %s: %s; %s", where, cv.key, value.JSON(v), cv.rule), cv.fix+" in "+where))
		}
	}
	return found
}

// lateByItself is whether a check is late by itself: its own cost, else the
// patterns.
func lateByItself(cfg *config.Loaded, check any) bool {
	switch cost := value.Prop(check, "cost"); cost {
	case "late", "static":
		return cost == "late"
	}
	command := value.Prop(check, "run")
	if command == nil || command == value.Undefined {
		command = value.Prop(check, "fails")
	}
	text, ok := command.(string)
	return !(ok && cfg.MatchesStatic(text))
}

// orderProblems: a cost: static written below a late check, which written
// order runs late.
func orderProblems(cfg *config.Loaded, checks []any) []out.Problem {
	firstLate := -1
	for i, check := range checks {
		if lateByItself(cfg, check) {
			firstLate = i
			break
		}
	}
	if firstLate < 0 || !cfg.CI.Cost.KeepWrittenOrder {
		return nil
	}
	var found []out.Problem
	for n := firstLate + 1; n < len(checks); n++ {
		if value.Prop(checks[n], "cost") == "static" {
			found = append(found, problem("ledger-static-after-late",
				fmt.Sprintf("check %d says cost: static below check %d, which is late: written order runs it late", n, firstLate),
				fmt.Sprintf("move check %d above check %d, or remove its cost: static", n, firstLate)))
		}
	}
	return found
}

// IDPattern is the pattern a task's whole ID must match: ledger.id, or any
// ID when the config has none.
func IDPattern(cfg *config.Loaded) *regexp.Regexp {
	id := ".+"
	if cfg.Ledger.ID != nil {
		id = *cfg.Ledger.ID
	}
	return regexp.MustCompile("^" + id + "$")
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

// taskProblems are one task's own problems, its checks' included.
func taskProblems(cfg *config.Loaded, task any) []out.Problem {
	var found []out.Problem
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
		return append(found, problem("ledger-done-when", "done_when is not a list", "write done_when as a list of checks"))
	}
	for n, check := range checks {
		found = append(found, checkProblems(check, fmt.Sprintf("check %d", n))...)
	}
	return append(found, orderProblems(cfg, checks)...)
}

// read is a ledger file's tasks, or the problem reading it.
func read(file string) ([]any, *out.Problem) {
	text, err := source.Read(file)
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
	seen := map[string]string{}
	var found []out.Problem
	for _, file := range files {
		tasks, unreadable := read(file)
		if unreadable != nil {
			found = append(found, *unreadable)
			continue
		}
		for i, task := range tasks {
			id, ok := value.Prop(task, "id").(string)
			if !ok {
				id = fmt.Sprintf("task %d of %s", i+1, file)
			}
			own := taskProblems(cfg, task)
			if other, twice := seen[id]; twice {
				own = append(own, problem("ledger-duplicate-id", "also in "+other, "give one of the two "+id+" tasks another id"))
			}
			seen[id] = file
			for _, p := range own {
				p.Message = id + ": " + p.Message
				found = append(found, p)
			}
		}
	}
	return found
}
