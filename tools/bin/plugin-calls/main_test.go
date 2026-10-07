package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeEnv, set, makes the test binary an itos: the program runs it as -itos,
// so the tests need no build of the real one and run wherever go test does.
// Its value picks how the fake answers.
const fakeEnv = "PLUGIN_CALLS_FAKE_ITOS"

func TestMain(m *testing.M) {
	if mode, ok := os.LookupEnv(fakeEnv); ok {
		os.Exit(fakeItos(mode, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
	}
	os.Exit(m.Run())
}

// fakeItos answers the commands the fixture plugin calls, as itos does, and
// refuses any other with exit 2, its usage error. mode "renamed-key" answers
// work list with entries in place of items.
func fakeItos(mode string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	list := func(key string) int {
		if mode == "renamed-key" && key == "items" {
			key = "entries"
		}
		fmt.Fprintf(stdout, `{"schema": 1, %q: [{"id": "T-1", "title": "A title"}]}`, key)
		return 0
	}
	switch strings.Join(args, " ") {
	case "guard claude-code":
		// The real guard exits 1 on an input it cannot read; the fake exits 2,
		// so a run that gives the guard no hook input fails these tests.
		var input struct {
			Event string `json:"hook_event_name"`
			Tool  string `json:"tool_name"`
		}
		if err := json.NewDecoder(stdin).Decode(&input); err != nil || input.Event != "PreToolUse" || input.Tool != "Bash" {
			fmt.Fprintln(stderr, "itos: guard claude-code cannot read Claude Code's PreToolUse input on stdin")
			return 2
		}
		return 0
	case "work list --all --json", "work list --json":
		return list("items")
	case "task list --json":
		return list("tasks")
	}
	fmt.Fprintf(stderr, "itos: unknown command: %s (itos --help)\n", strings.Join(args, " "))
	return 2
}

// The fixture plugin's scripts, written as the real plugin's are.
const guardSh = `# The guard: itos guard claude-code, run by the itos on the PATH.
command -v itos >/dev/null 2>&1 || exit 0
itos guard claude-code
s=$?
[ "$s" -ne 2 ] || s=1
exit "$s"
`

const registerTs = `import { listed, merge } from "./titles";

// The itos on the PATH; "itos" in a comment is not a call.
const ITOS = "itos";

async function run($: Engine, root: string, argv: string[]) {
	return await $.process.run(argv, { cwd: root, timeoutMs: 15_000 });
}

/* itos task list, in a block comment, is not one either. */
async function itos($: Engine, root: string, args: string[]): Promise<unknown> {
	const answer = await run($, root, [ITOS, ...args, "--json"]);
	return answer?.exitCode === 0 ? JSON.parse(answer.stdout) : undefined;
}

async function everyItem($: Engine, root: string): Promise<unknown> {
	return (await itos($, root, ["work", "list", "--all"])) ?? itos($, root, ["work", "list"]);
}

export async function refresh($: Engine) {
	const root = await $.session.root();
	const [work, tasks] = await Promise.all([everyItem($, root), itos($, root, ["task", "list"])]);
	return merge(listed(work, "items"), listed(tasks, "tasks"));
}
`

// titles.ts calls nothing, but its regular expressions hold quotes and
// brackets a reader must not take for strings.
const titlesTs = "export function listed(json: unknown, list: \"items\" | \"tasks\") {\n" +
	"\tconst raw = String(json).replace(/^([\"'])(.*)\\1$/, \"$2\");\n" +
	"\tconst escape = (s: string) => s.replace(/[.*+?^${}()|[\\]\\\\]/g, \"\\\\$&\");\n" +
	"\treturn `\\`${raw}: ${escape(list)}\\``;\n" +
	"}\n"

// A test beside the scripts is not read: this one names itos everywhere.
const registerTestTs = `const SHIPPED = "itos";
run(["sh", "-c", "itos frobnicate"], SHIPPED);
`

const hooksJSON = `{
	"modules": ["./register.ts"],
	"hooks": {"PreToolUse": [{"matcher": "Bash", "hooks": [
		{"type": "command", "command": "sh \"${CLAUDE_PLUGIN_ROOT}/hooks/guard.sh\"", "timeout": 30}
	]}]}
}`

func fixture() map[string]string {
	return map[string]string{
		"plugin/hooks/guard.sh":         guardSh,
		"plugin/hooks/register.ts":      registerTs,
		"plugin/hooks/titles.ts":        titlesTs,
		"plugin/hooks/register.test.ts": registerTestTs,
		"plugin/hooks/hooks.json":       hooksJSON,
	}
}

// check runs plugin-calls on a fixture plugin, in a scratch folder, against
// the fake itos in the given mode, returning its exit status and output.
func check(t *testing.T, files map[string]string, mode string) (int, string, string) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, text := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
	t.Setenv(fakeEnv, mode)
	var stdout, stderr bytes.Buffer
	code := run([]string{"-plugin", "plugin", "-itos", self}, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func contains(t *testing.T, what, out string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Errorf("%s lacks %q:\n%s", what, want, out)
		}
	}
}

func TestEveryCallAnsweredPasses(t *testing.T) {
	code, stdout, stderr := check(t, fixture(), "")
	if code != 0 {
		t.Fatalf("exit %d, want 0; stderr:\n%s", code, stderr)
	}
	contains(t, "stdout", stdout,
		"itos guard claude-code, called by plugin/hooks/guard.sh:3, is answered (exit 0)",
		"itos work list --all --json, called by plugin/hooks/register.ts:17, is answered with items",
		"itos work list --json, called by plugin/hooks/register.ts:17, is answered with items",
		"itos task list --json, called by plugin/hooks/register.ts:22, is answered with tasks",
	)
	if n := strings.Count(stdout, "is answered"); n != 4 {
		t.Errorf("%d calls answered, want 4:\n%s", n, stdout)
	}
}

func TestCallInAHookCommandIsRead(t *testing.T) {
	files := fixture()
	files["plugin/hooks/hooks.json"] = `{"hooks": {"PreToolUse": [{"hooks": [{"type": "command", "command": "itos guard claude-code"}]}]}}`
	code, stdout, stderr := check(t, files, "")
	if code != 0 {
		t.Fatalf("exit %d, want 0; stderr:\n%s", code, stderr)
	}
	contains(t, "stdout", stdout, "itos guard claude-code, called by plugin/hooks/guard.sh:3, plugin/hooks/hooks.json (a hook's command):1, is answered")
}

func TestCallItosDoesNotAnswerFails(t *testing.T) {
	files := fixture()
	files["plugin/hooks/guard.sh"] = strings.Replace(guardSh, "itos guard claude-code\n", "itos guard codex\n", 1)
	code, _, stderr := check(t, files, "")
	if code != 1 {
		t.Fatalf("exit %d, want 1; stderr:\n%s", code, stderr)
	}
	contains(t, "stderr", stderr,
		"itos guard codex, called by plugin/hooks/guard.sh:3, exits 2: itos does not take it as written: itos: unknown command: guard codex",
		"decision 41",
	)
	if strings.Contains(stderr, "work list") {
		t.Errorf("stderr names a call itos answers:\n%s", stderr)
	}
}

func TestRenamedFlagInAHelperCallFails(t *testing.T) {
	files := fixture()
	files["plugin/hooks/register.ts"] = strings.Replace(registerTs, `["work", "list", "--all"]`, `["work", "list", "--every"]`, 1)
	code, _, stderr := check(t, files, "")
	if code != 1 {
		t.Fatalf("exit %d, want 1; stderr:\n%s", code, stderr)
	}
	contains(t, "stderr", stderr, "itos work list --every --json, called by plugin/hooks/register.ts:17, exits 2")
}

func TestAnswerWithoutTheKeyFails(t *testing.T) {
	code, _, stderr := check(t, fixture(), "renamed-key")
	if code != 1 {
		t.Fatalf("exit %d, want 1; stderr:\n%s", code, stderr)
	}
	contains(t, "stderr", stderr,
		"itos work list --all --json, called by plugin/hooks/register.ts:17, answers with none of the lists the plugin reads (items, tasks)",
		`the plugin reads "items" from an itos answer (plugin/hooks/register.ts:23), but no --json call answers with it as a list`,
	)
}

func TestUnreadableCallsExit2(t *testing.T) {
	for name, tc := range map[string]struct{ file, text, want string }{
		"a shell word known when it runs": {"plugin/hooks/guard.sh", "itos \"$cmd\" claude-code\n",
			`plugin/hooks/guard.sh:1: itos is called with "$cmd"`},
		"itos as a wrapper's argument": {"plugin/hooks/guard.sh", "timeout 5 itos guard claude-code\n",
			`plugin/hooks/guard.sh:1: "itos" names itos where this cannot tell how it runs`},
		"itos alone": {"plugin/hooks/guard.sh", "itos\n", "plugin/hooks/guard.sh:1: itos is called alone"},
		"a spread that is no parameter": {"plugin/hooks/run.ts", "const argv = [\"x\"];\nrun([\"itos\", ...argv]);\n",
			"plugin/hooks/run.ts:2: the argv spreads argv, which is not a parameter"},
		"the const used as a value": {"plugin/hooks/run.ts", "const ITOS = \"itos\";\nrun(ITOS, \"guard\");\n",
			"plugin/hooks/run.ts:2: ITOS names itos, but not at the head of an argv"},
		"a command line in a string": {"plugin/hooks/run.ts", "run([\"sh\", \"-c\", \"itos guard claude-code\"]);\n",
			`plugin/hooks/run.ts:1: the string "itos guard claude-code" holds an itos command line`},
		"a helper called with a variable": {"plugin/hooks/run.ts",
			"function itos(args: string[]) {\n\treturn run([\"itos\", ...args]);\n}\nconst words = [\"task\", \"list\"];\nitos(words);\n",
			"plugin/hooks/run.ts:5: itos is called without an array literal of strings for its parameter args"},
		"an argv changed after it is written": {"plugin/hooks/run.ts", "run([\"itos\"].concat(words));\n",
			"plugin/hooks/run.ts:1: the argv is changed after it is written (.concat)"},
		"a key read from a variable": {"plugin/hooks/run.ts", "listed(answer, key);\n",
			"plugin/hooks/run.ts:1: listed(…) is called without a string literal"},
	} {
		t.Run(name, func(t *testing.T) {
			files := fixture()
			files[tc.file] = tc.text
			code, _, stderr := check(t, files, "")
			if code != 2 {
				t.Fatalf("exit %d, want 2; stderr:\n%s", code, stderr)
			}
			contains(t, "stderr", stderr, tc.want, "a call it cannot read word by word")
		})
	}
}

func TestNoCallFoundExits2(t *testing.T) {
	code, _, stderr := check(t, map[string]string{"plugin/hooks/titles.ts": titlesTs}, "")
	if code != 2 {
		t.Fatalf("exit %d, want 2; stderr:\n%s", code, stderr)
	}
	contains(t, "stderr", stderr, "no itos call found in the 1 script(s) under plugin (plugin/hooks/titles.ts)")
}

func TestJSONCallWithNoKeyReadExits2(t *testing.T) {
	files := fixture()
	files["plugin/hooks/register.ts"] = strings.Replace(registerTs, `merge(listed(work, "items"), listed(tasks, "tasks"))`, `merge(work, tasks)`, 1)
	code, _, stderr := check(t, files, "")
	if code != 2 {
		t.Fatalf("exit %d, want 2; stderr:\n%s", code, stderr)
	}
	contains(t, "stderr", stderr, `no listed(answer, "<key>") in its scripts says which key it reads`)
}

func TestItosThatCannotStartExits2(t *testing.T) {
	dir := t.TempDir()
	for name, text := range fixture() {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
	var stdout, stderr bytes.Buffer
	code := run([]string{"-plugin", "plugin", "-itos", filepath.Join(dir, "no-itos")}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d, want 2; stderr:\n%s", code, stderr.String())
	}
	contains(t, "stderr", stderr.String(), "cannot run itos guard claude-code with")
}
