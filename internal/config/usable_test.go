package config

import (
	"errors"
	"slices"
	"testing"

	"github.com/donvargax/itos/v8/internal/value"
)

// loadProblems are the messages of the problems Load refuses the text with,
// nil when it loads.
func loadProblems(t *testing.T, text string) []string {
	t.Helper()
	_, err := load(t, text)
	if err == nil {
		return nil
	}
	var invalid *Error
	if !errors.As(err, &invalid) {
		t.Fatalf("%q: %v", text, err)
	}
	var got []string
	for _, p := range invalid.Problems {
		got = append(got, p.Message)
	}
	return got
}

// The values a command misbehaves on are refused when the config loads
// (slice 130), so every command, config check with them, exits 2.
func TestLoadRefusesUnusableValues(t *testing.T) {
	cases := map[string][]string{
		// The defaults, and a config that sets each key well, load.
		"version: 1\n": nil,
		"version: 1\nshell: [bash, -c]\nledger: { files: 'tasks/{group}.yaml', check: { timeout: 0.5 } }\n": nil,
		// A number not finite is named wherever it is, by its key path.
		"version: 1\nci: { watch: { interval: .nan, timeout: .inf } }\nhooks: { commit_msg: { check_timeout: -.inf } }\n": {
			"ci.watch.interval is NaN, and a number in the config is finite",
			"ci.watch.timeout is Infinity, and a number in the config is finite",
			"hooks.commit_msg.check_timeout is -Infinity, and a number in the config is finite",
		},
		// ci.watch.interval's bound is 0, which looks again at once.
		"version: 1\nci: { watch: { interval: 0 } }\n": nil,
		"version: 1\nshell: []\n":                      {"shell names no program, so no command can run under it"},
		"version: 1\nshell: [' ', -c]\n":               {"shell names no program, so no command can run under it"},
		"version: 1\nshell: [sh, '']\n":                nil,
		"version: 1\nshell: [sh]\n":                    nil,
		// A timeout of no seconds is no timeout; a non-finite one is named once.
		"version: 1\nledger: { files: 'tasks/{group}.yaml', check: { timeout: 0 } }\n": {
			"ledger.check.timeout is a number of seconds above 0, not 0",
		},
		"version: 1\nledger: { files: 'tasks/{group}.yaml', check: { timeout: -2 } }\n": {
			"ledger.check.timeout is a number of seconds above 0, not -2",
		},
		"version: 1\nledger: { files: 'tasks/{group}.yaml', check: { timeout: -.inf } }\n": {
			"ledger.check.timeout is -Infinity, and a number in the config is finite",
		},
		"version: 1\nledger: { files: 'tasks/{group}.yaml', check: { timeout: .nan } }\n": {
			"ledger.check.timeout is NaN, and a number in the config is finite",
		},
	}
	for text, want := range cases {
		if got := loadProblems(t, text); !slices.Equal(got, want) {
			t.Errorf("%q: got %q, want %q", text, got, want)
		}
	}
}

func TestUnusable(t *testing.T) {
	const kinds = "commits: { types: [feat, fix] }\ntests:\n"
	cases := map[string][]string{
		// The defaults, and a config that sets each key well, are usable.
		"version: 1\n": nil,
		"version: 1\n" + kinds +
			"  a: { adapter: { command: x, supports_at: true }, range_checks: [{ name: m, builtin: moves, except_types: [fix] }] }\n" +
			"  b: { wip_tag: '@skip' }\n": nil,
		"version: 1\ntests: { a: {} }\n": nil,
		// Every type except_types names is one of commits.types, when it lists them.
		"version: 1\n" + kinds + "  a: { range_checks: [{ name: m, range: x }, { name: n, range: y, except_types: [fead, fix, doc] }] }\n": {
			"tests.a.range_checks[1].except_types names fead, which is not one of commits.types",
			"tests.a.range_checks[1].except_types names doc, which is not one of commits.types",
		},
		"version: 1\ntests: { a: { range_checks: [{ name: m, range: x, except_types: [fead] }] } }\n": nil,
		// wip_tag is the Gherkin adapter's alone.
		"version: 1\ntests:\n  a: { adapter: { command: x }, wip_tag: '@wip' }\n  b: { adapter: { command: x } }\n  c: { adapter: gherkin, wip_tag: '@wip' }\n": {
			"tests.a.wip_tag is read only by the gherkin adapter, and tests.a.adapter is a command, which says itself which tests are live",
		},
		"version: 1\nci: { steps: [], prose: { paths: [a], steps: [b] } }\nwork: { statuses: [todo] }\n": nil,
	}
	for text, want := range cases {
		c, err := load(t, text)
		if err != nil {
			t.Fatalf("%q: %v", text, err)
		}
		var got []string
		for _, p := range c.Unusable() {
			got = append(got, p.Message)
		}
		if !slices.Equal(got, want) {
			t.Errorf("%q: got %q, want %q", text, got, want)
		}
	}
}

// No list of the schema holds a number today, but one that did would name
// its item by index.
func TestNonFiniteNamesListItems(t *testing.T) {
	tree, err := value.Parse("a: [1, .nan, { b: .inf }]\n")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range nonFinite(tree, "") {
		got = append(got, p.Message+" | "+p.Fix)
	}
	want := []string{
		"a[1] is NaN, and a number in the config is finite | set a[1] to a finite number",
		"a[2].b is Infinity, and a number in the config is finite | set a[2].b to a finite number",
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Each problem says what to write instead, for a config itos upgrade carried
// to a release that refuses it.
func TestUnusableSaysWhatToWrite(t *testing.T) {
	_, err := load(t, "version: 1\nshell: []\nledger: { files: 'tasks/{group}.yaml', check: { timeout: 0 } }\n"+
		"ci: { watch: { interval: .nan } }\n")
	var invalid *Error
	if !errors.As(err, &invalid) {
		t.Fatalf("loaded: %v", err)
	}
	var got []string
	for _, p := range invalid.Problems {
		got = append(got, p.Rule+": "+p.Fix)
	}
	c, err := load(t, "version: 1\ncommits: { types: [feat] }\ntests:\n  a: { adapter: { command: x, supports_at: true }, wip_tag: '@wip', "+
		"range_checks: [{ name: m, builtin: moves, except_types: [fead] }] }\n")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range c.Unusable() {
		got = append(got, p.Rule+": "+p.Fix)
	}
	want := []string{
		"config-number-finite: set ci.watch.interval to a finite number",
		"config-shell: remove shell to run commands under sh -c, or name the program first: [bash, -c]",
		"config-check-timeout: set ledger.check.timeout to the seconds a check may run, or remove it for the default, 600",
		"config-except-types: add fead to commits.types, or remove it from tests.a.range_checks[0].except_types",
		"config-wip-tag: remove tests.a.wip_tag",
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
