// The previous-release check's self-test (T-071, T-106, T-117, a Go test since T-143): the last
// release's scenarios and corpus against the new binary, in a scratch repository whose v1.0.0 tag
// holds a release of its own: two scenarios (go test ./features, a black box running whatever
// ITOS_BIN names, as this repository's are), and a corpus of four cases and two help cases run by
// this repository's own conformance runner; two of the four start with the same word, and one's
// name is longer than a footer line may be.
//
// The binary under test is a script; the broken one breaks one scenario and one case by its exit
// code, and says other help, the wordy one says other help and other usage errors and nothing
// else, and each says a version the tag never had, as T-069's builds will. Three more cases are
// refusals: two usage errors and a config error; the reworded binary says other words for all
// three and changes one usage error's exit code. Two more pin a report's lines and a JSON object
// holding a message and a fix, and one more writes a YAML and a Markdown file: the adding binary
// adds keys to the object, rewords its message and fix, rewrites the report and rewrites the
// files' comments, reordering the YAML's keys and adding one, and six more each break one of them
// by machine output, a written file by its data.
//
//   - the binary the release was written for passes, its version the corpus's {{version}}, and
//     what is left out of every case (plain output, message and fix) is said with its count;
//   - the wordy one passes: only machine output is judged (decision 35);
//   - the reworded one fails naming the usage error whose exit code changed, and neither the
//     other usage error nor the config error, whose words alone differ;
//   - the adding one passes (T-076, decision 35, T-112, decision 40); one with a key removed, a
//     value changed or an exit code changed, in the JSON or a written YAML file, or a written
//     Markdown file's text changed, fails, naming that case alone;
//   - the broken one fails with no commit since the tag that says why, naming the scenario, the
//     case and the remedy, and no help case; a fix and a ! before the tag do not count;
//   - it passes with a fix since the tag naming both in Changes: footers, and fails naming the
//     case alone when the fix names only the scenario;
//   - it passes with a fix naming the long case by a prefix of its name (T-081), and fails with
//     one naming it by a prefix another case starts with too, warning that it is ambiguous;
//   - it fails with a feat naming both;
//   - a breaking change excuses only what its Changes: names (T-106);
//   - a Changes: entry that names nothing of the release is a warning, not a failure;
//   - -range-from checks nothing when the range has no feat or fix; no release tag passes,
//     saying so; a shallow clone and a tag that cannot be checked out stop it with exit 2, never
//     a pass.
//
// The corpus runner is this tree's (tools/bin/conformance/corpus), linked into previous-release,
// so the scratch release's tree holds none of its own (T-144). Not on windows, where the
// binaries under test, shell scripts, cannot start.
package main_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/tools/selftest/scratch"
)

// longCase is a case whose name is longer than a footer line may be: "Changes: greet.yaml: <it>"
// is 126 characters, and the header lint caps a line at 100.
const longCase = "greets the world with hello on a line of its own, as it has since its first release and as scripts expect"

