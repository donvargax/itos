package initcmd

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// bs is a checksums.txt's SHA-256.
const bs = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

// pinned is a policy's pin.
const pinned = "pin: { version: 9.1.0, checksums: \"" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" + "\" }\n"

// The policy made a project in a repository with a commit: the config, its
// since HEAD, the newest release pinned, the fresh ledger and registry,
// hook install and the offers, then what to commit, the plugin's project
// settings and the rules for agents among it.
func TestPolicyInitMakesTheProject(t *testing.T) {
	f := scratch(t, "commit", map[string]string{"sub/p.yaml": "# The policy.\nversion: 1\ncommits:\n  types: [chore]\n"})
	f.offered.ProjectPlugin, f.offered.RulesWritten = true, []string{"AGENTS.md"}
	c, err := Parse(context.Background(), []string{"--policy", "p.yaml"}, "sub", false, nil, f)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	code, err := c.Run(context.Background(), &stdout, &stderr, NewHandler(f))
	want := "wrote itos.yaml\nwrote tasks/phase-1.yaml\nwrote tasks/work-items.yaml\n" +
		"itos.yaml holds the policy of p.yaml, made fresh for this project: no task, owner, status, commit or result of " +
		"the policy's own came with it.\ncommits.since is HEAD, 0123456: no commit before it is judged.\n" +
		"Pinned itos 9.2.0, the newest release.\nhooks declared\noffered\n" +
		"Commit what init wrote with the task that adopts itos: git add itos.yaml tasks/phase-1.yaml tasks/work-items.yaml " +
		".claude/settings.json AGENTS.md, then itos commit --task T-1 -m 'chore: adopt itos'\n"
	if code != 0 || err != nil || stdout.String() != want {
		t.Fatalf("exit %d, %v:\n%s%s", code, err, stdout.String(), stderr.String())
	}
	if text := read("itos.yaml"); !strings.HasPrefix(text, "# The policy.\nversion: 1\ncommits:\n  types: [chore]\n  since: "+head+"\n") ||
		!strings.Contains(text, "ledger:\n") || !strings.Contains(text, "version: \"9.2.0\"") {
		t.Errorf("itos.yaml:\n%s", text)
	}
	if read("tasks/phase-1.yaml") != (starter{}).ledger() || read("tasks/work-items.yaml") != starterRegistryText {
		t.Error("the fresh data is not the starter's")
	}
	if len(f.offers) != 1 || f.offers[0] != (Offer{Ask: true}) || len(f.installs) != 1 {
		t.Errorf("offers %+v, hooks %v", f.offers, f.installs)
	}
}

