package evidence

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
)

// keyOf is the key a request has in the rig's engine.
func keyOf(t *testing.T, r *rig, req Request, tree string) string {
	t.Helper()
	b, err := r.bind(t.Context(), req, tree)
	if err != nil {
		t.Fatal(err)
	}
	return b.Key()
}

// The OS, the architecture and itos's version are bound, which no scenario
// can vary: each changes the key, and the same binding gives the same one.
func TestTheKeyBindsTheOSTheArchitectureAndTheVersion(t *testing.T) {
	r := newRig(t)
	req := r.request("true", config.Evidence{})
	base := keyOf(t, r, req, "tree")
	if again := keyOf(t, r, req, "tree"); again != base {
		t.Errorf("the same request, two keys: %s %s", base, again)
	}
	for name, binding := range map[string]Binding{
		"the OS":           {OS: "other", Arch: r.Binding.Arch, Version: r.Binding.Version},
		"the architecture": {OS: r.Binding.OS, Arch: "other", Version: r.Binding.Version},
		"the version":      {OS: r.Binding.OS, Arch: r.Binding.Arch, Version: "9.9.9"},
	} {
		other := newRig(t)
		other.Binding = binding
		if got := keyOf(t, other, req, "tree"); got == base {
			t.Errorf("another %s, the same key", name)
		}
	}
	if got := ThisBinary(); got.OS != runtime.GOOS || got.Arch != runtime.GOARCH || got.Version == "" {
		t.Errorf("this binary's binding: %+v", got)
	}
}

// The tree, the command, the timeout, the policy and the declaration are
// bound; the order a declaration is written in is not.
func TestTheKeyBindsWhatTheCheckRanOn(t *testing.T) {
	r := newRig(t)
	t.Setenv("ITOS_EVIDENCE_A", "one")
	t.Setenv("ITOS_EVIDENCE_B", "two")
	decl := config.Evidence{Env: []string{"ITOS_EVIDENCE_A", "ITOS_EVIDENCE_B"}, Inputs: []string{"x", "y"}, Tools: []string{"sh"}}
	req := r.request("sh -c  true", decl)
	base := keyOf(t, r, req, "tree")
	reordered := config.Evidence{Env: []string{"ITOS_EVIDENCE_B", "ITOS_EVIDENCE_A", "ITOS_EVIDENCE_A"}, Inputs: []string{"y", "x"}, Tools: []string{"sh"}}
	if got := keyOf(t, r, r.request("sh -c true", reordered), "tree"); got != base {
		t.Error("the order of a declaration, or a name twice, or the whitespace of a command, changed the key")
	}
	timed := req
	timed.Seconds = 3
	other := newRig(t)
	other.Cfg = load(t, "version: 1\nledger: { files: \"t/{group}.yaml\", check: { timeout: 7 } }\n")
	for name, key := range map[string]string{
		"the tree":    keyOf(t, r, req, "other"),
		"the command": keyOf(t, r, r.request("sh -c false", decl), "tree"),
		"the timeout": keyOf(t, r, timed, "tree"),
		"the policy":  keyOf(t, other, req, "tree"),
		"the declaration": keyOf(t, r, r.request("sh -c  true", config.Evidence{Env: decl.Env, Inputs: decl.Inputs,
			Tools: []string{"sh", "cat"}}), "tree"),
	} {
		if key == base {
			t.Errorf("another %s, the same key", name)
		}
	}
	t.Setenv("ITOS_EVIDENCE_A", "changed")
	if keyOf(t, r, req, "tree") == base {
		t.Error("another value of a declared variable, the same key")
	}
	t.Setenv("ITOS_EVIDENCE_A", "one")
	unset := os.Unsetenv
	if err := unset("ITOS_EVIDENCE_B"); err != nil {
		t.Fatal(err)
	}
	if keyOf(t, r, req, "tree") == base {
		t.Error("a declared variable unset, the same key")
	}
}

// The policy is the settings, not where they were read from, in whatever
// order the file wrote them.
func TestThePolicyIsTheSettingsAlone(t *testing.T) {
	a, b := newRig(t), newRig(t)
	a.Cfg = load(t, "version: 1\nledger: { check: { timeout: 7 }, files: \"t/{group}.yaml\" }\n")
	b.Cfg = load(t, "version: 1\nledger: { files: \"t/{group}.yaml\", check: { timeout: 7 } }\n")
	b.Cfg.Path, b.Cfg.Stealth = "/elsewhere/itos.yaml", true
	req := a.request("true", config.Evidence{})
	if keyOf(t, a, req, "tree") != keyOf(t, b, req, "tree") {
		t.Error("the order of the file's keys, its path or its stealth flag changed the key")
	}
}

