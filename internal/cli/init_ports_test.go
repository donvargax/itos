package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	trees "github.com/donvargax/itos/v8/internal/store/tree/adapter"
)

// runInit reads its line through the slice and binds its ports to git, the
// working tree, pin's release, hook install and the three offers: a line it
// refuses is the usage error; a folder is made absolute as filepath.Abs
// makes it; hook install writes its object under json; and a first run
// writes the starter, the offers made as the flags answer
// them, the rules' files written named in what to commit.
func TestRunInitBindsItsPorts(t *testing.T) {
	var stdout, stderr strings.Builder
	o := Out{Stdout: &stdout, Stderr: &stderr}
	if _, err := runInit([]string{"--bogus"}, o); ExitCode(err) != ExitUsage {
		t.Errorf("--bogus: %v", err)
	}
	initScratch(t, "commit", nil)
	ctx, p := context.Background(), initPorts{o: o}
	if p.Worktree(ctx) != trees.Worktree || len(p.TaskFlags(ctx)) == 0 {
		t.Error("the working tree or the task flags")
	}
	abs, _ := filepath.Abs("bin")
	if got, err := p.Abs(ctx, "bin"); got != abs || err != nil {
		t.Errorf("bin made absolute: %q, %v", got, err)
	}
	if _, _, err := p.Release(ctx, ""); err == nil {
		t.Error("a release server that cannot be reached answered")
	}
	putFile(t, "itos.yaml", "version: 1\n")
	var hooks strings.Builder
	if code, err := p.InstallHooks(ctx, &hooks, true); code != 0 || err != nil || !json.Valid([]byte(hooks.String())) {
		t.Errorf("hook install: exit %d, %v:\n%s", code, err, hooks.String())
	}
	initScratch(t, "commit", nil)
	code, err := runInit([]string{"--plugin", "no", "--no-git-shim", "--agent-rules"}, o)
	want := "git add itos.yaml tasks/phase-1.yaml tasks/work-items.yaml AGENTS.md CLAUDE.md, then itos commit --task T-1"
	if code != 0 || err != nil || !strings.Contains(stdout.String(), want) {
		t.Errorf("init: exit %d, %v:\n%s", code, err, stdout.String())
	}
}

// The offers' ports: stdin, no terminal under go test, a null device none
// either; exec.LookPath, and a program run with its output and exit; the
// folder and file writes; and git-shim install's links on the OS o names.
func TestInitPortsBindTheOffers(t *testing.T) {
	dir := scratchFolder(t)
	ctx, p := context.Background(), initPorts{o: Out{OS: runtime.GOOS}}
	if p.Stdin(ctx) != os.Stdin || p.Terminal(ctx) {
		t.Error("stdin, or a terminal under go test")
	}
	null, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	if terminal(null) {
		t.Error("the null device is a terminal")
	}
	null.Close()
	if terminal(null) {
		t.Error("a closed file is a terminal")
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.LookPath(ctx, filepath.Join(dir, "no-such-program")); err == nil {
		t.Error("a program that is not there was found")
	}
	if output, err := p.Command(ctx, self, "-test.run=^$"); err != nil || !strings.Contains(output, "PASS") {
		t.Errorf("a run: %v:\n%s", err, output)
	}
	t.Setenv(fakeGitFailureEnv, "1")
	var exit *exec.ExitError
	if output, err := p.Command(ctx, self); !errors.As(err, &exit) || exit.ExitCode() != 1 || output != "injected git failure\n" {
		t.Errorf("a failing run: %v:\n%s", err, output)
	}
	if err := p.MkdirAll(ctx, filepath.Join("a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteText(ctx, filepath.Join("a", "b", "c.md"), "one\ntwo\n", true); err != nil {
		t.Fatal(err)
	}
	if text, _ := os.ReadFile(filepath.Join("a", "b", "c.md")); string(text) != "one\r\ntwo\r\n" {
		t.Errorf("the text written: %q", text)
	}
	gotSelf, link, err := p.ShimPlace(ctx, filepath.Join(dir, "bin"))
	if err != nil || filepath.Dir(link) != filepath.Join(dir, "bin") {
		t.Fatalf("the place: %s, %v", link, err)
	}
	if err := p.MakeLink(ctx, gotSelf, link, false); err != nil {
		t.Fatal(err)
	}
	if mine, _ := p.LinksItos(ctx, link); !mine {
		t.Errorf("%s does not link this itos", link)
	}
	at, realGit, realAt := p.PathStanding(ctx, filepath.Dir(link))
	if line := p.StandingLine(ctx, filepath.Dir(link), at, realGit, realAt); at != -1 ||
		!strings.HasPrefix(line, filepath.Dir(link)+" is not on the PATH") {
		t.Errorf("the folder stands at %d: %s", at, line)
	}
}
