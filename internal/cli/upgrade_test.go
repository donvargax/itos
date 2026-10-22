package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/engine/pin"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/release"
)

const (
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

// latest is the newest, as no version is; anything else not a version is a
// usage error.
func TestVersionArg(t *testing.T) {
	for args, want := range map[string]string{"": "", "latest": "", "2.5.0": "2.5.0", "v2.5.0": "2.5.0"} {
		if got, err := versionArg("pin", strings.Fields(args)); err != nil || got != want {
			t.Errorf("versionArg(%s) = %q, %v; want %q", args, got, err, want)
		}
	}
	for _, args := range []string{"newest", "Latest", "vlatest", "latest 2.5.0", "--force"} {
		if _, err := versionArg("pin", strings.Fields(args)); err == nil {
			t.Errorf("versionArg(%s) took it", args)
		}
	}
}

// releaseServer serves each path's body, 404 for any other, and 500 for
// a body of "fail".
func releaseServer(t *testing.T, files map[string]string) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := files[r.URL.Path]
		switch {
		case !ok:
			http.NotFound(w, r)
		case body == "fail":
			http.Error(w, "down", http.StatusInternalServerError)
		default:
			_, _ = w.Write([]byte(body))
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv(release.Env, server.URL)
}

// A release's schema declares renames on its properties, in the order they
// are written and at any depth, then its removals (slice 113).
func TestSchemaSteps(t *testing.T) {
	releaseServer(t, map[string]string{
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
	steps, err := schemaSteps("9.3.0")
	want := []configStep{{"renamed", "ci.watch.wait", "ci.watch.timeout"}, {"renamed", "aim", "goal"},
		{"removed", "work.adr", ""}, {"removed", "hooks.old", ""}}
	if err != nil || !slices.Equal(steps, want) {
		t.Errorf("schemaSteps = %v, %v; want %v", steps, err, want)
	}
	for _, v := range []string{"9.4.0", "9.2.0"} {
		if steps, err := schemaSteps(v); err != nil || len(steps) != 0 {
			t.Errorf("%s: %v, %v", v, steps, err)
		}
	}
	if _, err := schemaSteps("9.5.0"); kind.Of(err) != kind.Temporary {
		t.Errorf("a server that fails: %v", err)
	}
	for _, v := range []string{"1.0.0", "1.0.1", "1.0.2", "1.0.3", "1.0.4", "1.0.5"} {
		if _, err := schemaSteps(v); kind.Of(err) != kind.Missing || !strings.Contains(err.Error(), "itos.schema.json of itos "+v) {
			t.Errorf("%s: %v", v, err)
		}
	}
}

// The walk fetches each release's schema too, and a schema it cannot fetch
// stops it.
func TestWalkReleasesFetchesSchemas(t *testing.T) {
	asks := func(v, previous string) string {
		return `{"schema": 1, "version": "` + v + `", "previous": "` + previous + `"}`
	}
	releaseServer(t, map[string]string{
		"/download/v9.2.0/upgrading.json":   asks("9.2.0", "9.1.0"),
		"/download/v9.3.0/upgrading.json":   asks("9.3.0", "9.2.0"),
		"/download/v9.3.0/itos.schema.json": `{"x-itos-removed": ["a"]}`,
	})
	found, err := walkReleases("9.1.0", "9.3.0")
	if err != nil || len(found) != 2 {
		t.Fatalf("walkReleases = %v, %v", found, err)
	}
	if r := found[1].(releaseAsks); r.Version != "9.3.0" || len(r.steps) != 1 || len(found[0].(releaseAsks).steps) != 0 {
		t.Errorf("walkReleases = %+v", found)
	}

	releaseServer(t, map[string]string{
		"/download/v9.3.0/upgrading.json":   asks("9.3.0", "9.2.0"),
		"/download/v9.3.0/itos.schema.json": "fail",
	})
	if _, err := walkReleases("9.2.0", "9.3.0"); kind.Of(err) != kind.Temporary {
		t.Errorf("a schema that cannot be fetched: %v", err)
	}

	// A release with no upgrading.json, or one this itos cannot read, is
	// named by its notes and ends the walk, as does a previous not older
	// than its release; an upgrading.json that cannot be fetched stops it.
	releaseServer(t, map[string]string{
		"/download/v9.3.0/upgrading.json": asks("9.3.0", "9.3.0"),
		"/download/v9.2.0/upgrading.json": "{",
		"/download/v9.1.0/upgrading.json": "fail",
	})
	for to, want := range map[string]string{"9.3.0": "9.3.0", "9.2.0": "notes", "9.0.1": "notes"} {
		found, err := walkReleases("9.0.0", to)
		if err != nil || len(found) != 1 {
			t.Errorf("walkReleases to %s = %v, %v", to, found, err)
			continue
		}
		if _, isNotes := found[0].(releaseNotes); isNotes != (want == "notes") {
			t.Errorf("walkReleases to %s = %+v", to, found)
		}
	}
	if _, err := walkReleases("9.0.0", "9.1.0"); kind.Of(err) != kind.Temporary {
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
		if kind.Of(err) != kind.Usage || !strings.Contains(err.Error(), c.says) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// The report: each release's edits and what it changes, then By hand and
// what the releases still ask, or that nothing is left.
func TestUpgradePrint(t *testing.T) {
	var stdout strings.Builder
	u := upgrade{o: Out{Stdout: &stdout}, from: "9.0.0", to: "9.3.0", edited: []string{"itos.yaml"}, releases: []any{
		releaseNotes{Version: "9.1.0", Notes: "https://example.test/tag/v9.1.0"},
		releaseAsks{Version: "9.2.0", Upgrading: []upgradingEntry{{Commit: "0123456789", Header: "feat: a", Text: "Move the records.\nThen check."}},
			Changes: []upgradingEntry{{Commit: "abc", Header: "fix: b", Text: "@ID-ASK-12"}}},
		releaseAsks{Version: "9.3.0", Breaking: []upgradingEntry{{Commit: "def", Header: "feat!: c", Text: "Records are MADR 4."}},
			Config: []upgradingKey{{Key: "work.adr", Change: "key removed"}}},
	}, applied: []appliedStep{{"9.3.0", configStep{"renamed", "ci.watch.wait", "ci.watch.timeout"}, nil},
		{"9.3.0", configStep{"removed", "work.adr", ""}, float64(600)}}}
	u.print()
	got := stdout.String()
	order := []string{"Upgraded itos from 9.0.0 to 9.3.0: edited itos.yaml", "itos 9.2.0", "Changes on purpose (abc fix: b)",
		"@ID-ASK-12", "itos 9.3.0", "Renamed ci.watch.timeout to ci.watch.wait.", "Removed work.adr, which held 600.",
		"Config key work.adr: key removed", "By hand:", "itos 9.1.0 publishes no upgrading.json", "Upgrading (0123456 feat: a)",
		"    Then check.", "Breaking (def feat!: c)", "Records are MADR 4.", "Do what they ask"}
	at := 0
	for _, s := range order {
		i := strings.Index(got[at:], s)
		if i < 0 {
			t.Fatalf("the report does not say %q after %q:\n%s", s, got[:at], got)
		}
		at += i + len(s)
	}
	manual := u.manual()
	if len(manual) != 3 || manual[1] != (manualStep{"9.2.0", "Move the records.\nThen check."}) || manual[2].Text != "Records are MADR 4." {
		t.Errorf("manual = %v", manual)
	}

	stdout.Reset()
	u = upgrade{o: Out{Stdout: &stdout}, from: "9.2.0", to: "9.3.0", releases: []any{releaseAsks{Version: "9.3.0"}}}
	u.print()
	for _, s := range []string{"nothing needed editing", "  nothing to edit.", "By hand: nothing.", "Check: itos version prints itos 9.3.0"} {
		if !strings.Contains(stdout.String(), s) {
			t.Errorf("the report does not say %q:\n%s", s, stdout.String())
		}
	}

	// A release that asks nothing by hand has no place under By hand, even
	// when a later one does.
	stdout.Reset()
	u = upgrade{o: Out{Stdout: &stdout}, from: "9.1.0", to: "9.3.0", releases: []any{releaseAsks{Version: "9.2.0"},
		releaseAsks{Version: "9.3.0", Upgrading: []upgradingEntry{{Commit: "def", Header: "feat: c", Text: "Do it."}}}}}
	u.print()
	_, byHand, _ := strings.Cut(stdout.String(), "By hand:")
	if strings.Contains(byHand, "itos 9.2.0") || !strings.Contains(byHand, "itos 9.3.0") {
		t.Errorf("By hand names a release that asks nothing, or not the one that asks:\n%s", stdout.String())
	}
}

// --json lists the edits made and what is left by hand.
func TestUpgradeEmit(t *testing.T) {
	var stdout strings.Builder
	u := upgrade{o: Out{Stdout: &stdout, JSON: true}, from: "9.2.0", to: "9.3.0", edited: []string{}, releases: []any{
		releaseAsks{Version: "9.3.0", Upgrading: []upgradingEntry{{Text: "Do it."}}},
	}, applied: []appliedStep{{"9.3.0", configStep{"renamed", "b", "a"}, nil}, {"9.3.0", configStep{"removed", "c", ""}, nil}}}
	if err := u.emit("upgraded"); err != nil {
		t.Fatal(err)
	}
	want := `"applied":[{"version":"9.3.0","kind":"renamed","key":"b","from":"a"},` +
		`{"version":"9.3.0","kind":"removed","key":"c","value":null}],"manual":[{"version":"9.3.0","text":"Do it."}]}`
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(stdout.String())); err != nil || !strings.HasSuffix(compact.String(), want) {
		t.Errorf("emit (%v):\n%s\nwant it to end\n%s", err, compact.String(), want)
	}
}

// sumsOf is a checksums.txt of the version, for linux-amd64 and
// windows-amd64.
func sumsOf(v string) string {
	return linuxSum + "  itos-" + v + "-linux-amd64.tar.gz\n" + winSum + "  itos-" + v + "-windows-amd64.zip\n"
}

// upgradeIn is a project folder, the folder the test runs in, its config
// the text, and a release server whose newest is latest, offering 9.1.0,
// 9.2.0 and 9.3.0 and each one's upgrading.json, the files given laid over
// them.
func upgradeIn(t *testing.T, latest, config string, files map[string]string) string {
	t.Helper()
	served := map[string]string{"/latest/download/checksums.txt": sumsOf(latest)}
	for i, v := range []string{"9.1.0", "9.2.0", "9.3.0"} {
		served["/download/v"+v+"/checksums.txt"] = sumsOf(v)
		previous := ""
		if i > 0 {
			previous = []string{"9.1.0", "9.2.0"}[i-1]
		}
		served["/download/v"+v+"/upgrading.json"] = `{"schema": 1, "version": "` + v + `", "previous": "` + previous + `"}`
	}
	for path, body := range files {
		served[path] = body
	}
	releaseServer(t, served)
	dir := t.TempDir()
	t.Chdir(dir)
	file := filepath.Join(dir, "itos.yaml")
	t.Setenv("ITOS_CONFIG", file)
	if config != "" {
		if err := os.WriteFile(file, []byte(config), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return file
}

// pinOf is a config pinning the version, with its checksums.
func pinOf(v string) string {
	return "version: 1\npin: { version: " + v + ", checksums: " + release.SHA256([]byte(sumsOf(v))) + " }\n"
}

// upgradeRun is upgradeCommand's exit code and error, and what it printed.
func upgradeRun(json bool, args ...string) (int, string, error) {
	var stdout, stderr strings.Builder
	code, err := upgradeCommand(args, Out{JSON: json, Stdout: &stdout, Stderr: &stderr})
	return code, stdout.String() + stderr.String(), err
}

// What upgrade refuses, and how, each with every file left as it was.
func TestUpgradeCommandRefuses(t *testing.T) {
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
	upgradeIn(t, "9.3.0", pinOf("9.1.0"), nil)
	code, printed, err := upgradeRun(false, "--force")
	expect("a flag", 0, true, "upgrade takes only a <version>", code, err, printed)

	upgradeIn(t, "9.3.0", "", nil)
	code, printed, err = upgradeRun(false)
	expect("no config", 0, true, "cannot be read", code, err, printed)

	upgradeIn(t, "9.3.0", pinOf("9.1.0"), nil)
	if err := os.MkdirAll(pin.InstallScript, 0o755); err != nil {
		t.Fatal(err)
	}
	code, printed, err = upgradeRun(false)
	expect("an install script that cannot be read", 0, true, "cannot read "+pin.InstallScript, code, err, printed)

	upgradeIn(t, "9.3.0", "version: 1\n", nil)
	writeScript(t, "#!/bin/sh\n")
	code, printed, err = upgradeRun(false)
	expect("a script with no version", 0, true, "it has 0 version= lines", code, err, printed)

	upgradeIn(t, "9.2.0", pinOf("9.3.0"), nil)
	code, printed, err = upgradeRun(false)
	expect("a newest older than the pin", 1, false, "the newest itos, 9.2.0, is older than 9.3.0", code, err, printed)

	upgradeIn(t, "9.3.0", "version: 1\npin: { version: 9.3.0, checksums: \""+oldSum+"\" }\n", nil)
	code, printed, err = upgradeRun(false)
	expect("a release changed after it was pinned", 1, false, "the release changed after it was pinned", code, err, printed)

	upgradeIn(t, "9.3.0", pinOf("9.1.0"), map[string]string{"/download/v9.3.0/upgrading.json": "fail"})
	code, printed, err = upgradeRun(false)
	expect("an upgrading.json that cannot be fetched", ExitTemporary, false, "cannot fetch what itos 9.3.0 asks", code, err, printed)

	file := upgradeIn(t, "9.3.0", pinOf("9.1.0")+"ci:\n  watch: { timeout: 600 }\n", map[string]string{
		"/download/v9.3.0/itos.schema.json": `{"x-itos-removed": ["ci.watch.timeout"]}`,
	})
	code, printed, err = upgradeRun(false)
	expect("a config step it cannot make", 0, true, "cannot remove ci.watch.timeout in "+file, code, err, printed)
	if text, _ := os.ReadFile(file); string(text) != pinOf("9.1.0")+"ci:\n  watch: { timeout: 600 }\n" {
		t.Errorf("a config step it cannot make edited the config:\n%s", text)
	}

	upgradeIn(t, "9.3.0", "version: 1\npin: &p { version: 9.1.0, checksums: "+release.SHA256([]byte(sumsOf("9.1.0")))+" }\n", nil)
	code, printed, err = upgradeRun(false)
	expect("a pin it cannot edit", 0, true, "cannot edit the pin", code, err, printed)

	upgradeIn(t, "9.3.0", pinOf("9.1.0"), nil)
	writeScript(t, script("9.1.0", oldSum, oldSum)+"Darwin-arm64) platform=darwin-arm64 sum="+oldSum+" ;;\n")
	code, printed, err = upgradeRun(false)
	expect("a script it cannot edit", 0, true, "cannot edit "+pin.InstallScript, code, err, printed)
}

func writeScript(t *testing.T, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(pin.InstallScript), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pin.InstallScript, []byte(text), 0o755); err != nil {
		t.Fatal(err)
	}
}

// A file it cannot write is its error, the config's and the install
// script's alike. Skipped where a read-only file can be written anyway (as
// root).
func TestUpgradeCommandCannotWrite(t *testing.T) {
	file := upgradeIn(t, "9.3.0", pinOf("9.1.0"), nil)
	if err := os.Chmod(file, 0o444); err != nil {
		t.Fatal(err)
	}
	if f, err := os.OpenFile(file, os.O_WRONLY, 0); err == nil {
		f.Close()
		t.Skip("a read-only file can be written here")
	}
	if code, printed, err := upgradeRun(false); code != 0 || err == nil {
		t.Errorf("a config it cannot write: %d, %v, %s", code, err, printed)
	}

	upgradeIn(t, "9.3.0", "version: 1\n", nil)
	writeScript(t, script("9.1.0", oldSum, oldSum))
	if err := os.Chmod(pin.InstallScript, 0o555); err != nil {
		t.Fatal(err)
	}
	if code, printed, err := upgradeRun(false); code != 0 || err == nil {
		t.Errorf("a script it cannot write: %d, %v, %s", code, err, printed)
	}

	upgradeIn(t, "9.3.0", pinOf("9.1.0"), nil)
	writeFile(t, agentsFile, olderBlock)
	if err := os.Chmod(agentsFile, 0o444); err != nil {
		t.Fatal(err)
	}
	if code, printed, err := upgradeRun(false); code != 0 || err == nil {
		t.Errorf("an AGENTS.md it cannot write: %d, %v, %s", code, err, printed)
	}
}

// --json, already on the version and upgraded.
func TestUpgradeCommandJSON(t *testing.T) {
	upgradeIn(t, "9.3.0", pinOf("9.3.0"), nil)
	if code, printed, err := upgradeRun(true); code != 0 || err != nil || !strings.Contains(printed, `"action": "already"`) {
		t.Errorf("already: %d, %v, %s", code, err, printed)
	}
	// The version already on, named, is no move back.
	if code, printed, err := upgradeRun(false, "9.3.0"); code != 0 || err != nil ||
		!strings.Contains(printed, "already on itos 9.3.0: nothing to upgrade") {
		t.Errorf("already, named: %d, %v, %s", code, err, printed)
	}
	upgradeIn(t, "9.3.0", pinOf("9.1.0")+"ci:\n  watch:\n    timeout: 600\n", map[string]string{
		"/download/v9.3.0/itos.schema.json": `{"x-itos-removed": ["ci.watch.timeout"]}`,
	})
	code, printed, err := upgradeRun(true)
	if code != 0 || err != nil || !strings.Contains(printed, `"action": "upgraded"`) || !strings.Contains(printed, `"value": 600`) {
		t.Errorf("upgraded: %d, %v, %s", code, err, printed)
	}
}

// upgrade rewrites a present block of the rules for agents in this itos's
// words, listing AGENTS.md among its edits, and a project with no block
// keeps its AGENTS.md (slice 126).
func TestUpgradeCommandRewritesTheRulesBlock(t *testing.T) {
	upgradeIn(t, "9.3.0", pinOf("9.1.0"), nil)
	writeFile(t, agentsFile, "Mine.\n\n"+olderBlock)
	code, printed, err := upgradeRun(false)
	if text, _ := os.ReadFile(agentsFile); code != 0 || err != nil || string(text) != "Mine.\n\n"+rulesBlock ||
		!strings.Contains(printed, "itos.yaml and AGENTS.md, and committed nothing") {
		t.Errorf("an older block: %d, %v, %s\n%s", code, err, printed, text)
	}
	upgradeIn(t, "9.3.0", pinOf("9.1.0"), nil)
	writeFile(t, agentsFile, "Mine.\n")
	code, printed, err = upgradeRun(true)
	if text, _ := os.ReadFile(agentsFile); code != 0 || err != nil || string(text) != "Mine.\n" ||
		!strings.Contains(printed, `"action": "upgraded"`) || strings.Contains(printed, agentsFile) {
		t.Errorf("no block: %d, %v, %s\n%s", code, err, printed, text)
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
		if got := strings.Join(namingVersion(strings.Fields(in), "9.3.0"), " "); got != want {
			t.Errorf("namingVersion(%q) = %q, not %q", in, got, want)
		}
	}
}

// UpgradeTarget reads an upgrade the launcher can hand on, and leaves
// anything else to the binary called (slice 126).
func TestUpgradeTarget(t *testing.T) {
	upgradeIn(t, "9.3.0", "", nil)
	for _, line := range []string{"", "status", "upgrade --help", "help upgrade", "upgrade 9.1.0 9.2.0", "upgrade --force",
		"upgrade -- 9.2.0", "upgrade nine"} {
		if h, ok, err := UpgradeTarget(strings.Fields(line)); ok || err != nil || h.Version != "" {
			t.Errorf("%q: %v, %t, %v", line, h, ok, err)
		}
	}
	h, ok, err := UpgradeTarget([]string{"--json", "upgrade"})
	if !ok || err != nil || h.Version != "9.3.0" || string(h.Checksums) != sumsOf("9.3.0") ||
		!slices.Equal(h.Args, []string{"--json", "upgrade", "9.3.0"}) {
		t.Errorf("the newest: %v, %t, %v", h, ok, err)
	}
	h, ok, err = UpgradeTarget([]string{"upgrade", "v9.2.0"})
	if !ok || err != nil || h.Version != "9.2.0" || !slices.Equal(h.Args, []string{"upgrade", "9.2.0"}) {
		t.Errorf("a version: %v, %t, %v", h, ok, err)
	}
	h, ok, err = UpgradeTarget([]string{"upgrade", "9.9.0"})
	if !ok || kind.Of(err) != kind.Missing || h.Args != nil {
		t.Errorf("a version the server does not have: %v, %t, %v", h, ok, err)
	}
}
