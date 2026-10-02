package message

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/donvargax/itos/internal/config"
)

// A footer's IDs over several lines, split at whitespace and commas, the
// prefix taken off where it is given; a line that only mentions the key is
// not the footer.
func TestIDs(t *testing.T) {
	text := "feat: x\n\nScenarios: @ID-A-1, @ID-B-2\nScenarios:ID-C-3 @ID-D-4\r\nSee Scenarios: ID-E-5\n"
	got := IDs(text, "Scenarios", "@")
	if want := []string{"ID-A-1", "ID-B-2", "ID-C-3", "ID-D-4"}; !reflect.DeepEqual(got, want) {
		t.Errorf("IDs %q", got)
	}
	if got := Type("feat(scope): x"); got != "feat" {
		t.Errorf("Type %q", got)
	}
	if got := Type(": x"); got != "" {
		t.Errorf("Type %q", got)
	}
}

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

// The delegate's report: each problem line with its level and fix, the rest
// left out, and a failure with no error among them one header-lint problem.
func TestParseReport(t *testing.T) {
	cfg := load(t, "version: 1\n")
	report := "⧗   input: x\n  ✖   subject may not be empty [subject-empty]  \n⚠  lines are long [body-max-line-length]\n✖ [type-empty]\n"
	got, err := ParseReport(cfg, report, true)
	if err != nil {
		t.Fatal(err)
	}
	want := []Leveled{
		{Rule: "subject-empty", Message: "subject may not be empty", Fix: fixes["subject-empty"], Level: "error"},
		{Rule: "body-max-line-length", Message: "lines are long", Fix: fixes["body-max-line-length"], Level: "warning"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseReport %+v", got)
	}
	got, _ = ParseReport(cfg, "\n  no config  \n", true)
	if len(got) != 1 || got[0].Rule != "header-lint" || got[0].Message != "no config" {
		t.Errorf("ParseReport of a bare failure %+v", got)
	}
}
