package commit

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// The texts the oracle's commits give: the earlier commit's message (what an
// amend starts from), what stdin gives -F - and what the -F file holds, the
// file's carrying the trailing whitespace git's cleanup removes.
const (
	earlierText = "earlier\n\nbody of earlier\n"
	stdinText   = "from stdin\n"
	fileText    = "from file  \n\n\n"
)

// editorMarker is the file git's editor writes in the scratch repository, to
// show it ran; the editor leaves the message as it is.
const editorMarker = ".t327-editor-ran"

// token is one piece of git commit's arguments, written as a person would
// type it: its name, and the arguments it is, <file> standing for the -F
// file's path.
type token struct {
	name string
	args []string
}

var messageTokens = []token{
	{"-m one", []string{"-m", "one"}},
	{"-m two", []string{"-m", "two"}},
	{"-F <file>", []string{"-F", "<file>"}},
	{"-F -", []string{"-F", "-"}},
	{"--amend", []string{"--amend"}},
	{"--no-amend", []string{"--no-amend"}},
	{"--no-edit", []string{"--no-edit"}},
	{"--edit", []string{"--edit"}},
	{"-C HEAD", []string{"-C", "HEAD"}},
	{"-c HEAD", []string{"-c", "HEAD"}},
	{"--reuse-message=HEAD", []string{"--reuse-message=HEAD"}},
	{"--message=three", []string{"--message=three"}},
	{"-mfour", []string{"-mfour"}},
}

// sequences are every sequence of up to n tokens, each token as often as it
// is wanted, the empty one first.
func sequences(tokens []token, n int) [][]token {
	all := [][]token{nil}
	for from, to := 0, 1; n > 0; n-- {
		for _, prefix := range all[from:to] {
			for _, t := range tokens {
				all = append(all, append(slices.Clone(prefix), t))
			}
		}
		from, to = to, len(all)
	}
	return all
}

// nameOf is a sequence as typed.
func nameOf(seq []token) string {
	names := make([]string, len(seq))
	for i, t := range seq {
		names[i] = t.name
	}
	return strings.Join(names, " ")
}

// cleanup is git's default cleanup of a message that is not edited, which the
// comparison applies to both texts: trailing whitespace of each line removed,
// consecutive blank lines made one, and the blank lines at both ends dropped.
func cleanup(text string) string {
	var kept []string
	blank := true
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = strings.TrimRight(line, " \t\r")
		if line == "" {
			if !blank {
				kept = append(kept, "")
			}
			blank = true
			continue
		}
		kept = append(kept, line)
		blank = false
	}
	return strings.TrimRight(strings.Join(kept, "\n"), "\n")
}

// scratch is a repository of one commit, whose git runs away from the
// repository and the settings a hook hands the tests.
type scratch struct {
	dir, file, earlier string
	env                []string
}

// gitEnv is the environment git runs in: the process's without any GIT_
// variable (GIT_DIR, GIT_INDEX_FILE, GIT_CONFIG_COUNT and GIT_CONFIG_PARAMETERS
// among them), no user or system config, an author, and an editor that only
// marks that it ran.
func gitEnv(home string) []string {
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(kv), "GIT_") {
			env = append(env, kv)
		}
	}
	return append(env, "HOME="+home, "USERPROFILE="+home, "XDG_CONFIG_HOME="+home,
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		"GIT_EDITOR=echo ran > "+editorMarker+"; true")
}

