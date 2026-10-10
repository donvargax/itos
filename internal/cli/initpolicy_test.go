package cli

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v7/internal/config"
	"github.com/donvargax/itos/v7/internal/git"
	"github.com/donvargax/itos/v7/internal/release"
)

// The flags that answer init's three offers, so no test asks or installs.
var noOffers = []string{"--plugin", "no", "--no-git-shim", "--no-agent-rules"}

// A scratch folder as the current one, away from the user's git config and
// any release server, holding the files; a repository with one commit when
// repo is "commit", one with none when it is "unborn", and none when "".
func initScratch(t *testing.T, repo string, files map[string]string) string {
	t.Helper()
	for _, name := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_COMMON_DIR"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("ITOS_CONFIG", "")
	os.Unsetenv("ITOS_CONFIG")
	t.Setenv(release.Env, "http://127.0.0.1:1")
	dir := t.TempDir()
	t.Chdir(dir)
	for p, text := range files {
		putFile(t, p, text)
	}
	sh := func(args ...string) {
		t.Helper()
		if out, err := exec.Command(git.Bin(), args...).CombinedOutput(); err != nil {
			t.Fatalf("git %s: %s", strings.Join(args, " "), out)
		}
	}
	if repo != "" {
		sh("init", "-q")
	}
	if repo == "commit" {
		putFile(t, "README.md", "# Scratch\n")
		sh("add", "README.md")
		sh("-c", "user.name=itos", "-c", "user.email=itos@example.com", "commit", "-q", "-m", "docs: start")
	}
	return dir
}

// putFile writes a file, its folder made first.
func putFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, text)
}

// initRun runs init with the arguments and the offers declined.
func initRun(t *testing.T, json bool, args ...string) (int, string, string, error) {
	t.Helper()
	var stdout, stderr strings.Builder
	code, err := initCommand(append(args, noOffers...), Out{JSON: json, Stdout: &stdout, Stderr: &stderr})
	return code, stdout.String(), stderr.String(), err
}

// readOnly makes a folder one nobody may write in, for the test alone; it
// skips where that does not hold (Windows folders, root).
func readOnly(t *testing.T, dir string) {
	t.Helper()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("a read-only folder needs a unix user other than root")
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

// The arguments init refuses before it looks at any folder.
func TestInitRefusesItsArguments(t *testing.T) {
	for _, args := range [][]string{{"--plugin", "everywhere"}, {"--bogus"}, {"--no-git-shim", "--git-shim-dir", "x"}} {
		if _, err := initCommand(args, Out{Stdout: &strings.Builder{}, Stderr: &strings.Builder{}}); ExitCode(err) != ExitUsage {
			t.Errorf("init %v: %v", args, err)
		}
	}
}

// In a git folder, with ITOS_CONFIG naming a file that is not there, and
// under a stealth config with --plugin project, init and init --policy are
// usage errors, and write nothing.
func TestInitUsageErrorsWhereItRuns(t *testing.T) {
	dir := initScratch(t, "commit", map[string]string{"p.yaml": "version: 1\n"})
	policy := filepath.Join(dir, "p.yaml")
	t.Chdir(".git")
	for _, args := range [][]string{nil, {"--policy", policy}} {
		if _, _, _, err := initRun(t, false, args...); ExitCode(err) != ExitUsage || !strings.Contains(err.Error(), "git folder") {
			t.Errorf("in .git, init %v: %v", args, err)
		}
	}
	t.Chdir(dir)
	t.Setenv("ITOS_CONFIG", "missing.yaml")
	for _, args := range [][]string{nil, {"--policy", "p.yaml"}} {
		if _, _, _, err := initRun(t, false, args...); ExitCode(err) != ExitUsage || !strings.Contains(err.Error(), "missing.yaml") {
			t.Errorf("with ITOS_CONFIG, init %v: %v", args, err)
		}
	}
	t.Setenv("ITOS_CONFIG", "")
	os.Unsetenv("ITOS_CONFIG")
	putFile(t, filepath.Join(".git", "itos", "itos.yaml"), "version: 1\n")
	_, err := initCommand([]string{"--plugin", "project"}, Out{Stdout: &strings.Builder{}, Stderr: &strings.Builder{}})
	if ExitCode(err) != ExitUsage {
		t.Errorf("under a stealth config, --plugin project: %v", err)
	}
	if _, err := os.Stat("itos.yaml"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("itos.yaml was written: %v", err)
	}
}

// A git that cannot run git init fails init, and init --policy once the
// policy is judged, with git's output.
func TestInitWhereGitInitFails(t *testing.T) {
	initScratch(t, "", map[string]string{"p.yaml": "version: 1\n"})
	useFailingGit(t)
	for _, args := range [][]string{nil, {"--policy", "p.yaml"}} {
		if _, _, _, err := initRun(t, false, args...); err == nil || !strings.Contains(err.Error(), "git init: injected git failure") {
			t.Errorf("init %v: %v", args, err)
		}
	}
}

// A policy that cannot be read, or not edited in place, is refused with
// nothing written.
func TestPolicyInitRefusesAPolicyItCannotUse(t *testing.T) {
	initScratch(t, "commit", map[string]string{"flow.yaml": "{version: 1}\n", "empty.yaml": "version: 1\ncommits: {}\n"})
	for policy, want := range map[string]string{
		"missing.yaml": "missing.yaml: cannot be read",
		"flow.yaml":    "flow.yaml: init --policy cannot make the project's config of it",
		"empty.yaml":   "empty.yaml: init --policy cannot make the project's config of it: commits.since",
	} {
		_, _, _, err := initRun(t, false, "--policy", policy)
		if ExitCode(err) != ExitUsage || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v", policy, err)
		}
	}
	if _, err := os.Stat("itos.yaml"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("itos.yaml was written: %v", err)
	}
}

