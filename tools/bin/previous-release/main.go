// Command previous-release runs the last release's scenarios and conformance
// corpus against this tree's itos (T-071): an old scenario or case that fails
// is accepted only when a commit since that release says why.
//
// A scenario edited while doing something new no longer checks the behaviour
// it promised, and the suite that judges a change is the edited one, so a
// behaviour change could pass every check and ship as a minor under T-069. The
// features' steps are a black box that runs whatever ITOS_BIN names
// (features/README.md), and the corpus runner runs whatever --bin names, so
// the last release's own suite can judge the new binary: this checks the
// release's tag out into a scratch worktree, with its own features, steps,
// go.mod and corpus, and runs there
//
//	ITOS_BIN=<bin> go test ./features -count=1 -json
//	node <this tree>/tools/itos/conformance/run.ts --bin <bin> --additive --only <its corpus>
//
// The release's scenarios run as they are, each judged by what its steps
// assert. Its corpus is judged by itos's machine output alone, the only
// contract it keeps (docs/decisions/0035-only-machine-output-is-itos-s-contract-exit-codes-json-less-message-and-fix-and-the-files-it-writes.md):
// an old case's exit code, its files_after and its json, the json less every
// key named message or fix at any depth, since those are the sentences a
// person reads and change with them. Its stdout, stderr, stdout_has and
// stderr_has are not compared: every plain-text output, help, a refusal or a
// success alike, is for people and may change in any release. They are taken
// out of the worktree's fixtures before the runner reads them
// (oldCorpusScript), with the message and fix keys, and the check says how
// many it left out. A help case or a usage error is judged as any other case
// is, by its exit code and the files it leaves: no rule of its own. This
// tree's own corpus, run without any of this, still pins every word exactly.
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
// It imports nothing but the standard library, as tools/bin/deps-check and
// tools/bin/schema-contract do (T-067).
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
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/donvargax/itos/v6/internal/release"
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
	top, err := git("rev-parse", "--show-toplevel")
	if err != nil {
		return fail("%v", err)
	}
	top = strings.TrimSpace(top)
	if _, err := git("worktree", "add", "--detach", tree, tag); err != nil {
		return fail("cannot check out %s into a scratch worktree: %v", tag, err)
	}
	if err := prepare(tree, top); err != nil {
		return fail("%s's worktree: %v", tag, err)
	}
	what := judged
	if *bin == "" {
		what = "this tree's itos, built as its release would be"
	}
	fmt.Printf("%s: %s's scenarios and conformance corpus against %s (itos %s), in a scratch worktree of %s\n",
		self, tag, what, version, tag)
	corpusReadied, err := readyOldCorpus(tree)
	if err != nil {
		return fail("%s's corpus: cannot leave out its plain output and its message and fix keys: %v", tag, err)
	}
	cases := corpusReadied.Names
	fmt.Printf("%s: %s's conformance corpus: its %s judged by itos's machine output alone, by this tree's runner (--additive):\n"+
		"  each one's exit code, files_after and json, a json key added passing and one removed or changed failing;\n"+
		"  left out as for people, the plain stdout and stderr of %s and %s named message or fix in their json\n"+
		"  (docs/decisions/0035-only-machine-output-is-itos-s-contract-exit-codes-json-less-message-and-fix-and-the-files-it-writes.md);\n"+
		"  this tree's own corpus pins every output exactly\n",
		self, tag, counted(corpusReadied.Cases, "case", "cases"),
		counted(corpusReadied.Plain, "case", "cases"), counted(corpusReadied.Keys, "key", "keys"))

	var features, corpus suite
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); features = runFeatures(tree, binary) }()
	go func() { defer wg.Done(); corpus = runCorpus(tree, top, binary) }()
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

// prepare readies the release's worktree: the checkout top's node_modules is
// linked in, for the YAML parser oldCorpusScript reads its fixtures with.
func prepare(tree, top string) error {
	modules := filepath.Join(top, "node_modules")
	if _, err := os.Stat(modules); err != nil {
		return fmt.Errorf("no node_modules in this checkout, which the corpus runner needs (vp install)")
	}
	return os.Symlink(modules, filepath.Join(tree, "node_modules"))
}

