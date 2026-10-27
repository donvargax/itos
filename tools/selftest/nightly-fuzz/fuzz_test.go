// Package fuzz holds the self-test of the nightly's fuzz step (T-328, ADR-0051):
// tools/bin/nightly-fuzz runs every Fuzz target go test -list finds under cmd and internal, each
// alone for a bounded time, goes on past a target that fails, prints each failing input's path
// and bytes and exits 1 once all have run; and the nightly's plan runs it. It runs the script in a
// scratch module with a fake go on the PATH that lists two targets of one package and records
// each fuzz run.
package fuzz

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/tools/selftest/scratch"
)

// fakeGo lists FuzzA and FuzzB of internal/a (internal/b and cmd/c have none), and for a fuzz run
// appends its arguments to $RECORD, failing with a failing input written when its target is
// $FAIL.
const fakeGo = `#!/bin/sh
case "$*" in
"test -list ^Fuzz ./cmd/... ./internal/...")
	if [ -n "${LIST_FAILS-}" ]; then echo "build failed"; exit 1; fi
	printf 'FuzzA\nFuzzB\nok  \texample.com/m/internal/a\t0.01s\nok  \texample.com/m/internal/b\t0.01s\n?   \texample.com/m/cmd/c\t[no test files]\n'
	;;
*)
	echo "$*" >> "$RECORD"
	target=${6#^}
	target=${target%\$}
	if [ "$target" = "${FAIL-}" ]; then
		printf -- '--- FAIL: %s (0.01s)\n    Failing input written to testdata/fuzz/%s/abc\n' "$target" "$target"
		exit 1
	fi
	;;
esac
`

// noTargets is a go whose list holds no target.
const noTargets = `#!/bin/sh
case "$*" in
"test -list ^Fuzz ./cmd/... ./internal/...") printf 'ok  \texample.com/m/internal/a\t0.01s\n' ;;
*) echo "$*" >> "$RECORD" ;;
esac
`

// run is the script run in a scratch module with a go made of script, FAIL and LIST_FAILS set as
// given, and the arguments; record is what the fake go recorded of the fuzz runs, a line each.
func run(t *testing.T, script, fail string, listFails bool, args ...string) (result scratch.Result, record []string) {
	t.Helper()
	bin, module := t.TempDir(), t.TempDir()
	recordFile := filepath.Join(t.TempDir(), "go")
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(module, "internal", "a", "testdata", "fuzz", "FuzzB")
	if err := os.MkdirAll(input, 0o755); err != nil {
		t.Fatal(err)
	}
	for path, text := range map[string]string{
		filepath.Join(module, "go.mod"): "module example.com/m\n\ngo 1.27\n",
		filepath.Join(input, "abc"):     "go test fuzz v1\nstring(\"0000\")\n",
	} {
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	env := scratch.With(scratch.OutsideEnv(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"RECORD="+recordFile, "FAIL="+fail)
	if listFails {
		env = scratch.With(env, "LIST_FAILS=1")
	}
	name, a := scratch.Program(filepath.Join(scratch.Root(t), "tools", "bin", "nightly-fuzz"))
	result = scratch.Run(module, env, "", name, append(a, args...)...)
	if text, err := os.ReadFile(recordFile); err == nil {
		record = strings.Split(strings.TrimSuffix(string(text), "\n"), "\n")
	}
	return result, record
}

func TestSelftest(t *testing.T) {
	scratch.Gate(t)
	if runtime.GOOS == "windows" {
		t.Skip("tools/bin/nightly-fuzz is a shell script run by the Linux nightly")
	}
	pkg := "example.com/m/internal/a"
	t.Run("each target of each package runs alone for sixty seconds", func(t *testing.T) {
		result, record := run(t, fakeGo, "", false)
		if result.Code != 0 {
			t.Fatalf("exit %d:\n%s", result.Code, result.Output())
		}
		want := []string{
			"test " + pkg + " -run ^$ -fuzz ^FuzzA$ -fuzztime 60s",
			"test " + pkg + " -run ^$ -fuzz ^FuzzB$ -fuzztime 60s",
		}
		if strings.Join(record, "\n") != strings.Join(want, "\n") {
			t.Errorf("go ran:\n%s\nwant:\n%s", strings.Join(record, "\n"), strings.Join(want, "\n"))
		}
	})
	t.Run("the first argument is the fuzz time", func(t *testing.T) {
		_, record := run(t, fakeGo, "", false, "5s")
		if len(record) != 2 || !strings.HasSuffix(record[0], "-fuzztime 5s") || !strings.HasSuffix(record[1], "-fuzztime 5s") {
			t.Errorf("go ran:\n%s", strings.Join(record, "\n"))
		}
	})
	t.Run("a failing target fails the step after the others ran, and its input is printed", func(t *testing.T) {
		result, record := run(t, fakeGo, "FuzzA", false)
		if result.Code != 1 {
			t.Errorf("exit %d, want 1:\n%s", result.Code, result.Output())
		}
		if len(record) != 2 {
			t.Errorf("go ran %d targets, want both:\n%s", len(record), strings.Join(record, "\n"))
		}
		result, _ = run(t, fakeGo, "FuzzB", false)
		for _, want := range []string{
			"Failing input written to testdata/fuzz/FuzzB/abc\n",
			"fuzz input: internal/a/testdata/fuzz/FuzzB/abc\ngo test fuzz v1\nstring(\"0000\")\n",
		} {
			if !strings.Contains(result.Stdout, want) {
				t.Errorf("the output lacks %q:\n%s", want, result.Output())
			}
		}
	})
	t.Run("no target is a pass that says so", func(t *testing.T) {
		result, record := run(t, noTargets, "", false)
		if result.Code != 0 || len(record) != 0 || !strings.Contains(result.Stdout, "no Fuzz target") {
			t.Errorf("exit %d, go ran %v:\n%s", result.Code, record, result.Output())
		}
	})
	t.Run("a package that does not build fails the step", func(t *testing.T) {
		result, record := run(t, fakeGo, "", true)
		if result.Code != 1 || len(record) != 0 || !strings.Contains(result.Stdout, "build failed") {
			t.Errorf("exit %d, go ran %v:\n%s", result.Code, record, result.Output())
		}
	})
	t.Run("the nightly's plan runs the step once", func(t *testing.T) {
		n := 0
		for _, step := range scratch.CIPlan(t, scratch.Root(t), "--nightly").Steps {
			if strings.Contains(step, "tools/bin/nightly-fuzz") {
				n++
			}
		}
		if n != 1 {
			t.Errorf("the nightly plan runs tools/bin/nightly-fuzz %d times, want once", n)
		}
	})
}
