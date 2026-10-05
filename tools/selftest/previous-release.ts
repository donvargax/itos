// The last release's scenarios and corpus against the new binary (T-071,
// tools/bin/previous-release), in a scratch repository whose v1.0.0 tag holds
// a release of its own: two scenarios (go test ./features, a black box running
// whatever ITOS_BIN names, as this repository's are), and a corpus of four
// cases and two help cases run by this repository's own conformance runner;
// two of the four start with the same word, and one's name is longer than a
// footer line may be.
// The binary under test is a script; the broken one breaks one scenario, one
// case and both help cases, the wordy one only the help cases, and each says
// a version the tag never had, as T-069's builds will. Three more cases
// are refusals: two usage errors and a config error; the reworded binary says
// other words for all three and changes one usage error's exit code. Two more
// pin a report's lines and a JSON object: the adding binary adds lines to the
// one and keys to the other, and four more each break one of them.
//
//   - the binary the release was written for passes, its version the
//     corpus's {{version}} whatever the tag's package.json says, and the help
//     cases are left out, said with their count;
//   - the wordy one passes: help text is documentation, not compatibility, so
//     an old help case that differs is never judged; nor is an old usage
//     error, which pins only what a command refuses (T-095), so both usage
//     errors are left out, said with their count;
//   - the reworded one fails naming the config error whose words differ, and
//     neither usage error, though one's words differ and the other's exit
//     code;
//   - the adding one passes, the old corpus judged additively (T-076): a key
//     added to the JSON, at the top and deeper, and lines added between and
//     after the report's pass; one with a key removed, a value changed, a line
//     changed or two lines swapped fails, naming that case alone;
//   - the broken one fails with no commit since the tag that says why, naming
//     the scenario, the case and the remedy, and no help case; a fix and a !
//     before the tag do not count;
//   - it passes with a fix since the tag naming both in Changes: footers (the
//     scenario by its ID, the case by its file and name), and fails naming
//     the case alone when the fix names only the scenario;
//   - it passes with a fix naming the long case by a prefix of its name, short
//     enough for a footer line (T-081), and fails with one naming it by a
//     prefix another case starts with too, warning that it is ambiguous;
//   - it fails with a feat naming both, saying a feat's Changes: never
//     excuses one;
//   - it passes with a breaking change: a BREAKING-CHANGE: footer, a ! in a
//     header;
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
	// A report's lines and a JSON object, to which a new binary may add, and
	// from which it may take nothing.
	"tools/itos/conformance/report.yaml": `cases:
  - name: reports three lines
    argv: [report]
    exit: 0
    stdout: "one\\ntwo\\nthree\\n"
  - name: says its state as JSON
    argv: [state]
    exit: 0
    json: { state: { ready: true, items: [a, b] }, count: 2 }
`,
	// Refusals: two usage errors, never judged, since refusing less is additive
	// (T-095), and a config error, whose words the new binary may not change.
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
	// Help cases, whose text the new binary may change freely: one asking with
	// --help, one with help <command>.
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
// more, one that also greets otherwise, and one that refuses otherwise: other
// words for every refusal, and exit 1 for greet's argument.
interface Says {
	greeting?: string;
	help?: string;
	takes?: string;
	refusal?: number;
	config?: string;
	report?: string;
	state?: string;
}
const lines = "one\\ntwo\\nthree\\n";
const state = '{"count":2,"state":{"ready":true,"items":["a","b"]}}';
const binary = ({
	greeting = "hello",
	help = "usage: itos greet|name|version",
	takes = "",
	refusal = 2,
	config = "version 2 is not 1",
	report = lines,
	state: said = state,
}: Says) => `#!/bin/sh
case "$1" in
--help | help) echo "${help}" ;;
report) printf '${report}' ;;
state) echo '${said}' ;;
greet)
	if [ $# -gt 1 ]; then
		echo "itos: greet takes no argument${takes} ($2) (itos --help)" >&2
		exit ${refusal}
	fi
	echo ${greeting} ;;
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
writeFileSync(broken, binary({ greeting: "bye", help: moreHelp }));
writeFileSync(reworded, binary({ takes, refusal: 1, config: "version should be 1, not 2" }));
const adding = join(tmp, "adding");
writeFileSync(
	adding,
	binary({
		report: "one\\ntwo\\nand a half\\nthree\\nfour\\n",
		state: '{"count":2,"since":1,"state":{"ready":true,"items":["a","b"],"note":"x"}}',
	}),
);
// Each takes something away: a key, a value, a line, an order.
const taking = [
	{
		what: "a key removed",
		case: "says its state as JSON",
		says: { state: '{"state":{"ready":true,"items":["a","b"]}}' },
	},
	{
		what: "a value changed",
		case: "says its state as JSON",
		says: { state: state.replace("true", "false") },
	},
	{ what: "a line changed", case: "reports three lines", says: { report: "one\\n2\\nthree\\n" } },
	{
		what: "two lines swapped",
		case: "reports three lines",
		says: { report: "one\\nthree\\ntwo\\n" },
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
	branch(
		"unknown",
		"fix: something else\n\nBody.\n\nChanges: @ID-GREET-09, @ID-GREET-02\nChanges: greet.yaml: waves\nChanges: whatever",
	);
	git(repo, "checkout", "-q", "-b", "docs", released);
	commit(repo, "docs: say something");
	const on = (name: string) => git(repo, "checkout", "-q", name);

	// 1. The binary the release was written for passes, its own version the
	// corpus's {{version}}; the help cases are left out, said with their count.
	on("main");
	let r = run(repo, ["-bin", good]);
	const leftOut = "2 help cases (help.yaml 2) not judged, since help text is documentation";
	const relaxed = "2 usage errors (refuse.yaml 2) not judged, since a usage error pins only";
	const additively = "v1.0.0's conformance corpus: judged additively";
	expect(
		r.status === 0 &&
			r.output.includes("v1.0.0's scenarios: all 2 pass") &&
			r.output.includes("7/7 conformance cases pass") &&
			r.output.includes(leftOut) &&
			r.output.includes(relaxed) &&
			r.output.includes(additively),
		`the release's own binary should pass every scenario and case, the help cases and the usage errors left out, both counted, and the corpus judged additively, said, exited ${r.status}:\n${r.output}`,
	);

	// 1a. One whose help and usage errors' words differ, and nothing else,
	// passes with no commit that says why: an old help case is never judged,
	// nor an old usage error.
	r = run(repo, ["-bin", wordy]);
	expect(
		r.status === 0 &&
			r.output.includes(leftOut) &&
			r.output.includes(relaxed) &&
			r.output.includes("7/7 conformance cases pass") &&
			!r.output.includes("help.yaml: itos --help") &&
			!r.output.includes("fails"),
		`a binary whose help and usage errors' words alone differ should pass, its old help cases and usage errors not judged, exited ${r.status}:\n${r.output}`,
	);

	// 1b. One whose config error's words differ is refused, naming that case
	// alone: neither usage error, though one's words differ and the other's
	// exit code too, since an old usage error is never judged.
	r = run(repo, ["-bin", reworded]);
	expect(
		r.status === 1 &&
			r.output.includes(
				"v1.0.0's conformance case refuse.yaml: a config that is not version 1 fails",
			) &&
			r.output.includes("- FAIL itos.yaml: version 2 is not 1") &&
			!r.output.includes("greet takes no argument") &&
			!r.output.includes("an unknown command is a usage error") &&
			r.output.includes("6/7 conformance cases pass"),
		`a config error whose words differ should be refused, and neither usage error named, whatever its words or exit code, exited ${r.status}:\n${r.output}`,
	);

	// 1c. One that only adds, keys to the JSON at the top and deeper, lines to
	// the report between and after its own, passes: the old corpus is judged
	// additively.
	r = run(repo, ["-bin", adding]);
	expect(
		r.status === 0 &&
			r.output.includes("7/7 conformance cases pass") &&
			!r.output.includes("fails"),
		`a binary that only adds keys to an old case's JSON and lines to its stdout should pass, exited ${r.status}:\n${r.output}`,
	);
	// One that takes a key away, changes a value, changes a line or swaps two
	// lines is refused, naming that case alone.
	for (const t of taking) {
		r = run(repo, ["-bin", t.bin]);
		const other = taking.find((o) => o.case !== t.case)!.case;
		expect(
			r.status === 1 &&
				r.output.includes(`v1.0.0's conformance case report.yaml: ${t.case}`) &&
				!r.output.includes(`report.yaml: ${other}`) &&
				r.output.includes("6/7 conformance cases pass"),
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
			r.output.includes("6/7 conformance cases pass") &&
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

	// 4. A feat naming both is refused: a feat's Changes: never excuses one.
	on("featured");
	r = run(repo, ["-bin", broken]);
	expect(
		r.status === 1 &&
			r.output.includes(`${scenario} fails against this tree's itos, and only the feat`) &&
			r.output.includes("never excuses one") &&
			!r.output.includes("accepted"),
		`a feat naming the scenario in Changes: should be refused, saying a feat never excuses one, exited ${r.status}:\n${r.output}`,
	);

	// 5. A breaking change since the tag passes: a footer, a ! in a header.
	for (const name of ["footer", "bang"]) {
		on(name);
		r = run(repo, ["-bin", broken]);
		expect(
			r.status === 0 &&
				r.output.includes(`${scenario} fails, accepted:`) &&
				r.output.includes("marks a breaking change"),
			`a breaking change (${name}) should accept the broken scenario and case, exited ${r.status}:\n${r.output}`,
		);
	}

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
	"An old scenario or case the new binary breaks is refused, unless a breaking change or a fix's Changes: footer says why, naming a case by its name or a prefix no other case starts with; a feat's never does; an old case's output is judged additively, a key or a line added passing and one removed, changed or reordered refused; an old help case or usage error is never judged, an old config error word for word; and a run that cannot check out the release never passes",
);