// oldCorpusScript readies the fixtures of the corpus in its working directory
// for the run, leaving in each case only what itos's contract holds it to
// (docs/decisions/0035-only-machine-output-is-itos-s-contract-exit-codes-json-less-message-and-fix-and-the-files-it-writes.md):
// it takes out every case's stdout, stderr, stdout_has and stderr_has, which
// pin plain text, and every key named message or fix at any depth of its json,
// which are the same sentences; the exit code, files_after, every other json
// key and what a case sets up stay as written. It prints how many cases it
// read, how many of them lost their plain output, how many message and fix
// keys it left out, and the names of every file's cases, as JSON
// ({"cases": n, "plain": n, "keys": n, "names": {"<file>": ["<name>", …]}}).
//
// It runs with the yaml package prepare links in, the one the corpus runner
// parses fixtures with, and writes each file back from what it parsed, which
// it checks reads back the same; a file that does not parse is left for the
// runner to report.
const oldCorpusScript = `
import { readdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { parse, stringify } from "yaml";

const dir = "tools/itos/conformance";
const plain = ["stdout", "stderr", "stdout_has", "stderr_has"];
const sentences = new Set(["message", "fix"]);
const told = { cases: 0, plain: 0, keys: 0, names: {} };
const machine = (value) => {
	if (Array.isArray(value)) return value.map(machine);
	if (value === null || typeof value !== "object") return value;
	const kept = {};
	for (const [key, v] of Object.entries(value)) {
		if (sentences.has(key)) told.keys++;
		else kept[key] = machine(v);
	}
	return kept;
};
for (const file of readdirSync(dir).filter((f) => f.endsWith(".yaml")).sort()) {
	const path = join(dir, file);
	told.names[file] = [];
	let doc;
	try {
		doc = parse(readFileSync(path, "utf8"));
	} catch {
		continue;
	}
	if (!doc || !Array.isArray(doc.cases)) continue;
	told.names[file] = doc.cases.flatMap((c) => (c && typeof c.name === "string" ? [c.name] : []));
	const cases = doc.cases.map((c) => {
		if (!c || typeof c !== "object" || Array.isArray(c)) return c;
		told.cases++;
		const kept = { ...c };
		if (plain.some((key) => Object.hasOwn(kept, key))) told.plain++;
		for (const key of plain) delete kept[key];
		if (Object.hasOwn(kept, "json")) kept.json = machine(kept.json);
		return kept;
	});
	const readied = { ...doc, cases };
	const text = stringify(readied, { lineWidth: 0 });
	if (JSON.stringify(parse(text)) !== JSON.stringify(readied))
		throw new Error(file + ": its cases do not read back as readied");
	writeFileSync(path, text);
}
console.log(JSON.stringify(told));
`

// readied is what oldCorpusScript did to the release's corpus: how many cases
// it read, how many lost their plain output, how many message and fix keys it
// left out of their json, and each corpus file's case names, by its base name,
// which Changes: entries are read against.
type readied struct {
	Cases int                 `json:"cases"`
	Plain int                 `json:"plain"`
	Keys  int                 `json:"keys"`
	Names map[string][]string `json:"names"`
}

// readyOldCorpus readies the release's corpus in tree as oldCorpusScript
// does, and says what it did.
func readyOldCorpus(tree string) (readied, error) {
	var r readied
	cmd := exec.Command("node", "--input-type=module", "-")
	cmd.Dir = tree
	cmd.Env = env()
	cmd.Stdin = strings.NewReader(oldCorpusScript)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return r, fmt.Errorf("node: %v\n%s", err, stderr.String())
	}
	if err := json.Unmarshal(out, &r); err != nil {
		return r, fmt.Errorf("node said %q: %v", out, err)
	}
	return r, nil
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
		"-ldflags", "-s -w -X github.com/donvargax/itos/v6/internal/version.stamp="+next,
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
		id, ok := byName[name]
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

var corpusFail = regexp.MustCompile(`^FAIL (\S+\.yaml): (.+)$`)

// runCorpus runs the release's corpus against bin with the runner of the
// checkout at top, judging it additively, and names each case that fails by
// its file and its name.
func runCorpus(tree, top, bin string) suite {
	s := suite{name: "conformance corpus"}
	runner := filepath.Join(top, "tools/itos/conformance/run.ts")
	fixtures, err := filepath.Glob(filepath.Join(tree, "tools/itos/conformance/*.yaml"))
	if err == nil && len(fixtures) == 0 {
		err = fmt.Errorf("it has no tools/itos/conformance/*.yaml")
	}
	if err != nil {
		s.err = err
		return s
	}
	sort.Strings(fixtures)
	started := time.Now()
	cmd := exec.Command("node", append([]string{runner, "--bin", bin, "--additive", "--only"}, fixtures...)...)
	cmd.Dir = tree
	cmd.Env = env()
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	s.seconds = time.Since(started).Seconds()
	last := strings.TrimSpace(stdout.String())
	if i := strings.LastIndex(last, "\n"); i >= 0 {
		last = last[i+1:]
	}
	s.summary = last
	if runErr == nil {
		return s
	}
	if exit, ok := runErr.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
		s.output = stdout.String() + stderr.String()
		s.err = fmt.Errorf("node %s: %v", runner, runErr)
		return s
	}
	for _, line := range strings.Split(stderr.String(), "\n") {
		if m := corpusFail.FindStringSubmatch(line); m != nil {
			key := caseKey(m[1], m[2])
			s.failures = append(s.failures, failure{Key: key, What: "conformance case " + key})
		}
	}
	s.output = stderr.String()
	if len(s.failures) == 0 {
		s.output = stdout.String() + stderr.String()
		s.err = fmt.Errorf("the corpus failed, and no case did")
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
