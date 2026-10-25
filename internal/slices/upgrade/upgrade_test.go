package upgrade

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/engine/agentrules"
	pinengine "github.com/donvargax/itos/v8/internal/engine/pin"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/release"
)

const (
	base     = "https://releases.test"
	oldSum   = "1111111111111111111111111111111111111111111111111111111111111111"
	linuxSum = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	winSum   = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

// newSums is a checksums.txt of 9.3.0 for linux-amd64 and windows-amd64.
var newSums = []byte(linuxSum + "  itos-9.3.0-linux-amd64.tar.gz\n" +
	winSum + "  itos-9.3.0-windows-amd64.zip\n" +
	"cccc  itos.schema.json\n")

func script(version, linux, windows string) string {
	return "#!/bin/sh\nset -eu\nversion=" + version + "\n" +
		"case \"$(uname -s)-$(uname -m)\" in\n" +
		"Linux-x86_64) platform=linux-amd64 sum=" + linux + " ;;\n" +
		"MINGW*-x86_64 | MSYS*-x86_64) platform=windows-amd64 sum=" + windows + " ;;\n" +
		"esac\nversion_note=\"version=$version\"\n"
}

// sumsOf is a checksums.txt of the version, for linux-amd64 and
// windows-amd64.
func sumsOf(v string) string {
	return linuxSum + "  itos-" + v + "-linux-amd64.tar.gz\n" + winSum + "  itos-" + v + "-windows-amd64.zip\n"
}

// pinOf is a config pinning the version, with its checksums.
func pinOf(v string) string {
	return "version: 1\npin: { version: " + v + ", checksums: " + release.SHA256([]byte(sumsOf(v))) + " }\n"
}

// ports is upgrade's ports over a config file and a release server's
// files, keyed by their path after the base: the newest release, latest;
// the release that cannot be asked for, unasked; whether the config is
// stealth; the edits of the rules; a file it cannot write; each call
// recorded.
type ports struct {
	file       string
	latest     string
	unasked    error
	served     map[string]string
	stealth    bool
	rules      []agentrules.Edit
	unwritable string
	calls      *[]string
}

func (p ports) record(format string, a ...any) {
	if p.calls != nil {
		*p.calls = append(*p.calls, fmt.Sprintf(format, a...))
	}
}

func (p ports) Config(context.Context) (pinengine.Config, error) {
	p.record("config")
	return pinengine.ReadConfig("", p.file)
}

func (p ports) Release(_ context.Context, want string) (string, []byte, error) {
	p.record("release %s", want)
	if p.unasked != nil {
		return "", nil, p.unasked
	}
	if want == "" {
		want = p.latest
	}
	return want, []byte(sumsOf(want)), nil
}

func (p ports) Fetch(_ context.Context, url string) ([]byte, error) {
	path := strings.TrimPrefix(url, base)
	p.record("fetch %s", path)
	body, ok := p.served[path]
	switch {
	case !ok:
		return nil, kind.Wrap(kind.Missing, &release.StatusError{URL: url, Status: "404 Not Found", Code: 404})
	case body == "fail":
		return nil, kind.Wrap(kind.Temporary, &release.StatusError{URL: url, Status: "500 Internal Server Error", Code: 500})
	}
	return []byte(body), nil
}

func (p ports) Stealth(_ context.Context, file string) bool {
	p.record("stealth %s", filepath.Base(file))
	return p.stealth
}

func (p ports) Rules(_ context.Context, stealth bool) []agentrules.Edit {
	p.record("rules %v", stealth)
	return p.rules
}

func (p ports) Write(_ context.Context, file, text string) error {
	p.record("write %s", filepath.Base(file))
	if file == p.unwritable {
		return errors.New("cannot write " + file)
	}
	return pinengine.WriteKeepingMode("", file, text)
}

// served is a release server offering 9.1.0, 9.2.0 and 9.3.0's
// upgrading.json, each one's previous the one before, the files given
// laid over them.
func served(files map[string]string) map[string]string {
	s := map[string]string{}
	for i, v := range []string{"9.1.0", "9.2.0", "9.3.0"} {
		previous := ""
		if i > 0 {
			previous = []string{"9.1.0", "9.2.0"}[i-1]
		}
		s["/download/v"+v+"/upgrading.json"] = `{"schema": 1, "version": "` + v + `", "previous": "` + previous + `"}`
	}
	for path, body := range files {
		s[path] = body
	}
	return s
}

// project is a project folder, the folder the test runs in, its config the
// text (none for ""), and ports over it whose newest release is 9.3.0.
func project(t *testing.T, config string, files map[string]string) ports {
	t.Helper()
	t.Setenv(release.Env, base)
	dir := t.TempDir()
	t.Chdir(dir)
	file := filepath.Join(dir, "itos.yaml")
	if config != "" {
		if err := os.WriteFile(file, []byte(config), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return ports{file: file, latest: "9.3.0", served: served(files)}
}

func writeScript(t *testing.T, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(pinengine.InstallScript), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pinengine.InstallScript, []byte(text), 0o755); err != nil {
		t.Fatal(err)
	}
}

// upgradeRun runs upgrade over p, as itos would with --json when json, and
// gives its exit code, what it printed and its error.
func upgradeRun(p ports, json bool, args ...string) (int, string, error) {
	c, err := Parse(args, json)
	if err != nil {
		return 0, "", err
	}
	var stdout, stderr strings.Builder
	code, err := c.Run(context.Background(), &stdout, &stderr, NewHandler(p))
	return code, stdout.String() + stderr.String(), err
}

// compact is --json's object on one line.
func compact(t *testing.T, s string) string {
	t.Helper()
	var b bytes.Buffer
	if err := json.Compact(&b, []byte(s)); err != nil {
		t.Fatalf("%v: %q", err, s)
	}
	return b.String()
}

// quoted is s as a JSON string.
func quoted(t *testing.T, s string) string {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
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
	if _, err := Parse([]string{"--force"}, false); !errors.As(err, &u) || err.Error() != "upgrade takes only a <version> (--force)" {
		t.Errorf("--force: %v", err)
	}
}

// The command line handed to the version an upgrade moves to: that version
// named in place of none or latest, every global flag kept where it stood.
func TestNamingVersion(t *testing.T) {
	for in, want := range map[string]string{
		"upgrade":                                  "upgrade 9.3.0",
		"upgrade --json":                           "upgrade 9.3.0 --json",
		"--json upgrade latest":                    "--json upgrade 9.3.0",
		"upgrade v9.3.0 -q":                        "upgrade 9.3.0 -q",
		"--root dir upgrade --config c.yaml":       "--root dir upgrade 9.3.0 --config c.yaml",
		"--root=dir upgrade --config=c.yaml 9.3.0": "--root=dir upgrade --config=c.yaml 9.3.0",
	} {
		if got := strings.Join(NamingVersion(strings.Fields(in), "9.3.0"), " "); got != want {
			t.Errorf("NamingVersion(%q) = %q, not %q", in, got, want)
		}
	}
}

func TestMoveInstallScript(t *testing.T) {
	got, err := moveInstallScript(script("9.1.0", oldSum, oldSum), "9.3.0", newSums)
	if err != nil {
		t.Fatal(err)
	}
	if want := script("9.3.0", linuxSum, winSum); got != want {
		t.Errorf("moved script:\n%s\nwant:\n%s", got, want)
	}
}

func TestMoveInstallScriptRefuses(t *testing.T) {
	cases := map[string]string{
		"no version line":   strings.Replace(script("9.1.0", oldSum, oldSum), "version=9.1.0\n", "", 1),
		"two version lines": "version=9.1.0\n" + script("9.1.0", oldSum, oldSum),
		"a platform the release lacks": script("9.1.0", oldSum, oldSum) +
			"Darwin-arm64) platform=darwin-arm64 sum=" + oldSum + " ;;\n",
	}
	for name, text := range cases {
		if got, err := moveInstallScript(text, "9.3.0", newSums); err == nil {
			t.Errorf("%s: moved, to\n%s", name, got)
		}
	}
}

func TestMoveSchemaLine(t *testing.T) {
	const line = "# yaml-language-server: $schema=https://example.test/r/download/v"
	cases := []struct{ text, want string }{
		{line + "9.1.0/itos.schema.json\nversion: 1\n", line + "9.3.0/itos.schema.json\nversion: 1\n"},
		{line + "9.1.0/itos.schema.json\r\nversion: 1\r\n", line + "9.3.0/itos.schema.json\r\nversion: 1\r\n"},
		{line + "9.1.0/itos.schema.json", line + "9.3.0/itos.schema.json"},
		// Not a release's schema, or not the first line: left alone.
		{"# yaml-language-server: $schema=./itos.schema.json\n", "# yaml-language-server: $schema=./itos.schema.json\n"},
		{"version: 1\n" + line + "9.1.0/itos.schema.json\n", "version: 1\n" + line + "9.1.0/itos.schema.json\n"},
	}
	for _, c := range cases {
		if got := moveSchemaLine(c.text, "9.3.0"); got != c.want {
			t.Errorf("moveSchemaLine(%q) = %q, want %q", c.text, got, c.want)
		}
	}
}

// A release's schema declares renames on its properties, in the order they
// are written and at any depth, then its removals (slice 113).
func TestSchemaSteps(t *testing.T) {
	p := project(t, "", map[string]string{
		"/download/v9.3.0/itos.schema.json": `{"type": "object", "properties": {
			"ci": {"properties": {"watch": {"properties": {"wait": {"x-itos-renamed-from": "ci.watch.timeout"}}}}},
			"aim": {"x-itos-renamed-from": "goal"}},
			"x-itos-removed": ["work.adr", "hooks.old"]}`,
		"/download/v9.4.0/itos.schema.json": `{"properties": {"version": {"type": "number"}}}`,
		"/download/v9.5.0/itos.schema.json": "fail",
		"/download/v1.0.0/itos.schema.json": `[]`,
		"/download/v1.0.1/itos.schema.json": `{"properties": {"a": {"x-itos-renamed-from": 1}}}`,
		"/download/v1.0.2/itos.schema.json": `{"properties": {"a": {"x-itos-renamed-from": "b..c"}}}`,
		"/download/v1.0.3/itos.schema.json": `{"x-itos-removed": "a"}`,
		"/download/v1.0.4/itos.schema.json": `{"x-itos-removed": ["a", 2]}`,
		"/download/v1.0.5/itos.schema.json": `{"properties": {"a": {"properties": {"b": {"x-itos-renamed-from": ""}}}}}`,
	})
	h, ctx := NewHandler(p), context.Background()
	steps, err := h.schemaSteps(ctx, "9.3.0")
	want := []configStep{{"renamed", "ci.watch.wait", "ci.watch.timeout"}, {"renamed", "aim", "goal"},
		{"removed", "work.adr", ""}, {"removed", "hooks.old", ""}}
	if err != nil || !slices.Equal(steps, want) {
		t.Errorf("schemaSteps = %v, %v; want %v", steps, err, want)
	}
	for _, v := range []string{"9.4.0", "9.2.0"} {
		if steps, err := h.schemaSteps(ctx, v); err != nil || len(steps) != 0 {
			t.Errorf("%s: %v, %v", v, steps, err)
		}
	}
	if _, err := h.schemaSteps(ctx, "9.5.0"); kind.Of(err) != kind.Temporary ||
		err.Error() != "cannot fetch the schema of itos 9.5.0: "+base+"/download/v9.5.0/itos.schema.json: 500 Internal Server Error" {
		t.Errorf("a server that fails: %v", err)
	}
	for v, says := range map[string]string{
		"1.0.0": "is not a JSON object",
		"1.0.1": "declares a renamed from 1, not a key's dotted path",
		"1.0.2": `declares a renamed from "b..c", not a key's dotted path`,
		"1.0.3": `lists x-itos-removed as "a", not a list`,
		"1.0.4": "lists 2 in x-itos-removed, not a key's dotted path",
		"1.0.5": `declares a.b renamed from "", not a key's dotted path`,
	} {
		want := "the itos.schema.json of itos " + v + " (" + base + "/download/v" + v + "/itos.schema.json) " + says
		if _, err := h.schemaSteps(ctx, v); kind.Of(err) != kind.Missing || err.Error() != want {
			t.Errorf("%s: %v, want %q", v, err, want)
		}
	}
}

// The walk fetches each release's schema too, and a schema it cannot fetch
// stops it.
func TestWalkReleasesFetchesSchemas(t *testing.T) {
	asks := func(v, previous string) string {
		return `{"schema": 1, "version": "` + v + `", "previous": "` + previous + `"}`
	}
	ctx := context.Background()
	p := project(t, "", nil)
	p.served = map[string]string{
		"/download/v9.2.0/upgrading.json":   asks("9.2.0", "9.1.0"),
		"/download/v9.3.0/upgrading.json":   asks("9.3.0", "9.2.0"),
		"/download/v9.3.0/itos.schema.json": `{"x-itos-removed": ["a"]}`,
	}
	found, err := NewHandler(p).walkReleases(ctx, "9.1.0", "9.3.0")
	if err != nil || len(found) != 2 {
		t.Fatalf("walkReleases = %v, %v", found, err)
	}
	if r := found[1].(releaseAsks); r.Version != "9.3.0" || len(r.steps) != 1 || len(found[0].(releaseAsks).steps) != 0 {
		t.Errorf("walkReleases = %+v", found)
	}

	p.served = map[string]string{
		"/download/v9.3.0/upgrading.json":   asks("9.3.0", "9.2.0"),
		"/download/v9.3.0/itos.schema.json": "fail",
	}
	if _, err := NewHandler(p).walkReleases(ctx, "9.2.0", "9.3.0"); kind.Of(err) != kind.Temporary {
		t.Errorf("a schema that cannot be fetched: %v", err)
	}

	// A release with no upgrading.json, or one this itos cannot read, is
	// named by its notes and ends the walk, as does a previous not older
	// than its release; an upgrading.json that cannot be fetched stops it.
	p.served = map[string]string{
		"/download/v9.3.0/upgrading.json": asks("9.3.0", "9.3.0"),
		"/download/v9.2.0/upgrading.json": "{",
		"/download/v9.1.0/upgrading.json": "fail",
	}
	for to, want := range map[string]string{"9.3.0": "9.3.0", "9.2.0": "notes", "9.0.1": "notes"} {
		found, err := NewHandler(p).walkReleases(ctx, "9.0.0", to)
		if err != nil || len(found) != 1 {
			t.Errorf("walkReleases to %s = %v, %v", to, found, err)
			continue
		}
		if _, isNotes := found[0].(releaseNotes); isNotes != (want == "notes") {
			t.Errorf("walkReleases to %s = %+v", to, found)
		}
	}
	if _, err := NewHandler(p).walkReleases(ctx, "9.0.0", "9.1.0"); kind.Of(err) != kind.Temporary ||
		!strings.HasPrefix(err.Error(), "cannot fetch what itos 9.1.0 asks: ") {
		t.Errorf("an upgrading.json that cannot be fetched: %v", err)
	}
}

// Each release's steps are made oldest first, each on the text the one
// before left; a key that is not there is no edit, and a layout the editor
// cannot change is a usage error naming the key and the release.
func TestApplySteps(t *testing.T) {
	text := "version: 1\n# ten minutes\nci:\n  watch:\n    timeout: 600\nwork:\n  adr: docs\n  registry: r\n"
	releases := []any{
		releaseNotes{Version: "9.1.0", Notes: "notes"},
		releaseAsks{Version: "9.2.0", steps: []configStep{{kind: "renamed", key: "ci.watch.limit", from: "ci.watch.timeout"}}},
		releaseAsks{Version: "9.3.0", steps: []configStep{{kind: "renamed", key: "ci.watch.wait", from: "ci.watch.limit"},
			{kind: "removed", key: "work.adr"}, {kind: "removed", key: "hooks.gone"}}},
	}
	got, applied, err := applySteps("itos.yaml", text, releases)
	if err != nil {
		t.Fatal(err)
	}
	if want := "version: 1\n# ten minutes\nci:\n  watch:\n    wait: 600\nwork:\n  registry: r\n"; got != want {
		t.Errorf("applySteps:\n got %q\nwant %q", got, want)
	}
	want := []appliedStep{{"9.2.0", configStep{"renamed", "ci.watch.limit", "ci.watch.timeout"}, nil},
		{"9.3.0", configStep{"renamed", "ci.watch.wait", "ci.watch.limit"}, nil},
		{"9.3.0", configStep{"removed", "work.adr", ""}, "docs"}}
	if !slices.Equal(applied, want) {
		t.Errorf("applied %v, want %v", applied, want)
	}

	cases := map[string]struct{ text, says string }{
		"a flow mapping":       {"ci:\n  watch: { timeout: 600 }\n", "cannot rename ci.watch.timeout to ci.watch.limit in itos.yaml, as itos 9.2.0's"},
		"an unreadable config": {"ci: [\n", "cannot rename ci.watch.timeout"},
		"an anchor":            {"work: &w\n  adr: docs\n", "cannot remove work.adr in itos.yaml, as itos 9.3.0's"},
	}
	for name, c := range cases {
		_, _, err := applySteps("itos.yaml", c.text, releases)
		if kind.Of(err) != kind.Usage || !strings.Contains(err.Error(), c.says) ||
			!strings.HasSuffix(err.Error(), "edit it by hand, then run itos upgrade again; every file is left as it was") {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// The report: each release's edits and what it changes, then By hand and
// what the releases still ask, or that nothing is left.
func TestReport(t *testing.T) {
	t.Setenv(release.Env, base)
	var stdout strings.Builder
	a := Answer{From: "9.0.0", To: "9.3.0", Edited: []string{"itos.yaml"}, Releases: []any{
		releaseNotes{Version: "9.1.0", Notes: "https://example.test/tag/v9.1.0"},
		releaseAsks{Version: "9.2.0", Upgrading: []upgradingEntry{{Commit: "0123456789", Header: "feat: a", Text: "Move the records.\nThen check."}},
			Changes: []upgradingEntry{{Commit: "abc", Header: "fix: b", Text: "@ID-ASK-12"}}},
		releaseAsks{Version: "9.3.0", Breaking: []upgradingEntry{{Commit: "def", Header: "feat!: c", Text: "Records are MADR 4."}},
			Config: []upgradingKey{{Key: "work.adr", Change: "key removed"}}},
	}, Applied: []appliedStep{{"9.3.0", configStep{"renamed", "ci.watch.wait", "ci.watch.timeout"}, nil},
		{"9.3.0", configStep{"removed", "work.adr", ""}, float64(600)}}}
	report(&stdout, a)
	want := "Upgraded itos from 9.0.0 to 9.3.0: edited itos.yaml, and committed nothing.\n" +
		"Each release since 9.0.0, oldest first:\n" +
		"\nitos 9.2.0 (" + release.NotesURL("9.2.0") + "):\n" +
		"  Changes on purpose (abc fix: b):\n    @ID-ASK-12\n" +
		"\nitos 9.3.0 (" + release.NotesURL("9.3.0") + "):\n" +
		"  Renamed ci.watch.timeout to ci.watch.wait.\n  Removed work.adr, which held 600.\n" +
		"  Config key work.adr: key removed\n" +
		"\nBy hand:\n" +
		"\nitos 9.1.0 publishes no upgrading.json: read the Upgrading section of its notes, and of each release " +
		"before it since 9.0.0, by hand: https://example.test/tag/v9.1.0\n" +
		"\nitos 9.2.0 (" + release.NotesURL("9.2.0") + "):\n" +
		"  Upgrading (0123456 feat: a):\n    Move the records.\n    Then check.\n" +
		"\nitos 9.3.0 (" + release.NotesURL("9.3.0") + "):\n" +
		"  Breaking (def feat!: c):\n    Records are MADR 4.\n" +
		"\nDo what they ask, then check: itos version prints itos 9.3.0, itos version --check exits 0 and itos config " +
		"check passes. Commit the move as the project's own commit.\n"
	if got := stdout.String(); got != want {
		t.Errorf("report:\n%s\nwant:\n%s", got, want)
	}
	steps := manual(a)
	if len(steps) != 3 || steps[0].Text != "itos 9.1.0 publishes no upgrading.json: read the Upgrading section of its "+
		"notes, and of each release before it since 9.0.0: https://example.test/tag/v9.1.0" ||
		steps[1] != (manualStep{"9.2.0", "Move the records.\nThen check."}) || steps[2].Text != "Records are MADR 4." {
		t.Errorf("manual = %v", steps)
	}

	stdout.Reset()
	report(&stdout, Answer{From: "9.2.0", To: "9.3.0", Releases: []any{releaseAsks{Version: "9.3.0"}}})
	if got, want := stdout.String(), "Upgraded itos from 9.2.0 to 9.3.0: nothing needed editing, and committed nothing.\n"+
		"Each release since 9.2.0, oldest first:\n\nitos 9.3.0 ("+release.NotesURL("9.3.0")+"):\n  nothing to edit.\n"+
		"\nBy hand: nothing.\n\nCheck: itos version prints itos 9.3.0, itos version --check exits 0 and itos config check "+
		"passes. Commit the move as the project's own commit.\n"; got != want {
		t.Errorf("report:\n%s\nwant:\n%s", got, want)
	}

	// A release that asks nothing by hand has no place under By hand, even
	// when a later one does.
	stdout.Reset()
	report(&stdout, Answer{From: "9.1.0", To: "9.3.0", Releases: []any{releaseAsks{Version: "9.2.0"},
		releaseAsks{Version: "9.3.0", Upgrading: []upgradingEntry{{Commit: "def", Header: "feat: c", Text: "Do it."}}}}})
	_, byHand, _ := strings.Cut(stdout.String(), "By hand:")
	if strings.Contains(byHand, "itos 9.2.0") || !strings.Contains(byHand, "itos 9.3.0") {
		t.Errorf("By hand names a release that asks nothing, or not the one that asks:\n%s", stdout.String())
	}
}

// --json lists the edits made and what is left by hand.
func TestEmit(t *testing.T) {
	var stdout strings.Builder
	a := Answer{Action: upgraded, From: "9.2.0", To: "9.3.0", Edited: []string{}, Releases: []any{
		releaseAsks{Version: "9.3.0", Upgrading: []upgradingEntry{{Text: "Do it."}}},
	}, Applied: []appliedStep{{"9.3.0", configStep{"renamed", "b", "a"}, nil}, {"9.3.0", configStep{"removed", "c", ""}, nil}}}
	if err := emit(&stdout, a); err != nil {
		t.Fatal(err)
	}
	want := `{"schema":1,"action":"upgraded","from":"9.2.0","to":"9.3.0","releases":[{"version":"9.3.0","breaking":null,` +
		`"upgrading":[{"commit":"","header":"","text":"Do it."}],"changes":null,"config":null}],"edited":[],` +
		`"applied":[{"version":"9.3.0","kind":"renamed","key":"b","from":"a"},` +
		`{"version":"9.3.0","kind":"removed","key":"c","value":null}],"manual":[{"version":"9.3.0","text":"Do it."}],` +
		`"action_uses":[],"action_scan_problems":[]}`
	if got := compact(t, stdout.String()); got != want {
		t.Errorf("emit:\n%s\nwant\n%s", got, want)
	}
}

// What upgrade refuses, and how, each with every file left as it was.
func TestRunRefuses(t *testing.T) {
	expect := func(name string, wantCode int, wantErr bool, says string, code int, err error, printed string) {
		t.Helper()
		got := printed
		if err != nil {
			got += err.Error()
		}
		if code != wantCode || (err != nil) != wantErr || !strings.Contains(got, says) {
			t.Errorf("%s: %d, %v, %q; want %d, an error %t, saying %q", name, code, err, printed, wantCode, wantErr, says)
		}
	}
	p := project(t, pinOf("9.1.0"), nil)
	code, printed, err := upgradeRun(p, false, "--force")
	expect("a flag", 0, true, "upgrade takes only a <version>", code, err, printed)

	p = project(t, "", nil)
	code, printed, err = upgradeRun(p, false)
	expect("no config", 0, true, "cannot be read", code, err, printed)

	p = project(t, pinOf("9.1.0"), nil)
	if err := os.MkdirAll(pinengine.InstallScript, 0o755); err != nil {
		t.Fatal(err)
	}
	code, printed, err = upgradeRun(p, false)
	expect("an install script that cannot be read", 0, true, "cannot read "+pinengine.InstallScript, code, err, printed)

	p = project(t, "version: 1\n", nil)
	writeScript(t, "#!/bin/sh\n")
	code, printed, err = upgradeRun(p, false)
	expect("a script with no version", 0, true, "cannot read "+pinengine.InstallScript+": it has 0 version= lines, not one; "+
		"every file is left as it was", code, err, printed)

	p = project(t, "version: 1\n", nil)
	code, printed, err = upgradeRun(p, false)
	expect("no pin and no script", 1, false, "itos: "+p.file+" pins no itos, and there is no "+pinengine.InstallScript+
		" whose version to move: nothing to upgrade; itos pin pins a release\n", code, err, printed)

	p = project(t, pinOf("9.2.0"), nil)
	code, printed, err = upgradeRun(p, false, "9.1.0")
	expect("an older version", 1, false, "itos: itos 9.1.0 is older than 9.2.0, the version this project is on: "+
		"itos upgrade only moves forward; itos pin 9.1.0 moves it back\n", code, err, printed)

	p = project(t, pinOf("9.3.0"), nil)
	p.latest = "9.2.0"
	code, printed, err = upgradeRun(p, false)
	expect("a newest older than the pin", 1, false, "itos: the newest itos, 9.2.0, is older than 9.3.0, the version this "+
		"project is on: nothing to upgrade; itos pin 9.2.0 moves it back\n", code, err, printed)

	p = project(t, "version: 1\npin: { version: 9.3.0, checksums: \""+oldSum+"\" }\n", nil)
	code, printed, err = upgradeRun(p, false)
	expect("a release changed after it was pinned", 1, false, "itos: "+p.file+" pins itos 9.3.0 with the checksums "+oldSum+
		", but its checksums.txt is "+release.SHA256([]byte(sumsOf("9.3.0")))+" now: the release changed after it was "+
		"pinned; every file is left as it was\n", code, err, printed)

	p = project(t, pinOf("9.1.0"), map[string]string{"/download/v9.3.0/upgrading.json": "fail"})
	code, printed, err = upgradeRun(p, false)
	expect("an upgrading.json that cannot be fetched", temporary, false, "itos: cannot fetch what itos 9.3.0 asks: "+base+
		"/download/v9.3.0/upgrading.json: 500 Internal Server Error; every file is left as it was\n", code, err, printed)

	p = project(t, pinOf("9.1.0")+"ci:\n  watch: { timeout: 600 }\n", map[string]string{
		"/download/v9.3.0/itos.schema.json": `{"x-itos-removed": ["ci.watch.timeout"]}`,
	})
	code, printed, err = upgradeRun(p, false)
	expect("a config step it cannot make", 0, true, "cannot remove ci.watch.timeout in "+p.file, code, err, printed)
	if text, _ := os.ReadFile(p.file); string(text) != pinOf("9.1.0")+"ci:\n  watch: { timeout: 600 }\n" {
		t.Errorf("a config step it cannot make edited the config:\n%s", text)
	}

	p = project(t, "version: 1\npin: &p { version: 9.1.0, checksums: "+release.SHA256([]byte(sumsOf("9.1.0")))+" }\n", nil)
	code, printed, err = upgradeRun(p, false)
	expect("a pin it cannot edit", 0, true, "cannot edit the pin", code, err, printed)

	p = project(t, pinOf("9.1.0"), nil)
	writeScript(t, script("9.1.0", oldSum, oldSum)+"Darwin-arm64) platform=darwin-arm64 sum="+oldSum+" ;;\n")
	code, printed, err = upgradeRun(p, false)
	expect("a script it cannot edit", 0, true, "cannot edit "+pinengine.InstallScript+": the checksums.txt of itos 9.3.0 lists "+
		"no itos-9.3.0-darwin-arm64.tar.gz, the archive of its darwin-arm64 line; every file is left as it was", code, err, printed)
	if text, _ := os.ReadFile(p.file); string(text) != pinOf("9.1.0") {
		t.Errorf("a script it cannot edit edited the config:\n%s", text)
	}
}

// A refusal under --json is the one object, exit 1, its versions as far as
// upgrade read them.
func TestRunRefusesJSON(t *testing.T) {
	for _, r := range []struct {
		name, config string
		args         []string
		from, to     string
	}{
		{"no pin and no script", "version: 1\n", nil, "", ""},
		{"no pin and no script, a version named", "version: 1\n", []string{"9.2.0"}, "", "9.2.0"},
		{"an older version", pinOf("9.2.0"), []string{"9.1.0"}, "9.2.0", "9.1.0"},
		{"a changed release", "version: 1\npin: { version: 9.3.0, checksums: \"" + oldSum + "\" }\n", nil, "9.3.0", "9.3.0"},
	} {
		p := project(t, r.config, nil)
		code, printed, err := upgradeRun(p, true, r.args...)
		want := `{"schema":1,"action":"refused","from":"` + r.from + `","to":"` + r.to +
			`","releases":[],"edited":[],"applied":[],"manual":[],"action_uses":[],"action_scan_problems":[]}`
		if code != policy || err != nil || compact(t, printed) != want {
			t.Errorf("%s: exit %d, %v, %s", r.name, code, err, printed)
		}
	}
}

// A release that cannot be asked for is a line on stderr, every file as it
// was, exit by its kind.
func TestRunUnfetched(t *testing.T) {
	for _, r := range []struct {
		kind kind.Kind
		code int
	}{{kind.Missing, missing}, {kind.Temporary, temporary}, {kind.Usage, usage}, {kind.Unknown, software}} {
		p := project(t, pinOf("9.1.0"), nil)
		p.unasked = kind.Wrap(r.kind, errors.New("cannot fetch itos 9.9.0"))
		if r.kind == kind.Unknown {
			p.unasked = errors.New("cannot fetch itos 9.9.0")
		}
		for _, json := range []bool{false, true} {
			code, printed, err := upgradeRun(p, json, "9.9.0")
			if err != nil || code != r.code || printed != "itos: cannot fetch itos 9.9.0; every file is left as it was\n" {
				t.Errorf("%v: exit %d, %v, %q", r.kind, code, err, printed)
			}
		}
		if text, _ := os.ReadFile(p.file); string(text) != pinOf("9.1.0") {
			t.Errorf("%v: the config is %q", r.kind, text)
		}
	}
}

// A project on the version changes nothing, the version named or not.
func TestRunAlready(t *testing.T) {
	p := project(t, pinOf("9.3.0"), nil)
	if code, printed, err := upgradeRun(p, false); code != 0 || err != nil ||
		printed != "This project is already on itos 9.3.0: nothing to upgrade.\n" {
		t.Errorf("already: exit %d, %v, %q", code, err, printed)
	}
	if code, printed, err := upgradeRun(p, false, "9.3.0"); code != 0 || err != nil ||
		printed != "This project is already on itos 9.3.0: nothing to upgrade.\n" {
		t.Errorf("already, named: exit %d, %v, %q", code, err, printed)
	}
	want := `{"schema":1,"action":"already","from":"9.3.0","to":"9.3.0","releases":[],"edited":[],"applied":[],"manual":[],` +
		`"action_uses":[],"action_scan_problems":[]}`
	if code, printed, err := upgradeRun(p, true); code != 0 || err != nil || compact(t, printed) != want {
		t.Errorf("already --json: exit %d, %v, %s", code, err, printed)
	}
	// A pin on it with no checksums is no changed release.
	p = project(t, "version: 1\npin: { version: 9.3.0 }\n", nil)
	if code, printed, err := upgradeRun(p, false); code != 0 || err != nil || !strings.Contains(printed, "already on itos 9.3.0") {
		t.Errorf("already, no checksums: exit %d, %v, %q", code, err, printed)
	}
}

// An upgrade moves the pin, the schema line and the install script, makes
// each release's config edits and rewrites the rules' blocks, asking its
// ports in that order, then says so.
func TestRunUpgrades(t *testing.T) {
	const schema = "# yaml-language-server: $schema=https://example.test/download/v9.1.0/itos.schema.json\n"
	p := project(t, schema+pinOf("9.1.0")+"ci:\n  watch:\n    timeout: 600\n", map[string]string{
		"/download/v9.3.0/itos.schema.json": `{"x-itos-removed": ["ci.watch.timeout"]}`,
	})
	writeScript(t, script("9.1.0", oldSum, oldSum))
	rules := filepath.Join(t.TempDir(), "AGENTS.md")
	p.rules = []agentrules.Edit{{Path: "AGENTS.md", Disk: rules, Text: "Mine.\n"}}
	var calls []string
	p.calls, p.stealth = &calls, true
	code, printed, err := upgradeRun(p, false)
	if code != 0 || err != nil {
		t.Fatalf("exit %d, %v, %s", code, err, printed)
	}
	want := "Upgraded itos from 9.1.0 to 9.3.0: edited " + p.file + ", " + pinengine.InstallScript + " and AGENTS.md, " +
		"and committed nothing.\nEach release since 9.1.0, oldest first:\n" +
		"\nitos 9.2.0 (" + base + "/tag/v9.2.0):\n  nothing to edit.\n" +
		"\nitos 9.3.0 (" + base + "/tag/v9.3.0):\n  Removed ci.watch.timeout, which held 600.\n" +
		"\nBy hand: nothing.\n\nCheck: itos version prints itos 9.3.0, itos version --check exits 0 and itos config check " +
		"passes. Commit the move as the project's own commit.\n"
	if printed != want {
		t.Errorf("printed:\n%s\nwant:\n%s", printed, want)
	}
	if text, _ := os.ReadFile(p.file); string(text) != strings.Replace(schema, "9.1.0", "9.3.0", 1)+
		"version: 1\npin: { version: 9.3.0, checksums: "+release.SHA256([]byte(sumsOf("9.3.0")))+" }\n" {
		t.Errorf("the config is %q", text)
	}
	if text, _ := os.ReadFile(pinengine.InstallScript); string(text) != script("9.3.0", linuxSum, winSum) {
		t.Errorf("the install script is %q", text)
	}
	if text, _ := os.ReadFile(rules); string(text) != "Mine.\n" {
		t.Errorf("the rules are %q", text)
	}
	wantCalls := []string{"config", "release ", "fetch /download/v9.3.0/upgrading.json", "fetch /download/v9.3.0/itos.schema.json",
		"fetch /download/v9.2.0/upgrading.json", "fetch /download/v9.2.0/itos.schema.json", "stealth itos.yaml", "rules true",
		"write itos.yaml", "write install-itos"}
	if !slices.Equal(calls, wantCalls) {
		t.Errorf("calls %q, want %q", calls, wantCalls)
	}
}

// With no pin, the version moved from is the install script's, and only
// the script is edited; --json says what each release asks and what was
// applied.
func TestRunUpgradesTheScript(t *testing.T) {
	p := project(t, "version: 1\nci:\n  watch:\n    timeout: 600\n", map[string]string{
		"/download/v9.3.0/upgrading.json": `{"schema": 1, "version": "9.3.0", "previous": "9.2.0",
			"upgrading": [{"commit": "abc", "header": "feat: x", "text": "Do it."}]}`,
		"/download/v9.3.0/itos.schema.json": `{"x-itos-removed": ["ci.watch.timeout"]}`,
	})
	writeScript(t, script("9.2.0", oldSum, oldSum))
	code, printed, err := upgradeRun(p, true)
	want := `{"schema":1,"action":"upgraded","from":"9.2.0","to":"9.3.0","releases":[{"version":"9.3.0","breaking":[],` +
		`"upgrading":[{"commit":"abc","header":"feat: x","text":"Do it."}],"changes":[],"config":[]}],` +
		`"edited":[` + quoted(t, p.file) + `,"` + pinengine.InstallScript + `"],` +
		`"applied":[{"version":"9.3.0","kind":"removed","key":"ci.watch.timeout","value":600}],` +
		`"manual":[{"version":"9.3.0","text":"Do it."}],"action_uses":[],"action_scan_problems":[]}`
	if code != 0 || err != nil || compact(t, printed) != want {
		t.Errorf("exit %d, %v:\n%s\nwant\n%s", code, err, compact(t, printed), want)
	}
	if text, _ := os.ReadFile(p.file); string(text) != "version: 1\n" {
		t.Errorf("the config is %q", text)
	}
	if text, _ := os.ReadFile(pinengine.InstallScript); string(text) != script("9.3.0", linuxSum, winSum) {
		t.Errorf("the install script is %q", text)
	}
}

// A file it cannot write is its error: the config, the install script and
// a block of the rules alike.
func TestRunCannotWrite(t *testing.T) {
	p := project(t, pinOf("9.1.0"), nil)
	p.unwritable = p.file
	if code, printed, err := upgradeRun(p, false); code != 0 || err == nil || printed != "" {
		t.Errorf("a config it cannot write: %d, %v, %s", code, err, printed)
	}

	p = project(t, "version: 1\n", nil)
	writeScript(t, script("9.1.0", oldSum, oldSum))
	p.unwritable = pinengine.InstallScript
	if code, printed, err := upgradeRun(p, false); code != 0 || err == nil || printed != "" {
		t.Errorf("a script it cannot write: %d, %v, %s", code, err, printed)
	}

	p = project(t, pinOf("9.1.0"), nil)
	p.rules = []agentrules.Edit{{Path: "AGENTS.md", Disk: t.TempDir(), Text: "Mine.\n"}}
	if code, printed, err := upgradeRun(p, false); code != 0 || err == nil || printed != "" {
		t.Errorf("an AGENTS.md it cannot write: %d, %v, %s", code, err, printed)
	}
}

// An object it cannot write is the command line's error.
func TestRunCannotEmit(t *testing.T) {
	for _, config := range []string{"version: 1\n", pinOf("9.3.0"), pinOf("9.2.0")} {
		p := project(t, config, nil)
		if _, err := (CLI{JSON: true}).Run(context.Background(), failing{}, &strings.Builder{}, NewHandler(p)); err == nil {
			t.Errorf("%q: an object it cannot write is no error", config)
		}
	}
}
