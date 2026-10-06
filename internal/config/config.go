// Package config finds, reads and validates itos.yaml, the one policy file
// (tools/itos/config.ts): the file --config or ITOS_CONFIG names, else
// itos.yaml in the folder itos runs in (--root changes that folder first, and
// so does a run from a subfolder of a repository whose config is at its top,
// top.go), else the stealth mode's <git common dir>/itos/itos.yaml
// (stealth.go).
//
// Load holds the file to the schema, where an unknown key is an error that
// names the key it misspells, then to what the schema cannot say (names that
// must refer to something, patterns that must compile, commits.since a full
// SHA), and lays it over the one table of defaults (defaults.go), which
// `config check --print-defaults` prints: a default is applied exactly when it
// is printed. Every reader of the config goes through Load; none has a
// fallback of its own.
package config

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/donvargax/itos/v5/internal/out"
	"github.com/donvargax/itos/v5/internal/source"
	"github.com/donvargax/itos/v5/internal/value"
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

// Invalid is a config error of one sentence and no fix, as a tool reports a
// key it needs and the config lacks.
func Invalid(file, message string) *Error {
	return &Error{File: file, Problems: []out.Problem{{Rule: "config-invalid", Message: message}}}
}

// Loaded is the config as the tools read it: the file laid over the table of
// defaults, typed, with the file as written beside it.
type Loaded struct {
	Config
	// Path is the file it was read from.
	Path string
	// Stealth is whether it is the stealth config (stealth.go), whose own
	// data is read beside it, in the git folder, and never at a commit.
	Stealth bool
	// file is the file as written, its $sets expanded; tree is it laid over
	// the defaults, as the typed config is decoded from.
	file    *value.Map
	tree    *value.Map
	statics []*regexp.Regexp
}

// File is the config file as written, without the defaults under it.
func (l *Loaded) File() *value.Map { return l.file }

// Get is the value at a dotted key path (hooks.bin, tests.scenario.root) as
// the tools read it: the file's, else its default, a stealth config's own
// values over both, and its own data's paths beside it (beside);
// value.Undefined when the key has neither. KnownKey says whether the schema
// has the key.
func (l *Loaded) Get(key string) any {
	var at any = l.tree
	for _, part := range strings.Split(key, ".") {
		at = value.Prop(at, part)
	}
	return at
}

// HasSection is whether the file has a section, or a key at a dotted path
// (ci.steps), rather than only its defaults.
func (l *Loaded) HasSection(key string) bool {
	parts := strings.Split(key, ".")
	at := l.file
	for _, part := range parts[:len(parts)-1] {
		next, ok := at.At(part).(*value.Map)
		if !ok {
			return false
		}
		at = next
	}
	return at.Has(parts[len(parts)-1])
}

// Section is an error when the file lacks a section, or a key of one (as
// ci plan and ci run need ci.steps), that a tool cannot work without.
func (l *Loaded) Section(key string) error {
	if l.HasSection(key) {
		return nil
	}
	fix := "add a " + key + ": section"
	if strings.Contains(key, ".") {
		fix = "add " + key
	}
	return &Error{File: l.Path, Problems: []out.Problem{{
		Rule:    "config-missing-section",
		Message: key + " is missing",
		Fix:     fix,
	}}}
}

// decode reads a tree into the typed config.
func decode(tree *value.Map, into *Config) error {
	return json.Unmarshal([]byte(value.JSON(tree)), into)
}

// configText is the config's text from the source itos reads its data from;
// one that source does not hold, such as an ITOS_CONFIG outside the
// repository, is read where it is.
func configText(file string) (string, error) {
	if source.Has(file) {
		return source.Read(file)
	}
	return source.Worktree.Read(file)
}

// Load reads, validates and lays over the defaults the config at file.
func Load(file string) (*Loaded, error) {
	text, err := configText(file)
	var raw any
	if err == nil {
		raw, err = value.Parse(text)
	}
	if err != nil {
		return nil, &Error{File: file, Problems: []out.Problem{{
			Rule:    "config-unreadable",
			Message: "cannot be read: " + err.Error(),
			Fix:     "create " + file + ", or correct its YAML",
		}}}
	}
	if found := problems(raw, schema, ""); len(found) > 0 {
		return nil, &Error{File: file, Problems: found}
	}
	tree := raw.(*value.Map)
	if version := tree.At("version"); version != 1.0 {
		return nil, &Error{File: file, Problems: []out.Problem{{
			Rule:    "config-version",
			Message: "version " + value.String(version) + " is not 1",
			Fix:     "set version: 1",
		}}}
	}
	found := expandSets(tree)
	var written Config
	if err := decode(tree, &written); err != nil {
		return nil, err
	}
	found = append(found, crossProblems(tree, &written)...)
	if len(found) > 0 {
		return nil, &Error{File: file, Problems: found}
	}
	loaded := &Loaded{Path: file, file: tree, Stealth: IsStealth(file)}
	loaded.tree = withDefaults(tree, loaded.Stealth)
	if loaded.Stealth {
		beside(loaded.tree, file)
	}
	if err := decode(loaded.tree, &loaded.Config); err != nil {
		return nil, err
	}
	return loaded, nil
}

