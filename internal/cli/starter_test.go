package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v6/internal/config"
	"github.com/donvargax/itos/v6/internal/tests"
	"github.com/donvargax/itos/v6/internal/value"
)

// Every shape of the starter loads as a config, with what the slice settled:
// hooks.bin itos, a Task footer, and the Scenarios footer, the kind and the
// smoke set only with feature files, the footer required of feat and fix
// unless no scenario is tagged; a since and a pin only when given.
func TestStarterLoads(t *testing.T) {
	sha := strings.Repeat("a", 40)
	for _, s := range []starter{
		{},
		{since: sha, scenarios: true, pin: &[2]string{"9.2.0", strings.Repeat("b", 64)}},
		{stealth: true, since: sha},
		{since: sha, scenarios: true, untagged: true},
	} {
		file := filepath.Join(t.TempDir(), "itos.yaml")
		if err := os.WriteFile(file, []byte(s.config()), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := config.Load(file)
		if err != nil {
			t.Fatalf("%+v: %v\n%s", s, err, s.config())
		}
		if cfg.Hooks.Bin != "itos" {
			t.Errorf("%+v: hooks.bin is %s", s, cfg.Hooks.Bin)
		}
		if !slices.Contains(cfg.Commits.Footers.Keys, "Task") {
			t.Errorf("%+v: no Task footer", s)
		}
		if got := slices.Contains(cfg.Commits.Footers.Keys, "Scenarios"); got != s.scenarios {
			t.Errorf("%+v: a Scenarios footer: %t", s, got)
		}
		if s.scenarios {
			required := cfg.Get("commits.footers.Scenarios.required_for")
			list, _ := required.([]any)
			if want := map[bool]int{false: 2, true: 0}[s.untagged]; len(list) != want {
				t.Errorf("%+v: Scenarios is required of %v", s, required)
			}
		}
		if got := cfg.Get("commits.since") != value.Undefined; got != (s.since != "") {
			t.Errorf("%+v: commits.since: %t", s, got)
		}
		if got := cfg.Get("pin") != value.Undefined; got != (s.pin != nil) {
			t.Errorf("%+v: a pin: %t", s, got)
		}
	}
}

// The smoke set names each file's first live test, and is an empty list when
// no file has one.
func TestStarterSmokeSet(t *testing.T) {
	text, n := starterSmokeSet(listing(
		[3]string{"ID-A-01", "a.feature", "wip"},
		[3]string{"ID-A-02", "a.feature", ""},
		[3]string{"ID-A-03", "a.feature", ""},
		[3]string{"ID-B-01", "b.feature", ""},
	))
	if n != 2 || !strings.Contains(text, `- file: a.feature
  scenarios:
    - id: "@ID-A-02"`) || !strings.Contains(text, `"@ID-B-01"`) || strings.Contains(text, "ID-A-03") {
		t.Errorf("%d scenarios:\n%s", n, text)
	}
	if text, n := starterSmokeSet(listing([3]string{"ID-A-01", "a.feature", "wip"})); n != 0 || !strings.HasSuffix(text, "[]\n") {
		t.Errorf("%d scenarios:\n%s", n, text)
	}
}

// listing is a listing of tests, each its ID, its file and "wip" or "".
func listing(each ...[3]string) tests.List {
	var list tests.List
	for _, t := range each {
		list.Tests = append(list.Tests, tests.Test{ID: t[0], File: t[1], Live: t[2] != "wip"})
	}
	return list
}