// In a folder that is no repository it runs git init, writes no since,
// keeps the policy's pin and says so; under --json the pin is the
// policy's and the policy is named. Under --stealth it all goes in the
// git folder, and nothing is to commit; with no release server and no pin
// in the policy, it pins nothing and says why.
func TestPolicyInitInAFolderWithNoRepository(t *testing.T) {
	f := scratch(t, "", map[string]string{"p.yaml": "version: 1\ncommits:\n  since: " + strings.Repeat("f", 40) + "\n" + pinned})
	code, stdout, _, err := call(t, f, "--policy", "p.yaml")
	if code != 0 || err != nil || !strings.HasPrefix(stdout, "Ran git init") ||
		!strings.Contains(stdout, "The repository has no commit, so the config has no commits.since: every commit is judged.\n") ||
		!strings.Contains(stdout, "Pinned itos 9.1.0, as the policy does.\n") {
		t.Fatalf("exit %d, %v:\n%s", code, err, stdout)
	}
	if strings.Contains(read("itos.yaml"), "since") {
		t.Errorf("the config keeps a since:\n%s", read("itos.yaml"))
	}

	f = scratch(t, "", map[string]string{"p.yaml": "version: 1\n" + pinned + "commits: { types: [chore], since: " +
		strings.Repeat("f", 40) + " }\n"})
	code, stdout, _, err = call(t, f, "--json", "--policy", "p.yaml")
	var got struct {
		Config, Action, Policy string
		GitInit                bool `json:"git_init"`
		Since                  *string
		Files                  []writtenFile
		Pin                    map[string]string
		PinProblem             *string `json:"pin_problem"`
		Hooks                  map[string]any
		Plugin                 string
	}
	if code != 0 || err != nil || json.Unmarshal([]byte(stdout), &got) != nil || got.Config != "itos.yaml" ||
		got.Action != "initialized" || got.Policy != "p.yaml" || !got.GitInit || got.Since != nil || len(got.Files) != 3 ||
		got.Pin["version"] != "9.1.0" || got.PinProblem != nil || len(got.Hooks) != 1 || got.Plugin != "plugin" {
		t.Errorf("--json: exit %d, %v:\n%s", code, err, stdout)
	}
	if text := read("itos.yaml"); !strings.Contains(text, "commits: { types: [chore] }\n") {
		t.Errorf("the config:\n%s", text)
	}

	f = scratch(t, "commit", map[string]string{"p.yaml": "version: 1\n"})
	f.releaseErr = errors.New("no server")
	code, stdout, _, err = call(t, f, "--policy", "p.yaml", "--stealth")
	stealth := filepath.Join(".git", "itos")
	if code != 0 || err != nil || !strings.Contains(stdout, "wrote "+filepath.ToSlash(stealth)+"/itos.yaml\n") ||
		!strings.Contains(stdout, "Pinned no itos: no server. Run itos pin when the release server can be reached.\n") ||
		!strings.HasSuffix(stdout, "Nothing the project tracks changed: commit as ever, with itos commit, and push with itos push.\n") ||
		read(filepath.Join(stealth, "tasks", "phase-1.yaml")) != (starter{stealth: true}).ledger() {
		t.Errorf("--stealth: exit %d, %v:\n%s", code, err, stdout)
	}
	f = scratch(t, "commit", map[string]string{"p.yaml": "version: 1\n"})
	f.releaseErr = errors.New("no server")
	_, stdout, _, _ = call(t, f, "--json", "--policy", "p.yaml")
	if !strings.Contains(stdout, `"pin": null,`) || !strings.Contains(stdout, `"pin_problem": "no server"`) {
		t.Errorf("--json, no server:\n%s", stdout)
	}
}

// A policy that cannot be read, read as a config, or edited in place is
// refused with nothing written; so is one run in the git folder, or with
// ITOS_CONFIG set; and git init failing fails it once the policy is judged.
func TestPolicyInitRefusesAPolicyItCannotUse(t *testing.T) {
	f := scratch(t, "commit", map[string]string{"flow.yaml": "{version: 1}\n", "empty.yaml": "version: 1\ncommits: {}\n",
		"broken.yaml": "version: [\n", "p.yaml": "version: 1\n"})
	for policy, want := range map[string]string{
		"missing.yaml": "missing.yaml: cannot be read",
		"flow.yaml":    "flow.yaml: init --policy cannot make the project's config of it",
		"empty.yaml":   "empty.yaml: init --policy cannot make the project's config of it: commits.since",
		"broken.yaml":  "broken.yaml",
	} {
		_, _, _, err := call(t, f, "--policy", policy)
		var c *config.Error
		if !errors.As(err, &c) || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v", policy, err)
		}
	}
	if exists("itos.yaml") {
		t.Error("itos.yaml was written")
	}
	f.Located[""] = vcsport.Location{GitDir: "."}
	if _, _, _, err := call(t, f, "--policy", "p.yaml"); !isUsage(err, "git folder") {
		t.Errorf("in the git folder: %v", err)
	}
	t.Setenv("ITOS_CONFIG", "missing.yaml")
	if _, _, _, err := call(t, f, "--policy", "p.yaml"); !isUsage(err, "init --policy writes the project's own config, "+
		"itos.yaml at its top or with --stealth in the git folder, not the missing.yaml that --config or ITOS_CONFIG names") {
		t.Errorf("ITOS_CONFIG: %v", err)
	}
	f = scratch(t, "", map[string]string{"p.yaml": "version: 1\n"})
	f.initErr = errors.New("git init: injected git failure")
	if _, _, _, err := call(t, f, "--policy", "p.yaml"); err != f.initErr {
		t.Errorf("git init failing: %v", err)
	}
}

