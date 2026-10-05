package release

import "testing"

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
