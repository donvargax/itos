package ci

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/check"
	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/evidence"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/plan"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// tree is a repository whose top is a temporary folder holding the files,
// and whose folder is its sub folder "app": a config over the fake
// repository, and the folder.
func tree(t *testing.T, files ...string) (*config.Loaded, string) {
	t.Helper()
	top := t.TempDir()
	folder := filepath.Join(top, "app")
	for _, name := range files {
		file := filepath.Join(top, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("package x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := load(t)
	cfg.VCS = &vcsfake.VCS{Root: folder, Located: map[string]vcsport.Location{"": {Top: top}}}
	return cfg, top
}

// annotated runs the plan with Annotate set, over cfg.
func annotated(t *testing.T, cfg *config.Loaded, p *plan.Plan, o Options) (int, string, []string) {
	t.Helper()
	var stdout, stderr strings.Builder
	o.Stdout, o.Stderr, o.Annotate = &stdout, &stderr, true
	code, err := Run(cfg, p, o)
	if err != nil {
		t.Fatal(err)
	}
	var commands []string
	for _, line := range strings.Split(stderr.String(), "\n") {
		if strings.HasPrefix(line, "::error") {
			commands = append(commands, line)
		}
	}
	return code, stdout.String(), commands
}

func steps(commands ...string) *plan.Plan {
	p := &plan.Plan{}
	for _, c := range commands {
		p.Order = append(p.Order, plan.Item{Step: &plan.Step{Command: c}})
	}
	return p
}

func printing(lines ...string) string {
	var b strings.Builder
	for _, l := range lines {
		fmt.Fprintf(&b, "echo '%s'; ", l)
	}
	return b.String()
}

func wantCommands(t *testing.T, got []string, want ...string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("annotations\n got %q\nwant %q", got, want)
	}
}

func TestEscapes(t *testing.T) {
	if got := escapeMessage("100%\r\na: b, c\n"); got != "100%25%0D%0Aa: b, c%0A" {
		t.Errorf("message %q", got)
	}
	if got := escapeProperty("a%b\r\nc:d,e"); got != "a%25b%0D%0Ac%3Ad%2Ce" {
		t.Errorf("property %q", got)
	}
}

// A step's annotation is at the first line of its stdout that names a file of
// its folder and a line, else of its stderr; the file is written relative to
// the repository's top, whatever folder the commands run in.
func TestAStepIsAnnotatedAtTheFileAndLineItNames(t *testing.T) {
	cfg, _ := tree(t, "app/f.go", "o.go")
	command := printing(
		"app/f.go:9: not in the folder",
		"    f.go:1: indented",
		"nowhere.go:2:1: no file",
		"f.go:3:9: boom (X)",
		"f.go:5: later",
	) + "echo 'f.go:7: stderr' >&2; exit 1"
	_, _, got := annotated(t, cfg, steps(command), Options{})
	wantCommands(t, got, "::error file=app/f.go,line=3::"+command+" failed: boom (X)")

	// The stderr line is read when stdout names none; a path may climb out of
	// the folder and stay in the repository.
	command = printing("nothing here") + "echo '../o.go:4:2: climbs' >&2; exit 2"
	_, _, got = annotated(t, cfg, steps(command), Options{})
	wantCommands(t, got, "::error file=o.go,line=4::"+command+" failed: climbs")
}

// A step that names no file, or one outside the repository, is a plain error
// naming its exit code.
func TestAStepThatNamesNoFileIsAPlainError(t *testing.T) {
	cfg, _ := tree(t, "app/f.go", "o.go")
	_, _, got := annotated(t, cfg, steps("exit 3"), Options{})
	wantCommands(t, got, "::error::exit 3 failed: it exited 3")

	outside := printing("../o.go:2: outside") + "exit 4"
	repo := cfg.VCS.(*vcsfake.VCS)
	repo.Located[""] = vcsport.Location{Top: repo.Root}
	_, _, got = annotated(t, cfg, steps(outside), Options{})
	wantCommands(t, got, "::error::"+outside+" failed: it exited 4")

	delete(repo.Located, "")
	_, _, got = annotated(t, cfg, steps("echo f.go:1: x; exit 5"), Options{})
	wantCommands(t, got, "::error::echo f.go:1: x; exit 5 failed: it exited 5")
}

func TestAStepWithNoExitCodeIsAPlainError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows has no signals")
	}
	cfg, _ := tree(t)
	_, _, got := annotated(t, cfg, steps("kill -TERM $$"), Options{})
	wantCommands(t, got, "::error::kill -TERM $$ failed: it gave no exit code")
}