// A policy whose ledger cannot name group 1's file nor hold T-1, a stealth
// ledger outside the stealth folder and a ledger outside the project are
// refused before anything is written.
func TestPolicyInitRefusesDataItCannotMake(t *testing.T) {
	f := scratch(t, "commit", map[string]string{
		"layout.yaml": "version: 1\nledger: { files: \"t/phase-{group}.yaml\", group: { pattern: '[a-z]+' }, id: 'X-\\d+' }\n" +
			"commits: { types: [feat] }\n",
		"stealth.yaml": "version: 1\nledger: { files: \"../../t/phase-{group}.yaml\" }\n",
		"outside.yaml": "version: 1\nledger: { files: \"../t/phase-{group}.yaml\" }\n",
		"nogroup.yaml": "version: 1\nledger: { files: \"t/tasks.yaml\" }\n",
	})
	for args, want := range map[string][]string{
		"--policy layout.yaml": {"cannot name the file of the group 1", "ledger.id (X-\\d+) does not match T-1",
			"commits.types does not list chore, the type of T-1, the adoption task"},
		"--policy stealth.yaml --stealth": {"ledger.files puts t/phase-1.yaml outside .git/itos",
			"work.registry puts t/work-items.yaml outside .git/itos"},
		"--policy outside.yaml": {"ledger.files puts ../t/phase-1.yaml outside the project",
			"work.registry puts ../t/work-items.yaml outside the project"},
		"--policy nogroup.yaml":          {"ledger.files has no {group}"},
		"--policy layout.yaml --stealth": {"cannot name the file of the group 1"},
	} {
		_, _, _, err := call(t, f, strings.Fields(args)...)
		var c *config.Error
		if !errors.As(err, &c) || len(c.Problems) != len(want) {
			t.Errorf("init %s: %v", args, err)
			continue
		}
		for i, w := range want {
			if !strings.Contains(c.Problems[i].Message, w) {
				t.Errorf("init %s: %q", args, c.Problems[i].Message)
			}
		}
	}
	if exists("itos.yaml") {
		t.Error("itos.yaml was written")
	}
}

// What of a project is there already refuses init --policy, exit 1, each
// file named once, the config found first; under --json as its own object.
func TestPolicyInitRefusesAProjectThere(t *testing.T) {
	f := scratch(t, "commit", map[string]string{"p.yaml": "version: 1\n", "itos.yaml": "version: 1\n",
		"tasks/phase-2.yaml": "[]\n", "tasks/notes.md": "", "tasks/work-items.yaml": ""})
	code, _, stderr, err := call(t, f, "--policy", "p.yaml")
	want := "itos: init --policy makes a fresh project and never writes over one, so it wrote nothing; these are there " +
		"already:\n  itos.yaml\n  tasks/phase-2.yaml\n  tasks/work-items.yaml\nRun init --policy where no itos config or data " +
		"is, or init without --policy to check this project.\n"
	if code != 1 || err != nil || stderr != want {
		t.Errorf("exit %d, %v:\n%s", code, err, stderr)
	}
	f = scratch(t, "commit", map[string]string{"p.yaml": "version: 1\n", "tasks/work-items.yaml": ""})
	code, stdout, _, err := call(t, f, "--json", "--policy", "p.yaml")
	var got struct {
		Config, Action string
		Problems       []struct{ Rule, Message, Fix string }
	}
	if code != 1 || err != nil || json.Unmarshal([]byte(stdout), &got) != nil || got.Config != "itos.yaml" || got.Action != "refused" ||
		len(got.Problems) != 1 || got.Problems[0].Rule != "init-policy-exists" ||
		got.Problems[0].Message != "tasks/work-items.yaml is there already: init --policy makes a fresh project, never over one" {
		t.Errorf("--json: exit %d, %v:\n%s", code, err, stdout)
	}
	// A stealth config there is found as well.
	f = scratch(t, "commit", map[string]string{"p.yaml": "version: 1\n", ".git/itos/itos.yaml": "version: 1\n"})
	if code, _, stderr, _ := call(t, f, "--policy", "p.yaml"); code != 1 || !strings.Contains(stderr, "  .git/itos/itos.yaml\n") {
		t.Errorf("a stealth config: exit %d:\n%s", code, stderr)
	}
}

