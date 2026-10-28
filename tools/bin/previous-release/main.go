// Command previous-release runs the last release's scenarios and conformance
// corpus against this tree's itos (T-071): an old scenario or case that fails
// is accepted only when a commit since that release says why.
//
// A scenario edited while doing something new no longer checks the behaviour
// it promised, and the suite that judges a change is the edited one, so a
// behaviour change could pass every check and ship as a minor under T-069. The
// features' steps are a black box that runs whatever ITOS_BIN names
// (features/README.md), and the corpus runner runs whatever binary it is
// given, so the last release's own suite can judge the new binary: this checks
// the release's tag out into a scratch worktree, with its own features, steps,
// go.mod and corpus, and runs there
//
//	ITOS_BIN=<bin> go test ./features -count=1 -json
//
// and its corpus, tools/itos/conformance/*.yaml, through this tree's corpus
// runner (tools/bin/conformance/corpus), linked into this program, so the
// runner that judges the old corpus is always this tree's, whatever tree the
// check runs in (its self-test's scratch repository has none of its own,
// T-153). The runner builds and inspects each case's repository with
// internal/git's Real, the first git on the PATH that is not an itos. It reads
// a release's fixtures as that release wrote them, YAML 1.2 that
// go.yaml.in/yaml/v3 alone refuses included (v8.0.0's ask.yaml), since a
// release's tree is never rewritten (T-144).
//
// The release's scenarios run as they are, each judged by what its steps
// assert. Its corpus is judged by itos's machine output alone, the only
// contract it keeps (docs/decisions/0035-only-machine-output-is-itos-s-contract-exit-codes-json-less-message-and-fix-and-the-files-it-writes.md):
// an old case's exit code, its files_after and its json, the json less every
// key named message or fix at any depth and text at its top, since those are
// the sentences a person reads and change with them (text, guide prose, since
// docs/decisions/0054-script-facing-plain-outputs-and-the-guard-s-hook-json-are-contract-guide-text-is-exempt-like-message-and-fix.md). Its stdout, stderr, stdout_has and
// stderr_has are not compared: every plain-text output, help, a refusal or a
// success alike, is for people and may change in any release, but the plain
// outputs scripts read (ADR-0054), whose cases say stdout_contract and whose
// stdout is compared whole. The rest are taken
// out of the release's cases before the runner runs them (corpus.File's
// Machine), with the message, fix and text keys, and the check says how many
// it left out. A help case or a usage error is judged as any other case
// is, by its exit code and the files it leaves: no rule of its own. This
// tree's own corpus, run without any of this, pins its json less those keys
// too, and every other output exactly.
//
// The runner is this tree's, in its --additive mode (T-076): what an old case's
// json holds the new output must still hold, every key with the value it
// expects, at every depth, an array element by element at the same length. A
// key added passes, since an output that only adds breaks no consumer
// (docs/decisions/0021-ci-cuts-a-release-from-a-green-push-judged-against-the-last-release.md);
// one removed or changed fails, so a rule id renamed or a problem dropped is a
// break, as is an exit code changed or a file's data written differently. A
// written YAML file is judged by its data as json is, and a Markdown file by
// its text less its HTML comments: a file's comment lines are for people
// (docs/decisions/0040-a-written-file-is-contract-by-its-data-not-its-comment-lines.md). The runner
// asks the binary its version for the corpus's {{version}}, so a binary
// stamped with a version the tag never had is not failed for saying it.
//
// The binary judged is, by default, this tree's itos as its release would be
// built: ./cmd/itos stamped with the version go run ./tools/bin/release-version
// computes for the commits since the tag (the next patch when they release
// nothing), as GoReleaser stamps a release. tools/bin/itos says a pre-release
// between releases (2.3.1-dev.5.g1234abc), which a config's requires cannot
// name, so the old corpus's cases that require the binary's own version would
// fail for the stamp alone; the release a consumer gets says X.Y.Z.
//
// An old scenario or case that fails is accepted only when a commit since the
// tag names it in a Changes: footer and that commit is a fix or is marked as
// breaking (a ! before its header's colon, or a BREAKING-CHANGE: or BREAKING
// CHANGE: footer in its last paragraph, read as tools/bin/schema-contract reads
// them) (T-106): a fix whose old scenario held the bug stays a patch, and a
// breaking change says which promises it breaks, said in its commit. Being
// breaking excuses nothing a commit does not name: one breaking commit once
// waived every failure of v6.0.0, 232 of them, most of which it did not cause,
// so a release could change what it never announced. A feat's Changes: never
// excuses one unless the feat is breaking: a feat that changes what an old
// scenario promised is a breaking change. Changes: is a footer of free text
// (itos.yaml's commits.footers, written by itos commit --changes), one entry a
// line:
//
//	Changes: @ID-CMSG-03 @ID-CMSG-04    scenarios, by their IDs
//	Changes: hooks.yaml: <case name>    a corpus case, by its file and its name,
//	                                    as the runner's FAIL line names it, or
//	                                    by a prefix of its name
//
// A case entry names the case of that file whose name is the entry's, or else
// the one case whose name starts with it (T-081): the header lint caps a
// footer line at 100 characters, and a case's name alone may be longer
// (v2.8's init.yaml has one of 103), so a long name is given by as much of it
// as tells it from the file's other cases. A prefix more than one case starts
// with names none of them.
//
// An entry that names no scenario or case of the release, or a prefix that
// several cases share, is printed as a warning, never a failure: a pushed
// commit cannot be rewritten, and a failure would hold CI red until a release
// that a red CI never cuts.
//
// The last release is the newest vX.Y.Z tag reachable from HEAD. With no such
// tag there is nothing to hold the tree to, which the check says and passes; a
// shallow clone, which may hide the tag, a tag that cannot be checked out, or
// a suite that cannot run (a build that fails, a fixture it cannot read, a
// failure it cannot name) stops it with exit 2, never a pass.
//
// Beside the standard library it imports internal/release, internal/git and
// the corpus runner, which reads YAML with go.yaml.in/yaml/v3, as itos does.
//
//	go run ./tools/bin/previous-release [-range-from <rev>] [-bin <itos>]
//
// -range-from <rev> checks nothing unless a commit in <rev>..HEAD is a feat or
// a fix, which is how CI runs it over a push's range; an empty <rev> checks.
// -bin is the itos to judge instead of this tree's, built as above. Exit status: 0 passed,
// 1 an old scenario or case fails and no commit says why, 2 the check could
// not run.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"

	itosgit "github.com/donvargax/itos/v8/internal/git"
	"github.com/donvargax/itos/v8/internal/release"
	"github.com/donvargax/itos/v8/tools/bin/conformance/corpus"
)

