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
//   - what the hooks leave out fails CI's own steps: a refactor that changes
//     what itos prints, in a module no unit test imports, passes both hooks and
//     fails the push's features step, whose smoke set reads that output.
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { namedTestsStep, planFor } from "../itos/ci-plan.ts";
import { STEPS, tasksIn } from "../itos/ci-scope.ts";
import { type Run, scratchRepo } from "./scratch.ts";

const repo = scratchRepo("gates-selftest");
const { root, env, sh, git, edit, commit } = repo;

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
	base = repo.open();
	// The audit's "new" is measured against the base, as it is against the
	// upstream branch in the checkout.
	env.FALLOW_AUDIT_BASE = base;

	const features = "go test ./features -count=1";
	for (const step of [features, "vp run test:coverage"])
		expect(STEPS.includes(step), `CI no longer runs \`${step}\`, which the hooks leave to it`);

	// 1. A change to a leaf module runs the tests that reach it and not the
	// whole suite, on both gates.
	const module = "tools/itos/moves.ts";
	const moduleTest = "tools/itos/moves.test.ts";
	const moduleLine = "const SCENARIO_LINE";
	edit(module, moduleLine, `// gates self-test: a harmless change\n${moduleLine}`);
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
	expect(!run.output.includes(features), "pre-push still runs the features for a refactor");

	// 2. The negative proof: the same file, broken, fails both gates, and fails
	// in the module's test alone.
	git(`reset -q --hard ${base}`);
	edit(module, "changes the live scenario ${id}", "alters the live scenario ${id}");
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
	sha = commit("chore: name a scenario and a task\n\nScenarios: @ID-SINCE-01\nTask: T-007");
	run = prePush("a push naming a scenario and a task", base, sha);
	expect(run.status === 0, `pre-push failed on a footed push:\n${run.output}`);
	expect(!run.output.includes(features), "pre-push ran the scenarios a footer names");
	expect(
		!run.output.includes("$ tools/bin/itos task"),
		"pre-push ran the checks of a task a footer names",
	);
	// ...and CI finds the task in the pushed range.
	expect(
		tasksIn(base, sha).includes("T-007"),
		`CI did not find T-007 in the pushed range: ${tasksIn(base, sha).join(", ") || "none"}`,
	);

	// The commit-msg hook, through its shim. A docs commit may not touch itos;
	// a test commit may not rename a live scenario; commitlint rejects a header
	// without a type; a docs commit with its footer passes.
	git(`reset -q --hard ${base}`);
	edit(module, moduleLine, `// gates self-test: a docs commit\n${moduleLine}`);
	run = commitMsg("a docs commit touching itos", "docs: touch the module\n\nTask: T-007\n");
	expect(
		run.status === 1 && run.output.includes(`docs commits may not touch ${module}`),
		`commit-msg did not reject a docs commit touching ${module}:\n${run.output}`,
	);
	git(`reset -q --hard ${base}`);
	edit(
		"features/since.feature",
		"Scenario: verify skips the commit commits.since names, and its ancestors",
		"Scenario: verify skips commits.since, renamed",
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

	// 5. What the hooks leave out, CI's steps catch. A refactor that changes
	// what verify prints, in a module no unit test imports, passes both hooks...
	git(`reset -q --hard ${base}`);
	edit("tools/itos/verify-commits.ts", "commits pass the commit rules", "commits pass");
	run = preCommit("a refactor that changes what itos prints");
	expect(run.status === 0, `pre-commit should not see the changed output:\n${run.output}`);
	sha = commit("refactor: shorten the summary");
	run = prePush("that refactor", base, sha);
	expect(run.status === 0, `pre-push should leave the features to CI:\n${run.output}`);
	// ...and fails a push's features step, whose smoke set reads that line.
	const step = namedTestsStep(planFor(base, sha)) ?? "";
	expect(
		step.startsWith(`${features} -scenarios=`),
		`a push's features step is not a selection: ${step}`,
	);
	run = gate("CI's features step for a push, same refactor", step);
	expect(
		run.status !== 0,
		"a push's features step passed a refactor that changes what itos prints",
	);
} finally {
	repo.remove();
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
