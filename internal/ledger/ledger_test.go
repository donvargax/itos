package ledger

import (
	"slices"
	"testing"

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
		"{ run: x, why: y }":   "remove why; a check's keys are run, fails, after, timeout, prose, cost",
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
