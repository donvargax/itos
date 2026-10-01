// `itos config check` guards the config, the ledger, the work registry and the
// smoke set, which every later gate reads. Run against the real hook and the
// real plan in a scratch worktree of the current tree (uncommitted edits
// included), so nothing here touches the checkout it was started from:
//
//   - the pre-commit hook rejects a commit staging a ledger with a misspelt
//     key, though the ledger is under tasks/, which its prose exit skips, and
//     lets a sound ledger edit through;
//   - CI's plan runs the check for a prose-only range that touches the
//     registry (docs/work-items.yaml is under docs/**), and for a range that
//     touches the ledger.
import { readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { scratchRepo } from "./scratch.ts";

const check = "tools/bin/itos config check";
const repo = scratchRepo("config-gate-selftest");
const { dir, sh, git, edit, commit } = repo;
const preCommit = (file: string) => {
	git(`add ${file}`);
	return sh("sh .vite-hooks/pre-commit");
};
// Whether CI's plan for a range runs the check, and whether it read the range
// as prose-only.
const plan = (from: string, to: string) => {
	const run = sh(`tools/bin/itos ci plan ${from} ${to} --json`);
	if (run.status !== 0) throw new Error(`itos ci plan failed:\n${run.output}`);
	const parsed = JSON.parse(run.output) as { prose: boolean; order: { step?: string }[] };
	return { prose: parsed.prose, checks: parsed.order.some((o) => o.step === check) };
};

const problems: string[] = [];
const expect = (ok: boolean, problem: string) => {
	if (!ok) problems.push(problem);
};

try {
	const base = repo.open();

	// 1. A misspelt key in the ledger: the hook stops the commit, with the
	// check's own word for why.
	const ledger = "tasks/phase-1.yaml";
	const title = "  title: itos config check in the gates";
	edit(ledger, title, title.replace("title:", "titel:"));
	let run = preCommit(ledger);
	expect(
		run.status !== 0 && run.output.includes("unknown key titel"),
		`pre-commit let a ledger with a misspelt key through (exit ${run.status}):\n${run.output}`,
	);

	// 2. A sound edit to the same ledger passes.
	git(`reset -q --hard ${base}`);
	edit(ledger, title, `  # config gate self-test: a sound edit\n${title}`);
	run = preCommit(ledger);
	expect(run.status === 0, `pre-commit rejected a sound ledger edit:\n${run.output}`);

	// 3. A prose-only range that touches the registry runs the check in CI.
	git(`reset -q --hard ${base}`);
	const registry = "docs/work-items.yaml";
	writeFileSync(
		join(dir, registry),
		`# config gate self-test: a prose edit\n${readFileSync(join(dir, registry), "utf8")}`,
	);
	let range = plan(base, commit("docs: edit the registry"));
	expect(range.prose, `a range touching only ${registry} is no longer read as prose-only`);
	expect(range.checks, `CI's plan for a prose-only range touching ${registry} skips \`${check}\``);

	// 4. So does a range that touches the ledger, which is not prose.
	git(`reset -q --hard ${base}`);
	edit(ledger, title, `  # config gate self-test: a ledger edit\n${title}`);
	range = plan(base, commit("docs: edit the ledger"));
	expect(!range.prose, `a range touching ${ledger} is read as prose-only`);
	expect(range.checks, `CI's plan for a range touching ${ledger} skips \`${check}\``);
} finally {
	repo.remove();
}

for (const problem of problems) console.error(`FAIL ${problem}`);
console.log(
	problems.length
		? `\n${problems.length} config gate check(s) failed`
		: `The pre-commit hook and CI's plan, prose-only included, run \`${check}\``,
);
process.exit(problems.length ? 1 : 0);