// A policy whose ledger cannot name group 1's file nor hold T-1, a stealth
// ledger outside the stealth folder and a smoke set of a kind with no root
// are refused before anything is written.
func TestPolicyInitRefusesDataItCannotMake(t *testing.T) {
	initScratch(t, "commit", map[string]string{
		"layout.yaml": "version: 1\nledger: { files: \"t/phase-{group}.yaml\", group: { pattern: '[a-z]+' } }\n" +
			"commits: { types: [feat] }\n",
		"stealth.yaml": "version: 1\nledger: { files: \"../../t/phase-{group}.yaml\" }\n",
		"noroot.yaml":  "version: 1\ntests: { scenario: { smoke: { file: smoke.yaml } } }\n",
	})
	for args, want := range map[string]string{
		"--policy layout.yaml":            "cannot name the file of the group 1",
		"--policy stealth.yaml --stealth": "ledger.files puts t/phase-1.yaml outside .git/itos",
		"--policy noroot.yaml":            "tests.scenario.root is missing",
	} {
		_, _, _, err := initRun(t, false, strings.Fields(args)...)
		if ExitCode(err) != ExitUsage || !strings.Contains(err.Error(), want) {
			t.Errorf("init %s: %v", args, err)
		}
	}
	_, _, _, err := initRun(t, false, "--policy", "layout.yaml")
	var c *config.Error
	if !errors.As(err, &c) || len(c.Problems) != 2 || !strings.Contains(c.Problems[1].Message, "commits.types does not list chore") {
		t.Errorf("layout.yaml's problems: %v", err)
	}
}

// A command adapter's list that fails, or prints what the protocol refuses,
// refuses init --policy with its error, exit 1, before anything is written,
// and under --json as its own object; one that lists the project's tests
// gives the smoke set, each ID after the kind's tag prefix.
func TestPolicyInitRunsACommandAdaptersList(t *testing.T) {
	policy := func(script string) string {
		return "version: 1\ntests: { unit: { adapter: { command: sh " + script + " }, tag_prefix: '', smoke: { file: smoke.yaml } } }\n"
	}
	initScratch(t, "commit", map[string]string{
		"fails.sh": "echo 'no test binary yet' >&2\nexit 3\n", "fails.yaml": policy("fails.sh"),
		"text.sh": "echo 'not json'\n", "text.yaml": policy("text.sh"),
		"lists.sh":   `printf '%s\n' '{"protocol":1,"tests":[{"id":"U-1","file":"a_test.go","live":true}],"files":["a_test.go"]}'` + "\n",
		"lists.yaml": policy("lists.sh"),
	})
	for p, want := range map[string]string{"fails.yaml": "exited 3\n  no test binary yet\n", "text.yaml": "did not print JSON"} {
		code, _, stderr, err := initRun(t, false, "--policy", p)
		if code != ExitPolicy || err != nil || !strings.Contains(stderr, "the list of tests.unit failed") || !strings.Contains(stderr, want) {
			t.Errorf("%s: exit %d, %v:\n%s", p, code, err, stderr)
		}
	}
	code, stdout, _, err := initRun(t, true, "--policy", "fails.yaml")
	var got struct {
		Action   string
		Problems []struct{ Rule, Message string }
	}
	if code != ExitPolicy || err != nil || json.Unmarshal([]byte(stdout), &got) != nil || got.Action != "refused" ||
		len(got.Problems) != 1 || got.Problems[0].Rule != "init-policy-list" {
		t.Errorf("--json: exit %d, %v:\n%s", code, err, stdout)
	}
	if _, err := os.Stat("itos.yaml"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("itos.yaml was written: %v", err)
	}
	if code, _, stderr, err := initRun(t, false, "--policy", "lists.yaml"); code != 0 || err != nil {
		t.Fatalf("lists.yaml: exit %d, %v:\n%s", code, err, stderr)
	}
	if text, _ := os.ReadFile("smoke.yaml"); !strings.Contains(string(text), `- id: "U-1"`) {
		t.Errorf("the smoke set:\n%s", text)
	}
}

