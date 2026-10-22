package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/draft"
	"github.com/donvargax/itos/v8/internal/release"
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
	if err := os.WriteFile("work-items.yaml", []byte(items("one")), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := run("status", "--as", "zed")
	if code != ExitMissing || !strings.Contains(stderr, `"zed" is not in people.yaml`) {
		t.Errorf("--as not among the people: exit %d\n%s", code, stderr)
	}
	if err := os.Remove("work-items.yaml"); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = run("status", "--as", "a")
	if code != ExitPolicy || !strings.Contains(stderr, "no work registry at work-items.yaml") {
		t.Errorf("no registry: exit %d\n%s", code, stderr)
	}
}

// Where git cannot name the common dir the drafts are not read, not even
// to say they cannot be, and the rest is still read.
func TestStatusWithNoDraftsFile(t *testing.T) {
	statusRepo(t, map[string]string{"work-items.yaml": items("one"), "people.yaml": "- a\n",
		filepath.Join(".git", config.StealthFolder, draft.Folder, draft.FileName): "drafts: [\n"})
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	useWrappedGit(t, "rev-parse", "", "")
	st, found, err := goPorts{o: Out{Stderr: io.Discard, OS: runtime.GOOS}}.Standing(t.Context(), cfg)
	if err != nil || found != nil || st.Drafts == nil || len(st.Drafts) != 0 || len(st.Next) != 1 ||
		strings.Contains(strings.Join(st.Unread, "\n"), "drafts") {
		t.Errorf("no drafts file: %v %v, %+v", err, found, st)
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

// The head's line names the repository's visibility as ci.watch's provider
// tells it, and --json's head holds it; a provider that cannot tell it says
// nothing of it.
func TestStatusNamesTheVisibility(t *testing.T) {
	visibility := "public"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/o/n" && visibility != "":
			w.Write([]byte(`{"visibility":"` + visibility + `"}`))
		case strings.HasSuffix(r.URL.Path, "/runs"):
			w.Write([]byte(`{"workflow_runs":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	statusRepo(t, map[string]string{
		"itos.yaml": "version: 1\nwork:\n  registry: work-items.yaml\n  people: { source: yaml, file: people.yaml }\n" +
			"ci:\n  steps: []\n  watch: { provider: github, interval: 0 }\n",
		"work-items.yaml": items("one"),
		"people.yaml":     "- a\n",
	})
	for name, value := range map[string]string{"GITHUB_API_URL": server.URL, "GITHUB_REPOSITORY": "o/n", "GITHUB_TOKEN": "t"} {
		t.Setenv(name, value)
	}
	gitIn(t, "add", ".")
	gitIn(t, "-c", "user.name=a", "-c", "user.email=a@example.com", "commit", "-q", "-m", "chore: start")
	remote := filepath.Join(t.TempDir(), "origin.git")
	gitIn(t, "init", "-q", "--bare", remote)
	gitIn(t, "remote", "add", "origin", remote)
	gitIn(t, "push", "-q", "-u", "origin", "HEAD:main")
	gitIn(t, "branch", "-q", "-m", "main")
	gitIn(t, "branch", "-q", "-u", "origin/main")
	if code, stdout, stderr := run("status", "--as", "a"); code != 0 || !strings.Contains(stdout, "origin/main (public): ") {
		t.Fatalf("exit %d\n%s%s", code, stdout, stderr)
	}
	visibility = "private"
	code, stdout, stderr := run("status", "--as", "a", "--json")
	var st struct {
		Head map[string]any `json:"head"`
	}
	if err := json.Unmarshal([]byte(stdout), &st); code != 0 || err != nil || st.Head["visibility"] != "private" {
		t.Fatalf("exit %d, %v\n%s%s", code, err, stdout, stderr)
	}
	visibility = ""
	code, stdout, stderr = run("status", "--as", "a", "--json")
	st.Head = nil
	if err := json.Unmarshal([]byte(stdout), &st); code != 0 || err != nil || st.Head == nil || st.Head["visibility"] != nil {
		t.Fatalf("exit %d, %v\n%s%s", code, err, stdout, stderr)
	}
	if code, stdout, stderr := run("status", "--as", "a"); code != 0 || !strings.Contains(stdout, "origin/main: ") {
		t.Fatalf("exit %d\n%s%s", code, stdout, stderr)
	}
}
