package followup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/donvargax/itos/v8/internal/slices/followup/domain"
	"github.com/donvargax/itos/v8/internal/store/private/port"
	"github.com/donvargax/itos/v8/internal/store/private/port/porttest"
)

var at = time.Date(2026, 10, 4, 9, 30, 0, 0, time.UTC)

// ports is followup's ports over the store's fake and a stopped clock.
type ports struct{ *porttest.Store }

func (ports) Now(context.Context) time.Time { return at }

// Folder is the process's folder, which the tests stand in.
func (ports) Folder(context.Context) string { return "" }

// in is the ports run in the folder dir.
type in struct {
	ports
	dir string
}

func (p in) Folder(context.Context) string { return p.dir }

// faulty is the fake store failing as set: a Path, Read or Write that fails,
// or a lock that cannot be given back.
type faulty struct {
	ports
	path, read, write, release error
}

func (f faulty) Path(ctx context.Context, name string) (string, error) {
	if f.path != nil {
		return "", f.path
	}
	return f.ports.Path(ctx, name)
}

func (f faulty) Read(ctx context.Context, name string) ([]byte, bool, error) {
	if f.read != nil {
		return nil, false, f.read
	}
	return f.ports.Read(ctx, name)
}

func (f faulty) Write(ctx context.Context, name string, text []byte) error {
	if f.write != nil {
		return f.write
	}
	return f.ports.Write(ctx, name, text)
}

func (f faulty) Hold(ctx context.Context, name string) (port.Held, error) {
	held, err := f.ports.Hold(ctx, name)
	if err != nil || f.release == nil {
		return held, err
	}
	return stuck{f.release}, nil
}

// stuck is a lock that cannot be given back.
type stuck struct{ err error }

func (s stuck) Release(context.Context) error { return s.err }

// store is a fake store over a clone at a new folder, whose work tree is
// that folder.
func store(t *testing.T) *porttest.Store {
	t.Helper()
	return &porttest.Store{Folder: filepath.Join(t.TempDir(), ".git", "itos")}
}

// followup runs the line over p, as itos would with --json when json.
func followup(t *testing.T, p Ports, json bool, args ...string) (int, string, string, error) {
	t.Helper()
	c, err := Parse(args, "", json, false)
	if err != nil {
		t.Fatalf("followup %s: %v", strings.Join(args, " "), err)
	}
	var stdout, stderr strings.Builder
	code, err := c.Run(context.Background(), &stdout, &stderr, NewHandler(p))
	return code, stdout.String(), stderr.String(), err
}

func TestParse(t *testing.T) {
	for _, c := range []struct {
		args []string
		q    Query
	}{
		{nil, Query{}},
		{[]string{"--all"}, Query{All: true}},
		{[]string{"add", "a", "--with", "ana", "--title=T", "--note", "N"}, Query{Sub: "add", ID: "a", With: "ana", Title: "T", Note: "N"}},
		{[]string{"note", "a", "one", "two"}, Query{Sub: "note", ID: "a", Note: "one two"}},
		{[]string{"note", "a", "--", "-x"}, Query{Sub: "note", ID: "a", Note: "-x"}},
		{[]string{"close", "a", "--note", "done"}, Query{Sub: "close", ID: "a", Note: "done"}},
		{[]string{"show", "a"}, Query{Sub: "show", ID: "a"}},
		{[]string{"doc", "a", "-"}, Query{Sub: "doc", ID: "a", Path: "-", Typed: "-"}},
		{[]string{"doc", "a", "n.md", "--force"}, Query{Sub: "doc", ID: "a", Path: "n.md", Typed: filepath.Join("sub", "n.md"), Force: true}},
	} {
		got, err := Parse(c.args, "sub", true, true)
		if err != nil || got != (CLI{c.q, true, true}) {
			t.Errorf("%q: %+v, %v", c.args, got, err)
		}
	}
	abs := filepath.Join(t.TempDir(), "n.md")
	if got, err := Parse([]string{"doc", "a", abs}, "sub", false, false); err != nil || got.Typed != abs {
		t.Errorf("an absolute path: %+v, %v", got, err)
	}
	if got, err := Parse([]string{"doc", "a", "n.md"}, "", false, false); err != nil || got.Typed != "n.md" {
		t.Errorf("no origin: %+v, %v", got, err)
	}
}

