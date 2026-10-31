package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/git"
	"github.com/donvargax/itos/v8/internal/kind"
)

// attemptPorts bind the attempt slice to the real code: the config, the
// brief slice's request at a commit, the result file named as typed, and
// the journal in itos's folder of the git common dir, none until a line is
// recorded, each line appended; runAttempt reads the line and runs the
// slice, in a repository whose one commit holds the registry, AGENTS.md
// and a feature file tagged for the item.
func TestAttemptPortsBindTheRealCode(t *testing.T) {
	dir := gitConfigRepo(t, "version: 1\nwork:\n  registry: work-items.yaml\ntests:\n  scenario: { root: features, id: \"ID-[A-Z]+-\\\\d+\" }\n")
	writeFiles(t, map[string]string{
		"work-items.yaml":         "phases: { 1: null }\nitems:\n  - { id: slice-9, title: Nine, phase: 1, owner: null, status: todo, depends_on: [] }\n",
		"AGENTS.md":               "Rules.\n",
		"features/tagged.feature": "Feature: Tagged\n\n  @ID-A-01 @wip @slice-9\n  Scenario: one\n    When it runs\n",
	})
	for _, args := range [][]string{
		{"add", "--", "."},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--no-verify", "-m", "test: a tagged scenario"},
	} {
		if out, err := exec.Command(git.Bin(runtime.GOOS), append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %s: %s", strings.Join(args, " "), out)
		}
	}
	out, err := exec.Command(git.Bin(runtime.GOOS), "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(string(out))
	p := attemptPorts{o: Out{Dir: dir}}
	p.git.Dir = dir
	ctx := t.Context()
	if cfg, err := p.Config(ctx); err != nil || cfg.Work.Registry != "work-items.yaml" {
		t.Errorf("Config = %v, %v", cfg, err)
	}
	request, problems, err := p.Request(ctx, "slice-9", "implementer", head)
	if err != nil || len(problems) != 0 || !strings.HasPrefix(request, "sha256:") {
		t.Errorf("Request = %q, %+v, %v", request, problems, err)
	}
	if _, problems, err := p.Request(ctx, "slice-7", "implementer", head); err != nil || len(problems) != 1 || problems[0].Rule != "work-unknown-item" {
		t.Errorf("Request of an unknown item = %+v, %v", problems, err)
	}
	if _, err := p.ReadFile(ctx, "missing.json"); err == nil || !strings.Contains(err.Error(), "open missing.json:") {
		t.Errorf("ReadFile of a missing file: %v", err)
	}
	if data, err := p.Journal(ctx, "slice-9"); err != nil || data != nil {
		t.Errorf("Journal before any record = %q, %v", data, err)
	}
	for _, line := range []string{"{\"a\":1}\n", "{\"b\":2}\n"} {
		if err := p.Record(ctx, "slice-9", []byte(line)); err != nil {
			t.Fatal(err)
		}
	}
	journal := filepath.Join(dir, ".git", "itos", "attempts", "slice-9.jsonl")
	if data, err := os.ReadFile(journal); err != nil || string(data) != "{\"a\":1}\n{\"b\":2}\n" {
		t.Errorf("the journal on disk = %q, %v", data, err)
	}
	if data, err := p.Journal(ctx, "slice-9"); err != nil || string(data) != "{\"a\":1}\n{\"b\":2}\n" {
		t.Errorf("Journal = %q, %v", data, err)
	}
	// A journal that is a folder cannot be read or appended to; a folder of
	// attempts that is a file cannot be made.
	if err := os.Mkdir(filepath.Join(dir, ".git", "itos", "attempts", "slice-8.jsonl"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Journal(ctx, "slice-8"); err == nil {
		t.Error("Journal of a folder is no error")
	}
	if err := p.Record(ctx, "slice-8", []byte("x\n")); err == nil {
		t.Error("Record into a folder is no error")
	}
	if err := os.RemoveAll(filepath.Join(dir, ".git", "itos", "attempts")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, ".git", "itos", "attempts"), "not a folder\n")
	if err := p.Record(ctx, "slice-9", []byte("x\n")); err == nil {
		t.Error("Record under a file is no error")
	}
	if err := os.Remove(filepath.Join(dir, ".git", "itos", "attempts")); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := run("attempt", "submit", "slice-9", "result.json", "--json")
	if code != 1 || !strings.Contains(stdout, "attempt-malformed") || !strings.Contains(stdout, "result.json") {
		t.Errorf("attempt submit of a missing file: exit %d, stdout %s, stderr %s", code, stdout, stderr)
	}
	if code, _, stderr := run("attempt", "submit", "slice-9"); code != ExitUsage || !strings.Contains(stderr, "result") {
		t.Errorf("attempt submit without a file: exit %d, stderr %q", code, stderr)
	}
	// Outside a repository there is no git common dir to keep a journal in:
	// a missing environment.
	outside := attemptPorts{o: Out{Dir: t.TempDir()}}
	outside.git.Dir = outside.o.Dir
	if _, err := outside.Journal(ctx, "slice-9"); err == nil || kind.Of(err) != kind.Missing {
		t.Errorf("Journal outside a repository: %v", err)
	}
	if err := outside.Record(ctx, "slice-9", []byte("x\n")); err == nil || kind.Of(err) != kind.Missing {
		t.Errorf("Record outside a repository: %v", err)
	}
}
