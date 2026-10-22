package ci

import (
	"errors"
	"testing"

	"github.com/donvargax/itos/v8/internal/out"
)

func TestParseReadsEachSubcommand(t *testing.T) {
	for _, c := range []struct {
		args []string
		want Query
	}{
		{[]string{"plan", "a", "b"}, Query{Sub: "plan", From: "a", To: "b"}},
		{[]string{"plan", "", "b"}, Query{Sub: "plan", To: "b"}},
		{[]string{"plan"}, Query{Sub: "plan", Bare: true}},
		{[]string{"plan", "--nightly"}, Query{Sub: "plan", Nightly: true}},
		{[]string{"plan", "--whole", "--data-at", "c"}, Query{Sub: "plan", Whole: true, DataAt: "c"}},
		{[]string{"plan", "--data-at", "c", "a", "b"}, Query{Sub: "plan", From: "a", To: "b", DataAt: "c"}},
		{[]string{"run"}, Query{Sub: "run"}},
		{[]string{"run", "a", "b", "--tests"}, Query{Sub: "run", From: "a", To: "b", Tests: true}},
		{[]string{"run", "--nightly", "--tests"}, Query{Sub: "run", Nightly: true, Tests: true}},
		{[]string{"scope", "a", "b"}, Query{Sub: "scope", From: "a", To: "b"}},
		{[]string{"watch"}, Query{Sub: "watch", Rev: "HEAD"}},
		{[]string{"watch", "a"}, Query{Sub: "watch", From: "a", Rev: "a"}},
		{[]string{"range", "--head", "h"}, Query{Sub: "range", Head: "h"}},
		{[]string{"range", "--base", "b", "--head", "h"}, Query{Sub: "range", Head: "h", Base: "b"}},
	} {
		got, err := Parse(c.args, true, true)
		if err != nil || !got.JSON || !got.Quiet || got.Query != c.want {
			t.Errorf("Parse(%q) = %+v, %v\nwant %+v", c.args, got.Query, err, c.want)
		}
	}
}

func TestParseRefusesAWrongLineAsAUsageError(t *testing.T) {
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"plan", "a"}, "ci plan needs <from> <to>, --nightly or --whole"},
		{[]string{"plan", "a", ""}, "ci plan needs <from> <to>, --nightly or --whole"},
		{[]string{"scope", "a"}, "ci scope needs <from> <to>"},
		{[]string{"watch", "a", "b"}, "ci watch takes at most one commit (b)"},
		{[]string{"range", "--base", "b"}, "ci range needs --head <sha>"},
		{[]string{"nope"}, "unknown command: ci nope"},
		{nil, "unknown command: ci"},
	} {
		_, err := Parse(c.args, false, false)
		var u out.UsageError
		if !errors.As(err, &u) || err.Error() != c.want {
			t.Errorf("Parse(%q) = %v, want the usage error %q", c.args, err, c.want)
		}
	}
}
