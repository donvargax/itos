// The features: every *.feature file in this folder, run by godog through go
// test, each scenario a subtest of TestFeatures. features/README.md holds the
// rules: the black-box boundary, the ID scheme, the tags, the smoke set, the
// moving rule.
//
//	go test ./features -count=1                        every live scenario
//	go test ./features -count=1 -scenarios=<regexp>    the live scenarios with a
//	                                                   tag the expression matches
//	go test ./features -count=1 -timings=<file>        any run, writing where it
//	                                                   spent its time to the file
//	go test ./features -count=1 -concurrency=1         one scenario at a time
//	GOCOVERDIR=<dir> go test ./features -count=1       any run, the itos under
//	                                                   test built with -cover,
//	                                                   its coverage in <dir>
//	ITOS_CC_TEST_COVERDIR=<dir> go test ./features     the same, each scenario's
//	                                                   in <dir>/<its ID>
//
// The scenarios run -concurrency at a time, as many as the machine has CPUs
// unless it says otherwise (T-115). A scenario's time goes to starting
// processes, git's and itos's, one after another, which windows makes slow
// (the three gits that commit a scenario's ledger take about 200 ms there,
// 4 ms on linux), so running several at once is what keeps the windows job
// well inside go test's ten minutes. Each scenario has its own world,
// folders and servers; what the steps share across scenarios (the PATH
// without claude, script-exe, the timings) is guarded, and a step never
// changes the process's environment or working directory. With
// -concurrency=1 they run in one goroutine, each failure's stack as it was.
//
// godog's own tag filter takes exact tags joined by commas; itos's run
// templates join the IDs they select with |, as a regular expression
// (itos.yaml's tests.scenario.run: ids_pattern ^@(?:{ids})$), so -scenarios
// takes that expression: the harness reads every tag in the feature files
// and hands godog the matching ones as its own filter, each &&~@wip. An
// expression that matches no tag fails the run. A scenario tagged @wip never
// runs.
//
// # The black box
//
// The steps (steps_test.go) treat itos as a black box: they never read its
// code. Each scenario builds a scratch git repository in a temporary folder,
// writes its itos.yaml and ledger, runs the binary ITOS_BIN names
// (tools/bin/itos by default, the Go binary built from the tree, relative to
// the module's root) in it, and asserts the exit code, the output and the
// files it leaves. Every scenario's repository, and each clone itos runs in,
// declares itos's two hooks in its git config (declareHooks), each running a
// stand-in itos that passes whatever it is given, so itos commits and pushes
// there and no hook's check runs unless the scenario installs one.
//
// A command runs in a clean environment: the caller's, less GIT_*, ITOS_*,
// GITHUB_*, GH_* and CI, with no global or system git config and a fixed
// identity, so a run inside a git hook or on a runner sees what it sees
// locally. Its PATH is the caller's with no claude on it (callerPath): each
// folder holding one is replaced, once a run, by a folder of links to the
// rest of it, so no scenario reaches the Claude Code of the machine it runs
// on; a scenario's own claude goes first. ITOS_CACHE is a folder of the
// scenario's and ITOS_RELEASES the scenario's release server, or, with none,
// an address nothing answers on beside ITOS_NO_UPDATE=1, so no scenario
// reaches the network or a real cache. binOnPath's folder, the support
// folder's bin put first, holds the stand-ins a scenario writes (a
// signed-out gh, a fake claude, a script running the itos under test).
//
// # The harness, file by file
//
//   - release_test.go, for the launcher's scenarios: an httptest server,
//     logging every path asked of it, offering fake releases laid out as real
//     ones are (/download/v<version>/<asset>, and the last version under
//     /latest/download/<asset>): the archive for the running platform, whose
//     itos is a shell script that records its version, the ITOS_VERSION it
//     ran with and its arguments and exits with the code the scenario chose,
//     and a real checksums.txt. Steps replace a release's checksums.txt or
//     archive after the config pins it, or make the server unreachable. The
//     launcher's daily state is never read by a step: a scenario tells "once
//     a day" by what the server was asked.
//   - program_test.go: a scenario's fake programs are shell scripts, which
//     Linux and macOS run by their #! line. Windows runs no script, so there
//     writeProgram writes <name>.exe instead: script-exe
//     (features/testdata/script-exe, built once a run) with the script after a
//     marker line, which writes the script with the program's arguments quoted
//     into it, runs that with Git for Windows' sh (the one on the PATH, else
//     the one beside git.exe), and hands on its streams, environment and exit
//     code.
//   - extensions_test.go: each extension is a shell script itos-<name> in a
//     folder of the scenario's put first on the PATH, which records its
//     arguments, its folder and the ITOS_* variables it saw, then exits with
//     the code the scenario chose or calls back through $ITOS_BIN.
//   - stealth_test.go: the world's dataDir is where the config, the ledger,
//     the registry and the people go, the root or .git/itos. A linked worktree
//     is added after the scratch repository moves into a folder of its own,
//     so ../wt is the scenario's alone. A hook declared in the git config is
//     read back as git reads it (git config --get-regexp, then git hook
//     list). The step that installs itos's hooks puts a script named itos on
//     the PATH that runs the itos under test (a link would not do:
//     tools/bin/itos finds its checkout from its own path), and runs it once,
//     so a hook that cannot start does not pass for one that refuses. A
//     remote is a bare repository in the support folder.
//   - init_test.go: the scratch repository starts with nothing of itos, or
//     with its .git removed. "No file changed since the last run" records
//     every file, the git folder's included, as its mode and text before each
//     run (markRun) and compares after, so a rerun that wrote anything fails.
//     Claude Code is a fake claude that records each run's arguments and
//     answers plugin list --json.
//   - commit_test.go: the commit-msg hook is a shim in git's own hooks folder
//     that runs the itos under test, so a commit made through itos meets the
//     hook any commit meets. A footer of HEAD is read back as git reads its
//     trailers (%(trailers:only,unfold)), so a footer that landed in the body
//     does not pass; under a stealth config from HEAD's note. An amend has to
//     make a new HEAD, so a refused one fails its step rather than leaving
//     the old commit's note to pass.
//   - push_test.go: the remote is a bare clone in the support folder
//     (origin.git) and itos runs in a clone of it; the remote gains commits
//     from another clone, pushed with --no-verify, each writing its file as
//     one line naming the commit, so two touching the same file conflict.
//     The remote's branch is read in the bare repository itself, so what was
//     pushed is what the remote has. Two takes of one item edit the item's
//     one registry line, so they conflict.
//   - range_test.go: the fake GitHub, an httptest server itos is pointed at
//     through GITHUB_API_URL, given GITHUB_REPOSITORY and GITHUB_TOKEN as
//     Actions gives them. It holds runs of several workflows, each on a
//     branch, and answers a workflow's runs (head_sha and branch narrowing
//     them, newest first, or a stale list a step sets) and a run's jobs; it
//     can refuse the token.
//   - watch_test.go: push's clone, its ci.watch the github provider, committed
//     and pushed with the hooks left out whenever a step changes it. The fake
//     GitHub answers the watched commit's nth request with the nth look, so
//     "one poll later" is the second look and "never asked" no request. "No
//     gh on the PATH" hides gh as claude always is; "no GitHub token" takes
//     the fake's token out.
//   - shim_test.go: the link is to the binary the itos under test runs,
//     never tools/bin/itos; it lives in the support folder's shim/, put first
//     on every command's PATH (pathFirst), so no step leaves a shim on the
//     PATH of anything else. The repository with no itos config is plain/ in
//     the support folder.
//   - guard_test.go: the steps write Claude Code's PreToolUse input as its
//     documentation shows it and run itos guard claude-code with it on stdin
//     (runWith); a deny is read as Claude Code reads it, stdout's JSON.
//   - cover_test.go: with GOCOVERDIR or ITOS_CC_TEST_COVERDIR set, the itos
//     under test is a -cover build of the tree, made once a run (T-121), and
//     with ITOS_CC_TEST_COVERDIR every command a scenario starts writes its
//     coverage to a folder named after the scenario's ID, for itos-cc.
//   - timings_test.go: -timings, which judges nothing: each scenario's time,
//     from before its set-up to after its clean-up, and each kind of step's
//     (its text, quoted strings blanked), written as a report once the run
//     ends. The platform jobs print it (T-115).
//
// The header lint, where a scenario needs one, is itos's built-in one (use:
// builtin), which needs nothing installed and holds no footer rule, so a
// scenario's footer rules are itos's own. The conformance corpus's runner
// (tools/itos/conformance/run.ts) cleans its environment the same way.
package features

