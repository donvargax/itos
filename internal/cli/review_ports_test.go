package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// reviewPorts bind the review slice to the real code: the config file at its
// path in the working tree and its load, git's resolve, the branch's
// upstream's remote else origin, the GitHub repository the environment or
// the remote names, the files of the folder itos runs in, and gh; runReview
// reads the line and runs the slice over them.
func TestReviewPortsBindTheRealCode(t *testing.T) {
	dir := gitConfigRepo(t, "version: 1\nwork: { registry: work-items.yaml }\n")
	gitLine(t, "remote", "add", "origin", "git@github.com:o/r.git")
	p := reviewPorts{o: Out{Dir: dir}, git: vcs.Git{Dir: dir}}
	if !p.ConfigFound(t.Context()) {
		t.Error("ConfigFound false with the working tree's config")
	}
	cfg, err := p.LoadConfig(t.Context())
	if err != nil || cfg.Work.Registry != "work-items.yaml" {
		t.Fatalf("LoadConfig = %+v, %v, want the working tree's config", cfg, err)
	}
	if p.Remote(t.Context()) != "origin" {
		t.Errorf("Remote = %q on a branch with no upstream, want origin", p.Remote(t.Context()))
	}
	if _, err := p.Resolve(t.Context(), "HEAD"); err == nil {
		t.Error("Resolve HEAD with no commit")
	}
	writeFiles(t, map[string]string{"a.txt": "a\n"})
	gitLine(t, "add", "a.txt")
	gitLine(t, "-c", "user.name=t", "-c", "user.email=t@example", "commit", "-q", "-m", "a")
	sha, err := p.Resolve(t.Context(), "HEAD")
	if err != nil || !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(sha) {
		t.Errorf("Resolve HEAD = %q, %v, want the commit's full SHA", sha, err)
	}
	branch := gitLine(t, "rev-parse", "--abbrev-ref", "HEAD")
	gitLine(t, "config", "branch."+branch+".remote", "up")
	gitLine(t, "config", "branch."+branch+".merge", "refs/heads/"+branch)
	if p.Remote(t.Context()) != "up" {
		t.Errorf("Remote = %q, want the branch's upstream's", p.Remote(t.Context()))
	}
	gitLine(t, "checkout", "-q", "--detach")
	if p.Remote(t.Context()) != "origin" {
		t.Errorf("Remote = %q on a detached HEAD, want origin", p.Remote(t.Context()))
	}
	t.Setenv("GITHUB_REPOSITORY", "")
	if got := p.GitHubRepository(t.Context(), cfg, "origin"); got != "o/r" {
		t.Errorf("GitHubRepository = %q, want the remote's", got)
	}
	t.Setenv("GITHUB_REPOSITORY", "x/y")
	if got := p.GitHubRepository(t.Context(), cfg, "nowhere"); got != "x/y" {
		t.Errorf("GitHubRepository = %q, want the environment's", got)
	}
	if text, err := p.ReadFile(t.Context(), "a.txt"); err != nil || string(text) != "a\n" {
		t.Errorf("ReadFile = %q, %v", text, err)
	}
	if _, err := p.ReadFile(t.Context(), "missing.md"); err == nil || !strings.Contains(err.Error(), "missing.md") {
		t.Errorf("ReadFile of a missing file = %v, want an error naming it as given", err)
	}
	if err := os.Remove("itos.yaml"); err != nil {
		t.Fatal(err)
	}
	if p.ConfigFound(t.Context()) {
		t.Error("ConfigFound with no config file")
	}

	var stdout, stderr strings.Builder
	code, err := runReview([]string{"post", "slice-9", "--verdict", "approve", "--at", "HEAD"}, Out{Stdout: &stdout, Stderr: &stderr, Dir: dir})
	if code != 1 || err != nil || stdout.String() != "" || !strings.Contains(stderr.String(), "does not set push.mode: pr") {
		t.Errorf("runReview with no config = %d, %v\n%s%s", code, err, stdout.String(), stderr.String())
	}
	if _, err := runReview(nil, Out{Dir: dir}); err == nil || !strings.HasPrefix(err.Error(), "review needs post") {
		t.Errorf("runReview with nothing = %v, want the usage error", err)
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
