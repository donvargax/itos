package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/donvargax/itos/v8/internal/cli"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/launch"
)

// parse hands the launcher every field of cli.Parse it reads: the global
// flags of a built-in command wherever they stand, and of an extension only
// those before its name.
func TestParse(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	got := parse([]string{"--root", "r", "work", "--config", "c.yaml", "-h", "x"})
	want := launch.Line{Help: true, Config: "c.yaml", Root: "r", Rest: []string{"work", "x"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parse = %+v, want %+v", got, want)
	}
	// An extension's program, by the name each OS's PATH lookup finds.
	dir := t.TempDir()
	for _, name := range []string{"itos-probe", "itos-probe.exe"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	args := []string{"--root", "r", "probe", "--config", "c.yaml"}
	got = parse(args)
	if got.Extension == "" || got.Extension != cli.Parse(args).Extension || got.Root != "r" || got.Config != "" ||
		!slices.Equal(got.Rest, []string{"probe", "--config", "c.yaml"}) {
		t.Errorf("an extension: parse = %+v", got)
	}
}

// commandLine hands the launcher internal/cli's parser, upgrade target and
// exit codes.
func TestCommandLine(t *testing.T) {
	cl := commandLine()
	if got := cl.Parse([]string{"--json", "version"}); !slices.Equal(got.Rest, []string{"version"}) {
		t.Errorf("Parse = %+v", got)
	}
	if code := cl.ExitCode(kind.Wrap(kind.Temporary, errors.New("no answer"))); code != cli.ExitTemporary {
		t.Errorf("ExitCode = %d, want %d", code, cli.ExitTemporary)
	}
	if h, ok, err := cl.UpgradeTarget([]string{"pin"}); ok || err != nil || h.Version != "" {
		t.Errorf("UpgradeTarget(pin) = %+v, %t, %v; want nothing to hand on", h, ok, err)
	}
}
