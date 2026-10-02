package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/donvargax/itos/internal/config"
	"github.com/donvargax/itos/internal/out"
	"github.com/donvargax/itos/internal/shell"
	"github.com/donvargax/itos/internal/tests"
	"github.com/donvargax/itos/internal/value"
)

// testsList is `tests list <kind> [--at <tree>]` (tests-command.ts): the
// kind's adapter's listing at the working tree, the index or a commit, one
// test a line, or under --json the protocol's object as the adapter gave it.
func testsList(name, at string, o Out) (int, error) {
	cfg, err := config.Load(config.Path())
	if err != nil {
		return 0, err
	}
	if at == "" {
		at = "worktree"
	}
	list, err := tests.ListTests(cfg, name, at)
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

// smokeSet is the config and the kind's smoke set in the working tree.
func smokeSet(name string) (*config.Loaded, config.Kind, []tests.SmokeFile, error) {
	cfg, err := config.Load(config.Path())
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
	found, err := tests.SmokeIssues(cfg, name, smoke, root)
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

// smokeRun is `tests smoke run <kind> [-- <runner args>…]`: the kind's run
// of exactly the smoke set (its select template over the smoke IDs), the
// arguments each one shell word after it, through the config's shell with
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
