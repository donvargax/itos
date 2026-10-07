// The last release's scenarios and corpus against the new binary (T-071,
// tools/bin/previous-release), in a scratch repository whose v1.0.0 tag holds
// a release of its own: two scenarios (go test ./features, a black box running
// whatever ITOS_BIN names, as this repository's are), and a corpus of four
// cases and two help cases run by this repository's own conformance runner;
// two of the four start with the same word, and one's name is longer than a
// footer line may be.
// The binary under test is a script; the broken one breaks one scenario and
// one case by its exit code, and says other help, the wordy one says other
// help and other usage errors and nothing else, and each says a version the
// tag never had, as T-069's builds will. Three more cases are refusals: two
// usage errors and a config error; the reworded binary says other words for
// all three and changes one usage error's exit code. Two more pin a report's
// lines and a JSON object holding a message and a fix, and one more writes a
// YAML and a Markdown file: the adding binary adds keys to the object,
// rewords its message and fix, rewrites the report and rewrites the files'
// comments, reordering the YAML's keys and adding one, and six more each
// break one of them by machine output, a written file by its data.
//
//   - the binary the release was written for passes, its version the
//     corpus's {{version}} whatever the tag's package.json says, and what is
//     left out of every case (plain output, message and fix) is said with its
//     count;
//   - the wordy one passes: only machine output is judged (decision 35), an
//     exit code, files_after and json less message and fix, so a help case or
//     a usage error whose words differ passes as any case does;
//   - the reworded one fails naming the usage error whose exit code changed,
//     and neither the other usage error nor the config error, whose words
//     alone differ;
//   - the adding one passes: a key added to the JSON, at the top and deeper,
//     a message and a fix reworded and a report rewritten (T-076, decision
//     35), and a written file's comment lines changed, its YAML's keys
//     reordered and one added (T-112, decision 40); one with a key removed, a
//     value changed or an exit code changed, in the JSON or a written YAML
//     file, or a written Markdown file's text changed, fails, naming that case
//     alone;
//   - the broken one fails with no commit since the tag that says why, naming
//     the scenario, the case and the remedy, and no help case, its help's
//     words not judged; a fix and a !
//     before the tag do not count;
//   - it passes with a fix since the tag naming both in Changes: footers (the
//     scenario by its ID, the case by its file and name), and fails naming
//     the case alone when the fix names only the scenario;
//   - it passes with a fix naming the long case by a prefix of its name, short
//     enough for a footer line (T-081), and fails with one naming it by a
//     prefix another case starts with too, warning that it is ambiguous;
//   - it fails with a feat naming both, saying a feat's Changes: excuses one
//     only on a fix or a breaking change;
//   - a breaking change, a BREAKING-CHANGE: footer or a ! in a header,
//     excuses only what its Changes: names (T-106): one naming nothing fails,
//     naming both; one naming both passes; one naming the scenario alone
//     leaves the case refused;
//   - a Changes: entry that names nothing of the release is a warning, not a
//     failure;
//   - -range-from checks nothing when the range has no feat or fix; no
//     release tag passes, saying so; a shallow clone and a tag that cannot be
//     checked out stop it with exit 2, never a pass.
import { spawnSync } from "node:child_process";
import {
	chmodSync,
	mkdirSync,
	mkdtempSync,
	readFileSync,
	rmSync,
	symlinkSync,
	writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { finish, outsideEnv, releaseRepos } from "./scratch.ts";

const root = resolve(".");
const tmp = mkdtempSync(join(tmpdir(), "previous-release-selftest-"));
const repo = join(tmp, "repo");
const check = join(tmp, "previous-release");
const problems: string[] = [];
const expect = (ok: boolean, problem: string) => ok || problems.push(problem);

const { env, git, commit, untagged, shallow } = releaseRepos(tmp);
const write = (dir: string, files: Record<string, string>) => {
	for (const [path, text] of Object.entries(files)) {
		mkdirSync(dirname(join(dir, path)), { recursive: true });
		writeFileSync(join(dir, path), text);
	}
};

// A case whose name is longer than a footer line may be: "Changes:
// greet.yaml: <it>" is 126 characters, and the header lint caps a line at 100.
const longCase =
	"greets the world with hello on a line of its own, as it has since its first release and as scripts expect";

// The release: its scenarios, the steps that run them, its corpus and the
// runner, as this repository lays them out.
const release: Record<string, string> = {
	"go.mod": "module example.com/scratch\n\ngo 1.22\n",
	"package.json": '{ "name": "scratch", "version": "1.0.0", "type": "module" }\n',
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
	t.Run("It greets the world", func(t *testing.T) { say(t, "greet", "hello\\n") })
	t.Run("It says its name", func(t *testing.T) { say(t, "name", "itos\\n") })
}
`,
	"tools/itos/conformance/run.ts": readFileSync(
		join(root, "tools/itos/conformance/run.ts"),
		"utf8",
	),
	"tools/itos/conformance/greet.yaml": `cases:
  - name: ${longCase}
    argv: [greet]
    exit: 0
    stdout: "hello\\n"
  - name: greets no one when asked its name
    argv: [name]
    exit: 0
    stdout: "itos\\n"
  - name: says its name
    argv: [name]
    exit: 0
    stdout: "itos\\n"
  - name: says its version
    argv: [version]
    exit: 0
    stdout: "itos {{version}}\\n"
`,
	// A report's lines, which are for people, a JSON object, to which a new
	// binary may add and from which it may take nothing, its message and fix
	// for people too, and a YAML and a Markdown file it writes, whose data is
	// its contract and whose comments are for people (decision 40).
	"tools/itos/conformance/report.yaml": `cases:
  - name: reports three lines
    argv: [report]
    exit: 0
    stdout: "one\\ntwo\\nthree\\n"
  - name: says its state as JSON
    argv: [state]
    exit: 0
    json: { state: { ready: true, items: [a, b], message: all ready }, count: 2, fix: nothing to fix }
  - name: writes its policy and its notes
    argv: [write]
    exit: 0
    files_after:
      policy.yaml: "# The policy, for people.\\nname: scratch\\nchecks: [a, b] # both of them\\n"
      NOTES.md: "<!-- Written by itos. -->\\n# Notes\\n\\nready <!-- for now -->\\n"
`,
	// Refusals: two usage errors and a config error, judged as every case is,
	// by their exit codes alone, their words for people.
	"tools/itos/conformance/refuse.yaml": `cases:
  - name: an unknown command is a usage error
    argv: [wave]
    exit: 2
    stderr: "itos: unknown command: wave (itos --help)\\n"
  - name: greet takes no argument
    argv: [greet, loudly]
    exit: 2
    stdout: ""
    stderr: "itos: greet takes no argument (loudly) (itos --help)\\n"
  - name: a config that is not version 1
    argv: [check]
    exit: 2
    stderr: "FAIL itos.yaml: version 2 is not 1\\n"
`,
	// Help cases, whose text is for people: one asking with --help, one with
	// help <command>.
	"tools/itos/conformance/help.yaml": `cases:
  - name: itos --help
    argv: [--help]
    exit: 0
    stdout: "usage: itos greet|name|version\\n"
  - name: help greet is greet's --help
    argv: [help, greet]
    exit: 0
    stdout: "usage: itos greet|name|version\\n"
`,
};

// The binary the release was written for, one whose help and usage errors say
// more, one that also greets otherwise and fails doing it, and one that
// refuses otherwise: other words for every refusal, and exit 1 for greet's
// argument.
interface Says {
	greeting?: string;
	greeted?: number;
	help?: string;
	takes?: string;
	refusal?: number;
	config?: string;
	report?: string;
	reported?: number;
	state?: string;
	policy?: string;
	notes?: string;
}
const lines = "one\\ntwo\\nthree\\n";
const state =
	'{"count":2,"fix":"nothing to fix","state":{"ready":true,"items":["a","b"],"message":"all ready"}}';
const policy = "# The policy, for people.\\nname: scratch\\nchecks: [a, b] # both of them\\n";
const notes = "<!-- Written by itos. -->\\n# Notes\\n\\nready <!-- for now -->\\n";
const binary = ({
	greeting = "hello",
	greeted = 0,
	help = "usage: itos greet|name|version",
	takes = "",
	refusal = 2,
	config = "version 2 is not 1",
	report = lines,
	reported = 0,
	state: said = state,
	policy: wrote = policy,
	notes: noted = notes,
}: Says) => `#!/bin/sh
case "$1" in
--help | help) echo "${help}" ;;
report)
	printf '${report}'
	exit ${reported} ;;
