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
// The release's corpus runs through this tree's runner, in its --additive
// mode (T-076): an old case's JSON is judged as what the new output must still
// hold, every key it expects there with the value it expects, at every depth,
// an array element by element at the same length, and its stdout and stderr as
// lines that must all still appear, in the same order. A key or a line added
// passes, since an output that only adds breaks no consumer (PLAN.md §7: keys
// only ever added); one removed, changed or reordered fails. Its exit code,
// stdout_has, stderr_has and files_after are judged as ever. This tree's own
// corpus, run without the mode, still pins every output exactly. The runner
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
// The old corpus's help cases are left out, never judged: a case that asks for
// help (an argv holding --help or -h before any --, one starting with help, or
// none at all, a bare itos printing the help) pins help text, which is
// documentation, not compatibility: --json and exit codes are the stable
// interface (PLAN.md), and this tree's own corpus still pins its help exactly.
// Judged, every feat that adds a flag or a command, which changes itos --help,
// could pass only as a breaking change. They are taken out of the worktree's
// fixtures before its runner reads them (oldCorpusScript, with the checkout's
// yaml), and the check says how many it left out and why. help.yaml's other
// cases, such as itos version's, stay judged.
//
// The old corpus's usage errors are judged by their exit code alone (T-075):
// a case whose argv the command refuses, exit 2 with one "itos: <message>
// (itos --help)" line on stderr and nothing on stdout, pins a usage message
// that lists what the command takes, documentation as help is, and judged
// word for word every feat that adds a flag to a command could not say so
// there without a breaking change. The same script takes such a case's stdout
// and stderr out, keeping its exit code, 2, and the check says how many it
// relaxed and why. A config error (config check's "FAIL …" lines, or an
// "itos: … is missing" about a file the config names) keeps its words judged,
// since what a config may hold is the stable interface too, and so does any
// case that pins its output another way. This tree's own corpus still pins
// every usage message exactly.
//
// An old scenario or case that fails is accepted when a commit since the tag
// is marked as breaking (a ! before its header's colon, or a BREAKING-CHANGE:
// or BREAKING CHANGE: footer in its last paragraph, read as
// tools/bin/schema-contract reads them), or when a fix since the tag names it
// in a Changes: footer: a fix whose old scenario held the bug stays a patch,
// said in its commit. A feat's Changes: never excuses one: a feat that changes
// what an old scenario promised is a breaking change. Changes: is a footer of
// free text (itos.yaml's commits.footers, written by itos commit --changes),
// one entry a line:
//
//	Changes: @ID-CMSG-03 @ID-CMSG-04    scenarios, by their IDs
//	Changes: hooks.yaml: <case name>    a corpus case, by its file and its name,
//	                                    as the runner's FAIL line names it
//
// An entry that names no scenario or case of the release is printed as a
// warning, never a failure: a pushed commit cannot be rewritten, and a failure
// would hold CI red until a release that a red CI never cuts.
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
	skipped, relaxed, err := readyOldCorpus(tree)
	if err != nil {
		return fail("%s's corpus: cannot leave out its help cases and relax its usage errors: %v", tag, err)
	}
	fmt.Printf("%s: %s's conformance corpus: %s not judged, since help text is documentation, not compatibility:\n"+
		"  --json and exit codes are the stable interface (PLAN.md), and this tree's own corpus pins its help exactly\n",
		self, tag, skipped)
	fmt.Printf("%s: %s's conformance corpus: %s judged by their exit code alone:\n"+
		"  a usage message is documentation, as help is, and a refused argument's exit code, 2, the stable interface\n"+
		"  (PLAN.md §7); this tree's own corpus pins its usage messages exactly, and a config error's words stay judged\n",
		self, tag, relaxed)
	fmt.Printf("%s: %s's conformance corpus: judged additively, by this tree's runner (--additive):\n"+
		"  an old case's JSON must still hold every key it expects with the value it expects, and its stdout and\n"+
		"  stderr every line it expects, in order; a key or a line added passes (PLAN.md §7: keys only ever added),\n"+
		"  one removed, changed or reordered does not, and this tree's own corpus pins every output exactly\n",
		self, tag)

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
	warnChanges(tag, tree, commits, ids)
	status := report(tag, failures, commits, features, corpus)
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
// for the run, keeping the rest of each file as written: it takes the help
// cases out, and takes the stdout and stderr out of each usage error's case,
// so the runner judges it by its exit code alone. It prints how many of each
// it touched in each file as JSON ({"help": {"<file>": n}, "usage": {…}}).
//
// A usage error's case is one whose argv the command refuses as a usage
// error: exit 2, nothing on stdout (or stdout not compared), and on stderr
// exactly one line, "itos: <message> (itos --help)", the line cli's failure
// prints for a usage error and for nothing else. A config error prints
// config check's "FAIL <config>: …" lines, or "itos: …" without the help's
// name ("… is missing" about a file the config names); a case pinning its
// output any other way (stdout_has, stderr_has, json) is not read as one.
// Either stays judged word for word. A case's files_after stays judged.
//
// It runs with the yaml package prepare links in, the one the corpus runner
// parses fixtures with; a file that does not parse is left for the runner to
// report.
const oldCorpusScript = `
import { readdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { isMap, isSeq, parse, parseDocument } from "yaml";

const dir = "tools/itos/conformance";
const asksHelp = (argv) => {
	if (!Array.isArray(argv)) return false;
	if (argv.length === 0 || argv[0] === "help") return true;
	const end = argv.indexOf("--");
	const flags = end < 0 ? argv : argv.slice(0, end);
	return flags.includes("--help") || flags.includes("-h");
};
const usageError = (c) =>
	c !== null &&
	typeof c === "object" &&
	c.exit === 2 &&
	typeof c.stderr === "string" &&
	/^itos: [^\n]* \(itos --help\)\n$/.test(c.stderr) &&
	(c.stdout === undefined || c.stdout === "") &&
	["stdout_has", "stderr_has", "json"].every((key) => c[key] === undefined);
const touched = { help: {}, usage: {} };
for (const file of readdirSync(dir).filter((f) => f.endsWith(".yaml")).sort()) {
	const path = join(dir, file);
	const doc = parseDocument(readFileSync(path, "utf8"));
	const cases = doc.get("cases");
	if (doc.errors.length || !isSeq(cases)) continue;
	const plain = doc.toJS().cases;
	const help = new Set(plain.flatMap((c, i) => (asksHelp(c && c.argv) ? [i] : [])));
	const usage = plain.flatMap((c, i) => (!help.has(i) && usageError(c) ? [c.name] : []));
	if (!help.size && !usage.length) continue;
	for (const item of cases.items)
		if (isMap(item) && usage.includes(item.get("name"))) for (const key of ["stdout", "stderr"]) item.delete(key);
	cases.items = cases.items.filter((_, i) => !help.has(i));
	const text = String(doc);
	const meant = plain.flatMap((c, i) => {
		if (help.has(i)) return [];
		if (!usage.includes(c.name)) return [c];
		const { stdout, stderr, ...judged } = c;
		return [judged];
	});
	if (JSON.stringify(parse(text).cases) !== JSON.stringify(meant))
		throw new Error(file + ": its cases do not read back as written, less its help cases and its usage errors' words");
	writeFileSync(path, text);
	if (help.size) touched.help[file] = help.size;
	if (usage.length) touched.usage[file] = usage.length;
}
console.log(JSON.stringify(touched));
`

