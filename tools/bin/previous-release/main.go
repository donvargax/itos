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
//	node tools/itos/conformance/run.ts --bin <bin>
//
// The corpus's {{version}} is the version the binary must say it is, which the
// runner reads from package.json beside it: the worktree's package.json is
// given the binary's own version first (`<bin> version`), so a binary stamped
// with a version the tag never had is not failed for saying it.
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
// -bin is the itos to judge, tools/bin/itos by default. Exit status: 0 passed,
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
	bin := flag.String("bin", "tools/bin/itos", "the itos to run the last release's scenarios and corpus against")
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

	binary, err := filepath.Abs(*bin)
	if err != nil {
		return fail("%v", err)
	}
	version, err := binaryVersion(binary)
	if err != nil {
		return fail("%v", err)
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
	if _, err := git("worktree", "add", "--detach", tree, tag); err != nil {
		return fail("cannot check out %s into a scratch worktree: %v", tag, err)
	}
	if err := prepare(tree, version); err != nil {
		return fail("%s's worktree: %v", tag, err)
	}
	fmt.Printf("%s: %s's scenarios and conformance corpus against %s (itos %s), in a scratch worktree of %s\n",
		self, tag, *bin, version, tag)

	var features, corpus suite
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); features = runFeatures(tree, binary) }()
	go func() { defer wg.Done(); corpus = runCorpus(tree, binary) }()
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

// prepare readies the release's worktree: package.json says the binary's own
// version, for the corpus's {{version}}, and the checkout's node_modules is
// linked in, for the corpus runner's YAML parser.
func prepare(tree, version string) error {
	pkg := filepath.Join(tree, "package.json")
	text, err := os.ReadFile(pkg)
	if err != nil {
		return err
	}
	field := regexp.MustCompile(`"version"\s*:\s*"[^"]*"`)
	if !field.Match(text) {
		return fmt.Errorf("package.json says no version, which the corpus's {{version}} reads")
	}
	done := false
	text = field.ReplaceAllFunc(text, func(m []byte) []byte {
		if done {
			return m
		}
		done = true
		return []byte(fmt.Sprintf(`"version": %q`, version))
	})
	if err := os.WriteFile(pkg, text, 0o644); err != nil {
		return err
	}
	top, err := git("rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	modules := filepath.Join(strings.TrimSpace(top), "node_modules")
	if _, err := os.Stat(modules); err != nil {
		return fmt.Errorf("no node_modules in this checkout, which the corpus runner needs (vp install)")
	}
	return os.Symlink(modules, filepath.Join(tree, "node_modules"))
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

// runCorpus runs the release's corpus against bin with the release's runner,
// and names each case that fails by its file and its name.
func runCorpus(tree, bin string) suite {
	s := suite{name: "conformance corpus"}
	started := time.Now()
	cmd := exec.Command("node", "tools/itos/conformance/run.ts", "--bin", bin)
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
		s.err = fmt.Errorf("node tools/itos/conformance/run.ts: %v", runErr)
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
