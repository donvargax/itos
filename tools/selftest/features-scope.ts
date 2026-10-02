// What a CI run's features step selects, proved by running it: go test's own
// record of the scenarios it ran (`-json`), each subtest's name read back to
// its ID from the feature files, for ranges made with `git commit-tree` on top
// of HEAD (objects only: no branch moves):
//
//   - `tools/bin/itos tests smoke run scenario` runs exactly the smoke set;
//   - a range naming one scenario runs the smoke set plus that one;
//   - the nightly, and a range CI can't read, run every live scenario;
//   - a range naming the ledger's tasks whose checks are runs of the features
//     and CI steps (T-016, T-009) runs one go test run, selecting the smoke set
//     and each subset; the tasks' other checks still run, and the gates
//     self-test is left to the nightly;
//   - the smoke rule holds today.
//
// The plan, the scenarios, the smoke set and the rule are tools/bin/itos's
// answers (`ci plan --json`, `tests list`, `tests smoke ids` and `check`), the
// feature files are read here only to name go test's subtests.
//
// What the plan does whatever the repository (the cost order, the written
// order, the prose shortcut, merging, covering, the smoke rule's failures) is
// tools/itos/conformance/plans.yaml's and smoke.yaml's.
import { strict as assert } from "node:assert";
import { spawnSync } from "node:child_process";
import { mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { parse } from "yaml";
import { ciPlan, featuresStep, itos } from "./scratch.ts";

// The scenario kind, as itos.yaml states it.
interface Kind {
	root: string;
	id: string;
	tag_prefix: string;
	run: { whole: string };
	recognize: { command: string }[];
}
const kind = (parse(readFileSync("itos.yaml", "utf8")) as { tests: { scenario: Kind } }).tests
	.scenario;

// Hooks export GIT_DIR and friends; the steps' own scratch repositories clean
// their environment, the commits made here need it clean too.
const env: NodeJS.ProcessEnv = { ...process.env };
for (const key of Object.keys(env)) if (key.startsWith("GIT_") || key === "CI") delete env[key];
const sh = (command: string) => {
	const run = spawnSync("sh", ["-c", command], { env, encoding: "utf8", maxBuffer: 64 << 20 });
	return { status: run.status ?? 1, stdout: run.stdout, output: `${run.stdout}${run.stderr}` };
};
const git = (command: string) => {
	const run = sh(`git ${command}`);
	assert.equal(run.status, 0, `git ${command} failed:\n${run.output}`);
	return run.stdout.trim();
};

const scratch = mkdtempSync(join(tmpdir(), "features-scope-selftest-"));
// A commit on top of HEAD with HEAD's tree and this message.
const head = git("rev-parse HEAD");
const range = (message: string) => {
	const file = join(scratch, "message");
	writeFileSync(file, message);
	return git(
		`-c user.name=features-scope -c user.email=selftest@localhost commit-tree HEAD^{tree} -p HEAD -F ${file}`,
	);
};

// Each scenario's ID by its subtest's name: go test names a subtest after the
// scenario, its spaces turned to underscores; the ID is the tag above it.
const idTag = new RegExp(`^${kind.tag_prefix}(${kind.id})$`);
const byName = new Map<string, string>();
for (const file of readdirSync(kind.root).filter((f) => f.endsWith(".feature"))) {
	let tags: string[] = [];
	for (const line of readFileSync(join(kind.root, file), "utf8").split("\n")) {
		const text = line.trim();
		if (text.startsWith(kind.tag_prefix)) tags.push(...text.split(/\s+/));
		const name = /^Scenario(?: Outline)?:\s*(.*?)\s*$/.exec(text)?.[1];
		if (name === undefined) continue;
		const id = tags.map((t) => idTag.exec(t)?.[1]).find(Boolean);
		if (id) byName.set(`TestFeatures/${name.replaceAll(" ", "_")}`, id);
		tags = [];
	}
}
// Whether a task check is a run of the features the plan merges into its one
// run: one of the kind's recognize templates, {pattern} standing for anything.
const recognized = (command: string) =>
	kind.recognize.some(({ command: template }) => {
		const [before, after] = template.split("{pattern}");
		return after === undefined
			? command === template
			: command.startsWith(before!) && command.endsWith(after);
	});

// The scenario IDs a command runs, as go test reports them.
function ran(command: string): Set<string> {
	const run = sh(`${command} -json`);
	assert.equal(run.status, 0, `${command} failed:\n${run.output}`);
	const ids = new Set<string>();
	for (const line of run.stdout.split("\n")) {
		if (!line.startsWith("{")) continue;
		const event = JSON.parse(line) as { Action?: string; Test?: string };
		if (event.Action !== "run" || !event.Test?.startsWith("TestFeatures/")) continue;
		const scenario = byName.get(event.Test);
		assert.ok(scenario, `go test ran ${event.Test}, which no feature file names`);
		ids.add(scenario);
	}
	return ids;
}
const same = (a: Set<string>, b: Set<string>, what: string) => {
	const missing = [...b].filter((x) => !a.has(x));
	const extra = [...a].filter((x) => !b.has(x));
	assert.ok(
		missing.length === 0 && extra.length === 0,
		`${what}: missing ${missing.join(", ") || "none"}; extra ${extra.join(", ") || "none"}`,
	);
};

try {
	const whole = kind.run.whole;
	const { tests } = JSON.parse(itos(["tests", "list", "scenario", "--json"])) as {
		tests: { id: string; live: boolean }[];
	};
	const live = new Set(tests.filter((t) => t.live).map((t) => t.id));
	const smoke = new Set(itos(["tests", "smoke", "ids", "scenario"]).split("\n").filter(Boolean));
	same(ran(whole), live, "the whole run is not every live scenario");
	same(
		ran("tools/bin/itos tests smoke run scenario --"),
		smoke,
		"the smoke run is not the smoke set",
	);

	// A range naming one scenario: the smoke set and it, one outside the smoke
	// set.
	const named = [...live].find((x) => !smoke.has(x)) ?? [...live].at(-1)!;
	let plan = ciPlan([head, range(`feat: name a scenario\n\nScenarios: @${named}\n`)]);
	const step = featuresStep(plan, whole);
	assert.ok(step, "a push naming a scenario ran no features step");
	same(ran(step), new Set([...smoke, named]), `a range naming @${named}`);

	// The nightly, and a range that can't be read, run every scenario.
	const nightly = ciPlan(["--nightly"]).steps;
	assert.equal(nightly[0], whole, `the nightly's first step is not every scenario: ${nightly[0]}`);
	assert.ok(
		nightly.includes("node tools/selftest/gates.ts"),
		"the nightly runs no gates self-test",
	);
	assert.equal(
		featuresStep(ciPlan(["", head]), whole),
		whole,
		"an unread range should run everything",
	);

	// A range naming tasks whose checks are runs of the features and CI steps
	// (T-016: the smoke check and the corpus, both steps, the smoke run and this
	// self-test; T-009: the gates self-test, the nightly's).
	plan = ciPlan([head, range("test: name two tasks\n\nTask: T-016, T-009\n")]);
	const runs = plan.steps.filter((s) => s.startsWith(whole));
	assert.equal(runs.length, 1, `expected one go test run, got:\n${runs.join("\n")}`);
	const steps = new Set(plan.steps);
	const checks = plan.order.flatMap((o) => (o.check ? [{ ...o.check, action: o.action }] : []));
	for (const check of checks) {
		const command = check.command ?? "";
		const expected = recognized(command)
			? "merged"
			: steps.has(command)
				? "covered"
				: command === "node tools/selftest/gates.ts"
					? "nightly"
					: "run";
		assert.equal(check.action, expected, `${check.task}'s \`${command}\` should be ${expected}`);
	}
	assert.ok(
		checks.some((c) => c.task === "T-016" && c.action === "merged"),
		"T-016's smoke run is not merged into the features step",
	);
	assert.ok(
		checks.some((c) => c.task === "T-009" && c.action === "nightly"),
		"a push runs the gates self-test",
	);
	same(ran(runs[0]!), smoke, "the merged run");

	// The smoke rule holds today.
	itos(["tests", "smoke", "check", "scenario"]);
} finally {
	rmSync(scratch, { recursive: true, force: true });
}

console.log(
	"features scope: a push runs the smoke set and what it names in one run; the nightly runs everything",
);
