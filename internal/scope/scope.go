// Package scope is the path rules of commits.scopes (commit-scope.ts's
// scopeIssues): which paths each commit type may touch. `only`: every
// touched path must match; `never`: no touched path may match, unless it
// matches `except`, which takes paths back out of never alone; `must_touch`:
// at least one must. Its `$sets` are already expanded by the config's loader.
//
// It takes a type and its paths and gives the problems, so commit
// check-paths, the commit-msg hook (on the staged paths) and verify (on each
// commit's) share one judgement, and none of them goes through the command
// line to reach it.
package scope

import (
	"fmt"
	"io"
	"strings"

	"github.com/donvargax/itos/v4/internal/config"
	"github.com/donvargax/itos/v4/internal/glob"
	"github.com/donvargax/itos/v4/internal/out"
)

// Rules are a config's path rules, and the line a rejection starts with.
type Rules struct {
	scopes        config.Ordered[config.Scope]
	rejectMessage string
}

// Of is the config's path rules: a config error when the file has no
// commits section, as section("commits") is.
func Of(cfg *config.Loaded) (*Rules, error) {
	if err := cfg.Section("commits"); err != nil {
		return nil, err
	}
	return &Rules{scopes: cfg.Commits.Scopes, rejectMessage: cfg.Commits.RejectMessage}, nil
}

// Ruled is whether the type has path rules, even empty ones. One without
// (a merge, a revert, a type the config does not know) is the header lint's
// business, so the commit-msg hook runs neither the paths nor the staged
// range checks for it.
func (r *Rules) Ruled(typ string) bool {
	_, ok := r.scopes.Get(typ)
	return ok
}

// Issues is the path rules' problems for a type and its paths, in the paths'
// order (only, then never, for each), then must_touch's; none for a type
// with no rule. An error is a glob JavaScript would refuse, reached.
func (r *Rules) Issues(typ string, files []string) ([]out.Problem, error) {
	found := []out.Problem{}
	rules, ok := r.scopes.Get(typ)
	if !ok {
		return found, nil
	}
	for _, file := range files {
		if rules.Only != nil {
			ok, err := glob.MatchesAny(file, rules.Only)
			if err != nil {
				return nil, err
			}
			if !ok {
				p, err := r.split("scope-only", typ, file)
				if err != nil {
					return nil, err
				}
				found = append(found, p)
			}
		}
		if rules.Never != nil {
			ok, err := barred(rules, file)
			if err != nil {
				return nil, err
			}
			if ok {
				p, err := r.split("scope-never", typ, file)
				if err != nil {
					return nil, err
				}
				found = append(found, p)
			}
		}
	}
	if rules.MustTouch != nil {
		touched := false
		for _, file := range files {
			ok, err := glob.MatchesAny(file, rules.MustTouch)
			if err != nil {
				return nil, err
			}
			if ok {
				touched = true
				break
			}
		}
		if !touched {
			either := strings.Join(rules.MustTouch, " or ")
			found = append(found, out.Problem{
				Rule:    "scope-must-touch",
				Message: typ + " commits must change " + either,
				Fix:     "stage a change to " + either + ", or use the type that fits the change",
			})
		}
	}
	return found, nil
}

// split is a path's problem under only or never, its fix naming the types
// that would take the path.
func (r *Rules) split(rule, typ, file string) (out.Problem, error) {
	types, err := r.TypesFor(file)
	if err != nil {
		return out.Problem{}, err
	}
	return out.Problem{
		Rule:    rule,
		Message: typ + " commits may not touch " + file,
		Fix: "split the commit: stage " + file + " in a commit of another type (" +
			strings.Join(types, ", ") + ")",
	}, nil
}

// TypesFor is the types whose only and never (with its except) let a path
// through, in the
// order commits.scopes writes them: where a rejected path goes. A type
// without path rules is not among them.
func (r *Rules) TypesFor(file string) ([]string, error) {
	types := []string{}
	for _, typ := range r.scopes.Keys {
		rules := r.scopes.Values[typ]
		if rules.Only != nil {
			ok, err := glob.MatchesAny(file, rules.Only)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
		}
		if rules.Never != nil {
			ok, err := barred(rules, file)
			if err != nil {
				return nil, err
			}
			if ok {
				continue
			}
		}
		types = append(types, typ)
	}
	return types, nil
}

// barred is whether a scope's never refuses a path: it matches never and
// not except.
func barred(rules config.Scope, file string) (bool, error) {
	ok, err := glob.MatchesAny(file, rules.Never)
	if err != nil || !ok || rules.Except == nil {
		return ok, err
	}
	excepted, err := glob.MatchesAny(file, rules.Except)
	return !excepted, err
}

// Reject prints the rejection as the hook has always printed it:
// commits.reject_message, then each problem's sentence.
func (r *Rules) Reject(w io.Writer, found []out.Problem) {
	fmt.Fprintln(w, r.rejectMessage)
	for _, p := range found {
		fmt.Fprintf(w, "  - %s\n", p.Message)
	}
}
