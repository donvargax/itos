package gitshim

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/out"
)

// ports is git-shim's ports over a folder: the running itos, Self, the
// link's default folder, Home, what a git there links, where the folders
// stand on the PATH, and the failures set; each call it takes is recorded.
type ports struct {
	self, home  string
	place, link error
	mine, other bool
	at, realAt  int
	git         string
	code        int
	ran         error
	calls       *[]string
}

func (p ports) record(format string, a ...any) {
	if p.calls != nil {
		*p.calls = append(*p.calls, fmt.Sprintf(format, a...))
	}
}

func (p ports) Place(_ context.Context, dir string) (string, string, error) {
	p.record("place %s", dir)
	if p.place != nil {
		return "", "", p.place
	}
	if dir == "" {
		dir = p.home
	}
	return p.self, filepath.Join(dir, "git"), nil
}

func (p ports) LinksItos(_ context.Context, link string) (bool, bool) {
	p.record("links %s", filepath.Base(link))
	return p.mine, p.other
}

func (p ports) MakeLink(_ context.Context, self, link string, replace bool) error {
	p.record("make %s %s %v", filepath.Base(self), filepath.Base(link), replace)
	if p.link != nil {
		return p.link
	}
	return os.WriteFile(link, []byte(self), 0o644)
}

func (p ports) Standing(_ context.Context, dir string) (int, string, int) {
	p.record("standing %s", filepath.Base(dir))
	return p.at, p.git, p.realAt
}

func (p ports) Commit(_ context.Context, args []string) (int, error) {
	p.record("commit %q", args)
	return p.code, p.ran
}

func (p ports) Push(_ context.Context, args []string) (int, error) {
	p.record("push %q", args)
	return p.code, p.ran
}

// gitShim runs the line over p, as itos would with --json when json, and
// gives its exit code, error and output.
func gitShim(t *testing.T, p Ports, json bool, args ...string) (int, string, string, error) {
	t.Helper()
	c, err := Parse(args, "", json)
	if err != nil {
		t.Fatalf("git-shim %s: %v", strings.Join(args, " "), err)
	}
	var stdout, stderr strings.Builder
	code, err := c.Run(context.Background(), &stdout, &stderr, NewHandler(p))
	return code, stdout.String(), stderr.String(), err
}

// failing is a writer that cannot be written to.
type failing struct{}

func (failing) Write([]byte) (int, error) { return 0, errors.New("closed") }

func TestParse(t *testing.T) {
	origin := filepath.Join("sub", "folder")
	abs := filepath.Join(t.TempDir(), "bin")
	for _, c := range []struct {
		args []string
		want CLI
	}{
		{[]string{"install"}, CLI{Query: Query{Sub: "install"}}},
		{[]string{"uninstall", "--dir", "bin"}, CLI{Query: Query{Sub: "uninstall", Dir: filepath.Join(origin, "bin")}}},
		{[]string{"install", "--dir", "a", "--dir", abs}, CLI{Query: Query{Sub: "install", Dir: abs}}},
		{[]string{"install", "--dir", "-"}, CLI{Query: Query{Sub: "install", Dir: "-"}}},
		{[]string{"run", "commit", "-m", "x"}, CLI{Query: Query{Sub: "run", Run: "commit", Args: []string{"-m", "x"}}}},
		{[]string{"run", "--", "push", "--no-wait"}, CLI{Query: Query{Sub: "run", Run: "push", Args: []string{"--no-wait"}}}},
		{[]string{"run", "--", "push"}, CLI{Query: Query{Sub: "run", Run: "push", Args: []string{}}}},
	} {
		got, err := Parse(c.args, origin, false)
		if err != nil || !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q: %+v, %v", c.args, got, err)
		}
	}
	if got, err := Parse([]string{"install"}, "", true); err != nil || !got.JSON {
		t.Errorf("--json: %+v, %v", got, err)
	}
	if got, err := Parse([]string{"install", "--dir", "bin"}, "", false); err != nil || got.Dir != "bin" {
		t.Errorf("no origin: %+v, %v", got, err)
	}
}

