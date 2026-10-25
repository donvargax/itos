// Package properties holds the self-test of the nightly's property-test step (T-310, ADR-0051):
// tools/bin/nightly-properties runs every TestProperty with a seed of its own, drawn anew each
// run, which it prints, and a thousand checks, and fails as go test fails; and the nightly's plan
// runs it, and itos.yaml names no seed (a push pins the seed in internal/proptest). It runs the script
// with a fake go on the PATH that records what it was handed.
package properties

import (
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/tools/selftest/scratch"
)

// run is the script run with a go that records its arguments and RAPID_SEED and RAPID_CHECKS to
// record and exits with code.
func run(t *testing.T, code int) (result scratch.Result, record string) {
	t.Helper()
	bin := t.TempDir()
	record = filepath.Join(t.TempDir(), "go")
	fake := "#!/bin/sh\nprintf '%s\\n' \"$*\" \"${RAPID_SEED-unset}\" \"${RAPID_CHECKS-unset}\" > '" + record + "'\nexit " + strconv.Itoa(code) + "\n"
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	env := scratch.With(scratch.OutsideEnv(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	name, args := scratch.Program(filepath.Join(scratch.Root(t), "tools", "bin", "nightly-properties"))
	return scratch.Run(t.TempDir(), env, "", name, args...), record
}

// seen is what the fake go recorded: its arguments, RAPID_SEED and RAPID_CHECKS.
func seen(t *testing.T, record string) (args, seed, checks string) {
	t.Helper()
	text, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("go never ran: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(text), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("recorded %q", text)
	}
	return lines[0], lines[1], lines[2]
}

func TestSelftest(t *testing.T) {
	scratch.Gate(t)
	if runtime.GOOS == "windows" {
		t.Skip("tools/bin/nightly-properties is a shell script run by the Linux nightly")
	}
	t.Run("the properties run with a printed seed and many checks", func(t *testing.T) {
		result, record := run(t, 0)
		if result.Code != 0 {
			t.Fatalf("exit %d:\n%s", result.Code, result.Output())
		}
		args, seed, checks := seen(t, record)
		if args != "test -count=1 -run ^TestProperty ./cmd/... ./internal/..." {
			t.Errorf("go was run as go %s", args)
		}
		if n, err := strconv.ParseUint(seed, 10, 64); err != nil || n == 0 {
			t.Errorf("RAPID_SEED = %q, want a seed rapid takes: a number that is not 0, which it draws from", seed)
		}
		if checks != "1000" {
			t.Errorf("RAPID_CHECKS = %q, want 1000", checks)
		}
		if want := "property tests: RAPID_SEED=" + seed + " RAPID_CHECKS=1000"; !strings.Contains(result.Stdout, want) {
			t.Errorf("the run did not print its seed (%q):\n%s", want, result.Output())
		}
	})
	t.Run("each run draws a seed of its own", func(t *testing.T) {
		seeds := map[string]bool{}
		for range 4 {
			result, record := run(t, 0)
			if result.Code != 0 {
				t.Fatalf("exit %d:\n%s", result.Code, result.Output())
			}
			_, seed, _ := seen(t, record)
			seeds[seed] = true
		}
		if len(seeds) < 4 {
			t.Errorf("4 runs drew %d seeds: %v", len(seeds), slices.Sorted(maps.Keys(seeds)))
		}
	})
	t.Run("a failing property fails the step", func(t *testing.T) {
		if result, _ := run(t, 3); result.Code != 3 {
			t.Errorf("exit %d, want go test's 3:\n%s", result.Code, result.Output())
		}
	})
	t.Run("the nightly's plan runs the step and itos.yaml names no seed", func(t *testing.T) {
		root := scratch.Root(t)
		nightly := 0
		for _, step := range scratch.CIPlan(t, root, "--nightly").Steps {
			if strings.Contains(step, "tools/bin/nightly-properties") {
				nightly++
			}
		}
		if nightly != 1 {
			t.Errorf("the nightly plan runs tools/bin/nightly-properties %d times, want once", nightly)
		}
		config, err := os.ReadFile(filepath.Join(root, "itos.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if n := strings.Count(string(config), "RAPID_SEED"); n != 0 {
			t.Errorf("itos.yaml names RAPID_SEED %d times: a push pins the seed (internal/proptest), only tools/bin/nightly-properties draws one", n)
		}
	})
}
