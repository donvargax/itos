package task

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/out"
)

// Parse takes the subcommand off the line, keeping every argument for the
// checks' run, and refuses a word after next-id before anything is read.
func TestParse(t *testing.T) {
	for _, c := range []struct {
		args        []string
		json, quiet bool
		want        CLI
	}{
		{nil, false, false, CLI{}},
		{[]string{"T-1", "--pending"}, true, false, CLI{Query: Query{Args: []string{"T-1", "--pending"}, ToStderr: true}, JSON: true}},
		{[]string{"list", "--group", "1"}, false, false, CLI{Query: Query{Sub: "list", Args: []string{"--group", "1"}}}},
		{[]string{"add", "--group=1"}, false, true, CLI{Query: Query{Sub: "add", Args: []string{"--group=1"}}, Quiet: true}},
		{[]string{"next-id"}, true, false, CLI{Query: Query{Sub: "next-id", Args: []string{}, ToStderr: true}, JSON: true}},
		{[]string{"next-id", "--json"}, false, false, CLI{Query: Query{Sub: "next-id", Args: []string{"--json"}}}},
	} {
		got, err := Parse(c.args, c.json, c.quiet)
		if err != nil || !reflect.DeepEqual(got, c.want) {
			t.Errorf("Parse(%q, %v, %v) = %+v, %v, want %+v", c.args, c.json, c.quiet, got, err, c.want)
		}
	}
	for _, args := range [][]string{{"next-id", "T-1"}, {"next-id", "--json", "a", "b"}} {
		_, err := Parse(args, false, false)
		var u out.UsageError
		if !errors.As(err, &u) || err.Error() != "task next-id takes no arguments" {
			t.Errorf("Parse(%q): %v", args, err)
		}
	}
}

func addConfig() *config.Loaded {
	cfg := &config.Loaded{}
	cfg.Ledger.Group.Label = "phase"
	return cfg
}

func TestAddArgsParsesChecksAndTimeouts(t *testing.T) {
	n, err := AddArgs(addConfig(), []string{
		"--phase=1", "--type=chore", "--title=Title", "--why=Reason",
		"--check=true", "--timeout=2", "--check", "echo done", "--timeout=0.5",
	})
	if err != nil {
		t.Fatalf("AddArgs: %v", err)
	}
	if n.Group != "1" || n.Type != "chore" || n.Title != "Title" || n.Why != "Reason" {
		t.Fatalf("AddArgs fields = %#v", n)
	}
	if len(n.Checks) != 2 || n.Checks[0].Run != "true" || n.Checks[1].Run != "echo done" {
		t.Fatalf("AddArgs checks = %#v", n.Checks)
	}
	if n.Checks[0].Timeout == nil || *n.Checks[0].Timeout != 2 || n.Checks[1].Timeout == nil || *n.Checks[1].Timeout != 0.5 {
		t.Fatalf("AddArgs timeouts = %#v, want 2 and 0.5", n.Checks)
	}
}

func TestAddArgsRejectsInvalidCheckAndTimeoutOptions(t *testing.T) {
	base := []string{"--group", "1", "--type", "chore", "--title", "Title", "--why", "Reason"}
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "missing check", args: base, want: addUsage},
		{name: "missing group", args: []string{"--type", "chore", "--title", "Title", "--why", "Reason", "--check", "true"}, want: addUsage},
		{name: "missing type", args: []string{"--group", "1", "--title", "Title", "--why", "Reason", "--check", "true"}, want: addUsage},
		{name: "missing title", args: []string{"--group", "1", "--type", "chore", "--why", "Reason", "--check", "true"}, want: addUsage},
		{name: "missing why", args: []string{"--group", "1", "--type", "chore", "--title", "Title", "--check", "true"}, want: addUsage},
		{name: "unknown flag", args: append(append([]string{}, base...), "--unknown", "value"), want: "task add does not take --unknown"},
		{name: "timeout before check", args: append(append([]string{}, base...), "--timeout", "2"), want: "task add takes one --timeout"},
		{name: "empty check", args: append(append([]string{}, base...), "--check", " "), want: "task add --check needs a command"},
		{name: "invalid timeout", args: append(append(append([]string{}, base...), "--check", "true", "--timeout", "nope"), "--check", "echo done"), want: "task add takes one --timeout"},
		{name: "duplicate timeout", args: append(append(append([]string{}, base...), "--check", "true", "--timeout", "2", "--timeout", "3"), "--check", "echo done"), want: "task add takes one --timeout"},
		{name: "missing final option value", args: []string{"--group", "1", "--type", "chore", "--title", "Title", "--why", "Reason", "--check"}, want: "task add --check needs a value"},
		{name: "duplicate field", args: append(append([]string{}, base...), "--title", "Another title", "--check", "true"), want: "task add takes one --title"},
		{name: "blank field", args: []string{"--group", "1", "--type", "chore", "--title", " ", "--why", "Reason", "--check", "true"}, want: "task add takes one --title"},
		{name: "zero timeout", args: append(append([]string{}, base...), "--check", "true", "--timeout", "0"), want: "task add takes one --timeout"},
		{name: "negative timeout", args: append(append([]string{}, base...), "--check", "true", "--timeout", "-1"), want: "task add takes one --timeout"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := AddArgs(addConfig(), test.args)
			var u out.UsageError
			if !errors.As(err, &u) || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("AddArgs error = %v, want a usage error containing %q", err, test.want)
			}
		})
	}
}

func TestAddArgsRefusesCallerIDs(t *testing.T) {
	base := []string{"--group", "1", "--type", "chore", "--title", "Title", "--why", "Reason", "--check", "true"}
	for _, ids := range [][]string{{"T-004"}, {"T-004", "T-005"}} {
		_, err := AddArgs(addConfig(), append(append([]string{}, ids...), base...))
		if err == nil || err.Error() != "task add mints the task id; do not pass one" {
			t.Fatalf("AddArgs with %v = %v, want the caller-ID usage error", ids, err)
		}
	}
}
