package cli

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/draft"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/release"
	"github.com/donvargax/itos/v8/internal/source"
)

// statusRepo is a scratch repository as the current folder, with no remote,
// its config naming the registry and the people, and the files given; the
// release server is an address nothing answers on.
func statusRepo(t *testing.T, files map[string]string) {
	t.Helper()
	dir := gitConfigRepo(t, "version: 1\nwork:\n  registry: work-items.yaml\n  people: { source: yaml, file: people.yaml }\n")
	t.Setenv(release.Env, "http://127.0.0.1:1")
	for name, text := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// items is a registry of the items, each todo, unowned and in phase 1.
func items(ids ...string) string {
	text := "phases: { 1: null }\nitems:\n"
	for _, id := range ids {
		text += "  - { id: " + id + ", title: " + id + ", phase: 1, owner: null, status: todo, depends_on: [] }\n"
	}
	return text
}

// A registry that cannot be read is an error, with no exit code of its own,
// and --as not among the people is exit 3, said on stderr.
func TestStatusRefuses(t *testing.T) {
	statusRepo(t, map[string]string{"work-items.yaml": "items: [\n", "people.yaml": "- a\n"})
	if code, stdout, stderr := run("status", "--as", "a"); code != ExitUsage {
		t.Errorf("a registry that cannot be read: exit %d\n%s%s", code, stdout, stderr)
	}
	cfg, err := config.Load(config.Path())
	if err != nil {
		t.Fatal(err)
	}
	if st, found, code, err := statusOf(cfg, "a", Out{}); err == nil || st != nil || found != nil || code != 0 {
		t.Errorf("a registry that cannot be read: %v %v exit %d, %v", st, found, code, err)
	}
	if err := os.WriteFile("work-items.yaml", []byte(items("one")), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := run("status", "--as", "zed")
	if code != ExitMissing || !strings.Contains(stderr, `"zed" is not in people.yaml`) {
		t.Errorf("--as not among the people: exit %d\n%s", code, stderr)
	}
}

// secondReadFails is the working tree, but for the registry, which reads as
// it is once and fails after: a registry changed between its check and its
// load.
type secondReadFails struct {
	source.Source
	reads int
}

func (s *secondReadFails) Read(path string) (string, error) {
	if path == "work-items.yaml" {
		if s.reads++; s.reads > 1 {
			return "", errors.New("changed while read")
		}
	}
	return s.Source.Read(path)
}

// A registry that cannot be loaded after its check is the error.
func TestStatusRegistryChanged(t *testing.T) {
	statusRepo(t, map[string]string{"work-items.yaml": items("one"), "people.yaml": "- a\n"})
	cfg, err := config.Load(config.Path())
	if err != nil {
		t.Fatal(err)
	}
	var code int
	err = source.ReadingFrom(&secondReadFails{Source: source.Worktree}, func() error {
		_, _, c, err := statusOf(cfg, "a", Out{})
		code = c
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "changed while read") || code != 0 {
		t.Errorf("the second read's failure is not the error: exit %d, %v", code, err)
	}
}

// Under a stealth config with no --as the session owns every item, which
// --json says; with nobody among the people only the unowned are listed,
// five of them and how many more, and a question names its item.
func TestStatusEveryAndNobody(t *testing.T) {
	statusRepo(t, map[string]string{
		"work-items.yaml": items("i1", "i2", "i3", "i4", "i5", "i6"),
		"people.yaml":     "[]\n",
		"asks.yaml":       "questions:\n  - id: q-1\n    item: i1\n    question: Which way?\n",
	})
	cfg, err := config.Load(config.Path())
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	o := Out{Stdout: &stdout, Stderr: &stderr}
	cfg.Stealth = true
	st, _, _, err := statusOf(cfg, "", o)
	if err != nil || !st.Every {
		t.Fatalf("a stealth config's session does not own every item: %v %+v", err, st)
	}
	var b strings.Builder
	if err := out.Emit(&b, st.fields()...); err != nil || !strings.Contains(b.String(), `"every_item": true`) {
		t.Errorf("--json does not say every_item: %v\n%s", err, b.String())
	}
	cfg.Stealth = false
	if st, _, _, err = statusOf(cfg, "", o); err != nil {
		t.Fatal(err)
	}
	st.print(&stdout)
	for _, want := range []string{"Working for nobody", "and 1 more: itos work", "q-1  Which way?  (i1)"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("the status does not say %q:\n%s", want, stdout.String())
		}
	}
}

// A person the identity provider names who is not among the people is
// said, and questions or drafts that cannot be read are each a line, the
// rest still printed.
func TestStatusUnread(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake gh is a shell script")
	}
	common := filepath.Join(".git", config.StealthFolder, draft.Folder, draft.FileName)
	statusRepo(t, map[string]string{
		"work-items.yaml": items("one"),
		"people.yaml":     "- a\n- b\n",
		"asks.yaml":       "questions: [\n",
		common:            "drafts: [\n",
		"bin/gh":          "#!/bin/sh\necho c\n",
	})
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Join(wd, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	code, stdout, stderr := run("status")
	if code != 0 {
		t.Fatalf("exit %d\n%s%s", code, stdout, stderr)
	}
	for _, want := range []string{"c is not in people.yaml", "The questions cannot be read", "The drafts cannot be read"} {
		if !strings.Contains(stdout+stderr, want) {
			t.Errorf("the status does not say %q:\n%s%s", want, stdout, stderr)
		}
	}
}
