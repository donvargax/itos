package cli

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/donvargax/itos/v5/internal/config"
	"github.com/donvargax/itos/v5/internal/kind"
)

// Every exit code comes from the kind of the error, never from a default
// (slice 86, decision 36): an error nobody classified exits 70, which no
// scenario can provoke on purpose.
func TestTheExitCodeIsTheErrorsKind(t *testing.T) {
	temporary := kind.Wrap(kind.Temporary, errors.New("connection refused"))
	for _, c := range []struct {
		name string
		err  error
		want int
	}{
		{"a usage error", usage("pin takes only a <version> (--bogus)"), ExitUsage},
		{"a config error", config.Invalid("itos.yaml", "ci.watch.provider is none"), ExitUsage},
		{"a data file itos refuses", kind.Wrap(kind.Usage, errors.New("asks.yaml: two questions have the id q-1")), ExitUsage},
		{"a missing environment", kind.Wrap(kind.Missing, errors.New("no git on the PATH but itos")), ExitMissing},
		{"a temporary failure", temporary, ExitTemporary},
		{"a temporary failure wrapped after", fmt.Errorf("cannot fetch itos 9.1.0: %w", temporary), ExitTemporary},
		{"a kind given over another", kind.Wrap(kind.Missing, temporary), ExitMissing},
		{"a kind left as it was", kind.Else(kind.Missing, temporary), ExitTemporary},
		{"an error of no kind", errors.New("Command failed: git rev-list nosuchref"), ExitSoftware},
		{"an error of no kind, wrapped", fmt.Errorf("cannot write: %w", errors.New("disk full")), ExitSoftware},
	} {
		if got := ExitCode(c.err); got != c.want {
			t.Errorf("%s: exit %d, want %d", c.name, got, c.want)
		}
	}
}

func TestAnUnclassifiedFailureExits70WithItsMessage(t *testing.T) {
	var stdout, stderr strings.Builder
	code := failure(errors.New("something itos did not expect"), Out{Stdout: &stdout, Stderr: &stderr})
	if code != ExitSoftware || stderr.String() != "itos: something itos did not expect\n" || stdout.Len() != 0 {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}
