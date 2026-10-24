package ledger

import (
	"reflect"
	"slices"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/value"
)

// A check's own timeout is a finite number of seconds above 0 (slice 129):
// one of 0 or less is no timeout, and the commit-msg hook takes the lesser of
// it and its cap.
func TestCheckProblemsHoldATimeoutAboveZero(t *testing.T) {
	cases := map[string][]string{
		"{ run: x, timeout: 0.5 }": nil,
		"{ run: x, timeout: 600 }": nil,
		"{ run: x, timeout: 0 }":   {"check 0 says timeout: 0; its timeout is a finite number of seconds above 0"},
		"{ run: x, timeout: -1 }":  {"check 0 says timeout: -1; its timeout is a finite number of seconds above 0"},
		"{ run: x, timeout: .nan }": {
			"check 0 says timeout: NaN; its timeout is a finite number of seconds above 0",
		},
		"{ run: x, timeout: .inf }": {
			"check 0 says timeout: Infinity; its timeout is a finite number of seconds above 0",
		},
		"{ run: x, timeout: -.inf }": {
			"check 0 says timeout: -Infinity; its timeout is a finite number of seconds above 0",
		},
		// Not a number at all is the type's problem alone, named as JSON.
		"{ run: x, timeout: ten }": {`check 0 says timeout: "ten"; its timeout is a number of seconds`},
	}
	for text, want := range cases {
		check, err := value.Parse(text)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, p := range checkProblems(check, "check 0") {
			got = append(got, p.Message)
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s: got %q, want %q", text, got, want)
		}
	}
}

func TestCheckProblemsSayWhatTimeoutToWrite(t *testing.T) {
	check, err := value.Parse("{ run: x, timeout: 0 }")
	if err != nil {
		t.Fatal(err)
	}
	found := checkProblems(check, "check 0")
	if len(found) != 1 || found[0].Rule != "ledger-check-value" ||
		found[0].Fix != "write the timeout as the seconds the check may run, above 0 in check 0" {
		t.Fatalf("got %+v", found)
	}
}

// A check runs a command, under run or fails alone, with no key a check does
// not have.
func TestCheckProblems(t *testing.T) {
	cases := map[string][]string{
		"{ run: x }":   nil,
		"{ fails: x }": nil,
		"[run, x]":     {"check 0 is not a mapping"},
		"{ run: x, after: push, prose: true, cost: late, timeout: 1 }": nil,
		"{ run: x, why: y }":       {"check 0 has an unknown key why"},
		"{ run: x, fails: y }":     {"check 0 needs exactly one of run and fails"},
		"{ timeout: 1 }":           {"check 0 needs exactly one of run and fails"},
		"{ fails: [a, b] }":        {"check 0's command is not text"},
		"{ run: null, fails: x }":  {"check 0 needs exactly one of run and fails"},
		"{ run: x, after: merge }": {`check 0 says after: "merge"; after: push is its only value`},
	}
	for text, want := range cases {
		check, err := value.Parse(text)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, p := range checkProblems(check, "check 0") {
			got = append(got, p.Message)
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s: got %q, want %q", text, got, want)
		}
	}
	for text, fix := range map[string]string{
		"[run, x]":             "write check 0 as run: <command> or fails: <command>",
		"{ run: x, why: y }":   "remove why; a check's keys are run, fails, after, timeout, prose, cost, evidence, snapshot",
		"{ run: x, fails: y }": "give check 0 either run: or fails:, not both",
		"{ run: 3 }":           "quote check 0's command as one string",
	} {
		check, err := value.Parse(text)
		if err != nil {
			t.Fatal(err)
		}
		if found := checkProblems(check, "check 0"); len(found) != 1 || found[0].Fix != fix {
			t.Errorf("%s: got %+v, want the fix %q", text, found, fix)
		}
	}
}

// A check's evidence is a mapping of the three lists, and its texts hold to
// the declaration's rules; the problems name the check.
func TestCheckProblemsHoldEvidence(t *testing.T) {
	for text, want := range map[string]string{
		"{ run: x, evidence: {} }": "",
		"{ run: x, evidence: { inputs: [gen, a.txt], env: [E], tools: [sh] } }": "",
		"{ run: x, evidence: yes }":                         "check 0's evidence is not a mapping",
		"{ run: x, evidence: { inptus: [a] } }":             "check 0's evidence has an unknown key inptus",
		"{ run: x, evidence: { inputs: a } }":               "check 0's evidence inputs is not a list of texts",
		"{ run: x, evidence: { inputs: [\"gen/*.txt\"] } }": "check 0's evidence input gen/*.txt is a glob",
		"{ run: x, evidence: { env: [\"A=b\"] } }":          "check 0's evidence env \"A=b\" is not the name of a variable",
		"{ run: x, evidence: { head: true } }":              "",
		"{ run: x, evidence: { head: \"yes\" } }":           "check 0's evidence has head: \"yes\"; head is true or false",
	} {
		check, err := value.Parse(text)
		if err != nil {
			t.Fatal(err)
		}
		found := checkProblems(check, "check 0")
		switch {
		case want == "" && len(found) > 0:
			t.Errorf("%s: %+v", text, found)
		case want != "" && (len(found) != 1 || found[0].Message != want || found[0].Rule != "ledger-check-evidence"):
			t.Errorf("%s: got %+v, want %q", text, found, want)
		}
	}
}

