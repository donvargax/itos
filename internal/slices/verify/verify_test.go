package verify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/out"
	treefake "github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	"github.com/donvargax/itos/v8/internal/tests"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// The rules every test judges by: docs may touch Markdown alone.
const rules = "version: 1\ncommits:\n  types: [docs]\n  scopes: { docs: { only: [\"**/*.md\"] } }\n"

// root is the first commit, a full name, which commits.since may name.
var root = strings.Repeat("ab", 20)

// missing is a full commit name the repository lacks.
var missing = strings.Repeat("cd", 20)

// ports are the fakes: the repository, the stealth check's answer and the
// config, or the error loading it gives; asked counts the stealth checks.
type ports struct {
	*vcsfake.VCS
	stealth bool
	asked   *int
	cfg     *config.Loaded
	err     error
}

func (p ports) Stealth(context.Context) bool {
	*p.asked++
	return p.stealth
}

func (p ports) Config(context.Context) (*config.Loaded, error) { return p.cfg, p.err }

// repository is a root, a docs commit that holds and one touching code,
// which does not, HEAD on the last; local adds neither to a remote branch.
func repository(located bool) *vcsfake.VCS {
	v := &vcsfake.VCS{
		History: []vcsfake.Commit{
			{Name: root, Message: "docs: start\n", Tree: map[string]string{"a.md": "a\n"}},
			{Name: "good", Parents: []string{root}, Message: "docs: add b\n", Tree: map[string]string{"a.md": "a\n", "b.md": "b\n"}},
			{Name: "bad", Parents: []string{"good"}, Message: "docs: add code\n", Tree: map[string]string{"a.md": "a\n", "b.md": "b\n", "c.go": "package c\n"}},
		},
		Named: map[string]string{"HEAD": "bad", "refs/remotes/origin/main": root},
	}
	if located {
		v.Located = map[string]vcsport.Location{"": {Top: "/r", GitDir: ".git", Common: ".git"}}
	}
	return v
}

// over is the ports over repo with the config text, as stealth says.
func over(t *testing.T, repo *vcsfake.VCS, text string, stealth bool) ports {
	t.Helper()
	trees := treefake.Trees{Common: ".git", Files: map[string]map[string]string{"worktree": {}}}
	src, err := trees.At(t.Context(), "worktree")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.FromText(repo, trees, src, "itos.yaml", text, false)
	if err != nil {
		t.Fatal(err)
	}
	return ports{VCS: repo, stealth: stealth, asked: new(int), cfg: cfg}
}

// run is verify's command line over p, its exit, error and streams.
func run(t *testing.T, args []string, json bool, p ports) (int, error, string, string) {
	t.Helper()
	c, err := Parse(args, json, &tests.Warnings{})
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	code, err := c.Run(t.Context(), &stdout, &stderr, NewHandler(p))
	return code, err, stdout.String(), stderr.String()
}

func isUsage(err error, text string) bool {
	var u out.UsageError
	return errors.As(err, &u) && err.Error() == text
}

func TestParseTakesARangeOrNone(t *testing.T) {
	warnings := &tests.Warnings{}
	c, err := Parse([]string{"a", "b", "c"}, true, warnings)
	if err != nil || c.From != "a" || c.To != "b" || c.Bare || !c.JSON || c.Tests != warnings {
		t.Errorf("Parse(a b c) = %+v, %v", c, err)
	}
	c, err = Parse(nil, false, nil)
	if err != nil || !c.Bare || c.From != "" || c.To != "" || c.JSON {
		t.Errorf("Parse() = %+v, %v", c, err)
	}
	if _, err := Parse([]string{"a"}, false, nil); !isUsage(err, "verify needs <from> <to>") {
		t.Errorf("Parse(a) = %v, want the usage error", err)
	}
}

