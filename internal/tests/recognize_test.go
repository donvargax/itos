package tests

import (
	"reflect"
	"testing"
)

// A task check read back as a selection: a pattern double-quoted,
// single-quoted or bare, the whole run, the smoke set, a check that calls
// hooks.bin read as itos; a pattern the shell would expand, a double-quoted
// one with a backslash, and other flags are not runs of the kind.
func TestRecognize(t *testing.T) {
	cfg := load(t, `version: 1
tests:
  scenario:
    root: features
    recognize:
      - { command: "vp run e2e", as: whole }
      - { command: "vp run e2e --grep {pattern}", as: pattern }
      - { command: "itos e2e smoke", as: smoke }
hooks: { bin: bin/itos }
`)
	smoke := []string{"ID-A-01"}
	cases := []struct {
		command string
		want    Selection
		ok      bool
	}{
		{`vp run e2e --grep "@phase-4"`, Pattern("@phase-4"), true},
		{`vp run e2e --grep '@a|@b'`, Pattern("@a|@b"), true},
		{"vp   run e2e --grep @a ", Pattern("@a"), true},
		{"vp run e2e", Whole(), true},
		{"bin/itos e2e smoke", IDs("ID-A-01"), true},
		{"other/itos e2e smoke", Selection{}, false},
		{"vp run e2e --grep $X", Selection{}, false},
		{`vp run e2e --grep "a\b"`, Selection{}, false},
		{"vp run e2e --workers 1", Selection{}, false},
	}
	for _, c := range cases {
		got, ok, err := Recognize(cfg, "scenario", c.command, smoke)
		if err != nil || ok != c.ok || !reflect.DeepEqual(got, c.want) {
			t.Errorf("Recognize(%q) = %+v, %v, %v", c.command, got, ok, err)
		}
	}
}
