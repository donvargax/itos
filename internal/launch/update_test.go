package launch

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/release"
)

func TestReadConfigTellsNoConfigFromNoPin(t *testing.T) {
	t.Chdir(t.TempDir())
	if c := readConfig(standIn, nil); c.state != absent {
		t.Errorf("no itos.yaml reads as %v, not absent", c.state)
	}
	writeConfig(t, "version: 1\n")
	if c := readConfig(standIn, nil); c.state != unpinned {
		t.Errorf("a config with no pin reads as %v, not unpinned", c.state)
	}
}

// latestServer answers <base>/latest/download/checksums.txt with a release of
// the version, counting the questions.
func latestServer(t *testing.T, v string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var asked atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/latest/download/checksums.txt" {
			http.NotFound(rw, req)
			return
		}
		asked.Add(1)
		_, _ = rw.Write([]byte("aa  itos-" + v + "-linux-amd64.tar.gz\n"))
	}))
	t.Cleanup(s.Close)
	t.Setenv(release.Env, s.URL)
	return s, &asked
}

// at fixes the time the daily state is read and written against.
func at(t *testing.T, when time.Time) {
	t.Helper()
	old := now
	now = func() time.Time { return when }
	t.Cleanup(func() { now = old })
}

func TestAnnouncedAsksAtMostOnceAnHour(t *testing.T) {
	cache := offline(t)
	_, asked := latestServer(t, "9.2.0")
	start := time.Unix(1_800_000_000, 0)
	at(t, start)
	if v, sums, did := announced(cache, true); v != "9.2.0" || sums == nil || !did {
		t.Fatalf("announced = %q, %q, %t; want 9.2.0 asked", v, sums, did)
	}
	at(t, start.Add(59*time.Minute))
	if v, _, did := announced(cache, true); v != "9.2.0" || did {
		t.Errorf("announced within the hour = %q, %t; want 9.2.0 remembered", v, did)
	}
	at(t, start.Add(61*time.Minute))
	if _, _, did := announced(cache, true); !did {
		t.Error("announced an hour later did not ask")
	}
	if n := asked.Load(); n != 2 {
		t.Errorf("the server was asked %d times, not 2", n)
	}
	if _, _, did := announced(cache, false); did {
		t.Error("announced asked where it may not")
	}
}

func TestAnnouncedCountsNoAnswerAsAsked(t *testing.T) {
	cache := offline(t)
	if v, _, did := announced(cache, true); v != "" || !did {
		t.Fatalf("announced = %q, %t; want nothing, asked", v, did)
	}
	if _, _, did := announced(cache, true); did {
		t.Error("a question that got no answer was asked again within the hour")
	}
}

func TestNoticeOnceADayPerRepository(t *testing.T) {
	offline(t)
	t.Setenv(EnvNoUpdate, "")
	s, _ := latestServer(t, "9.2.0")
	start := time.Unix(1_800_000_000, 0)
	at(t, start)
	var b strings.Builder
	notice("one/itos.yaml", "9.1.0", "", &b, here)
	want := "itos 9.2.0 is out (this repository pins 9.1.0): " + s.URL + "/tag/v9.2.0 (itos upgrade moves this repository to it)\n"
	if b.String() != want {
		t.Fatalf("notice said %q, want %q", b.String(), want)
	}
	b.Reset()
	notice("one/itos.yaml", "9.1.0", "", &b, here)
	notice("two/itos.yaml", "9.2.0", "", &b, here)
	if b.Len() != 0 {
		t.Errorf("notice said %q again within the day, or under the newest pin", b.String())
	}
	notice("two/itos.yaml", "9.0.0", "", &b, here)
	if b.Len() == 0 {
		t.Error("another repository's pin fallen behind was not told")
	}
	b.Reset()
	at(t, start.Add(25*time.Hour))
	t.Setenv("CI", "true")
	notice("one/itos.yaml", "9.1.0", "", &b, here)
	t.Setenv("CI", "")
	t.Setenv(EnvNoUpdateNotice, "1")
	notice("one/itos.yaml", "9.1.0", "", &b, here)
	if b.Len() != 0 {
		t.Errorf("notice said %q under CI or ITOS_NO_UPDATE_NOTICE", b.String())
	}
}