// A range's every commit is judged, the count on stdout; one that fails is
// exit 1.
func TestRunJudgesTheRange(t *testing.T) {
	p := over(t, repository(true), rules, false)
	code, err, stdout, stderr := run(t, []string{root, "good"}, false, p)
	if err != nil || code != 0 || stdout != "1/1 commits pass the commit rules\n" || stderr != "" {
		t.Errorf("root..good: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	code, err, stdout, _ = run(t, []string{root, "HEAD"}, false, p)
	if err != nil || code != refused || !strings.HasSuffix(stdout, "1/2 commits pass the commit rules\n") {
		t.Errorf("root..HEAD: exit %d, %v, stdout %q", code, err, stdout)
	}
	if *p.asked != 0 {
		t.Errorf("a typed range asked the stealth check %d times", *p.asked)
	}
}

// Under --json stdout holds one object, commits.since in it when the config
// names one, and the logs go to stderr.
func TestRunJSON(t *testing.T) {
	p := over(t, repository(true), rules+"  since: "+root+"\n", false)
	code, err, stdout, stderr := run(t, []string{"", "HEAD"}, true, p)
	if err != nil || code != refused {
		t.Fatalf("exit %d, %v", code, err)
	}
	want := `{"schema":1,"range":{"from":"","to":"HEAD"},"since":"` + root + `","commits":[{"sha":"good","header":"docs: add b","ok":true},` +
		`{"sha":"bad","header":"docs: add code","ok":false}],"passed":1,"total":2,"range_checks":true}`
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(stdout)); err != nil || compact.String() != want || strings.Count(stdout, "\n}") != 1 {
		t.Errorf("stdout %s, want %s", stdout, want)
	}
	if !strings.Contains(stderr, "Not checked: abababa (commits.since)") || !strings.HasSuffix(stderr, "1/2 commits pass the commit rules\n") {
		t.Errorf("stderr %q", stderr)
	}
	_, _, stdout, _ = run(t, []string{root, "good"}, true, over(t, repository(true), rules, false))
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["since"]; ok {
		t.Errorf("no commits.since, yet since in %s", stdout)
	}
}

// With no range, a project's config is the usage error, after the stealth
// check; the stealth one judges HEAD's commits on no remote branch.
func TestRunWithNoRangeAsksTheStealthCheck(t *testing.T) {
	p := over(t, repository(true), rules, false)
	if _, err, _, _ := run(t, nil, false, p); !isUsage(err, "verify needs <from> <to>") || *p.asked != 1 {
		t.Errorf("a project's config: %v, the stealth check asked %d times", err, *p.asked)
	}
	p = over(t, repository(true), rules, true)
	code, err, stdout, _ := run(t, nil, true, p)
	var got struct {
		Range span `json:"range"`
		Total int  `json:"total"`
	}
	if err != nil || code != refused || json.Unmarshal([]byte(stdout), &got) != nil ||
		got.Range != (span{vcsport.Unpushed, "HEAD"}) || got.Total != 2 {
		t.Errorf("the stealth config: exit %d, %v, %s", code, err, stdout)
	}
}

// A typed ref that names no commit is refused before the config is read.
func TestRunRefusesARefThatNamesNoCommit(t *testing.T) {
	p := over(t, repository(true), rules, false)
	p.cfg, p.err = nil, errors.New("read")
	if _, err, _, _ := run(t, []string{root, "nope"}, false, p); !isUsage(err, "verify: nope is not a commit of this repository") {
		t.Errorf("got %v", err)
	}
}

// The config's failure to load, and the judge's to read the range, are the
// command line's to report.
func TestRunPassesOnErrors(t *testing.T) {
	p := over(t, repository(true), rules, false)
	read := errors.New("read")
	p.cfg, p.err = nil, read
	if code, err, _, _ := run(t, []string{root, "HEAD"}, false, p); code != 0 || !errors.Is(err, read) {
		t.Errorf("the config: exit %d, %v", code, err)
	}
	p = over(t, repository(false), rules, false)
	if code, err, _, _ := run(t, []string{"nope", "HEAD"}, false, p); code != 0 || err == nil {
		t.Errorf("an unreadable range: exit %d, %v", code, err)
	}
}

// A since that is no commit here is exit 2 with nothing judged: a FAIL line
// a problem, or under --json the range and the problems.
func TestRunRefusesASinceThatIsNoCommit(t *testing.T) {
	p := over(t, repository(true), rules+"  since: "+missing+"\n", false)
	code, err, stdout, stderr := run(t, []string{root, "HEAD"}, false, p)
	want := "FAIL commits.since names " + missing + ", which is not a commit of this repository\n"
	if err != nil || code != unusable || stdout != "" || stderr != want {
		t.Errorf("exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	code, err, stdout, stderr = run(t, []string{root, "HEAD"}, true, p)
	var got struct {
		Range    span          `json:"range"`
		Problems []out.Problem `json:"problems"`
	}
	if err != nil || code != unusable || stderr != "" || json.Unmarshal([]byte(stdout), &got) != nil ||
		got.Range != (span{root, "HEAD"}) || len(got.Problems) != 1 {
		t.Errorf("--json: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
}

// failing is a writer that takes nothing.
type failing struct{}

func (failing) Write([]byte) (int, error) { return 0, errors.New("closed") }

// A --json object that cannot be written is an error.
func TestRunReportsAnObjectNotWritten(t *testing.T) {
	c, err := Parse([]string{root, "good"}, true, &tests.Warnings{})
	if err != nil {
		t.Fatal(err)
	}
	var stderr strings.Builder
	code, err := c.Run(t.Context(), failing{}, &stderr, NewHandler(over(t, repository(true), rules, false)))
	if code != 0 || err == nil {
		t.Errorf("exit %d, %v", code, err)
	}
}
