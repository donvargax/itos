package corpus

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// FixtureError is a fixture, or the runner's own setup, that cannot be read or used: the
// command line's exit 2, never a case's failure.
type FixtureError struct{ msg string }

func (e *FixtureError) Error() string { return e.msg }

func fixtureErr(format string, a ...any) error { return &FixtureError{fmt.Sprintf(format, a...)} }

// File is a fixture file as read, before its cases are held to the format: what previous-release
// readies (Machine) before the cases are taken (Cases).
type File struct {
	Path string // as given
	doc  *mapping
}

// Fixture is a fixture file's cases, its defaults laid under each.
type Fixture struct {
	Path  string // as given
	Shown string // as a failure names it
	Cases []*Case
}

// Base is the fixture's file name, by which a Changes: footer names its cases.
func (f *Fixture) Base() string { return filepath.Base(f.Path) }

// entry is one path of a files mapping: its text, or none to delete it.
type entry struct {
	path       string
	null       bool
	text       string
	executable *bool
}

// files is a files mapping, in the order it was written.
type files []entry

// over is f with g laid over it: g's paths replace f's where they stand, the rest follow.
func (f files) over(g files) files {
	out := append(files(nil), f...)
	for _, e := range g {
		replaced := false
		for i := range out {
			if out[i].path == e.path {
				out[i], replaced = e, true
				break
			}
		}
		if !replaced {
			out = append(out, e)
		}
	}
	return out
}

// step is one step of a case's scratch repository.
type step struct {
	commit                *string
	stage, write, files   files
	hasStage, hasWrite    bool
	branch, checkout, run string
	label                 string
}

// pair is a key and its value, in the order a fixture wrote them.
type pair struct {
	key   string
	value *string // nil: null
}

// Case is one case of a fixture, its file's defaults laid under it.
type Case struct {
	Name           string
	Argv           []string
	git            []step
	hasGit         bool
	files          files
	stdin          *string
	env            []pair
	cwd            string
	hide           []string
	github         any // nil: none
	exit           float64
	stdout, stderr *string
	stdoutHas      []string
	stderrHas      []string
	stdoutLacks    []string
	stderrLacks    []string
	json           any
	hasJSON        bool
	jsonHas        any
	hasJSONHas     bool
	stdoutNonempty bool
	stdoutContract bool
	filesAfter     files
	gitConfigAfter []pair
}

var (
	caseKeys = set("name", "argv", "git", "files", "stdin", "env", "cwd", "hide", "exit", "stdout",
		"stderr", "stdout_has", "stderr_has", "json", "json_has", "stdout_nonempty", "files_after",
		"git_config_after", "github", "stdout_contract", "stdout_lacks", "stderr_lacks")
	stepKeys = set("commit", "stage", "write", "files", "branch", "checkout", "run", "label")
	topKeys  = set("files", "git", "env", "github", "cases")
)

func set(keys ...string) map[string]bool {
	m := map[string]bool{}
	for _, k := range keys {
		m[k] = true
	}
	return m
}

// Files are the fixture files of a folder, sorted.
func Files(dir string) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

// ReadFile reads a fixture file's YAML.
func ReadFile(path string) (*File, error) {
	text, err := os.ReadFile(path)
	if err != nil {
		return nil, fixtureErr("%v", err)
	}
	doc, err := parseYAML(text, true)
	if err != nil {
		return nil, fixtureErr("%s: %v", path, err)
	}
	m, ok := doc.(*mapping)
	if !ok {
		return nil, fixtureErr("%s: has no list of cases", path)
	}
	if _, ok := m.vals["cases"].([]any); !ok {
		return nil, fixtureErr("%s: has no list of cases", path)
	}
	return &File{Path: path, doc: m}, nil
}

// ReadFixture reads a fixture file and holds its cases to the format.
func ReadFixture(path string) (*Fixture, error) {
	f, err := ReadFile(path)
	if err != nil {
		return nil, err
	}
	return f.Fixture()
}

// Names are the names of the file's cases, each that is a mapping with a name.
func (f *File) Names() []string {
	var names []string
	for _, c := range f.doc.vals["cases"].([]any) {
		if m, ok := c.(*mapping); ok {
			if name, ok := m.vals["name"].(string); ok {
				names = append(names, name)
			}
		}
	}
	return names
}