// A smoke set the project has is kept as it is when the smoke check passes
// over it, a command kind's through its list, and named in what to commit;
// one the check refuses, one it cannot read and one whose list fails are
// refused with their problems, exit 1, before anything is written.
func TestPolicyInitKeepsTheProjectsSmokeSet(t *testing.T) {
	policy := func(script string) string {
		return "version: 1\ntests:\n  scenario: { root: specs, id: 'ID-[A-Z]+-\\d+', smoke: { file: specs/smoke.yaml } }\n" +
			"  unit: { adapter: { command: sh " + script + " }, smoke: { file: unit.yaml } }\n"
	}
	set := func(file, id string) string {
		return "- file: " + file + "\n  scenarios:\n    - id: \"" + id + "\"\n      why: reviewed with the template\n"
	}
	initScratch(t, "commit", map[string]string{
		"specs/pages.feature": "Feature: Pages\n\n  @ID-PAGE-01\n  Scenario: A page\n    Given a page\n",
		"specs/smoke.yaml":    set("pages.feature", "@ID-PAGE-01"), "unit.yaml": set("a_test.go", "@U-1"),
		"lists.sh":   `printf '%s\n' '{"protocol":1,"tests":[{"id":"U-1","file":"a_test.go","live":true}],"files":["a_test.go"]}'` + "\n",
		"fails.sh":   "echo 'no test binary yet' >&2\nexit 3\n",
		"keeps.yaml": policy("lists.sh"), "fails.yaml": policy("fails.sh"),
	})
	code, _, stderr, err := initRun(t, false, "--policy", "fails.yaml")
	if code != ExitPolicy || err != nil || !strings.Contains(stderr, "the list of tests.unit failed") {
		t.Errorf("fails.yaml: exit %d, %v:\n%s", code, err, stderr)
	}
	for text, want := range map[string]string{
		set("pages.feature", "@ID-PAGE-09"): "FAIL the smoke list names @ID-PAGE-09, which is not a live scenario of pages.feature\n",
		"{}\n":                              "FAIL specs/smoke.yaml: is not a list of files\n",
	} {
		putFile(t, "specs/smoke.yaml", text)
		code, _, stderr, err := initRun(t, false, "--policy", "keeps.yaml")
		if code != ExitPolicy || err != nil || !strings.Contains(stderr, "; specs/smoke.yaml fails it:\n  "+want) {
			t.Errorf("%q: exit %d, %v:\n%s", text, code, err, stderr)
		}
	}
	putFile(t, "specs/smoke.yaml", "{}\n")
	code, stdout, _, err := initRun(t, true, "--policy", "keeps.yaml")
	var got struct {
		Action   string
		Problems []struct{ Rule, Fix string }
	}
	if code != ExitPolicy || err != nil || json.Unmarshal([]byte(stdout), &got) != nil || got.Action != "refused" ||
		len(got.Problems) != 1 || got.Problems[0].Rule != "init-policy-smoke" ||
		got.Problems[0].Fix != "correct specs/smoke.yaml, or remove it for init --policy to derive one" {
		t.Errorf("--json: exit %d, %v:\n%s", code, err, stdout)
	}
	if _, err := os.Stat("itos.yaml"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("itos.yaml was written: %v", err)
	}
	putFile(t, "specs/smoke.yaml", set("pages.feature", "@ID-PAGE-01"))
	code, stdout, _, err = initRun(t, false, "--policy", "keeps.yaml")
	if code != 0 || err != nil || !strings.Contains(stdout, "kept specs/smoke.yaml\nkept unit.yaml\n") ||
		!strings.Contains(stdout, "git add itos.yaml tasks/phase-1.yaml tasks/work-items.yaml specs/smoke.yaml unit.yaml,") {
		t.Errorf("keeps.yaml: exit %d, %v:\n%s", code, err, stdout)
	}
	if text, _ := os.ReadFile("unit.yaml"); string(text) != set("a_test.go", "@U-1") {
		t.Errorf("unit.yaml:\n%s", text)
	}
}

