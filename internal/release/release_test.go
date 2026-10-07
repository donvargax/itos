package release

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/donvargax/itos/v7/internal/kind"
)

func TestVersionOf(t *testing.T) {
	cases := map[string]string{
		"aa  itos-9.2.0-linux-amd64.tar.gz\nbb  itos.schema.json\n":     "9.2.0",
		"bb  itos.schema.json\naa *itos-1.0.0-rc.1-windows-amd64.zip\n": "1.0.0-rc.1",
		"bb  itos.schema.json\n":           "",
		"aa  itos-v9-linux-amd64.tar.gz\n": "",
	}
	for text, want := range cases {
		if got := VersionOf([]byte(text)); got != want {
			t.Errorf("VersionOf(%q) = %q, want %q", text, got, want)
		}
	}
}

func TestURLs(t *testing.T) {
	t.Setenv(Env, "http://releases.test/base/")
	if got := URL("9.1.0", "checksums.txt"); got != "http://releases.test/base/download/v9.1.0/checksums.txt" {
		t.Errorf("URL = %s", got)
	}
	if got := LatestURL("checksums.txt"); got != "http://releases.test/base/latest/download/checksums.txt" {
		t.Errorf("LatestURL = %s", got)
	}
	if got := NotesURL("9.1.0"); got != "http://releases.test/base/tag/v9.1.0" {
		t.Errorf("NotesURL = %s", got)
	}
	t.Setenv(Env, "")
	if got := Base(); got != Default {
		t.Errorf("Base with no %s = %s", Env, got)
	}
}

func TestListed(t *testing.T) {
	text := []byte("AA  itos-1.0.0-linux-amd64.tar.gz\nbb *itos-1.0.0-windows-amd64.zip\ncc  itos.schema.json\n")
	cases := []struct {
		asset, want string
		ok          bool
	}{
		{"itos-1.0.0-linux-amd64.tar.gz", "aa", true},
		{"itos-1.0.0-windows-amd64.zip", "bb", true},
		{"itos-1.0.0-darwin-arm64.tar.gz", "", false},
	}
	for _, c := range cases {
		got, ok := Listed(text, c.asset)
		if got != c.want || ok != c.ok {
			t.Errorf("Listed(%q) = %q, %t; want %q, %t", c.asset, got, ok, c.want, c.ok)
		}
	}
}

// A server that fails or limits the rate may answer when asked again
// (kind.Temporary, exit 75), as may one that cannot be reached; an address
// it has nothing at is a release that is not there (kind.Missing, exit 3).
func TestGetGivesEachFailureItsKind(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/busy":
			w.WriteHeader(http.StatusServiceUnavailable)
		case "/limited":
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			http.NotFound(w, r)
		}
	}))
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	defer server.Close()
	for url, want := range map[string]kind.Kind{
		server.URL + "/busy":    kind.Temporary,
		server.URL + "/limited": kind.Temporary,
		server.URL + "/none":    kind.Missing,
		closed.URL + "/any":     kind.Temporary,
	} {
		if _, err := Get(url, 5*time.Second); kind.Of(err) != want {
			t.Errorf("Get(%s): %v, kind %d, want %d", url, err, kind.Of(err), want)
		}
	}
	if _, err := Get(server.URL+"/none", 5*time.Second); !NotFound(err) {
		t.Errorf("a 404 is not NotFound: %v", err)
	}
}
