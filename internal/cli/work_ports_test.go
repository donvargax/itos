package cli

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/store/tracker/adapter"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// workPorts bind the work slice to the real code: the config the working
// tree has, the working tree, work.identity's provider and the tracker of
// the config on the OS itos runs on; runWork reads the line and runs the
// slice over them.
func TestWorkPortsBindTheRealCode(t *testing.T) {
	gitConfigRepo(t, "version: 1\nwork:\n  registry: work-items.yaml\n  identity: { provider: none, hint: say --as }\n")
	p := workPorts{Out{OS: runtime.GOOS}}
	cfg, err := p.Config(t.Context())
	if err != nil || cfg.Work.Registry != "work-items.yaml" {
		t.Fatalf("Config = %+v, %v, want the working tree's config", cfg, err)
	}
	if tree := p.Worktree(t.Context()); !tree.Has(t.Context(), "itos.yaml") || tree.Tree(t.Context()) != "worktree" {
		t.Errorf("Worktree = %#v, want the working tree", tree)
	}
	if answer := p.Identity(t.Context(), cfg)(); !strings.Contains(answer.Problem, "say --as") {
		t.Errorf("Identity() = %+v, want work.identity none's", answer)
	}
	files, ok := p.Tracker(t.Context(), cfg).(adapter.Files)
	if !ok || files.OS != runtime.GOOS || files.Lock != "work-items.yaml" || files.Stealth {
		t.Errorf("Tracker = %#v, want the files adapter of the config on this OS", files)
	}
	if err := os.WriteFile("work-items.yaml", []byte("phases: { 1: null }\nitems:\n  - { id: one, title: One, phase: 1, owner: null, status: todo, depends_on: [] }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	if code, err := runWork([]string{"list"}, Out{Stdout: &stdout, Stderr: &stderr, OS: runtime.GOOS}); code != 0 || err != nil ||
		!strings.Contains(stdout.String(), "one") || !strings.Contains(stdout.String(), "One") {
		t.Errorf("runWork list = %d, %v, stdout %q, stderr %q", code, err, stdout.String(), stderr.String())
	}
	if code, err := runWork([]string{"show", "a", "b"}, Out{}); code != 0 || err == nil || err.Error() != "work show needs one <id>" {
		t.Errorf("runWork show a b = %d, %v, want the usage error alone", code, err)
	}
	if code, err := runWork([]string{"done"}, Out{}); code != 0 || err == nil || err.Error() != "work done needs one <id>" {
		t.Errorf("runWork done = %d, %v, want the usage error alone", code, err)
	}
	stdout.Reset()
	stderr.Reset()
	if code, err := runWork([]string{"done", "nope"}, Out{Stdout: &stdout, Stderr: &stderr, OS: runtime.GOOS}); code != ExitPolicy || err != nil ||
		!strings.Contains(stderr.String(), "nope") {
		t.Errorf("runWork done nope = %d, %v, stdout %q, stderr %q", code, err, stdout.String(), stderr.String())
	}
}

// A task's promotion writes the ledger file, the registry and the questions
// through the tracker of the config, committed together, the id minted from
// the repository's counter past the ledger's and the registry's.
func TestWorkPortsPromoteThroughTheRealCode(t *testing.T) {
	dir := registryWriterRepo(t, "phases: { 1: null }\nitems:\n"+
		"  - { id: p1-idea, title: Idea, phase: 1, status: todo, kind: idea, why: Specified }\n"+
		"  - { id: T-001, title: Existing task, phase: 1, owner: null, status: done, kind: task, depends_on: [] }\n")
	writeFiles(t, map[string]string{
		"tasks/phase-1.yaml": "- { id: T-001, type: chore, title: Existing task }\n",
		"asks.yaml":          "questions:\n  - { id: q-1, item: p1-idea, question: \"Why?\" }\n",
	})
	gitIn(t, "add", "--", "tasks", "asks.yaml")
	gitIn(t, "commit", "-q", "-m", "docs: a ledger")
	var stdout, stderr strings.Builder
	code, err := runWork([]string{"promote", "p1-idea", "--kind", "task", "--type", "build", "--check", "true"}, Out{Stdout: &stdout, Stderr: &stderr, OS: runtime.GOOS})
	if err != nil || code != 0 || stdout.String() != "p1-idea is the task T-002 of tasks/phase-1.yaml, named now by q-1: committed "+
		strings.TrimSpace(gitIn(t, "rev-parse", "--short=7", "HEAD"))+" docs: promote p1-idea to T-002\n" {
		t.Fatalf("runWork promote = (%d, %v), stdout=%q stderr=%q", code, err, stdout.String(), stderr.String())
	}
	if text, err := os.ReadFile(filepath.Join(dir, "tasks", "phase-1.yaml")); err != nil ||
		!strings.HasSuffix(string(text), "- id: T-002\n  type: build\n  title: Idea\n  why: >\n    Specified\n  done_when:\n    - run: \"true\"\n") {
		t.Errorf("ledger after the promote = %q (%v)", text, err)
	}
	if files := strings.Fields(gitIn(t, "show", "--name-only", "--format=", "HEAD")); strings.Join(files, " ") != "asks.yaml registry.yaml tasks/phase-1.yaml" {
		t.Errorf("the promote committed %q, want the questions, the registry and the ledger file", files)
	}
	if body := strings.Join(strings.Fields(gitIn(t, "log", "-1", "--format=%b")), " "); !strings.HasSuffix(body,
		"with itos work promote, and write its ledger entry, with its why and checks, in tasks/phase-1.yaml, and rename it in q-1 of asks.yaml.") {
		t.Errorf("the promote's body is %q", body)
	}
	if status := gitIn(t, "status", "--porcelain"); status != "" {
		t.Errorf("the promote left %q", status)
	}
}

// draft reads a work promote line with the slice's PromoteLine.
func TestWorkPromoteLineIsTheSlices(t *testing.T) {
	if p, err := workPromoteLine([]string{"a", "--kind", "slice"}); err != nil || p.Kind != "slice" || p.ID != "a" {
		t.Errorf("workPromoteLine = %+v, %v", p, err)
	}
	if _, err := workPromoteLine([]string{"a"}); err == nil || err.Error() != "work promote needs --kind slice|task" {
		t.Errorf("workPromoteLine a = %v", err)
	}
}

// A write the slice answers holds, reads and commits the registry through
// the tracker of the config, and ItemID is newItemID: the id a draft
// reserved, else one minted from the repository's counter.
func TestWorkPortsWriteThroughTheRealCode(t *testing.T) {
	registryWriterRepo(t, "phases: { 1: null }\nitems:\n  - { id: slice-9, title: Nine, phase: 1, owner: null, status: todo, depends_on: [] }\n")
	var stdout, stderr strings.Builder
	if code, err := runWork([]string{"take", "slice-9"}, Out{Stdout: &stdout, Stderr: &stderr, OS: runtime.GOOS}); code != 0 || err != nil ||
		!strings.HasPrefix(stdout.String(), "slice-9 is in progress with no owner recorded: committed ") {
		t.Errorf("runWork take slice-9 = %d, %v, stdout %q, stderr %q", code, err, stdout.String(), stderr.String())
	}
	if header := strings.TrimSpace(gitIn(t, "log", "-1", "--format=%s")); header != "docs: take slice-9" {
		t.Errorf("the take committed %q", header)
	}
	cfg, err := loadConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if id, err := (workPorts{Out{OS: runtime.GOOS, reserved: "slice-77"}}).ItemID(t.Context(), cfg, "slice", []string{"slice-9"}); id != "slice-77" || err != nil {
		t.Errorf("ItemID with a reserved id = %q, %v", id, err)
	}
	if id, err := (workPorts{Out{OS: runtime.GOOS}}).ItemID(t.Context(), cfg, "slice", []string{"slice-9"}); id != "slice-10" || err != nil {
		t.Errorf("ItemID minted = %q, %v", id, err)
	}
}

// draft reads a work add line with the slice's AddLine.
func TestWorkAddLineIsTheSlices(t *testing.T) {
	if n, err := workAddLine([]string{"--kind", "bug", "--title", "T", "--why", "W"}); err != nil || n.Kind != "bug" || n.ID != "" {
		t.Errorf("workAddLine = %+v, %v", n, err)
	}
	if _, err := workAddLine([]string{"a", "b"}); err == nil || err.Error() != "work add takes at most one <id>" {
		t.Errorf("workAddLine a b = %v", err)
	}
}

// work done runs a task's static checks through TaskChecks, judge.StaticFailure
// in the commit-msg hook's environment, which has no GIT_INDEX_FILE: every
// static check in written order, one below a late check included, the late one
// never run, the first failure's output on stderr and described. No check opts
// in to evidence, so no engine is made.
func TestWorkPortsRunATasksStaticChecksAsTheHookDoes(t *testing.T) {
	closeRepo(t)
	putFile(t, filepath.Join("tasks", "phase-1.yaml"), "- id: T-1\n  type: docs\n  title: One\n  why: Because.\n  done_when:\n"+
		"    - { run: 'test -z \"$GIT_INDEX_FILE\"', cost: static }\n    - { run: \"exit 7\", cost: late }\n    - { run: \"false\", cost: static }\n")
	t.Setenv("GIT_INDEX_FILE", "elsewhere")
	cfg, err := loadConfig("")
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := ledger.Tasks(cfg)
	if err != nil || len(tasks) != 1 {
		t.Fatalf("the ledger's tasks: %v, %v", tasks, err)
	}
	var stderr strings.Builder
	checked, err := workPorts{}.TaskChecks(t.Context(), cfg, tasks[0], &stderr)
	f := checked.Failure
	if err != nil || f == nil || f.Task != "T-1" || f.Command != "false" || f.Why != "`false` exited 1" || f.Code != 1 || f.TimedOut ||
		checked.Evidence != nil || stderr.String() != "T-1 One\n  $ false\n" {
		t.Errorf("TaskChecks = %+v, %v, stderr %q", checked, err, stderr.String())
	}
}

// An opted-in check is looked up by HEAD's tree, never the index's: a staged
// change and an unstaged edit reach neither its run nor its key, nothing is
// said of them, the miss is recorded, the evidence is given, and a second run
// answers from the record.
func TestWorkPortsKeepOptedInChecksOnHEADsTree(t *testing.T) {
	closeRepo(t)
	putFile(t, filepath.Join("tasks", "phase-1.yaml"), "- id: T-1\n  type: docs\n  title: One\n  why: Because.\n  done_when:\n"+
		"    - { run: \"cat input; exit 3\", cost: static, evidence: {} }\n")
	putFile(t, "input", "committed\n")
	gitIn(t, "add", "--", "tasks/phase-1.yaml", "input")
	gitIn(t, "commit", "-q", "-m", "chore: add the check")
	head := strings.TrimSpace(gitIn(t, "rev-parse", "HEAD^{tree}"))
	putFile(t, "input", "staged\n")
	gitIn(t, "add", "--", "input")
	putFile(t, "input", "edited\n")
	cfg, err := loadConfig("")
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := ledger.Tasks(cfg)
	if err != nil || len(tasks) != 1 {
		t.Fatalf("the ledger's tasks: %v, %v", tasks, err)
	}
	ports := workPorts{Out{OS: runtime.GOOS}}
	var stderr strings.Builder
	checked, err := ports.TaskChecks(t.Context(), cfg, tasks[0], &stderr)
	f := checked.Failure
	if err != nil || f == nil || f.Code != 3 || f.Output != "committed\n" || len(checked.Evidence) != 1 {
		t.Fatalf("TaskChecks = %+v, %v, stderr %q", checked, err, stderr.String())
	}
	first := checked.Evidence[0]
	if first.Command != "cat input; exit 3" || first.Tree != head || !first.Recorded || first.Hit || first.Reason != "" || strings.Contains(stderr.String(), "not staged") {
		t.Errorf("the miss: %+v, stderr %q", first, stderr.String())
	}
	stderr.Reset()
	again, err := ports.TaskChecks(t.Context(), cfg, tasks[0], &stderr)
	if err != nil || again.Failure == nil || again.Failure.Output != "committed\n" || len(again.Evidence) != 1 || !again.Evidence[0].Hit || again.Evidence[0].Key != first.Key {
		t.Errorf("the hit: %+v, %v", again, err)
	}
}

// work done's view of the repository is HEAD's: its tree stands where the
// index's does, its paths are the tracked ones (a path only the index holds is
// not), and no change is unstaged.
func TestDoneGitReadsHEADsTree(t *testing.T) {
	closeRepo(t)
	putFile(t, "input", "committed\n")
	gitIn(t, "add", "--", "input")
	gitIn(t, "commit", "-q", "-m", "chore: add the input")
	putFile(t, "staged", "new\n")
	gitIn(t, "add", "--", "staged")
	putFile(t, "input", "edited\n")
	g := doneGit{evidenceGit{vcs.Git{Dir: "."}}}
	tree, err := g.IndexTree(t.Context())
	if want := strings.TrimSpace(gitIn(t, "rev-parse", "HEAD^{tree}")); err != nil || tree != want {
		t.Errorf("IndexTree = %q, %v, not HEAD's %q", tree, err, want)
	}
	paths, err := g.IndexPaths(t.Context())
	if err != nil || !slices.Contains(paths, "input") || slices.Contains(paths, "staged") {
		t.Errorf("IndexPaths = %v, %v", paths, err)
	}
	if unstaged, err := g.UnstagedPaths(t.Context()); err != nil || unstaged != nil {
		t.Errorf("UnstagedPaths = %v, %v", unstaged, err)
	}
}

// work done asks for a commit's run through Watcher, ci.watch's provider
// for the config, here GitHub's at the address the environment gives.
func TestWorkPortsWatchThroughTheConfigsProvider(t *testing.T) {
	gitConfigRepo(t, "version: 1\nci:\n  watch:\n    provider: github\n    interval: 0\n    github:\n      workflow: ci.yml\n")
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(req.URL.Path, "/jobs") {
			fmt.Fprint(rw, `{"jobs":[]}`)
			return
		}
		fmt.Fprint(rw, `{"workflow_runs":[{"id":1,"html_url":"u","status":"completed","conclusion":"success","head_sha":"abc","head_branch":"main","created_at":"1"}]}`)
	}))
	t.Cleanup(server.Close)
	t.Setenv("GITHUB_API_URL", server.URL)
	t.Setenv("GITHUB_REPOSITORY", "owner/scratch")
	t.Setenv("GITHUB_TOKEN", "token")
	cfg, err := loadConfig("")
	if err != nil {
		t.Fatal(err)
	}
	wr, ok, err := workPorts{}.Watcher(t.Context(), cfg, "origin")
	if err != nil || !ok {
		t.Fatalf("Watcher = %v, %v", ok, err)
	}
	if run, found, err := wr.Look("abc"); err != nil || !found || run.URL != "u" || run.Conclusion != "success" {
		t.Errorf("Look = %+v, %v, %v", run, found, err)
	}
}

