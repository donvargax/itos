package tests

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/donvargax/itos/v2/internal/config"
)

func load(t *testing.T, text string) *config.Loaded {
	t.Helper()
	file := filepath.Join(t.TempDir(), "itos.yaml")
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(file)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

const kinds = `version: 1
tests:
  scenario:
    root: features
    id: "ID-[A-Z]+-\\d+"
    run:
      whole: "go test ./features"
      select: "go test ./features -scenarios={pattern}"
      ids_pattern: "^@(?:{ids})$"
      join: { each: "(?:{p})", sep: "|" }
  bare: { root: features, id: "X" }
`

// The selections merged into one command: whole wins, an empty IDs
// selection adds nothing, IDs lose their tag prefix and repeats, the
// patterns are deduplicated in order and joined, and the whole is one shell
// word; a template the selections need and the kind lacks is a config error.
func TestCommandFor(t *testing.T) {
	cfg := load(t, kinds)
	cases := []struct {
		selections []Selection
		want       string
		ok         bool
	}{
		{nil, "", false},
		{[]Selection{IDs()}, "", false},
		{[]Selection{IDs("@ID-A-01", "ID-A-01", "@ID-B-02")}, `go test ./features -scenarios='^@(?:ID-A-01|ID-B-02)$'`, true},
		{[]Selection{IDs("@ID-A-01"), IDs(), Pattern("^@ID-C-"), IDs("ID-A-01")},
			`go test ./features -scenarios='(?:^@(?:ID-A-01)$)|(?:^@ID-C-)'`, true},
		{[]Selection{IDs("@ID-A-01"), Whole()}, "go test ./features", true},
		{[]Selection{Pattern("it's")}, `go test ./features -scenarios='it'\''s'`, true},
	}
	for _, c := range cases {
		got, ok, err := CommandFor(cfg, "scenario", c.selections)
		if err != nil || got != c.want || ok != c.ok {
			t.Errorf("CommandFor(%+v) = %q, %v, %v", c.selections, got, ok, err)
		}
	}
	if _, ok, err := CommandFor(cfg, "bare", []Selection{IDs()}); ok || err != nil {
		t.Errorf("an empty selection needs no template: %v, %v", ok, err)
	}
	var invalid *config.Error
	_, _, err := CommandFor(cfg, "bare", []Selection{IDs("X")})
	if !errors.As(err, &invalid) || invalid.Problems[0].Message != "tests.bare.run.ids_pattern is missing" {
		t.Errorf("a missing template: %v", err)
	}
}
