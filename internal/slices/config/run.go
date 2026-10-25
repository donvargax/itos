package config

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/engine/configcheck"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/value"
)

// Run is the config command over h: get's value, check's defaults, or
// check's findings and its exit, 2 for a config or ledger folder nothing can
// be read from, 1 for any other problem. An error is the command line's to
// report.
func (c CLI) Run(ctx context.Context, stdout, stderr io.Writer, h Handler) (int, error) {
	a, err := h.Handle(ctx, c.Query, Out{JSON: c.JSON, Tests: c.Tests})
	if err != nil {
		return 0, err
	}
	switch {
	case c.Sub == "get":
		return 0, c.got(stdout, a.Value)
	case c.Defaults:
		if c.JSON {
			return 0, out.Emit(stdout, out.Field{Key: "defaults", Value: a.Defaults})
		}
		_, err := fmt.Fprint(stdout, withStepsExample(value.YAML(a.Defaults)))
		return 0, err
	}
	return a.Code, c.checked(stdout, stderr, a)
}

// stepsExample is ci.steps as a comment, the same in every repository.
const stepsExample = `  # steps has no default. A step is a command, a command with its cost class,
  # or the run of a kind's named tests (itos help ci run). Each is given the
  # range's ends as ITOS_FROM and ITOS_TO, and as {from} and {to} in its text:
  # steps:
  #   - tools/lint-range {from} {to}
  #   - run: go vet ./...
  #     cost: static
  #   - tests: scenario
`

// withStepsExample is the defaults' YAML with stepsExample at the end of its
// ci mapping, the line before the next top-level key; unchanged without a ci.
func withStepsExample(yaml string) string {
	lines := strings.SplitAfter(yaml, "\n")
	at := slices.Index(lines, "ci:\n")
	if at < 0 {
		return yaml
	}
	end := at + 1
	for end < len(lines) && strings.HasPrefix(lines[end], " ") {
		end++
	}
	return strings.Join(lines[:end], "") + stepsExample + strings.Join(lines[end:], "")
}

// checked prints check's findings: under --json one object, else each
// problem as FAIL and each warning as WARN on stderr, then, when it found
// none and not under --quiet, the lines that say so.
func (c CLI) checked(stdout, stderr io.Writer, a Answer) error {
	found, warnings := a.Found, a.Warnings
	var err error
	if c.JSON {
		if found == nil {
			found = []configcheck.Found{}
		}
		fields := []out.Field{
			{Key: "config", Value: a.Path},
			{Key: "valid", Value: len(found) == 0},
			{Key: "problems", Value: found},
		}
		// Only when there is one, so a report without a warning is as it was.
		if len(warnings) > 0 {
			fields = append(fields, out.Field{Key: "warnings", Value: warnings})
		}
		err = out.Emit(stdout, fields...)
	} else {
		for _, p := range found {
			fmt.Fprintf(stderr, "FAIL %s\n", p.Message)
		}
		for _, p := range warnings {
			fmt.Fprintf(stderr, "WARN %s\n", p.Message)
		}
	}
	if len(found) == 0 && !c.JSON && !c.Quiet {
		for _, line := range a.Lines {
			fmt.Fprintln(stdout, line)
		}
	}
	return err
}

// got prints get's value: a scalar bare on one line, a mapping or a list as
// JSON on one line, no value nothing; under --json,
// {"schema":1,"key","value"}, value null for no value.
func (c CLI) got(stdout io.Writer, got any) error {
	if c.JSON {
		var v any = got
		if got == value.Undefined {
			v = nil
		}
		return out.Emit(stdout, out.Field{Key: "key", Value: c.Key}, out.Field{Key: "value", Value: v})
	}
	var err error
	switch got.(type) {
	case string, float64, bool:
		_, err = fmt.Fprintln(stdout, value.String(got))
	case *value.Map, []any:
		_, err = fmt.Fprintln(stdout, value.JSON(got))
	}
	return err
}
