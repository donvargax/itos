package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/draft"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
	"github.com/donvargax/itos/v8/internal/release"
	trees "github.com/donvargax/itos/v8/internal/store/tree/adapter"
	"github.com/donvargax/itos/v8/internal/store/tree/port"
	"github.com/donvargax/itos/v8/internal/vcs/port/porttest"
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
	cfg, err := loadConfig()
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
	port.Source
	reads int
}

func (s *secondReadFails) Read(ctx context.Context, path string) (string, error) {
	if path == "work-items.yaml" {
		if s.reads++; s.reads > 1 {
			return "", errors.New("changed while read")
		}
	}
	return s.Source.Read(ctx, path)
}

// A registry that cannot be loaded after its check is the error.
func TestStatusRegistryChanged(t *testing.T) {
	statusRepo(t, map[string]string{"work-items.yaml": items("one"), "people.yaml": "- a\n"})
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Source = &secondReadFails{Source: trees.Worktree}
	_, _, code, err := statusOf(cfg, "a", Out{})
	if err == nil || !strings.Contains(err.Error(), "changed while read") || code != 0 {
		t.Errorf("the second read's failure is not the error: exit %d, %v", code, err)
	}
}

// A remote that cannot be reached leaves the head as last fetched, said as
// such, its header read from the fetched commit; never fetched, neither the
// head nor its CI run is read.
func TestStatusRemoteUnreached(t *testing.T) {
	statusRepo(t, map[string]string{"work-items.yaml": items("one"), "people.yaml": "- a\n"})
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	fetched := strings.Repeat("f", 40)
	down := map[string]*porttest.Remote{"origin": {URL: "https://example.invalid/o/n", Down: true}}
	cfg.VCS = &porttest.VCS{History: []porttest.Commit{{Name: fetched, Message: "feat: fetched\n\nIts body.\n"}},
		Named: map[string]string{"refs/remotes/origin/main": fetched}, Remote: down}
	st := &standing{}
	st.readBranch(cfg, "origin", "refs/heads/main", "main", Out{})
	if h := st.Head; h == nil || h.Commit != fetched || !h.LastFetched || h.Header != "feat: fetched" ||
		len(st.Unread) == 0 || st.Unread[0] != "origin cannot be reached, so its main is as last fetched." {
		t.Errorf("last fetched: head %+v, unread %q", st.Head, st.Unread)
	}
	cfg.VCS = &porttest.VCS{Remote: down}
	st = &standing{}
	st.readBranch(cfg, "origin", "refs/heads/main", "main", Out{})
	want := "origin cannot be reached and origin/main was never fetched, so neither its head nor its CI run is read."
	if st.Head != nil || len(st.Unread) != 1 || st.Unread[0] != want {
		t.Errorf("never fetched: head %+v, unread %q", st.Head, st.Unread)
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
	cfg, err := loadConfig()
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

// visibility is "" for no provider, one that cannot look or cannot tell, and
// a look that fails.
func TestVisibilityOfAProvider(t *testing.T) {
	tells := providers.Watcher{Visibility: func() (string, error) { return "internal", nil }}
	fails := providers.Watcher{Visibility: func() (string, error) { return "public", errors.New("no network") }}
	for name, got := range map[string]string{
		"none":        visibility(tells, false, nil),
		"cannot look": visibility(tells, true, errors.New("no token")),
		"cannot tell": visibility(providers.Watcher{}, true, nil),
		"fails":       visibility(fails, true, nil),
	} {
		if got != "" {
			t.Errorf("%s: %q", name, got)
		}
	}
	if got := visibility(tells, true, nil); got != "internal" {
		t.Errorf("tells: %q", got)
	}
}

// readCI says why it did not read the run, for a provider that cannot look,
// none, and a look that fails, with no CI standing; a commit with no run yet
// is CI none, and a run found is its result, each said on the head's lines.
func TestStatusReadsCI(t *testing.T) {
	found := providers.Run{URL: "u", Status: "completed", Conclusion: "success"}
	for _, c := range []struct {
		name   string
		ok     bool
		err    error
		look   providers.Watch
		result string
		line   string
	}{
		{"cannot look", true, errors.New("no token"), nil, "", "CI cannot be read: no token"},
		{"none", false, nil, nil, "", "CI is not read: ci.watch.provider is none."},
		{"a look that fails", true, nil, func(string) (providers.Run, bool, error) { return providers.Run{}, false, errors.New("502") }, "", "CI cannot be read: 502"},
		{"no run yet", true, nil, func(string) (providers.Run, bool, error) { return providers.Run{}, false, nil }, "none", "CI: no run of abcdef1 yet"},
		{"a run", true, nil, func(string) (providers.Run, bool, error) { return found, true, nil }, "success", "CI: success, u"},
	} {
		st := &standing{}
		st.readCI(providers.Watcher{Look: c.look}, c.ok, c.err, "abcdef1234567")
		result := ""
		if st.CI != nil {
			result = st.CI.Result
		}
		unread := c.result == ""
		if result != c.result || len(st.start) != 1 || st.start[0] != c.line || (len(st.Unread) == 1) != unread {
			t.Errorf("%s: CI %+v, lines %q, unread %q", c.name, st.CI, st.start, st.Unread)
		}
	}
}