// commit is one commit since the release's tag.
type commit struct {
	SHA, Header, Type string
	Breaking          bool
	Changes           []string // its Changes: entries, a line each
}

// failure is one old scenario or case that fails against the binary: Key is
// how Changes: names it (@ID-… or <file>.yaml: <name>), What how it is shown.
type failure struct {
	Key, What string
}

const self = "previous-release"

func main() {
	os.Exit(run())
}

func run() int {
	rangeFrom := flag.String("range-from", "", "check nothing unless a commit in this revision..HEAD is a feat or a fix")
	bin := flag.String("bin", "", "the itos to run the last release's scenarios and corpus against; by default this tree's, stamped with the version its release would carry")
	flag.Parse()
	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "%s: unexpected argument %q\n", self, flag.Arg(0))
		return 2
	}
	fail := func(format string, a ...any) int {
		fmt.Fprintf(os.Stderr, self+": "+format+"\n", a...)
		return 2
	}
	started := time.Now()

	if *rangeFrom != "" {
		headers, err := git("log", "--format=%s", *rangeFrom+"..HEAD")
		if err != nil {
			return fail("cannot read the commits since %s: %v", *rangeFrom, err)
		}
		if !anyReleasable(headers) {
			fmt.Printf("%s: no feat or fix since %s: nothing to check\n", self, *rangeFrom)
			return 0
		}
	}

	if shallow, err := git("rev-parse", "--is-shallow-repository"); err != nil {
		return fail("%v", err)
	} else if strings.TrimSpace(shallow) == "true" {
		return fail("this is a shallow clone, which may not have the last release's tag: fetch the whole history (actions/checkout's fetch-depth: 0)")
	}
	tag, err := lastRelease()
	if err != nil {
		return fail("%v", err)
	}
	if tag == "" {
		fmt.Printf("%s: no vX.Y.Z tag is reachable from HEAD, so there is no release whose scenarios to hold this tree to: nothing to check\n", self)
		return 0
	}
	commits, err := commitsSince(tag)
	if err != nil {
		return fail("cannot read the commits since %s: %v", tag, err)
	}

	scratch, err := os.MkdirTemp("", self+"-")
	if err != nil {
		return fail("%v", err)
	}
	tree := filepath.Join(scratch, tag)
	defer func() {
		_, _ = git("worktree", "remove", "--force", tree)
		_ = os.RemoveAll(scratch)
		_, _ = git("worktree", "prune")
	}()
	judged := *bin
	if judged == "" {
		if judged, err = buildAsReleased(scratch, tag); err != nil {
			return fail("this tree's itos: %v", err)
		}
	}
	binary, err := filepath.Abs(judged)
	if err != nil {
		return fail("%v", err)
	}
	version, err := binaryVersion(binary)
	if err != nil {
		return fail("%v", err)
	}
	if _, err := git("worktree", "add", "--detach", tree, tag); err != nil {
		return fail("cannot check out %s into a scratch worktree: %v", tag, err)
	}
	what := judged
	if *bin == "" {
		what = "this tree's itos, built as its release would be"
	}
	fmt.Printf("%s: %s's scenarios and conformance corpus against %s (itos %s), in a scratch worktree of %s\n",
		self, tag, what, version, tag)
	corpusReadied := readyOldCorpus(tree)
	cases := corpusReadied.Names
	fmt.Printf("%s: %s's conformance corpus: its %s judged by itos's machine output alone, by this tree's runner (--additive):\n"+
		"  each one's exit code, files_after and json, a json key added passing and one removed or changed failing,\n"+
		"  and a stdout_contract case's stdout, whole (docs/decisions/0054-script-facing-plain-outputs-and-the-guard-s-hook-json-are-contract-guide-text-is-exempt-like-message-and-fix.md);\n"+
		"  left out as for people, the other plain stdout and stderr of %s and %s named message or fix, or text at the top, in their json\n"+
		"  (docs/decisions/0035-only-machine-output-is-itos-s-contract-exit-codes-json-less-message-and-fix-and-the-files-it-writes.md);\n"+
		"  this tree's own corpus pins its json less those keys, and every other output exactly\n",
		self, tag, counted(corpusReadied.Cases, "case", "cases"),
		counted(corpusReadied.Plain, "case", "cases"), counted(corpusReadied.Keys, "key", "keys"))

	var features, corpus suite
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); features = runFeatures(tree, binary) }()
	go func() { defer wg.Done(); corpus = runCorpus(corpusReadied, binary) }()
	wg.Wait()
	for _, s := range []suite{features, corpus} {
		if s.err != nil {
			fmt.Fprint(os.Stderr, s.output)
			return fail("%s's %s could not run: %v", tag, s.name, s.err)
		}
		fmt.Printf("%s: %s's %s: %s (%.1f s)\n", self, tag, s.name, s.summary, s.seconds)
	}

	failures := append(features.failures, corpus.failures...)
	ids, err := scenarioIDs(filepath.Join(tree, "features"))
	if err != nil {
		return fail("%s's features: %v", tag, err)
	}
	warnChanges(tag, commits, ids, cases)
	status := report(tag, failures, commits, cases, features, corpus)
	fmt.Printf("%s: done in %.1f s\n", self, time.Since(started).Seconds())
	return status
}

