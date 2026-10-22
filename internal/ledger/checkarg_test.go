package ledger

import (
	"errors"
	"testing"

	"github.com/donvargax/itos/v8/internal/out"
)

// A --check adds a check and a --timeout gives the last one its seconds,
// once; a blank command, a --timeout before any check, a second one, and
// one that is no number above 0 are usage errors naming the command.
func TestCheckArg(t *testing.T) {
	var checks []NewCheck
	timed := true
	steps := []struct{ name, v string }{{"--check", "true"}, {"--timeout", "2"}, {"--check", "echo done"}, {"--timeout", "0.5"}}
	for _, s := range steps {
		if err := CheckArg("task add", &checks, &timed, s.name, s.v); err != nil {
			t.Fatalf("%s %s: %v", s.name, s.v, err)
		}
	}
	if len(checks) != 2 || checks[0].Run != "true" || *checks[0].Timeout != 2 || checks[1].Run != "echo done" || *checks[1].Timeout != 0.5 || !timed {
		t.Fatalf("checks %+v, timed %v", checks, timed)
	}
	if err := CheckArg("task add", &checks, &timed, "--check", "go test"); err != nil || timed || len(checks) != 3 || checks[2].Timeout != nil {
		t.Fatalf("a third --check: %v, timed %v, %+v", err, timed, checks)
	}
	timeout := "work promote takes one --timeout <seconds> after a --check, a number above 0, for that check"
	for _, c := range []struct {
		checks     []NewCheck
		timed      bool
		name, v    string
		want       string
		wantChecks int
	}{
		{nil, false, "--check", " ", "work promote --check needs a command", 0},
		{nil, false, "--timeout", "2", timeout, 0},
		{[]NewCheck{{Run: "true"}}, true, "--timeout", "2", timeout, 1},
		{[]NewCheck{{Run: "true"}}, false, "--timeout", "nope", timeout, 1},
		{[]NewCheck{{Run: "true"}}, false, "--timeout", "0", timeout, 1},
		{[]NewCheck{{Run: "true"}}, false, "--timeout", "-1", timeout, 1},
	} {
		timed := c.timed
		err := CheckArg("work promote", &c.checks, &timed, c.name, c.v)
		var u out.UsageError
		if !errors.As(err, &u) || err.Error() != c.want || len(c.checks) != c.wantChecks || c.checks != nil && c.checks[0].Timeout != nil {
			t.Errorf("%s %q after %d checks (timed %v): %v", c.name, c.v, len(c.checks), c.timed, err)
		}
	}
}
