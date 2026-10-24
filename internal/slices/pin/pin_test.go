package pin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pinengine "github.com/donvargax/itos/v8/internal/engine/pin"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/release"
)

// sums is the release's checksums.txt, and sum its SHA-256.
const sums = "aa  itos-9.2.0-linux-amd64.tar.gz\n"

var sum = release.SHA256([]byte(sums))

// old is checksums a pin holds that are not the release's.
var old = strings.Repeat("1a", 32)

// ports is pin's ports over a config file and fixed answers: the release
// they have, or the failure to ask for it, and the version last asked for.
type ports struct {
	file, text string
	broken     error
	version    string
	fetch      error
	asked      *string
}

func (p ports) Config(context.Context) (pinengine.Config, error) {
	if p.broken != nil {
		return pinengine.Config{}, p.broken
	}
	if err := os.WriteFile(p.file, []byte(p.text), 0o644); err != nil {
		return pinengine.Config{}, err
	}
	return pinengine.ReadConfig("", p.file)
}

func (p ports) Release(_ context.Context, want string) (string, []byte, error) {
	if p.asked != nil {
		*p.asked = want
	}
	if p.fetch != nil {
		return "", nil, p.fetch
	}
	return p.version, []byte(sums), nil
}

// pinning is a config pinning v with the checksums.
func pinning(v, checksums string) string {
	return "version: 1\npin:\n  version: " + v + " # kept\n  checksums: " + checksums + "\n"
}

// failing is a writer that cannot be written to.
type failing struct{}

func (failing) Write([]byte) (int, error) { return 0, errors.New("closed") }

func TestParse(t *testing.T) {
	if c, err := Parse([]string{"v9.2.0"}, true); err != nil || c != (CLI{Query: Query{Want: "9.2.0"}, JSON: true}) {
		t.Errorf("v9.2.0 --json: %+v, %v", c, err)
	}
	if c, err := Parse([]string{"latest"}, false); err != nil || c != (CLI{}) {
		t.Errorf("latest: %+v, %v", c, err)
	}
	var u out.UsageError
	if _, err := Parse([]string{"--force"}, false); !errors.As(err, &u) || err.Error() != "pin takes only a <version> (--force)" {
		t.Errorf("--force: %v", err)
	}
}

