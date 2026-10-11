package config

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/value"
)

// Unusable are the values of a config that loads which nothing reads as
// written (slice 129): config check reports them, exit 1, naming the key. A
// range check's except_types naming a type commits.types does not list, and
// a wip_tag written on a kind whose adapter is a command, which reads no tag.
// The group label, which must be a flag of itos task's, is the command line's
// to judge (cli's configFindings). The values a command would misbehave on
// are refused when the config loads (unusableProblems).
func (l *Loaded) Unusable() []out.Problem {
	return append(exceptTypeProblems(&l.Config), wipTagProblems(l.file, &l.Config)...)
}

// unusableProblems are the values a command misbehaves on when it reads
// them, refused when the config loads, exit 2, for every command (slice 130;
// slice 129 had config check name them, exit 1, while every other command
// read the config as before): a number that is .nan or .inf anywhere in the
// file, a shell with no program, and a ledger.check.timeout of 0 or less.
// The typed config reads a .nan as no value at all, so a .nan
// ci.watch.interval panicked itos push.
func unusableProblems(tree *value.Map, c *Config) []out.Problem {
	found := nonFinite(tree, "")
	if tree.Has("shell") {
		found = append(found, shellProblems(c.Shell)...)
	}
	return append(found, ledgerTimeoutProblems(tree)...)
}

// nonFinite is every number of the file as written that is NaN or infinite,
// by its key path: the typed config reads one as no value at all, so a
// ci.watch.interval of .nan stopped itos push and a check_timeout of .nan
// left the commit-msg hook uncapped.
func nonFinite(v any, at string) []out.Problem {
	switch t := v.(type) {
	case float64:
		if !math.IsNaN(t) && !math.IsInf(t, 0) {
			return nil
		}
		return []out.Problem{{
			Rule:    "config-number-finite",
			Message: at + " is " + value.Number(t) + ", and a number in the config is finite",
			Fix:     "set " + at + " to a finite number",
		}}
	case *value.Map:
		var found []out.Problem
		for _, k := range t.Keys() {
			key := k
			if at != "" {
				key = at + "." + k
			}
			found = append(found, nonFinite(t.At(k), key)...)
		}
		return found
	case []any:
		var found []out.Problem
		for i, item := range t {
			found = append(found, nonFinite(item, fmt.Sprintf("%s[%d]", at, i))...)
		}
		return found
	}
	return nil
}

// shellProblems: every command runs with shell as its argv prefix, so a
// shell with no word, or an empty first word, runs no program: shell: []
// ran sh <command>, which reads the command as a script's path.
func shellProblems(shell []string) []out.Problem {
	if len(shell) > 0 && strings.TrimSpace(shell[0]) != "" {
		return nil
	}
	return []out.Problem{{
		Rule:    "config-shell",
		Message: "shell names no program, so no command can run under it",
		Fix:     "remove shell to run commands under sh -c, or name the program first: [bash, -c]",
	}}
}

// exceptTypeProblems: a range check leaves alone the commit types its
// except_types names, so a type commits.types does not list (fead for feat)
// leaves the type it meant judged by the rule, with no hint of why, as a
// footer's required_for and validate_for are held to commits.types too.
func exceptTypeProblems(c *Config) []out.Problem {
	types := c.Commits.Types
	if types == nil {
		return nil
	}
	var found []out.Problem
	for _, name := range c.Tests.Keys {
		for i, check := range c.Tests.Values[name].RangeChecks {
			at := fmt.Sprintf("tests.%s.range_checks[%d].except_types", name, i)
			for _, typ := range check.ExceptTypes {
				if slices.Contains(types, typ) {
					continue
				}
				found = append(found, out.Problem{
					Rule:    "config-except-types",
					Message: at + " names " + typ + ", which is not one of commits.types",
					Fix:     "add " + typ + " to commits.types, or remove it from " + at,
				})
			}
		}
	}
	return found
}

// wipTagProblems: only the Gherkin adapter reads a kind's wip_tag; a command
// adapter says itself which tests are live. Judged on the file as written,
// since every kind has the default.
func wipTagProblems(file *value.Map, c *Config) []out.Problem {
	var found []out.Problem
	for _, name := range c.Tests.Keys {
		written := value.Prop(value.Prop(file.At("tests"), name), "wip_tag") != value.Undefined
		if !written || c.Tests.Values[name].Adapter.Command == "" {
			continue
		}
		at := "tests." + name + ".wip_tag"
		found = append(found, out.Problem{
			Rule:    "config-wip-tag",
			Message: at + " is read only by the gherkin adapter, and tests." + name + ".adapter is a command, which says itself which tests are live",
			Fix:     "remove " + at,
		})
	}
	return found
}

// ledgerTimeoutProblems: a check's timeout of 0 is none at all to the shell,
// and the commit-msg hook takes the lesser of it and its own cap, so a
// ledger.check.timeout of 0 or less lifted the cap. Judged on the file as
// written: its default is 600, and a .nan or .inf is nonFinite's.
func ledgerTimeoutProblems(file *value.Map) []out.Problem {
	seconds, ok := value.Prop(value.Prop(file.At("ledger"), "check"), "timeout").(float64)
	if !ok || seconds > 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return nil
	}
	return []out.Problem{{
		Rule:    "config-check-timeout",
		Message: "ledger.check.timeout is a number of seconds above 0, not " + value.Number(seconds),
		Fix:     "set ledger.check.timeout to the seconds a check may run, or remove it for the default, 600",
	}}
}
