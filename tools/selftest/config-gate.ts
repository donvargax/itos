// itos's own data, the config, the ledger, the work registry and the smoke
// set, which every later gate reads, is checked at commit by the commit-msg
// hook, from the staged tree, and in CI by `itos config check`. Run against the
// real hooks and the real plan in a scratch worktree of the current tree
// (uncommitted edits included), so nothing here touches the checkout it was
// started from:
//
//   - a commit staging a ledger with a misspelt key is rejected, by the
//     commit-msg hook, and a sound ledger edit goes through;
//   - a commit staging only docs/** and tasks/** (the ledger and the registry)
//     runs no unit tests: the pre-commit hook's prose exit skips them;
//   - CI's plan runs the check for a range that touches the registry
//     (tasks/work-items.yaml, beside the ledger, so not prose), for one that
//     touches the ledger, and for a prose-only range that touches
//     CONTRIBUTORS.md, the people the registry's owners must be among.
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { scratchRepo } from "./scratch.ts";

const check = "tools/bin/itos config check";
const repo = scratchRepo("config-gate-selftest");
const { dir, sh, git, edit, commit } = repo;
// A commit as git makes one, without making it: the files staged, the
// pre-commit hook, then the commit-msg hook on the message. Which hook stopped
// it, if one did, and what the hooks printed.
const messages = mkdtempSync(join(tmpdir(), "config-gate-selftest-msg-"));
const attempt = (files: string[], message: string) => {
	git(`add -- ${files.join(" ")}`);
	const preCommit = sh("sh .vite-hooks/pre-commit");
	if (preCommit.status !== 0) return { stopped: "pre-commit", output: preCommit.output };
	const file = join(messages, "COMMIT_EDITMSG");
	writeFileSync(file, message);
	const commitMsg = sh(`sh .vite-hooks/commit-msg ${file}`);
	return {
		stopped: commitMsg.status !== 0 ? "commit-msg" : "",
		output: `${preCommit.output}${commitMsg.output}`,
	};
};
// Whether the pre-commit hook went past its prose exit to the unit tests: the
// scratch copy's tools/bin/go-unit-tests, which picks them, is replaced by one
// that says it ran whatever is staged (the real one is silent when no Go file
// is, so its silence would prove nothing).
const RAN = "config gate self-test: the unit tests ran";
const ranTests = (output: string) => output.includes(RAN);
// Whether CI's plan for a range runs the check, and whether it read the range
// as prose-only.
const plan = (from: string, to: string) => {
	const run = sh(`tools/bin/itos ci plan ${from} ${to} --json`);
	if (run.status !== 0) throw new Error(`itos ci plan failed:\n${run.output}`);
	const parsed = JSON.parse(run.output) as { prose: boolean; order: { step?: string }[] };
	return { prose: parsed.prose, checks: parsed.order.some((o) => o.step === check) };
};

const prepend = (file: string, text: string) =>
	writeFileSync(join(dir, file), text + readFileSync(join(dir, file), "utf8"));

const problems: string[] = [];
const expect = (ok: boolean, problem: string) => {
	if (!ok) problems.push(problem);
};

try {
	const base = repo.open();

	// 1. A misspelt key in the ledger: the commit-msg hook stops the commit,
	// with the check's own word for why. The pre-commit hook no longer reads
	// the ledger.
	const ledger = "tasks/phase-1.yaml";
	const title = "  title: itos config check in the gates";
	const message = "docs: edit the ledger\n\nTask: T-023\n";
	edit(ledger, title, title.replace("title:", "titel:"));
	let run = attempt([ledger], message);
	expect(
		run.stopped === "commit-msg" && run.output.includes("unknown key titel"),
		`a ledger with a misspelt key was not rejected by the commit-msg hook (stopped by ${run.stopped || "nothing"}):\n${run.output}`,
	);

	// 2. A sound edit to the same ledger goes through.
	git(`reset -q --hard ${base}`);
	edit(ledger, title, `  # config gate self-test: a sound edit\n${title}`);
	run = attempt([ledger], message);
	expect(run.stopped === "", `a sound ledger edit was rejected by ${run.stopped}:\n${run.output}`);

	// 3. A commit staging only docs/** (not Markdown) and tasks/** runs no unit
	// tests: neither is anything a test reaches.
	git(`reset -q --hard ${base}`);
	writeFileSync(join(dir, "tools/bin/go-unit-tests"), `#!/bin/sh\necho "${RAN}"\n`);
	const notes = "docs/config-gate-selftest.yaml";
	writeFileSync(join(dir, notes), "note: a file under docs/ that is not Markdown\n");
	const registry = "tasks/work-items.yaml";
	prepend(registry, "# config gate self-test: a registry edit\n");
	run = attempt([notes, registry], "docs: edit the notes and the registry\n\nTask: T-023\n");
	expect(run.stopped === "", `a commit of ${notes} and ${registry} was rejected:\n${run.output}`);
	expect(
		!ranTests(run.output),
		`the pre-commit hook ran the unit tests for a commit of only ${notes} and ${registry}:\n${run.output}`,
	);

	// 4. A range that touches only the registry runs the check in CI. The
	// registry is under tasks/, beside the ledger, so the range is not prose.
	git(`reset -q --hard ${base}`);
	prepend(registry, "# config gate self-test: a registry edit\n");
	let range = plan(base, commit("docs: edit the registry"));
	expect(!range.prose, `a range touching only ${registry} is read as prose-only`);
	expect(range.checks, `CI's plan for a range touching ${registry} skips \`${check}\``);

	// 5. So does a range that touches the ledger, which is not prose.
	git(`reset -q --hard ${base}`);
	edit(ledger, title, `  # config gate self-test: a ledger edit\n${title}`);
	range = plan(base, commit("docs: edit the ledger"));
	expect(!range.prose, `a range touching ${ledger} is read as prose-only`);
	expect(range.checks, `CI's plan for a range touching ${ledger} skips \`${check}\``);

	// 6. And a prose-only range that touches the people, whom a registry's
	// owners must be among.
	git(`reset -q --hard ${base}`);
	const people = "CONTRIBUTORS.md";
	prepend(people, "<!-- config gate self-test: a prose edit -->\n");
	range = plan(base, commit("docs: edit the people"));
	expect(range.prose, `a range touching only ${people} is no longer read as prose-only`);
	expect(range.checks, `CI's plan for a prose-only range touching ${people} skips \`${check}\``);
} finally {
	repo.remove();
	rmSync(messages, { recursive: true, force: true });
}

for (const problem of problems) console.error(`FAIL ${problem}`);
console.log(
	problems.length
		? `\n${problems.length} config gate check(s) failed`
		: `The commit-msg hook checks the staged ledger, the pre-commit hook skips docs/** and tasks/**, and CI's plan, prose-only included, runs \`${check}\``,
);
process.exit(problems.length ? 1 : 0);
