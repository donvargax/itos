package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/shell"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
	trackerfake "github.com/donvargax/itos/v8/internal/store/tracker/port/porttest"
	treefake "github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	namedtests "github.com/donvargax/itos/v8/internal/tests"
	"github.com/donvargax/itos/v8/internal/value"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// commit is the one commit of the repository the VCS fake holds, which HEAD
// names.
const commit = "1111111111111111111111111111111111111111"

// kindText is the scenario kind every test here reads: features its root,
// an ID- pattern, @ its tag prefix, a select template and smoke.yaml its
// smoke set, every file needing a smoke test unless every_file says.
const kindText = "  scenario:\n    root: features\n    id: \"ID-[A-Z]+-\\\\d+\"\n    tag_prefix: \"@\"\n" +
	"    run: { select: \"run {pattern}\", ids_pattern: \"^@(?:{ids})$\" }\n" +
	"    smoke: { file: smoke.yaml, every_file: %s }\n"

// configText is the config of kindText, every_file as given.
func configText(everyFile string) string {
	return "version: 1\nwork: { registry: work-items.yaml }\ntests:\n" + strings.Replace(kindText, "%s", everyFile, 1)
}

// feature is a feature file of the scenarios with these tag lines.
func feature(tags ...string) string {
	text := "Feature: A\n"
	for i, tag := range tags {
		text += "  " + tag + "\n  Scenario: S" + strconv.Itoa(i) + "\n    Given a step\n"
	}
	return text
}

// smokeOf is a smoke set listing a.feature's ID-A-01.
const smokeOf = "- file: a.feature\n  scenarios:\n    - { id: \"@ID-A-01\", why: the file's point }\n"

// worktree is a working tree of a.feature, its ID-A-01 live and slice-3's
// ID-A-02 not, the smoke set listing ID-A-01.
func worktree() map[string]string {
	return map[string]string{
		"features/a.feature": feature("@ID-A-01 @slice-3", "@ID-A-02 @wip"),
		"smoke.yaml":         smokeOf,
	}
}

// ports are the tests command's ports over the fakes: a repository of one
// commit, the config or its failure, the tracker, and the shell's answer,
// each command it ran kept.
type ports struct {
	*vcsfake.VCS
	cfg     *config.Loaded
	err     error
	tracker *trackerfake.Tracker
	result  shell.Result
	ran     []string
}

func (p *ports) Config(context.Context) (*config.Loaded, error) { return p.cfg, p.err }

func (p *ports) Tracker(context.Context, *config.Loaded) trackerport.Tracker { return p.tracker }

func (p *ports) Shell(_ context.Context, _ *config.Loaded, command string) shell.Result {
	p.ran = append(p.ran, command)
	return p.result
}