// suite is one of the release's two suites, run: what failed, what it
// printed about those failures, a line saying how it went, and an error when
// it could not run or its failures could not be named.
type suite struct {
	name, summary, output string
	failures              []failure
	seconds               float64
	err                   error
}

// readied is the release's corpus readied for the run: each fixture with only
// what itos's contract holds it to (corpus.File's Machine), how many cases it
// read, how many lost their plain output, how many message, fix and top-level text keys it
// left out of their json, and each corpus file's case names, by its base name,
// which Changes: entries are read against. A file that cannot be read is an
// error the corpus's run reports, its names none.
type readied struct {
	Cases, Plain, Keys int
	Names              map[string][]string
	Fixtures           []*corpus.Fixture
	Err                error
}

// readyOldCorpus readies the release's corpus in tree.
func readyOldCorpus(tree string) readied {
	r := readied{Names: map[string][]string{}}
	paths, err := corpus.Files(filepath.Join(tree, "tools", "itos", "conformance"))
	if err == nil && len(paths) == 0 {
		err = fmt.Errorf("it has no tools/itos/conformance/*.yaml")
	}
	if err != nil {
		r.Err = err
		return r
	}
	for _, path := range paths {
		base := filepath.Base(path)
		r.Names[base] = []string{}
		file, err := corpus.ReadFile(path)
		if err != nil {
			r.Err = errors.Join(r.Err, err)
			continue
		}
		r.Names[base] = file.Names()
		cases, plain, keys := file.Machine()
		r.Cases, r.Plain, r.Keys = r.Cases+cases, r.Plain+plain, r.Keys+keys
		fixture, err := file.Fixture()
		if err != nil {
			r.Err = errors.Join(r.Err, err)
			continue
		}
		fixture.Shown = "tools/itos/conformance/" + base
		r.Fixtures = append(r.Fixtures, fixture)
	}
	return r
}

