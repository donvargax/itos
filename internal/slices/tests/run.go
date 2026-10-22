package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/donvargax/itos/v8/internal/out"
	namedtests "github.com/donvargax/itos/v8/internal/tests"
	"github.com/donvargax/itos/v8/internal/value"
)

// refused is the policy failure's exit code (itos --help): a rule that does
// not hold, or a runner that did not exit by itself.
const refused = 1

// Run is the tests command over h: list's tests one a line, or under --json
// the protocol's object as the adapter gave it; the moves rule's and the
// smoke rule's problems on stderr, or under --json in the one object, exit
// 1; the smoke set's IDs; next-id's tags one a line; smoke run's exit the
// runner's. An error is the command line's to report.
func (c CLI) Run(ctx context.Context, stdout, stderr io.Writer, h Handler) (int, error) {
	a, err := h.Handle(ctx, c.Query)
	if err != nil {
		return 0, err
	}
	switch c.Sub {
	case "list":
		return c.list(stdout, a.List)
	case "moves":
		return c.moves(stdout, stderr, a.Problems)
	case "next-id":
		return c.nextID(stdout, a)
	}
	switch c.Action {
	case "check":
		return c.smokeCheck(stdout, stderr, a)
	case "ids":
		return c.smokeIDs(stdout, a.IDs)
	}
	if a.Empty {
		fmt.Fprintf(stderr, "The smoke set of tests.%s is empty: there is nothing to run\n", c.Kind)
	}
	return a.Code, nil
}

// list prints the listing, one test a line, or under --json its object.
func (c CLI) list(stdout io.Writer, list namedtests.List) (int, error) {
	if c.JSON {
		return 0, out.Emit(stdout, listFields(list)...)
	}
	for _, t := range list.Tests {
		live := ""
		if !t.Live {
			live = "  (not live)"
		}
		fmt.Fprintf(stdout, "%s  %s%s\n", t.ID, t.File, live)
	}
	return 0, nil
}

// listFields are a listing's keys in order: a command adapter's object as it
// printed it, each value as JSON.stringify writes it, else the protocol's.
func listFields(list namedtests.List) []out.Field {
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

// moves prints the moves rule's judgement: 0 when the staged feature files
// only move scenarios, if anything, 1 with each problem on stderr or under
// --json.
func (c CLI) moves(stdout, stderr io.Writer, found []out.Problem) (int, error) {
	switch {
	case c.JSON:
		if err := out.Emit(stdout,
			out.Field{Key: "kind", Value: c.Kind},
			out.Field{Key: "ok", Value: len(found) == 0},
			out.Field{Key: "problems", Value: found}); err != nil {
			return 0, err
		}
	case len(found) > 0:
		for _, p := range found {
			fmt.Fprintf(stderr, "  - %s\n", p.Message)
		}
	case !c.Quiet:
		fmt.Fprintln(stdout, "The staged feature files only move scenarios, if anything")
	}
	if len(found) > 0 {
		return refused, nil
	}
	return 0, nil
}

// smokeCheck prints the smoke rule's judgement: 0 when it holds, 1 when it
// does not, each problem a FAIL line on stderr or under --json.
func (c CLI) smokeCheck(stdout, stderr io.Writer, a Answer) (int, error) {
	if c.JSON {
		if err := out.Emit(stdout,
			out.Field{Key: "kind", Value: c.Kind},
			out.Field{Key: "ok", Value: len(a.Problems) == 0},
			out.Field{Key: "ids", Value: a.Count},
			out.Field{Key: "problems", Value: a.Problems}); err != nil {
			return 0, err
		}
	} else {
		for _, p := range a.Problems {
			fmt.Fprintf(stderr, "FAIL %s\n", p.Message)
		}
	}
	if len(a.Problems) > 0 {
		return refused, nil
	}
	if !c.JSON && !c.Quiet {
		if a.EveryFile {
			fmt.Fprintf(stdout, "Every feature file has a smoke scenario (%d in all)\n", a.Count)
		} else {
			fmt.Fprintf(stdout, "Every smoke scenario is live (%d in all)\n", a.Count)
		}
	}
	return 0, nil
}

// smokeIDs prints the smoke set's IDs, one a line.
func (c CLI) smokeIDs(stdout io.Writer, ids []string) (int, error) {
	if c.JSON {
		return 0, out.Emit(stdout, out.Field{Key: "kind", Value: c.Kind}, out.Field{Key: "ids", Value: ids})
	}
	for _, id := range ids {
		fmt.Fprintln(stdout, id)
	}
	return 0, nil
}

// nextID prints the tags next-id found, or claimed, one a line.
func (c CLI) nextID(stdout io.Writer, a Answer) (int, error) {
	if c.JSON {
		return 0, out.Emit(stdout,
			out.Field{Key: "kind", Value: c.Kind},
			out.Field{Key: "stem", Value: c.Stem},
			out.Field{Key: "id", Value: a.IDs[0]},
			out.Field{Key: "ids", Value: a.IDs},
			out.Field{Key: "claimed", Value: a.Claimed})
	}
	fmt.Fprintln(stdout, strings.Join(a.IDs, "\n"))
	return 0, nil
}