// newScratch makes a repository whose one commit has earlierText, and the
// -F file.
func newScratch(t *testing.T) *scratch {
	t.Helper()
	root := t.TempDir()
	s := &scratch{dir: filepath.Join(root, "repo"), file: filepath.Join(root, "message.txt"), env: gitEnv(root)}
	if err := os.Mkdir(s.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.file, []byte(fileText), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := s.run("", "init", "-q"); err != nil {
		t.Fatalf("git init: %s", out)
	}
	if out, err := s.run("", "commit", "-q", "--allow-empty", "-m", strings.TrimSpace(earlierText)); err != nil {
		t.Fatalf("git commit: %s", out)
	}
	out, err := s.run("", "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("git rev-parse: %s", out)
	}
	s.earlier = strings.TrimSpace(out)
	return s
}

// realGit is the git the oracle runs: the one itos exports as ITOS_GIT, which
// is never itos linked as git, else the PATH's.
func realGit() string {
	if p := os.Getenv("ITOS_GIT"); p != "" {
		return p
	}
	return "git"
}

// run runs git in the repository with stdin as its stdin.
func (s *scratch) run(stdin string, args ...string) (string, error) {
	cmd := exec.Command(realGit(), args...)
	cmd.Dir, cmd.Env, cmd.Stdin = s.dir, s.env, strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// head is HEAD's message and what git did to reach it.
type head struct {
	message string
	amended bool
	edited  bool
}

// commit runs git commit with args on the repository's one commit, then puts
// the repository back; ok is false when git refuses the arguments.
func (s *scratch) commit(t *testing.T, args []string) (done head, ok bool) {
	t.Helper()
	defer func() {
		if out, err := s.run("", "reset", "-q", "--soft", s.earlier); err != nil {
			t.Fatalf("git reset: %s", out)
		}
		os.Remove(filepath.Join(s.dir, editorMarker))
	}()
	if _, err := s.run(stdinText, append([]string{"commit", "-q", "--allow-empty"}, args...)...); err != nil {
		return head{}, false
	}
	out, err := s.run("", "log", "--format=%x00%B")
	if err != nil {
		t.Fatalf("git log: %s", out)
	}
	commits := strings.Split(strings.TrimPrefix(out, "\x00"), "\x00")
	_, ran := os.Stat(filepath.Join(s.dir, editorMarker))
	return head{message: commits[0], amended: len(commits) == 1, edited: ran == nil}, true
}

// earlierCommit is the repository's commit as the VCS port shows HEAD, the
// message an amend with --no-edit starts from.
type earlierCommit struct{ vcsport.VCS }

func (earlierCommit) Show(context.Context, string) (vcsport.Entry, error) {
	return vcsport.Entry{Message: strings.TrimSpace(earlierText)}, nil
}

// disagreement is what itos's reading of seq claims that git's commit does
// not bear out, "" when it agrees; refused is whether git refused seq, which
// claims nothing.
func (s *scratch) disagreement(t *testing.T, seq []token) (wrong string, refused bool) {
	t.Helper()
	var args []string
	for _, tok := range seq {
		for _, a := range tok.args {
			if a == "<file>" {
				a = s.file
			}
			args = append(args, a)
		}
	}
	done, ok := s.commit(t, args)
	if !ok {
		return "", true
	}
	g := readGitArgs(args)
	var found []string
	if g.amend != done.amended {
		found = append(found, fmt.Sprintf("amend is %v, git amended: %v", g.amend, done.amended))
	}
	if g.noEdit && done.edited {
		found = append(found, "noEdit is true, git started the editor")
	}
	if text, ok := g.message(context.Background(), earlierCommit{}); ok {
		if cleanup(text) != cleanup(done.message) {
			found = append(found, fmt.Sprintf("message is %q, git recorded %q", text, done.message))
		}
		if len(g.messages) == 0 && g.file == "" && done.edited {
			found = append(found, "the message is a commit's, git started the editor")
		}
	}
	return strings.Join(found, "; "), false
}

// TestMessageSourceMatchesGit checks itos commit's reading of git commit's
// message arguments (readGitArgs, gitArgs.message) against git's: every
// sequence of up to three of messageTokens runs through git commit in a
// scratch repository with one earlier commit (stdin giving stdinText, -F's
// file fileText, an editor that leaves the message) and through itos's reading.
// Wherever itos gives a message, git's recorded one must be it, both after
// git's default cleanup; amend must be whether git amended, and noEdit true
// only where git started no editor; and where the message is a commit's, -C
// or -c with --no-edit, or HEAD's for an amend (no -m, no -F), git started no
// editor either. A sequence git refuses claims nothing, nor does a message
// itos cannot read. The stand-in Show gives the earlier text for any
// revision, so that the commit named is the one read is not held here
// (ID-COMMITCMD-36).
func TestMessageSourceMatchesGit(t *testing.T) {
	pool := make(chan *scratch, min(runtime.NumCPU(), 8))
	for range cap(pool) {
		pool <- newScratch(t)
	}
	var ran, refused atomic.Int64
	t.Run("sequences", func(t *testing.T) {
		for _, seq := range sequences(messageTokens, 3) {
			name := nameOf(seq)
			t.Run("["+strings.ReplaceAll(name, " ", "_")+"]", func(t *testing.T) {
				t.Parallel()
				s := <-pool
				defer func() { pool <- s }()
				wrong, no := s.disagreement(t, seq)
				if no {
					refused.Add(1)
					return
				}
				ran.Add(1)
				if wrong != "" {
					t.Errorf("git commit %s: %s", name, wrong)
				}
			})
		}
	})
	t.Logf("%d sequences compared, %d refused by git", ran.Load(), refused.Load())
}
