package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/ids"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/shell"
	"github.com/donvargax/itos/v8/internal/tests"
	"github.com/donvargax/itos/v8/internal/value"
)

func runTests(args []string, o Out) (int, error) {
	sub, rest := split(args)
	switch sub {
	case "list":
		name, ok := first(positional(rest, "--at"))
		if !ok {
			return 0, usage("tests list needs <kind>")
		}
		at, _ := flagValue(rest, "--at")
		if at != "worktree" && at != "index" {
			if err := commitRefs("tests list", at); err != nil {
				return 0, err
			}
		}
		return testsList(name, at, o)
	case "moves":
		name, ok := first(positional(rest))
		if !ok {
			return 0, usage("tests moves needs <kind>")
		}
		return testsMoves(name, o)
	case "next-id":
		return testsNextID(rest, o)
	case "smoke":
	default:
		return 0, usage("unknown command: tests %s", sub)
	}
	action, more := split(rest)
	name, _ := first(positional(more, "--features"))
	if name == "" {
		if len(rest) == 0 {
			action = "check|ids|run"
		}
		return 0, usage("tests smoke %s needs <kind>", action)
	}
	switch action {
	case "check":
		var root *string
		if features, ok := flagValue(more, "--features"); ok {
			features = typed(o.Origin, features)
			root = &features
		}
		return smokeCheck(name, root, o)
	case "ids":
		return smokeIDs(name, o)
	case "run":
		// Everything after the kind is the runner's, as given; a -- right
		// after it is taken, so what looks like an itos flag can follow it,
		// and is not passed on (bug 8: they were read only after a --, and
		// --workers=1 without one was dropped without a word).
		runner := more[slices.Index(more, name)+1:]
		if len(runner) > 0 && runner[0] == "--" {
			runner = runner[1:]
		}
		return smokeRun(name, runner, o)
	}
	return 0, usage("unknown command: tests smoke %s", action)
}

// testsList is `tests list <kind> [--at <tree>]` (tests-command.ts): the
// kind's adapter's listing at the working tree, the index or a commit, one
// test a line, or under --json the protocol's object as the adapter gave it.
func testsList(name, at string, o Out) (int, error) {
	cfg, err := loadConfig()
	if err != nil {
		return 0, err
	}
	if at == "" {
		at = "worktree"
	}
	list, err := tests.ListTests(cfg, name, at, o.Tests)
	if err != nil {
		return 0, err
	}
	if o.JSON {
		return 0, out.Emit(o.Stdout, listFields(list)...)
	}
	for _, t := range list.Tests {
		live := ""
		if !t.Live {
			live = "  (not live)"
		}
		fmt.Fprintf(o.Stdout, "%s  %s%s\n", t.ID, t.File, live)
	}
	return 0, nil
}

// listFields are a listing's keys in order: a command adapter's object as it
// printed it, each value as JSON.stringify writes it, else the protocol's.
func listFields(list tests.List) []out.Field {
	raw, ok := list.Raw.(*value.Map)
	if !ok {
		return []out.Field{
			{Key: "protocol", Value: list.Protocol},
			{Key: "tests", Value: list.Tests},
			{Key: "files", Value: list.Files},
		}
	}
	fields := []out.Field{}
	for _, k := range raw.Keys() {
		fields = append(fields, out.Field{Key: k, Value: json.RawMessage(value.JSON(raw.At(k)))})
	}
	return fields
}

// testsMoves is `tests moves <kind>` (moves.ts's testsMoves): the staged
// feature files against HEAD's by the built-in moves rule, by the kind's
// moves checks' allowed renames. 0 when they only move scenarios, if
// anything, 1 with each problem on stderr or under --json.
func testsMoves(name string, o Out) (int, error) {
	cfg, err := loadConfig()
	if err != nil {
		return 0, err
	}
	found, err := tests.NewMoves(cfg, o.Tests).Index(name)
	if err != nil {
		return 0, err
	}
	switch {
	case o.JSON:
		if err := out.Emit(o.Stdout,
			out.Field{Key: "kind", Value: name},
			out.Field{Key: "ok", Value: len(found) == 0},
			out.Field{Key: "problems", Value: found}); err != nil {
			return 0, err
		}
	case len(found) > 0:
		for _, p := range found {
			fmt.Fprintf(o.Stderr, "  - %s\n", p.Message)
		}
	case !o.Quiet:
		fmt.Fprintln(o.Stdout, "The staged feature files only move scenarios, if anything")
	}
	if len(found) > 0 {
		return ExitPolicy, nil
	}
	return 0, nil
}

// smokeSet is the config and the kind's smoke set in the working tree.
func smokeSet(name string) (*config.Loaded, config.Kind, []tests.SmokeFile, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, config.Kind{}, nil, err
	}
	smoke, err := tests.LoadSmoke(cfg, name)
	if err != nil {
		return nil, config.Kind{}, nil, err
	}
	k, err := tests.KindOf(cfg, name)
	return cfg, k, smoke, err
}