// A smoke set the project has is kept as it is when the smoke check passes
// over it, and named in what to commit; one the check refuses, or cannot
// read, is refused with its problems, exit 1, before anything is written;
// where there is none, one is derived from the project's own tests.
func TestPolicyInitJudgesTheSmokeSets(t *testing.T) {
	policy := "version: 1\ntests:\n  scenario: { root: specs, id: 'ID-[A-Z]+-\\d+', smoke: { file: specs/smoke.yaml } }\n"
	set := func(id string) string {
		return "- file: pages.feature\n  scenarios:\n    - id: \"" + id + "\"\n      why: reviewed with the template\n"
	}
	files := map[string]string{"p.yaml": policy, "specs/pages.feature": "Feature: Pages\n\n  @ID-PAGE-01\n  Scenario: A page\n    Given a page\n"}
	f := scratch(t, "commit", files)
	code, stdout, _, err := call(t, f, "--policy", "p.yaml")
	if code != 0 || err != nil || !strings.Contains(stdout, "wrote specs/smoke.yaml\n") || !strings.Contains(read("specs/smoke.yaml"), `"@ID-PAGE-01"`) {
		t.Errorf("derived: exit %d, %v:\n%s", code, err, stdout)
	}
	for text, want := range map[string]string{
		set("@ID-PAGE-09"): "FAIL the smoke list names @ID-PAGE-09, which is not a live scenario of pages.feature\n",
		"{}\n":             "FAIL specs/smoke.yaml: is not a list of files\n",
	} {
		files["specs/smoke.yaml"] = text
		f := scratch(t, "commit", files)
		code, _, stderr, err := call(t, f, "--policy", "p.yaml")
		opening := "itos: init --policy keeps the smoke set a project has only when tests smoke check passes over it, so it " +
			"wrote nothing; specs/smoke.yaml fails it:\n  "
		tail := "Correct specs/smoke.yaml, or remove it for init --policy to derive one, then run init --policy again.\n"
		if code != 1 || err != nil || !strings.HasPrefix(stderr, opening+want) || !strings.HasSuffix(stderr, tail) {
			t.Errorf("%q: exit %d, %v:\n%s", text, code, err, stderr)
		}
	}
	files["specs/smoke.yaml"] = "{}\n"
	f = scratch(t, "commit", files)
	_, stdout, _, _ = call(t, f, "--json", "--policy", "p.yaml")
	if !strings.Contains(stdout, `"fix": "correct specs/smoke.yaml, or remove it for init --policy to derive one"`) {
		t.Errorf("--json:\n%s", stdout)
	}
	files["specs/smoke.yaml"] = set("@ID-PAGE-01")
	f = scratch(t, "commit", files)
	code, stdout, _, err = call(t, f, "--policy", "p.yaml")
	if code != 0 || err != nil || !strings.Contains(stdout, "kept specs/smoke.yaml\n") ||
		!strings.Contains(stdout, "git add itos.yaml tasks/phase-1.yaml tasks/work-items.yaml specs/smoke.yaml,") ||
		read("specs/smoke.yaml") != set("@ID-PAGE-01") {
		t.Errorf("kept: exit %d, %v:\n%s", code, err, stdout)
	}
}

