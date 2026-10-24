package adapter

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/donvargax/itos/v8/internal/git"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/lock"
	"github.com/donvargax/itos/v8/internal/store/tracker/port"
	"github.com/donvargax/itos/v8/internal/store/tracker/port/porttest"
	vcsadapter "github.com/donvargax/itos/v8/internal/vcs/adapter"
	vcs "github.com/donvargax/itos/v8/internal/vcs/port"
)

// gitIn runs git in the folder the test stands in and gives what it prints.
func gitIn(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command(git.Bin(runtime.GOOS), args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %s", strings.Join(args, " "), out)
	}
	return string(out)
}

// project stands the test in a new repository with one empty commit and an
// empty folder tasks, with no git setting from outside, no repository above
// it, its hooks in a folder of their own (core.hooksPath), none yet, and a
// short lock wait. Its Files commits there, or is under a stealth config
// when stealth, its hooks ready until unready says they are not.
func project(t *testing.T, stealth bool) (Files, *error) {
	t.Helper()
	for _, name := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_COMMON_DIR"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for name, value := range map[string]string{
		"GIT_AUTHOR_NAME": "A", "GIT_AUTHOR_EMAIL": "a@example.com",
		"GIT_COMMITTER_NAME": "A", "GIT_COMMITTER_EMAIL": "a@example.com",
	} {
		t.Setenv(name, value)
	}
	was := lock.Wait
	lock.Wait = 50 * time.Millisecond
	t.Cleanup(func() { lock.Wait = was })
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	t.Chdir(dir)
	gitIn(t, "init", "-q")
	gitIn(t, "config", "core.hooksPath", "hooks")
	gitIn(t, "config", "commit.gpgsign", "false")
	gitIn(t, "commit", "-q", "--allow-empty", "-m", "start")
	for _, folder := range []string{"tasks", "hooks"} {
		if err := os.Mkdir(folder, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(".git", "info", "exclude"), []byte("/hooks/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var unready error
	return Files{
		VCS:        vcsadapter.Git{Dir: dir},
		OS:         runtime.GOOS,
		Stealth:    stealth,
		Lock:       "registry.yaml",
		HooksReady: func(context.Context) error { return unready },
	}, &unready
}

// hook writes the hook, by its name, in the project's hooks folder.
func hook(t *testing.T, name, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the hook is a shell script")
	}
	if err := os.WriteFile(filepath.Join("hooks", name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

// The adapter keeps the tracker on disk and in git as the fake keeps it in
// memory.
func TestFilesKeepTheContract(t *testing.T) {
	porttest.TrackerContract(t, func(t *testing.T, stealth bool) porttest.Fixture {
		files, unready := project(t, stealth)
		return porttest.Fixture{
			Tracker: files,
			Edit: func(name, text string) {
				if err := os.WriteFile(name, []byte(text), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			Refuse:  func() { hook(t, "commit-msg", "#!/bin/sh\necho refused >&2\nexit 1\n") },
			Unready: func() { *unready = errors.New("the hooks are not ready") },
		}
	})
}

// broken is the git adapter with one step failing, as a git that does not
// start fails it.
type broken struct {
	vcs.VCS
	step string
}

var errBroken = errors.New("broken")

func (b broken) Add(ctx context.Context, paths []string, talk vcs.Talk) error {
	if b.step == "add" {
		return errBroken
	}
	return b.VCS.Add(ctx, paths, talk)
}

func (b broken) Commit(ctx context.Context, c vcs.Commit, talk vcs.Talk) error {
	if b.step == "commit" {
		return errBroken
	}
	return b.VCS.Commit(ctx, c, talk)
}

func (b broken) Unstage(ctx context.Context, paths []string) error {
	if b.step == "unstage" {
		return errBroken
	}
	return b.VCS.Unstage(ctx, paths)
}

func (b broken) Untrack(ctx context.Context, paths []string) error {
	if b.step == "untrack" {
		return errBroken
	}
	return b.VCS.Untrack(ctx, paths)
}

// Locate fails as internal/git's Read fails a repository it cannot find:
// kind.Missing.
func (b broken) Locate(ctx context.Context, dir string) (vcs.Location, error) {
	if b.step == "locate" {
		return vcs.Location{}, kind.Wrap(kind.Missing, errBroken)
	}
	return b.VCS.Locate(ctx, dir)
}

func (b broken) PathStatus(ctx context.Context, path string) ([]string, error) {
	if b.step == "status" {
		return nil, errBroken
	}
	return b.VCS.PathStatus(ctx, path)
}

func (b broken) Resolve(ctx context.Context, rev string) (string, error) {
	if b.step == "resolve" {
		return "", errBroken
	}
	return b.VCS.Resolve(ctx, rev)
}

// seeded is the project with registry.yaml committed, "r\n".
func seeded(t *testing.T, stealth bool) Files {
	t.Helper()
	files, _ := project(t, false)
	if _, err := files.Transact(t.Context(), port.Transaction{
		Changes: []port.Change{{Name: "registry.yaml", Text: "r\n", Created: true}}, Paragraphs: []string{"docs: start"},
	}); err != nil {
		t.Fatal(err)
	}
	files.Stealth = stealth
	return files
}

// edit is the change of registry.yaml and a new ledger file, as task add
// writes them.
func edit(ledger string) port.Transaction {
	return port.Transaction{Changes: []port.Change{
		{Name: "registry.yaml", Old: "r\n", Text: "r2\n"},
		{Name: ledger, Text: "- id: T-2\n", Created: true},
	}, Paragraphs: []string{"docs: add T-2", "Add T-2."}}
}

// asItWas is registry.yaml as seeded, the ledger file gone, and git status
// reporting nothing.
func asItWas(t *testing.T, ledger string) {
	t.Helper()
	if text, err := os.ReadFile("registry.yaml"); err != nil || string(text) != "r\n" {
		t.Errorf("registry.yaml reads %q (%v)", text, err)
	}
	if _, err := os.Stat(ledger); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("%s is still there (%v)", ledger, err)
	}
	if status := gitIn(t, "status", "--porcelain", "--untracked-files=all"); status != "" {
		t.Errorf("git status reports:\n%s", status)
	}
}

// A file keeps its mode; a new one is 0644.
func TestTransactKeepsModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows keeps no Unix permission bits")
	}
	files := seeded(t, false)
	if err := os.Chmod("registry.yaml", 0o600); err != nil {
		t.Fatal(err)
	}
	gitIn(t, "update-index", "--refresh")
	if _, err := files.Transact(t.Context(), edit("new.yaml")); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]os.FileMode{"registry.yaml": 0o600, "new.yaml": 0o644} {
		if info, err := os.Stat(path); err != nil || info.Mode().Perm() != want {
			t.Errorf("%s: %v, %v; want %v", path, info.Mode().Perm(), err, want)
		}
	}
}

// A write that fails after the first one puts the first back, in a project
// and under a stealth config alike; a file that cannot even be opened was
// not touched, so only the files before it are.
func TestTransactPutsBackWhenAWriteFails(t *testing.T) {
	for _, stealth := range []bool{false, true} {
		files := seeded(t, stealth)
		ledger := filepath.Join("missing", "new.yaml")
		_, err := files.Transact(t.Context(), edit(ledger))
		if err == nil || !strings.HasPrefix(err.Error(), ledger+" cannot be written: ") || strings.Contains(err.Error(), "put back") {
			t.Errorf("stealth %v: the error is %v", stealth, err)
		}
		asItWas(t, ledger)
	}
}

// A git that cannot start, to add or to commit, puts everything back.
func TestTransactPutsBackWhenGitCannotRun(t *testing.T) {
	for _, step := range []string{"add", "commit"} {
		files := seeded(t, false)
		files.VCS = broken{files.VCS, step}
		if _, err := files.Transact(t.Context(), edit("new.yaml")); err == nil || err.Error() != "cannot run git: broken" {
			t.Errorf("%s: the error is %v", step, err)
		}
		asItWas(t, "new.yaml")
	}
}

// git add refusing a created file puts everything back, and its words go to
// Words.
func TestTransactGitAddRefused(t *testing.T) {
	files := seeded(t, false)
	if err := os.WriteFile(filepath.Join(".git", "info", "exclude"), []byte("/hooks/\nnew.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var words strings.Builder
	change := edit("new.yaml")
	change.Words = &words
	_, err := files.Transact(t.Context(), change)
	var refused *port.Refused
	if !errors.As(err, &refused) || refused.Step != "git add of new.yaml" || refused.Code != 1 || refused.Restore != nil {
		t.Errorf("the error is %v", err)
	}
	if !strings.Contains(words.String(), "ignored") {
		t.Errorf("the words are %q", words.String())
	}
	asItWas(t, "new.yaml")
}

// A refused commit whose files cannot be put back says why, each file
// named: one a hook made a folder, a created one a folder holding a file,
// and git's index of a file that cannot be set.
func TestTransactCannotPutBack(t *testing.T) {
	files := seeded(t, false)
	hook(t, "commit-msg", "#!/bin/sh\nrm registry.yaml new.yaml && mkdir registry.yaml new.yaml && echo x > registry.yaml/x && echo x > new.yaml/x\nexit 1\n")
	_, err := files.Transact(t.Context(), edit("new.yaml"))
	var refused *port.Refused
	if !errors.As(err, &refused) || refused.Step != "the commit of registry.yaml and new.yaml" || refused.Restore == nil ||
		!strings.Contains(refused.Restore.Error(), "registry.yaml cannot be put back: ") ||
		!strings.Contains(refused.Restore.Error(), "new.yaml cannot be put back: ") {
		t.Errorf("the error is %v", err)
	}
	for _, step := range []string{"unstage", "untrack"} {
		files := seeded(t, false)
		files.VCS = broken{files.VCS, step}
		hook(t, "commit-msg", "#!/bin/sh\nexit 1\n")
		_, err := files.Transact(t.Context(), edit("new.yaml"))
		if !errors.As(err, &refused) || refused.Restore == nil || !strings.Contains(refused.Restore.Error(), "cannot be put back: broken") {
			t.Errorf("%s: the error is %v", step, err)
		}
	}
}

// A write that fails puts back the files before it, and says so when one
// cannot be.
func TestTransactSaysWhatCannotBePutBack(t *testing.T) {
	files := seeded(t, false)
	files.VCS = broken{files.VCS, "unstage"}
	ledger := filepath.Join("missing", "new.yaml")
	_, err := files.Transact(t.Context(), edit(ledger))
	if want := ledger + " cannot be written: "; err == nil || !strings.HasPrefix(err.Error(), want) ||
		!strings.Contains(err.Error(), ", and registry.yaml cannot be put back: broken") {
		t.Errorf("the error is %v", err)
	}
}

// A commit made whose index cannot be set, or whose HEAD cannot be read,
// says so.
func TestTransactAfterTheCommit(t *testing.T) {
	files := seeded(t, false)
	files.VCS = broken{files.VCS, "unstage"}
	if _, err := files.Transact(t.Context(), edit("new.yaml")); err == nil ||
		err.Error() != "the commit of registry.yaml and new.yaml was made, but git's index of them cannot be set to it: broken" {
		t.Errorf("the error is %v", err)
	}
	files = seeded(t, false)
	files.VCS = broken{files.VCS, "resolve"}
	if sha, err := files.Transact(t.Context(), edit("new.yaml")); sha != "" || !errors.Is(err, errBroken) {
		t.Errorf("Transact: %q, %v", sha, err)
	}
}

// A pre-commit hook that rewrites the files it commits, as a formatter does,
// leaves them in the commit and the working tree in its version; git's
// index of them is set to the commit's (bug 17), and a change staged before
// stays staged.
func TestTransactLeavesTheIndexAsCommitted(t *testing.T) {
	files := seeded(t, false)
	if err := os.WriteFile("other.yaml", []byte("o\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, "add", "--", "other.yaml")
	hook(t, "pre-commit", `#!/bin/sh
git diff --cached --name-only -- registry.yaml new.yaml | while IFS= read -r file; do
	echo '# formatted' >>"$file"
	git add -- "$file" || exit 1
done
`)
	sha, err := files.Transact(t.Context(), edit("new.yaml"))
	if err != nil || sha != strings.TrimSpace(gitIn(t, "rev-parse", "HEAD")) {
		t.Fatalf("Transact: %q, %v", sha, err)
	}
	if text := gitIn(t, "show", "HEAD:new.yaml"); text != "- id: T-2\n# formatted\n" {
		t.Errorf("the commit holds new.yaml as %q", text)
	}
	if status := gitIn(t, "status", "--porcelain", "--untracked-files=all"); status != "A  other.yaml\n" {
		t.Errorf("git status reports:\n%s", status)
	}
}

// The commit runs in the transaction's environment, and its message is its
// paragraphs.
func TestTransactCommitsAsTheTransactionSays(t *testing.T) {
	files := seeded(t, false)
	hook(t, "commit-msg", "#!/bin/sh\necho \"$MARK\" >>\"$1\"\n")
	change := edit("new.yaml")
	change.Env = append(os.Environ(), "MARK=marked")
	if _, err := files.Transact(t.Context(), change); err != nil {
		t.Fatal(err)
	}
	if got := gitIn(t, "log", "-1", "--format=%B"); got != "docs: add T-2\n\nAdd T-2.\nmarked\n\n" {
		t.Errorf("the commit's message is %q", got)
	}
}

// A document that is a folder, a folder no name can be, or a lock in a
// file, is the disk's error. A file listed as a folder is not one: Windows
// says the path is not found, which is no folder, as adr.Next has read it.
func TestFilesSayTheDisksErrors(t *testing.T) {
	files, _ := project(t, false)
	if docs, err := files.Load(t.Context(), []string{"tasks"}); err == nil {
		t.Errorf("a folder loaded as a document: %v", docs)
	}
	if names, err := files.List(t.Context(), "bad\x00name"); err == nil {
		t.Errorf("a name no folder can have listed: %q", names)
	}
	if err := os.WriteFile("plain", nil, 0o644); err != nil {
		t.Fatal(err)
	}
	files.Stealth = true
	files.Lock = filepath.Join("plain", "registry.yaml")
	if _, err := files.Hold(t.Context()); err == nil {
		t.Error("a lock taken in a file")
	}
}
