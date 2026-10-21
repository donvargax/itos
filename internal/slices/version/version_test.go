package version

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// ports is the version command's ports over fixed answers: the version,
// the config's requires or its failure, and the one range it satisfies.
type ports struct {
	version, satisfied string
	requires           *string
	err                error
}

func (p ports) BuildVersion(context.Context) string { return p.version }

func (p ports) Requires(context.Context) (*string, error) { return p.requires, p.err }

func (p ports) Satisfies(_ context.Context, v, r string) bool {
	return v == p.version && r == p.satisfied
}

func TestParse(t *testing.T) {
	if c := Parse([]string{"--check"}, true); c != (CLI{Check: true, JSON: true}) {
		t.Errorf("--check --json: %+v", c)
	}
	if c := Parse(nil, false); c != (CLI{}) {
		t.Errorf("nothing: %+v", c)
	}
}

// failing is a writer that cannot be written to.
type failing struct{}

func (failing) Write([]byte) (int, error) { return 0, errors.New("closed") }

func TestRun(t *testing.T) {
	held, newer := ">=1.0.0", ">=2.0.0"
	broken := errors.New("itos.yaml cannot be read")
	for _, r := range []struct {
		name           string
		c              CLI
		p              ports
		code           int
		stdout, stderr string
		err            error
	}{
		{"text", CLI{}, ports{version: "1.2.0", satisfied: held, requires: &newer}, 0, "itos 1.2.0\n", "", nil},
		{"json", CLI{JSON: true}, ports{version: "1.2.0", satisfied: held, requires: &held}, 0,
			"{\n  \"schema\": 1,\n  \"version\": \"1.2.0\",\n  \"requires\": \">=1.0.0\",\n  \"satisfied\": true\n}\n", "", nil},
		{"json without requires", CLI{JSON: true}, ports{version: "1.2.0", err: broken}, 0,
			"{\n  \"schema\": 1,\n  \"version\": \"1.2.0\"\n}\n", "", nil},
		{"check held", CLI{Check: true}, ports{version: "1.2.0", satisfied: held, requires: &held}, 0, "itos 1.2.0\n", "", nil},
		{"check without requires", CLI{Check: true}, ports{version: "1.2.0"}, 0, "itos 1.2.0\n", "", nil},
		{"check refused", CLI{Check: true}, ports{version: "1.2.0", satisfied: held, requires: &newer}, 1, "itos 1.2.0\n",
			"itos 1.2.0 does not satisfy >=2.0.0 (the config's requires)\n", nil},
		{"check broken", CLI{Check: true}, ports{version: "1.2.0", err: broken}, 0, "", "", broken},
	} {
		var stdout, stderr strings.Builder
		code, err := r.c.Run(context.Background(), &stdout, &stderr, NewHandler(r.p))
		if code != r.code || stdout.String() != r.stdout || stderr.String() != r.stderr || !errors.Is(err, r.err) {
			t.Errorf("%s: exit %d, %v, stdout %q, stderr %q", r.name, code, err, stdout.String(), stderr.String())
		}
	}
	if _, err := (CLI{JSON: true}).Run(context.Background(), failing{}, failing{}, NewHandler(ports{})); err == nil {
		t.Error("an object it cannot write is no error")
	}
}