func TestParseRefuses(t *testing.T) {
	for line, want := range map[string]string{
		"bogus":                   "followup has no subcommand bogus: " + takes,
		"--all --all":             "followup takes one --all",
		"--bogus":                 "followup does not take --bogus",
		"add":                     "followup add needs <id> --with <who> --title <title> --note <text>",
		"add -x":                  "followup add does not take -x",
		"add a b":                 "followup add takes one <id>, not b",
		"add a --with":            "followup add --with needs a value",
		"add a --with x --with y": "followup add takes one --with with a value",
		"add a --with=":           "followup add takes one --with with a value",
		"add a --with x":          "followup add needs --with <who>, --title <title> and --note <text>",
		"add _a":                  `followup add: "_a" is no thread id (letters, digits, '.', '_' and '-', a letter or digit first)`,
		"note":                    "followup note needs <id> <text>",
		"note a":                  "followup note needs <id> <text>",
		"note a --x":              "followup note does not take --x",
		"close a b":               "followup close takes one <id>, not b (a last note goes in --note)",
		"close a --x":             "followup close does not take --x",
		"close":                   "followup close needs <id>",
		"show a b":                "followup show takes one <id>, not b",
		"show a --x":              "followup show does not take --x",
		"show":                    "followup show needs <id>",
		"doc a":                   "followup doc needs <id> <path>",
		"doc a x y":               "followup doc needs <id> <path>",
		"doc a x --force --force": "followup doc takes one --force",
		"doc":                     "followup doc needs <id> <path>",
	} {
		if _, err := Parse(strings.Fields(line), "", false, false); err == nil || err.Error() != want {
			t.Errorf("followup %s: %v, want %q", line, err, want)
		}
	}
	if _, err := Parse([]string{"doc", "a", ""}, "", false, false); err == nil || err.Error() != "followup doc needs <id> <path>" {
		t.Errorf("an empty path: %v", err)
	}
}

// utc reads every time in UTC, as TZ=UTC would.
func utc(t *testing.T) {
	was := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = was })
}

// A thread's life: opened, noted, listed, shown, closed, listed again.
func TestRunKeepsAThread(t *testing.T) {
	utc(t)
	p := ports{store(t)}
	for _, r := range []struct {
		args           []string
		json           bool
		stdout, stderr string
		code           int
	}{
		{nil, false, "No open threads.\n", "", 0},
		{[]string{"--all"}, false, "No threads.\n", "", 0},
		{nil, true, "\"threads\": []\n}\n", "", 0},
		{[]string{"add", "sync-ana", "--with", "ana", "--title", "The sync design", "--note", "Started."}, false, "sync-ana opened, with ana: The sync design\n", "", 0},
		{[]string{"add", "sync-ana", "--with", "bo", "--title", "T", "--note", "N"}, false, "", "sync-ana is already a thread, with ana: The sync design\n  fix: itos followup note sync-ana adds to it; another id opens a new one\n", 1},
		{[]string{"add", "sync-ana", "--with", "bo", "--title", "T", "--note", "N"}, true, `"rule": "follow-id-taken"`, "", 1},
		{[]string{"note", "sync-ana", "Two", "lines\nhere"}, false, "sync-ana: note 2 added\n", "", 0},
		{[]string{"note", "nobody", "x"}, false, "", "there is no thread nobody\n  fix: itos followup --all lists the threads; itos followup add nobody opens one\n", 1},
		{[]string{"add", "bo", "--with", "bo", "--title", "Flaky", "--note", "N"}, true, `"status": "open"`, "", 0},
		{nil, false, "Open threads:\n  sync-ana  The sync design  (ana, 2026-10-04 09:30)\n  bo        Flaky  (bo, 2026-10-04 09:30)\n", "", 0},
		{[]string{"show", "sync-ana"}, false, "sync-ana: The sync design\nwith ana, open\n\n2026-10-04 09:30  Started.\n2026-10-04 09:30  Two lines\n                  here\n", "", 0},
		{[]string{"show", "sync-ana"}, true, `"with": "ana"`, "", 0},
		{[]string{"show", "nobody"}, false, "", "there is no thread nobody\n", 1},
		{[]string{"close", "bo", "--note", "Fixed."}, false, "bo closed: Flaky\n", "", 0},
		{[]string{"close", "bo"}, false, "", "bo is already closed, since 2026-10-04 09:30\n  fix: itos followup note bo adds to it\n", 1},
		{[]string{"close", "nobody"}, false, "", "there is no thread nobody\n", 1},
		{nil, false, "Open threads:\n  sync-ana  The sync design  (ana, 2026-10-04 09:30)\n", "", 0},
		{[]string{"--all"}, false, "Open threads:\n  sync-ana  The sync design  (ana, 2026-10-04 09:30)\n\nClosed threads:\n  bo        Flaky  (bo, 2026-10-04 09:30)\n", "", 0},
		{[]string{"--all"}, true, `"last": "2026-10-04T09:30:00Z",` + "\n      \"notes\": 2", "", 0},
		{[]string{"show", "bo"}, false, "bo: Flaky\nwith bo, closed 2026-10-04 09:30\n\n", "", 0},
		{[]string{"close", "sync-ana"}, false, "sync-ana closed: The sync design\n", "", 0},
		{nil, false, "No open threads.\n", "", 0},
		{[]string{"--all"}, false, "No open threads.\n\nClosed threads:\n", "", 0},
	} {
		code, stdout, stderr, err := followup(t, p, r.json, r.args...)
		if err != nil || code != r.code || !strings.Contains(stdout, r.stdout) || (r.stdout == "") != (stdout == "") || !strings.Contains(stderr, r.stderr) || (r.stderr == "") != (stderr == "") {
			t.Errorf("followup %q: exit %d, %v\nstdout %q\nstderr %q", r.args, code, err, stdout, stderr)
		}
	}
	text, _, _ := p.Read(context.Background(), domain.FileName)
	if f, err := domain.Parse(domain.FileName, text); err != nil || len(f.Threads) != 2 || f.Threads[1].Closed == "" {
		t.Errorf("the threads written: %+v, %v", f, err)
	}
}

