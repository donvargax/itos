// What a CI run's E2E step selects, proved against Playwright's own listing of
// the project's suite (`--list`, no browser), for ranges made with `git
// commit-tree` on top of HEAD (objects only: no branch moves):
//
//   - `vp run e2e:smoke` runs exactly the smoke set;
//   - a range naming one scenario runs the smoke set plus that one;
//   - the nightly, and a range CI can't read, run every scenario;
//   - a range naming the ledger's tasks whose checks are E2E subsets and CI
//     steps (T-003, T-005, T-009) runs one Playwright run, selecting the smoke
//     set and each subset, and no second build; the tasks' other checks still
//     run, and the gates self-test is left to the nightly;
//   - the smoke rule holds today, and the config calls no step or build check
//     static.
//
// What the plan does whatever the repository — the cost order, the written
// order, the prose shortcut, merging, covering, the smoke rule's failures —
// is tools/itos/conformance/plans.yaml's and smoke.yaml's.
import { strict as assert } from "node:assert";
import { spawnSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { ciPlan, e2eStep, isStatic, planFor } from "../itos/ci-plan.ts";
import { smokeIds, smokeProblems } from "../itos/e2e-scope.ts";
import { recognize } from "../itos/tests.ts";

// Hooks export GIT_DIR and friends, and CI changes what Playwright lists.
const env: NodeJS.ProcessEnv = { ...process.env };
for (const key of Object.keys(env)) if (key.startsWith("GIT_") || key === "CI") delete env[key];
const sh = (command: string, extra: NodeJS.ProcessEnv = {}) => {
	const run = spawnSync("sh", ["-c", command], {
		env: { ...env, ...extra },
		encoding: "utf8",
		maxBuffer: 64 * 1024 * 1024,
	});
	return { status: run.status ?? 1, output: `${run.stdout}${run.stderr}` };
};
const git = (command: string) => {
	const run = sh(`git ${command}`);
	assert.equal(run.status, 0, `git ${command} failed:\n${run.output}`);
	return run.output.trim();
};
// A commit on top of HEAD with HEAD's tree and this message.
const head = git("rev-parse HEAD");
const range = (message: string) => {
	const file = join(scratch, "message");
	writeFileSync(file, message);
	return git(
		`-c user.name=e2e-scope -c user.email=selftest@localhost commit-tree HEAD^{tree} -p HEAD -F ${file}`,
	);
};

// The scenario IDs an E2E command selects, as Playwright lists them.
interface Suite {
	suites?: Suite[];
	specs?: { tags?: string[]; tests?: { tags?: string[] }[] }[];
}
const idsIn = (suite: Suite): string[] => [
	...(suite.specs ?? []).flatMap((spec) =>
		[...(spec.tags ?? []), ...(spec.tests ?? []).flatMap((t) => t.tags ?? [])]
			.map((tag) => tag.replace(/^@/, ""))
			.filter((tag) => tag.startsWith("ID-")),
	),
	...(suite.suites ?? []).flatMap(idsIn),
];
function listed(command: string): Set<string> {
	const report = join(scratch, "list.json");
	rmSync(report, { force: true });
	const run = sh(`${command} --list --reporter=json`, { PLAYWRIGHT_JSON_OUTPUT_NAME: report });
	assert.equal(run.status, 0, `${command} --list failed:\n${run.output}`);
	return new Set(idsIn(JSON.parse(readFileSync(report, "utf8")) as Suite));
}
const same = (a: Set<string>, b: Set<string>, what: string) => {
	const missing = [...b].filter((id) => !a.has(id));
	const extra = [...a].filter((id) => !b.has(id));
	assert.ok(
		missing.length === 0 && extra.length === 0,
		`${what}: missing ${missing.join(", ") || "none"}; extra ${extra.join(", ") || "none"}`,
	);
};

const scratch = mkdtempSync(join(tmpdir(), "e2e-scope-selftest-"));
try {
	const smoke = new Set(smokeIds());
	const everything = listed("vp run e2e");
	assert.ok(everything.size >= smoke.size, "the suite lists fewer scenarios than the smoke set");
	same(
		listed("tools/bin/itos tests smoke run scenario --"),
		smoke,
		"`vp run e2e:smoke` is not the smoke set",
	);

	// A range naming one scenario: the smoke set and it. The last one listed,
	// outside the smoke set once the suite has one there.
	const named = [...everything].find((id) => !smoke.has(id)) ?? [...everything].at(-1)!;
	let plan = planFor(head, range(`feat: name a scenario\n\nScenarios: @${named}\n`));
	let step = e2eStep(plan);
	assert.ok(step, "a push naming a scenario ran no E2E step");
	same(listed(step), new Set([...smoke, named]), `a range naming @${named}`);

	// The nightly, and a range that can't be read, run every scenario.
	assert.deepEqual(ciPlan({ known: false, nightly: true }).steps, [
		"vp run e2e",
		"node tools/selftest/gates.ts",
	]);
	assert.equal(e2eStep(planFor("", head)), "vp run e2e", "an unread range should run everything");

	// A range naming tasks whose checks are E2E subsets and CI steps (T-003:
	// `vp test run` and the coverage step; T-005: the smoke check, an E2E
	// subset, the smoke run and this self-test; T-009: the gates self-test,
	// the nightly's).
	plan = planFor(head, range("ci: name three tasks\n\nTask: T-003, T-005, T-009\n"));
	const runs = plan.steps.filter((s) => s.startsWith("vp run e2e"));
	assert.equal(runs.length, 1, `expected one Playwright run, got:\n${runs.join("\n")}`);
	assert.equal(plan.steps.filter((s) => s === "vp build").length, 1, "the build runs twice");
	for (const planned of plan.checks) {
		const command = planned.check.run ?? "";
		const expected = recognize("scenario", command, [...smoke])
			? "merged"
			: [
						"vp test run",
						"vp run test:coverage",
						"tools/bin/itos tests smoke check scenario",
				  ].includes(command)
				? "covered"
				: command === "node tools/selftest/gates.ts"
					? "nightly"
					: "run";
		const got = planned.merged
			? "merged"
			: planned.coveredBy
				? "covered"
				: planned.nightly
					? "nightly"
					: "run";
		assert.equal(got, expected, `${planned.task}'s \`${command}\` should be ${expected}`);
	}
	assert.ok(
		plan.checks.some((c) => c.task === "T-005" && !c.merged && !c.coveredBy && !c.nightly),
		"T-005's own check no longer runs",
	);
	assert.ok(
		plan.checks.some((c) => c.task === "T-009" && c.nightly),
		"a push runs the gates self-test",
	);
	assert.ok(
		!plan.steps.includes("node tools/selftest/gates.ts"),
		"a push runs the gates self-test",
	);
	const subsets = plan.checks
		.filter((c) => c.merged)
		.map((c) => listed(c.check.run!))
		.flatMap((ids) => [...ids]);
	same(listed(runs[0]!), new Set([...smoke, ...subsets]), "the merged run");

	// A step and a check this config may not call static.
	const built = "test -f dist/index.html";
	for (const command of ["vp run test:coverage", "vp build", "vp run e2e", built])
		assert.ok(!isStatic(command), `\`${command}\` should not be static`);

	// The smoke rule holds today.
	assert.deepEqual(smokeProblems(), []);
	assert.equal(sh("tools/bin/itos tests smoke check scenario").status, 0);
} finally {
	rmSync(scratch, { recursive: true, force: true });
}

console.log(
	"e2e scope: a push runs the smoke set and what it names in one run; the nightly runs everything",
);
