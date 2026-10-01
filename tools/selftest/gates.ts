// The local gates run only what a change affects, and CI still catches what
// they leave out. Run against the real hooks in a scratch worktree of the
// current tree (uncommitted edits included), so nothing here touches the
// checkout it was started from:
//
//   - pre-commit runs exactly the unit tests a change reaches, by import or by
//     vite.config.ts's forceRerunTriggers, and fails on a change that breaks one;
//   - pre-push does the same against the remote commit it builds on, fails on
//     that broken change, and passes a prose-only push;
//   - pre-push runs neither the scenarios a commit's `Scenarios:` footer names
//     nor the checks of the tasks its `Task:` footer names; CI reads those
//     footers from the pushed range and runs them;
//   - the commit-msg hook, a one-line shim calling `itos hook commit-msg`,
//     rejects a commit whose type may not touch a staged path, a scenario
//     renamed outside feat and fix, and a header commitlint rejects, and lets a
//     sound commit through;
//   - what the hooks leave out fails CI's own steps: a refactor that breaks
//     every scenario passes both hooks and fails the push's E2E step, and a
//     coverage threshold the tree misses passes pre-commit and fails
//     `vp run test:coverage`.
import { spawnSync } from "node:child_process";
import { copyFileSync, mkdirSync, mkdtempSync, readFileSync, rmSync, symlinkSync } from "node:fs";
import { writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { e2eStep, planFor } from "../itos/ci-plan.ts";
import { STEPS, tasksIn } from "../itos/ci-scope.ts";

const root = resolve(".");
const scratch = mkdtempSync(join(tmpdir(), "gates-selftest-"));
// Hooks export GIT_DIR and friends; the scratch tree must use its own.
const env = Object.fromEntries(
	Object.entries(process.env).filter(([k]) => !k.startsWith("GIT_") && k !== "CI"),
) as NodeJS.ProcessEnv;
Object.assign(env, {
	GIT_AUTHOR_NAME: "gates self-test",
	GIT_AUTHOR_EMAIL: "selftest@localhost",
	GIT_COMMITTER_NAME: "gates self-test",
	GIT_COMMITTER_EMAIL: "selftest@localhost",
	// A port of its own, in case a suite is running in the checkout.
	E2E_PORT: "5284",
});

interface Run {
	status: number;
	output: string;
	seconds: number;
}
function sh(command: string, input?: string, cwd = scratch): Run {
	const started = performance.now();
	const result = spawnSync("sh", ["-c", command], {
		cwd,
		env,
		input,
		encoding: "utf8",
		maxBuffer: 64 * 1024 * 1024,
	});
	return {
		status: result.status ?? 1,
		output: `${result.stdout}${result.stderr}`,
		seconds: (performance.now() - started) / 1000,
	};
}
const git = (command: string, cwd = scratch) => {
	const run = sh(`git ${command}`, undefined, cwd);
	if (run.status !== 0) throw new Error(`git ${command} failed:\n${run.output}`);
	return run.output.trim();
};

const edit = (file: string, from: string, to: string) => {
	const path = join(scratch, file);
	const text = readFileSync(path, "utf8");
	if (!text.includes(from)) throw new Error(`${file} no longer contains ${JSON.stringify(from)}`);
	writeFileSync(path, text.replace(from, to));
};
const commit = (message: string) => {
	git("add -A");
	const sha = git(`commit-tree ${git("write-tree")} -p HEAD -m "${message}"`);
	git(`reset -q --hard ${sha}`);
	return sha;
};

// How many unit test files a run executed (vitest's summary line; without a
// terminal it names only the files that failed), and the ones that failed.
const plain = (output: string) => output.replace(/\x1b\[[0-9;]*m/g, "");
const ranFiles = (output: string) => Number(/Test Files .*\((\d+)\)/.exec(plain(output))?.[1] ?? 0);
const failedFiles = (output: string) =>
	new Set(
		[...plain(output).matchAll(/FAIL\s+((?:src|tools)\/[\w./-]+\.test\.ts)/g)].map((m) => m[1]!),
	);
const allTests = sh("git ls-files '*.test.ts'", undefined, root).output.split("\n").filter(Boolean);

const problems: string[] = [];
const timings: string[] = [];
const expect = (ok: boolean, problem: string) => {
	if (!ok) problems.push(problem);
};
function gate(label: string, command: string, input?: string): Run {
	const run = sh(command, input);
	timings.push(
		`${label.padEnd(58)} ${run.status === 0 ? "pass" : "FAIL"}  ${run.seconds.toFixed(1)} s`,
	);
	return run;
}
const preCommit = (label: string) => {
	git("add -A");
	return gate(`pre-commit, ${label}`, "sh .vite-hooks/pre-commit");
};
const prePush = (label: string, base: string, sha: string) =>
	gate(
		`pre-push, ${label}`,
		"sh .vite-hooks/pre-push upstream git@example.invalid:upstream.git",
		`refs/heads/main ${sha} refs/heads/main ${base}\n`,
	);
const messages = mkdtempSync(join(tmpdir(), "gates-selftest-msg-"));
const commitMsg = (label: string, message: string) => {
	git("add -A");
	const file = join(messages, "COMMIT_EDITMSG");
	writeFileSync(file, message);
	return gate(`commit-msg, ${label}`, `sh .vite-hooks/commit-msg ${file}`);
};
const show = (files: Set<string>) => [...files].join(", ") || "none";

let base = "";
try {
	// The scratch tree is the current tree: HEAD plus every tracked edit and the
	// untracked files a gate could run, committed as the base the pushes build on.
	git(`worktree add -q --detach ${scratch} HEAD`, root);
	const diff = sh("git diff HEAD --binary", undefined, root).output;
	if (diff.trim()) {
		const applied = sh("git apply --whitespace=nowarn -", diff);
		if (applied.status !== 0)
			throw new Error(`could not copy the working tree:\n${applied.output}`);
	}
	for (const file of sh(
		"git ls-files --others --exclude-standard -- tools .vite-hooks src e2e",
		undefined,
		root,
	)
		.output.split("\n")
		.filter(Boolean)) {
		mkdirSync(dirname(join(scratch, file)), { recursive: true });
		copyFileSync(join(root, file), join(scratch, file));
	}
	symlinkSync(join(root, "node_modules"), join(scratch, "node_modules"));
	base = commit("gates self-test base");
	// The audit's "new" is measured against the base, as it is against the
	// upstream branch in the checkout.
	env.FALLOW_AUDIT_BASE = base;

	for (const step of ["vp run e2e", "vp run test:coverage"])
		expect(STEPS.includes(step), `CI no longer runs \`${step}\`, which the hooks leave to it`);

	// 1. A change to a leaf module runs the tests that reach it and not the
	// whole suite, on both gates.
	const module = "src/greeting.ts";
	const moduleTest = "src/greeting.test.ts";
	edit(
		module,
		"export function greeting",
		"// gates self-test: a harmless change\nexport function greeting",
	);
	let run = preCommit("a harmless change to a module");
	expect(run.status === 0, `pre-commit failed on a harmless change:\n${run.output}`);
	const reached = (n: number) => n >= 1 && n < allTests.length;
	expect(
		reached(ranFiles(run.output)),
		`pre-commit should run only the test files that reach ${module}, ran ${ranFiles(run.output)} of ${allTests.length}`,
	);
	let sha = commit("refactor: touch the module");
	run = prePush("that change as a refactor", base, sha);
	expect(run.status === 0, `pre-push failed on a harmless refactor:\n${run.output}`);
	expect(
		reached(ranFiles(run.output)),
		`pre-push should run only the test files that reach ${module}, ran ${ranFiles(run.output)} of ${allTests.length}`,
	);
	expect(!run.output.includes("$ vp run e2e"), "pre-push still runs the E2E suite for a refactor");

	// 2. The negative proof: the same file, broken, fails both gates, and fails
	// in the module's test alone.
	git(`reset -q --hard ${base}`);
	edit(module, "`Hello, ", "`Hi, ");
	run = preCommit("a change that breaks the module's test");
	expect(run.status !== 0, `pre-commit passed a change that breaks ${moduleTest}`);
	expect(
		reached(ranFiles(run.output)) &&
			failedFiles(run.output).size === 1 &&
			failedFiles(run.output).has(moduleTest),
		`pre-commit should fail in ${moduleTest} alone, failed in ${show(failedFiles(run.output))}`,
	);
	sha = commit("refactor: break the module");
	run = prePush("that change as a refactor", base, sha);
	expect(run.status !== 0, `pre-push passed a change that breaks ${moduleTest}`);
	expect(failedFiles(run.output).has(moduleTest), `pre-push did not fail in ${moduleTest}`);

	// 3. What the tests read from disk counts as a change they depend on
	// (forceRerunTriggers: the task tool's config reruns the whole suite).
	git(`reset -q --hard ${base}`);
	edit("itos.yaml", "version: 1", "# gates self-test: a harmless change\nversion: 1");
	run = preCommit("a change to itos.yaml");
	expect(run.status === 0, `pre-commit failed on a comment in itos.yaml:\n${run.output}`);
	expect(
		ranFiles(run.output) === allTests.length,
		`a change to itos.yaml, which the tests read from disk, should rerun all ${allTests.length} test files; ran ${ranFiles(run.output)}`,
	);

	// 4. A push that touches only prose passes pre-push.
	git(`reset -q --hard ${base}`);
	edit("README.md", "# ", "A prose edit.\n\n# ");
	sha = commit("docs: edit the readme");
	run = prePush("a prose-only push", base, sha);
	expect(run.status === 0, `pre-push failed on a prose-only push:\n${run.output}`);

	// A commit that names a scenario and a task leaves both to CI...
	git(`reset -q --hard ${base}`);
	edit("README.md", "# ", "A footed edit.\n\n# ");
	sha = commit("chore: name a scenario and a task\n\nScenarios: @ID-APP-01\nTask: T-007");
	run = prePush("a push naming a scenario and a task", base, sha);
	expect(run.status === 0, `pre-push failed on a footed push:\n${run.output}`);
	expect(!run.output.includes("$ vp run e2e"), "pre-push ran the scenarios a footer names");
	expect(!run.output.includes("$ vp run task"), "pre-push ran the checks of a task a footer names");
	// ...and CI finds the task in the pushed range.
	expect(
		tasksIn(base, sha).includes("T-007"),
		`CI did not find T-007 in the pushed range: ${tasksIn(base, sha).join(", ") || "none"}`,
	);

	// The commit-msg hook, through its shim. A docs commit may not touch src/;
	// a test commit may not rename a live scenario; commitlint rejects a header
	// without a type; a docs commit with its footer passes.
	git(`reset -q --hard ${base}`);
	edit(
		module,
		"export function greeting",
		"// gates self-test: a docs commit\nexport function greeting",
	);
	run = commitMsg("a docs commit touching src/", "docs: touch the module\n\nTask: T-007\n");
	expect(
		run.status === 1 && run.output.includes(`docs commits may not touch ${module}`),
		`commit-msg did not reject a docs commit touching ${module}:\n${run.output}`,
	);
	git(`reset -q --hard ${base}`);
	edit(
		"e2e/features/app.feature",
		"Scenario: The page opens and greets the visitor by name",
		"Scenario: The page opens, renamed",
	);
	run = commitMsg("a test commit renaming a scenario", "test: rename a scenario\n\nTask: T-007\n");
	expect(
		run.status === 1 && run.output.includes("  - a test commit "),
		`commit-msg did not reject a test commit renaming a live scenario:\n${run.output}`,
	);
	git(`reset -q --hard ${base}`);
	edit("README.md", "# ", "A message check.\n\n# ");
	run = commitMsg("a header without a type", "update things\n");
	expect(
		run.status !== 0 && run.output.includes("[type-empty]"),
		`commit-msg did not pass the header to commitlint:\n${run.output}`,
	);
	run = commitMsg("a sound docs commit", "docs: edit the readme\n\nTask: T-007\n");
	expect(run.status === 0, `commit-msg rejected a sound docs commit:\n${run.output}`);

	// 5. What the hooks leave out, CI's steps catch. A refactor that changes the
	// page's heading, which every scenario reads, passes both hooks...
	git(`reset -q --hard ${base}`);
	edit("src/main.ts", '"h1"', '"h2"');
	run = preCommit("a refactor that breaks every scenario");
	expect(run.status === 0, `pre-commit should not see a broken scenario:\n${run.output}`);
	sha = commit("refactor: change the heading");
	run = prePush("that refactor", base, sha);
	expect(run.status === 0, `pre-push should leave the scenarios to CI:\n${run.output}`);
	// ...and fails a push's E2E step, the smoke set among it.
	const step = e2eStep(planFor(base, sha)) ?? "";
	expect(step.startsWith("vp run e2e --grep"), `a push's E2E step is not a selection: ${step}`);
	run = gate("CI's E2E step for a push, same refactor", step);
	expect(run.status !== 0, "a push's E2E step passed a refactor that breaks every scenario");

	// A coverage threshold the tree misses passes pre-commit (a partial run
	// holds no threshold) and fails CI's coverage step.
	git(`reset -q --hard ${base}`);
	edit(
		module,
		"const who = name?.trim();",
		'const who = name?.trim();\n\tif (who === "gates self-test") return who;',
	);
	run = preCommit("a branch no test covers");
	expect(run.status === 0, `pre-commit should leave the thresholds to CI:\n${run.output}`);
	run = gate("CI step `vp run test:coverage`, same branch", "vp run test:coverage");
	expect(run.status !== 0, "`vp run test:coverage` passed a tree that misses its thresholds");
} finally {
	sh(`git worktree remove --force ${scratch}`, undefined, root);
	rmSync(scratch, { recursive: true, force: true });
	rmSync(messages, { recursive: true, force: true });
}

console.log(timings.join("\n"));
for (const problem of problems) console.error(`FAIL ${problem}`);
console.log(
	problems.length
		? `\n${problems.length} gate check(s) failed`
		: `\nThe hooks run what a change affects (${allTests.length} unit test files in all); CI catches the rest`,
);
process.exit(problems.length ? 1 : 0);
