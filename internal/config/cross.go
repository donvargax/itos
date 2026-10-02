package config

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/donvargax/itos/internal/out"
	"github.com/donvargax/itos/internal/value"
)

// crossProblems is what the schema cannot say, over the file as written:
// names that must refer to something, patterns that must compile.
func crossProblems(tree *value.Map, c *Config) []out.Problem {
	var found []out.Problem
	for _, check := range []func(*value.Map, *Config) []out.Problem{
		sinceProblems, scopeProblems, footerProblems, stepProblems,
		patternProblems, providerProblems, hookProblems, rangeCheckProblems,
	} {
		found = append(found, check(tree, c)...)
	}
	return found
}

// FullSHA is a full commit SHA, SHA-1 or SHA-256, as git prints it.
var FullSHA = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

// sinceProblems: commits.since is a full SHA, since an abbreviation can grow
// ambiguous and a branch or tag can move. Whether the repository has that
// commit is config check's question (SinceIssue), since a shallow clone may
// not.
func sinceProblems(_ *value.Map, c *Config) []out.Problem {
	since := c.Commits.Since
	if since == nil || FullSHA.MatchString(*since) {
		return nil
	}
	return []out.Problem{{
		Rule:    "config-since",
		Message: "commits.since is not the full SHA of a commit: " + *since,
		Fix:     "set commits.since to the commit's full SHA, as `git rev-parse <commit>` prints it",
	}}
}

func scopeProblems(_ *value.Map, c *Config) []out.Problem {
	types := c.Commits.Types
	if types == nil {
		return nil
	}
	var found []out.Problem
	for _, typ := range c.Commits.Scopes.Keys {
		if !slices.Contains(types, typ) {
			found = append(found, out.Problem{
				Rule:    "config-scope-type",
				Message: "commits.scopes." + typ + " is not one of commits.types",
				Fix:     "add " + typ + " to commits.types, or remove commits.scopes." + typ,
			})
		}
	}
	return found
}

// footerProblems: a footer's source must be the ledger or a kind of tests the
// config has, and the types it names must be commit types.
func footerProblems(_ *value.Map, c *Config) []out.Problem {
	var found []out.Problem
	for _, key := range c.Commits.Footers.Keys {
		f := c.Commits.Footers.Values[key]
		where := "commits.footers." + key
		found = append(found, sourceProblems(c, where, f.Source)...)
		found = append(found, namedTypeProblems(c, where+".required_for", f.RequiredFor)...)
		found = append(found, namedTypeProblems(c, where+".validate_for", f.ValidateFor)...)
	}
	return found
}

func sourceProblems(c *Config, where string, s FooterSource) []out.Problem {
	if s.IsName {
		if s.Name == "ledger" {
			return nil
		}
		return []out.Problem{{
			Rule:    "config-footer-source",
			Message: where + ".source is ledger or { tests: <kind> }",
			Fix:     "set " + where + ".source to ledger or { tests: <kind> }",
		}}
	}
	if _, ok := c.Tests.Get(s.Tests); ok {
		return nil
	}
	return []out.Problem{{
		Rule:    "config-footer-source",
		Message: where + ".source names tests." + s.Tests + ", which the config does not have",
		Fix:     "add tests." + s.Tests + ", or name a kind tests: has",
	}}
}

func namedTypeProblems(c *Config, where string, named *Types) []out.Problem {
	if named == nil || (!named.IsList && named.Word == "all") {
		return nil
	}
	if !named.IsList {
		return []out.Problem{{
			Rule:    "config-footer-types",
			Message: where + " is a list of types or all",
			Fix:     "write " + where + " as a list of commit types, or all",
		}}
	}
	types := c.Commits.Types
	if types == nil {
		types = named.List
	}
	var found []out.Problem
	for _, typ := range named.List {
		if !slices.Contains(types, typ) {
			found = append(found, out.Problem{
				Rule:    "config-footer-types",
				Message: where + " names " + typ + ", which is not one of commits.types",
				Fix:     "add " + typ + " to commits.types, or remove it from " + where,
			})
		}
	}
	return found
}

