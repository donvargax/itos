package tests

import (
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
)

// A smoke set read at a commit: the commit's file, an error when the commit
// cannot be read, and one naming the file when the commit lacks it.
func TestLoadSmokeAt(t *testing.T) {
	cfg := load(t, `version: 1
tests:
  scenario: { root: features, id: "ID-[A-Z]+-\\d+", tag_prefix: "@", smoke: { file: features/smoke.yaml, every_file: false } }
`)
	cfg.Trees = porttest.Trees{Common: ".git", Files: map[string]map[string]string{
		"abc": {"features/smoke.yaml": "- file: features/a.feature\n  scenarios: []\n"},
	}}
	if smoke, err := LoadSmokeAt(cfg, "scenario", "abc"); err != nil || len(smoke) != 1 || smoke[0].File != "features/a.feature" {
		t.Errorf("the commit's smoke set: %+v, %v", smoke, err)
	}
	if _, err := LoadSmokeAt(cfg, "scenario", "def"); err == nil || err.Error() != "def holds no features/smoke.yaml" {
		t.Errorf("a commit without the file: %v", err)
	}
	cfg.Trees = porttest.Trees{}
	if _, err := LoadSmokeAt(cfg, "scenario", "abc"); err == nil || err.Error() != "abc cannot be read outside a git repository" {
		t.Errorf("a commit outside a repository: %v", err)
	}
}

// The moves rule reads feature files, so a kind whose adapter is a command
// is a config error naming it.
func TestMovesIndexRefusesACommandKind(t *testing.T) {
	cfg := load(t, `version: 1
tests:
  unit: { adapter: { command: sh lists.sh } }
`)
	_, err := NewMoves(cfg, nil).Index("unit")
	if err == nil || !strings.Contains(err.Error(), "tests.unit.adapter is not gherkin") {
		t.Errorf("a command kind's moves: %v", err)
	}
}