// Under push.mode pr work done's remote is asked its HEAD, from the folder
// itos runs in; its gh is the one on the PATH, and its GitHub repository the
// variable the config names for the watch's, else the remote's URL.
func TestWorkPortsReachTheRemoteAndGh(t *testing.T) {
	gitConfigRepo(t, "version: 1\n")
	remote := t.TempDir()
	gitLine(t, "init", "-q", "--bare", "-b", "main", remote)
	gitLine(t, "remote", "add", "origin", remote)
	gitLine(t, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "--allow-empty", "-q", "-m", "x")
	gitLine(t, "push", "-q", "origin", "HEAD:refs/heads/main")
	p := workPorts{Out{OS: runtime.GOOS}}
	if got, err := p.RemoteHead(t.Context(), "origin", vcsport.Talk{NoPrompt: true}); got != "main" || err != nil {
		t.Errorf("RemoteHead = %q, %v, want main", got, err)
	}
	var failed *vcsport.Failed
	if _, err := p.RemoteHead(t.Context(), "nowhere", vcsport.Talk{NoPrompt: true}); !errors.As(err, &failed) {
		t.Errorf("RemoteHead of no remote = %v, want a failed git", err)
	}

	cfg, err := p.Config(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	gitLine(t, "remote", "set-url", "origin", "git@github.com:o/r.git")
	t.Setenv("GITHUB_REPOSITORY", "")
	if got := p.GitHubRepository(t.Context(), cfg, "origin"); got != "o/r" {
		t.Errorf("GitHubRepository = %q, want the remote's", got)
	}
	t.Setenv("GITHUB_REPOSITORY", "x/y")
	if got := p.GitHubRepository(t.Context(), cfg, "origin"); got != "x/y" {
		t.Errorf("GitHubRepository = %q, want the environment's", got)
	}
	if runtime.GOOS == "windows" {
		t.Skip("the gh is a shell script")
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte("#!/bin/sh\nprintf '%s' \"$*\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if out, err := p.Gh(t.Context(), "pr", "list"); out != "pr list" || err != nil {
		t.Errorf("Gh = %q, %v", out, err)
	}
}