// Machine leaves in each case only what itos's contract holds it to, as previous-release judges
// the last release's corpus (docs/decisions/0035-only-machine-output-is-itos-s-contract-exit-codes-json-less-message-and-fix-and-the-files-it-writes.md):
// it takes out every case's stdout, stderr and their _has and _lacks, which judge plain text, but
// the stdout of a case marked stdout_contract, a plain output scripts read
// (docs/decisions/0054-script-facing-plain-outputs-and-the-guard-s-hook-json-are-contract-guide-text-is-exempt-like-message-and-fix.md),
// and the keys of its json that are sentences for people (unjudged). json_has and
// stdout_nonempty stay: they name only what the case chose to pin. It says how many cases it
// read, how many of them lost their plain output, and how many message, fix and text keys it left
// out.
func (f *File) Machine() (cases, plainOut, keys int) {
	for _, c := range f.doc.vals["cases"].([]any) {
		m, ok := c.(*mapping)
		if !ok {
			continue
		}
		cases++
		lost := false
		plain := []string{"stderr", "stdout_has", "stderr_has", "stdout_lacks", "stderr_lacks"}
		if v, _ := m.get("stdout_contract"); v != true {
			plain = append(plain, "stdout")
		}
		for _, k := range plain {
			if _, ok := m.get(k); ok {
				lost = true
				m.del(k)
			}
		}
		if lost {
			plainOut++
		}
		if j, ok := m.get("json"); ok {
			m.set("json", machine(j, &keys, true))
		}
	}
	return cases, plainOut, keys
}

// unjudged are the json keys no case's json: compares, at any depth: message and fix, a
// problem's sentences (docs/decisions/0035-only-machine-output-is-itos-s-contract-exit-codes-json-less-message-and-fix-and-the-files-it-writes.md).
// unjudgedTop are those it leaves out of the top-level object alone: text, guide --json's and
// go --json's prose (ADR-0054); a text key deeper down is data, a follow note's or a footer's.
var (
	unjudged    = set("message", "fix")
	unjudgedTop = set("text")
)

// machine is a json value less its unjudged keys, and with top its own unjudgedTop keys,
// counted.
func machine(v any, keys *int, top bool) any {
	switch t := v.(type) {
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = machine(e, keys, false)
		}
		return out
	case *mapping:
		out := newMapping()
		for _, k := range t.keys {
			if unjudged[k] || top && unjudgedTop[k] {
				*keys++
				continue
			}
			out.set(k, machine(t.vals[k], keys, false))
		}
		return out
	}
	return v
}

// Fixture holds the file's cases to the format, its defaults laid under each.
func (f *File) Fixture() (*Fixture, error) {
	fail := func(format string, a ...any) error {
		return fixtureErr("%s: %s", f.Path, fmt.Sprintf(format, a...))
	}
	top := f.doc
	if k := unknownKey(top, topKeys); k != "" {
		return nil, fail("unknown key %s", k)
	}
	topGit, hasTopGit, err := readSteps(top.vals["git"])
	if err != nil {
		return nil, fail("%v", err)
	}
	topFiles, err := readFiles(top.vals["files"], "files")
	if err != nil {
		return nil, fail("%v", err)
	}
	topEnv, err := readPairs(top.vals["env"], "env")
	if err != nil {
		return nil, fail("%v", err)
	}
	fixture := &Fixture{Path: f.Path, Shown: f.Path}
	names := map[string]bool{}
	for i, raw := range top.vals["cases"].([]any) {
		c, why := readCase(raw, names)
		if why != "" {
			label := ""
			if m, ok := raw.(*mapping); ok {
				if name, ok := m.vals["name"].(string); ok && name != "" {
					label = " (" + name + ")"
				}
			}
			return nil, fail("case %d%s %s", i+1, label, why)
		}
		names[c.Name] = true
		if !c.hasGit {
			c.git, c.hasGit = topGit, hasTopGit
		}
		c.files = topFiles.over(c.files)
		c.env = overPairs(topEnv, c.env)
		if _, own := raw.(*mapping).get("github"); !own {
			c.github = top.vals["github"]
		}
		fixture.Cases = append(fixture.Cases, c)
	}
	return fixture, nil
}