// Under --json the annotation is on stderr beside the log, stdout holds the
// one object, and a run that passes prints none.
func TestAnnotationsLeaveJSONAlone(t *testing.T) {
	cfg, _ := tree(t, "app/f.go")
	command := printing("f.go:2: boom") + "exit 1"
	code, stdout, got := annotated(t, cfg, steps(command), Options{JSON: true})
	wantCommands(t, got, "::error file=app/f.go,line=2::"+command+" failed: boom")
	var verdict map[string]any
	if err := json.Unmarshal([]byte(stdout), &verdict); err != nil || code != 1 || strings.Contains(stdout, "::error") {
		t.Errorf("exit %d, stdout %q, %v", code, stdout, err)
	}
	code, _, got = annotated(t, cfg, steps("exit 0"), Options{})
	wantCommands(t, got)
	if code != 0 {
		t.Errorf("exit %d", code)
	}
}

// Without Annotate a failing run prints no workflow command.
func TestAFailingRunPrintsNoCommandWithoutAnnotate(t *testing.T) {
	cfg, _ := tree(t, "app/f.go")
	var stdout, stderr strings.Builder
	if code, err := Run(cfg, steps(printing("f.go:2: boom")+"exit 1"), Options{Stdout: &stdout, Stderr: &stderr}); err != nil || code != 1 {
		t.Fatalf("exit %d, %v", code, err)
	}
	if strings.Contains(stdout.String()+stderr.String(), "::error") {
		t.Errorf("stdout %q, stderr %q", stdout.String(), stderr.String())
	}
}

func checkItem(id, title, command string) plan.Item {
	return plan.Item{Check: &plan.Check{
		CostedCheck: check.CostedCheck{Task: ledger.Task{ID: id, Title: title}, Check: ledger.Check{Run: &command}},
		Action:      plan.Run,
	}}
}

// A task check's annotation names its task and title, and is at the file and
// line its stdout, else its stderr, names; else it is plain naming its check.
func TestATaskCheckIsAnnotated(t *testing.T) {
	cfg, _ := tree(t, "app/f.go")
	stderrOnly := "printf '%s\\n' 'f.go:2: skips the tax' >&2; exit 1"
	plain := "exit 4"
	for _, asJSON := range []bool{false, true} {
		_, _, got := annotated(t, cfg, &plan.Plan{Order: []plan.Item{
			checkItem("T-1", "Tidy", stderrOnly), checkItem("T-2", "", plain),
		}}, Options{JSON: asJSON})
		wantCommands(t, got, "::error file=app/f.go,line=2::T-1 (Tidy) failed: skips the tax")
		cfg.CI.StopAtFirstFailure = false
		_, _, got = annotated(t, cfg, &plan.Plan{Order: []plan.Item{
			checkItem("T-1", "Tidy", stderrOnly), checkItem("T-2", "", plain),
		}}, Options{JSON: asJSON})
		wantCommands(t, got, "::error file=app/f.go,line=2::T-1 (Tidy) failed: skips the tax", "::error::T-2 failed: its check "+plain)
		cfg.CI.StopAtFirstFailure = true
	}
	// Output of an earlier check is not read for a later one.
	_, _, got := annotated(t, cfg, &plan.Plan{Order: []plan.Item{checkItem("T-1", "", "exit 0"), checkItem("T-2", "T", plain)}}, Options{})
	wantCommands(t, got, "::error::T-2 (T) failed: its check "+plain)
}