// --quiet says nothing of a change.
func TestRunQuiet(t *testing.T) {
	c, err := Parse([]string{"add", "a", "--with", "x", "--title", "t", "--note", "n"}, "", false, true)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	if code, err := c.Run(context.Background(), &stdout, &stderr, NewHandler(ports{store(t)})); code != 0 || err != nil || stdout.Len()+stderr.Len() != 0 {
		t.Errorf("exit %d, %v, %q, %q", code, err, stdout.String(), stderr.String())
	}
}

// followup doc writes the thread where the person typed, private, or
// prints it with -.
func TestRunDoc(t *testing.T) {
	utc(t)
	s := store(t)
	p := ports{s}
	if code, _, _, err := followup(t, p, false, "add", "a", "--with", "ana", "--title", "T", "--note", "Started."); code != 0 || err != nil {
		t.Fatal(code, err)
	}
	markdown := "# T\n\nA thread with ana, open (itos followup show a).\n\n## 2026-10-04 09:30\n\nStarted.\n"
	if code, stdout, _, err := followup(t, p, false, "doc", "a", "-"); code != 0 || err != nil || stdout != markdown {
		t.Errorf("doc -: exit %d, %v, %q", code, err, stdout)
	}
	if code, stdout, _, err := followup(t, p, true, "doc", "a", "-"); code != 0 || err != nil || !strings.Contains(stdout, `"markdown": "# T`) {
		t.Errorf("--json doc -: exit %d, %v, %q", code, err, stdout)
	}
	if code, _, stderr, _ := followup(t, p, false, "doc", "nobody", "-"); code != 1 || !strings.Contains(stderr, "there is no thread nobody") {
		t.Errorf("doc nobody -: exit %d, %q", code, stderr)
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "notes", "a.md")
	if code, stdout, stderr, err := followup(t, p, false, "doc", "a", target); code != 0 || err != nil || stdout != "a written to "+target+"\n" || stderr != "" {
		t.Errorf("doc: exit %d, %v, %q, %q", code, err, stdout, stderr)
	}
	if text, err := os.ReadFile(target); err != nil || string(text) != markdown {
		t.Errorf("the document: %q, %v", text, err)
	}
	if info, err := os.Stat(target); runtime.GOOS != "windows" && (err != nil || info.Mode().Perm() != 0o600) {
		t.Errorf("the document's mode: %v", err)
	}
	if code, _, stderr, _ := followup(t, p, false, "doc", "a", target); code != 1 || stderr != target+" is already there; nothing written\n  fix: --force writes over it\n" {
		t.Errorf("doc over a file: exit %d, %q", code, stderr)
	}
	if code, _, stderr, _ := followup(t, p, false, "doc", "a", dir); code != 1 || !strings.Contains(stderr, dir+" is a folder\n  fix: name the file to write, "+filepath.Join(dir, "a.md")+" say") {
		t.Errorf("doc to a folder: exit %d, %q", code, stderr)
	}
	s.Exposed = []string{target}
	if code, stdout, stderr, err := followup(t, p, true, "doc", "a", target, "--force"); code != 0 || err != nil ||
		!strings.Contains(stdout, `"path": `) || !strings.HasPrefix(stderr, "itos: warning: "+target+" is in the work tree and not ignored by git") {
		t.Errorf("doc --force where git add could commit it: exit %d, %v, %q, %q", code, err, stdout, stderr)
	}
	if code, stdout, _, _ := followup(t, p, false, "show", "a"); code != 0 || !strings.HasSuffix(stdout, "\nWritten out to "+target+"\n") {
		t.Errorf("show after doc: %q", stdout)
	}
	if _, _, _, err := followup(t, p, false, "doc", "a", filepath.Join(target, "under-a-file.md")); err == nil {
		t.Error("a path under a file was written")
	}
	locked := filepath.Join(t.TempDir(), "locked")
	if err := os.Mkdir(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		return
	}
	if _, _, _, err := followup(t, p, false, "doc", "a", filepath.Join(locked, "a.md")); err == nil {
		t.Error("a file was written in a folder no one may write in")
	}
}

