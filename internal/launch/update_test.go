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
)

func TestVersionOf(t *testing.T) {
	cases := map[string]string{
		"aa  itos-9.2.0-linux-amd64.tar.gz\nbb  itos.schema.json\n":     "9.2.0",
		"bb  itos.schema.json\naa *itos-1.0.0-rc.1-windows-amd64.zip\n": "1.0.0-rc.1",
		"bb  itos.schema.json\n":           "",
		"aa  itos-v9-linux-amd64.tar.gz\n": "",
	}
	for text, want := range cases {
		if got := versionOf([]byte(text)); got != want {
			t.Errorf("versionOf(%q) = %q, want %q", text, got, want)
		}
	}
}

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
	t.Setenv(EnvReleases, s.URL)
	return s, &asked
}

// at fixes the time the daily state is read and written against.
func at(t *testing.T, when time.Time) {
	t.Helper()
	old := now
	now = func() time.Time { return when }
	t.Cleanup(func() { now = old })
}

func TestAnnouncedAsksAtMostOnceADay(t *testing.T) {
	cache := offline(t)
	_, asked := latestServer(t, "9.2.0")
	start := time.Unix(1_800_000_000, 0)
	at(t, start)
	if v, sums, did := announced(cache, true); v != "9.2.0" || sums == nil || !did {
		t.Fatalf("announced = %q, %q, %t; want 9.2.0 asked", v, sums, did)
	}
	at(t, start.Add(23*time.Hour))
	if v, _, did := announced(cache, true); v != "9.2.0" || did {
		t.Errorf("announced within the day = %q, %t; want 9.2.0 remembered", v, did)
	}
	at(t, start.Add(25*time.Hour))
	if _, _, did := announced(cache, true); !did {
		t.Error("announced a day later did not ask")
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
		t.Error("a question that got no answer was asked again within the day")
	}
}

func TestNoticeOnceADayPerRepository(t *testing.T) {
	offline(t)
	t.Setenv(EnvNoUpdate, "")
	s, _ := latestServer(t, "9.2.0")
	start := time.Unix(1_800_000_000, 0)
	at(t, start)
	var b strings.Builder
	notice("one/itos.yaml", "9.1.0", &b)
	want := "itos 9.2.0 is out (this repository pins 9.1.0): " + s.URL + "/tag/v9.2.0\n"
	if b.String() != want {
		t.Fatalf("notice said %q, want %q", b.String(), want)
	}
	b.Reset()
	notice("one/itos.yaml", "9.1.0", &b)
	notice("two/itos.yaml", "9.2.0", &b)
	if b.Len() != 0 {
		t.Errorf("notice said %q again within the day, or under the newest pin", b.String())
	}
	notice("two/itos.yaml", "9.0.0", &b)
	if b.Len() == 0 {
		t.Error("another repository's pin fallen behind was not told")
	}
	b.Reset()
	at(t, start.Add(25*time.Hour))
	t.Setenv("CI", "true")
	notice("one/itos.yaml", "9.1.0", &b)
	t.Setenv("CI", "")
	t.Setenv(EnvNoUpdateNotice, "1")
	notice("one/itos.yaml", "9.1.0", &b)
	if b.Len() != 0 {
		t.Errorf("notice said %q under CI or ITOS_NO_UPDATE_NOTICE", b.String())
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