// The tasks named that no file has are one error naming every one, in the
// footers' order.
func TestUnknownTasksAreOneError(t *testing.T) {
	cfg, _ := tree(t)
	cfg, err := config.FromText(cfg.VCS, cfg.Trees, cfg.Source, "itos.yaml", "version: 1\nledger: { files: \"tasks/phase-{group}.yaml\" }\n", false)
	if err != nil {
		t.Fatal(err)
	}
	code, _, got := annotated(t, cfg, &plan.Plan{Unknown: []string{"T-9", "T-8"}}, Options{})
	wantCommands(t, got, "::error::the tasks named failed: no task T-9, T-8 in tasks/")
	if code != 1 {
		t.Errorf("exit %d", code)
	}
}

func refusal(problems string) string {
	return `printf '%s\n' '{"schema":1,"ok":false,"problems":` + problems + `}'; exit 1`
}

// Each of the code proof's problems is an error of its own, at the file and
// line its subject names when it names both, the file as given and escaped;
// a proof that cannot run puts its own problem first, plain.
func TestTheCodeProofsProblemsAreAnnotatedOneByOne(t *testing.T) {
	cfg, _ := tree(t)
	proof := func(command string) *plan.Plan {
		return &plan.Plan{Order: []plan.Item{{Step: &plan.Step{Command: command, Proof: true}}}}
	}
	_, _, got := annotated(t, cfg, proof(refusal(`[
		{"rule":"a.one","message":"100% of x\r\nfailed","fix":"f","file":"src/odd,name:1.go","line":4},
		{"rule":"a.two","message":"no line","fix":"f","file":"src/a.go"},
		{"rule":"a.three","message":"line 0","fix":"f","file":"src/a.go","line":0},
		{"rule":"a.four","message":"a string line","fix":"f","file":"src/a.go","line":"3"},
		{"rule":"a.five","message":"a fractional line","fix":"f","file":"src/a.go","line":2.5},
		{"rule":"a.six","message":"a number file","fix":"f","file":7,"line":2},
		{"rule":"a.seven","message":"no file","fix":"f","line":2},
		{"rule":"a.eight","message":"an empty file","fix":"f","file":"","line":2}]`)), Options{})
	wantCommands(t, got,
		"::error file=src/odd%2Cname%3A1.go,line=4::the code proof failed: a.one: 100%25 of x%0D%0Afailed",
		"::error::the code proof failed: a.two: no line",
		"::error::the code proof failed: a.three: line 0",
		"::error::the code proof failed: a.four: a string line",
		"::error::the code proof failed: a.five: a fractional line",
		"::error::the code proof failed: a.six: a number file",
		"::error::the code proof failed: a.seven: no file",
		"::error::the code proof failed: a.eight: an empty file")

	_, _, got = annotated(t, cfg, proof(`printf '%s\n' '{"schema":1,"ok":false,"problems":[{"rule":"a.one","message":"m","file":"x.go","line":3}]}'; exit 2`), Options{})
	wantCommands(t, got,
		"::error::the code proof failed: ci-code-proof: the code proof could not run, so the run fails rather than passes it: "+
			`printf '%25s\n' '{"schema":1,"ok":false,"problems":[{"rule":"a.one","message":"m","file":"x.go","line":3}]}'; exit 2 exited 2`,
		"::error file=x.go,line=3::the code proof failed: a.one: m")

	_, _, got = annotated(t, cfg, proof(refusal(`[]`)), Options{})
	wantCommands(t, got, "::error::the code proof failed: ci-code-proof: the code proof refused and named no problem")

	_, _, got = annotated(t, cfg, proof(`printf '%s\n' '{"itos_proof":{"version":1,"judged":0,"unit":"function"}}'`), Options{})
	wantCommands(t, got, "::error::the code proof failed: ci-code-proof-empty: the code proof reported that it judged no work (judged 0), so it proves nothing for this range")
	_, _, got = annotated(t, cfg, proof(`printf '%s\n' '{"itos_proof":{"version":2}}'`), Options{})
	if len(got) != 1 || !strings.HasPrefix(got[0], "::error::the code proof failed: ci-code-proof-summary: the code proof's summary breaks its contract: ") {
		t.Errorf("a proof with a broken summary: %q", got)
	}
	if _, _, got = annotated(t, cfg, proof("exit 0"), Options{}); len(got) != 0 {
		t.Errorf("a proof that passes: %q", got)
	}
}

