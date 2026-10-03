package tests

import (
	"strings"

	"github.com/donvargax/itos/v2/internal/config"
)

// SelectionKind is what one run of a kind selects.
type SelectionKind int

const (
	// SelectIDs is some IDs: a footer's, a task's subset, the smoke set.
	SelectIDs SelectionKind = iota
	// SelectWhole is every test.
	SelectWhole
	// SelectPattern is a pattern read back from a task check.
	SelectPattern
)

// Selection is what one run of a kind selects (tests.ts's Selection): every
// test, some IDs (with or without the tag prefix), or a pattern from a task
// check, which goes into the select template as it is.
type Selection struct {
	Kind    SelectionKind
	IDs     []string
	Pattern string
}

// Whole selects every test.
func Whole() Selection { return Selection{Kind: SelectWhole} }

// IDs selects these tests; none selects nothing.
func IDs(ids ...string) Selection { return Selection{Kind: SelectIDs, IDs: ids} }

// Pattern selects what a pattern read back from a task check selects.
func Pattern(p string) Selection { return Selection{Kind: SelectPattern, Pattern: p} }

// template is one of the kind's run templates, a config problem when it has
// none.
func template(cfg *config.Loaded, name, key string, t *string) (string, error) {
	if t == nil {
		return "", config.Invalid(cfg.Path, "tests."+name+".run."+key+" is missing")
	}
	return *t, nil
}

// replaceFirst is String.prototype.replace with a string pattern and a
// function: the first occurrence alone, the replacement taken as it is (no
// `$&` or `$1` read in it).
func replaceFirst(s, old, new string) string { return strings.Replace(s, old, new, 1) }

// unique is a list without its repeats, in the order first seen.
func unique(list []string) []string {
	seen := map[string]bool{}
	kept := []string{}
	for _, s := range list {
		if !seen[s] {
			seen[s] = true
			kept = append(kept, s)
		}
	}
	return kept
}

// CommandFor is the one command that runs every selection (tests.ts's
// commandFor), and false when nothing is selected: the kind's `whole`
// template if any selection is whole; else each IDs selection one pattern
// by `ids_pattern` (its `{ids}` the bare IDs, deduplicated, joined by `|`),
// a pattern selection its pattern, the patterns deduplicated in order and,
// when more than one, each put in `join.each`'s `{p}` and joined by
// `join.sep`, the whole one shell word in `select`'s `{pattern}`. An IDs
// selection with no IDs selects nothing: rendered, its `ids_pattern` would
// be an empty alternation, which matches every test. A template the kind
// lacks is a config error, and only when the selections need it.
func CommandFor(cfg *config.Loaded, name string, all []Selection) (string, bool, error) {
	k, err := KindOf(cfg, name)
	if err != nil {
		return "", false, err
	}
	var selections []Selection
	for _, s := range all {
		if s.Kind != SelectIDs || len(s.IDs) > 0 {
			selections = append(selections, s)
		}
	}
	if len(selections) == 0 {
		return "", false, nil
	}
	for _, s := range selections {
		if s.Kind == SelectWhole {
			whole, err := template(cfg, name, "whole", k.Run.Whole)
			return whole, err == nil, err
		}
	}
	var patterns []string
	for _, s := range selections {
		if s.Kind == SelectPattern {
			patterns = append(patterns, s.Pattern)
			continue
		}
		idsPattern, err := template(cfg, name, "ids_pattern", k.Run.IDsPattern)
		if err != nil {
			return "", false, err
		}
		bare := make([]string, len(s.IDs))
		for i, id := range s.IDs {
			bare[i] = BareID(k, id)
		}
		patterns = append(patterns, replaceFirst(idsPattern, "{ids}", strings.Join(unique(bare), "|")))
	}
	patterns = unique(patterns)
	pattern := patterns[0]
	if len(patterns) > 1 {
		each := make([]string, len(patterns))
		for i, p := range patterns {
			each[i] = replaceFirst(k.Run.Join.Each, "{p}", p)
		}
		pattern = strings.Join(each, k.Run.Join.Sep)
	}
	selectTemplate, err := template(cfg, name, "select", k.Run.Select)
	if err != nil {
		return "", false, err
	}
	return replaceFirst(selectTemplate, "{pattern}", ShellWord(pattern)), true, nil
}
