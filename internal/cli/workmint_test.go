package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func registryWriterRepo(t *testing.T, registry string) string {
	t.Helper()
	dir := gitConfigRepo(t, "version: 1\nledger:\n  files: \"tasks/phase-{group}.yaml\"\n  id: \"T-\\\\d+\"\nwork:\n  registry: registry.yaml\n")
	standInHooks(t)
	gitIn(t, "config", "user.name", "itos test")
	gitIn(t, "config", "user.email", "test@localhost")
	if err := os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte(registry), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, "add", "--", "itos.yaml", "registry.yaml")
	gitIn(t, "commit", "-q", "-m", "docs: start")
	return dir
}

func TestWorkArgsOptionalIDDistinguishesMissingAndExplicitIDs(t *testing.T) {
	flags := []string{"--kind", "--title", "--why"}
	id, hasID, values, err := workArgsOptionalID("add", []string{"--kind", "slice", "--title", "Slice", "--why", "Because."}, flags...)
	if err != nil || id != "" || hasID || values["--kind"] != "slice" {
		t.Fatalf("no-ID parse = (%q, %t, %v, %v)", id, hasID, values, err)
	}
	id, hasID, _, err = workArgsOptionalID("add", []string{"p1-idea", "--kind", "idea", "--title", "Idea", "--why", "Because."}, flags...)
	if err != nil || id != "p1-idea" || !hasID {
		t.Fatalf("idea-ID parse = (%q, %t, %v)", id, hasID, err)
	}
	if _, _, _, err := workArgsOptionalID("add", []string{"slice-1", "slice-2", "--kind", "slice"}, flags...); err == nil {
		t.Fatal("workArgsOptionalID accepted two positional IDs")
	}
}

func TestWorkAddMintsSliceAndRefusesCallerID(t *testing.T) {
	dir := registryWriterRepo(t, "phases: { 1: null }\nitems:\n  - { id: slice-9, title: Nine, phase: 1, owner: null, status: todo, depends_on: [] }\n")
	var stdout, stderr strings.Builder
	code, err := workAdd([]string{"--kind", "slice", "--phase", "1", "--title", "Ten", "--why", "A slice."}, Out{Stdout: &stdout, Stderr: &stderr})
	if err != nil || code != 0 || !strings.Contains(stdout.String(), "slice-10") {
		t.Fatalf("workAdd = (%d, %v), stdout=%q stderr=%q", code, err, stdout.String(), stderr.String())
	}
	text, err := os.ReadFile(filepath.Join(dir, "registry.yaml"))
	if err != nil || !strings.Contains(string(text), "id: slice-10") {
		t.Fatalf("registry after workAdd = %q (%v), want slice-10", text, err)
	}
	before := string(text)
	code, err = workAdd([]string{"slice-11", "--kind", "slice", "--phase", "1", "--title", "Eleven", "--why", "Not caller named."}, Out{Stdout: &stdout, Stderr: &stderr})
	if code != 0 || err == nil || !strings.Contains(err.Error(), "mints the slice id") {
		t.Fatalf("explicit-ID workAdd = (%d, %v), want usage error", code, err)
	}
	after, readErr := os.ReadFile(filepath.Join(dir, "registry.yaml"))
	if readErr != nil || string(after) != before {
		t.Fatalf("explicit-ID workAdd changed registry: %q (%v)", after, readErr)
	}
}

func TestWorkPromoteMintsSliceAndRewritesDependencies(t *testing.T) {
	dir := registryWriterRepo(t, "phases: { 1: null }\nitems:\n"+
		"  - { id: p1-idea, title: Idea, phase: 1, status: todo, kind: idea, why: Missing }\n"+"  - { id: slice-9, title: Nine, phase: 1, owner: null, status: todo, depends_on: [p1-idea] }\n")
	var stdout, stderr strings.Builder
	code, err := workPromote([]string{"p1-idea", "--kind", "slice"}, Out{Stdout: &stdout, Stderr: &stderr})
	if err != nil || code != 0 || !strings.Contains(stdout.String(), "slice-10") {
		t.Fatalf("workPromote = (%d, %v), stdout=%q stderr=%q", code, err, stdout.String(), stderr.String())
	}
	text, err := os.ReadFile(filepath.Join(dir, "registry.yaml"))
	if err != nil || !strings.Contains(string(text), "id: slice-10") || !strings.Contains(string(text), "depends_on: [slice-10]") {
		t.Fatalf("registry after workPromote = %q (%v), want promoted ID and rewritten dependency", text, err)
	}
}
