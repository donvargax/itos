package config

import (
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/value"
)

// A declaration is the same whatever order and however many times its names
// are written, and the text of an empty one has empty lists.
func TestEvidenceNormalAndText(t *testing.T) {
	a := Evidence{Inputs: []string{"b", "a", "a"}, Env: []string{"Z", "Y"}}
	b := Evidence{Inputs: []string{"a", "b"}, Env: []string{"Y", "Z"}, Tools: nil}
	if a.Text() != b.Text() {
		t.Errorf("%s != %s", a.Text(), b.Text())
	}
	if got := a.Text(); got != `{"inputs":["a","b"],"env":["Y","Z"],"tools":[]}` {
		t.Errorf("text %s", got)
	}
	if got := (Evidence{}).Text(); got != `{"inputs":[],"env":[],"tools":[]}` {
		t.Errorf("empty text %s", got)
	}
	if n := (Evidence{}).Normal(); n.Inputs == nil || n.Env == nil || n.Tools == nil {
		t.Errorf("an empty declaration has nil lists: %+v", n)
	}
	if a.Inputs[0] != "b" {
		t.Error("Normal sorted the declaration's own list")
	}
}

// A declaration that reads HEAD is another declaration, and one that does not
// has the text it always had, so that no record made before head existed is
// lost.
func TestEvidenceHeadIsPartOfTheText(t *testing.T) {
	reads := Evidence{Head: true}
	if got := reads.Text(); got != `{"inputs":[],"env":[],"tools":[],"head":true}` {
		t.Errorf("text %s", got)
	}
	if reads.Text() == (Evidence{}).Text() || !reads.Normal().Head {
		t.Errorf("head does not tell declarations apart: %s %s", reads.Text(), (Evidence{}).Text())
	}
	if got := (Evidence{Head: false}).Text(); got != `{"inputs":[],"env":[],"tools":[]}` {
		t.Errorf("a declaration without head has text %s", got)
	}
}

func TestIsGlob(t *testing.T) {
	for p, want := range map[string]bool{"gen/*.txt": true, "a?": true, "a[b]": true, "gen/a.txt": false, "gen": false} {
		if IsGlob(p) != want {
			t.Errorf("IsGlob(%q) = %v", p, !want)
		}
	}
}