// unknownKey is the first key of m that known does not hold, "" for none.
func unknownKey(m *mapping, known map[string]bool) string {
	for _, k := range m.keys {
		if !known[k] {
			return k
		}
	}
	return ""
}

// readCase is one case, or what is wrong with it.
func readCase(raw any, names map[string]bool) (*Case, string) {
	m, ok := raw.(*mapping)
	if !ok {
		return nil, "is not a mapping"
	}
	if k := unknownKey(m, caseKeys); k != "" {
		return nil, "unknown key " + k
	}
	c := &Case{}
	if c.Name, ok = m.vals["name"].(string); !ok {
		return nil, "has no name"
	}
	if names[c.Name] {
		return nil, "uses a name twice"
	}
	argv, ok := m.vals["argv"].([]any)
	if !ok {
		return nil, "has an argv that is not a list of strings"
	}
	for _, a := range argv {
		s, ok := a.(string)
		if !ok {
			return nil, "has an argv that is not a list of strings"
		}
		c.Argv = append(c.Argv, s)
	}
	if c.exit, ok = m.vals["exit"].(float64); !ok {
		return nil, "has no exit"
	}
	var err error
	if c.git, c.hasGit, err = readSteps(m.vals["git"]); err != nil {
		return nil, err.Error()
	}
	if c.files, err = readFiles(m.vals["files"], "files"); err != nil {
		return nil, err.Error()
	}
	if c.filesAfter, err = readFiles(m.vals["files_after"], "files_after"); err != nil {
		return nil, err.Error()
	}
	if c.env, err = readPairs(m.vals["env"], "env"); err != nil {
		return nil, err.Error()
	}
	if c.gitConfigAfter, err = readPairs(m.vals["git_config_after"], "git_config_after"); err != nil {
		return nil, err.Error()
	}
	strs := []struct {
		key string
		to  **string
	}{{"stdin", &c.stdin}, {"stdout", &c.stdout}, {"stderr", &c.stderr}}
	for _, s := range strs {
		if v, ok := m.get(s.key); ok && v != nil {
			text, ok := v.(string)
			if !ok {
				return nil, "has a " + s.key + " that is not a string"
			}
			*s.to = &text
		}
	}
	if v, ok := m.get("cwd"); ok && v != nil {
		if c.cwd, ok = v.(string); !ok {
			return nil, "has a cwd that is not a string"
		}
	}
	lists := []struct {
		key string
		to  *[]string
	}{{"hide", &c.hide}, {"stdout_has", &c.stdoutHas}, {"stderr_has", &c.stderrHas},
		{"stdout_lacks", &c.stdoutLacks}, {"stderr_lacks", &c.stderrLacks}}
	for _, l := range lists {
		if *l.to, err = stringList(m.vals[l.key], l.key); err != nil {
			return nil, err.Error()
		}
	}
	for _, l := range lists[3:] {
		if slices.Contains(*l.to, "") {
			return nil, "has a " + l.key + " that holds the empty text, which every output holds"
		}
	}
	c.json, c.hasJSON = m.get("json")
	c.jsonHas, c.hasJSONHas = m.get("json_has")
	if v, ok := m.get("stdout_contract"); ok {
		if v != true {
			return nil, "has a stdout_contract that is not true"
		}
		if c.stdout == nil {
			return nil, "has a stdout_contract without a stdout"
		}
		c.stdoutContract = true
	}
	if c.stdout != nil && *c.stdout != "" && !c.stdoutContract {
		return nil, "pins its stdout exactly, which only a stdout_contract may (ADR-0052): use json, stdout_has or stdout_lacks"
	}
	if c.stderr != nil && *c.stderr != "" {
		return nil, "pins its stderr exactly, which no case may but as empty (ADR-0052): use stderr_has or stderr_lacks"
	}
	if v, ok := m.get("stdout_nonempty"); ok {
		if v != true {
			return nil, "has a stdout_nonempty that is not true"
		}
		c.stdoutNonempty = true
	}
	c.github = m.vals["github"]
	return c, ""
}

