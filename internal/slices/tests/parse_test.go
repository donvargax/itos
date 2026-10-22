package tests

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/out"
)

// described is a query as a test compares it, its Features followed.
func described(q Query) string {
	features := "<nil>"
	if q.Features != nil {
		features = *q.Features
	}
	return fmt.Sprintf("sub %q action %q kind %q at %q features %q runner %q stem %q count %d counted %v",
		q.Sub, q.Action, q.Kind, q.At, features, q.Runner, q.Stem, q.Count, q.Counted)
}

func TestParseReadsEachSubcommand(t *testing.T) {
	features := filepath.Join("sub", "copy")
	for _, c := range []struct {
		args []string
		want Query
	}{
		{[]string{"list", "scenario"}, Query{Sub: "list", Kind: "scenario"}},
		{[]string{"list", "--at", "HEAD", "scenario"}, Query{Sub: "list", Kind: "scenario", At: "HEAD"}},
		{[]string{"moves", "scenario"}, Query{Sub: "moves", Kind: "scenario"}},
		{[]string{"next-id", "scenario", "ID-A"}, Query{Sub: "next-id", Kind: "scenario", Stem: "ID-A", Count: 1}},
		{[]string{"next-id", "scenario", "--count", "3", "ID-A"}, Query{Sub: "next-id", Kind: "scenario", Stem: "ID-A", Count: 3, Counted: true}},
		{[]string{"smoke", "check", "scenario"}, Query{Sub: "smoke", Action: "check", Kind: "scenario"}},
		{[]string{"smoke", "check", "--features", "copy", "scenario"}, Query{Sub: "smoke", Action: "check", Kind: "scenario", Features: &features}},
		{[]string{"smoke", "ids", "scenario"}, Query{Sub: "smoke", Action: "ids", Kind: "scenario"}},
		{[]string{"smoke", "run", "scenario"}, Query{Sub: "smoke", Action: "run", Kind: "scenario"}},
		{[]string{"smoke", "run", "scenario", "--", "--workers=1", "--"}, Query{Sub: "smoke", Action: "run", Kind: "scenario", Runner: []string{"--workers=1", "--"}}},
		{[]string{"smoke", "run", "scenario", "-v", "x"}, Query{Sub: "smoke", Action: "run", Kind: "scenario", Runner: []string{"-v", "x"}}},
	} {
		got, err := Parse(c.args, "sub", true, true)
		if err != nil || !got.JSON || !got.Quiet || described(got.Query) != described(c.want) {
			t.Errorf("Parse(%q) = %s, %v\nwant %s", c.args, described(got.Query), err, described(c.want))
		}
	}
}

func TestParseRefusesAWrongLineAsAUsageError(t *testing.T) {
	for _, c := range []struct {
		args []string
		want string
	}{
		{nil, "unknown command: tests"},
		{[]string{"bogus"}, "unknown command: tests bogus"},
		{[]string{"list", "--at", "HEAD"}, "tests list needs <kind>"},
		{[]string{"moves", "--quiet"}, "tests moves needs <kind>"},
		{[]string{"next-id", "scenario"}, "tests next-id needs <kind> <stem>"},
		{[]string{"next-id", "scenario", "ID-A", "extra"}, "tests next-id needs <kind> <stem>"},
		{[]string{"next-id", "scenario", "ID-A", "--count", "0"}, `tests next-id --count takes how many ids to claim, a number from 1 up, not "0"`},
		{[]string{"next-id", "scenario", "ID-A", "--count", "two"}, `not "two"`},
		{[]string{"smoke"}, "tests smoke check|ids|run needs <kind>"},
		{[]string{"smoke", "ids"}, "tests smoke ids needs <kind>"},
		{[]string{"smoke", "check", "--features", "copy"}, "tests smoke check needs <kind>"},
		{[]string{"smoke", "bogus", "scenario"}, "unknown command: tests smoke bogus"},
	} {
		_, err := Parse(c.args, "", false, false)
		var u out.UsageError
		if !errors.As(err, &u) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("Parse(%q) = %v, want a usage error saying %q", c.args, err, c.want)
		}
	}
}

// A path typed in a subfolder is read from it; one typed at the top, an
// absolute one, - and an empty one stay as typed.
func TestTypedReadsAPathFromTheSubfolder(t *testing.T) {
	abs, err := filepath.Abs("copy")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ origin, p, want string }{
		{"sub", "copy", filepath.Join("sub", "copy")},
		{"", "copy", "copy"},
		{"sub", abs, abs},
		{"sub", "-", "-"},
		{"sub", "", ""},
	} {
		if got := typed(c.origin, c.p); got != c.want {
			t.Errorf("typed(%q, %q) = %q, want %q", c.origin, c.p, got, c.want)
		}
	}
}
