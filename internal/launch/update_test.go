package launch

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/donvargax/itos/v7/internal/release"
)

func TestReadConfigTellsNoConfigFromNoPin(t *testing.T) {
	t.Chdir(t.TempDir())
	if c := readConfig(nil); c.state != absent {
		t.Errorf("no itos.yaml reads as %v, not absent", c.state)
	}
	writeConfig(t, "version: 1\n")
	if c := readConfig(nil); c.state != unpinned {
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
	notice("one/itos.yaml", "9.1.0", "", &b)
	want := "itos 9.2.0 is out (this repository pins 9.1.0): " + s.URL + "/tag/v9.2.0\n"
	if b.String() != want {
		t.Fatalf("notice said %q, want %q", b.String(), want)
	}
	b.Reset()
	notice("one/itos.yaml", "9.1.0", "", &b)
	notice("two/itos.yaml", "9.2.0", "", &b)
	if b.Len() != 0 {
		t.Errorf("notice said %q again within the day, or under the newest pin", b.String())
	}
	notice("two/itos.yaml", "9.0.0", "", &b)
	if b.Len() == 0 {
		t.Error("another repository's pin fallen behind was not told")
	}
	b.Reset()
	at(t, start.Add(25*time.Hour))
	t.Setenv("CI", "true")
	notice("one/itos.yaml", "9.1.0", "", &b)
	t.Setenv("CI", "")
	t.Setenv(EnvNoUpdateNotice, "1")
	notice("one/itos.yaml", "9.1.0", "", &b)
	if b.Len() != 0 {
		t.Errorf("notice said %q under CI or ITOS_NO_UPDATE_NOTICE", b.String())
	}
}

// The notice names the newest release the launcher knows of, not only the
// server's last answer (bug 48): a release the cache holds, or the itos that
// runs, newer than that answer is the one told; a pre-release or a build
// without a version is never told as out.
func TestNoticeNamesTheNewestReleaseKnown(t *testing.T) {
	cache := offline(t)
	t.Setenv(EnvNoUpdate, "1")
	at(t, time.Unix(1_800_000_000, 0))
	if err := writeState(filepath.Join(cache, "state", "latest"), "9.1.0"); err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"9.2.0", "9.4.0-rc.1"} {
		if err := store(cache, filepath.Join(cache, v), []byte("binary"), []byte("sums\n")); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct{ repo, own, want string }{
		{"one/itos.yaml", "", "itos 9.2.0 is out"},
		{"two/itos.yaml", "9.3.0", "itos 9.3.0 is out"},
		{"three/itos.yaml", "9.5.0-dev.3.gabc", "itos 9.2.0 is out"},
		{"four/itos.yaml", "(devel)", "itos 9.2.0 is out"},
	} {
		var b strings.Builder
		notice(c.repo, "9.0.0", c.own, &b)
		if !strings.HasPrefix(b.String(), c.want+" (this repository pins 9.0.0): ") {
			t.Errorf("notice with own %q said %q, want %q", c.own, b.String(), c.want)
		}
	}
}

func TestNewestRunsTheNewestStableReleaseCached(t *testing.T) {
	cache := offline(t)
	for _, v := range []string{"9.1.0", "9.3.0-rc.1", "9.2.0"} {
		if err := store(cache, filepath.Join(cache, v), []byte("binary"), []byte("sums\n")); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(cache, "9.9.9"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, ok := newest("2.0.0"); !ok || got.version != "9.2.0" {
		t.Errorf("newest = %v, %t; want 9.2.0", got, ok)
	}
	if _, ok := newest("10.0.0"); ok {
		t.Error("newest launched another release than a binary newer than the cache")
	}
	if _, ok := newest("(devel)"); ok {
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
		notice(c.repo, c.pin, "", &b)
		if told := strings.HasPrefix(b.String(), "itos 9.2.0 is out (this repository pins "+c.pin+"): "); told != c.told {
			t.Errorf("notice under the pin %s said %q, told %t, want %t", c.pin, b.String(), told, c.told)
		}
	}
}