// stringList is a string or a list of them, as a list.
func stringList(v any, key string) ([]string, error) {
	switch t := v.(type) {
	case nil:
		return nil, nil
	case string:
		return []string{t}, nil
	case []any:
		var out []string
		for _, e := range t {
			s, ok := e.(string)
			if !ok {
				return nil, fmt.Errorf("has a %s that is not text or a list of texts", key)
			}
			out = append(out, s)
		}
		return out, nil
	}
	return nil, fmt.Errorf("has a %s that is not text or a list of texts", key)
}

// readSteps is a git key's steps, and whether it was given at all.
func readSteps(v any) ([]step, bool, error) {
	if v == nil {
		return nil, false, nil
	}
	list, ok := v.([]any)
	if !ok {
		return nil, false, fmt.Errorf("has a git that is not a list of steps")
	}
	steps := []step{}
	for _, raw := range list {
		m, ok := raw.(*mapping)
		if !ok {
			return nil, false, fmt.Errorf("has a git step that is not a mapping")
		}
		if k := unknownKey(m, stepKeys); k != "" {
			return nil, false, fmt.Errorf("a git step has an unknown key %s", k)
		}
		var s step
		var err error
		if v, ok := m.get("commit"); ok && v != nil {
			text, ok := v.(string)
			if !ok {
				return nil, false, fmt.Errorf("a git step's commit is not text")
			}
			s.commit = &text
		}
		for _, f := range []struct {
			key string
			to  *files
			has *bool
		}{{"stage", &s.stage, &s.hasStage}, {"write", &s.write, &s.hasWrite}, {"files", &s.files, nil}} {
			if *f.to, err = readFiles(m.vals[f.key], f.key); err != nil {
				return nil, false, err
			}
			if f.has != nil {
				*f.has = m.vals[f.key] != nil
			}
		}
		for _, f := range []struct {
			key string
			to  *string
		}{{"branch", &s.branch}, {"checkout", &s.checkout}, {"run", &s.run}, {"label", &s.label}} {
			if v, ok := m.get(f.key); ok && v != nil {
				if *f.to, ok = v.(string); !ok {
					return nil, false, fmt.Errorf("a git step's %s is not text", f.key)
				}
			}
		}
		steps = append(steps, s)
	}
	return steps, true, nil
}

// readFiles is a files mapping: a path's text, null to delete it, or {text, executable}.
func readFiles(v any, key string) (files, error) {
	if v == nil {
		return nil, nil
	}
	m, ok := v.(*mapping)
	if !ok {
		return nil, fmt.Errorf("has %s that are not a mapping of paths", key)
	}
	var out files
	for _, path := range m.keys {
		e := entry{path: path}
		switch t := m.vals[path].(type) {
		case nil:
			e.null = true
		case string:
			e.text = t
		case *mapping:
			text, ok := t.vals["text"].(string)
			if !ok {
				return nil, fmt.Errorf("%s: %s has no text", key, path)
			}
			e.text = text
			if x, ok := t.vals["executable"].(bool); ok {
				e.executable = &x
			}
		default:
			return nil, fmt.Errorf("%s: %s is not text", key, path)
		}
		out = append(out, e)
	}
	return out, nil
}

// readPairs is a mapping of names to texts, a null value kept as none.
func readPairs(v any, key string) ([]pair, error) {
	if v == nil {
		return nil, nil
	}
	m, ok := v.(*mapping)
	if !ok {
		return nil, fmt.Errorf("has %s that is not a mapping", key)
	}
	var out []pair
	for _, k := range m.keys {
		switch t := m.vals[k].(type) {
		case nil:
			out = append(out, pair{key: k})
		case string:
			out = append(out, pair{key: k, value: &t})
		case float64, bool:
			text := fmt.Sprint(t)
			out = append(out, pair{key: k, value: &text})
		default:
			return nil, fmt.Errorf("%s: %s is not text", key, k)
		}
	}
	return out, nil
}

// overPairs is a with b laid over it, as files are.
func overPairs(a, b []pair) []pair {
	out := append([]pair(nil), a...)
	for _, p := range b {
		replaced := false
		for i := range out {
			if out[i].key == p.key {
				out[i], replaced = p, true
				break
			}
		}
		if !replaced {
			out = append(out, p)
		}
	}
	return out
}

// shownArgv is a case's command as a failure shows it.
func (c *Case) shownArgv() string { return strings.Join(c.Argv, " ") }
