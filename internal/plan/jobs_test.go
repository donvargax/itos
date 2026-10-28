package plan

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/store/tree/adapter"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

const jobsConfig = scratch + `  jobs:
    platform: { paths: ["internal/**", go.mod] }
    docs: { paths: ["docs/**"] }
    lint: {}
    empty: { paths: [] }
`

// loadAt is a config written at path (from the folder the test stands in)
// and loaded from there.
func loadAt(t *testing.T, path, text string) *config.Loaded {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(vcs.Git{}, adapter.Trees{VCS: vcs.Git{}}, adapter.Worktree, abs)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func answers(jobs []JobRun) string {
	var b strings.Builder
	for _, j := range jobs {
		b.WriteString(j.Name + "=")
		if j.Runs {
			b.WriteString("runs:")
		} else {
			b.WriteString("skips:")
		}
		b.WriteString(j.Reason + " ")
	}
	return strings.TrimSpace(b.String())
}

// A job runs for the first reason that holds, in the config's order: it
// declares no paths (or an empty list), the range is whole, the range touches
// the config file or a workflow, else by its paths.
func TestJobsOfGivesTheFirstReasonThatHolds(t *testing.T) {
	gitRange(t)
	cfg := loadAt(t, "itos.yaml", jobsConfig)
	for _, c := range []struct {
		name    string
		whole   bool
		changed []string
		want    string
	}{
		{"a path of one job", false, []string{"README.md", "internal/x.go"},
			"platform=runs:touched docs=skips:untouched lint=runs:no-paths empty=runs:no-paths"},
		{"a path of another", false, []string{"docs/a.md"},
			"platform=skips:untouched docs=runs:touched lint=runs:no-paths empty=runs:no-paths"},
		{"an exact path", false, []string{"go.mod"},
			"platform=runs:touched docs=skips:untouched lint=runs:no-paths empty=runs:no-paths"},
		{"no path", false, nil,
			"platform=skips:untouched docs=skips:untouched lint=runs:no-paths empty=runs:no-paths"},
		{"a range that is whole", true, []string{"docs/a.md"},
			"platform=runs:whole docs=runs:whole lint=runs:no-paths empty=runs:no-paths"},
		{"the config", false, []string{"itos.yaml"},
			"platform=runs:config docs=runs:config lint=runs:no-paths empty=runs:no-paths"},
		{"a workflow", false, []string{".github/workflows/ci.yml"},
			"platform=runs:workflow docs=runs:workflow lint=runs:no-paths empty=runs:no-paths"},
		{"a workflow's folder alone", false, []string{".github/workflows", ".github/x.yml"},
			"platform=skips:untouched docs=skips:untouched lint=runs:no-paths empty=runs:no-paths"},
		{"the config and a workflow", false, []string{".github/workflows/ci.yml", "itos.yaml"},
			"platform=runs:config docs=runs:config lint=runs:no-paths empty=runs:no-paths"},
		{"another file named itos.yaml", false, []string{"sub/itos.yaml"},
			"platform=skips:untouched docs=skips:untouched lint=runs:no-paths empty=runs:no-paths"},
	} {
		jobs, err := jobsOf(cfg, c.whole, c.changed)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := answers(jobs); got != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.name, got, c.want)
		}
	}
	if jobs, err := jobsOf(load(t, scratch), false, []string{"a"}); err != nil || len(jobs) != 0 {
		t.Errorf("a config that declares no job: %v, %v", jobs, err)
	}
}

// A glob itos cannot read is an error only once a path reaches it, as the
// prose shortcut's is. Loading does not refuse one, so it is set after.
func TestJobsOfStopsAtAGlobItCannotRead(t *testing.T) {
	gitRange(t)
	cfg := loadAt(t, "itos.yaml", jobsConfig)
	job := cfg.CI.Jobs.Values["docs"]
	job.Paths = []string{"?docs"}
	cfg.CI.Jobs.Values["docs"] = job
	if jobs, err := jobsOf(cfg, false, nil); err != nil || len(jobs) != 4 {
		t.Errorf("no path reaches the glob: %v, %v", jobs, err)
	}
	if jobs, err := jobsOf(cfg, false, []string{"docs/a.md"}); err == nil || jobs != nil {
		t.Errorf("a path reaches the glob: %v, %v", jobs, err)
	}
	if jobs, err := jobsOf(cfg, true, []string{"docs/a.md"}); err != nil || len(jobs) != 4 {
		t.Errorf("a whole range reads no glob: %v, %v", jobs, err)
	}
}

