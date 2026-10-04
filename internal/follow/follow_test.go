package follow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var at = time.Date(2026, 10, 4, 9, 30, 15, 0, time.FixedZone("", -7*3600))

func TestSaveThenLoadKeepsEveryThread(t *testing.T) {
	path := filepath.Join(t.TempDir(), "itos", FileName)
	var f File
	f.Add("sync-ana", "ana", "The sync design", "She covered\nthe retries.", at)
	b := f.Add("flaky-bo", "bo", "The flaky login test", "Started.", at)
	b.Close("Fixed: 2 retries.", at.Add(time.Hour))
	b.Wrote("/notes/flaky.md")
	b.Wrote("/notes/flaky.md")
	if err := Save(path, f); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("the file's mode is %v, not 0600", info.Mode().Perm())
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Threads) != 2 {
		t.Fatalf("%d threads read back, not 2", len(got.Threads))
	}
	a, c := got.Find("sync-ana"), got.Find("flaky-bo")
	if a.Notes[0].Text != "She covered\nthe retries." || a.Status != Open || a.Opened() != "2026-10-04T09:30:15-07:00" {
		t.Errorf("sync-ana read back as %+v", *a)
	}
	if c.Status != Closed || c.Closed != "2026-10-04T10:30:15-07:00" || len(c.Notes) != 2 || len(c.Docs) != 1 {
		t.Errorf("flaky-bo read back as %+v", *c)
	}
	if got.Find("nobody") != nil {
		t.Error("an id no thread has was found")
	}
}

func TestLoadReadsNoFileAndAnEmptyOneAsNoThreads(t *testing.T) {
	dir := t.TempDir()
	for _, path := range []string{filepath.Join(dir, "missing.yaml"), filepath.Join(dir, "empty.yaml")} {
		if strings.HasSuffix(path, "empty.yaml") {
			if err := os.WriteFile(path, nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		f, err := Load(path)
		if err != nil || len(f.Threads) != 0 {
			t.Errorf("%s: %v, %d threads", path, err, len(f.Threads))
		}
	}
}

func TestLoadRefusesAFileItosDoesNotWrite(t *testing.T) {
	for text, want := range map[string]string{
		"threads:\n  - { id: a, with: x, title: t, status: open, notes: [], colour: red }\n":                         "colour",
		"threads:\n  - { id: a, with: x, title: t, status: open }\n  - { id: a, with: y, title: u, status: open }\n": "two threads have the id a",
		"threads:\n  - { id: a, with: x, title: t, status: done }\n":                                                 `"done", not open or closed`,
		"threads:\n  - { id: 'a b', with: x, title: t, status: open }\n":                                             "no id",
	} {
		path := filepath.Join(t.TempDir(), FileName)
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v, not an error saying %q", text, err, want)
		}
	}
}

func TestMarkdownIsTheWholeThreadInOrder(t *testing.T) {
	var f File
	th := f.Add("sync-ana", "ana", "The sync design", "She covered the retries.", at)
	th.Note("Backoff agreed.", at.Add(2*time.Hour))
	th.Close("", at.Add(3*time.Hour))
	want := "# The sync design\n\nA thread with ana, closed 2026-10-04 12:30 (itos follow show sync-ana).\n\n" +
		"## 2026-10-04 09:30\n\nShe covered the retries.\n\n## 2026-10-04 11:30\n\nBackoff agreed.\n"
	if got := Markdown(*th); got != want {
		t.Errorf("Markdown:\n%s\nwant:\n%s", got, want)
	}
}

func TestShowKeepsATimeItCannotRead(t *testing.T) {
	if got := Show("last Tuesday"); got != "last Tuesday" {
		t.Errorf("Show gave %q", got)
	}
}

func TestValidID(t *testing.T) {
	for id, want := range map[string]bool{"sync-ana": true, "a.b_c": true, "9": true, "-a": false, "a b": false, "": false} {
		if ValidID(id) != want {
			t.Errorf("ValidID(%q) is %v", id, !want)
		}
	}
}
