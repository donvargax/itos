package cli

import (
	"slices"
	"strings"
	"testing"
)

func TestParseGlobals(t *testing.T) {
	g := ParseGlobals([]string{"version", "--json", "--config", "c.yaml", "-q", "--root", "sub", "--check"})
	if !g.JSON || !g.Quiet || g.Config != "c.yaml" || g.Root != "sub" {
		t.Errorf("flags: %+v", g)
	}
	if !slices.Equal(g.Rest, []string{"version", "--check"}) {
		t.Errorf("rest: %q", g.Rest)
	}
	// After "--" every argument is the command's.
	g = ParseGlobals([]string{"tests", "smoke", "run", "scenario", "--", "--json", "-q"})
	if g.JSON || g.Quiet || !slices.Equal(g.Rest, []string{"tests", "smoke", "run", "scenario", "--", "--json", "-q"}) {
		t.Errorf("after --: %+v", g)
	}
	// A valued flag at the end takes nothing.
	if g = ParseGlobals([]string{"version", "--config"}); g.Config != "" || !slices.Equal(g.Rest, []string{"version"}) {
		t.Errorf("trailing --config: %+v", g)
	}
}

func run(args ...string) (int, string, string) {
	var stdout, stderr strings.Builder
	code := Main(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// A command whose group is not ported fails loudly, once its arguments are
// right: exit 3, saying so, and nothing on stdout.
func TestNotPortedFailsLoudly(t *testing.T) {
	for _, args := range [][]string{
		{"work"},
		{"work", "check"},
		{"ci", "plan", "--nightly"},
		{"ci", "run"},
		{"ci", "scope", "a", "b"},
		{"ci", "range", "--head", "x"},
		{"hook", "commit-msg", "m"},
		{"hook", "pre-push"},
		{"hooks", "install", "--manager", "git"},
	} {
		code, stdout, stderr := run(args...)
		path := strings.Join(args, " ")
		if code != ExitMissing || stdout != "" || !strings.Contains(stderr, " is not in this build yet") {
			t.Errorf("itos %s: exit %d, stdout %q, stderr %q", path, code, stdout, stderr)
		}
	}
}

// The usage errors the corpus leaves out read as the TypeScript's.
func TestUsageErrors(t *testing.T) {
	for args, want := range map[string]string{
		"tests moves":         "tests moves needs <kind>",
		"tests smoke":         "tests smoke check|ids|run needs <kind>",
		"tests smoke bogus x": "unknown command: tests smoke bogus",
		"tests":               "unknown command: tests",
		"hook commit-msg":     "hook commit-msg needs <file>",
		"hooks":               "unknown command: hooks",
		"ci range --head":     "ci range needs --head <sha>",
		"ci":                  "unknown command: ci",
	} {
		code, _, stderr := run(strings.Fields(args)...)
		if code != ExitUsage || stderr != "itos: "+want+" (itos --help)\n" {
			t.Errorf("itos %s: exit %d, stderr %q", args, code, stderr)
		}
	}
}

// The help is the longest command path it knows, whatever else the
// arguments hold; a bare itos prints itos's own and exits 2.
func TestHelp(t *testing.T) {
	for _, c := range []struct {
		args []string
		code int
		want string
	}{
		{nil, ExitUsage, mainHelp},
		{[]string{"--help"}, 0, mainHelp},
		{[]string{"bogus", "--help"}, 0, mainHelp},
		{[]string{"help", "ci", "plan"}, 0, helpTexts["ci plan"]},
		{[]string{"verify", "-h"}, 0, helpTexts["verify"]},
		{[]string{"tests", "smoke", "check", "scenario", "--help"}, 0, helpTexts["tests smoke"]},
		{[]string{"task", "T-007", "--help"}, 0, helpTexts["task"]},
		{[]string{"--json", "commit", "--help"}, 0, helpTexts["commit"]},
	} {
		code, stdout, stderr := run(c.args...)
		if code != c.code || stdout != c.want+"\n" || stderr != "" {
			t.Errorf("itos %s: exit %d, stdout %q, stderr %q", strings.Join(c.args, " "), code, stdout, stderr)
		}
	}
}
