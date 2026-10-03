// The local gates leave to CI what they do not run, and CI catches it. Run
// against the real hooks in a scratch worktree of the current tree
// (uncommitted edits included), so nothing here touches the checkout it was
// started from:
//
//   - CI runs the whole unit suite and the features, which the hooks leave to it;
//   - pre-push passes a prose-only push;
//   - pre-push runs neither the scenarios a commit's `Scenarios:` footer names
//     nor the checks of the tasks its `Task:` footer names; CI reads those
//     footers from the pushed range and runs them;
//   - the commit-msg hook, a one-line shim calling `itos hook commit-msg`,
//     rejects a commit whose type may not touch a staged path, a scenario
//     renamed outside feat and fix, and a header without a type, by the
//     built-in header lint with no node_modules in the scratch copy, so no
//     commitlint (T-063), and lets a sound commit through there too;
//   - what the hooks leave out fails CI's own steps: a refactor that changes
//     what itos prints, in a Go package whose unit tests do not read that
//     line, passes both hooks and fails the push's features step, whose smoke
//     set reads that output from the scratch copy's tools/bin/itos, rebuilt
//     from the changed source.
//
// The plan and the steps are the scratch copy's `tools/bin/itos ci plan --json`.
// That the hooks run exactly the unit tests a change reaches, and fail on a
// change that breaks one, is tools/selftest/go-hooks.ts's (T-059), which the
// nightly runs beside this: since the TypeScript left (T-062) the unit tests
// are the Go packages', and tools/bin/go-unit-tests picks them for both hooks.
import { mkdtempSync, rmSync, symlinkSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { ciPlan, featuresStep, hookGates, scratchRepo } from "./scratch.ts";

const repo = scratchRepo("gates-selftest");
const { dir, env, git, edit, commit } = repo;

const { problems, timings, expect, gate, preCommit, prePush } = hookGates(repo);
const messages = mkdtempSync(join(tmpdir(), "gates-selftest-msg-"));
const commitMsg = (label: string, message: string) => {
	git("add -A");
	const file = join(messages, "COMMIT_EDITMSG");
	writeFileSync(file, message);
	return gate(`commit-msg, ${label}`, `sh .vite-hooks/commit-msg ${file}`);
};

let base = "";
try {
	base = repo.open();
	// The audit's "new" is measured against the base, as it is against the
	// upstream branch in the checkout.
	env.FALLOW_AUDIT_BASE = base;

	const features = "go test ./features -count=1";
	const steps = ciPlan(["--whole"], dir).steps;
	for (const step of [features, "go test ./cmd/... ./internal/..."])
		expect(steps.includes(step), `CI no longer runs \`${step}\`, which the hooks leave to it`);

	// A push that touches only prose passes pre-push.
	git(`reset -q --hard ${base}`);
	edit("README.md", "# ", "A prose edit.\n\n# ");
	let sha = commit("docs: edit the readme");
	let run = prePush("a prose-only push", base, sha);
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
	const named = ciPlan([base, sha], dir).tasks;
	expect(
		named.includes("T-007"),
		`CI did not find T-007 in the pushed range: ${named.join(", ") || "none"}`,
	);

	// The commit-msg hook, through its shim. A docs commit may not touch itos;
	// a test commit may not rename a live scenario; the header lint rejects a
	// header without a type; a docs commit with its footer passes.
	git(`reset -q --hard ${base}`);
	const goSource = "internal/version/version.go";
	edit(goSource, "\npackage version\n", "\n// gates self-test: a docs commit\npackage version\n");
	run = commitMsg("a docs commit touching itos", "docs: touch the package\n\nTask: T-007\n");
	expect(
		run.status === 1 && run.output.includes(`docs commits may not touch ${goSource}`),
		`commit-msg did not reject a docs commit touching ${goSource}:\n${run.output}`,
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
	// The header lint is the built-in one: with node_modules gone from the
	// scratch copy, nothing of commitlint's can run, and the header is still
	// judged, by commitlint's rule and words.
	git(`reset -q --hard ${base}`);
	edit("README.md", "# ", "A message check.\n\n# ");
	const modules = join(dir, "node_modules");
	rmSync(modules);
	run = commitMsg("a header without a type, no node_modules", "update things\n");
	expect(
		run.status === 1 && run.output.includes("✖   type may not be empty [type-empty]"),
		`commit-msg did not reject a header without a type with no node_modules:\n${run.output}`,
	);
	run = commitMsg("a sound docs commit, no node_modules", "docs: edit the readme\n\nTask: T-007\n");
	expect(run.status === 0, `commit-msg rejected a sound docs commit:\n${run.output}`);
	symlinkSync(join(repo.root, "node_modules"), modules);

	// What the hooks leave out, CI's steps catch. A refactor that changes
	// what verify prints, in a Go package whose unit tests do not read that
	// line, passes both hooks...
	git(`reset -q --hard ${base}`);
	edit("internal/cli/verify.go", "commits pass the commit rules", "commits pass");
	run = preCommit("a refactor that changes what itos prints");
	expect(run.status === 0, `pre-commit should not see the changed output:\n${run.output}`);
	sha = commit("refactor: shorten the summary");
	run = prePush("that refactor", base, sha);
	expect(run.status === 0, `pre-push should leave the features to CI:\n${run.output}`);
	// ...and fails a push's features step, whose smoke set reads that line.
	const step = featuresStep(ciPlan([base, sha], dir)) ?? "";
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
		: "\nThe hooks leave the features and the whole unit suite to CI, and CI catches what they let through",
);
process.exit(problems.length ? 1 : 0);