// stepProblems: a step runs a command, a kind's tests or, in the nightly
// alone, the checks of the done tasks, since a push runs the checks of the
// tasks its commits name.
func stepProblems(_ *value.Map, c *Config) []out.Problem {
	var found []out.Problem
	for _, s := range c.CI.Steps {
		found = append(found, oneStepProblems(c, s, false)...)
	}
	if c.CI.Nightly != nil {
		for _, s := range c.CI.Nightly.Steps {
			found = append(found, oneStepProblems(c, s, true)...)
		}
	}
	return found
}

func oneStepProblems(c *Config, s Step, nightly bool) []out.Problem {
	if s.Text != nil {
		return nil
	}
	if !nightly && s.Tasks != nil {
		return []out.Problem{{
			Rule:    "config-step-tasks",
			Message: fmt.Sprintf("a CI step runs tasks: %s, which only a nightly step does: %s", *s.Tasks, s.JSON),
			Fix:     "move the step to ci.nightly.steps; a push runs the checks of the tasks its commits name",
		}}
	}
	given := 0
	for _, v := range []*string{s.Run, s.Tests, s.Tasks} {
		if v != nil {
			given++
		}
	}
	if given != 1 {
		if nightly {
			return []out.Problem{{
				Rule:    "config-step",
				Message: "a nightly step needs exactly one of run, tests and tasks: " + s.JSON,
				Fix:     "give the step one of run: <command>, tests: <kind> or tasks: done",
			}}
		}
		return []out.Problem{{
			Rule:    "config-step",
			Message: "a CI step needs exactly one of run and tests: " + s.JSON,
			Fix:     "give the step either run: <command> or tests: <kind>, not both",
		}}
	}
	if s.Tasks != nil {
		if s.Cost != nil && *s.Cost == "late" {
			return []out.Problem{{
				Rule:    "config-step-cost",
				Message: "a nightly step runs tasks: " + *s.Tasks + " with cost: late; its cost is static or left out",
				Fix:     "write cost: static to run only the static checks, or leave cost out to run them all",
			}}
		}
		return nil
	}
	if s.Tests == nil {
		return nil
	}
	if k, ok := c.Tests.Get(*s.Tests); ok && k.Run.Whole != nil && *k.Run.Whole != "" {
		return nil
	}
	return []out.Problem{{
		Rule:    "config-step-tests",
		Message: "a CI step runs tests: " + *s.Tests + ", and tests." + *s.Tests + ".run.whole is missing",
		Fix:     "add tests." + *s.Tests + ".run.whole, the command that runs every " + *s.Tests,
	}}
}

// regexpProblem is a pattern that does not compile. The patterns were written
// for JavaScript's regular expressions, which the message names; Go compiles
// them as RE2, which lacks lookaround and backreferences.
func regexpProblem(pattern, where string) []out.Problem {
	if _, err := regexp.Compile(pattern); err == nil {
		return nil
	}
	return []out.Problem{{
		Rule:    "config-regexp",
		Message: where + " is not a regular expression: " + pattern,
		Fix:     "correct " + where + " so that it compiles as a JavaScript regular expression",
	}}
}

func patternProblems(tree *value.Map, c *Config) []out.Problem {
	var found []out.Problem
	for i, p := range c.CI.Cost.Static {
		found = append(found, regexpProblem(p, fmt.Sprintf("ci.cost.static[%d]", i))...)
	}
	for i, rule := range c.CI.Covers {
		found = append(found, regexpProblem(rule.Matches, fmt.Sprintf("ci.covers[%d].matches", i))...)
	}
	if !tree.Has("ledger") {
		return found
	}
	ledger := c.Ledger
	if ledger.ID != nil && *ledger.ID != "" {
		found = append(found, regexpProblem(*ledger.ID, "ledger.id")...)
	}
	if ledger.Group.Pattern != "" {
		found = append(found, regexpProblem(ledger.Group.Pattern, "ledger.group.pattern")...)
	}
	if !strings.Contains(ledger.Files, "{group}") {
		found = append(found, out.Problem{
			Rule:    "config-ledger-files",
			Message: "ledger.files has no {group}: " + ledger.Files,
			Fix:     "write {group} in ledger.files where a file's group stands, as in tasks/phase-{group}.yaml",
		})
	}
	return found
}

