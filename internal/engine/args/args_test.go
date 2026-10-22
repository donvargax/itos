package args

import (
	"errors"
	"reflect"
	"testing"

	"github.com/donvargax/itos/v8/internal/out"
)

func TestFlagValueIsTheArgumentAfterTheFlag(t *testing.T) {
	for _, c := range []struct {
		args  []string
		value string
		ok    bool
	}{
		{[]string{"--at", "HEAD", "x"}, "HEAD", true},
		{[]string{"x", "--at", "HEAD"}, "HEAD", true},
		{[]string{"x", "--at"}, "", false},
		{[]string{"x", "HEAD"}, "", false},
		{nil, "", false},
	} {
		if value, ok := FlagValue(c.args, "--at"); value != c.value || ok != c.ok {
			t.Errorf("FlagValue(%q, --at) = %q, %v, want %q, %v", c.args, value, ok, c.value, c.ok)
		}
	}
}

func TestPositionalLeavesOutFlagsAndTheValuedFlagsValues(t *testing.T) {
	for _, c := range []struct {
		args, valued, want []string
	}{
		{[]string{"a", "--at", "HEAD", "b", "--yes", "c"}, []string{"--at"}, []string{"a", "b", "c"}},
		{[]string{"--yes", "a", "-", "b"}, nil, []string{"a", "b"}},
		{[]string{"--at", "a", "--count", "2", "b"}, []string{"--at", "--count"}, []string{"b"}},
		{[]string{"--at"}, []string{"--at"}, nil},
		{[]string{"--at", "a"}, []string{"--count"}, []string{"a"}},
	} {
		if got := Positional(c.args, c.valued...); !reflect.DeepEqual(got, c.want) {
			t.Errorf("Positional(%q, %q) = %q, want %q", c.args, c.valued, got, c.want)
		}
	}
}

func TestFirstIsTheFirstElementWhenThereIsOne(t *testing.T) {
	if got, ok := First([]string{"a", "b"}); got != "a" || !ok {
		t.Errorf("First(a b) = %q, %v", got, ok)
	}
	if got, ok := First(nil); got != "" || ok {
		t.Errorf("First(nil) = %q, %v", got, ok)
	}
}

func TestSplitIsTheSubcommandAndTheArgumentsAfterIt(t *testing.T) {
	if sub, rest := Split([]string{"list", "a", "--at", "b"}); sub != "list" || !reflect.DeepEqual(rest, []string{"a", "--at", "b"}) {
		t.Errorf("Split(list a --at b) = %q, %q", sub, rest)
	}
	if sub, rest := Split([]string{"list"}); sub != "list" || rest == nil || len(rest) != 0 {
		t.Errorf("Split(list) = %q, %#v, want list and an empty list", sub, rest)
	}
	if sub, rest := Split(nil); sub != "" || rest != nil {
		t.Errorf("Split(nil) = %q, %#v", sub, rest)
	}
}

func TestSubReadsPositionalsValuesAndSwitches(t *testing.T) {
	valued, switches := []string{"--id", "--why"}, []string{"--yes", "--json"}
	for _, c := range []struct {
		args   []string
		pos    []string
		values map[string]string
		set    map[string]bool
	}{
		{nil, nil, map[string]string{}, map[string]bool{}},
		{[]string{"a", "-", "b"}, []string{"a", "-", "b"}, map[string]string{}, map[string]bool{}},
		{
			[]string{"a", "--id", "T-1", "--why=because so", "--yes", "b"},
			[]string{"a", "b"},
			map[string]string{"--id": "T-1", "--why": "because so"},
			map[string]bool{"--yes": true},
		},
		{
			[]string{"--json", "--", "--id", "--yes", "c"},
			[]string{"--id", "--yes", "c"},
			map[string]string{},
			map[string]bool{"--json": true},
		},
		{[]string{"--id", "--yes"}, nil, map[string]string{"--id": "--yes"}, map[string]bool{}},
	} {
		pos, values, set, err := Sub("work", "add", c.args, valued, switches)
		if err != nil || !reflect.DeepEqual(pos, c.pos) || !reflect.DeepEqual(values, c.values) || !reflect.DeepEqual(set, c.set) {
			t.Errorf("Sub(%q) = %q, %v, %v, %v, want %q, %v, %v", c.args, pos, values, set, err, c.pos, c.values, c.set)
		}
	}
}

// Each refusal is a usage error, which the command line prints naming the
// help.
func TestSubRefusesWhatTheSubcommandDoesNotTake(t *testing.T) {
	valued, switches := []string{"--id"}, []string{"--yes"}
	for _, c := range []struct {
		command, sub string
		args         []string
		want         string
	}{
		{"work", "add", []string{"--yes", "--yes"}, "work add takes one --yes"},
		{"work", "add", []string{"--yes=1"}, "work add does not take --yes"},
		{"work", "add", []string{"a", "--id"}, "work add --id needs a value"},
		{"work", "add", []string{"--id="}, "work add takes one --id with a value"},
		{"work", "add", []string{"--id", " "}, "work add takes one --id with a value"},
		{"work", "add", []string{"--id", "a", "--id=b"}, "work add takes one --id with a value"},
		{"work", "add", []string{"--other"}, "work add does not take --other"},
		{"work", "add", []string{"-x=1"}, "work add does not take -x"},
		{"followup", "", []string{"--other"}, "followup does not take --other"},
	} {
		pos, values, set, err := Sub(c.command, c.sub, c.args, valued, switches)
		var u out.UsageError
		if !errors.As(err, &u) || err.Error() != c.want || pos != nil || values != nil || set != nil {
			t.Errorf("Sub(%q, %q, %q) = %q, %v, %v, %v, want the usage error %q", c.command, c.sub, c.args, pos, values, set, err, c.want)
		}
	}
}