// counted is n things, one or many: "1 case", "3 cases".
func counted(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// buildAsReleased builds this tree's ./cmd/itos into dir, stamped as
// GoReleaser stamps a release: with the version tools/bin/release-version
// computes, or the patch after tag when the commits release nothing.
func buildAsReleased(dir, tag string) (string, error) {
	cmd := exec.Command("go", "run", "./tools/bin/release-version")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("go run ./tools/bin/release-version: %v\n%s", err, stderr.String())
	}
	next := ""
	for _, line := range strings.Split(string(out), "\n") {
		if v, ok := strings.CutPrefix(line, "next="); ok {
			next = v
		}
	}
	if next == "" {
		var parts [3]int
		_, _ = fmt.Sscanf(tag, "v%d.%d.%d", &parts[0], &parts[1], &parts[2])
		next = fmt.Sprintf("%d.%d.%d", parts[0], parts[1], parts[2]+1)
	}
	bin := filepath.Join(dir, "itos")
	build := exec.Command("go", "build", "-trimpath",
		"-ldflags", "-s -w -X github.com/donvargax/itos/v8/internal/version.stamp="+next,
		"-o", bin, "./cmd/itos")
	build.Env = env("CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go build ./cmd/itos: %v\n%s", err, out)
	}
	return bin, nil
}

// binaryVersion is what the binary says it is: the last word of `<bin>
// version`'s first line.
func binaryVersion(bin string) (string, error) {
	out, err := exec.Command(bin, "version").Output()
	if err != nil {
		return "", fmt.Errorf("%s version: %v", bin, err)
	}
	line, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	words := strings.Fields(line)
	if len(words) == 0 {
		return "", fmt.Errorf("%s version said nothing", bin)
	}
	return words[len(words)-1], nil
}

// env is this process's environment without what a hook exports, which would
// point git at another repository.
func env(extra ...string) []string {
	var kept []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_") {
			kept = append(kept, kv)
		}
	}
	return append(kept, extra...)
}

// testEvent is one line of go test -json.
type testEvent struct {
	Action, Test, Output string
}