// readyOldCorpus readies the release's corpus in tree as oldCorpusScript
// does, and says what it touched: "37 help cases (help.yaml 37)" and "34 usage
// errors (cli.yaml 19, …)".
func readyOldCorpus(tree string) (help, usage string, err error) {
	cmd := exec.Command("node", "--input-type=module", "-")
	cmd.Dir = tree
	cmd.Env = env()
	cmd.Stdin = strings.NewReader(oldCorpusScript)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("node: %v\n%s", err, stderr.String())
	}
	var touched struct{ Help, Usage map[string]int }
	if err := json.Unmarshal(out, &touched); err != nil {
		return "", "", fmt.Errorf("node said %q: %v", out, err)
	}
	return counted(touched.Help, "help case", "help cases"), counted(touched.Usage, "usage error", "usage errors"), nil
}

// counted says how many cases counts holds, and how many in each file:
// "37 help cases (help.yaml 37)", or "no help cases".
func counted(counts map[string]int, one, many string) string {
	files := make([]string, 0, len(counts))
	total := 0
	for file, n := range counts {
		files = append(files, fmt.Sprintf("%s %d", file, n))
		total += n
	}
	sort.Strings(files)
	switch total {
	case 0:
		return "no " + many
	case 1:
		return fmt.Sprintf("1 %s (%s)", one, files[0])
	}
	return fmt.Sprintf("%d %s (%s)", total, many, strings.Join(files, ", "))
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
		"-ldflags", "-s -w -X github.com/donvargax/itos/v2/internal/version.stamp="+next,
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

// excuse is why a failure is accepted, or "" when nothing excuses it; named
// is a feat that names it in Changes:, which never excuses one.
func excuse(f failure, commits []commit) (why string, named *commit) {
	for i := range commits {
		c := &commits[i]
		if c.Breaking {
			return fmt.Sprintf("%.7s %q marks a breaking change", c.SHA, c.Header), nil
		}
	}
	for i := range commits {
		c := &commits[i]
		if !names(c.Changes, f.Key) {
			continue
		}
		if c.Type == "fix" {
			return fmt.Sprintf("the fix %.7s %q names it in Changes:", c.SHA, c.Header), nil
		}
		if named == nil {
			named = c
		}
	}
	return "", named
}

// names is whether a commit's Changes: entries name key: a scenario by its ID
// among the entry's words, a case by its file and its name.
func names(entries []string, key string) bool {
	for _, e := range entries {
		if ids, ok := entryIDs(e); ok {
			for _, id := range ids {
				if id == key {
					return true
				}
			}
		} else if file, name, ok := entryCase(e); ok && caseKey(file, name) == key {
			return true
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

// warnChanges prints each Changes: entry since the tag that names no scenario
// or case of the release, or is neither form.
func warnChanges(tag, tree string, commits []commit, ids map[string]bool) {
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
				text, err := os.ReadFile(filepath.Join(tree, "tools/itos/conformance", filepath.Base(file)))
				switch {
				case err != nil:
					why = fmt.Sprintf("%s has no corpus file %s", tag, filepath.Base(file))
				case !bytes.Contains(text, []byte(name)):
					why = fmt.Sprintf("%s's %s has no case %q", tag, filepath.Base(file), name)
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
func report(tag string, failures []failure, commits []commit, features, corpus suite) int {
	if len(failures) == 0 {
		fmt.Printf("%s: this tree's itos keeps every promise of %s's scenarios and corpus\n", self, tag)
		return 0
	}
	var refused []string
	for _, f := range failures {
		why, named := excuse(f, commits)
		if why != "" {
			fmt.Printf("%s: %s's %s fails, accepted: %s\n", self, tag, f.What, why)
			continue
		}
		line := fmt.Sprintf("%s: %s's %s fails against this tree's itos", self, tag, f.What)
		if named != nil {
			line += fmt.Sprintf(", and only the %s %.7s %q names it in Changes:, which never excuses one", named.Type, named.SHA, named.Header)
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
	fmt.Fprintf(os.Stderr, "\n%s: %d of %s's scenarios and cases fail against this tree's itos, and no commit since %s says why:\n"+
		"  the behaviour each promised has changed, and the release would ship it as a minor or a patch.\n"+
		"  If a fix changed it and the old scenario or case held the bug, name each in a fix's Changes: footer\n"+
		"  (itos commit --changes '@ID-…', or --changes '<file>.yaml: <case name>' for a case), so the release\n"+
		"  stays a patch; a feat's Changes: never excuses one. Otherwise mark the change as breaking with a\n"+
		"  BREAKING-CHANGE: footer saying what a consumer must change (itos commit --breaking '<what to change>')\n"+
		"  or a ! in its header (feat!: …), so the release is a major; or keep the old behaviour.\n",
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
	version    = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)
)

func anyReleasable(headers string) bool {
	for _, h := range strings.Split(headers, "\n") {
		if releasable.MatchString(h) {
			return true
		}
	}
	return false
}

// lastRelease is the newest vX.Y.Z tag reachable from HEAD, by version, or ""
// for none.
func lastRelease() (string, error) {
	out, err := git("tag", "--merged", "HEAD", "--list", "v*", "--sort=-v:refname")
	if err != nil {
		return "", err
	}
	for _, t := range strings.Split(out, "\n") {
		if version.MatchString(t) {
			return t, nil
		}
	}
	return "", nil
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
		commits = append(commits, c)
	}
	return commits, nil
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