// Under --json a project's data there already is refused as its own object,
// exit 1.
func TestPolicyInitRefusesUnderJSON(t *testing.T) {
	initScratch(t, "commit", map[string]string{"p.yaml": "version: 1\n", "tasks/work-items.yaml": ""})
	code, stdout, _, err := initRun(t, true, "--policy", "p.yaml")
	var got struct {
		Action   string
		Problems []struct{ Rule, Message string }
	}
	if code != ExitPolicy || err != nil || json.Unmarshal([]byte(stdout), &got) != nil || got.Action != "refused" ||
		len(got.Problems) != 1 || !strings.HasPrefix(got.Problems[0].Message, "tasks/work-items.yaml is there already") {
		t.Errorf("exit %d, %v:\n%s", code, err, stdout)
	}
}

// In a folder that is no repository, init --policy runs git init, writes
// the policy with no commits.since, keeps its pin and says so; under --json
// the pin is the policy's.
func TestPolicyInitInAFolderWithNoRepository(t *testing.T) {
	pin := "pin: { version: 9.1.0, checksums: " + strings.Repeat("a", 64) + " }\n"
	initScratch(t, "", map[string]string{"p.yaml": "version: 1\ncommits:\n  since: " + strings.Repeat("f", 40) + "\n" + pin})
	code, stdout, _, err := initRun(t, false, "--policy", "p.yaml")
	if code != 0 || err != nil || !strings.Contains(stdout, "Ran git init") ||
		!strings.Contains(stdout, "the config has no commits.since") || !strings.Contains(stdout, "Pinned itos 9.1.0, as the policy does.") {
		t.Fatalf("exit %d, %v:\n%s", code, err, stdout)
	}
	text, _ := os.ReadFile("itos.yaml")
	if strings.Contains(string(text), "since") {
		t.Errorf("the config keeps a since:\n%s", text)
	}
	// A since among commits' other keys goes alone; a kind with no smoke set
	// gets none.
	initScratch(t, "", map[string]string{"p.yaml": "version: 1\n" + pin + "commits: { types: [chore], since: " +
		strings.Repeat("f", 40) + " }\ntests: { scenario: { root: specs, id: 'ID-\\d+' } }\n"})
	code, stdout, _, err = initRun(t, true, "--policy", "p.yaml")
	if text, _ := os.ReadFile("itos.yaml"); !strings.Contains(string(text), "commits: { types: [chore] }\n") {
		t.Errorf("the config:\n%s", text)
	}
	var got struct {
		GitInit bool `json:"git_init"`
		Since   *string
		Pin     map[string]string
	}
	if code != 0 || err != nil || json.Unmarshal([]byte(stdout), &got) != nil || !got.GitInit || got.Since != nil ||
		got.Pin["version"] != "9.1.0" {
		t.Errorf("--json: exit %d, %v:\n%s", code, err, stdout)
	}
}

// The policy's text is the config's, CRLF and a missing last line break
// kept; the sections init adds follow it in its line endings.
func TestPolicyConfigKeepsTheTextsLineEndings(t *testing.T) {
	text := "version: 1\r\nhooks: { bin: itos }"
	src, err := config.FromText("p.yaml", text, false)
	if err != nil {
		t.Fatal(err)
	}
	_, made, err := policyConfig("p.yaml", text, src, policyPlan{file: "itos.yaml", pin: &[2]string{"9.2.0", "b"}})
	want := text + "\r\n\r\n# The tasks, as itos init lays them out: one file per phase.\r\nledger:\r\n" +
		"  files: \"tasks/phase-{group}.yaml\"\r\n  id: \"T-\\\\d+\"\r\n\r\n# The itos release this repository runs, the newest " +
		"when itos init ran:\r\n# itos pin moves it.\r\npin:\r\n  version: \"9.2.0\"\r\n  checksums: \"b\"\r\n"
	if made != want {
		t.Errorf("got %q (%v)\nwant %q", made, err, want)
	}
}