// The config file is placed from the repository's top level, its symlinks
// resolved; one outside the working tree, one that is not there and a folder
// that is no repository place none.
func TestConfigRelPlacesTheConfigFromTheTop(t *testing.T) {
	gitRange(t)
	top, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if got := configRel(loadAt(t, "itos.yaml", "version: 1\n")); got != "itos.yaml" {
		t.Errorf("the top's itos.yaml: %q", got)
	}
	if got := configRel(loadAt(t, filepath.Join(top, "ci", "itos.yaml"), "version: 1\n")); got != "ci/itos.yaml" {
		t.Errorf("a config in a folder: %q", got)
	}

	if err := os.Mkdir("real", 0o755); err != nil {
		t.Fatal(err)
	}
	real := loadAt(t, filepath.Join("real", "itos.yaml"), "version: 1\n")
	if err := os.Symlink(filepath.Join(top, "real"), "link"); err != nil {
		t.Skipf("no symbolic links here: %v", err)
	}
	linked := *real
	linked.Path = filepath.Join(top, "link", "itos.yaml")
	if got := configRel(&linked); got != "real/itos.yaml" {
		t.Errorf("a config reached through a link: %q", got)
	}

	elsewhere := *real
	elsewhere.Path = filepath.Join(t.TempDir(), "itos.yaml")
	if err := os.WriteFile(elsewhere.Path, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := configRel(&elsewhere); got != "" {
		t.Errorf("a config outside the working tree: %q", got)
	}
	inGit := *real
	inGit.Path = filepath.Join(top, ".git", "itos", "itos.yaml")
	if err := os.MkdirAll(filepath.Dir(inGit.Path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inGit.Path, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := configRel(&inGit); got != ".git/itos/itos.yaml" {
		t.Errorf("a config in the git folder is a path no range touches: %q", got)
	}
	missing := *real
	missing.Path = filepath.Join(top, "none.yaml")
	if got := configRel(&missing); got != "" {
		t.Errorf("a config that is not there: %q", got)
	}
	none := *real
	none.Path = ""
	if got := configRel(&none); got != "" {
		t.Errorf("no config file: %q", got)
	}

	t.Chdir(t.TempDir())
	if got := configRel(real); got != "" {
		t.Errorf("a folder that is no repository: %q", got)
	}
}

// The plan of a real range: the jobs of its changed paths, and whole for a
// start that is empty or all zeros, as typed, or a range git cannot diff.
func TestForGivesTheRangesJobs(t *testing.T) {
	from, to := gitRange(t)
	cfg := loadAt(t, "itos.yaml", jobsConfig)
	const missing = "1234567890123456789012345678901234567890"
	const zeros = "0000000000000000000000000000000000000000"
	for _, c := range []struct{ name, from, want string }{
		{"a range", from,
			"platform=runs:touched docs=skips:untouched lint=runs:no-paths empty=runs:no-paths"},
		{"an empty start", "",
			"platform=runs:whole docs=runs:whole lint=runs:no-paths empty=runs:no-paths"},
		{"an all-zeros start", zeros,
			"platform=runs:whole docs=runs:whole lint=runs:no-paths empty=runs:no-paths"},
		{"a range git cannot diff", missing,
			"platform=runs:whole docs=runs:whole lint=runs:no-paths empty=runs:no-paths"},
	} {
		p, err := For(cfg, c.from, to, Data{})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := answers(p.Jobs); got != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.name, got, c.want)
		}
	}
	trusted, err := ForTrusted(cfg, from, to, Data{}, &Trust{})
	if err != nil || !trusted.Prose || answers(trusted.Jobs) != "platform=runs:touched docs=skips:untouched lint=runs:no-paths empty=runs:no-paths" {
		t.Errorf("a trusted range: %v, %+v", err, trusted)
	}
	if got := answers(ForNightly(cfg).Jobs); got != "platform=runs:whole docs=runs:whole lint=runs:no-paths empty=runs:no-paths" {
		t.Errorf("the nightly: %s", got)
	}
	bad := loadAt(t, "bad.yaml", jobsConfig)
	job := bad.CI.Jobs.Values["platform"]
	job.Paths = []string{"?internal"}
	bad.CI.Jobs.Values["platform"] = job
	if p, err := For(bad, from, to, Data{}); err == nil || p != nil {
		t.Errorf("a job glob it cannot read gave %+v, %v", p, err)
	}
}

// The plan's JSON gives "jobs" after "not_started" and before "trusted": each
// declared job as {"runs","reason"} in the config's order, {} for none.
func TestFieldsGiveTheJobs(t *testing.T) {
	gitRange(t)
	emit := func(p *Plan) string {
		var b strings.Builder
		if err := out.Emit(&b, p.Fields(Range{})...); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	p := ForNightly(loadAt(t, "itos.yaml", jobsConfig))
	text := emit(p)
	var got struct {
		Jobs json.RawMessage
	}
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatal(err)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, got.Jobs); err != nil {
		t.Fatal(err)
	}
	want := `{"platform":{"runs":true,"reason":"whole"},"docs":{"runs":true,"reason":"whole"},` +
		`"lint":{"runs":true,"reason":"no-paths"},"empty":{"runs":true,"reason":"no-paths"}}`
	if compact.String() != want {
		t.Errorf("jobs:\n got %s\nwant %s", compact.String(), want)
	}
	p.Trusted = &Trust{}
	text = emit(p)
	if i, j, k := strings.Index(text, `"not_started"`), strings.Index(text, `"jobs"`), strings.Index(text, `"trusted"`); !(i < j && j < k) {
		t.Errorf("the keys are out of order:\n%s", text)
	}
	if empty := emit(ForNightly(load(t, scratch))); !strings.Contains(empty, `"jobs": {}`) {
		t.Errorf("a config with no jobs:\n%s", empty)
	}
}