// release is the release's tree: its scenarios, the steps that run them and its corpus, as this
// repository lays them out; the runner is added from the checkout.
var release = map[string]string{
	"go.mod": "module example.com/scratch\n\ngo 1.22\n",
	"features/greet.feature": `Feature: Greeting

  # It greets as it always has.
  @ID-GREET-01 @slice-1
  Scenario: It greets the world
    When itos greet runs

  @ID-GREET-02 @slice-1
  Scenario: It says its name
    When itos name runs
`,
	"features/features_test.go": `package features

import (
	"os"
	"os/exec"
	"testing"
)

func say(t *testing.T, arg, want string) {
	out, err := exec.Command(os.Getenv("ITOS_BIN"), arg).Output()
	if err != nil || string(out) != want {
		t.Fatalf("itos %s said %q (%v), want %q", arg, out, err, want)
	}
}

func TestFeatures(t *testing.T) {
	t.Run("It greets the world", func(t *testing.T) { say(t, "greet", "hello\n") })
	t.Run("It says its name", func(t *testing.T) { say(t, "name", "itos\n") })
}
`,
	"tools/itos/conformance/greet.yaml": `cases:
  - name: ` + longCase + `
    argv: [greet]
    exit: 0
    stdout: "hello\n"
  - name: greets no one when asked its name
    argv: [name]
    exit: 0
    stdout: "itos\n"
  - name: says its name
    argv: [name]
    exit: 0
    stdout: "itos\n"
  - name: says its version
    argv: [version]
    exit: 0
    stdout: "itos {{version}}\n"
`,
	// A report's lines, which are for people, a JSON object, to which a new binary may add and
	// from which it may take nothing, its message and fix for people too, and a YAML and a
	// Markdown file it writes, whose data is its contract and whose comments are for people
	// (decision 40).
	"tools/itos/conformance/report.yaml": `cases:
  - name: reports three lines
    argv: [report]
    exit: 0
    stdout: "one\ntwo\nthree\n"
  - name: says its state as JSON
    argv: [state]
    exit: 0
    json: { state: { ready: true, items: [a, b], message: all ready }, count: 2, fix: nothing to fix }
  - name: writes its policy and its notes
    argv: [write]
    exit: 0
    files_after:
      policy.yaml: "# The policy, for people.\nname: scratch\nchecks: [a, b] # both of them\n"
      NOTES.md: "<!-- Written by itos. -->\n# Notes\n\nready <!-- for now -->\n"
`,
	// Refusals: two usage errors and a config error, judged as every case is, by their exit
	// codes alone, their words for people.
	"tools/itos/conformance/refuse.yaml": `cases:
  - name: an unknown command is a usage error
    argv: [wave]
    exit: 2
    stderr: "itos: unknown command: wave (itos --help)\n"
  - name: greet takes no argument
    argv: [greet, loudly]
    exit: 2
    stdout: ""
    stderr: "itos: greet takes no argument (loudly) (itos --help)\n"
  - name: a config that is not version 1
    argv: [check]
    exit: 2
    stderr: "FAIL itos.yaml: version 2 is not 1\n"
`,
	// Help cases, whose text is for people: one asking with --help, one with help <command>.
	"tools/itos/conformance/help.yaml": `cases:
  - name: itos --help
    argv: [--help]
    exit: 0
    stdout: "usage: itos greet|name|version\n"
  - name: help greet is greet's --help
    argv: [help, greet]
    exit: 0
    stdout: "usage: itos greet|name|version\n"
`,
}

// says is what a binary under test says; its zero values are the release's binary's.
type says struct {
	greeting, help, takes, config, report, state, policy, notes string
	greeted, refusal, reported                                  int
}

const (
	lines  = `one\ntwo\nthree\n`
	state  = `{"count":2,"fix":"nothing to fix","state":{"ready":true,"items":["a","b"],"message":"all ready"}}`
	policy = `# The policy, for people.\nname: scratch\nchecks: [a, b] # both of them\n`
	notes  = `<!-- Written by itos. -->\n# Notes\n\nready <!-- for now -->\n`
)

// script is the binary saying what s says, as a shell script.
func script(s says) string {
	or := func(v, d string) string {
		if v == "" {
			return d
		}
		return v
	}
	refusal := s.refusal
	if refusal == 0 {
		refusal = 2
	}
	itoa := func(n int) string { return string(rune('0' + n)) }
	return `#!/bin/sh
case "$1" in
--help | help) echo "` + or(s.help, "usage: itos greet|name|version") + `" ;;
report)
	printf '` + or(s.report, lines) + `'
	exit ` + itoa(s.reported) + ` ;;
state) echo '` + or(s.state, state) + `' ;;
write)
	printf '` + or(s.policy, policy) + `' >policy.yaml
	printf '` + or(s.notes, notes) + `' >NOTES.md ;;
greet)
	if [ $# -gt 1 ]; then
		echo "itos: greet takes no argument` + s.takes + ` ($2) (itos --help)" >&2
		exit ` + itoa(refusal) + `
	fi
	echo ` + or(s.greeting, "hello") + `
	exit ` + itoa(s.greeted) + ` ;;
name) echo itos ;;
version) echo "itos 1.0.1-dev.3+gabcdef0" ;;
check)
	echo "FAIL itos.yaml: ` + or(s.config, "version 2 is not 1") + `" >&2
	exit 2 ;;
*)
	echo "itos: unknown command: $1` + s.takes + ` (itos --help)" >&2
	exit 2 ;;
esac
`
}