// A run annotates its first 10 failures, in run order, and no more, the log
// still naming every one.
func TestARunAnnotatesItsFirstTenFailures(t *testing.T) {
	cfg, _ := tree(t)
	cfg.CI.StopAtFirstFailure = false
	var commands []string
	for i := 1; i <= 12; i++ {
		commands = append(commands, fmt.Sprintf("exit %d", i))
	}
	var stdout, stderr strings.Builder
	code, err := Run(cfg, steps(commands...), Options{Stdout: &stdout, Stderr: &stderr, Annotate: true})
	if err != nil || code != 1 {
		t.Fatalf("exit %d, %v", code, err)
	}
	var got []string
	for _, line := range strings.Split(stderr.String(), "\n") {
		if strings.HasPrefix(line, "::error") {
			got = append(got, line)
		}
	}
	if len(got) != 10 || got[0] != "::error::exit 1 failed: it exited 1" || got[9] != "::error::exit 10 failed: it exited 10" {
		t.Errorf("annotations %q", got)
	}
	if strings.Count(stderr.String(), "failed at: exit") != 12 {
		t.Errorf("the log names fewer than 12 failures: %q", stderr.String())
	}
}

// A step that opts in to evidence is read as one stream, as stdout, when it
// runs and when its record answers.
func TestAStepThatOptsInIsAnnotatedFromItsMergedOutput(t *testing.T) {
	cfg, _ := tree(t, "app/f.go")
	command := printing("f.go:6: bad") + "exit 5"
	opted := &plan.Plan{Order: []plan.Item{{Step: &plan.Step{Command: command, Evidence: &config.Evidence{}}}}}
	store := t.TempDir()
	for i := range 2 {
		engine := &evidence.Engine{Cfg: load(t), Git: fakeGit{}, Store: fakeStore{store}, Binding: evidence.ThisBinary(), Mode: "ci", CIEnv: map[string]string{}}
		_, stdout, got := annotated(t, cfg, opted, Options{Evidence: engine, JSON: true})
		wantCommands(t, got, "::error file=app/f.go,line=6::"+command+" failed: bad")
		var report struct{ Evidence []struct{ Hit bool } }
		if err := json.Unmarshal([]byte(stdout), &report); err != nil || len(report.Evidence) != 1 || report.Evidence[0].Hit != (i == 1) {
			t.Errorf("run %d: want a record %v: %q, %v", i+1, map[bool]string{false: "recorded", true: "answered"}[i == 1], stdout, err)
		}
	}
}

// within is a path relative to a folder, with slashes, when it is inside it.
func TestWithin(t *testing.T) {
	// Every file exists: a path is made relative once its links, and the short
	// names some systems give a temporary folder, are resolved.
	top, other := t.TempDir(), t.TempDir()
	touch := func(dir string, names ...string) string {
		file := filepath.Join(append([]string{dir}, names...)...)
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		return file
	}
	for _, c := range []struct {
		top, file string
		want      string
		ok        bool
	}{
		{top, touch(top, "a", "b.go"), "a/b.go", true},
		{top, top, ".", true},
		{top, touch(top, "..x"), "..x", true},
		{filepath.Join(top, "a"), touch(top, "x.go"), "", false},
		{filepath.Join(top, "a"), top, "", false},
		{top, touch(other, "x.go"), "", false},
		{"", touch(top, "c.go"), "", false},
		{top, "", "", false},
	} {
		if got, ok := within(c.top, c.file); got != c.want || ok != c.ok {
			t.Errorf("within(%q, %q) = %q, %v; want %q, %v", c.top, c.file, got, ok, c.want, c.ok)
		}
	}
}