func TestParseRefuses(t *testing.T) {
	for _, c := range []struct {
		args []string
		want string
	}{
		{nil, "git-shim needs install, uninstall or run"},
		{[]string{"bogus"}, "git-shim needs install, uninstall or run"},
		{[]string{"install", "bin"}, "git-shim install takes only --dir <folder> (bin)"},
		{[]string{"uninstall", "--force"}, "git-shim uninstall takes only --dir <folder> (--force)"},
		{[]string{"install", "--dir"}, "git-shim install --dir needs <folder>"},
		{[]string{"uninstall", "--dir", ""}, "git-shim uninstall --dir needs <folder>"},
		{[]string{"run"}, "git-shim run needs commit or push, then git's arguments"},
		{[]string{"run", "--"}, "git-shim run needs commit or push, then git's arguments"},
		{[]string{"run", "status"}, "git-shim run needs commit or push, then git's arguments"},
		{[]string{"run", "--", "--", "commit"}, "git-shim run needs commit or push, then git's arguments"},
	} {
		var u out.UsageError
		if _, err := Parse(c.args, "", false); !errors.As(err, &u) || err.Error() != c.want {
			t.Errorf("%q: %v, want %q", c.args, err, c.want)
		}
	}
}

func TestTyped(t *testing.T) {
	abs := t.TempDir()
	for _, c := range []struct{ origin, p, want string }{
		{"sub", "bin", filepath.Join("sub", "bin")},
		{"", "bin", "bin"},
		{"sub", "", ""},
		{"sub", "-", "-"},
		{"sub", abs, abs},
	} {
		if got := typed(c.origin, c.p); got != c.want {
			t.Errorf("typed(%q, %q) = %q, want %q", c.origin, c.p, got, c.want)
		}
	}
}