// A kind whose tests cannot be listed stops init --policy: a command
// adapter's list that fails is a refusal naming it, exit 1, under --json
// its own object; any other kind's, the error.
func TestPolicyInitWhereTheListFails(t *testing.T) {
	f := scratch(t, "commit", map[string]string{"p.yaml": "version: 1\ntests: { unit: { adapter: { command: itos-no-such-command }, " +
		"tag_prefix: '', smoke: { file: smoke.yaml } } }\n"})
	code, _, stderr, err := call(t, f, "--policy", "p.yaml")
	if code != 1 || err != nil || !strings.HasPrefix(stderr, "itos: init --policy wrote nothing: the list of tests.unit failed, "+
		"so it cannot derive its smoke set:\n  ") ||
		!strings.HasSuffix(stderr, "\nMake tests.unit's adapter list the project's tests, then run init --policy again.\n") {
		t.Errorf("exit %d, %v:\n%s", code, err, stderr)
	}
	code, stdout, _, err := call(t, f, "--json", "--policy", "p.yaml")
	if code != 1 || err != nil || !strings.Contains(stdout, `"rule": "init-policy-list"`) ||
		!strings.Contains(stdout, `"fix": "make tests.unit's adapter list the project's tests, then run init --policy again"`) {
		t.Errorf("--json: exit %d, %v:\n%s", code, err, stdout)
	}
	f = scratch(t, "commit", map[string]string{"p.yaml": "version: 1\ntests: { scenario: { root: specs, smoke: { file: s.yaml } } }\n"})
	if _, _, _, err := call(t, f, "--policy", "p.yaml"); err == nil || exists("itos.yaml") {
		t.Errorf("a root that is not there: %v", err)
	}
}

// A file init --policy cannot write stops it: it names what it did, git
// init and the files written, and removes none of it; hook install failing
// names them too.
func TestPolicyInitReportsItsPartialWork(t *testing.T) {
	// Two files at one path: the second finds the first there.
	f := scratch(t, "", map[string]string{"p.yaml": "version: 1\ntests: { scenario: { root: specs, id: 'ID-\\d+', " +
		"smoke: { file: tasks/work-items.yaml } } }\n", "specs/a.feature": "Feature: A\n"})
	_, _, _, err := call(t, f, "--policy", "p.yaml")
	if err == nil || !strings.HasPrefix(err.Error(), "init --policy did not initialize the project: ") ||
		!strings.HasSuffix(err.Error(), "; it ran git init, wrote itos.yaml, wrote tasks/phase-1.yaml, wrote "+
			"tasks/work-items.yaml, and left them as they are") || read("tasks/work-items.yaml") != starterRegistryText {
		t.Errorf("one path twice: %v", err)
	}
	f = scratch(t, "commit", map[string]string{"p.yaml": "version: 1\n"})
	f.hooksErr = errors.New("git config failed")
	_, _, _, err = call(t, f, "--policy", "p.yaml")
	if err == nil || err.Error() != "init --policy did not initialize the project: git config failed; it wrote itos.yaml, "+
		"wrote tasks/phase-1.yaml, wrote tasks/work-items.yaml, and left them as they are" {
		t.Errorf("hook install failing: %v", err)
	}
	if got := partial(errors.New("no"), false, nil).Error(); got != "init --policy did not initialize the project: no" {
		t.Errorf("nothing done: %s", got)
	}
}

// hook install's code is init --policy's, the offers' only when it is 0,
// and with it not 0 nothing is named to commit.
func TestPolicyInitTakesHookInstallsCode(t *testing.T) {
	f := scratch(t, "commit", map[string]string{"p.yaml": "version: 1\n"})
	f.hooksCode, f.offered.Code = 3, 1
	code, stdout, _, err := call(t, f, "--policy", "p.yaml")
	if code != 3 || err != nil || !strings.HasSuffix(stdout, "offered\n") {
		t.Errorf("exit %d, %v:\n%s", code, err, stdout)
	}
	f = scratch(t, "commit", map[string]string{"p.yaml": "version: 1\n"})
	f.offered.Code = 1
	if code, _, _, err := call(t, f, "--json", "--policy", "p.yaml"); code != 1 || err != nil {
		t.Errorf("an offer failing: exit %d, %v", code, err)
	}
}

