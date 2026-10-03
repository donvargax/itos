// The last release's scenarios and corpus against the new binary (T-071,
// tools/bin/previous-release), in a scratch repository whose v1.0.0 tag holds
// a release of its own: two scenarios (go test ./features, a black box running
// whatever ITOS_BIN names, as this repository's are), and a corpus of three
// cases run by this repository's own conformance runner. The binary under
// test is a script; the broken one breaks one scenario and one case, and says
// a version the tag never had, as T-069's builds will.
//
//   - the binary the release was written for passes, its version the
//     corpus's {{version}} whatever the tag's package.json says;
//   - the broken one fails with no commit since the tag that says why, naming
//     the scenario, the case and the remedy; a fix and a ! before the tag do
//     not count;
//   - it passes with a fix since the tag naming both in Changes: footers (the
//     scenario by its ID, the case by its file and name), and fails naming
//     the case alone when the fix names only the scenario;
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
  - name: greets the world
    argv: [greet]
    exit: 0
    stdout: "hello\\n"
  - name: says its name
    argv: [name]
    exit: 0
    stdout: "itos\\n"
  - name: says its version
    argv: [version]
    exit: 0
    stdout: "itos {{version}}\\n"
`,
};

// The binary the release was written for, and one that greets otherwise.
const binary = (greeting: string) => `#!/bin/sh
case "$1" in
greet) echo ${greeting} ;;
name) echo itos ;;
version) echo "itos 1.0.1-dev.3+gabcdef0" ;;
*) exit 2 ;;
esac
`;
const good = join(tmp, "good");
const broken = join(tmp, "broken");
writeFileSync(good, binary("hello"));
writeFileSync(broken, binary("bye"));
chmodSync(good, 0o755);
chmodSync(broken, 0o755);

const run = (cwd: string, args: string[]) => {
	const r = spawnSync(check, args, { cwd, env, encoding: "utf8", maxBuffer: 64 * 1024 * 1024 });
	return { status: r.status, output: `${r.stdout}${r.stderr}` };
};

const scenario = "scenario @ID-GREET-01 (It greets the world)";
const corpusCase = "conformance case greet.yaml: greets the world";

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
		"fix: greet otherwise\n\nThe old greeting was the bug.\n\nChanges: @ID-GREET-01\nChanges: greet.yaml: greets the world",
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
	// corpus's {{version}}.
	on("main");
	let r = run(repo, ["-bin", good]);
	expect(
		r.status === 0 &&
			r.output.includes("v1.0.0's scenarios: all 2 pass") &&
			r.output.includes("3/3 conformance cases pass"),
		`the release's own binary should pass every scenario and case, exited ${r.status}:\n${r.output}`,
	);

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
		r.output.includes("1 of 2 fail") && !r.output.includes("says its name"),
		`only the scenario the binary breaks should be named, exited ${r.status}:\n${r.output}`,
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
	"An old scenario or case the new binary breaks is refused, unless a breaking change or a fix's Changes: footer says why; a feat's never does, and a run that cannot check out the release never passes",
);