// install links the running itos in the folder, unless a git that is not
// itos is there, and says where the folder stands on the PATH.
func TestRunInstall(t *testing.T) {
	for _, r := range []struct {
		name       string
		there      bool
		p          ports
		json       bool
		code       int
		stdout     string
		stderr     string
		calls      []string
		standsLast bool
	}{
		{"linked", false, ports{at: 0, git: "/usr/bin/git", realAt: 1}, false, 0,
			"Linked DIR/git to SELF.\nDIR comes before the real git (/usr/bin/git) on the PATH: git runs the shim, and in a repository itos manages git commit and git push are itos's.\n", "",
			[]string{"place ", "make itos git false", "standing home"}, true},
		{"kept", true, ports{mine: true, at: 1, git: "/usr/bin/git", realAt: 1}, false, 0,
			"DIR/git already links to SELF.\nDIR comes after the real git (/usr/bin/git) on the PATH: put it first for git to run the shim.\n", "",
			[]string{"place ", "links git", "standing home"}, true},
		{"replaced", true, ports{other: true, at: -1, git: "/usr/bin/git", realAt: 0}, false, 0,
			"Replaced DIR/git, another itos, with a link to SELF.\nDIR is not on the PATH: put it before /usr/bin, the real git's folder, for git to run the shim.\n", "",
			[]string{"place ", "links git", "make itos git true", "standing home"}, true},
		{"not on the PATH, no git", false, ports{at: -1, realAt: -1}, false, 0,
			"Linked DIR/git to SELF.\nDIR is not on the PATH, and the PATH has no other git for the shim to run.\n", "", nil, false},
		{"on the PATH, no git", false, ports{at: 0, realAt: -1}, false, 0,
			"Linked DIR/git to SELF.\nDIR is on the PATH, but the PATH has no other git for the shim to run: install git.\n", "", nil, false},
		{"refused", true, ports{}, false, 1, "",
			"itos: DIR/git is a git that is not itos; git-shim install never replaces one; left alone\n",
			[]string{"place ", "links git"}, false},
		{"linked --json", false, ports{at: 0, git: "/usr/bin/git", realAt: 1}, true, 0,
			`{"schema":1,"link":"DIR/git","target":"SELF","action":"linked","on_path":true,"before_git":true,"git":"/usr/bin/git"}`, "", nil, false},
		{"kept --json, after git", true, ports{mine: true, at: 1, git: "/usr/bin/git", realAt: 1}, true, 0,
			`{"schema":1,"link":"DIR/git","target":"SELF","action":"kept","on_path":true,"before_git":false,"git":"/usr/bin/git"}`, "", nil, false},
		{"replaced --json, no git", true, ports{other: true, at: 0, realAt: -1}, true, 0,
			`{"schema":1,"link":"DIR/git","target":"SELF","action":"replaced","on_path":true,"before_git":true}`, "", nil, false},
		{"--json, not on the PATH", false, ports{at: -1, realAt: -1}, true, 0,
			`{"schema":1,"link":"DIR/git","target":"SELF","action":"linked","on_path":false,"before_git":false}`, "", nil, false},
		{"refused --json", true, ports{}, true, 1, `{"schema":1,"link":"DIR/git","action":"refused"}`, "", nil, false},
	} {
		dir := filepath.Join(t.TempDir(), "home")
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(dir, "git")
		if r.there {
			if err := os.WriteFile(link, []byte("a git"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		var calls []string
		p := r.p
		p.self, p.home, p.calls = filepath.Join(t.TempDir(), "itos"), dir, &calls
		code, stdout, stderr, err := gitShim(t, p, r.json, "install")
		if r.json {
			stdout = compact(t, stdout)
		}
		want := func(s string) string {
			if r.json {
				return strings.NewReplacer("DIR/git", jsonString(link), "SELF", jsonString(p.self)).Replace(s)
			}
			return strings.NewReplacer("DIR/git", link, "DIR", dir, "SELF", p.self,
				"/usr/bin,", filepath.Dir("/usr/bin/git")+",").Replace(s)
		}
		if err != nil || code != r.code || stdout != want(r.stdout) || stderr != want(r.stderr) {
			t.Errorf("%s: exit %d, %v\nstdout %q\nstderr %q", r.name, code, err, stdout, stderr)
		}
		if r.calls != nil && !reflect.DeepEqual(calls, r.calls) {
			t.Errorf("%s: calls %q, want %q", r.name, calls, r.calls)
		}
		if text, err := os.ReadFile(link); r.code == 0 && !r.there && (err != nil || string(text) != p.self) {
			t.Errorf("%s: the link is %q, %v", r.name, text, err)
		}
	}
}

// uninstall removes an itos, says there is none, or leaves alone a git that
// is not itos.
func TestRunUninstall(t *testing.T) {
	for _, r := range []struct {
		name   string
		there  bool
		p      ports
		json   bool
		code   int
		stdout string
		stderr string
		gone   bool
	}{
		{"removed", true, ports{mine: true}, false, 0, "Removed DIR/git.\n", "", true},
		{"removed another itos", true, ports{other: true}, false, 0, "Removed DIR/git.\n", "", true},
		{"absent", false, ports{}, false, 0, "No git shim at DIR/git: nothing to remove.\n", "", true},
		{"refused", true, ports{}, false, 1, "",
			"itos: DIR/git is a git that is not itos; git-shim uninstall never removes one; left alone\n", false},
		{"removed --json", true, ports{mine: true}, true, 0, `{"schema":1,"link":"DIR/git","action":"removed"}`, "", true},
		{"absent --json", false, ports{}, true, 0, `{"schema":1,"link":"DIR/git","action":"absent"}`, "", true},
		{"refused --json", true, ports{}, true, 1, `{"schema":1,"link":"DIR/git","action":"refused"}`, "", false},
	} {
		dir := t.TempDir()
		link := filepath.Join(dir, "git")
		if r.there {
			if err := os.WriteFile(link, []byte("a git"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		p := r.p
		p.home = dir
		code, stdout, stderr, err := gitShim(t, p, r.json, "uninstall")
		want := strings.ReplaceAll(r.stdout, "DIR/git", link)
		if r.json {
			stdout, want = compact(t, stdout), strings.ReplaceAll(r.stdout, "DIR/git", jsonString(link))
		}
		if err != nil || code != r.code || stdout != want || stderr != strings.ReplaceAll(r.stderr, "DIR/git", link) {
			t.Errorf("%s: exit %d, %v\nstdout %q\nstderr %q", r.name, code, err, stdout, stderr)
		}
		if _, err := os.Lstat(link); errors.Is(err, os.ErrNotExist) != r.gone {
			t.Errorf("%s: the link is there: %v", r.name, err == nil)
		}
	}
}

// A link that cannot be placed, made or removed is a line on stderr, exit
// 3, under --json too; a link whose state cannot be read is the command
// line's error, nothing printed.
func TestRunFails(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "full")
	if err := os.MkdirAll(filepath.Join(full, "git", "inside"), 0o755); err != nil {
		t.Fatal(err)
	}
	unplaced := errors.New("cannot tell the itos binary running: gone")
	for _, json := range []bool{false, true} {
		for _, r := range []struct {
			name   string
			p      ports
			sub    string
			stderr string
		}{
			{"install unplaced", ports{place: unplaced}, "install", "itos: " + unplaced.Error() + "\n"},
			{"uninstall unplaced", ports{place: unplaced}, "uninstall", "itos: " + unplaced.Error() + "\n"},
			{"unlinked", ports{self: "itos", home: dir, link: errors.New("denied")}, "install",
				"itos: cannot link " + filepath.Join(dir, "git") + " to itos: denied\n"},
			{"unremoved", ports{home: full, mine: true}, "uninstall", "itos: cannot remove " + filepath.Join(full, "git") + ": "},
		} {
			code, stdout, stderr, err := gitShim(t, r.p, json, r.sub)
			if err != nil || code != 3 || stdout != "" || !strings.HasPrefix(stderr, r.stderr) {
				t.Errorf("%s, --json %v: exit %d, %v, stdout %q, stderr %q", r.name, json, code, err, stdout, stderr)
			}
		}
	}
	for _, sub := range []string{"install", "uninstall"} {
		code, stdout, stderr, err := gitShim(t, ports{home: "bad\x00name", mine: true}, false, sub)
		if err == nil || code != 0 || stdout != "" || stderr != "" {
			t.Errorf("%s of an unreadable link: exit %d, %v, stdout %q, stderr %q", sub, code, err, stdout, stderr)
		}
	}
	for _, sub := range []string{"install", "uninstall"} {
		existing := t.TempDir()
		if err := os.WriteFile(filepath.Join(existing, "git"), nil, 0o755); err != nil {
			t.Fatal(err)
		}
		c := CLI{Query: Query{Sub: sub}, JSON: true}
		for _, p := range []ports{{home: existing}, {home: existing, mine: true}} {
			if _, err := c.Run(context.Background(), failing{}, &strings.Builder{}, NewHandler(p)); err == nil {
				t.Errorf("%s: an object it cannot write is no error", sub)
			}
		}
	}
}

// run runs itos commit or itos push with git's arguments, exiting as it
// does.
func TestRunRuns(t *testing.T) {
	broken := errors.New("broken")
	for _, r := range []struct {
		args  []string
		p     ports
		code  int
		err   error
		calls []string
	}{
		{[]string{"run", "commit", "-m", "x"}, ports{code: 1}, 1, nil, []string{`commit ["-m" "x"]`}},
		{[]string{"run", "--", "push", "-q"}, ports{code: 75}, 75, nil, []string{`push ["-q"]`}},
		{[]string{"run", "push"}, ports{ran: broken}, 0, broken, []string{`push []`}},
	} {
		var calls []string
		p := r.p
		p.calls = &calls
		code, stdout, stderr, err := gitShim(t, p, true, r.args...)
		if code != r.code || !errors.Is(err, r.err) || (r.err == nil) != (err == nil) || stdout != "" || stderr != "" || !reflect.DeepEqual(calls, r.calls) {
			t.Errorf("%q: exit %d, %v, calls %q", r.args, code, err, calls)
		}
	}
}

// compact is one --json object on one line, its schema first.
func compact(t *testing.T, s string) string {
	t.Helper()
	var b strings.Builder
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		b.WriteString(strings.Replace(strings.TrimSpace(line), `": `, `":`, 1))
	}
	return b.String()
}

// jsonString is s as a JSON string's contents.
func jsonString(s string) string { return strings.ReplaceAll(s, `\`, `\\`) }
