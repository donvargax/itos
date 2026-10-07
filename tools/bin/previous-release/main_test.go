package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// fakeEnv makes this test binary the fake itos the old corpus runs against in
// TestOldCorpusJudgesMachineOutputAlone: set, TestMain answers as fakeItos
// does instead of running the tests.
const fakeEnv = "PREVIOUS_RELEASE_FAKE_ITOS"

func TestMain(m *testing.M) {
	if os.Getenv(fakeEnv) != "" {
		os.Exit(fakeItos(os.Args[1:]))
	}
	os.Exit(m.Run())
}

// fakeItos is the itos that oldCorpus's cases meet: one whose words have all
// changed since the release, and whose machine output has changed where a
// case's name says it has.
func fakeItos(args []string) int {
	problems := func(rule string) {
		out, _ := json.Marshal(map[string]any{
			"schema": 1, "ok": false, "count": 1,
			"problems": []map[string]any{{
				"rule": rule, "message": "said anew", "fix": "a fix said anew",
				"detail": map[string]any{"message": "deeper, said anew", "line": 3},
			}},
		})
		fmt.Println(string(out))
	}
	switch strings.Join(args, " ") {
	case "version":
		fmt.Println("itos 5.1.0")
	case "--help":
		fmt.Println("Usage: itos [command], said anew")
	case "refuse":
		fmt.Fprintln(os.Stderr, "itos: refuse takes nothing yet, said anew")
		return 2
	case "succeed":
		fmt.Println("finished, said anew")
	case "check reworded":
		problems("r-one")
		return 1
	case "check renamed":
		problems("r-two")
		return 1
	case "exit":
		return 3
	case "range":
		fmt.Println(`{"schema":1}`)
	case "write":
		if err := os.WriteFile("out.txt", []byte("new\n"), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 3
		}
	default:
		fmt.Fprintf(os.Stderr, "fake itos: no answer for %q\n", args)
		return 3
	}
	return 0
}

// A fixture of the last release's corpus: four cases whose words fakeItos
// changes, which pass, and four whose machine output it changes, which fail.
const oldCorpus = `cases:
  - name: help, said anew
    argv: [--help]
    exit: 0
    stdout: "usage: itos <command>\n"
  - name: a refusal, said anew
    argv: [refuse]
    exit: 2
    stderr: "itos: refuse is not written yet (itos --help)\n"
    stderr_has: not written yet
  - name: a success, said anew
    argv: [succeed]
    exit: 0
    stdout: "done\n"
    stdout_has: [done]
  - name: message and fix, said anew
    argv: [check, reworded]
    exit: 1
    json:
      schema: 1
      ok: false
      problems:
        - rule: r-one
          message: the old words
          fix: the old fix
          detail: { message: the old deeper words, line: 3 }
  - name: an exit code, changed
    argv: [exit]
    exit: 1
  - name: a rule id, changed
    argv: [check, renamed]
    exit: 1
    json: { schema: 1, problems: [{ rule: r-one, message: the old words }] }
  - name: a json key, removed
    argv: [range]
    exit: 0
    json: { schema: 1, from: abc1234 }
  - name: files_after, changed
    argv: [write]
    exit: 0
    stdout: ""
    files_after:
      out.txt: "old\n"
`

// oldTree is a checkout of a release whose corpus is oldCorpus, as cli.yaml,
// readied as run readies one; it skips the test without node or the yaml
// package oldCorpusScript and the runner read the fixtures with.
func oldTree(t *testing.T) (tree, top string) {
	t.Helper()
	top, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not on the PATH, and oldCorpusScript and the corpus runner run in it: skipping")
	}
	modules := filepath.Join(top, "node_modules")
	if _, err := os.Stat(filepath.Join(modules, "yaml")); err != nil {
		t.Skipf("%s has no yaml package, which the fixtures are read with (vp install): skipping", modules)
	}
	tree = t.TempDir()
	dir := filepath.Join(tree, "tools/itos/conformance")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cli.yaml"), []byte(oldCorpus), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := prepare(tree, top); err != nil {
		t.Fatal(err)
	}
	return tree, top
}