// A path that does not exist is related as written.
func TestResolvedKeepsWhatDoesNotExist(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	if got := resolved(missing); got != missing {
		t.Errorf("resolved(%q) = %q", missing, got)
	}
}

// A finder keeps the first line that begins with a path of a file of its
// folder, a line and a colon, whatever way the text is cut into writes.
func TestFinder(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"f.go", "g.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "d"), 0o755); err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("x", 2*maxLine)
	for _, c := range []struct {
		name   string
		writes []string
		want   *hit
	}{
		{"the first match", []string{"f.go:3: a\ng.go:4: b\n"}, &hit{"f.go", 3, "a"}},
		{"a column", []string{"f.go:3:9:  boom \r\n"}, &hit{"f.go", 3, "boom"}},
		{"a line cut in two writes", []string{"f.go:3: bo", "om\n"}, &hit{"f.go", 3, "boom"}},
		{"a last line with no end", []string{"x\ng.go:2: end"}, &hit{"g.go", 2, "end"}},
		{"an empty reason", []string{"f.go:3:\n"}, &hit{"f.go", 3, ""}},
		{"indented", []string{" f.go:3: a\n\tf.go:3: a\n"}, nil},
		{"no such file", []string{"h.go:3: a\n"}, nil},
		{"a folder", []string{"d:3: a\n"}, nil},
		{"no colon after the line", []string{"f.go:3 a\n"}, nil},
		{"a line that is no number", []string{"f.go:x: a\n"}, nil},
		{"line 0", []string{"f.go:0: a\n"}, nil},
		{"a line past int", []string{"f.go:99999999999999999999: a\n"}, nil},
		{"a path with white space", []string{"f .go:3: a\n"}, nil},
		{"an absolute path", []string{filepath.Join(dir, "f.go") + ":3: a\n"}, nil},
		{"a rooted path", []string{"/f.go:3: a\n"}, nil},
		{"a line read only up to its cap", []string{"f.go:3: " + long + "\n"}, &hit{"f.go", 3, long[:maxLine-len("f.go:3: ")]}},
		{"the cap does not reach the next line", []string{long + "\nf.go:5: b\n"}, &hit{"f.go", 5, "b"}},
	} {
		f := finder{folder: dir}
		for _, w := range c.writes {
			if n, err := f.Write([]byte(w)); n != len(w) || err != nil {
				t.Fatalf("%s: wrote %d of %d, %v", c.name, n, len(w), err)
			}
		}
		f.flush()
		if (f.hit == nil) != (c.want == nil) || f.hit != nil && *f.hit != *c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, f.hit, c.want)
		}
		f.reset()
		if f.hit != nil || len(f.partly) != 0 {
			t.Errorf("%s: reset left %+v", c.name, f)
		}
	}
}

// A backslash separates only on Windows: elsewhere it is part of a name.
func TestFinderReadsABackslashAsASeparatorOnWindowsOnly(t *testing.T) {
	dir := t.TempDir()
	name, want := `sub\f.go`, `sub\f.go`
	if runtime.GOOS == "windows" {
		name, want = "sub/f.go", "sub/f.go"
	}
	file := filepath.Join(dir, filepath.FromSlash(name))
	if runtime.GOOS != "windows" {
		file = filepath.Join(dir, name)
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	f := finder{folder: dir}
	_, _ = f.Write([]byte(`sub\f.go:2: x` + "\n"))
	if f.hit == nil || f.hit.path != want || f.hit.line != 2 {
		t.Errorf("got %+v, want %q", f.hit, want)
	}
}