// The once-a-day silence rests on the record of the last notice, never on
// the clock alone: a repository with no record is told, however early the
// clock reads, and then not again that day.
func TestNoticeUntoldIsToldWhateverTheClockReads(t *testing.T) {
	offline(t)
	t.Setenv(EnvNoUpdate, "1")
	at(t, time.Time{})
	var b strings.Builder
	notice("one/itos.yaml", "9.1.0", "9.2.0", &b, here)
	if !strings.HasPrefix(b.String(), "itos 9.2.0 is out (this repository pins 9.1.0): ") {
		t.Fatalf("a repository never told was not told, at the zero time: %q", b.String())
	}
	b.Reset()
	notice("one/itos.yaml", "9.1.0", "9.2.0", &b, here)
	if b.Len() != 0 {
		t.Errorf("notice said %q again within the day", b.String())
	}
}

// A notice is never an error: with no cache folder, no absolute path for the
// config or a record it cannot write, it says nothing, rather than saying it
// on every run.
func TestNoticeFailsSilent(t *testing.T) {
	cache := offline(t)
	t.Run("no cache folder", func(t *testing.T) {
		for _, name := range []string{EnvCache, "XDG_CACHE_HOME", "HOME", "home", "LocalAppData"} {
			t.Setenv(name, "")
		}
		if _, err := cacheDir(); err == nil {
			t.Fatal("cacheDir found a folder with no variable naming one")
		}
		var b strings.Builder
		notice("one/itos.yaml", "9.1.0", "9.2.0", &b, here)
		if b.Len() != 0 {
			t.Errorf("notice said %q with no cache folder", b.String())
		}
	})
	t.Run("no working directory", func(t *testing.T) {
		gone := filepath.Join(t.TempDir(), "gone")
		if err := os.Mkdir(gone, 0o755); err != nil {
			t.Fatal(err)
		}
		t.Chdir(gone)
		if err := os.Remove(gone); err != nil {
			t.Skipf("the working directory cannot be removed here: %v", err)
		}
		if _, err := filepath.Abs("itos.yaml"); err == nil {
			t.Skip("a removed working directory still has an absolute path here")
		}
		var b strings.Builder
		notice("itos.yaml", "9.1.0", "9.2.0", &b, here)
		if b.Len() != 0 {
			t.Errorf("notice said %q with no absolute path for the config", b.String())
		}
	})
	t.Run("an unwritable record", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(cache, "state"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		notice("one/itos.yaml", "9.1.0", "9.2.0", &b, here)
		if b.Len() != 0 {
			t.Errorf("notice said %q though it could not remember saying it", b.String())
		}
	})
}

// The notice names the newest release the launcher knows of, not only the
// server's last answer (bug 48): a release the cache holds, or the itos that
// runs, newer than that answer is the one told, and one older is not; a
// pre-release or a build without a version is never told as out.
func TestNoticeNamesTheNewestReleaseKnown(t *testing.T) {
	cache := offline(t)
	t.Setenv(EnvNoUpdate, "1")
	at(t, time.Unix(1_800_000_000, 0))
	if err := writeState(filepath.Join(cache, "state", "latest"), "9.1.0"); err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"9.2.0", "9.4.0-rc.1"} {
		if err := store(cache, filepath.Join(cache, v), []byte("binary"), []byte("sums\n"), here); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct{ repo, own, want string }{
		{"one/itos.yaml", "", "itos 9.2.0 is out"},
		{"two/itos.yaml", "9.3.0", "itos 9.3.0 is out"},
		{"three/itos.yaml", "9.5.0-dev.3.gabc", "itos 9.2.0 is out"},
		{"four/itos.yaml", "(devel)", "itos 9.2.0 is out"},
		{"five/itos.yaml", "9.0.5", "itos 9.2.0 is out"},
	} {
		var b strings.Builder
		notice(c.repo, "9.0.0", c.own, &b, here)
		if !strings.HasPrefix(b.String(), c.want+" (this repository pins 9.0.0): ") {
			t.Errorf("notice with own %q said %q, want %q", c.own, b.String(), c.want)
		}
	}
}

