package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/git"
	"github.com/donvargax/itos/v8/internal/kind"
)

// briefPorts bind the brief slice to the real code: git's tree entries, each
// its kind and object, git's first-parent log and the config; runBrief
// reads the line and runs the slice over them, in a repository whose one
// commit holds the registry, AGENTS.md and a feature file tagged for the
// item.
func TestBriefPortsBindTheRealCode(t *testing.T) {
	dir := gitConfigRepo(t, "version: 1\nwork:\n  registry: work-items.yaml\ntests:\n  scenario: { root: features, id: \"ID-[A-Z]+-\\\\d+\" }\n")
	writeFiles(t, map[string]string{
		"work-items.yaml":         "phases: { 1: null }\nitems:\n  - { id: slice-9, title: Nine, phase: 1, owner: null, status: todo, depends_on: [], refs: [internal/foo, docs/missing.md] }\n",
		"AGENTS.md":               "Rules.\n",
		"features/tagged.feature": "Feature: Tagged\n\n  @ID-A-01 @wip @slice-9\n  Scenario: one\n    When it runs\n",
		"internal/foo/doc.go":     "// Package foo.\n",
	})
	for _, args := range [][]string{
		{"add", "--", "."},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--no-verify", "-m", "test: a tagged scenario"},
	} {
		if out, err := exec.Command(git.Bin(runtime.GOOS), append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %s: %s", strings.Join(args, " "), out)
		}
	}
	head, err := exec.Command(git.Bin(runtime.GOOS), "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	p := briefPorts{o: Out{Dir: dir}}
	p.Git.Dir = dir
	entries, err := p.TreeEntries(t.Context(), "HEAD", []string{"AGENTS.md", "internal/foo", "internal/foo/doc.go", "docs/missing.md"})
	if err != nil || entries["AGENTS.md"].Kind != "blob" || entries["AGENTS.md"].Mode != "100644" || len(entries["AGENTS.md"].Object) != 40 || entries["internal/foo/doc.go"].Kind != "blob" || len(entries) != 2 {
		t.Errorf("TreeEntries = %+v, %v", entries, err)
	}
	if _, err := p.TreeEntries(t.Context(), "nosuchrev", []string{"AGENTS.md"}); err == nil {
		t.Error("TreeEntries of a revision git lacks is no error")
	}
	// The config is HEAD's: an uncommitted change to it, naming another
	// registry, changes no brief.
	writeFile(t, filepath.Join(dir, "itos.yaml"), "version: 1\nwork:\n  registry: other.yaml\n")
	if cfg, err := p.Config(t.Context()); err != nil || cfg.Work.Registry != "work-items.yaml" || cfg.Source.Tree(t.Context()) != "HEAD" {
		t.Errorf("Config = %v, %v", cfg, err)
	}
	code, stdout, stderr := run("brief", "slice-9", "--role", "implementer", "--json")
	var got struct {
		Spec    struct{ Commit string }
		Reads   []struct{ Path, Blob string }
		Omitted []struct{ Path, Reason string }
		Brief   string
	}
	if code != 0 || json.Unmarshal([]byte(stdout), &got) != nil {
		t.Fatalf("brief: exit %d, stdout %s, stderr %s", code, stdout, stderr)
	}
	if got.Spec.Commit != strings.TrimSpace(string(head)) || len(got.Reads) != 3 || got.Reads[2].Path != "internal/foo/doc.go" || got.Reads[2].Blob != entries["internal/foo/doc.go"].Object || len(got.Omitted) != 1 || got.Omitted[0].Path != "docs/missing.md" {
		t.Errorf("brief --json: %+v", got)
	}
	// The top as git prints it: its links resolved (macOS's /var is
	// /private/var), forward slashes on Windows.
	top, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Brief, "in the repository at "+filepath.ToSlash(top)+": Nine.") {
		t.Errorf("the brief names the top as git prints it, %s: %.200q", filepath.ToSlash(top), got.Brief)
	}
	if code, _, stderr := run("brief", "slice-9"); code != ExitUsage || !strings.Contains(stderr, "--role") {
		t.Errorf("brief without --role: exit %d, stderr %q", code, stderr)
	}
	if err := os.Remove(filepath.Join(dir, "itos.yaml")); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := run("brief", "slice-9", "--role", "implementer"); code != 0 {
		t.Errorf("brief with the config gone from the working tree, HEAD's kept: exit %d, stderr %q", code, stderr)
	}
	// Outside a repository there is no HEAD to read the config from: a
	// missing environment.
	outside := briefPorts{o: Out{Dir: t.TempDir()}}
	outside.Git.Dir = outside.o.Dir
	if cfg, err := outside.Config(t.Context()); err == nil || kind.Of(err) != kind.Missing {
		t.Errorf("Config outside a repository = %v, %v, want a missing environment", cfg, err)
	}
}
