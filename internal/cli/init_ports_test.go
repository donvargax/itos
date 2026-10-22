package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
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