// expandSets replaces each $name of a path list by commits.path_sets.<name>,
// and is a problem for a name the sets do not have.
func expandSets(tree *value.Map) []out.Problem {
	commits := value.Prop(tree, "commits")
	sets, _ := value.Prop(commits, "path_sets").(*value.Map)
	var found []out.Problem
	expand := func(globs []any, where string) []any {
		expanded := []any{}
		for _, g := range globs {
			glob := g.(string)
			if !strings.HasPrefix(glob, "$") {
				expanded = append(expanded, glob)
				continue
			}
			set, ok := sets.At(glob[1:]).([]any)
			if !ok {
				found = append(found, out.Problem{
					Rule:    "config-path-set",
					Message: where + " names " + glob + ", which commits.path_sets does not have",
					Fix:     "add commits.path_sets." + glob[1:] + ", or remove " + glob + " from " + where,
				})
			}
			expanded = append(expanded, set...)
		}
		return expanded
	}
	if scopes, ok := value.Prop(commits, "scopes").(*value.Map); ok {
		for _, typ := range scopes.Keys() {
			rule := scopes.At(typ).(*value.Map)
			for _, k := range []string{"only", "never", "except", "must_touch"} {
				if globs, ok := rule.At(k).([]any); ok {
					rule.Set(k, expand(globs, "commits.scopes."+typ+"."+k))
				}
			}
		}
	}
	if prose, ok := value.Prop(value.Prop(tree, "ci"), "prose").(*value.Map); ok {
		prose.Set("paths", expand(prose.At("paths").([]any), "ci.prose.paths"))
	}
	return found
}

// Normal is a command with its whitespace collapsed, as the command patterns
// read it.
func Normal(command string) string { return value.Collapse(value.Trim(command)) }

// Readings are a command as every command pattern of the config reads it: as
// written (trimmed), and, when its first word is hooks.bin, with that word
// read as `itos`. The ledger and ci.steps call itos the way the project does,
// so a pattern written for itos (`^itos work check` in ci.cost.static or a
// ci.covers rule, `itos work check` in ci.nightly_only, a
// tests.<kind>.recognize template) matches `tools/bin/itos work check`
// without spelling out the path. Only hooks.bin itself is read so; a
// hooks.bin of several words is read as its leading words. Each pattern is
// tried on every reading, so the reading only ever adds a match. It is for
// matching alone; the command runs as written.
func (l *Loaded) Readings(command string) []string {
	written := value.Trim(command)
	bin := strings.Split(Normal(l.Hooks.Bin), " ")
	words := value.Fields(written)
	for i, word := range bin {
		if word == "" || i >= len(words) || words[i] != word {
			return []string{written}
		}
	}
	quoted := make([]string, len(bin))
	for i, word := range bin {
		quoted[i] = regexp.QuoteMeta(word)
	}
	lead := regexp.MustCompile("^" + strings.Join(quoted, value.Space+"+"))
	asItos := "itos" + lead.ReplaceAllLiteralString(written, "")
	if asItos == written {
		return []string{written}
	}
	return []string{written, asItos}
}

// MatchesStatic is whether a command is static by ci.cost.static, over each
// of its readings with its whitespace collapsed: reading hooks.bin as itos
// only ever makes a command static, never late. CI's plan, the commit-msg
// hook and config check's order rule all come here.
func (l *Loaded) MatchesStatic(command string) bool {
	if l.statics == nil {
		l.statics = []*regexp.Regexp{}
		for _, p := range l.CI.Cost.Static {
			l.statics = append(l.statics, regexp.MustCompile(p))
		}
	}
	for _, reading := range l.Readings(command) {
		form := Normal(reading)
		for _, rule := range l.statics {
			if rule.MatchString(form) {
				return true
			}
		}
	}
	return false
}
