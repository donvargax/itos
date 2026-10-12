// Package programs holds the self-test of how the gates start the repository's programs (T-135,
// a Go test since T-143).
//
// The programs in tools/bin are shell scripts, which linux and macOS start by their #! line and
// windows cannot start at all: a spawn of one there ends with no exit code, and the gate that ran
// it fails without saying why. Every Go program and self-test under tools/ starts one of them
// through one resolver (scratch.Program), so these checks run the real programs the way those
// start them, and a change that leaves windows out is caught here rather than in a gate's own
// output. Cheap and offline, so it can also run by hand on windows.
//
// The programs are the repository's, run in the repository: tools/bin/itos builds and runs the Go
// binary from this tree, and tools/bin/dev-version asks git what version this checkout is.
// Neither reaches the network.
package programs

import (
	"path/filepath"
	"regexp"
	"testing"

	"github.com/donvargax/itos/v8/tools/selftest/scratch"
)

func TestSelftest(t *testing.T) {
	scratch.Gate(t)
	root := scratch.Root(t)
	answers := func(t *testing.T, script string, wanted *regexp.Regexp, args ...string) {
		t.Helper()
		name, argv := scratch.Program(filepath.Join(root, "tools", "bin", script), args...)
		r := scratch.Run(root, scratch.OutsideEnv(), "", name, argv...)
		if r.Code != 0 {
			t.Fatalf("tools/bin/%s exited %d:\n%s", script, r.Code, r.Output())
		}
		if !wanted.MatchString(r.Stdout) {
			t.Errorf("tools/bin/%s printed:\n%s", script, r.Output())
		}
	}

	// tools/bin/itos is what every hook, every task check and every self-test runs.
	t.Run("itos answers its version", func(t *testing.T) {
		answers(t, "itos", regexp.MustCompile(`(?m)^itos \S`), "--version")
	})

	// tools/bin/dev-version stamps the binary tools/bin/itos builds.
	t.Run("dev-version answers a version", func(t *testing.T) {
		answers(t, "dev-version", regexp.MustCompile(`\S`), root)
	})
}
