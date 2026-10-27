package commit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/out"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// shown answers Show with a message named after the revision, an error for
// "missing".
type shown struct{ vcsport.VCS }

func (shown) Show(_ context.Context, rev string) (vcsport.Entry, error) {
	if rev == "missing" {
		return vcsport.Entry{}, errors.New("no such commit")
	}
	return vcsport.Entry{Message: "message of " + rev}, nil
}

// The message is the named commit's where -C names one and --edit or -e did
// not come last, or -c names one and --no-edit did; HEAD's only for --amend
// --no-edit with neither, never beside --fixup or --squash, and unread for a
// commit that does not resolve or is an option.
func TestMessageOfReusedCommit(t *testing.T) {
	for args, want := range map[string]string{
		"-C HEAD~2":                         "message of HEAD~2",
		"-CHEAD~2":                          "message of HEAD~2",
		"-qC HEAD~2":                        "message of HEAD~2",
		"--reuse-message HEAD~2":            "message of HEAD~2",
		"--reuse-message=HEAD~2":            "message of HEAD~2",
		"-C a -C b":                         "message of b",
		"-C HEAD~2 --no-edit":               "message of HEAD~2",
		"-C HEAD~2 --edit":                  "",
		"-C HEAD~2 -e":                      "",
		"-C HEAD~2 --edit --no-edit":        "message of HEAD~2",
		"-C HEAD~2 --no-edit --edit":        "",
		"-c HEAD~2":                         "",
		"-c HEAD~2 --edit":                  "",
		"-c HEAD~2 --no-edit":               "message of HEAD~2",
		"--reedit-message=HEAD~2 --no-edit": "message of HEAD~2",
		"--no-edit -c HEAD~2 -e":            "",
		"--amend --no-edit -C HEAD~2":       "message of HEAD~2",
		"--amend --no-edit -c HEAD~2":       "message of HEAD~2",
		"--amend -C HEAD~2":                 "message of HEAD~2",
		"--amend --no-edit":                 "message of HEAD",
		"--amend -C HEAD~2 --edit":          "",
		"-C HEAD --squash HEAD":             "",
		"-C HEAD --fixup=HEAD":              "",
		"--no-edit -c HEAD --squash=HEAD":   "",
		"-C missing":                        "",
		"-C --output=x":                     "",
		"-C":                                "",
		"-m one -C HEAD":                    "one",
		"-F - -C HEAD":                      "",
		"-C HEAD -- --edit":                 "message of HEAD",
		"-C HEAD~1 --no-reuse-message --amend --no-edit":  "message of HEAD",
		"-C HEAD~1 --no-reuse-message":                    "",
		"-c HEAD~1 --no-reedit-message --amend --no-edit": "message of HEAD",
		"-c HEAD~1 --no-edit --no-reedit-message":         "",
		"-C HEAD~1 --no-reuse-message -C HEAD~2":          "message of HEAD~2",
		"--no-reuse-message -C HEAD~1":                    "message of HEAD~1",
	} {
		got, ok := readGitArgs(strings.Fields(args)).message(context.Background(), shown{})
		if got != want || ok != (want != "") {
			t.Errorf("message of %q = %q, %v; want %q", args, got, ok, want)
		}
	}
}

// isUsage is whether err is a usage error, exit 2, whose words have text.
func isUsage(err error, text string) bool {
	var u out.UsageError
	return errors.As(err, &u) && strings.Contains(err.Error(), text)
}