// A registry's groups key YAML would not read as itself is quoted.
func TestFreshRegistryQuotesItsKey(t *testing.T) {
	if got := freshRegistry("the groups"); !strings.Contains(got, "\n\"the groups\":\n  1: null") {
		t.Errorf("%s", got)
	}
}

// A data folder that is a loop of symbolic links resolves to nothing, so
// what goes in it is refused as outside the project.
func TestPolicyPathsRefuseALinkLoop(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a symbolic link needs a privilege on Windows")
	}
	initScratch(t, "", nil)
	if err := os.Symlink("loop", "loop"); err != nil {
		t.Fatal(err)
	}
	got := policyPaths(".", []policyFile{{key: "ledger.files", path: filepath.Join("loop", "phase-1.yaml")}})
	if len(got) != 1 || got[0].Message != "ledger.files puts loop/phase-1.yaml outside the project" {
		t.Errorf("%v", got)
	}
}

// A file init --policy cannot write stops it: it names what it did, git init
// and the files written, and removes none of it.
func TestPolicyInitReportsItsPartialWork(t *testing.T) {
	dir := initScratch(t, "", map[string]string{"p.yaml": "version: 1\n", "tasks/README.md": "mine\n"})
	readOnly(t, filepath.Join(dir, "tasks"))
	_, _, _, err := initRun(t, false, "--policy", "p.yaml")
	if err == nil || !strings.HasSuffix(err.Error(), "; it ran git init, wrote itos.yaml, and left them as they are") {
		t.Errorf("%v", err)
	}
	if _, err := os.Stat("itos.yaml"); err != nil {
		t.Errorf("itos.yaml was removed: %v", err)
	}

	// A folder it cannot make.
	dir = initScratch(t, "commit", map[string]string{"p.yaml": "version: 1\nledger: { files: \"a/b/phase-{group}.yaml\" }\n",
		"a/README.md": "mine\n"})
	readOnly(t, filepath.Join(dir, "a"))
	_, _, _, err = initRun(t, false, "--policy", "p.yaml")
	if err == nil || !strings.Contains(err.Error(), "mkdir a/b: permission denied; it wrote itos.yaml, and left") {
		t.Errorf("%v", err)
	}

	dir = initScratch(t, "commit", map[string]string{"p.yaml": "version: 1\n"})
	readOnly(t, dir)
	_, _, _, err = initRun(t, false, "--policy", "p.yaml")
	if err == nil || !strings.HasPrefix(err.Error(), "init --policy did not initialize the project: open itos.yaml") ||
		strings.Contains(err.Error(), "; it ") {
		t.Errorf("%v", err)
	}
}

// Hooks that cannot be declared stop init --policy once its files are
// written, naming them.
func TestPolicyInitWhereTheHooksCannotBeDeclared(t *testing.T) {
	dir := initScratch(t, "commit", map[string]string{"p.yaml": "version: 1\n"})
	readOnly(t, filepath.Join(dir, ".git"))
	_, _, _, err := initRun(t, false, "--policy", "p.yaml")
	if err == nil || !strings.Contains(err.Error(), "; it wrote itos.yaml, wrote tasks/phase-1.yaml, wrote tasks/work-items.yaml,") {
		t.Errorf("%v", err)
	}
}

// What init --policy says to commit names the files it wrote, the plugin's
// project settings and the rules for agents.
func TestPolicyInitNamesWhatToCommit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake claude is a shell script")
	}
	dir := initScratch(t, "commit", map[string]string{"p.yaml": "version: 1\n"})
	bin := filepath.Join(dir, ".git", "bin")
	putFile(t, filepath.Join(bin, "claude"), "#!/bin/sh\ncase \"$1 $2\" in\n\"plugin list\") echo '[]' ;;\n"+
		"\"plugin install\") mkdir -p .claude && echo '{}' > .claude/settings.json ;;\nesac\n")
	if err := os.Chmod(filepath.Join(bin, "claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	var stdout strings.Builder
	code, err := initCommand([]string{"--policy", "p.yaml", "--plugin", "project", "--agent-rules", "--no-git-shim"},
		Out{Stdout: &stdout, Stderr: &stdout})
	want := "git add itos.yaml tasks/phase-1.yaml tasks/work-items.yaml .claude/settings.json AGENTS.md CLAUDE.md, " +
		"then itos commit --task T-1 -m 'chore: adopt itos'\n"
	if code != 0 || err != nil || !strings.HasSuffix(stdout.String(), want) {
		t.Errorf("exit %d, %v:\n%s", code, err, stdout.String())
	}
}
