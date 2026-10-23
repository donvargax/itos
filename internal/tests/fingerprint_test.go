package tests

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

var fingerprintOptions = Options{ID: `ID-[A-Z]+-\d+`, TagPrefix: "@", WipTag: "@wip"}

// A feature file of two live scenarios under a Background and a feature tag, as the scenarios
// of tests-list.feature write theirs.
const fingerprinted = `@phase-1
Feature: One
  What it covers.

  Background:
    Given a repository

  @ID-A-01
  Scenario: ID-A-01 runs
    When it runs with
      | key | value |
      | one | 1     |
    Then it is done

  @ID-B-01
  Scenario: ID-B-01 runs
    When it runs
`

// An outline, under a rule with a Background of its own.
const outlined = `Feature: Outlined

  Background:
    Given a repository

  Rule: the rule
    Background:
      Given the rule holds

    @ID-A-01
    Scenario Outline: ID-A-01 runs <what>
      When it runs <what>

      @fast
      Examples:
        | what  |
        | once  |
        | twice |
`

func prints(t *testing.T, text string) map[string]string {
	t.Helper()
	return Fingerprints(text, fingerprintOptions)
}

// Each scenario of tests-list.feature's outlines as a unit case, and a few more: an edit of what
// ID-A-01 alone runs changes its fingerprint and leaves ID-B-01's, an edit of what both run
// changes both, and any other leaves both.
func TestFingerprints(t *testing.T) {
	replace := func(text, old, new string) func(string) string {
		return func(string) string {
			if !strings.Contains(text, old) {
				t.Fatalf("%q is not in the text", old)
			}
			return strings.Replace(text, old, new, 1)
		}
	}
	for _, c := range []struct {
		name    string
		text    string
		edit    func(string) string
		changed bool
		// whole is an edit of what every scenario of the file runs, ID-B-01's too.
		whole bool
	}{
		{"a comment above the scenario", fingerprinted, replace(fingerprinted, "  @ID-A-01\n", "  # Why.\n  @ID-A-01\n"), false, false},
		{"a comment between its steps", fingerprinted, replace(fingerprinted, "    Then it is done\n", "    # Why.\n    Then it is done\n"), false, false},
		{"the feature's description reworded", fingerprinted, replace(fingerprinted, "What it covers.", "What it covers, put another way."), false, false},
		{"the feature renamed", fingerprinted, replace(fingerprinted, "Feature: One", "Feature: Two"), false, false},
		{"a @wip scenario added", fingerprinted, func(s string) string {
			return s + "\n  @ID-C-01 @wip\n  Scenario: Not ready\n    When it is written\n"
		}, false, false},
		{"the scenario renamed", fingerprinted, replace(fingerprinted, "Scenario: ID-A-01 runs", "Scenario: ID-A-01 runs, renamed"), false, false},
		{"a step's keyword changed", fingerprinted, replace(fingerprinted, "    Then it is done\n", "    And it is done\n"), false, false},
		{"a table's cells padded otherwise", fingerprinted, replace(fingerprinted, "| one | 1     |", "| one   | 1 |"), false, false},
		{"a step reworded", fingerprinted, replace(fingerprinted, "Then it is done", "Then it is done again"), true, false},
		{"a table row changed", fingerprinted, replace(fingerprinted, "| one | 1     |", "| one | 2     |"), true, false},
		{"the Background gaining a step", fingerprinted, replace(fingerprinted, "    Given a repository\n", "    Given a repository\n    And it is set up\n"), true, true},
		{"the scenario gaining a tag", fingerprinted, replace(fingerprinted, "  @ID-A-01\n", "  @ID-A-01 @slow\n"), true, false},
		{"the feature gaining a tag", fingerprinted, replace(fingerprinted, "@phase-1\n", "@phase-1 @slow\n"), true, true},
		{"the feature's tag written twice", fingerprinted, replace(fingerprinted, "@phase-1\n", "@phase-1 @phase-1\n"), false, false},
		{"an Examples row changed", outlined, replace(outlined, "| twice |", "| again |"), true, false},
		{"the Examples' header changed", outlined, replace(outlined, "| what  |", "| how   |"), true, false},
		{"the Examples' tag changed", outlined, replace(outlined, "@fast", "@slow"), true, false},
		{"the Examples renamed", outlined, replace(outlined, "Examples:", "Examples: the ways"), false, false},
		{"the rule's Background gaining a step", outlined, replace(outlined, "      Given the rule holds\n", "      Given the rule holds\n      And holds again\n"), true, false},
		{"the rule renamed", outlined, replace(outlined, "Rule: the rule", "Rule: another"), false, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			was, now := prints(t, c.text), prints(t, c.edit(c.text))
			if len(was["ID-A-01"]) != 64 || strings.ToLower(was["ID-A-01"]) != was["ID-A-01"] {
				t.Fatalf("ID-A-01's fingerprint is %q, not 64 lowercase hex digits", was["ID-A-01"])
			}
			if (was["ID-A-01"] != now["ID-A-01"]) != c.changed {
				t.Errorf("ID-A-01 was %s and is %s; changed should be %t", was["ID-A-01"], now["ID-A-01"], c.changed)
			}
			if (was["ID-B-01"] != now["ID-B-01"]) != c.whole {
				t.Errorf("ID-B-01 was %s and is %s; changed should be %t", was["ID-B-01"], now["ID-B-01"], c.whole)
			}
		})
	}
}

