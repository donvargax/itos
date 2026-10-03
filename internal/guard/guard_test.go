package guard

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// abs is an absolute folder on this platform, as a command line writes it: on windows with a
// drive and slashes, since sh reads a backslash as an escape.
var abs = map[bool]string{false: "/abs", true: "C:/abs"}[runtime.GOOS == "windows"]

func TestGitCalls(t *testing.T) {
	cases := []struct {
		command string
		want    []GitCall
	}{
		{"git commit -m 'x'", []GitCall{{".", "commit"}}},
		{"/usr/bin/git push", []GitCall{{".", "push"}}},
		{"git -C sub -C ../other -c core.editor=true --no-pager commit", []GitCall{{"other", "commit"}}},
		{"git -C " + abs + " push", []GitCall{{abs, "push"}}},
		{"git --git-dir=.git --work-tree . -p status", []GitCall{{".", "status"}}},
		{"go vet ./... && git pull --rebase && git push", []GitCall{{".", "pull"}, {".", "push"}}},
		{"A=1 B=2 git commit", []GitCall{{".", "commit"}}},
		{"env -- A=1 git commit", nil},
		{"command env A=1 nohup git commit", []GitCall{{".", "commit"}}},
		{"exec git push", []GitCall{{".", "push"}}},
		{"time git push", []GitCall{{".", "push"}}},
		{"grep -n 'git commit' AGENTS.md", nil},
		{`echo "git push" | cat`, nil},
		{"echo $(git rev-parse HEAD)", []GitCall{{".", "rev-parse"}}},
		{"(cd sub; git commit)\nif true; then git push; fi", []GitCall{{".", "commit"}, {".", "push"}}},
		{`g\it "com"'mit'`, []GitCall{{".", "commit"}}},
		{"sh -c 'git commit'", nil},
		{"eval git commit", nil},
		{"$GIT commit", nil},
		{"git $SUB", nil},
		{"git", nil},
		{"git -C", nil},
		{"cat <<EOF\ngit commit\nEOF", nil},
	}
	for _, c := range cases {
		got, err := GitCalls(c.command)
		if err != nil {
			t.Errorf("%q: %v", c.command, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q: got %v, want %v", c.command, got, c.want)
		}
	}
	if _, err := GitCalls("git commit -m 'unclosed"); err == nil {
		t.Error("an unclosed quote parses")
	}
}

func TestRead(t *testing.T) {
	in, err := Read(strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls","timeout":1},"cwd":"/x","other":[1]}`))
	if err != nil || in != (Input{"Bash", "ls", "/x"}) {
		t.Errorf("got %+v, %v", in, err)
	}
	in, err = Read(strings.NewReader(`{"tool_name":"Edit","tool_input":{"file_path":"a"}}`))
	if err != nil || in != (Input{Tool: "Edit"}) {
		t.Errorf("got %+v, %v", in, err)
	}
	for _, bad := range []string{"not json", "", "null", "[]", `{"tool_name":1}`, `{}`, `{"tool_name":"Bash"}`,
		`{"tool_name":"Bash","tool_input":{"command":3}}`, `{"tool_name":"Edit"} {}`} {
		if _, err := Read(strings.NewReader(bad)); err == nil {
			t.Errorf("%q reads", bad)
		}
	}
}

func TestReason(t *testing.T) {
	t.Setenv("ITOS_CONFIG", "")
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "itos.yaml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	elsewhere := t.TempDir()
	cases := []struct {
		in   Input
		here string
		want []string
	}{
		{Input{"Bash", "git commit", repo}, elsewhere, []string{"itos commit --task"}},
		{Input{"Bash", "git push && git commit", filepath.Join(repo, "sub")}, elsewhere, []string{"itos commit", "itos push"}},
		{Input{"Bash", "git commit", "sub"}, repo, []string{"itos commit"}},
		{Input{"Bash", "git commit", ""}, filepath.Join(repo, "sub"), []string{"itos commit"}},
		{Input{"Bash", "git -C " + filepath.ToSlash(repo) + " push", elsewhere}, elsewhere, []string{"itos push"}},
		{Input{"Bash", "git commit", elsewhere}, repo, nil},
		{Input{"Bash", "git -C " + filepath.ToSlash(elsewhere) + " commit", repo}, repo, nil},
		{Input{"Bash", "git status", repo}, repo, nil},
		{Input{"Bash", "git commit 'unclosed", repo}, repo, nil},
		{Input{"Edit", "", repo}, repo, nil},
	}
	for _, c := range cases {
		reason, deny := Reason(c.in, c.here)
		if deny != (c.want != nil) {
			t.Errorf("%+v: deny %v, reason %q", c.in, deny, reason)
		}
		for _, w := range c.want {
			if !strings.Contains(reason, w) {
				t.Errorf("%+v: the reason %q does not say %q", c.in, reason, w)
			}
		}
	}
}

func TestDeny(t *testing.T) {
	var b strings.Builder
	if err := Deny(&b, "use <this>"); err != nil {
		t.Fatal(err)
	}
	want := `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"use <this>"}}` + "\n"
	if b.String() != want {
		t.Errorf("got %s", b.String())
	}
}