func TestNewestRunsTheNewestStableReleaseCached(t *testing.T) {
	cache := offline(t)
	for _, v := range []string{"9.1.0", "9.3.0-rc.1", "9.2.0"} {
		if err := store(cache, filepath.Join(cache, v), []byte("binary"), []byte("sums\n"), here); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(cache, "9.9.9"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, ok := newest("2.0.0", here); !ok || got.version != "9.2.0" {
		t.Errorf("newest = %v, %t; want 9.2.0", got, ok)
	}
	if _, ok := newest("10.0.0", here); ok {
		t.Error("newest launched another release than a binary newer than the cache")
	}
	if _, ok := newest("(devel)", here); ok {
		t.Error("newest launched another release than a build without a version")
	}
}

// A pin on a release candidate is behind its final release and ahead of the
// release before it (bug 50): the notice reads the order version.Compare
// gives, a pre-release below its release.
func TestNoticeTellsAReleaseCandidatesPinOfItsRelease(t *testing.T) {
	cache := offline(t)
	t.Setenv(EnvNoUpdate, "1")
	at(t, time.Unix(1_800_000_000, 0))
	if err := writeState(filepath.Join(cache, "state", "latest"), "9.2.0"); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		repo, pin string
		told      bool
	}{
		{"one/itos.yaml", "9.2.0-rc.1", true},
		{"two/itos.yaml", "9.3.0-rc.1", false},
	} {
		var b strings.Builder
		notice(c.repo, c.pin, "", &b, here)
		if told := strings.HasPrefix(b.String(), "itos 9.2.0 is out (this repository pins "+c.pin+"): "); told != c.told {
			t.Errorf("notice under the pin %s said %q, told %t, want %t", c.pin, b.String(), told, c.told)
		}
	}
}

// An upgrade to a version newer than this binary is handed to it (slice
// 126): a version it cannot have or fetch is said in one line, as upgrade
// says it, and exits 75 or 3; one no newer, an ITOS_UPGRADE_FROM already set
// and another command are left to this binary. What the arguments name is
// the command line's (CLI.UpgradeTarget), here a stand-in's answer.
func TestHandUpgrade(t *testing.T) {
	offline(t)
	t.Setenv(EnvUpgradeFrom, "")
	try := func(name string, h Handoff, ok bool, err error, wantCode int, wantHanded bool, says string) {
		t.Helper()
		asked := 0
		c := standIn
		c.UpgradeTarget = func(args []string) (Handoff, bool, error) {
			asked++
			if !slices.Equal(args, []string{"upgrade"}) {
				t.Errorf("%s: UpgradeTarget(%q)", name, args)
			}
			return h, ok, err
		}
		var stderr strings.Builder
		code, handed := handUpgrade(c, []string{"upgrade"}, &stderr, here)
		if code != wantCode || handed != wantHanded || !strings.Contains(stderr.String(), says) ||
			says == "" && stderr.Len() > 0 {
			t.Errorf("%s: %d, %t, %q; want %d, %t, saying %q", name, code, handed, stderr.String(), wantCode, wantHanded, says)
		}
		if want := map[bool]int{false: 1, true: 0}[os.Getenv(EnvUpgradeFrom) != ""]; asked != want {
			t.Errorf("%s: UpgradeTarget asked %d times, not %d", name, asked, want)
		}
	}
	try("another command", Handoff{}, false, nil, 0, false, "")
	unanswered := kind.Wrap(kind.Temporary, errors.New("cannot ask for the newest itos"))
	try("no server", Handoff{}, true, unanswered, 75, true, "itos: cannot ask for the newest itos; every file is left as it was\n")
	c := standIn
	c.UpgradeTarget = func([]string) (Handoff, bool, error) { return Handoff{}, true, unanswered }
	if code := Main(c, []string{"upgrade"}, io.Discard, here); code == nil || *code != 75 {
		t.Error("Main handed no run, or not with exit 75")
	}
	try("a version no newer than this binary", Handoff{Version: "0.0.0"}, true, nil, 0, false, "")
	latestServer(t, "99.0.0")
	newer := Handoff{Version: "99.0.0", Checksums: []byte("sums\n"), Args: []string{"upgrade", "99.0.0"}}
	try("a newer version that cannot be fetched", newer, true, nil, 3, true, "itos: cannot fetch itos 99.0.0")
	t.Setenv(EnvUpgradeFrom, "9.1.0")
	try("an upgrade handed off already", newer, true, nil, 0, false, "")
}
