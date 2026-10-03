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