// runFeatures runs the release's features against bin, and names each
// scenario that fails by its ID.
func runFeatures(tree, bin string) suite {
	s := suite{name: "scenarios"}
	started := time.Now()
	cmd := exec.Command("go", "test", "./features", "-count=1", "-json")
	cmd.Dir = tree
	cmd.Env = env("ITOS_BIN=" + bin)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, runErr := cmd.Output()
	s.seconds = time.Since(started).Seconds()

	byName, err := scenarioNames(filepath.Join(tree, "features"))
	if err != nil {
		s.err = err
		return s
	}
	outputs := map[string]*strings.Builder{}
	var failed []string
	passed := 0
	var all strings.Builder
	lines := bufio.NewScanner(bytes.NewReader(out))
	lines.Buffer(make([]byte, 1<<20), 16<<20)
	for lines.Scan() {
		var e testEvent
		if json.Unmarshal(lines.Bytes(), &e) != nil {
			all.WriteString(lines.Text() + "\n")
			continue
		}
		all.WriteString(e.Output)
		name, ok := strings.CutPrefix(e.Test, "TestFeatures/")
		if !ok {
			continue
		}
		switch e.Action {
		case "output":
			if outputs[name] == nil {
				outputs[name] = &strings.Builder{}
			}
			outputs[name].WriteString(e.Output)
		case "pass":
			passed++
		case "fail":
			failed = append(failed, name)
		}
	}
	if runErr == nil {
		s.summary = fmt.Sprintf("all %d pass", passed)
		return s
	}
	if len(failed) == 0 {
		s.output = all.String() + stderr.String()
		s.err = fmt.Errorf("go test ./features failed, and no scenario did (%v)", runErr)
		return s
	}
	var shown strings.Builder
	for _, name := range failed {
		id, ok := idOf(byName, name)
		if !ok {
			s.output = all.String()
			s.err = fmt.Errorf("the scenario %q failed, and no feature file names it with an ID", name)
			return s
		}
		s.failures = append(s.failures, failure{Key: id, What: fmt.Sprintf("scenario %s (%s)", id, strings.ReplaceAll(name, "_", " "))})
		if o := outputs[name]; o != nil {
			shown.WriteString(o.String())
		}
	}
	s.output = shown.String()
	s.summary = fmt.Sprintf("%d of %d fail", len(failed), len(failed)+passed)
	return s
}

// runCorpus runs the release's readied corpus against bin with this tree's
// runner, judging it additively, and names each case that fails by its file
// and its name.
func runCorpus(old readied, bin string) suite {
	s := suite{name: "conformance corpus"}
	if old.Err != nil {
		s.err = old.Err
		return s
	}
	nativeGit, err := itosgit.Real(runtime.GOOS)
	if err != nil {
		s.err = fmt.Errorf("no native git for the conformance runner: %v", err)
		return s
	}
	started := time.Now()
	runner, err := corpus.New(corpus.Options{Bin: bin, Git: nativeGit, Additive: true})
	if err != nil {
		s.err = fmt.Errorf("the conformance runner: %v", err)
		return s
	}
	defer runner.Close()
	if err := runner.LearnVersion(); err != nil {
		s.err = fmt.Errorf("the conformance runner: %v", err)
		return s
	}
	results := runner.Run(old.Fixtures)
	s.seconds = time.Since(started).Seconds()
	s.summary = corpus.Summary(old.Fixtures, results, time.Since(started))
	var output strings.Builder
	corpus.Failures(&output, results)
	s.output = output.String()
	for _, r := range results {
		if len(r.Problems) > 0 {
			key := caseKey(r.Fixture.Base(), r.Case.Name)
			s.failures = append(s.failures, failure{Key: key, What: "conformance case " + key})
		}
	}
	return s
}

// caseKey is how Changes: names a corpus case: its file's base name and its
// name.
func caseKey(file, name string) string {
	return filepath.Base(file) + ": " + strings.TrimSpace(name)
}

var (
	idTag    = regexp.MustCompile(`^@ID-[A-Z]+-\d+$`)
	scenario = regexp.MustCompile(`^\s*Scenario(?: Outline)?:\s*(.+?)\s*$`)
)

// outlineRow is the suffix go test gives the subtest of a scenario outline's
// second and later rows, which share the outline's name.
var outlineRow = regexp.MustCompile(`#\d+$`)

