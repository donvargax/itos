package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/providers"
)

// The ledger and registry of closeRepo, before work done.
const (
	closeLedger   = "- id: T-1\n  type: docs\n  title: One\n  why: Because.\n  done_when:\n    - run: \"true\"\n"
	closeRegistry = "phases: { 1: null }\nitems:\n  - { id: T-1, title: One, phase: 1, owner: null, status: doing, depends_on: [] }\n"
)

// A scratch repository, as rollbackRepo makes one, whose ledger has the task
// T-1 with a check that passes, its item doing, and no remote and no CI to
// ask: work done of T-1 closes it.
func closeRepo(t *testing.T) {
	t.Helper()
	config := "version: 1\nledger: { files: \"tasks/phase-{group}.yaml\", id: \"T-\\\\d+\" }\nci: { watch: { provider: none } }\n"
	dir := gitConfigRepo(t, config)
	standInHooks(t)
	for name, value := range map[string]string{
		"GIT_AUTHOR_NAME": "A", "GIT_AUTHOR_EMAIL": "a@example.com",
		"GIT_COMMITTER_NAME": "A", "GIT_COMMITTER_EMAIL": "a@example.com",
	} {
		t.Setenv(name, value)
	}
	if err := os.MkdirAll(filepath.Join(dir, "tasks"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"tasks/phase-1.yaml": closeLedger, "tasks/work-items.yaml": closeRegistry} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitIn(t, "add", "--", "itos.yaml", "tasks/phase-1.yaml", "tasks/work-items.yaml")
	gitIn(t, "commit", "-q", "-m", "chore: start")
}

// The close commit takes the task's checks out of its ledger entry, its why
// kept, and its body names the commit before it, which still holds them
// (slice 101).
func TestWorkDoneTakesTheChecksOut(t *testing.T) {
	closeRepo(t)
	before := strings.TrimSpace(gitIn(t, "rev-parse", "--short", "HEAD"))
	var stdout, stderr strings.Builder
	code, err := workDone([]string{"T-1"}, Out{Stdout: &stdout, Stderr: &stderr})
	if code != 0 || err != nil {
		t.Fatalf("exit %d, %v\n%s%s", code, err, stdout.String(), stderr.String())
	}
	if text, _ := os.ReadFile("tasks/phase-1.yaml"); string(text) != "- id: T-1\n  type: docs\n  title: One\n  why: Because.\n" {
		t.Errorf("the ledger reads %q", text)
	}
	if files := gitIn(t, "show", "--format=", "--name-only", "HEAD"); files != "tasks/phase-1.yaml\ntasks/work-items.yaml\n" {
		t.Errorf("the close commit touches:\n%s", files)
	}
	body := gitIn(t, "log", "-1", "--format=%B")
	for _, want := range []string{"docs: close T-1", "Its checks are taken out of tasks/phase-1.yaml", "The commit before this one, " + before + ", still holds them."} {
		if !strings.Contains(strings.Join(strings.Fields(body), " "), want) {
			t.Errorf("the body lacks %q:\n%s", want, body)
		}
	}
	if !strings.Contains(stdout.String(), "; its checks are out of tasks/phase-1.yaml") {
		t.Errorf("stdout:\n%s", stdout.String())
	}
}

// A close commit a hook refuses leaves the ledger and the registry as they
// were, and nothing staged.
func TestWorkDoneRefusedKeepsTheChecks(t *testing.T) {
	closeRepo(t)
	refusingHook(t)
	var stdout, stderr strings.Builder
	code, err := workDone([]string{"T-1"}, Out{Stdout: &stdout, Stderr: &stderr})
	if code != ExitPolicy || err != nil {
		t.Fatalf("exit %d, %v\n%s%s", code, err, stdout.String(), stderr.String())
	}
	for name, want := range map[string]string{"tasks/phase-1.yaml": closeLedger, "tasks/work-items.yaml": closeRegistry} {
		if text, _ := os.ReadFile(name); string(text) != want {
			t.Errorf("%s reads %q", name, text)
		}
	}
	if status := gitIn(t, "status", "--porcelain"); status != "" {
		t.Errorf("git status reports:\n%s", status)
	}
}

// closeRepo with a Task: footer linking commits to the ledger's tasks,
// committed.
func closeRepoLinked(t *testing.T) {
	t.Helper()
	closeRepo(t)
	text, err := os.ReadFile("itos.yaml")
	if err != nil {
		t.Fatal(err)
	}
	footers := "commits:\n  footers:\n    Task:\n      source: ledger\n      read_at: commit\n"
	if err := os.WriteFile("itos.yaml", append(text, footers...), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, "commit", "-q", "-m", "chore: link tasks", "--", "itos.yaml")
}

// closeRepoLinked with a remote, origin, a bare repository holding its
// commits.
func closeRepoWithRemote(t *testing.T) {
	t.Helper()
	closeRepoLinked(t)
	remote := filepath.Join(t.TempDir(), "origin.git")
	gitIn(t, "init", "-q", "--bare", remote)
	gitIn(t, "remote", "add", "origin", remote)
	gitIn(t, "push", "-q", "origin", "HEAD")
}

// An empty commit with the message.
func commitEmpty(t *testing.T, message string) string {
	t.Helper()
	gitIn(t, "commit", "-q", "--allow-empty", "-m", message)
	return gitLine(t, "rev-parse", "HEAD")
}

// The item's own commits are those whose footers name it (slice 127), the
// newest judged; an item with none has no such commit.
func TestOwnCommitIsTheNewestThatNamesTheItem(t *testing.T) {
	closeRepoLinked(t)
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if j := ownCommit(cfg, "T-1", nil); j.sha != "" || j.own {
		t.Fatalf("an item with no commit of its own judges %+v", j)
	}
	own := commitEmpty(t, "chore: one\n\nTask: T-1\n")
	commitEmpty(t, "chore: two\n\nTask: T-2\n")
	j := ownCommit(cfg, "T-1", nil)
	short := gitLine(t, "rev-parse", "--short", own)
	if j.sha != own || !j.own || j.what != short+", the newest commit of T-1" || j.run != "The CI run of "+short+", the newest commit of T-1," {
		t.Fatalf("T-1's commit %s is judged as %+v", own, j)
	}
}

// work done passes over a commit no remote has that is not the item's own,
// and refuses while one of the item's own is on no remote (slice 127).
func TestWorkDoneJudgesTheItemsOwnCommits(t *testing.T) {
	closeRepoWithRemote(t)
	commitEmpty(t, "chore: build T-1\n\nTask: T-1\n")
	gitIn(t, "push", "-q", "origin", "HEAD")
	commitEmpty(t, "chore: something else")
	var stdout, stderr strings.Builder
	code, err := workDone([]string{"T-1"}, Out{Stdout: &stdout, Stderr: &stderr})
	if code != 0 || err != nil {
		t.Fatalf("exit %d, %v\n%s%s", code, err, stdout.String(), stderr.String())
	}

	closeRepoWithRemote(t)
	own := commitEmpty(t, "chore: build T-1\n\nTask: T-1\n")
	stdout.Reset()
	stderr.Reset()
	code, err = workDone([]string{"T-1"}, Out{Stdout: &stdout, Stderr: &stderr})
	want := "The history up to " + gitLine(t, "rev-parse", "--short", own) + ", the newest commit of T-1, has a commit no remote has"
	if code != ExitPolicy || err != nil || !strings.Contains(stderr.String(), want) {
		t.Fatalf("exit %d, %v\n%s%s", code, err, stdout.String(), stderr.String())
	}
}

// A run that passed and covers the item's newest commit judges it (slice
// 127): its own, else one on the branch whose head has it, a failed one
// passed over; none for a commit that is not the item's own, or when no run
// passed. Under --json the run's end goes to stderr.
func TestPassedRunIsAnyCoveringRunThatPassed(t *testing.T) {
	failed := providers.Run{URL: "f", Status: "completed", Conclusion: "failure", HeadSHA: "abc"}
	passed := providers.Run{URL: "p", Status: "completed", Conclusion: "success", HeadSHA: "abc"}
	look := func(run providers.Run) providers.Watch {
		return func(string) (providers.Run, bool, error) { return run, true, nil }
	}
	runs := func(listed ...providers.Run) providers.Runs {
		return func(string) ([]providers.Run, error) { return listed, nil }
	}
	own := doneCommit{sha: "abc", own: true}
	var stdout, stderr strings.Builder
	o := Out{Stdout: &stdout, Stderr: &stderr}
	if _, ok := passedRun(providers.Watcher{Look: look(passed)}, doneCommit{sha: "abc"}, "", "main", o); ok {
		t.Fatal("a commit that is not the item's own was judged by a run that passed")
	}
	if w, ok := passedRun(providers.Watcher{Look: look(passed)}, own, "", "main", o); !ok || w.code != 0 || w.run.URL != "p" || !strings.Contains(stdout.String(), "CI passed: p") {
		t.Fatalf("its own run that passed: %v %+v\n%s", ok, w, stdout.String())
	}
	if w, ok := passedRun(providers.Watcher{Look: look(failed), Runs: runs(failed, passed)}, own, "", "main", o); !ok || w.run.URL != "p" {
		t.Fatalf("a run on the branch that passed: %v %+v", ok, w)
	}
	if _, ok := passedRun(providers.Watcher{Look: look(failed), Runs: runs(failed)}, own, "", "main", o); ok {
		t.Fatal("no run passed, yet one judged the commit")
	}
	stdout.Reset()
	if _, ok := passedRun(providers.Watcher{Look: look(passed)}, own, "", "main", Out{Stdout: &stdout, Stderr: &stderr, JSON: true}); !ok || stdout.Len() > 0 || !strings.Contains(stderr.String(), "CI passed: p") {
		t.Fatalf("under --json: %v\n%s%s", ok, stdout.String(), stderr.String())
	}
}

// work done asks GitHub for the item's newest commit's run (slice 127): one
// that passed lands it; one that failed, with none on the branch that
// passed, is waited for as before and refuses, naming it.
func TestLandedCIJudgesTheItemsOwnRun(t *testing.T) {
	gitConfigRepo(t, "version: 1\nci:\n  watch:\n    provider: github\n    interval: 0\n    github:\n      workflow: ci.yml\n")
	t.Setenv("GIT_AUTHOR_NAME", "itos")
	t.Setenv("GIT_COMMITTER_NAME", "itos")
	t.Setenv("GIT_AUTHOR_EMAIL", "itos@example.com")
	t.Setenv("GIT_COMMITTER_EMAIL", "itos@example.com")
	sha := commitEmpty(t, "chore: build slice-9")
	conclusion := "success"
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(req.URL.Path, "/jobs") {
			fmt.Fprint(rw, `{"jobs":[]}`)
			return
		}
		fmt.Fprintf(rw, `{"workflow_runs":[{"id":1,"html_url":"u","status":"completed","conclusion":%q,"head_sha":%q,"head_branch":"main","created_at":"1"}]}`, conclusion, sha)
	}))
	t.Cleanup(server.Close)
	t.Setenv("GITHUB_API_URL", server.URL)
	t.Setenv("GITHUB_REPOSITORY", "owner/scratch")
	t.Setenv("GITHUB_TOKEN", "token")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	j := doneCommit{sha: sha, run: "The CI run of it,", own: true}
	var stdout, stderr strings.Builder
	if w, code, err := landedCI(cfg, "slice-9", j, Out{Stdout: &stdout, Stderr: &stderr}); code != 0 || err != nil || w.run == nil || w.run.URL != "u" {
		t.Fatalf("exit %d, %v, %+v\n%s%s", code, err, w, stdout.String(), stderr.String())
	}
	conclusion = "failure"
	stdout.Reset()
	stderr.Reset()
	if _, code, err := landedCI(cfg, "slice-9", j, Out{Stdout: &stdout, Stderr: &stderr}); code != ExitPolicy || err != nil || !strings.Contains(stderr.String(), "The CI run of it, did not pass, so slice-9 is not done: u") {
		t.Fatalf("exit %d, %v\n%s%s", code, err, stdout.String(), stderr.String())
	}
}

// The run that judged a commit is named as the commit's own, unless a head
// that has the commit ran it: then that head's run is named, with the
// commit. No run, a run with no head, the commit's own run and a commit
// with nothing to name it by are the commit's own.
func TestRunOf(t *testing.T) {
	sha, other := strings.Repeat("a", 40), strings.Repeat("b", 40)
	j := doneCommit{sha: sha, run: "HEAD's CI run", what: "HEAD"}
	for _, c := range []struct {
		j    doneCommit
		run  *providers.Run
		want string
	}{
		{j, nil, "HEAD's CI run"},
		{j, &providers.Run{}, "HEAD's CI run"},
		{j, &providers.Run{HeadSHA: sha}, "HEAD's CI run"},
		{doneCommit{sha: sha, run: "HEAD's CI run"}, &providers.Run{HeadSHA: other}, "HEAD's CI run"},
		{j, &providers.Run{HeadSHA: other}, "The CI run of bbbbbbb, which has HEAD,"},
	} {
		if got := c.j.runOf(c.run); got != c.want {
			t.Errorf("%+v.runOf(%+v) = %q, want %q", c.j, c.run, got, c.want)
		}
	}
}
