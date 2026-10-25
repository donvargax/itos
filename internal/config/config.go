package config

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/store/tree/port"
	"github.com/donvargax/itos/v8/internal/value"
	vcs "github.com/donvargax/itos/v8/internal/vcs/port"
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
	// Trees opens the trees itos reads, and Source is the one the config and
	// its data are read from: the working tree, or the index in the
	// commit-msg hook (ADR-0053).
	Trees  port.Trees
	Source port.Source
	// VCS is the repository itos runs in: it locates the stealth config and
	// reads commits.since's commits and ranges.
	VCS vcs.VCS
	// file is the file as written, its $sets expanded; tree is it laid over
	// the defaults, as the typed config is decoded from.
	file    *value.Map
	tree    *value.Map
	statics []*regexp.Regexp
	// comment is CommentChar's answer, asked of the VCS once.
	comment func() string
}

// CommentChar is the repository's comment character (VCS.CommentChar), asked
// once per load; # for a config Load did not make.
func (l *Loaded) CommentChar() string {
	if l.comment == nil {
		return "#"
	}
	return l.comment()
}

// Folder is the folder itos runs in, which the config's relative paths
// are read from: its VCS's Folder, "" for the process's or with no VCS.
func (l *Loaded) Folder() string {
	if l.VCS == nil {
		return ""
	}
	return l.VCS.Folder(context.Background())
}

// withVCS is the config reading its data through trees and src and its
// repository through v.
func (l *Loaded) withVCS(v vcs.VCS, trees port.Trees, src port.Source) *Loaded {
	l.Trees, l.Source, l.VCS = trees, src, v
	l.comment = sync.OnceValue(func() string { return v.CommentChar(context.Background()) })
	return l
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

// Settings is the config's settings after the defaults as JSON: the tree the
// typed config is decoded from, and nothing of where it was read, whether it
// is the stealth one or what it reads its data from. Two configs with the
// same settings give the same text wherever they sit, which is what a key
// that binds the policy needs. Keys are in the order the file wrote them.
func (l *Loaded) Settings() string { return value.JSON(l.tree) }

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

// decode reads a tree into the typed config; a variable, so a test can make
// it fail, as no tree the schema accepts does.
var decode = func(tree *value.Map, into *Config) error {
	return json.Unmarshal([]byte(value.JSON(tree)), into)
}

// configText is the config's text from src; one src does not hold, such as
// an ITOS_CONFIG outside the repository, is read where it is.
func configText(trees port.Trees, src port.Source, file string) (string, error) {
	ctx := context.Background()
	if src.Has(ctx, file) {
		return src.Read(ctx, file)
	}
	worktree, err := trees.At(ctx, "worktree")
	if err != nil {
		return "", err
	}
	return worktree.Read(ctx, file)
}

// Load reads, validates and lays over the defaults the config at file, read
// from src, the tree its data is then read from; trees opens the others and
// v is the repository.
func Load(v vcs.VCS, trees port.Trees, src port.Source, file string) (*Loaded, error) {
	text, err := configText(trees, src, file)
	if err != nil {
		return nil, unreadable(file, err)
	}
	loaded, err := fromText(file, text, func(file string) bool { return IsStealth(v, file) })
	if err != nil {
		return nil, err
	}
	return loaded.withVCS(v, trees, src), nil
}

// DataFrom is the config with its data read from src: plan's --data-at
// reads a commit's data by the working tree's config.
func (l *Loaded) DataFrom(src port.Source) *Loaded {
	at := *l
	at.Source = src
	return &at
}

// FromText is Load of a config's text, not read from file but named by it,
// the stealth one or not as stealth says: itos init --policy (slice 108)
// validates a policy, and the config it makes of it, before it writes any.
// Its data is read from src.
func FromText(v vcs.VCS, trees port.Trees, src port.Source, file, text string, stealth bool) (*Loaded, error) {
	loaded, err := fromText(file, text, func(string) bool { return stealth })
	if err != nil {
		return nil, err
	}
	return loaded.withVCS(v, trees, src), nil
}

// unreadable is a config that cannot be read, or is not YAML.
func unreadable(file string, err error) *Error {
	return &Error{File: file, Problems: []out.Problem{{
		Rule:    "config-unreadable",
		Message: "cannot be read: " + err.Error(),
		Fix:     "create " + file + ", or correct its YAML",
	}}}
}

// fromText validates the config's text and lays it over the defaults,
// asking isStealth only of a config that is valid.
func fromText(file, text string, isStealth func(string) bool) (*Loaded, error) {
	raw, err := value.Parse(text)
	if err != nil {
		return nil, unreadable(file, err)
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
	loaded := &Loaded{Path: file, file: tree, Stealth: isStealth(file)}
	loaded.tree = withDefaults(tree, loaded.Stealth)
	if loaded.Stealth {
		beside(loaded.tree, file)
	}
	if err := decode(loaded.tree, &loaded.Config); err != nil {
		return nil, err
	}
	return loaded, nil
}

// expandSets replaces each $name of a path list (commits.scopes', ci.prose's
// paths, push.pr.direct.except, a range check's code) by commits.path_sets.<name>,
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
	if kinds, ok := value.Prop(tree, "tests").(*value.Map); ok {
		for _, name := range kinds.Keys() {
			checks, _ := value.Prop(kinds.At(name), "range_checks").([]any)
			for i, check := range checks {
				if globs, ok := value.Prop(check, "code").([]any); ok {
					check.(*value.Map).Set("code", expand(globs, "tests."+name+".range_checks["+strconv.Itoa(i)+"].code"))
				}
			}
		}
	}
	if prose, ok := value.Prop(value.Prop(tree, "ci"), "prose").(*value.Map); ok {
		prose.Set("paths", expand(prose.At("paths").([]any), "ci.prose.paths"))
	}
	if direct, ok := value.Prop(value.Prop(value.Prop(tree, "push"), "pr"), "direct").(*value.Map); ok {
		if globs, ok := direct.At("except").([]any); ok {
			direct.Set("except", expand(globs, "push.pr.direct.except"))
		}
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