// providerProblems: a command provider needs its command, and only the
// markdown table reads logins out of links.
func providerProblems(_ *value.Map, c *Config) []out.Problem {
	var found []out.Problem
	needsCommand := func(where, provider string, command *string) {
		if provider == "command" && (command == nil || value.Trim(*command) == "") {
			found = append(found, out.Problem{
				Rule:    "config-provider-command",
				Message: where + ".provider is command, and " + where + ".command is missing",
				Fix:     "add " + where + ".command, or choose another " + where + ".provider",
			})
		}
	}
	needsCommand("ci.range", c.CI.Range.Provider, c.CI.Range.Command)
	needsCommand("work.identity", c.Work.Identity.Provider, c.Work.Identity.Command)
	people := c.Work.People
	if people.LoginFrom == nil {
		return found
	}
	switch {
	case people.Source != "all-contributors-md":
		found = append(found, out.Problem{
			Rule:    "config-login-from",
			Message: "work.people.login_from is read only by all-contributors-md",
			Fix:     "remove work.people.login_from",
		})
	case strings.Count(*people.LoginFrom, "{login}") != 1:
		found = append(found, out.Problem{
			Rule:    "config-login-from",
			Message: "work.people.login_from needs one {login}: " + *people.LoginFrom,
			Fix:     "write {login} once in work.people.login_from, where the login stands in the link",
		})
	}
	return found
}

// hookProblems: a check_timeout of no seconds would be no timeout at all to
// the shell.
func hookProblems(_ *value.Map, c *Config) []out.Problem {
	seconds := c.Hooks.CommitMsg.CheckTimeout
	if seconds == nil || *seconds > 0 {
		return nil
	}
	return []out.Problem{{
		Rule:    "config-check-timeout",
		Message: "hooks.commit_msg.check_timeout is a number of seconds above 0, not " + value.Number(*seconds),
		Fix:     "set hooks.commit_msg.check_timeout to the seconds a check may hold a commit, or set hooks.commit_msg.task_checks to false",
	}}
}

// rangeCheckProblems: a built-in range check runs no command, and the moves
// rule reads feature files, so it needs a Gherkin kind; its renames are read
// by it alone.
func rangeCheckProblems(_ *value.Map, c *Config) []out.Problem {
	var found []out.Problem
	for _, name := range c.Tests.Keys {
		k := c.Tests.Values[name]
		for i, check := range k.RangeChecks {
			found = append(found, oneRangeCheckProblems(name, k.Adapter, check, fmt.Sprintf("tests.%s.range_checks[%d]", name, i))...)
		}
	}
	return found
}

func oneRangeCheckProblems(name string, adapter Adapter, check RangeCheck, at string) []out.Problem {
	refused := func(message, fix string) out.Problem {
		return out.Problem{Rule: "config-range-check-builtin", Message: at + " " + message, Fix: fix}
	}
	if check.Builtin == nil {
		if check.AllowedRenames != nil {
			return []out.Problem{refused(
				"has allowed_renames, which only builtin: moves reads",
				"remove allowed_renames from "+at+", or make it builtin: moves",
			)}
		}
		return nil
	}
	var found []out.Problem
	var commands []string
	if check.Staged != nil {
		commands = append(commands, "staged")
	}
	if check.Range != nil {
		commands = append(commands, "range")
	}
	if len(commands) > 0 {
		both := strings.Join(commands, " and ")
		found = append(found, refused(
			"is builtin: "+*check.Builtin+" and has "+both+"; a built-in check runs no command",
			"remove "+both+" from "+at+", or builtin to run the commands",
		))
	}
	if adapter.Given && (adapter.Name != "gherkin" || adapter.Command != "") {
		found = append(found, refused(
			"is builtin: "+*check.Builtin+", which reads feature files, and tests."+name+".adapter is not gherkin",
			"remove "+at+", or give it staged and range commands that judge the kind's tests",
		))
	}
	return found
}
