package guard

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// folders is the guard's ports over a set of managed folders.
type folders map[string]bool

func (f folders) Managed(_ context.Context, dir string) bool { return f[dir] }

func TestParse(t *testing.T) {
	if c, err := Parse([]string{"claude-code"}, "sub"); err != nil || c != (CLI{"claude-code", "sub"}) {
		t.Errorf("claude-code: %+v, %v", c, err)
	}
	for args, want := range map[string]string{"": "unknown command: guard", "codex": "unknown command: guard codex"} {
		if _, err := Parse(strings.Fields(args), "."); err == nil || err.Error() != want {
			t.Errorf("guard %s: %v", args, err)
		}
	}
}

// failing is a writer that cannot be written to.
type failing struct{}

func (failing) Write([]byte) (int, error) { return 0, errors.New("closed") }

func TestRun(t *testing.T) {
	repo := t.TempDir()
	h := NewHandler(folders{repo: true})
	c := CLI{Harness: "claude-code", Started: repo}
	for _, r := range []struct {
		input, stdout, stderr string
		code                  int
	}{
		{`{"tool_name":"Bash","tool_input":{"command":"git commit"}}`, `"permissionDecision":"deny"`, "", 0},
		{`{"tool_name":"Bash","tool_input":{"command":"git commit"},"cwd":".."}`, "", "", 0},
		{`{"tool_name":"Bash","tool_input":{"command":"git status"}}`, "", "", 0},
		{`not json`, "", "cannot read Claude Code's PreToolUse input on stdin", 1},
	} {
		var stdout, stderr strings.Builder
		code := c.Run(context.Background(), strings.NewReader(r.input), &stdout, &stderr, h)
		if code != r.code || !contains(stdout.String(), r.stdout) || !contains(stderr.String(), r.stderr) {
			t.Errorf("%s: exit %d, stdout %q, stderr %q", r.input, code, stdout.String(), stderr.String())
		}
	}
	var stderr strings.Builder
	in := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"git push"},"cwd":` + `"` + filepath.ToSlash(repo) + `"}`)
	if code := c.Run(context.Background(), in, failing{}, &stderr, h); code != 1 || !strings.Contains(stderr.String(), "cannot write its PreToolUse answer") {
		t.Errorf("an answer it cannot write: exit %d, stderr %q", code, stderr.String())
	}
}

// contains is whether got says want, or is empty when want is.
func contains(got, want string) bool {
	if want == "" {
		return got == ""
	}
	return strings.Contains(got, want)
}
