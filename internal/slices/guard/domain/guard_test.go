package domain

import (
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
		{"env -- A=1 git commit", []GitCall{{".", "commit"}}},
		{"command env A=1 nohup git commit", []GitCall{{".", "commit"}}},
		// A bare assignment before a command is bash's, with a shell variable's
		// name; env's is any word holding an =, and bash runs A-B=1 git as the
		// command A-B=1.
		{"A-B=1 git commit", nil},
		{"1A=1 git commit", nil},
		{"A=1 A-B=1 git commit", nil},
		{"exec A=1 git commit", nil},
		{"command A=1 git commit", nil},
		{"env A-B=1 git commit", []GitCall{{".", "commit"}}},
		{"env 1A=1 git commit", []GitCall{{".", "commit"}}},
		{"env A_1=1 A-B=2 git commit", []GitCall{{".", "commit"}}},
		{"env a.b=c=d git push", []GitCall{{".", "push"}}},
		{"env =1 git commit", nil},
		{"env A=1", nil},
		{"env git commit", []GitCall{{".", "commit"}}},
		// env's options: the ones it skips are read past, and the others end
		// what the guard reads.
		{"env -i A-B=1 git commit", []GitCall{{".", "commit"}}},
		{"env - git commit", []GitCall{{".", "commit"}}},
		{"env -0 --ignore-environment --null git commit", []GitCall{{".", "commit"}}},
		{"env -u HOME git commit", []GitCall{{".", "commit"}}},
		{"env -uHOME --unset=PATH --unset PWD git commit", []GitCall{{".", "commit"}}},
		{"env -u", nil},
		{"env --unset", nil},
		{"env -i -- A-B=1 git push", []GitCall{{".", "push"}}},
		{"env -S 'git commit'", nil},
		{"env -C sub git commit", nil},
		{"env --chdir=sub git commit", nil},
		{"env --split-string=x git commit", nil},
		{"env A=1 -i git commit", nil},
		{"env -- -i git commit", nil},
		{"env -- --unset=A git commit", []GitCall{{".", "commit"}}},
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
	repo, elsewhere := t.TempDir(), t.TempDir()
	// itos manages repo and the folders under it, as config.Managed judges a
	// repository with a config at its top.
	managed := func(dir string) bool {
		rel, err := filepath.Rel(repo, dir)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
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
		reason, deny := Reason(c.in, c.here, managed)
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