// idOf is the @ID- tag of the scenario a failed subtest ran, an outline's row
// by the outline's.
func idOf(byName map[string]string, name string) (string, bool) {
	id, ok := byName[outlineRow.ReplaceAllString(name, "")]
	return id, ok
}

// scenarioNames maps each scenario of the feature files under dir, by the
// name go test gives its subtest (spaces as underscores), to its @ID- tag.
func scenarioNames(dir string) (map[string]string, error) {
	names := map[string]string{}
	err := eachScenario(dir, func(name, id string) {
		if id != "" {
			names[strings.Map(func(r rune) rune {
				if unicode.IsSpace(r) {
					return '_'
				}
				return r
			}, name)] = id
		}
	})
	return names, err
}

// scenarioIDs are the @ID- tags of the scenarios under dir.
func scenarioIDs(dir string) (map[string]bool, error) {
	ids := map[string]bool{}
	err := eachScenario(dir, func(_, id string) {
		if id != "" {
			ids[id] = true
		}
	})
	return ids, err
}

// eachScenario calls fn with each scenario's name and the @ID- tag on the tag
// lines above it ("" for none), in every *.feature file under dir.
func eachScenario(dir string, fn func(name, id string)) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.feature"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("%s has no feature files", dir)
	}
	for _, file := range files {
		text, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		id := ""
		for _, line := range strings.Split(string(text), "\n") {
			trimmed := strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(trimmed, "@"):
				for _, word := range strings.Fields(trimmed) {
					if strings.HasPrefix(word, "#") {
						break
					}
					if idTag.MatchString(word) {
						id = word
					}
				}
			case strings.HasPrefix(trimmed, "#") || trimmed == "":
			default:
				if m := scenario.FindStringSubmatch(line); m != nil {
					fn(m[1], id)
				}
				id = ""
			}
		}
	}
	return nil
}

// excuse is why a failure is accepted, or "" when nothing excuses it: a commit
// that names it in Changes: and is a fix or marked as breaking (T-106). A
// breaking commit that does not name it excuses nothing. named is a commit
// that names it and is neither, a feat say, which never excuses one. cases are
// the release's corpus's case names, by file.
func excuse(f failure, commits []commit, cases map[string][]string) (why string, named *commit) {
	for i := range commits {
		c := &commits[i]
		if !names(c.Changes, f.Key, cases) {
			continue
		}
		switch {
		case c.Breaking:
			return fmt.Sprintf("the breaking change %.7s %q names it in Changes:", c.SHA, c.Header), nil
		case c.Type == "fix":
			return fmt.Sprintf("the fix %.7s %q names it in Changes:", c.SHA, c.Header), nil
		}
		if named == nil {
			named = c
		}
	}
	return "", named
}

// names is whether a commit's Changes: entries name key: a scenario by its ID
// among the entry's words, a case by its file and its name or a prefix of it
// (caseNamed).
func names(entries []string, key string, cases map[string][]string) bool {
	for _, e := range entries {
		if ids, ok := entryIDs(e); ok {
			for _, id := range ids {
				if id == key {
					return true
				}
			}
		} else if file, name, ok := entryCase(e); ok {
			if named, _ := caseNamed(file, name, cases); named != "" && caseKey(file, named) == key {
				return true
			}
		}
	}
	return false
}

var idWord = regexp.MustCompile(`^@?(ID-[A-Z]+-\d+)$`)

// entryIDs are the scenario IDs a Changes: entry gives, @ included, when the
// entry is nothing but IDs separated by commas or spaces.
func entryIDs(entry string) ([]string, bool) {
	words := strings.FieldsFunc(entry, func(r rune) bool { return r == ',' || unicode.IsSpace(r) })
	if len(words) == 0 {
		return nil, false
	}
	var ids []string
	for _, w := range words {
		m := idWord.FindStringSubmatch(w)
		if m == nil {
			return nil, false
		}
		ids = append(ids, "@"+m[1])
	}
	return ids, true
}

var caseEntry = regexp.MustCompile(`^(\S+\.yaml):\s+(.+)$`)