func TestSelftest(t *testing.T) {
	scratch.Gate(t)
	if runtime.GOOS == "windows" {
		t.Skip("the binaries under test are shell scripts")
	}
	rel := scratch.NewRelease(t)
	tmp := rel.Tmp
	repo := filepath.Join(tmp, "repo")
	check := scratch.Build(t, "previous-release")
	run := func(dir string, args ...string) scratch.Result { return scratch.Run(dir, rel.Env, "", check, args...) }

	const moreHelp = "usage: itos greet|name|version|wave"
	const takes = "; the commands are greet, name and version"
	binaries := map[string]says{
		"good":     {},
		"wordy":    {help: moreHelp, takes: takes},
		"broken":   {greeting: "bye", greeted: 1, help: moreHelp},
		"reworded": {takes: takes, refusal: 1, config: "version should be 1, not 2"},
		"adding": {
			report: `1\n3\n2\n`,
			state:  `{"count":2,"since":1,"fix":"none needed","state":{"ready":true,"items":["a","b"],"note":"x","message":"ready, all of it"}}`,
			policy: `# Reworded, as people read it.\nchecks:\n  - a\n  - b\nname: scratch\nsince: 1\n`,
			notes:  `# Notes\n<!-- itos wrote this;\n  edit it freely. -->\n\nready <!-- until it is not -->\n`,
		},
	}
	// Each takes something away: a key, a value, an exit code.
	taking := []struct {
		what, kase string
		says       says
	}{
		{"a key removed", "says its state as JSON",
			says{state: `{"fix":"nothing to fix","state":{"ready":true,"items":["a","b"],"message":"all ready"}}`}},
		{"a value changed", "says its state as JSON", says{state: strings.Replace(state, "true", "false", 1)}},
		{"an exit code changed", "reports three lines", says{reported: 1}},
		{"a written YAML file's value changed", "writes its policy and its notes", says{policy: strings.Replace(policy, "[a, b]", "[a, c]", 1)}},
		{"a written YAML file's key removed", "writes its policy and its notes", says{policy: strings.Replace(policy, `name: scratch\n`, "", 1)}},
		{"a written Markdown file's text changed", "writes its policy and its notes", says{notes: strings.Replace(notes, "ready", "not ready", 1)}},
	}
	for i, tk := range taking {
		binaries["taking-"+string(rune('0'+i))] = tk.says
	}
	bin := map[string]string{}
	for name, s := range binaries {
		bin[name] = filepath.Join(tmp, name)
		if err := os.WriteFile(bin[name], []byte(script(s)), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	const scenario = "scenario @ID-GREET-01 (It greets the world)"
	const corpusCase = "conformance case greet.yaml: " + longCase

	// The scratch history: a fix naming the scenario and a breaking change before the release,
	// the release's tag, then a feat that says nothing; branches add a commit that does.
	rel.Git(tmp, "init", "-q", "-b", "main", repo)
	scratch.WriteFiles(t, repo, release)
	rel.Git(repo, "add", "-A")
	rel.Commit(repo, "build: the release's tree")
	rel.Commit(repo, "fix: an old fix\n\nWhy.\n\nChanges: @ID-GREET-01\nChanges: greet.yaml: greets the world")
	rel.Commit(repo, "feat!: an old break, before the release")
	rel.Git(repo, "tag", "v1.0.0")
	released := rel.Git(repo, "rev-parse", "HEAD")
	rel.Commit(repo, "feat: greet otherwise\n\nNo word on the old scenario.\n\nTask: T-1")
	feat := rel.Git(repo, "rev-parse", "HEAD")
	branch := func(name, message string) {
		rel.Git(repo, "checkout", "-q", "-b", name, feat)
		rel.Commit(repo, message)
	}
	branch("fixed", "fix: greet otherwise\n\nThe old greeting was the bug.\n\nChanges: @ID-GREET-01\nChanges: greet.yaml: "+longCase)
	// The long case by a prefix of its name that no other case starts with, and by one the case
	// asked its name starts with too.
	branch("prefixed", "fix: greet otherwise\n\nThe old greeting was the bug.\n\nChanges: @ID-GREET-01\nChanges: greet.yaml: greets the world with hello")
	branch("ambiguous", "fix: greet otherwise\n\nThe old greeting was the bug.\n\nChanges: @ID-GREET-01\nChanges: greet.yaml: greets")
	branch("fixed-scenario", "fix: greet otherwise\n\nThe old greeting was the bug.\n\nChanges: @ID-GREET-01")
	branch("featured", "feat: greet otherwise\n\nA new greeting.\n\nChanges: @ID-GREET-01\nChanges: greet.yaml: greets the world")
	branch("footer", "feat: greet otherwise\n\nBody.\n\nBREAKING-CHANGE: itos greet says bye")
	branch("bang", "fix!: greet otherwise")
	// Breaking changes that name what they change: a feat, so that only being breaking can
	// excuse it.
	branch("footer-named", "feat: greet otherwise\n\nBody.\n\nBREAKING-CHANGE: itos greet says bye\nChanges: @ID-GREET-01\nChanges: greet.yaml: greets the world")
	branch("bang-named", "feat!: greet otherwise\n\nBody.\n\nChanges: @ID-GREET-01\nChanges: greet.yaml: greets the world")
	branch("footer-scenario", "feat: greet otherwise\n\nBody.\n\nBREAKING-CHANGE: itos greet says bye\nChanges: @ID-GREET-01")
	branch("unknown", "fix: something else\n\nBody.\n\nChanges: @ID-GREET-09, @ID-GREET-02\nChanges: greet.yaml: waves\nChanges: whatever")
	rel.Git(repo, "checkout", "-q", "-b", "docs", released)
	rel.Commit(repo, "docs: say something")
	on := func(name string) { rel.Git(repo, "checkout", "-q", name) }

	has := func(out string, all ...string) bool {
		for _, s := range all {
			if !strings.Contains(out, s) {
				return false
			}
		}
		return true
	}
	lacks := func(out string, all ...string) bool {
		for _, s := range all {
			if strings.Contains(out, s) {
				return false
			}
		}
		return true
	}

	const leftOut = "the plain stdout and stderr of 10 cases and 2 keys named message or fix"
	t.Run("the release's own binary passes", func(t *testing.T) {
		on("main")
		r := run(repo, "-bin", bin["good"])
		if r.Code != 0 || !has(r.Output(), "v1.0.0's scenarios: all 2 pass", "12/12 conformance cases pass",
			"v1.0.0's conformance corpus: its 12 cases judged by itos's machine output alone", leftOut, "docs/decisions/0035-") {
			t.Errorf("the release's own binary should pass every scenario and case, the corpus judged by machine output alone and what it leaves out counted, said, exited %d:\n%s", r.Code, r.Output())
		}
	})

	// One whose help and usage errors' words differ, and nothing else, passes with no commit
	// that says why: their words are for people.
	t.Run("a binary whose help and usage errors' words alone differ passes", func(t *testing.T) {
		on("main")
		r := run(repo, "-bin", bin["wordy"])
		if r.Code != 0 || !has(r.Output(), leftOut, "12/12 conformance cases pass") || !lacks(r.Output(), "help.yaml: itos --help", "fails") {
			t.Errorf("a binary whose help and usage errors' words alone differ should pass, exited %d:\n%s", r.Code, r.Output())
		}
	})

	// One that rewords every refusal and changes one usage error's exit code is refused, naming
	// that case alone: the config error and the other usage error differ only in words.
	t.Run("a usage error whose exit code changed is refused, and no case whose words alone differ", func(t *testing.T) {
		on("main")
		r := run(repo, "-bin", bin["reworded"])
		if r.Code != 1 || !has(r.Output(), "v1.0.0's conformance case refuse.yaml: greet takes no argument fails", "exit: expected 2, got 1", "11/12 conformance cases pass") ||
			!lacks(r.Output(), "a config that is not version 1", "an unknown command is a usage error") {
			t.Errorf("a usage error whose exit code changed should be refused, and no case whose words alone differ named, exited %d:\n%s", r.Code, r.Output())
		}
	})

	// One that adds keys to the JSON at the top and deeper, rewords its message and fix,
	// rewrites the report, and rewrites the comments of the files it writes, reorders the YAML's
	// keys and adds one, passes: the old json is judged additively, less message and fix, plain
	// output not at all, and a written file by its data, its comment lines for people
	// (decision 40).
	t.Run("a binary that adds to an old case's output and rewords it passes", func(t *testing.T) {
		on("main")
		r := run(repo, "-bin", bin["adding"])
		if r.Code != 0 || !has(r.Output(), "12/12 conformance cases pass") || !lacks(r.Output(), "fails") {
			t.Errorf("a binary that adds keys to an old case's JSON, rewords its message and fix, rewrites its stdout and changes only the comments and the key order of the files it writes, adding a key, should pass, exited %d:\n%s", r.Code, r.Output())
		}
	})
	// One that takes a key away, changes a value or changes an exit code, in the JSON or in a
	// file it writes, or changes a written Markdown file's text, is refused, naming that case
	// alone.
	for i, tk := range taking {
		t.Run("a binary with "+tk.what+" in an old case's output is refused", func(t *testing.T) {
			on("main")
			other := ""
			for _, o := range taking {
				if o.kase != tk.kase {
					other = o.kase
					break
				}
			}
			r := run(repo, "-bin", bin["taking-"+string(rune('0'+i))])
			if r.Code != 1 || !has(r.Output(), "v1.0.0's conformance case report.yaml: "+tk.kase, "11/12 conformance cases pass") || !lacks(r.Output(), "report.yaml: "+other) {
				t.Errorf("a binary with %s in an old case's output should be refused, naming that case alone, exited %d:\n%s", tk.what, r.Code, r.Output())
			}
		})
	}

	// The broken binary, with no commit since the tag that says why: refused, naming the
	// scenario, the case and the remedy; the fix and the ! before the tag do not count.
	t.Run("a broken scenario and case with no commit saying why are refused, naming both and the remedy", func(t *testing.T) {
		on("main")
		r := run(repo, "-bin", bin["broken"])
		if r.Code != 1 || !has(r.Output(), "v1.0.0's "+scenario+" fails against this tree's itos", "v1.0.0's "+corpusCase+" fails against this tree's itos", "Changes:", "BREAKING-CHANGE:") || !lacks(r.Output(), "accepted") {
			t.Errorf("a broken old scenario and case with no commit saying why should be refused (exit 1), naming both and the remedy, exited %d:\n%s", r.Code, r.Output())
		}
		if !has(r.Output(), "1 of 2 fail", "11/12 conformance cases pass") || !lacks(r.Output(), "says its name", "help.yaml: itos --help", "help greet") {
			t.Errorf("only the scenario and the case the binary breaks should be named, no help case:\n%s", r.Output())
		}
	})

	// A fix since the tag naming both in Changes: passes, saying so.
	t.Run("a fix naming the scenario and the case in Changes passes", func(t *testing.T) {
		on("fixed")
		r := run(repo, "-bin", bin["broken"])
		if r.Code != 0 || !has(r.Output(), scenario+" fails, accepted: the fix", corpusCase+" fails, accepted: the fix") {
			t.Errorf("a fix naming the scenario and the case in Changes: should pass, exited %d:\n%s", r.Code, r.Output())
		}
	})
	// One naming the case by a prefix of its name no other case starts with passes; one naming
	// it by a prefix two cases start with names neither, leaving the case refused and warning that
	// the entry is ambiguous.
	t.Run("a fix naming the case by a prefix no other case starts with passes", func(t *testing.T) {
		on("prefixed")
		r := run(repo, "-bin", bin["broken"])
		if r.Code != 0 || !has(r.Output(), corpusCase+" fails, accepted: the fix") || !lacks(r.Output(), "warning") {
			t.Errorf("a fix naming the long case by a prefix of its name no other case starts with should pass, exited %d:\n%s", r.Code, r.Output())
		}
	})
	t.Run("a fix naming the case by a prefix two cases start with leaves it refused, warning", func(t *testing.T) {
		on("ambiguous")
		r := run(repo, "-bin", bin["broken"])
		short := rel.Git(repo, "rev-parse", "HEAD")[:7]
		if r.Code != 1 || !has(r.Output(), scenario+" fails, accepted: the fix", corpusCase+" fails against this tree's itos",
			`warning: `+short+` "fix: greet otherwise" says "Changes: greet.yaml: greets"`,
			`greet.yaml has 2 cases whose names start "greets"`, "it is ambiguous, so it names none of them") {
			t.Errorf("a fix naming the case by a prefix two cases start with should leave it refused, warning that the entry is ambiguous, exited %d:\n%s", r.Code, r.Output())
		}
	})
	// One naming the scenario alone leaves the case refused, as does a breaking change naming it
	// alone (T-106), v6.0.0's shape.
	for _, c := range []struct{ branch, by, name string }{
		{"fixed-scenario", "the fix", "a fix naming only the scenario leaves the case refused"},
		{"footer-scenario", "the breaking change", "a breaking change naming only the scenario leaves the case refused"},
	} {
		t.Run(c.name, func(t *testing.T) {
			on(c.branch)
			r := run(repo, "-bin", bin["broken"])
			if r.Code != 1 || !has(r.Output(), scenario+" fails, accepted: "+c.by, corpusCase+" fails against this tree's itos") {
				t.Errorf("%s naming only the scenario should leave the case refused, exited %d:\n%s", c.by, r.Code, r.Output())
			}
		})
	}

	// A feat naming both is refused, naming each: a feat's Changes: excuses one only on a fix or
	// a breaking change (T-106).
	const onlyOn = "which excuses one only on a fix or a breaking change"
	t.Run("a feat naming the scenario and the case in Changes is refused", func(t *testing.T) {
		on("featured")
		r := run(repo, "-bin", bin["broken"])
		if r.Code != 1 || !has(r.Output(), scenario+" fails against this tree's itos, and only the feat", corpusCase+" fails against this tree's itos, and only the feat", onlyOn) || !lacks(r.Output(), "accepted") {
			t.Errorf("a feat naming the scenario and the case in Changes: should refuse both, exited %d:\n%s", r.Code, r.Output())
		}
	})

	// A breaking change since the tag, by a footer or by a ! in a header, excuses only what its
	// Changes: names (T-106). Naming nothing, it is refused, naming the scenario, the case and the
	// remedy.
	for _, c := range []struct{ branch, name string }{
		{"footer", "a breaking change by its footer naming nothing is refused"},
		{"bang", "a breaking change by its ! naming nothing is refused"},
	} {
		t.Run(c.name, func(t *testing.T) {
			on(c.branch)
			r := run(repo, "-bin", bin["broken"])
			if r.Code != 1 || !has(r.Output(), "v1.0.0's "+scenario+" fails against this tree's itos", "v1.0.0's "+corpusCase+" fails against this tree's itos",
				"being breaking excuses only what its Changes: names") || !lacks(r.Output(), "accepted", onlyOn) {
				t.Errorf("a breaking change (%s) whose Changes: names nothing should refuse the broken scenario and case, exited %d:\n%s", c.branch, r.Code, r.Output())
			}
		})
	}
	// Naming both in Changes:, it passes, saying the breaking change names each.
	for _, c := range []struct{ branch, name string }{
		{"footer-named", "a breaking change by its footer naming both passes"},
		{"bang-named", "a breaking change by its ! naming both passes"},
	} {
		t.Run(c.name, func(t *testing.T) {
			on(c.branch)
			r := run(repo, "-bin", bin["broken"])
			named := `accepted: the breaking change ` + rel.Git(repo, "rev-parse", "HEAD")[:7] + ` "`
			if r.Code != 0 || !has(r.Output(), scenario+" fails, "+named, corpusCase+" fails, "+named) {
				t.Errorf("a breaking change (%s) naming the scenario and the case in Changes: should accept both, exited %d:\n%s", c.branch, r.Code, r.Output())
			}
		})
	}

	// Changes: entries that name nothing of the release are warnings.
	t.Run("a Changes entry naming nothing of the release is a warning", func(t *testing.T) {
		on("unknown")
		r := run(repo, "-bin", bin["good"])
		if r.Code != 0 || !has(r.Output(), "@ID-GREET-09 is no scenario of v1.0.0", `has no case "waves"`, "neither scenario IDs") || !lacks(r.Output(), "@ID-GREET-02 is no") {
			t.Errorf("a Changes: entry naming nothing of the release should be a warning and pass, exited %d:\n%s", r.Code, r.Output())
		}
	})

	// -range-from: a range with no feat or fix checks nothing; one with a feat checks.
	t.Run("range-from checks nothing for a range of a docs commit", func(t *testing.T) {
		on("docs")
		r := run(repo, "-range-from", released, "-bin", bin["broken"])
		if r.Code != 0 || !strings.Contains(r.Output(), "no feat or fix") {
			t.Errorf("a range of a docs commit should check nothing, exited %d:\n%s", r.Code, r.Output())
		}
	})
	t.Run("range-from checks a range with a feat", func(t *testing.T) {
		on("main")
		r := run(repo, "-range-from", released, "-bin", bin["broken"])
		if r.Code != 1 || !strings.Contains(r.Output(), scenario) {
			t.Errorf("a range with a feat should check, and refuse the broken scenario, exited %d:\n%s", r.Code, r.Output())
		}
	})

	t.Run("no release tag passes, saying so", func(t *testing.T) {
		r := run(rel.Untagged(), "-bin", bin["broken"])
		if r.Code != 0 || !strings.Contains(r.Output(), "no vX.Y.Z tag") {
			t.Errorf("a repository with no release tag should pass, saying so, exited %d:\n%s", r.Code, r.Output())
		}
	})

	t.Run("a shallow clone stops it", func(t *testing.T) {
		r := run(rel.Shallow(repo), "-bin", bin["broken"])
		if r.Code != 2 || !strings.Contains(r.Output(), "shallow") {
			t.Errorf("a shallow clone should stop the check (exit 2), exited %d:\n%s", r.Code, r.Output())
		}
	})

	// A tag that cannot be checked out (its tree's objects missing) stops it.
	t.Run("a tag that cannot be checked out stops it", func(t *testing.T) {
		corrupt := filepath.Join(tmp, "corrupt")
		rel.Git(tmp, "init", "-q", "-b", "main", corrupt)
		scratch.WriteFiles(t, corrupt, map[string]string{"features/lost.feature": "Feature: only in the release\n"})
		rel.Git(corrupt, "add", "-A")
		rel.Commit(corrupt, "build: the release's tree")
		rel.Git(corrupt, "tag", "v1.0.0")
		blob := rel.Git(corrupt, "rev-parse", "v1.0.0:features/lost.feature")
		rel.Git(corrupt, "rm", "-q", "-r", "features")
		rel.Commit(corrupt, "feat: drop it")
		if err := os.Remove(filepath.Join(corrupt, ".git", "objects", blob[:2], blob[2:])); err != nil {
			t.Fatal(err)
		}
		r := run(corrupt, "-bin", bin["good"])
		if r.Code != 2 || !strings.Contains(r.Output(), "cannot check out v1.0.0") {
			t.Errorf("a tag that cannot be checked out should stop the check (exit 2), exited %d:\n%s", r.Code, r.Output())
		}
	})

	t.Run("the check leaves no scratch worktree behind", func(t *testing.T) {
		if list := rel.Git(repo, "worktree", "list"); len(strings.Split(list, "\n")) != 1 {
			t.Errorf("the check should leave no scratch worktree behind:\n%s", list)
		}
	})
}