// The policy's text is the config's, CRLF and a missing last line break
// kept; the sections init adds follow it in its line endings; each
// footer's since goes, and commits.since is set, dropped or added.
func TestPolicyConfig(t *testing.T) {
	h, ctx := NewHandler(scratch(t, "", nil)), context.Background()
	made := func(text string, plan policyPlan) string {
		t.Helper()
		src, err := h.fromText(ctx, "p.yaml", text, false)
		if err != nil {
			t.Fatal(err)
		}
		_, made, err := h.policyConfig(ctx, "p.yaml", text, src, plan)
		if err != nil {
			t.Fatal(err)
		}
		return made
	}
	text := "version: 1\r\nhooks: { bin: itos }"
	want := text + "\r\n\r\n# The tasks, as itos init lays them out: one file per phase.\r\nledger:\r\n" +
		"  files: \"tasks/phase-{group}.yaml\"\r\n  id: \"T-\\\\d+\"\r\n\r\n# The itos release this repository runs, the newest " +
		"when itos init ran:\r\n# itos pin moves it.\r\npin:\r\n  version: \"9.2.0\"\r\n  checksums: \"" + bs + "\"\r\n"
	if got := made(text, policyPlan{file: "itos.yaml", pin: &[2]string{"9.2.0", bs}}); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
	ledger := "ledger: { files: \"tasks/phase-{group}.yaml\" }\n"
	old := strings.Repeat("f", 40)
	for _, c := range []struct{ text, since, want string }{
		{"version: 1\n" + ledger, head, "version: 1\n" + ledger + "\n# Where verification starts: this commit and the ones before it, " +
			"written\n# before itos, are never judged.\ncommits:\n  since: \"" + head + "\"\n"},
		{"version: 1\n" + ledger + "commits:\n  since: " + old + "\n", "", "version: 1\n" + ledger},
		{"version: 1\n" + ledger + "commits:\n  since: " + old + "\n  types: [chore]\n", "", "version: 1\n" + ledger + "commits:\n  types: [chore]\n"},
		{"version: 1\n" + ledger + "commits:\n  since: " + old + "\n", head, "version: 1\n" + ledger + "commits:\n  since: " + head + "\n"},
		{"version: 1\n" + ledger + "commits:\n  footers:\n    Task: { source: ledger, since: " + old + " }\n", "",
			"version: 1\n" + ledger + "commits:\n  footers:\n    Task: { source: ledger }\n"},
	} {
		if got := made(c.text, policyPlan{file: "itos.yaml", since: c.since, pin: &[2]string{"9.2.0", bs}, kept: true}); got != c.want {
			t.Errorf("%q since %q:\ngot  %q\nwant %q", c.text, c.since, got, c.want)
		}
	}
}

// A registry's groups key YAML would not read as itself is quoted.
func TestFreshRegistryQuotesItsKey(t *testing.T) {
	if got := freshRegistry("the groups"); !strings.Contains(got, "\n\"the groups\":\n  1: null") {
		t.Errorf("%s", got)
	}
	if got := freshRegistry("groups"); !strings.Contains(got, "\ngroups:\n  1: null") {
		t.Errorf("%s", got)
	}
}

// A data folder that is a loop of symbolic links resolves to nothing, so
// what goes in it is refused as outside the project.
func TestPolicyPathsRefuseALinkLoop(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a symbolic link needs a privilege on Windows")
	}
	scratch(t, "", nil)
	if err := os.Symlink("loop", "loop"); err != nil {
		t.Fatal(err)
	}
	got := policyPaths(".", []policyFile{{key: "ledger.files", path: filepath.Join("loop", "phase-1.yaml")}})
	if len(got) != 1 || got[0].Message != "ledger.files puts loop/phase-1.yaml outside the project" {
		t.Errorf("%v", got)
	}
}

// writeNew makes the file's folder and writes it, and never writes over a
// file there.
func TestWriteNew(t *testing.T) {
	scratch(t, "", map[string]string{"there.txt": "mine\n"})
	if err := writeNew(filepath.Join("a", "b.txt"), "new\n"); err != nil || read(filepath.Join("a", "b.txt")) != "new\n" {
		t.Errorf("%v", err)
	}
	if err := writeNew("there.txt", "new\n"); err == nil || read("there.txt") != "mine\n" {
		t.Errorf("over a file: %v", err)
	}
	if err := writeNew(filepath.Join("there.txt", "c.txt"), "new\n"); err == nil {
		t.Error("in a file: no error")
	}
}