// entryCase is the corpus file and case name a Changes: entry gives.
func entryCase(entry string) (file, name string, ok bool) {
	m := caseEntry.FindStringSubmatch(entry)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

// caseNamed is the name of the release's corpus case a Changes: entry
// <file>.yaml: <name> names, cases holding each corpus file's case names: the
// case whose name is name, or else the one case whose name starts with it. It
// is "" when the entry names none, and why says how, in words that follow "the
// release's <file>": no such file, no case its name starts, or several.
func caseNamed(file, name string, cases map[string][]string) (named, why string) {
	all, ok := cases[filepath.Base(file)]
	if !ok {
		return "", "no such file"
	}
	var started []string
	for _, c := range all {
		c = strings.TrimSpace(c)
		if c == name {
			return c, ""
		}
		if strings.HasPrefix(c, name) {
			started = append(started, c)
		}
	}
	switch len(started) {
	case 1:
		return started[0], ""
	case 0:
		return "", fmt.Sprintf("has no case %q", name)
	}
	quoted := make([]string, len(started))
	for i, c := range started {
		quoted[i] = fmt.Sprintf("%q", c)
	}
	return "", fmt.Sprintf("has %d cases whose names start %q (%s): it is ambiguous, so it names none of them",
		len(started), name, strings.Join(quoted, ", "))
}

// warnChanges prints each Changes: entry since the tag that names no scenario
// or case of the release, names a case by a prefix several of them share, or
// is neither form.
func warnChanges(tag string, commits []commit, ids map[string]bool, cases map[string][]string) {
	for _, c := range commits {
		for _, e := range c.Changes {
			var why string
			if list, ok := entryIDs(e); ok {
				var unknown []string
				for _, id := range list {
					if !ids[id] {
						unknown = append(unknown, id)
					}
				}
				if len(unknown) > 0 {
					why = fmt.Sprintf("%s is no scenario of %s", strings.Join(unknown, ", "), tag)
				}
			} else if file, name, ok := entryCase(e); ok {
				if _, known := cases[filepath.Base(file)]; !known {
					why = fmt.Sprintf("%s has no corpus file %s", tag, filepath.Base(file))
				} else if named, how := caseNamed(file, name, cases); named == "" {
					why = fmt.Sprintf("%s's %s %s", tag, filepath.Base(file), how)
				}
			} else {
				why = "it is neither scenario IDs (@ID-…) nor a corpus case (<file>.yaml: <case name>)"
			}
			if why != "" {
				fmt.Printf("%s: warning: %.7s %q says \"Changes: %s\", and %s\n", self, c.SHA, c.Header, e, why)
			}
		}
	}
}

// report prints each failure, accepted or not, and gives the exit status.
func report(tag string, failures []failure, commits []commit, cases map[string][]string, features, corpus suite) int {
	if len(failures) == 0 {
		fmt.Printf("%s: this tree's itos keeps every promise of %s's scenarios and corpus\n", self, tag)
		return 0
	}
	var refused []string
	for _, f := range failures {
		why, named := excuse(f, commits, cases)
		if why != "" {
			fmt.Printf("%s: %s's %s fails, accepted: %s\n", self, tag, f.What, why)
			continue
		}
		line := fmt.Sprintf("%s: %s's %s fails against this tree's itos", self, tag, f.What)
		if named != nil {
			line += fmt.Sprintf(", and only the %s %.7s %q names it in Changes:, which excuses one only on a fix or a breaking change",
				named.Type, named.SHA, named.Header)
		}
		refused = append(refused, line)
	}
	if len(refused) == 0 {
		return 0
	}
	for _, s := range []suite{features, corpus} {
		if len(s.failures) > 0 {
			fmt.Fprintf(os.Stderr, "\n%s: what %s's %s said:\n%s", self, tag, s.name, s.output)
		}
	}
	fmt.Fprintln(os.Stderr)
	for _, line := range refused {
		fmt.Fprintln(os.Stderr, line)
	}
	fmt.Fprintf(os.Stderr, "\n%s: %d of %s's scenarios and cases fail against this tree's itos, and no commit since %s names them:\n"+
		"  the behaviour each promised has changed, and the release would ship it unannounced.\n"+
		"  Name each failure in a Changes: footer (itos commit --changes '@ID-…', or --changes\n"+
		"  '<file>.yaml: <case name>' for a case, its name or a prefix of it no other case of the file\n"+
		"  starts with), on a breaking commit too: being breaking excuses only what its Changes: names.\n"+
		"  If a fix changed it and the old scenario or case held the bug, name it in the fix's Changes:,\n"+
		"  so the release stays a patch. Otherwise the change is breaking: a commit with a BREAKING-CHANGE:\n"+
		"  footer saying what a consumer must change and a Changes: footer naming each failure\n"+
		"  (itos commit --breaking '<what to change>' --changes '…'), so the release is a major; a feat's\n"+
		"  Changes: excuses nothing unless the feat is breaking. Or keep the old behaviour.\n",
		self, len(refused), tag, tag)
	return 1
}

func git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %v\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return string(out), nil
}