// Each -m is wrapped in its place, however it is written; -F's text goes to
// a file of its own, named where the file was; nothing changes without a
// limit or when git will refuse -m with -F.
func TestWrapBody(t *testing.T) {
	long := "chore: tidy\n\n" + strings.Repeat("word ", 30)
	wrapped := "chore: tidy\n\n" + strings.TrimSpace(strings.Repeat("word ", 20)) + "\n" + strings.TrimSpace(strings.Repeat("word ", 10))
	for _, c := range []struct{ args, want []string }{
		{[]string{"-m", long}, []string{"-m", wrapped}},
		{[]string{"--message=" + long, "-q"}, []string{"--message=" + wrapped, "-q"}},
		{[]string{"-am" + long}, []string{"-am" + wrapped}},
		{[]string{"--message", long, "--", "x"}, []string{"--message", wrapped, "--", "x"}},
	} {
		got, written, err := wrapBody(c.args, readGitArgs(c.args), 100, nil, "#")
		if err != nil || written != "" || !slices.Equal(got, c.want) {
			t.Errorf("wrapBody(%q) = %q, %q, %v", c.args, got, written, err)
		}
	}
	for _, args := range [][]string{{"-m", "x", "-F", "y"}, {"-m", long}} {
		limit := 100
		if len(args) == 2 {
			limit = 0
		}
		if got, _, _ := wrapBody(args, readGitArgs(args), limit, nil, "#"); !slices.Equal(got, args) {
			t.Errorf("wrapBody(%q, %d) = %q", args, limit, got)
		}
	}

	file := filepath.Join(t.TempDir(), "msg.txt")
	if err := os.WriteFile(file, []byte(long+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{"-F", file}
	got, written, err := wrapBody(args, readGitArgs(args), 100, nil, "#")
	if err != nil || written == "" || !slices.Equal(got, []string{"-F", written}) {
		t.Fatalf("wrapBody(%q) = %q, %q, %v", args, got, written, err)
	}
	defer os.Remove(written)
	if text, _ := os.ReadFile(written); string(text) != wrapped+"\n" {
		t.Errorf("the written message is %q", text)
	}
}

// A link flag takes the next argument as its value only when it has none of
// its own, written --task=<ids>, and the next is no option; one at the end
// with no value is a usage error.
func TestReadCommitFlags(t *testing.T) {
	f, err := readCommitFlags([]string{"--task=T-1", "notes.md", "--item", "a", "-m", "x"}, nil)
	if err != nil || !slices.Equal(f.tasks, []string{"T-1"}) || !slices.Equal(f.items, []string{"a"}) ||
		!slices.Equal(f.git, []string{"notes.md", "-m", "x"}) {
		t.Errorf("readCommitFlags = %+v, %v", f, err)
	}
	for _, args := range [][]string{{"--task"}, {"-m", "x", "--item"}, {"--scenarios", "-m", "x"}} {
		if _, err := readCommitFlags(args, nil); !isUsage(err, "needs <") {
			t.Errorf("readCommitFlags(%q) = %v, not a usage error", args, err)
		}
	}
}

// The options itos commit refuses (slice 128) are read as git reads them: a
// cluster's letters (-am, -an), git's abbreviation of --no-verify, never a
// valued option's value (-m -n, -mn, --message --no-verify, -uno) nor what
// comes after "--"; the first refused is the one named.
func TestReadGitArgsRefuses(t *testing.T) {
	for args, want := range map[string]string{
		"--no-verify -m x":       "--no-verify",
		"--no-veri":              "--no-veri",
		"-n -m x":                "-n",
		"-a":                     "-a",
		"-am x":                  "-a",
		"-na":                    "-n",
		"-an":                    "-a",
		"-q --all":               "--all",
		"-m x -- --no-verify":    "",
		"-m -n":                  "",
		"-mn":                    "",
		"--message --no-verify":  "",
		"--message=-a":           "",
		"-uno":                   "",
		"-Sn":                    "",
		"--no-ver":               "",
		"--allow-empty --verify": "",
	} {
		if got := readGitArgs(strings.Fields(args)).refused; got != want {
			t.Errorf("readGitArgs(%q).refused = %q, want %q", args, got, want)
		}
	}
}

// A refused option is a usage error naming it and why; a core.hooksPath that
// git -c sets, handed on by the shim or by git, in any case, is refused too.
func TestRefuseCommitArgs(t *testing.T) {
	for _, name := range []string{"GIT_CONFIG_COUNT", "GIT_CONFIG_PARAMETERS"} {
		t.Setenv(name, "")
	}
	for refused, says := range map[string]string{"-a": "git add <path>", "--all": "stages every change",
		"-n": "skips git's hooks", "--no-verify": "skips git's hooks"} {
		err := refuseCommitArgs(gitArgs{refused: refused})
		if !isUsage(err, refused+":") || !strings.Contains(err.Error(), says) {
			t.Errorf("refusing %s: %v", refused, err)
		}
	}
	if err := refuseCommitArgs(gitArgs{}); err != nil {
		t.Errorf("nothing refused: %v", err)
	}
	t.Setenv("GIT_CONFIG_COUNT", "2")
	t.Setenv("GIT_CONFIG_KEY_0", "user.name")
	t.Setenv("GIT_CONFIG_KEY_1", "Core.HooksPath")
	if err := refuseCommitArgs(gitArgs{}); !isUsage(err, "core.hooksPath") {
		t.Errorf("git -c core.hooksPath through the shim: %v", err)
	}
}

// An amend is --amend among git commit's options, never a value of one, nor
// a path after --; a long option that takes no value takes no argument after
// it as one.
func TestAmends(t *testing.T) {
	for _, c := range []struct {
		args []string
		want bool
	}{
		{[]string{"--amend", "--no-edit"}, true},
		{[]string{"-m", "x", "--amend"}, true},
		{[]string{"--allow-empty", "--amend"}, true},
		{[]string{"-am", "x", "--amend"}, true},
		{[]string{"-m", "--amend"}, false},
		{[]string{"-m--amend"}, false},
		{[]string{"--message", "--amend"}, false},
		{[]string{"--message=--amend"}, false},
		{[]string{"-C", "--amend"}, false},
		{[]string{"-q", "--", "--amend"}, false},
		{[]string{"--amend", "--no-amend"}, false},
		{[]string{"-m", "x"}, false},
	} {
		if got := readGitArgs(c.args).amend; got != c.want {
			t.Errorf("readGitArgs(%q).amend = %v", c.args, got)
		}
	}
}

// A commit is given paths by a bare argument or after "--", or by a
// pathspec file; never by a value of an option, nor by an empty "--".
func TestReadGitArgsByPath(t *testing.T) {
	for args, want := range map[string]bool{
		"-m x":                        false,
		"-m x a.md":                   true,
		"a.md -m x":                   true,
		"-m x -- a.md":                true,
		"-m x --":                     false,
		"-m x --pathspec-from-file=l": true,
		"--pathspec-from-file l -m x": true,
		"-m x --author me":            false,
		"-F msg --amend --no-edit":    false,
		"-am x":                       false,
		"-S -q":                       false,
	} {
		if got := readGitArgs(strings.Fields(args)).byPath; got != want {
			t.Errorf("readGitArgs(%q).byPath = %v, want %v", args, got, want)
		}
	}
}
