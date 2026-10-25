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
//   - prepush_refs_test.go: the pre-push hook's commands record which of them
//     ran and the base per_base was given, read against the full SHAs of
//     the clone's commits by subject; another branch of the remote is made
//     in the clone the remote gains commits from, and an origin with no
//     branch is a new bare repository, the clone's remote-tracking refs
//     deleted.
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
//   - watch_failures_test.go: a failed job of the watched run is listed with
//     its id and the steps the jobs API lists, and its log is answered as
//     GitHub answers one: a redirect to a download address serving the text,
//     each line stamped with its time and ended by LF or CRLF, or 410. The
//     steps read the jobs of itos's --json back by name.
//   - shim_test.go: the link is to the binary the itos under test runs,
//     never tools/bin/itos; it lives in the support folder's shim/, put first
//     on every command's PATH (pathFirst), so no step leaves a shim on the
//     PATH of anything else. The repository with no itos config is plain/ in
//     the support folder.
//   - guard_test.go: the steps write Claude Code's PreToolUse input as its
//     documentation shows it and run itos guard claude-code with it on stdin
//     (runWith); a deny is read as Claude Code reads it, stdout's JSON.
//   - ci_proof_summary_test.go: the scratch root's own files, written as a
//     scenario says: a provider script that appends its arguments to
//     proof-runs and prints proof.stdout, a recording script, the config
//     replaced whole, and tags on the fixture's commits. itos is run with
//     --config itos.yaml; ci run's JSON is read as JSON, its logs by whole
//     lines, and what ran by the files the scripts append to.
//   - ci_proof_report_test.go: the same fixture; lines of ci plan's and ci run's
//     streams read whole and by their place, a tag's full SHA written into an
//     expected line, commits.since or the proof section edited in the scratch
//     config, and a commit's tree deleted from the objects (a loose object, or
//     the step fails as a setup error).
//   - cover_test.go: with GOCOVERDIR or ITOS_CC_TEST_COVERDIR set, the itos
//     under test is a -cover build of the tree, made once a run (T-121), and
//     with ITOS_CC_TEST_COVERDIR every command a scenario starts writes its
//     coverage to a folder named after the scenario's ID, for itos-cc.
//   - stamp_test.go: a scenario that pins the itos under test runs this
//     tree's itos built stamped with the pinned version, once a run (a -cover
//     build when the run measures coverage), since the launcher runs the
//     binary called only when the pin is its own version; the release
//     server's fake of that version exits 97, so a fake run in its place
//     fails the scenario.
//   - timings_test.go: -timings, which judges nothing: each scenario's time,
//     from before its set-up to after its clean-up, and each kind of step's
//     (its text, quoted strings blanked), written as a report once the run
//     ends. The platform jobs print it (T-115).
//   - repo_fixture_test.go: a scenario's repository starts as a copy of one
//     git init's git folder, with itos's hooks written into its config, and
//     reads HEAD from the git folder's files, so none of them starts a git
//     (T-308).
//   - shard_test.go: -shard=i/n runs the scenarios whose hash is i of n, the
//     others skipped before their set-up, so a platform splits the suite
//     across jobs (T-308).
//
// The header lint, where a scenario needs one, is itos's built-in one (use:
// builtin), which needs nothing installed and holds no footer rule, so a
// scenario's footer rules are itos's own. The conformance corpus's runner
// (tools/bin/conformance) cleans its environment the same way.
package features

import (
	"bufio"
	"context"
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
	inProcess := os.Getenv("ITOS_FEATURE_DRIVER") == "in-process"
	if inProcess && *concurrency != 1 {
		t.Logf("in-process driver requires serial scenarios; overriding -concurrency=%d with 1", *concurrency)
		*concurrency = 1
	}
	filter, err := tagFilter(*scenarios, ".")
	if err != nil {
		t.Fatal(err)
	}
	if *shardFlag != "" {
		if shardIndex, shardCount, err = parseShard(*shardFlag); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(removeCallerPath)
	t.Cleanup(removeRepoTemplate)
	removeCover, err := buildCover()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(removeCover)
	t.Cleanup(removeStamped)
	startTimings()
	initializer := initializeScenario
	var selected, ran int
	if inProcess {
		initializer = func(sc *godog.ScenarioContext) {
			initializeScenario(sc)
			sc.Before(func(ctx context.Context, scenario *godog.Scenario) (context.Context, error) {
				selected++
				id := inProcessScenarioID(scenario)
				if reason, deferred := inProcessDeferred[id]; deferred {
					t.Logf("in-process driver skipped @%s: %s", id, reason)
					return ctx, godog.ErrSkip
				}
				ran++
				return ctx, nil
			})
		}
	}
	suite := godog.TestSuite{
		Name:                "itos",
		ScenarioInitializer: initializer,
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
	if inProcess && (selected == 0 || ran == 0) {
		t.Fatalf("in-process driver ran no scenarios: all %d selected scenarios were skipped", selected)
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
