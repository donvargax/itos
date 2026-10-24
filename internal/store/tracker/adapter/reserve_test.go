package adapter

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/store/tracker/port"
)

// localCounter is the ID counter's file in the project's git folder.
var localCounter = filepath.Join(".git", "itos", "ids.yaml")

// A repository Reserve cannot locate is a git failing, which no kind fits
// (exit 70), whatever kind the locate gave it (bug-119); nothing is
// reserved.
func TestReserveGivesAFailedLocateNoKind(t *testing.T) {
	files, _ := project(t, false)
	files.VCS = broken{files.VCS, "locate"}
	first, err := files.Reserve(t.Context(), "slice", 0, 1)
	var classified *kind.Error
	if first != 0 || !errors.Is(err, errBroken) || !errors.As(err, &classified) || kind.Of(err) != kind.Unknown {
		t.Errorf("Reserve: %d, %v (kind %v); want errBroken, of kind Unknown", first, err, kind.Of(err))
	}
	if _, err := os.Stat(localCounter); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a reservation that failed left the counter: %v", err)
	}
}

// A git whose status fails is not a file with changes: Ready gives its error,
// of kind Unknown (exit 70), naming the file, never a Dirty (bug-130).
func TestReadyGivesAFailedStatusNoKind(t *testing.T) {
	files := seeded(t, false)
	files.VCS = broken{files.VCS, "status"}
	err := files.Ready(t.Context(), []string{"registry.yaml"})
	var dirty *port.Dirty
	var classified *kind.Error
	if errors.As(err, &dirty) || !errors.Is(err, errBroken) || !errors.As(err, &classified) || kind.Of(err) != kind.Unknown || !strings.Contains(err.Error(), "registry.yaml") {
		t.Errorf("Ready: %v (kind %v); want errBroken naming registry.yaml, of kind Unknown, no Dirty", err, kind.Of(err))
	}
}

// The local counter is in the common dir of the folder itos runs in, the
// VCS's, wherever the process stands, and the documents are read and
// written there.
func TestFilesWorkInTheVCSsFolder(t *testing.T) {
	files, _ := project(t, true)
	dir := files.VCS.Folder(t.Context())
	t.Chdir(t.TempDir())
	if first, err := files.Reserve(t.Context(), "slice", 0, 1); first != 1 || err != nil {
		t.Fatalf("Reserve: %d, %v; want 1", first, err)
	}
	if _, err := os.Stat(filepath.Join(dir, localCounter)); err != nil {
		t.Errorf("the counter is not in the folder's common dir: %v", err)
	}
	if _, err := files.Transact(t.Context(), port.Transaction{Changes: []port.Change{{Name: "tasks/a.yaml", Text: "a\n", Created: true}}}); err != nil {
		t.Fatal(err)
	}
	docs, err := files.Load(t.Context(), []string{"tasks/a.yaml"})
	if err != nil || len(docs) != 1 || docs[0].Text != "a\n" || docs[0].Name != "tasks/a.yaml" {
		t.Errorf("Load = %+v, %v", docs, err)
	}
	if names, err := files.List(t.Context(), "tasks"); err != nil || len(names) != 1 || names[0] != "a.yaml" {
		t.Errorf("List = %q, %v", names, err)
	}
	if err := os.Mkdir(filepath.Join(dir, "folder"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := files.Load(t.Context(), []string{"folder"}); err == nil || !strings.Contains(err.Error(), " folder: ") ||
		strings.Contains(err.Error(), dir) {
		t.Errorf("Load of a folder: %v, want an error naming it as given", err)
	}
}

// The counter is on the remote the branch pushes to, never in the git
// folder; under a stealth config it is in the git folder whatever the
// branch's upstream, and the remote's is not asked.
func TestReserveOnTheUpstream(t *testing.T) {
	files, _ := project(t, false)
	bare := filepath.Join(t.TempDir(), "upstream.git")
	gitIn(t, "init", "-q", "--bare", bare)
	branch := strings.TrimSpace(gitIn(t, "branch", "--show-current"))
	gitIn(t, "remote", "add", "upstream", bare)
	gitIn(t, "config", "branch."+branch+".remote", "upstream")
	gitIn(t, "config", "branch."+branch+".merge", "refs/heads/main")
	if first, err := files.Reserve(t.Context(), "slice", 4, 2); first != 5 || err != nil {
		t.Fatalf("Reserve on the upstream: %d, %v; want 5", first, err)
	}
	if text := gitIn(t, "--git-dir", bare, "show", "refs/itos/ids:counters/slice"); text != "6\n" {
		t.Errorf("the upstream's counter reads %q; want 6", text)
	}
	if _, err := os.Stat(localCounter); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a reservation on the upstream left a local counter: %v", err)
	}
	files.Stealth = true
	if first, err := files.Reserve(t.Context(), "slice", 0, 1); first != 1 || err != nil {
		t.Fatalf("Reserve under stealth: %d, %v; want 1", first, err)
	}
	if text, err := os.ReadFile(localCounter); err != nil || !strings.Contains(string(text), "slice: 1") {
		t.Errorf("the local counter reads %q, %v", text, err)
	}
	if text := gitIn(t, "--git-dir", bare, "show", "refs/itos/ids:counters/slice"); text != "6\n" {
		t.Errorf("a reservation under stealth moved the upstream's counter to %q", text)
	}
}