// readyOldCorpus takes out of every old case what is for people, its plain
// output and its json's message and fix keys at any depth, and keeps the rest
// as written: the exit code, files_after and every other json key
// (docs/decisions/0035-only-machine-output-is-itos-s-contract-exit-codes-json-less-message-and-fix-and-the-files-it-writes.md).
func TestReadyOldCorpusKeepsMachineOutputOnly(t *testing.T) {
	tree, _ := oldTree(t)
	r, err := readyOldCorpus(tree)
	if err != nil {
		t.Fatal(err)
	}
	if r.Cases != 8 || r.Plain != 4 || r.Keys != 4 {
		t.Errorf("cases read, cases whose plain output it left out, message and fix keys it left out: got %d, %d, %d, want 8, 4, 4",
			r.Cases, r.Plain, r.Keys)
	}
	if got := len(r.Names["cli.yaml"]); got != 8 {
		t.Errorf("the case names, as written: got %d, want 8: %q", got, r.Names["cli.yaml"])
	}

	written, err := os.ReadFile(filepath.Join(tree, "tools/itos/conformance/cli.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(written)
	var wrong []string
	for _, gone := range []string{"stdout", "stderr", "message:", "fix:", "the old"} {
		if strings.Contains(text, gone) {
			wrong = append(wrong, fmt.Sprintf("it should leave out %q", gone))
		}
	}
	for _, kept := range []string{
		"name: help, said anew",
		"exit: 2",
		"rule: r-one",
		"line: 3",
		"from: abc1234",
		"out.txt:",
	} {
		if !strings.Contains(text, kept) {
			wrong = append(wrong, fmt.Sprintf("it should keep %q", kept))
		}
	}
	if len(wrong) > 0 {
		t.Errorf("the run's fixture:\n  %s\nit reads:\n%s", strings.Join(wrong, "\n  "), text)
	}
}

// The old corpus, readied and run against an itos that rewords everything:
// a reworded stdout or stderr, help text included, and a reworded message or
// fix pass; a changed exit code, rule id or files_after, or a json key
// removed, fails (docs/decisions/0035-only-machine-output-is-itos-s-contract-exit-codes-json-less-message-and-fix-and-the-files-it-writes.md).
func TestOldCorpusJudgesMachineOutputAlone(t *testing.T) {
	tree, top := oldTree(t)
	if _, err := readyOldCorpus(tree); err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(fakeEnv, "1")
	s := runCorpus(tree, top, self)
	if s.err != nil {
		t.Fatalf("the corpus could not run: %v\n%s", s.err, s.output)
	}
	var failed []string
	for _, f := range s.failures {
		failed = append(failed, f.Key)
	}
	sort.Strings(failed)
	want := []string{
		"cli.yaml: a json key, removed",
		"cli.yaml: a rule id, changed",
		"cli.yaml: an exit code, changed",
		"cli.yaml: files_after, changed",
	}
	if !reflect.DeepEqual(failed, want) {
		t.Errorf("the cases that fail:\n  got  %q\n  want %q\nthe runner said:\n%s", failed, want, s.output)
	}
}

// The release's corpus the excuse tests name cases of.
var excuseCases = map[string][]string{
	"hooks.yaml": {"pre-push refuses an unpushed task", "pre-push passes a clean range"},
}

// Commits since a release, as their messages read: parsed as commitsSince
// parses them.
var (
	breakingNamesNothing = parseCommit("1111111aaaa", "feat: say the contract\n\nWhy.\n\nScenarios: @ID-NEW-01\nUpgrading: read the contract\nBREAKING-CHANGE: only machine output is the contract")
	breakingNamesOne     = parseCommit("2222222bbbb", "feat: rename a rule\n\nWhy.\n\nScenarios: @ID-NEW-02\nUpgrading: read r-two\nBREAKING-CHANGE: r-one is now r-two\nChanges: @ID-CMSG-03")
	bangNamesCase        = parseCommit("3333333cccc", "feat!: drop a key\n\nWhy.\n\nScenarios: @ID-NEW-03\nUpgrading: stop reading from\nChanges: hooks.yaml: pre-push refuses an")
	fixNamesOne          = parseCommit("4444444dddd", "fix: refuse what was let through\n\nWhy.\n\nScenarios: @bug-9\nUpgrading: none\nChanges: @ID-CMSG-04")
	featNamesOne         = parseCommit("5555555eeee", "feat: change an old promise quietly\n\nWhy.\n\nScenarios: @ID-NEW-04\nUpgrading: none\nChanges: @ID-CMSG-05")
)

// excuse accepts an old failure only when a commit names it in Changes: and
// is a fix or breaking (T-106); a breaking commit excuses nothing it does not
// name, and a feat that names one without being breaking excuses nothing.
func TestExcuseOnlyWhatACommitNames(t *testing.T) {
	for _, c := range []struct {
		name    string
		failure string
		commits []commit
		excused bool
		named   string // the SHA of the commit that names it without excusing it
	}{
		{"a breaking commit that names nothing excuses nothing", "@ID-CMSG-01",
			[]commit{breakingNamesNothing}, false, ""},
		{"nor a failure another breaking commit names", "@ID-CMSG-01",
			[]commit{breakingNamesNothing, breakingNamesOne}, false, ""},
		{"a breaking commit excuses a scenario it names", "@ID-CMSG-03",
			[]commit{breakingNamesNothing, breakingNamesOne}, true, ""},
		{"a ! header excuses a case it names by a prefix", "hooks.yaml: pre-push refuses an unpushed task",
			[]commit{bangNamesCase}, true, ""},
		{"but not the file's other case", "hooks.yaml: pre-push passes a clean range",
			[]commit{bangNamesCase}, false, ""},
		{"a fix excuses a scenario it names", "@ID-CMSG-04",
			[]commit{breakingNamesNothing, fixNamesOne}, true, ""},
		{"a feat that is not breaking excuses nothing it names", "@ID-CMSG-05",
			[]commit{featNamesOne, breakingNamesNothing}, false, featNamesOne.SHA},
	} {
		t.Run(c.name, func(t *testing.T) {
			why, named := excuse(failure{Key: c.failure}, c.commits, excuseCases)
			if (why != "") != c.excused {
				t.Errorf("excused: got %q, want excused %v", why, c.excused)
			}
			got := ""
			if named != nil {
				got = named.SHA
			}
			if got != c.named {
				t.Errorf("the commit that names it without excusing it: got %q, want %q", got, c.named)
			}
		})
	}
}

// The commits parsed as commitsSince reads them: whether each is breaking,
// and the Changes: entries it gives.
func TestParseCommit(t *testing.T) {
	for _, c := range []struct {
		commit   commit
		typ      string
		breaking bool
		changes  []string
	}{
		{breakingNamesNothing, "feat", true, nil},
		{breakingNamesOne, "feat", true, []string{"@ID-CMSG-03"}},
		{bangNamesCase, "feat", true, []string{"hooks.yaml: pre-push refuses an"}},
		{fixNamesOne, "fix", false, []string{"@ID-CMSG-04"}},
		{featNamesOne, "feat", false, []string{"@ID-CMSG-05"}},
	} {
		if c.commit.Type != c.typ || c.commit.Breaking != c.breaking || !reflect.DeepEqual(c.commit.Changes, c.changes) {
			t.Errorf("%q: got type %q, breaking %v, changes %q; want %q, %v, %q",
				c.commit.Header, c.commit.Type, c.commit.Breaking, c.commit.Changes, c.typ, c.breaking, c.changes)
		}
	}
}

// report fails a range whose only breaking commit names none of the old
// failures, as v6.0.0's did, and tells the reader to name each failure in
// Changes:, on a breaking commit too; it passes once every failure is named
// by a fix or a breaking commit.
func TestReportRefusesWhatNoCommitNames(t *testing.T) {
	failures := []failure{
		{Key: "@ID-CMSG-01", What: "scenario @ID-CMSG-01"},
		{Key: "@ID-CMSG-03", What: "scenario @ID-CMSG-03"},
		{Key: "hooks.yaml: pre-push refuses an unpushed task", What: "conformance case hooks.yaml: pre-push refuses an unpushed task"},
	}
	status, stdout, stderr := reported(t, failures, []commit{breakingNamesNothing, breakingNamesOne})
	if status != 1 {
		t.Errorf("one breaking commit naming one of three failures: got exit %d, want 1\n%s%s", status, stdout, stderr)
	}
	for _, want := range []string{
		"scenario @ID-CMSG-01 fails against this tree's itos",
		"case hooks.yaml: pre-push refuses an unpushed task fails against this tree's itos",
		"2 of v1.0.0's scenarios and cases fail",
		"Name each failure in a Changes: footer",
		"on a breaking commit too",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("what it says of the failures should hold %q; it said:\n%s", want, stderr)
		}
	}
	if !strings.Contains(stdout, "scenario @ID-CMSG-03 fails, accepted: the breaking change 2222222") {
		t.Errorf("the failure the breaking commit names should be accepted; it said:\n%s", stdout)
	}

	named := parseCommit("6666666ffff", "fix: name the rest\n\nWhy.\n\nScenarios: @bug-10\nUpgrading: none\nChanges: @ID-CMSG-01")
	status, stdout, stderr = reported(t, failures, []commit{breakingNamesNothing, breakingNamesOne, bangNamesCase, named})
	if status != 0 {
		t.Errorf("every failure named by a fix or a breaking commit: got exit %d, want 0\n%s%s", status, stdout, stderr)
	}
}

// reported runs report on failures and commits against a release v1.0.0,
// and gives its exit status and what it printed.
func reported(t *testing.T, failures []failure, commits []commit) (status int, stdout, stderr string) {
	t.Helper()
	dir := t.TempDir()
	out, err := os.Create(filepath.Join(dir, "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	errs, err := os.Create(filepath.Join(dir, "stderr"))
	if err != nil {
		t.Fatal(err)
	}
	keptOut, keptErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = out, errs
	status = report("v1.0.0", failures, commits, excuseCases,
		suite{name: "scenarios", failures: failures[:2]}, suite{name: "conformance corpus", failures: failures[2:]})
	os.Stdout, os.Stderr = keptOut, keptErr
	_ = out.Close()
	_ = errs.Close()
	o, err := os.ReadFile(filepath.Join(dir, "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	e, err := os.ReadFile(filepath.Join(dir, "stderr"))
	if err != nil {
		t.Fatal(err)
	}
	return status, string(o), string(e)
}