import (
	"bufio"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/cucumber/godog"
)

var (
	scenarios   = flag.String("scenarios", "", "run only the live scenarios with a tag this regular expression matches")
	concurrency = flag.Int("concurrency", runtime.NumCPU(), "run this many scenarios at a time")
)

func TestFeatures(t *testing.T) {
	filter, err := tagFilter(*scenarios, ".")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(removeCallerPath)
	removeCover, err := buildCover()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(removeCover)
	startTimings()
	suite := godog.TestSuite{
		Name:                "itos",
		ScenarioInitializer: initializeScenario,
		Options: &godog.Options{
			Format:      "pretty",
			Paths:       []string{"."},
			Tags:        filter,
			Strict:      true,
			Concurrency: *concurrency,
			TestingT:    t,
		},
	}
	failed := suite.Run() != 0
	if err := writeTimings(); err != nil {
		t.Error(err)
	}
	if failed {
		t.Fatal("a scenario failed")
	}
}

// godog's tag filter for a selection: every live scenario without one, else
// the live scenarios carrying a tag the expression matches. An expression
// that matches no tag is an error, so a selection never runs nothing.
func tagFilter(selection, dir string) (string, error) {
	if selection == "" {
		return "~@wip", nil
	}
	pattern, err := regexp.Compile(selection)
	if err != nil {
		return "", err
	}
	tags, err := featureTags(dir)
	if err != nil {
		return "", err
	}
	var each []string
	for _, tag := range tags {
		if pattern.MatchString(tag) {
			each = append(each, tag+"&&~@wip")
		}
	}
	if len(each) == 0 {
		return "", &noMatch{selection}
	}
	return strings.Join(each, ","), nil
}

type noMatch struct{ selection string }

func (e *noMatch) Error() string {
	return "no scenario has a tag that " + e.selection + " matches"
}

// Every tag written in the feature files under dir, once each, sorted.
func featureTags(dir string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.feature"))
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return nil, err
		}
		lines := bufio.NewScanner(f)
		for lines.Scan() {
			line := strings.TrimSpace(lines.Text())
			if !strings.HasPrefix(line, "@") {
				continue
			}
			for _, word := range strings.Fields(line) {
				if strings.HasPrefix(word, "#") {
					break
				}
				if strings.HasPrefix(word, "@") {
					seen[word] = true
				}
			}
		}
		f.Close()
		if err := lines.Err(); err != nil {
			return nil, err
		}
	}
	tags := make([]string, 0, len(seen))
	for tag := range seen {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags, nil
}