// smokeCheck is `tests smoke check <kind> [--features <dir>]`
// (smoke-rule.ts's smokeCheck): the smoke rule over the working tree, or
// over a copy of a Gherkin kind's files in <dir>. 0 when it holds, 1 when it
// does not, each problem a FAIL line on stderr or under --json.
func smokeCheck(name string, root *string, o Out) (int, error) {
	cfg, k, smoke, err := smokeSet(name)
	if err != nil {
		return 0, err
	}
	found, err := tests.SmokeIssues(cfg, name, smoke, root, o.Tests)
	if err != nil {
		return 0, err
	}
	if found == nil {
		found = []out.Problem{}
	}
	count := len(tests.SmokeIDs(k, smoke))
	if o.JSON {
		if err := out.Emit(o.Stdout,
			out.Field{Key: "kind", Value: name},
			out.Field{Key: "ok", Value: len(found) == 0},
			out.Field{Key: "ids", Value: count},
			out.Field{Key: "problems", Value: found}); err != nil {
			return 0, err
		}
	} else {
		for _, p := range found {
			fmt.Fprintf(o.Stderr, "FAIL %s\n", p.Message)
		}
	}
	if len(found) > 0 {
		return ExitPolicy, nil
	}
	if !o.JSON && !o.Quiet {
		if k.Smoke.EveryFile {
			fmt.Fprintf(o.Stdout, "Every feature file has a smoke scenario (%d in all)\n", count)
		} else {
			fmt.Fprintf(o.Stdout, "Every smoke scenario is live (%d in all)\n", count)
		}
	}
	return 0, nil
}

// smokeIDs is `tests smoke ids <kind>`: the smoke set's IDs without the tag
// prefix, one a line.
func smokeIDs(name string, o Out) (int, error) {
	_, k, smoke, err := smokeSet(name)
	if err != nil {
		return 0, err
	}
	ids := tests.SmokeIDs(k, smoke)
	if o.JSON {
		return 0, out.Emit(o.Stdout, out.Field{Key: "kind", Value: name}, out.Field{Key: "ids", Value: ids})
	}
	for _, id := range ids {
		fmt.Fprintln(o.Stdout, id)
	}
	return 0, nil
}

// smokeRun is `tests smoke run <kind> [--] [<runner args>…]`: the kind's run
// of exactly the smoke set (its select template over the smoke IDs), the
// arguments (everything after the kind but a -- right after it) each one
// shell word after it, through the config's shell with
// itos's streams. Its exit code is the runner's, 1 when the runner did not
// exit by itself, as spawnSync's `status ?? 1`. An empty smoke set runs
// nothing, and says so.
func smokeRun(name string, args []string, o Out) (int, error) {
	cfg, k, smoke, err := smokeSet(name)
	if err != nil {
		return 0, err
	}
	run, ok, err := tests.CommandFor(cfg, name, []tests.Selection{tests.IDs(tests.SmokeIDs(k, smoke)...)})
	if err != nil {
		return 0, err
	}
	if !ok {
		fmt.Fprintf(o.Stderr, "The smoke set of tests.%s is empty: there is nothing to run\n", name)
		return 0, nil
	}
	words := []string{run}
	for _, a := range args {
		words = append(words, tests.ShellWord(a))
	}
	result := shell.Run(cfg, strings.Join(words, " "),
		shell.Options{Stdin: os.Stdin, Stdout: o.Stdout, Stderr: o.Stderr})
	if result.Code < 0 || result.Err != nil {
		return ExitPolicy, nil
	}
	return result.Code, nil
}

// testsNextID is `tests next-id <kind> <stem> [--count <n>]`: the kind's
// next free tag of the stem (tests.NextTag), counting the registry's ids
// too. Of an area it claims the tag, or n tags in a row, through the id
// counter (claimTags), each printed; of any other stem it only reads, so
// --count there is a usage error, as is a count that is no number from 1 up
// and a kind the config does not have.
func testsNextID(args []string, o Out) (int, error) {
	words := positional(args, "--count")
	if len(words) != 2 {
		return 0, usage("tests next-id needs <kind> <stem>")
	}
	name, stem := words[0], words[1]
	text, counted := flagValue(args, "--count")
	count, err := strconv.Atoi(text)
	if !counted {
		count, err = 1, nil
	}
	if err != nil || count < 1 {
		return 0, usage("tests next-id --count takes how many ids to claim, a number from 1 up, not %q", text)
	}
	cfg, err := loadConfig()
	if err != nil {
		return 0, err
	}
	k, ok := cfg.Tests.Get(name)
	if !ok {
		return 0, usage("tests next-id: the config has no tests kind %s (it has %s)", name, kindNames(cfg))
	}
	area := tests.Area(k, stem)
	if counted && !area {
		return 0, usage("tests next-id --count claims an area's ids, and %s is no area of tests.%s's id: its next id is only read", stem, name)
	}
	others, err := registryIDs(cfg)
	if err != nil {
		return 0, err
	}
	next, err := tests.NextTag(cfg, name, stem, others)
	if err != nil {
		return 0, err
	}
	ids := []string{next}
	if area {
		if ids, err = claimTags(cfg, stem, next, count, o.OS); err != nil {
			return 0, err
		}
	}
	if o.JSON {
		return 0, out.Emit(o.Stdout,
			out.Field{Key: "kind", Value: name},
			out.Field{Key: "stem", Value: stem},
			out.Field{Key: "id", Value: ids[0]},
			out.Field{Key: "ids", Value: ids},
			out.Field{Key: "claimed", Value: area})
	}
	fmt.Fprintln(o.Stdout, strings.Join(ids, "\n"))
	return 0, nil
}

// claimTags claims count numbers of the area stem through the tracker of
// the config (tracker.go): ids.Claim.
func claimTags(cfg *config.Loaded, stem, next string, count int, goos string) ([]string, error) {
	return ids.Claim(context.Background(), tracker(cfg, goos), stem, next, count)
}

// kindNames are the config's tests kinds, as a usage error lists them.
func kindNames(cfg *config.Loaded) string {
	if len(cfg.Tests.Keys) == 0 {
		return "none"
	}
	return strings.Join(cfg.Tests.Keys, ", ")
}