func TestRun(t *testing.T) {
	t.Setenv(release.Env, "https://releases.test")
	notes := "https://releases.test/tag/v9.2.0"
	for _, r := range []struct {
		name           string
		c              CLI
		text           string
		code           int
		stdout, stderr string
		after          string
	}{
		{"moved", CLI{}, pinning("9.1.0", old), 0,
			"Pinned itos 9.2.0 in FILE, from 9.1.0.\nRead its notes, their Upgrading section above all, before you commit the change: " + notes + "\n",
			"", pinning("9.2.0", sum)},
		{"added", CLI{}, "version: 1\n", 0,
			"Pinned itos 9.2.0 in FILE, which pinned none.\nRead its notes, their Upgrading section above all, before you commit the change: " + notes + "\n",
			"", "version: 1\npin:\n  version: 9.2.0\n  checksums: " + sum + "\n"},
		{"moved --json", CLI{JSON: true}, pinning("9.1.0", old), 0,
			"{\n  \"schema\": 1,\n  \"config\": \"FILE\",\n  \"action\": \"pinned\",\n  \"version\": \"9.2.0\",\n  \"checksums\": \"" + sum +
				"\",\n  \"previous\": \"9.1.0\",\n  \"notes\": \"" + notes + "\"\n}\n",
			"", pinning("9.2.0", sum)},
		{"added --json", CLI{JSON: true}, "version: 1\n", 0,
			"{\n  \"schema\": 1,\n  \"config\": \"FILE\",\n  \"action\": \"pinned\",\n  \"version\": \"9.2.0\",\n  \"checksums\": \"" + sum +
				"\",\n  \"notes\": \"" + notes + "\"\n}\n",
			"", "version: 1\npin:\n  version: 9.2.0\n  checksums: " + sum + "\n"},
		{"the version with no valid checksums", CLI{}, pinning("9.2.0", "none"), 0,
			"Pinned itos 9.2.0 in FILE, from 9.2.0.\nRead its notes, their Upgrading section above all, before you commit the change: " + notes + "\n",
			"", pinning("9.2.0", sum)},
		{"already", CLI{}, pinning("9.2.0", sum), 0, "FILE already pins itos 9.2.0: nothing to change.\n", "", pinning("9.2.0", sum)},
		{"already --json", CLI{JSON: true}, pinning("9.2.0", sum), 0,
			"{\n  \"schema\": 1,\n  \"config\": \"FILE\",\n  \"action\": \"already\",\n  \"version\": \"9.2.0\",\n  \"checksums\": \"" + sum +
				"\",\n  \"previous\": \"9.2.0\"\n}\n",
			"", pinning("9.2.0", sum)},
		{"refused", CLI{}, pinning("9.2.0", old), 1, "",
			"itos: FILE pins itos 9.2.0 with the checksums " + old + ", but its checksums.txt is " + sum +
				" now: the release changed after it was pinned; the pin is left as it was\n",
			pinning("9.2.0", old)},
		{"refused --json", CLI{JSON: true}, pinning("9.2.0", old), 1,
			"{\n  \"schema\": 1,\n  \"config\": \"FILE\",\n  \"action\": \"refused\",\n  \"version\": \"9.2.0\",\n  \"checksums\": \"" + sum +
				"\",\n  \"previous\": \"9.2.0\"\n}\n",
			"", pinning("9.2.0", old)},
	} {
		file := filepath.Join(t.TempDir(), "itos.yaml")
		var stdout, stderr strings.Builder
		code, err := r.c.Run(context.Background(), &stdout, &stderr, NewHandler(ports{file: file, text: r.text, version: "9.2.0"}))
		want := func(s string) string { return strings.ReplaceAll(s, "FILE", file) }
		if json := strings.ReplaceAll(file, `\`, `\\`); r.c.JSON {
			want = func(s string) string { return strings.ReplaceAll(s, "FILE", json) }
		}
		if err != nil || code != r.code || stdout.String() != want(r.stdout) || stderr.String() != want(r.stderr) {
			t.Errorf("%s: exit %d, %v, stdout %q, stderr %q", r.name, code, err, stdout.String(), stderr.String())
		}
		if after, _ := os.ReadFile(file); string(after) != r.after {
			t.Errorf("%s: the config is %q, want %q", r.name, after, r.after)
		}
	}
}

// The version asked for is the one the line names, "" for the newest.
func TestRunAsks(t *testing.T) {
	var asked string
	p := ports{file: filepath.Join(t.TempDir(), "itos.yaml"), text: pinning("9.2.0", sum), version: "9.2.0", asked: &asked}
	if _, err := (CLI{Query: Query{Want: "9.2.0"}}).Run(context.Background(), &strings.Builder{}, &strings.Builder{}, NewHandler(p)); err != nil || asked != "9.2.0" {
		t.Errorf("asked for %q, %v", asked, err)
	}
}

// A release that cannot be fetched leaves the config as it was, exit by its
// kind.
func TestRunUnfetched(t *testing.T) {
	for _, r := range []struct {
		kind kind.Kind
		code int
	}{{kind.Missing, 3}, {kind.Temporary, 75}, {kind.Usage, 2}, {kind.Unknown, 70}} {
		file := filepath.Join(t.TempDir(), "itos.yaml")
		fetch := kind.Wrap(r.kind, fmt.Errorf("cannot fetch itos 9.2.0"))
		if r.kind == kind.Unknown {
			fetch = errors.New("cannot fetch itos 9.2.0")
		}
		for _, json := range []bool{false, true} {
			var stdout, stderr strings.Builder
			code, err := (CLI{JSON: json}).Run(context.Background(), &stdout, &stderr, NewHandler(ports{file: file, text: pinning("9.1.0", old), fetch: fetch}))
			if err != nil || code != r.code || stdout.String() != "" || stderr.String() != "itos: cannot fetch itos 9.2.0; "+file+" is left as it was\n" {
				t.Errorf("%v: exit %d, %v, stdout %q, stderr %q", r.kind, code, err, stdout.String(), stderr.String())
			}
			if after, _ := os.ReadFile(file); string(after) != pinning("9.1.0", old) {
				t.Errorf("%v: the config is %q", r.kind, after)
			}
		}
	}
}

// A config that cannot be read, a pin that cannot be edited in place and a
// config that cannot be written are the command line's errors, nothing
// printed.
func TestRunFails(t *testing.T) {
	dir := t.TempDir()
	broken := errors.New("itos.yaml cannot be read")
	folder := filepath.Join(dir, "folder")
	if err := os.Mkdir(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, r := range []struct {
		name string
		p    Ports
		err  func(error) bool
	}{
		{"unreadable", ports{broken: broken}, func(err error) bool { return errors.Is(err, broken) }},
		{"uneditable", ports{file: filepath.Join(dir, "itos.yaml"), text: "pin: [9.1.0]\n", version: "9.2.0"},
			func(err error) bool { return kind.Of(err) == kind.Usage }},
		{"unwritable", unwritable{folder}, func(err error) bool { return err != nil && kind.Of(err) != kind.Usage }},
	} {
		var stdout, stderr strings.Builder
		code, err := (CLI{}).Run(context.Background(), &stdout, &stderr, NewHandler(r.p))
		if code != 0 || !r.err(err) || stdout.String() != "" || stderr.String() != "" {
			t.Errorf("%s: exit %d, %v, stdout %q, stderr %q", r.name, code, err, stdout.String(), stderr.String())
		}
	}
	p := ports{file: filepath.Join(dir, "itos.yaml"), text: "version: 1\n", version: "9.2.0"}
	if _, err := (CLI{JSON: true}).Run(context.Background(), failing{}, &strings.Builder{}, NewHandler(p)); err == nil {
		t.Error("an object it cannot write is no error")
	}
}

// unwritable is ports whose config is a folder, which every system refuses
// to write over.
type unwritable struct{ folder string }

func (u unwritable) Config(context.Context) (pinengine.Config, error) {
	return pinengine.Config{File: u.folder, Text: "version: 1\n"}, nil
}

func (unwritable) Release(context.Context, string) (string, []byte, error) {
	return "9.2.0", []byte(sums), nil
}