// parsed is a YAML value as itos reads it.
func parsed(t *testing.T, text string) any {
	t.Helper()
	v, err := value.Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// The ledger's declaration keeps the texts of its lists; anything but a
// mapping is none.
func TestEvidenceOf(t *testing.T) {
	got := EvidenceOf(parsed(t, "{ inputs: [a, 2, b], env: [E], tools: x }"))
	if got == nil || !slices.Equal(got.Inputs, []string{"a", "b"}) || !slices.Equal(got.Env, []string{"E"}) || got.Tools != nil {
		t.Errorf("%+v", got)
	}
	if empty := EvidenceOf(parsed(t, "{}")); empty == nil || len(empty.Inputs)+len(empty.Env)+len(empty.Tools) != 0 {
		t.Errorf("an empty declaration is %+v", empty)
	}
	for _, text := range []string{"null", "x", "[a]", "3"} {
		if EvidenceOf(parsed(t, text)) != nil {
			t.Errorf("%s is a declaration", text)
		}
	}
	if EvidenceOf(value.Undefined) != nil {
		t.Error("no declaration is one")
	}
	for text, want := range map[string]bool{"{ head: true }": true, "{ head: false }": false, "{}": false, `{ head: "true" }`: false, "{ head: 1 }": false} {
		if got := EvidenceOf(parsed(t, text)); got == nil || got.Head != want {
			t.Errorf("%s: %+v, want head %v", text, got, want)
		}
	}
}

func TestEvidenceShape(t *testing.T) {
	for text, want := range map[string]string{
		"{}":                                   "",
		"{ inputs: [a], env: [], tools: [t] }": "",
		"null":                                 "is not a mapping",
		"[a]":                                  "is not a mapping",
		"{ inptus: [a] }":                      "an unknown key inptus",
		"{ inputs: a }":                        "inputs is not a list of texts",
		"{ env: [a, 1] }":                      "env is not a list of texts",
		"{ head: true }":                       "",
		"{ head: false, inputs: [a] }":         "",
		`{ head: "yes" }`:                      `has head: "yes"; head is true or false`,
		"{ head: null }":                       "head is true or false",
		"{ head: [a] }":                        "head is true or false",
	} {
		found := EvidenceShape(parsed(t, text), "check 0", "rule-x")
		switch {
		case want == "" && len(found) > 0:
			t.Errorf("%s: %v", text, found)
		case want != "" && (len(found) != 1 || !strings.Contains(found[0].Message, want) || found[0].Rule != "rule-x" || found[0].Fix == ""):
			t.Errorf("%s: %+v", text, found)
		}
	}
}

func TestEvidenceProblems(t *testing.T) {
	for _, c := range []struct {
		e    Evidence
		want string
	}{
		{Evidence{Inputs: []string{"gen", "gen/seed.txt", "a b/c"}, Env: []string{"HOME"}, Tools: []string{"sh"}}, ""},
		{Evidence{Inputs: []string{"gen/*.txt"}}, "input gen/*.txt is a glob"},
		{Evidence{Inputs: []string{"a?"}}, "input a? is a glob"},
		{Evidence{Inputs: []string{"a[1]"}}, "input a[1] is a glob"},
		{Evidence{Inputs: []string{"/etc/passwd"}}, `input "/etc/passwd" is not a path`},
		{Evidence{Inputs: []string{""}}, `input "" is not a path`},
		{Evidence{Inputs: []string{".."}}, `input ".." is not a path`},
		{Evidence{Inputs: []string{"a/../../b"}}, `input "a/../../b" is not a path`},
		{Evidence{Inputs: []string{`a\b`}}, `is not a path`},
		{Evidence{Env: []string{"A=1"}}, `env "A=1" is not the name`},
		{Evidence{Env: []string{""}}, `env "" is not the name`},
		{Evidence{Tools: []string{""}}, "tool is empty"},
	} {
		found := c.e.Problems("check 0", "rule-x")
		switch {
		case c.want == "" && len(found) > 0:
			t.Errorf("%+v: %v", c.e, found)
		case c.want != "" && (len(found) != 1 || !strings.Contains(found[0].Message, c.want) || !strings.HasPrefix(found[0].Message, "check 0's evidence ") || found[0].Rule != "rule-x" || found[0].Fix == ""):
			t.Errorf("%+v: %+v", c.e, found)
		}
	}
}

// A step declares evidence in ci.steps alone, on a command: the schema
// refuses it in the nightly's, and the cross-check on a step that runs tests
// or whose declaration is wrong.
func TestStepEvidence(t *testing.T) {
	ok := "version: 1\nci:\n  steps:\n    - { run: x, evidence: { inputs: [a], env: [E], tools: [t] } }\n    - { run: y, evidence: {} }\n    - { run: z, evidence: { head: true } }\n"
	c, err := load(t, ok)
	if err != nil {
		t.Fatal(err)
	}
	if e := c.CI.Steps[0].Evidence; e == nil || !slices.Equal(e.Inputs, []string{"a"}) || !slices.Equal(e.Tools, []string{"t"}) {
		t.Errorf("step 0: %+v", e)
	}
	if e := c.CI.Steps[1].Evidence; e == nil || len(e.Inputs) != 0 || e.Head {
		t.Errorf("step 1: %+v", e)
	}
	if e := c.CI.Steps[2].Evidence; e == nil || !e.Head {
		t.Errorf("step 2: %+v", e)
	}
	for text, want := range map[string]string{
		"version: 1\nci:\n  steps:\n    - { run: x, evidence: { inptus: [a] } }\n":                 "unknown key ci.steps[0].evidence.inptus",
		"version: 1\nci:\n  steps:\n    - { run: x, evidence: [a] }\n":                             "ci.steps[0].evidence",
		"version: 1\nci:\n  steps:\n    - { run: x, evidence: { head: yes } }\n":                   "ci.steps[0].evidence.head",
		"version: 1\nci:\n  steps: []\n  nightly:\n    steps:\n      - { run: x, evidence: {} }\n": "unknown key ci.nightly.steps[0].evidence",
	} {
		_, err := load(t, text)
		if err == nil || !strings.Contains(strings.Join(messages(t, err), "\n"), want) {
			t.Errorf("%q: got %v, want %q", text, err, want)
		}
	}
	bad := "version: 1\nci:\n  steps:\n    - { run: x, evidence: { inputs: [\"g/*.txt\"] } }\n"
	_, err = load(t, bad)
	if got := messages(t, err); len(got) != 1 || !strings.Contains(got[0], "a CI step x's evidence input g/*.txt is a glob") {
		t.Errorf("%v", got)
	}
	tests := "version: 1\ntests:\n  scenario: { root: features, run: { whole: x } }\nci:\n  steps:\n    - { tests: scenario, evidence: {} }\n"
	_, err = load(t, tests)
	if got := messages(t, err); len(got) != 1 || !strings.Contains(got[0], "runs tests: scenario and declares evidence") {
		t.Errorf("%v", got)
	}
}

// The settings are the tree under the defaults.
func TestSettings(t *testing.T) {
	c, err := load(t, "version: 1\nledger: { files: \"t/{group}.yaml\", check: { timeout: 7 } }\n")
	if err != nil {
		t.Fatal(err)
	}
	s := c.Settings()
	if !strings.Contains(s, `"version":1`) || !strings.Contains(s, `"shell":["sh","-c"]`) || !strings.Contains(s, `"timeout":7`) || !strings.Contains(s, `"hooks":`) {
		t.Errorf("%s", s)
	}
}