// ci.env is bound when the run applies it, the names and the values, and a
// ci run with none has the key of a hand run.
func TestCIEnvIsBoundWhenAppliedAndOnlyThen(t *testing.T) {
	r := newRig(t)
	req := r.request("true", config.Evidence{})
	base := keyOf(t, r, req, "tree")
	r.CIEnv = map[string]string{}
	if keyOf(t, r, req, "tree") != base {
		t.Error("a ci.env with nothing in it changed the key")
	}
	r.CIEnv = map[string]string{"A": "1", "B": "2"}
	withEnv := keyOf(t, r, req, "tree")
	if withEnv == base {
		t.Error("ci.env, the same key")
	}
	r.CIEnv = map[string]string{"A": "1", "B": "3"}
	if keyOf(t, r, req, "tree") == withEnv {
		t.Error("another value of ci.env, the same key")
	}
}

// A declared input is absent when missing, which an empty file is not; a
// tracked one is the tree's; a folder holds what git does not track.
func TestDeclaredInputsAreHashedAsTheyAre(t *testing.T) {
	r := newRig(t)
	req := r.request("true", config.Evidence{Inputs: []string{"gen", "seed.txt"}})
	absent := keyOf(t, r, req, "tree")
	write := func(name, text string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(r.work, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(r.work, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("seed.txt", "")
	empty := keyOf(t, r, req, "tree")
	write("seed.txt", "one")
	one := keyOf(t, r, req, "tree")
	write("gen/a.txt", "a")
	folder := keyOf(t, r, req, "tree")
	if absent == empty || empty == one || one == folder || absent == one {
		t.Errorf("absent, empty, one and folder keys: %s %s %s %s", absent, empty, one, folder)
	}
	r.git.indexed = []string{"seed.txt", "gen/a.txt"}
	r.tracked = nil
	tracked := keyOf(t, r, req, "tree")
	write("seed.txt", "edited in the working folder")
	if keyOf(t, r, req, "tree") != tracked {
		t.Error("the working copy of a tracked input changed the key")
	}
	if err := os.Remove(filepath.Join(r.work, "seed.txt")); err != nil {
		t.Fatal(err)
	}
	if keyOf(t, r, req, "tree") != tracked {
		t.Error("the removal of a tracked input's working copy changed the key")
	}
}

// The index is listed only for a declaration with inputs to classify: one with
// none binds without asking git for it, so a git that cannot list it does not
// stop the key, and one with inputs fails with git's error.
func TestTheIndexIsListedOnlyForDeclaredInputs(t *testing.T) {
	r := newRig(t)
	r.git.indexedErr = errors.New("index unreadable")
	if _, err := r.bind(t.Context(), r.request("true", config.Evidence{}), "tree"); err != nil {
		t.Errorf("no inputs declared: %v", err)
	}
	if _, err := r.bind(t.Context(), r.request("true", config.Evidence{Inputs: []string{"x"}}), "tree"); !errors.Is(err, r.git.indexedErr) {
		t.Errorf("an input declared: %v, want git's error", err)
	}
}

// A declared tool is hashed by its content, wherever the PATH finds it, and
// absent when it is not on it.
func TestToolsAreHashedByContent(t *testing.T) {
	r := newRig(t)
	bin := t.TempDir()
	name := "itos-evidence-tool"
	file := name
	if runtime.GOOS == "windows" {
		file += ".exe"
	}
	t.Setenv("PATH", bin)
	req := r.request("true", config.Evidence{Tools: []string{name}})
	absent := keyOf(t, r, req, "tree")
	if err := os.WriteFile(filepath.Join(bin, file), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	one := keyOf(t, r, req, "tree")
	if err := os.WriteFile(filepath.Join(bin, file), []byte("#!/bin/sh\n# another\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if two := keyOf(t, r, req, "tree"); absent == one || one == two {
		t.Errorf("absent, one and another tool keys: %s %s %s", absent, one, two)
	}
}