// A relative doc path is read from the folder itos runs in, wherever the
// process stands.
func TestRunDocInTheFolderItosRunsIn(t *testing.T) {
	dir := t.TempDir()
	p := in{ports{store(t)}, dir}
	if code, _, _, err := followup(t, p, false, "add", "a", "--with", "ana", "--title", "T", "--note", "N"); code != 0 || err != nil {
		t.Fatal(code, err)
	}
	t.Chdir(t.TempDir())
	if code, _, _, err := followup(t, p, false, "doc", "a", "a.md"); code != 0 || err != nil {
		t.Fatal(code, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.md")); err != nil {
		t.Errorf("the doc is not in the folder itos runs in: %v", err)
	}
}

// No repository is exit 3 and a line saying so.
func TestRunWithNoRepository(t *testing.T) {
	for _, args := range [][]string{nil, {"add", "a", "--with", "x", "--title", "t", "--note", "n"}} {
		code, stdout, stderr, err := followup(t, ports{&porttest.Store{}}, false, args...)
		if code != 3 || err != nil || stdout != "" || stderr != "itos: followup keeps its threads in the git folder, and this is no git repository\n" {
			t.Errorf("followup %q: exit %d, %v, %q, %q", args, code, err, stdout, stderr)
		}
	}
}

// What the store fails at is the command line's error, a file the domain
// refuses Invalid; a lock that cannot be given back is a line after all
// else, and a lock held is not waited past.
func TestRunSaysTheStoresFailures(t *testing.T) {
	broken := errors.New("broken")
	add := []string{"add", "a", "--with", "x", "--title", "t", "--note", "n"}
	for name, p := range map[string]Ports{
		"path":  faulty{ports: ports{store(t)}, path: broken},
		"read":  faulty{ports: ports{store(t)}, read: broken},
		"write": faulty{ports: ports{store(t)}, write: broken},
	} {
		if _, _, _, err := followup(t, p, false, add...); !errors.Is(err, broken) {
			t.Errorf("%s: %v", name, err)
		}
	}
	s := store(t)
	if err := s.Write(context.Background(), domain.FileName, []byte("threads: [")); err != nil {
		t.Fatal(err)
	}
	var invalid *domain.Invalid
	if _, _, _, err := followup(t, ports{s}, false); !errors.As(err, &invalid) {
		t.Errorf("an invalid file: %v", err)
	}
	held, err := s.Hold(context.Background(), domain.FileName)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release(context.Background())
	if _, _, _, err := followup(t, ports{s}, false, add...); err == nil || !strings.Contains(err.Error(), ".lock") {
		t.Errorf("a held lock: %v", err)
	}
	code, stdout, stderr, err := followup(t, faulty{ports: ports{store(t)}, release: broken}, false, add...)
	if code != 0 || err != nil || stdout != "a opened, with x: t\n" || stderr != "itos: the lock cannot be given back: broken\n" {
		t.Errorf("a lock not given back: exit %d, %v, %q, %q", code, err, stdout, stderr)
	}
	utc(t)
	exposed := store(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "a.md")
	exposed.Exposed = []string{target}
	if _, _, _, err := followup(t, ports{exposed}, false, add...); err != nil {
		t.Fatal(err)
	}
	code, _, stderr, err = followup(t, faulty{ports: ports{exposed}, write: broken}, false, "doc", "a", target)
	if !errors.Is(err, broken) || !strings.HasPrefix(stderr, "itos: warning: "+target) {
		t.Errorf("a doc whose thread cannot be saved: exit %d, %v, %q", code, err, stderr)
	}
}