// A check opts in by writing evidence, however little it declares.
func TestCheckOfReadsEvidence(t *testing.T) {
	for text, want := range map[string]*config.Evidence{
		"{ run: x }":                            nil,
		"{ run: x, evidence: null }":            nil,
		"{ run: x, evidence: yes }":             nil,
		"{ run: x, evidence: {} }":              {},
		"{ run: x, evidence: { env: [A, B] } }": {Env: []string{"A", "B"}},
	} {
		raw, err := value.Parse(text)
		if err != nil {
			t.Fatal(err)
		}
		got, ok := checkOf(raw)
		if !ok || !reflect.DeepEqual(got.Evidence, want) {
			t.Errorf("%s: %+v, want %+v", text, got.Evidence, want)
		}
	}
}

// snapshot is true or false, and false is refused beside evidence: a check
// that runs in the working folder cannot be keyed by the tree.
func TestCheckProblemsHoldSnapshot(t *testing.T) {
	for text, want := range map[string][2]string{
		"{ run: x }":                                             {},
		"{ run: x, snapshot: false }":                            {},
		"{ run: x, snapshot: true }":                             {},
		"{ run: x, snapshot: true, evidence: {} }":               {},
		"{ run: x, snapshot: \"no\" }":                           {"ledger-check-value", "check 0 says snapshot: \"no\"; snapshot is true or false"},
		"{ run: x, snapshot: 0 }":                                {"ledger-check-value", "check 0 says snapshot: 0; snapshot is true or false"},
		"{ run: x, snapshot: false, evidence: {} }":              {"ledger-check-evidence", "check 0 says snapshot: false and declares evidence"},
		"{ run: x, snapshot: false, evidence: yes }":             {"ledger-check-evidence", "check 0's evidence is not a mapping"},
		"{ run: x, snapshot: \"no\", evidence: { head: true } }": {"ledger-check-value", "check 0 says snapshot: \"no\"; snapshot is true or false"},
	} {
		check, err := value.Parse(text)
		if err != nil {
			t.Fatal(err)
		}
		found := checkProblems(check, "check 0")
		switch {
		case want[0] == "" && len(found) > 0:
			t.Errorf("%s: %+v", text, found)
		case want[0] != "" && !slices.ContainsFunc(found, func(p out.Problem) bool { return p.Rule == want[0] && p.Message == want[1] && p.Fix != "" }):
			t.Errorf("%s: got %+v, want the %s problem %q", text, found, want[0], want[1])
		}
	}
}

// Only the boolean false opts out of the snapshot, whatever the ledger says
// (it is read without config check): another value, or none, runs the check
// in it. Evidenced is whether the engine looks the check up.
func TestCheckOfReadsSnapshotFalseAlone(t *testing.T) {
	for text, want := range map[string]bool{
		"{ run: x }":                   false,
		"{ run: x, snapshot: true }":   false,
		"{ run: x, snapshot: false }":  true,
		"{ run: x, snapshot: \"no\" }": false,
		"{ run: x, snapshot: 0 }":      false,
		"{ run: x, snapshot: null }":   false,
	} {
		raw, err := value.Parse(text)
		if err != nil {
			t.Fatal(err)
		}
		got, ok := checkOf(raw)
		if !ok || got.NoSnapshot != want {
			t.Errorf("%s: NoSnapshot %v, want %v", text, got.NoSnapshot, want)
		}
	}
	declared := &config.Evidence{}
	for _, c := range []struct {
		check Check
		want  bool
	}{
		{Check{}, false}, {Check{Evidence: declared}, true},
		{Check{Evidence: declared, NoSnapshot: true}, false}, {Check{NoSnapshot: true}, false},
	} {
		if c.check.Evidenced() != c.want {
			t.Errorf("%+v: Evidenced %v", c.check, !c.want)
		}
	}
}