state) echo '${said}' ;;
write)
	printf '${wrote}' >policy.yaml
	printf '${noted}' >NOTES.md ;;
greet)
	if [ $# -gt 1 ]; then
		echo "itos: greet takes no argument${takes} ($2) (itos --help)" >&2
		exit ${refusal}
	fi
	echo ${greeting}
	exit ${greeted} ;;
name) echo itos ;;
version) echo "itos 1.0.1-dev.3+gabcdef0" ;;
check)
	echo "FAIL itos.yaml: ${config}" >&2
	exit 2 ;;
*)
	echo "itos: unknown command: $1${takes} (itos --help)" >&2
	exit 2 ;;
esac
`;
const good = join(tmp, "good");
const wordy = join(tmp, "wordy");
const broken = join(tmp, "broken");
const reworded = join(tmp, "reworded");
const moreHelp = "usage: itos greet|name|version|wave";
const takes = "; the commands are greet, name and version";
writeFileSync(good, binary({}));
writeFileSync(wordy, binary({ help: moreHelp, takes }));
writeFileSync(broken, binary({ greeting: "bye", greeted: 1, help: moreHelp }));
writeFileSync(reworded, binary({ takes, refusal: 1, config: "version should be 1, not 2" }));
const adding = join(tmp, "adding");
writeFileSync(
	adding,
	binary({
		report: "1\\n3\\n2\\n",
		state:
			'{"count":2,"since":1,"fix":"none needed","state":{"ready":true,"items":["a","b"],"note":"x","message":"ready, all of it"}}',
		policy:
			"# Reworded, as people read it.\\nchecks:\\n  - a\\n  - b\\nname: scratch\\nsince: 1\\n",
		notes:
			"# Notes\\n<!-- itos wrote this;\\n  edit it freely. -->\\n\\nready <!-- until it is not -->\\n",
	}),
);
// Each takes something away: a key, a value, an exit code.
const taking = [
	{
		what: "a key removed",
		case: "says its state as JSON",
		says: {
			state:
				'{"fix":"nothing to fix","state":{"ready":true,"items":["a","b"],"message":"all ready"}}',
		},
	},
	{
		what: "a value changed",
		case: "says its state as JSON",
		says: { state: state.replace("true", "false") },
	},
	{ what: "an exit code changed", case: "reports three lines", says: { reported: 1 } },
	{
		what: "a written YAML file's value changed",
		case: "writes its policy and its notes",
		says: { policy: policy.replace("[a, b]", "[a, c]") },
	},
	{
		what: "a written YAML file's key removed",
		case: "writes its policy and its notes",
		says: { policy: policy.replace("name: scratch\\n", "") },
	},
	{
		what: "a written Markdown file's text changed",
		case: "writes its policy and its notes",
		says: { notes: notes.replace("ready", "not ready") },
	},
].map((t, i) => ({ ...t, bin: join(tmp, `taking-${i}`) }));
for (const t of taking) writeFileSync(t.bin, binary(t.says));
for (const bin of [good, wordy, broken, reworded, adding, ...taking.map((t) => t.bin)])
	chmodSync(bin, 0o755);

const run = (cwd: string, args: string[]) => {
	const r = spawnSync(check, args, { cwd, env, encoding: "utf8", maxBuffer: 64 * 1024 * 1024 });
	return { status: r.status, output: `${r.stdout}${r.stderr}` };
};

const scenario = "scenario @ID-GREET-01 (It greets the world)";
const corpusCase = `conformance case greet.yaml: ${longCase}`;

try {
	const built = spawnSync("go", ["build", "-o", check, "./tools/bin/previous-release"], {
		cwd: root,
		env: outsideEnv(),
		encoding: "utf8",
	});
	if (built.status !== 0)
		throw new Error(`go build ./tools/bin/previous-release:\n${built.stderr}`);

	// The scratch history: a fix naming the scenario and a breaking change
	// before the release, the release's tag, then a feat that says nothing;
	// branches add a commit that does.
	git(tmp, "init", "-q", "-b", "main", repo);
	write(repo, release);
	git(repo, "add", "-A");
	commit(repo, "build: the release's tree");
	commit(
		repo,
		"fix: an old fix\n\nWhy.\n\nChanges: @ID-GREET-01\nChanges: greet.yaml: greets the world",
	);
	commit(repo, "feat!: an old break, before the release");
	git(repo, "tag", "v1.0.0");
	const released = git(repo, "rev-parse", "HEAD");
	// The corpus runner's YAML parser, as the checkout's node_modules.
	symlinkSync(join(root, "node_modules"), join(repo, "node_modules"));
	commit(repo, "feat: greet otherwise\n\nNo word on the old scenario.\n\nTask: T-1");
	const feat = git(repo, "rev-parse", "HEAD");
	const branch = (name: string, message: string) => {
		git(repo, "checkout", "-q", "-b", name, feat);
		commit(repo, message);
	};
	branch(
		"fixed",
		`fix: greet otherwise\n\nThe old greeting was the bug.\n\nChanges: @ID-GREET-01\nChanges: greet.yaml: ${longCase}`,
	);
	// The long case by a prefix of its name that no other case starts with,
	// and by one the case asked its name starts with too.
	branch(
		"prefixed",
		"fix: greet otherwise\n\nThe old greeting was the bug.\n\nChanges: @ID-GREET-01\nChanges: greet.yaml: greets the world with hello",
	);
	branch(
		"ambiguous",
		"fix: greet otherwise\n\nThe old greeting was the bug.\n\nChanges: @ID-GREET-01\nChanges: greet.yaml: greets",
	);
	branch(
		"fixed-scenario",
		"fix: greet otherwise\n\nThe old greeting was the bug.\n\nChanges: @ID-GREET-01",
	);
	branch(
		"featured",
		"feat: greet otherwise\n\nA new greeting.\n\nChanges: @ID-GREET-01\nChanges: greet.yaml: greets the world",
	);
	branch("footer", "feat: greet otherwise\n\nBody.\n\nBREAKING-CHANGE: itos greet says bye");
	branch("bang", "fix!: greet otherwise");
	// Breaking changes that name what they change: a feat, so that only being
	// breaking can excuse it.
	branch(
		"footer-named",
		"feat: greet otherwise\n\nBody.\n\nBREAKING-CHANGE: itos greet says bye\nChanges: @ID-GREET-01\nChanges: greet.yaml: greets the world",
	);
	branch(
		"bang-named",
		"feat!: greet otherwise\n\nBody.\n\nChanges: @ID-GREET-01\nChanges: greet.yaml: greets the world",
	);
	branch(
		"footer-scenario",
		"feat: greet otherwise\n\nBody.\n\nBREAKING-CHANGE: itos greet says bye\nChanges: @ID-GREET-01",
	);
	branch(
		"unknown",
		"fix: something else\n\nBody.\n\nChanges: @ID-GREET-09, @ID-GREET-02\nChanges: greet.yaml: waves\nChanges: whatever",
	);
	git(repo, "checkout", "-q", "-b", "docs", released);
	commit(repo, "docs: say something");
	const on = (name: string) => git(repo, "checkout", "-q", name);

	// 1. The binary the release was written for passes, its own version the
	// corpus's {{version}}; what is left out of every case is said with its count.
	on("main");
	let r = run(repo, ["-bin", good]);
	const machine = "v1.0.0's conformance corpus: its 12 cases judged by itos's machine output alone";
	const leftOut = "the plain stdout and stderr of 10 cases and 2 keys named message or fix";
	expect(
		r.status === 0 &&
			r.output.includes("v1.0.0's scenarios: all 2 pass") &&
			r.output.includes("12/12 conformance cases pass") &&
			r.output.includes(machine) &&
			r.output.includes(leftOut) &&
			r.output.includes("docs/decisions/0035-"),
		`the release's own binary should pass every scenario and case, the corpus judged by machine output alone and what it leaves out counted, said, exited ${r.status}:\n${r.output}`,
	);

	// 1a. One whose help and usage errors' words differ, and nothing else,
	// passes with no commit that says why: their words are for people.
	r = run(repo, ["-bin", wordy]);
	expect(
		r.status === 0 &&
			r.output.includes(leftOut) &&
			r.output.includes("12/12 conformance cases pass") &&
			!r.output.includes("help.yaml: itos --help") &&
			!r.output.includes("fails"),
		`a binary whose help and usage errors' words alone differ should pass, exited ${r.status}:\n${r.output}`,
	);

	// 1b. One that rewords every refusal and changes one usage error's exit code
	// is refused, naming that case alone: the config error and the other usage
	// error differ only in words.
	r = run(repo, ["-bin", reworded]);
	expect(
		r.status === 1 &&
			r.output.includes("v1.0.0's conformance case refuse.yaml: greet takes no argument fails") &&
			r.output.includes("exit: expected 2, got 1") &&
			!r.output.includes("a config that is not version 1") &&
			!r.output.includes("an unknown command is a usage error") &&
			r.output.includes("11/12 conformance cases pass"),
		`a usage error whose exit code changed should be refused, and no case whose words alone differ named, exited ${r.status}:\n${r.output}`,
	);

	// 1c. One that adds keys to the JSON at the top and deeper, rewords its
	// message and fix, rewrites the report, and rewrites the comments of the
	// files it writes, reorders the YAML's keys and adds one, passes: the old
	// json is judged additively, less message and fix, plain output not at all,
	// and a written file by its data, its comment lines for people (decision 40).
	r = run(repo, ["-bin", adding]);
	expect(
		r.status === 0 &&
			r.output.includes("12/12 conformance cases pass") &&
			!r.output.includes("fails"),
		`a binary that adds keys to an old case's JSON, rewords its message and fix, rewrites its stdout and changes only the comments and the key order of the files it writes, adding a key, should pass, exited ${r.status}:\n${r.output}`,
	);
	// One that takes a key away, changes a value or changes an exit code, in
	// the JSON or in a file it writes, or changes a written Markdown file's
	// text, is refused, naming that case alone.
	for (const t of taking) {
		r = run(repo, ["-bin", t.bin]);
		const other = taking.find((o) => o.case !== t.case)!.case;
		expect(
			r.status === 1 &&
				r.output.includes(`v1.0.0's conformance case report.yaml: ${t.case}`) &&
				!r.output.includes(`report.yaml: ${other}`) &&
				r.output.includes("11/12 conformance cases pass"),
			`a binary with ${t.what} in an old case's output should be refused, naming that case alone, exited ${r.status}:\n${r.output}`,
		);
	}

	// 2. The broken binary, with no commit since the tag that says why: refused,
	// naming the scenario, the case and the remedy; the fix and the ! before the
	// tag do not count.
	r = run(repo, ["-bin", broken]);
	expect(
		r.status === 1 &&
			r.output.includes(`v1.0.0's ${scenario} fails against this tree's itos`) &&
			r.output.includes(`v1.0.0's ${corpusCase} fails against this tree's itos`) &&
			r.output.includes("Changes:") &&
			r.output.includes("BREAKING-CHANGE:") &&
			!r.output.includes("accepted"),
		`a broken old scenario and case with no commit saying why should be refused (exit 1), naming both and the remedy, exited ${r.status}:\n${r.output}`,
	);
	expect(
		r.output.includes("1 of 2 fail") &&
			!r.output.includes("says its name") &&
			r.output.includes("11/12 conformance cases pass") &&
			!r.output.includes("help.yaml: itos --help") &&
			!r.output.includes("help greet"),
		`only the scenario and the case the binary breaks should be named, no help case, exited ${r.status}:\n${r.output}`,
	);

	// 3. A fix since the tag naming both in Changes: passes, saying so.
	on("fixed");
	r = run(repo, ["-bin", broken]);
	expect(
		r.status === 0 &&
			r.output.includes(`${scenario} fails, accepted: the fix`) &&
			r.output.includes(`${corpusCase} fails, accepted: the fix`),
		`a fix naming the scenario and the case in Changes: should pass, exited ${r.status}:\n${r.output}`,
	);
	// One naming the case by a prefix of its name no other case starts with
	// passes; one naming it by a prefix two cases start with names neither,
	// leaving the case refused and warning that the entry is ambiguous.
	on("prefixed");
	r = run(repo, ["-bin", broken]);
	expect(
		r.status === 0 &&
			r.output.includes(`${corpusCase} fails, accepted: the fix`) &&
			!r.output.includes("warning"),
		`a fix naming the long case by a prefix of its name no other case starts with should pass, exited ${r.status}:\n${r.output}`,
	);
	on("ambiguous");
	r = run(repo, ["-bin", broken]);
	expect(
		r.status === 1 &&
			r.output.includes(`${scenario} fails, accepted: the fix`) &&
			r.output.includes(`${corpusCase} fails against this tree's itos`) &&
			r.output.includes(
				`warning: ${git(repo, "rev-parse", "HEAD").slice(0, 7)} "fix: greet otherwise" says "Changes: greet.yaml: greets"`,
			) &&
			r.output.includes('greet.yaml has 2 cases whose names start "greets"') &&
			r.output.includes("it is ambiguous, so it names none of them"),
		`a fix naming the case by a prefix two cases start with should leave it refused, warning that the entry is ambiguous, exited ${r.status}:\n${r.output}`,
	);
	// One naming the scenario alone leaves the case refused.
	on("fixed-scenario");
	r = run(repo, ["-bin", broken]);
	expect(
		r.status === 1 &&
			r.output.includes(`${scenario} fails, accepted: the fix`) &&
			r.output.includes(`${corpusCase} fails against this tree's itos`),
		`a fix naming only the scenario should leave the case refused, exited ${r.status}:\n${r.output}`,
	);

	// 4. A feat naming both is refused, naming each: a feat's Changes: excuses
	// one only on a fix or a breaking change (T-106).
	on("featured");
	r = run(repo, ["-bin", broken]);
	const onlyOn = "which excuses one only on a fix or a breaking change";
	expect(
		r.status === 1 &&
			r.output.includes(`${scenario} fails against this tree's itos, and only the feat`) &&
			r.output.includes(`${corpusCase} fails against this tree's itos, and only the feat`) &&
			r.output.includes(onlyOn) &&
			!r.output.includes("accepted"),
		`a feat naming the scenario and the case in Changes: should refuse both, saying a feat's Changes: excuses one only on a fix or a breaking change, exited ${r.status}:\n${r.output}`,
	);

	// 5. A breaking change since the tag, by a footer or by a ! in a header,
	// excuses only what its Changes: names (T-106). Naming nothing, it is
	// refused, naming the scenario, the case and the remedy.
	for (const name of ["footer", "bang"]) {
		on(name);
		r = run(repo, ["-bin", broken]);
		expect(
			r.status === 1 &&
				r.output.includes(`v1.0.0's ${scenario} fails against this tree's itos`) &&
				r.output.includes(`v1.0.0's ${corpusCase} fails against this tree's itos`) &&
				r.output.includes("being breaking excuses only what its Changes: names") &&
				!r.output.includes("accepted") &&
				!r.output.includes(onlyOn),
			`a breaking change (${name}) whose Changes: names nothing should refuse the broken scenario and case, exited ${r.status}:\n${r.output}`,
		);
	}
	// Naming both in Changes:, it passes, saying the breaking change names each.
	for (const name of ["footer-named", "bang-named"]) {
		on(name);
		r = run(repo, ["-bin", broken]);
		const named = `accepted: the breaking change ${git(repo, "rev-parse", "HEAD").slice(0, 7)} "`;
		expect(
			r.status === 0 &&
				r.output.includes(`${scenario} fails, ${named}`) &&
				r.output.includes(`${corpusCase} fails, ${named}`),
			`a breaking change (${name}) naming the scenario and the case in Changes: should accept both, exited ${r.status}:\n${r.output}`,
		);
	}
	// Naming the scenario alone, it leaves the case refused.
	on("footer-scenario");
	r = run(repo, ["-bin", broken]);
	expect(
		r.status === 1 &&
			r.output.includes(`${scenario} fails, accepted: the breaking change`) &&
			r.output.includes(`v1.0.0's ${corpusCase} fails against this tree's itos`),
		`a breaking change naming only the scenario in Changes: should leave the case refused, exited ${r.status}:\n${r.output}`,
	);

	// 6. Changes: entries that name nothing of the release are warnings.
	on("unknown");
	r = run(repo, ["-bin", good]);
	expect(
		r.status === 0 &&
			r.output.includes("@ID-GREET-09 is no scenario of v1.0.0") &&
			!r.output.includes("@ID-GREET-02 is no") &&
			r.output.includes('has no case "waves"') &&
			r.output.includes("neither scenario IDs"),
		`a Changes: entry naming nothing of the release should be a warning and pass, exited ${r.status}:\n${r.output}`,
	);

	// 7. -range-from: a range with no feat or fix checks nothing; one with a feat
	// checks.
	on("docs");
	r = run(repo, ["-range-from", released, "-bin", broken]);
	expect(
		r.status === 0 && r.output.includes("no feat or fix"),
		`a range of a docs commit should check nothing, exited ${r.status}:\n${r.output}`,
	);
	on("main");
	r = run(repo, ["-range-from", released, "-bin", broken]);
	expect(
		r.status === 1 && r.output.includes(scenario),
		`a range with a feat should check, and refuse the broken scenario, exited ${r.status}:\n${r.output}`,
	);

	// 8. No release tag: nothing to hold the tree to, said, and passed.
	r = run(untagged(), ["-bin", broken]);
	expect(
		r.status === 0 && r.output.includes("no vX.Y.Z tag"),
		`a repository with no release tag should pass, saying so, exited ${r.status}:\n${r.output}`,
	);

	// 9. A shallow clone, which may hide the tag, stops it.
	r = run(shallow(repo), ["-bin", broken]);
	expect(
		r.status === 2 && r.output.includes("shallow"),
		`a shallow clone should stop the check (exit 2), exited ${r.status}:\n${r.output}`,
	);

	// 10. A tag that cannot be checked out (its tree's objects missing) stops it.
	const corrupt = join(tmp, "corrupt");
	git(tmp, "init", "-q", "-b", "main", corrupt);
	write(corrupt, { "features/lost.feature": "Feature: only in the release\n" });
	git(corrupt, "add", "-A");
	commit(corrupt, "build: the release's tree");
	git(corrupt, "tag", "v1.0.0");
	const blob = git(corrupt, "rev-parse", "v1.0.0:features/lost.feature");
	git(corrupt, "rm", "-q", "-r", "features");
	commit(corrupt, "feat: drop it");
	rmSync(join(corrupt, ".git/objects", blob.slice(0, 2), blob.slice(2)));
	r = run(corrupt, ["-bin", good]);
	expect(
		r.status === 2 && r.output.includes("cannot check out v1.0.0"),
		`a tag that cannot be checked out should stop the check (exit 2), exited ${r.status}:\n${r.output}`,
	);
	expect(
		git(repo, "worktree", "list").split("\n").length === 1,
		`the check should leave no scratch worktree behind:\n${git(repo, "worktree", "list")}`,
	);
} catch (error) {
	problems.push((error as Error).message);
} finally {
	rmSync(tmp, { recursive: true, force: true });
}

finish(
	problems,
	"previous-release",
	"An old scenario or case the new binary breaks is refused, unless the Changes: footer of a fix or a breaking commit names it, a case by its name or a prefix no other case starts with; a feat's never does, nor a breaking commit's footer that does not name it; an old case is judged by machine output alone, its exit code, files_after and json less message and fix, a key added passing and one removed or changed refused, its plain output never; and a run that cannot check out the release never passes",
);