// over is the ports of the config text over the trees, worktree's files
// the working tree's, the repository having its git folder at .git. The
// working tree's files are written in a scratch folder the test stands in
// too, where internal/tests reads a Gherkin kind's files.
func over(t *testing.T, text string, trees map[string]map[string]string) *ports {
	t.Helper()
	t.Chdir(t.TempDir())
	for name, text := range trees["worktree"] {
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	v := &vcsfake.VCS{
		Located: map[string]vcsport.Location{"": {Top: "/r", GitDir: ".git", Common: ".git"}},
		History: []vcsfake.Commit{{Name: commit}},
		Named:   map[string]string{"HEAD": commit},
	}
	tree := treefake.Trees{Common: ".git", Files: trees}
	src, err := tree.At(t.Context(), "worktree")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.FromText(v, tree, src, "itos.yaml", text, false)
	if err != nil {
		t.Fatal(err)
	}
	return &ports{VCS: v, cfg: cfg, tracker: &trackerfake.Tracker{}}
}

// overWorktree is over with the working tree's files alone.
func overWorktree(t *testing.T, text string, files map[string]string) *ports {
	t.Helper()
	return over(t, text, map[string]map[string]string{"worktree": files})
}

// call is the command's line run over p: its exit, stdout, stderr and
// error; a --json or --quiet first sets the global flag.
func call(t *testing.T, p *ports, args ...string) (int, string, string, error) {
	t.Helper()
	asJSON, quiet := false, false
	for len(args) > 0 && (args[0] == "--json" || args[0] == "--quiet") {
		asJSON, quiet = asJSON || args[0] == "--json", quiet || args[0] == "--quiet"
		args = args[1:]
	}
	c, err := Parse(args, "", asJSON, quiet)
	if err != nil {
		t.Fatalf("Parse(%q) = %v", args, err)
	}
	var stdout, stderr strings.Builder
	code, err := c.Run(t.Context(), &stdout, &stderr, NewHandler(p, namedtests.NewWarnings(&stderr)))
	return code, stdout.String(), stderr.String(), err
}

// compact is a --json object on one line.
func compact(text string) string {
	var b bytes.Buffer
	if json.Compact(&b, []byte(text)) != nil {
		return text
	}
	return b.String()
}

// want holds one call's answer to what it should be.
func want(t *testing.T, args []string, code int, stdout, stderr string, gotCode int, gotStdout, gotStderr string, err error) {
	t.Helper()
	if err != nil || gotCode != code || gotStdout != stdout || gotStderr != stderr {
		t.Errorf("tests %q = exit %d, %v\nstdout %q\nstderr %q\nwant exit %d, stdout %q, stderr %q",
			args, gotCode, err, gotStdout, gotStderr, code, stdout, stderr)
	}
}

func TestListPrintsTheKindsTestsAtATree(t *testing.T) {
	trees := map[string]map[string]string{
		"worktree": worktree(),
		"index":    {"features/a.feature": feature("@ID-A-01")},
		"HEAD":     {"features/a.feature": feature("@ID-A-09 @wip")},
	}
	for _, c := range []struct {
		args   []string
		stdout string
	}{
		{[]string{"list", "scenario"}, "ID-A-01  a.feature\nID-A-02  a.feature  (not live)\n"},
		{[]string{"list", "scenario", "--at", "worktree"}, "ID-A-01  a.feature\nID-A-02  a.feature  (not live)\n"},
		{[]string{"list", "scenario", "--at", "index"}, "ID-A-01  a.feature\n"},
		{[]string{"list", "scenario", "--at", "HEAD"}, "ID-A-09  a.feature  (not live)\n"},
		{[]string{"list", "scenario", "--at", "index", "--tsv"},
			"ID-A-01\tfeatures/a.feature\t13652fa0cb9e647dcda863f34bd523b67488ed1286c08dcb4a236f61fe1c4ce6\n"},
		{[]string{"list", "scenario", "--at", "HEAD", "--tsv"}, ""},
		{[]string{"--json", "list", "scenario", "--at", "index"},
			`{"schema":1,"protocol":1,"tests":[{"id":"ID-A-01","file":"a.feature","live":true,` +
				`"fingerprint":"13652fa0cb9e647dcda863f34bd523b67488ed1286c08dcb4a236f61fe1c4ce6"}],"files":["a.feature"]}`},
	} {
		code, stdout, stderr, err := call(t, over(t, configText("true"), trees), c.args...)
		if c.args[0] == "--json" {
			stdout = compact(stdout)
		}
		want(t, c.args, 0, c.stdout, "", code, stdout, stderr, err)
	}
}

// A TSV listing with a test no row can hold prints nothing on stdout and each
// such test on stderr, exit 1.
func TestTSVPrintsTheRefusedRowsOnStderr(t *testing.T) {
	var stdout, stderr strings.Builder
	c := CLI{Query: Query{Sub: "list", Kind: "fixture", TSV: true}}
	code, err := c.tsv(&stdout, &stderr, Answer{Rows: []namedtests.Row{{ID: "F-1", File: "a"}}, Refused: []string{"test 2 (id \"\", file \"b\"): its id is empty"}})
	want := "tests list --tsv: tests.fixture's test 2 (id \"\", file \"b\"): its id is empty, which a project-relative TSV row cannot hold\n"
	if err != nil || code != 1 || stdout.String() != "" || stderr.String() != want {
		t.Errorf("tsv = %d, %v, stdout %q, stderr %q, want 1 and %q on stderr alone", code, err, stdout.String(), stderr.String(), want)
	}
}

// failingWriter refuses every write with its error.
type failingWriter struct{ err error }

func (f failingWriter) Write([]byte) (int, error) { return 0, f.err }

// A row that stdout will not take stops the listing with that error.
func TestTSVStopsAtAStdoutThatFails(t *testing.T) {
	closed := errors.New("stdout is closed")
	var stderr strings.Builder
	c := CLI{Query: Query{Sub: "list", Kind: "scenario", TSV: true}}
	code, err := c.tsv(failingWriter{closed}, &stderr, Answer{Rows: []namedtests.Row{{ID: "ID-A-01", File: "features/a.feature"}}})
	if !errors.Is(err, closed) || code != 0 || stderr.String() != "" {
		t.Errorf("tsv to a failing stdout = %d, %v, stderr %q, want its error", code, err, stderr.String())
	}
}

// A tree that names no commit is refused before the config is read; a
// config that does not load, and a kind it does not have, stop the listing.
func TestListRefusesWhatItCannotList(t *testing.T) {
	p := overWorktree(t, configText("true"), worktree())
	p.err = errors.New("the config does not load")
	_, _, _, err := call(t, p, "list", "scenario", "--at", "nope")
	var u out.UsageError
	if !errors.As(err, &u) || err.Error() != "tests list: nope is not a commit of this repository" {
		t.Errorf("list --at nope = %v, want the ref refused", err)
	}
	if code, _, _, err := call(t, p, "list", "scenario"); code != 0 || !errors.Is(err, p.err) {
		t.Errorf("list over a config that does not load = %v, want its error", err)
	}
	p.err = nil
	if _, stdout, _, err := call(t, p, "list", "unit"); err == nil || !strings.Contains(err.Error(), "tests.unit is missing") || stdout != "" {
		t.Errorf("list unit = %q, %v, want the kind missing", stdout, err)
	}

	commandConfig := "version: 1\nwork: { registry: work-items.yaml }\ntests:\n" +
		"  scenario:\n    adapter: { command: \"false\", supports_at: true }\n    id: 'ID-[A-Z]+-\\d+'\n"
	p = overWorktree(t, commandConfig, worktree())
	if code, _, _, err := call(t, p, "list", "scenario", "--tsv"); code != 0 || err == nil {
		t.Errorf("list --tsv over a failing adapter = exit %d, %v, want its error", code, err)
	}
}

// A command adapter's object keeps its keys, in its order, extra ones
// included; the built-in adapter's is the protocol's.
func TestListFieldsKeepACommandAdaptersObject(t *testing.T) {
	raw, err := value.ParseJSON(`{"tests":[],"protocol":1,"files":["a"],"extra":{"b":true}}`)
	if err != nil {
		t.Fatal(err)
	}
	var keys, values []string
	for _, f := range listFields(namedtests.List{Raw: raw}) {
		keys = append(keys, f.Key)
		values = append(values, string(f.Value.(json.RawMessage)))
	}
	if strings.Join(keys, " ") != "tests protocol files extra" || strings.Join(values, " ") != `[] 1 ["a"] {"b":true}` {
		t.Errorf("listFields = %q %q, want the adapter's keys and values in order", keys, values)
	}
	if fields := listFields(namedtests.List{Protocol: 1}); len(fields) != 3 || fields[0].Key != "protocol" {
		t.Errorf("listFields of the built-in adapter = %+v, want protocol, tests and files", fields)
	}
}

// moves judges the index against HEAD: a scenario moved passes, one lost
// is a problem, exit 1.
func TestMovesJudgesTheIndexAgainstHEAD(t *testing.T) {
	head := map[string]string{"features/a.feature": feature("@ID-A-01", "@ID-A-02")}
	moved := map[string]string{"features/a.feature": feature("@ID-A-01"), "features/b.feature": "Feature: B\n  @ID-A-02\n  Scenario: S1\n    Given a step\n"}
	lost := map[string]string{"features/a.feature": feature("@ID-A-01")}
	text := configText("true")
	for _, c := range []struct {
		args           []string
		index          map[string]string
		code           int
		stdout, stderr string
	}{
		{[]string{"moves", "scenario"}, moved, 0, "The staged feature files only move scenarios, if anything\n", ""},
		{[]string{"--quiet", "moves", "scenario"}, moved, 0, "", ""},
		{[]string{"--json", "moves", "scenario"}, moved, 0, `{"schema":1,"kind":"scenario","ok":true,"problems":[]}`, ""},
		{[]string{"moves", "scenario"}, lost, 1, "", "  - "},
		{[]string{"--json", "moves", "scenario"}, lost, 1, `{"schema":1,"kind":"scenario","ok":false,"problems":[{"rule":"moves"`, ""},
	} {
		p := over(t, text, map[string]map[string]string{"worktree": {}, "HEAD": head, "index": c.index})
		code, stdout, stderr, err := call(t, p, c.args...)
		if c.args[0] == "--json" {
			stdout = compact(stdout)
		}
		if err != nil || code != c.code || !strings.HasPrefix(stdout, c.stdout) || !strings.HasPrefix(stderr, c.stderr) ||
			(c.stdout == "" && stdout != "") || (c.stderr == "" && stderr != "") {
			t.Errorf("tests %q = exit %d, %v\nstdout %q\nstderr %q", c.args, code, err, stdout, stderr)
		}
	}
	p := over(t, text, map[string]map[string]string{"worktree": {}, "HEAD": head, "index": lost})
	p.err = errors.New("the config does not load")
	if _, _, _, err := call(t, p, "moves", "scenario"); !errors.Is(err, p.err) {
		t.Errorf("moves over a config that does not load = %v, want its error", err)
	}
	p.err = nil
	if _, _, _, err := call(t, p, "moves", "unit"); err == nil {
		t.Error("moves of a kind the config does not have: no error")
	}
	c, err := Parse([]string{"moves", "scenario"}, "", true, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Run(t.Context(), failing{}, failing{}, NewHandler(p, nil)); err == nil {
		t.Error("moves --json to a stream that cannot be written: no error")
	}
}

// failing is a writer that cannot be written to.
type failing struct{}

func (failing) Write([]byte) (int, error) { return 0, errors.New("closed") }

// The smoke rule holds when every file has a live smoke test, worded by
// every_file; a smoke ID that is not live fails it, exit 1.
func TestSmokeCheckJudgesTheSmokeRule(t *testing.T) {
	broken := worktree()
	broken["smoke.yaml"] = "- file: a.feature\n  scenarios:\n    - { id: \"@ID-A-02\", why: not live }\n"
	for _, c := range []struct {
		args           []string
		everyFile      string
		files          map[string]string
		code           int
		stdout, stderr string
	}{
		{[]string{"smoke", "check", "scenario"}, "true", worktree(), 0, "Every feature file has a smoke scenario (1 in all)\n", ""},
		{[]string{"smoke", "check", "scenario"}, "false", worktree(), 0, "Every smoke scenario is live (1 in all)\n", ""},
		{[]string{"--quiet", "smoke", "check", "scenario"}, "true", worktree(), 0, "", ""},
		{[]string{"--json", "smoke", "check", "scenario"}, "true", worktree(), 0, `{"schema":1,"kind":"scenario","ok":true,"ids":1,"problems":[]}`, ""},
		{[]string{"smoke", "check", "scenario"}, "true", broken, 1, "", "FAIL "},
		{[]string{"--json", "smoke", "check", "scenario"}, "true", broken, 1, `{"schema":1,"kind":"scenario","ok":false,"ids":1,"problems":[{`, ""},
	} {
		code, stdout, stderr, err := call(t, overWorktree(t, configText(c.everyFile), c.files), c.args...)
		if c.args[0] == "--json" {
			stdout = compact(stdout)
		}
		if err != nil || code != c.code || !strings.HasPrefix(stdout, c.stdout) || !strings.HasPrefix(stderr, c.stderr) ||
			(c.stdout == "" && stdout != "") || (c.stderr == "" && stderr != "") {
			t.Errorf("tests %q = exit %d, %v\nstdout %q\nstderr %q", c.args, code, err, stdout, stderr)
		}
	}
}

// --features reads a copy of the kind's files in its stead.
func TestSmokeCheckReadsTheFeaturesCopy(t *testing.T) {
	files := worktree()
	files["copy/a.feature"] = feature("@ID-A-01 @wip")
	code, _, stderr, err := call(t, overWorktree(t, configText("true"), files), "smoke", "check", "scenario", "--features", "copy")
	if err != nil || code != 1 || !strings.Contains(stderr, "ID-A-01") {
		t.Errorf("smoke check --features copy = exit %d, %v, stderr %q, want ID-A-01 not live in the copy", code, err, stderr)
	}
}

// A config that does not load, a smoke set that is not there and a kind
// with no root stop every smoke subcommand; --json to a stream that
// cannot be written is an error.
func TestSmokeStopsAtWhatItCannotRead(t *testing.T) {
	noRoot := strings.Replace(configText("true"), "    root: features\n", "", 1)
	for _, args := range [][]string{{"smoke", "check", "scenario"}, {"smoke", "ids", "scenario"}, {"smoke", "run", "scenario"}} {
		p := overWorktree(t, configText("true"), worktree())
		p.err = errors.New("the config does not load")
		if _, _, _, err := call(t, p, args...); !errors.Is(err, p.err) {
			t.Errorf("tests %q over a config that does not load = %v, want its error", args, err)
		}
		if _, _, _, err := call(t, overWorktree(t, configText("true"), map[string]string{}), args...); err == nil || !strings.Contains(err.Error(), "smoke.yaml is missing") {
			t.Errorf("tests %q with no smoke set = %v, want it missing", args, err)
		}
	}
	if _, _, _, err := call(t, overWorktree(t, noRoot, worktree()), "smoke", "check", "scenario"); err == nil || !strings.Contains(err.Error(), "root is missing") {
		t.Errorf("smoke check of a kind with no root = %v, want the root missing", err)
	}
	c, err := Parse([]string{"smoke", "check", "scenario"}, "", true, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Run(t.Context(), failing{}, failing{}, NewHandler(overWorktree(t, configText("true"), worktree()), nil)); err == nil {
		t.Error("smoke check --json to a stream that cannot be written: no error")
	}
}

func TestSmokeIDsPrintsTheSmokeSet(t *testing.T) {
	p := overWorktree(t, configText("true"), worktree())
	code, stdout, stderr, err := call(t, p, "smoke", "ids", "scenario")
	want(t, []string{"smoke", "ids"}, 0, "ID-A-01\n", "", code, stdout, stderr, err)
	code, stdout, stderr, err = call(t, p, "--json", "smoke", "ids", "scenario")
	want(t, []string{"--json", "smoke", "ids"}, 0, `{"schema":1,"kind":"scenario","ids":["ID-A-01"]}`, "", code, compact(stdout), stderr, err)
}

// smoke run runs the select template over the smoke IDs, each argument
// after the kind one shell word after it; its exit is the runner's, 1 when
// the runner did not exit by itself or could not start.
func TestSmokeRunRunsTheSmokeSet(t *testing.T) {
	for _, c := range []struct {
		result shell.Result
		code   int
	}{
		{shell.Result{Code: 0}, 0},
		{shell.Result{Code: 3}, 3},
		{shell.Result{Code: -1}, 1},
		{shell.Result{Code: 0, Err: errors.New("cannot start")}, 1},
	} {
		p := overWorktree(t, configText("true"), worktree())
		p.result = c.result
		code, stdout, stderr, err := call(t, p, "smoke", "run", "scenario", "--", "-v", "it's")
		if err != nil || code != c.code || stdout != "" || stderr != "" ||
			strings.Join(p.ran, "\n") != `run '^@(?:ID-A-01)$' '-v' 'it'\''s'` {
			t.Errorf("smoke run of %+v = exit %d, %v, ran %q, stdout %q, stderr %q", c.result, code, err, p.ran, stdout, stderr)
		}
	}
}

// An empty smoke set runs nothing and says so; a kind with no select
// template cannot run one.
func TestSmokeRunOfNothingRunsNothing(t *testing.T) {
	files := worktree()
	files["smoke.yaml"] = "[]\n"
	p := overWorktree(t, configText("true"), files)
	code, stdout, stderr, err := call(t, p, "smoke", "run", "scenario")
	want(t, []string{"smoke", "run"}, 0, "", "The smoke set of tests.scenario is empty: there is nothing to run\n", code, stdout, stderr, err)
	if len(p.ran) > 0 {
		t.Errorf("an empty smoke set ran %q", p.ran)
	}
	noSelect := strings.Replace(configText("true"), "    run: { select: \"run {pattern}\", ids_pattern: \"^@(?:{ids})$\" }\n", "", 1)
	p = overWorktree(t, noSelect, worktree())
	if _, _, _, err := call(t, p, "smoke", "run", "scenario"); err == nil || len(p.ran) > 0 {
		t.Errorf("smoke run with no select template = %v, ran %q, want a config error", err, p.ran)
	}
}

// An area's tag is claimed through the tracker, so the next run gives the
// one after; --count claims a run; any other stem only reads.
func TestNextIDClaimsAnAreasTagsAndOnlyReadsAnyOtherStem(t *testing.T) {
	p := overWorktree(t, configText("true"), worktree())
	for _, c := range []struct {
		args   []string
		stdout string
	}{
		{[]string{"next-id", "scenario", "ID-A"}, "@ID-A-03\n"},
		{[]string{"next-id", "scenario", "ID-A"}, "@ID-A-04\n"},
		{[]string{"next-id", "scenario", "ID-A", "--count", "2"}, "@ID-A-05\n@ID-A-06\n"},
		{[]string{"next-id", "scenario", "ID-NEW"}, "@ID-NEW-01\n"},
		{[]string{"next-id", "scenario", "slice"}, "@slice-4\n"},
		{[]string{"next-id", "scenario", "slice"}, "@slice-4\n"},
		{[]string{"--json", "next-id", "scenario", "ID-A", "--count", "2"},
			`{"schema":1,"kind":"scenario","stem":"ID-A","id":"@ID-A-07","ids":["@ID-A-07","@ID-A-08"],"claimed":true}`},
		{[]string{"--json", "next-id", "scenario", "slice"},
			`{"schema":1,"kind":"scenario","stem":"slice","id":"@slice-4","ids":["@slice-4"],"claimed":false}`},
	} {
		code, stdout, stderr, err := call(t, p, c.args...)
		if c.args[0] == "--json" {
			stdout = compact(stdout)
		}
		want(t, c.args, 0, c.stdout, "", code, stdout, stderr, err)
	}
	if first, err := p.tracker.Reserve(t.Context(), "slice", 0, 1); err != nil || first != 1 {
		t.Errorf("slice's counter gives %d, %v after only reads, want 1", first, err)
	}
}

// A kind the config does not have and a --count of a stem that only reads
// are usage errors; a config, a registry or a listing that cannot be read
// and a tag with no number to claim after stop it; none claims anything.
func TestNextIDRefusesWhatItCannotRead(t *testing.T) {
	var u out.UsageError
	p := overWorktree(t, configText("true"), worktree())
	if _, _, _, err := call(t, p, "next-id", "unit", "ID-A"); !errors.As(err, &u) ||
		err.Error() != "tests next-id: the config has no tests kind unit (it has scenario)" {
		t.Errorf("next-id unit = %v, want the kind refused", err)
	}
	if _, _, _, err := call(t, overWorktree(t, "version: 1\n", nil), "next-id", "unit", "ID-A"); !errors.As(err, &u) ||
		!strings.HasSuffix(err.Error(), "(it has none)") {
		t.Errorf("next-id with no kinds = %v, want none listed", err)
	}
	if _, _, _, err := call(t, p, "next-id", "scenario", "slice", "--count", "1"); !errors.As(err, &u) ||
		err.Error() != "tests next-id --count claims an area's ids, and slice is no area of tests.scenario's id: its next id is only read" {
		t.Errorf("next-id slice --count 1 = %v, want --count refused", err)
	}
	p.err = errors.New("the config does not load")
	if _, _, _, err := call(t, p, "next-id", "scenario", "ID-A"); !errors.Is(err, p.err) {
		t.Errorf("next-id over a config that does not load = %v, want its error", err)
	}
	registry := worktree()
	registry["work-items.yaml"] = "items: [\n"
	if _, _, _, err := call(t, overWorktree(t, configText("true"), registry), "next-id", "scenario", "ID-A"); err == nil {
		t.Error("next-id over a registry that does not load: no error")
	}
	noRoot := strings.Replace(configText("true"), "    root: features\n", "", 1)
	if _, _, _, err := call(t, overWorktree(t, noRoot, worktree()), "next-id", "scenario", "ID-A"); err == nil || !strings.Contains(err.Error(), "root is missing") {
		t.Errorf("next-id of a kind with no root = %v, want the root missing", err)
	}
	past := map[string]string{"features/a.feature": feature("@ID-A-" + strconv.Itoa(math.MaxInt))}
	p = overWorktree(t, configText("true"), past)
	if _, stdout, _, err := call(t, p, "next-id", "scenario", "ID-A"); err == nil || stdout != "" || !strings.Contains(err.Error(), "cannot read the number") {
		t.Errorf("next-id past the largest int = %q, %v, want the number refused", stdout, err)
	}
	if first, err := p.tracker.Reserve(t.Context(), "ID-A", 0, 1); err != nil || first != 1 {
		t.Errorf("the counter after refused claims gives %d, %v, want 1", first, err)
	}
}

// A bare stem that would be an area with ID- in front is refused with the
// command that claims it, --count kept, and claims nothing; a stem that a tag
// or a registry item already uses as a series, slice and bug stay previews,
// and the refusal of --count for them is the old one.
func TestNextIDRefusesAMistakenBareAreaWithItsCorrection(t *testing.T) {
	files := worktree()
	files["features/b.feature"] = feature("@OPS-2")
	files["work-items.yaml"] = "items:\n  - { id: batch-9, title: B, phase: 1, owner: null, status: todo, depends_on: [] }\n"
	p := overWorktree(t, strings.Replace(configText("true"), `ID-[A-Z]+-\\d+`, `ID-[A-Za-z]+-\\d+`, 1), files)
	var u out.UsageError
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"next-id", "scenario", "SEC"}, `tests next-id: SEC is no area of tests.scenario's id ID-[A-Za-z]+-\d+, and ID-SEC is: to claim its ids, run itos tests next-id scenario ID-SEC`},
		{[]string{"next-id", "scenario", "SEC", "--count", "3"}, `tests next-id: SEC is no area of tests.scenario's id ID-[A-Za-z]+-\d+, and ID-SEC is: to claim its ids, run itos tests next-id scenario ID-SEC --count 3`},
	} {
		if _, stdout, _, err := call(t, p, c.args...); !errors.As(err, &u) || err.Error() != c.want || stdout != "" {
			t.Errorf("%q = %q, %v, want the usage error %q", c.args, stdout, err, c.want)
		}
	}
	for _, c := range []struct {
		args   []string
		stdout string
	}{
		{[]string{"next-id", "scenario", "OPS"}, "@OPS-3\n"},
		{[]string{"next-id", "scenario", "batch"}, "@batch-10\n"},
		{[]string{"next-id", "scenario", "slice"}, "@slice-4\n"},
		{[]string{"next-id", "scenario", "bug"}, "@bug-1\n"},
	} {
		code, stdout, stderr, err := call(t, p, c.args...)
		want(t, c.args, 0, c.stdout, "", code, stdout, stderr, err)
	}
	for _, stem := range []string{"OPS", "batch", "slice", "bug"} {
		if _, _, _, err := call(t, p, "next-id", "scenario", stem, "--count", "2"); !errors.As(err, &u) ||
			!strings.HasSuffix(err.Error(), stem+" is no area of tests.scenario's id: its next id is only read") {
			t.Errorf("next-id %s --count 2 = %v, want --count refused", stem, err)
		}
	}
	if first, err := p.tracker.Reserve(t.Context(), "ID-SEC", 0, 1); err != nil || first != 1 {
		t.Errorf("the counter of ID-SEC gives %d, %v after refusals, want 1", first, err)
	}
}