var (
	releasable = regexp.MustCompile(`^(feat|fix)(\([^)]*\))?!?: `)
	headerType = regexp.MustCompile(`^([a-zA-Z]+)(\([^)]*\))?(!?): `)
)

func anyReleasable(headers string) bool {
	for _, h := range strings.Split(headers, "\n") {
		if releasable.MatchString(h) {
			return true
		}
	}
	return false
}

// lastRelease is the newest release tag reachable from HEAD, or "" for none,
// picked by release.Newest, the release cut's rule (T-090).
func lastRelease() (string, error) {
	out, err := git("tag", "--merged", "HEAD", "--list", "v*")
	if err != nil {
		return "", err
	}
	return release.Newest(strings.Split(out, "\n")), nil
}

// commitsSince lists the commits in <tag>..HEAD, newest first, each with its
// type, whether it is marked as breaking, and the Changes: entries of its
// footers.
func commitsSince(tag string) ([]commit, error) {
	out, err := git("log", "--format=%H%x1f%B%x1e", tag+"..HEAD")
	if err != nil {
		return nil, err
	}
	var commits []commit
	for _, record := range strings.Split(out, "\x1e") {
		sha, message, ok := strings.Cut(strings.TrimLeft(record, "\n"), "\x1f")
		if !ok {
			continue
		}
		commits = append(commits, parseCommit(sha, message))
	}
	return commits, nil
}

// parseCommit reads one commit's message: its header, its type, whether it is
// marked as breaking, and the Changes: entries of its footers.
func parseCommit(sha, message string) commit {
	message = strings.TrimSpace(message)
	header, _, _ := strings.Cut(message, "\n")
	c := commit{SHA: sha, Header: header, Breaking: markedBreaking(message)}
	if m := headerType.FindStringSubmatch(header); m != nil {
		c.Type = m[1]
	}
	for _, line := range strings.Split(lastParagraph(message), "\n") {
		if text, ok := strings.CutPrefix(line, "Changes:"); ok && strings.TrimSpace(text) != "" {
			c.Changes = append(c.Changes, strings.TrimSpace(text))
		}
	}
	return c
}

// markedBreaking: a ! before the header's colon, or a BREAKING-CHANGE: or
// BREAKING CHANGE: footer in the message's last paragraph.
func markedBreaking(message string) bool {
	header, _, _ := strings.Cut(message, "\n")
	if m := headerType.FindStringSubmatch(header); m != nil && m[3] == "!" {
		return true
	}
	for _, line := range strings.Split(lastParagraph(message), "\n") {
		if strings.HasPrefix(line, "BREAKING-CHANGE:") || strings.HasPrefix(line, "BREAKING CHANGE:") {
			return true
		}
	}
	return false
}

// lastParagraph is a message's footers: its last paragraph after the header,
// where git's trailers are, so a body line that starts with a footer's key is
// not read as one.
func lastParagraph(message string) string {
	_, rest, _ := strings.Cut(message, "\n")
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return ""
	}
	paragraphs := strings.Split(rest, "\n\n")
	return paragraphs[len(paragraphs)-1]
}