// A scenario moved unchanged to a file of the same Background and feature tags keeps its
// fingerprint; under another Background it does not.
func TestFingerprintsOfAMovedScenario(t *testing.T) {
	block := fingerprinted[strings.Index(fingerprinted, "  @ID-A-01\n"):strings.Index(fingerprinted, "  @ID-B-01\n")]
	same := "@phase-1\nFeature: Elsewhere\n\n  Background:\n    Given a repository\n\n" + block
	other := "@phase-1\nFeature: Elsewhere\n\n" + block
	want := prints(t, fingerprinted)["ID-A-01"]
	if got := prints(t, same)["ID-A-01"]; got != want {
		t.Errorf("moved under the same header: %s, not %s", got, want)
	}
	if got := prints(t, other)["ID-A-01"]; got == want {
		t.Errorf("moved under no Background, it kept %s", got)
	}
	if prints(t, fingerprinted)["ID-A-01"] == prints(t, fingerprinted)["ID-B-01"] {
		t.Error("two scenarios have one fingerprint")
	}
}

// The canonical text, exactly: the version line, the effective tags sorted once each, the
// Background's steps then the scenario's, the keywords left out, a doc string's media type and
// content and a table's cells escaped; the fingerprint is its SHA-256.
func TestCanonical(t *testing.T) {
	text := "@b @a\nFeature: F\n\n  Background:\n    Given a \\ b\n\n  @ID-A-01 @a\n  Scenario: S\n" +
		"    When it reads\n      ```json\n      {\"a\":\t1}\n      \\n\n      ```\n" +
		"    Then it has\n      | x \\| y | two\\nlines | back\\\\slash |\n"
	want := "itos scenario fingerprint v1\n" +
		"tag @ID-A-01\ntag @a\ntag @b\n" +
		"step a \\\\ b\n" +
		"step it reads\ndoc json\t{\"a\":\\t1}\\n\\\\n\n" +
		"step it has\nrow x | y\ttwo\\nlines\tback\\\\slash\n"
	doc := prints(t, text)
	sum := sha256.Sum256([]byte(want))
	if got := doc["ID-A-01"]; got != hex.EncodeToString(sum[:]) {
		t.Errorf("the fingerprint is %s, not the SHA-256 of\n%s", got, want)
	}
}

// Without the escaping two different tables would write one text: one cell holding a tab, and
// two cells.
func TestFingerprintsKeepCellsApart(t *testing.T) {
	scenario := func(row string) string {
		return "Feature: F\n\n  @ID-A-01\n  Scenario: S\n    When it has\n      " + row + "\n"
	}
	if prints(t, scenario("| a\tb |"))["ID-A-01"] == prints(t, scenario("| a | b |"))["ID-A-01"] {
		t.Error("a cell holding a tab and two cells have one fingerprint")
	}
}

// A text Gherkin does not parse gives no fingerprint, nor does an ID pattern that does not
// compile (ParseFeature's error to report).
func TestFingerprintsOfNoGherkin(t *testing.T) {
	if got := prints(t, "  @ID-A-01\n  Scenario: no feature line\n    When it runs\n"); len(got) != 0 {
		t.Errorf("%v", got)
	}
	if got := prints(t, ""); len(got) != 0 {
		t.Errorf("%v", got)
	}
	if got := Fingerprints(fingerprinted, Options{ID: "(", TagPrefix: "@"}); len(got) != 0 {
		t.Errorf("an ID pattern that does not compile gave %v", got)
	}
}

func TestFingerprintsIgnoreAnEmptyIDCapture(t *testing.T) {
	text := "Feature: F\n\n  @x\n  Scenario: no ID\n    When it runs\n"
	got := Fingerprints(text, Options{ID: "(|x)", TagPrefix: "@"})
	if len(got) != 0 {
		t.Errorf("fingerprints of an empty ID capture = %v, want none", got)
	}
}
