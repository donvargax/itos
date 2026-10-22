package adapter

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/kind"
	vcs "github.com/donvargax/itos/v8/internal/vcs/port"
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

// located is the VCS with Locate giving the common dir it holds.
type located struct {
	vcs.VCS
	common string
}

func (l located) Locate(context.Context, string) (vcs.Location, error) {
	return vcs.Location{Common: l.common}, nil
}

// A folder gone from under itos stops the reservation: the common dir
// cannot be made absolute, or the folder cannot be read.
func TestReserveWhereTheFolderIsGone(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows cannot remove the folder a process stands in")
	}
	for _, common := range []string{".git", t.TempDir()} {
		files, _ := project(t, false)
		files.VCS = located{files.VCS, common}
		gone := filepath.Join(t.TempDir(), "gone")
		if err := os.Mkdir(gone, 0o755); err != nil {
			t.Fatal(err)
		}
		t.Chdir(gone)
		if err := os.Remove(gone); err != nil {
			t.Fatal(err)
		}
		if first, err := files.Reserve(t.Context(), "slice", 0, 1); first != 0 || err == nil {
			t.Errorf("%s: Reserve in a folder gone: %d, %v", common, first, err)
		}
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
