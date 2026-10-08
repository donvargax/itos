// What a CI run's features step selects, proved by running it: go test's own
// record of the scenarios it ran (`-json`), each subtest's name read back to
// its ID from the feature files, for ranges made with `git commit-tree` on top
// of HEAD (objects only: no branch moves):
//
//   - `tools/bin/itos tests smoke run scenario` runs exactly the smoke set;
//   - a range naming one scenario runs the smoke set plus that one;
//   - the nightly, and a range CI can't read, run every live scenario;
//   - a range naming the ledger's tasks, none of which holds a check that is a
//     run of the features or a step any more (T-125 emptied them), still runs
//     one go test run, over the smoke set alone;
//   - the smoke rule holds today.
//
// The plan, the scenarios, the smoke set and the rule are tools/bin/itos's
// answers (`ci plan --json`, `tests list`, `tests smoke ids` and `check`), the
// feature files are read here only to name go test's subtests.
//
// What the plan does whatever the repository (the cost order, the written
// order, the prose shortcut, merging a task check into that run, covering one
// by a step, leaving one to ci.nightly_only, the smoke rule's failures) is
// tools/itos/conformance/plans.yaml's and smoke.yaml's, over configurations of
// their own, since no task of this repository holds a check to try it on.
import { strict as assert } from "node:assert";
import { spawnSync } from "node:child_process";
import { mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { parse } from "yaml";
import { ciPlan, featuresStep, itos, outsideEnv, shGit } from "./scratch.ts";

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
const env = outsideEnv();
const sh = (command: string) => {
	const run = spawnSync("sh", ["-c", command], { env, encoding: "utf8", maxBuffer: 64 << 20 });
	return { status: run.status ?? 1, stdout: run.stdout, output: `${run.stdout}${run.stderr}` };
};
const git = (command: string) => {
	const run = sh(`${shGit()} ${command}`);
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

// go test's subtest name for a scenario, and the scenarios' names by it: go
// test names a subtest after the scenario, its spaces turned to underscores,
// and two scenarios may share a name (ID-ASK-01 and ID-ASK-20), which it
// tells apart by a "#01" suffix on the later one. The suffix is dropped here
// and the scenarios sharing a name are counted, since which of them ran is
// not in what go test reports.
const idTag = new RegExp(`^${kind.tag_prefix}(${kind.id})$`);
const subtestOf = new Map<string, string>(); // an ID, its subtest name
const namesOf = new Set<string>(); // every subtest name
for (const file of readdirSync(kind.root).filter((f) => f.endsWith(".feature"))) {
	let tags: string[] = [];
	for (const line of readFileSync(join(kind.root, file), "utf8").split("\n")) {
		const text = line.trim();
		if (text.startsWith(kind.tag_prefix)) tags.push(...text.split(/\s+/));
		const name = /^Scenario(?: Outline)?:\s*(.*?)\s*$/.exec(text)?.[1];
		if (name === undefined) continue;
		const id = tags.map((t) => idTag.exec(t)?.[1]).find(Boolean);
		if (id) {
			const subtest = `TestFeatures/${name.replaceAll(" ", "_")}`;
			namesOf.add(subtest);
			subtestOf.set(id, subtest);
		}
		tags = [];
	}
}
// What a command ran, as go test reports it: each subtest's name and how many
// times, the "#01" of a name two scenarios share dropped.
type Ran = Map<string, number>;
function ran(command: string): Ran {
	const run = sh(`${command} -json`);
	assert.equal(run.status, 0, `${command} failed:\n${run.output}`);
	const counts: Ran = new Map();
	for (const line of run.stdout.split("\n")) {
		if (!line.startsWith("{")) continue;
		const event = JSON.parse(line) as { Action?: string; Test?: string };
		if (event.Action !== "run" || !event.Test?.startsWith("TestFeatures/")) continue;
		const name = event.Test.replace(/#\d+$/, "");
		assert.ok(namesOf.has(name), `go test ran ${event.Test}, which no feature file names`);
		counts.set(name, (counts.get(name) ?? 0) + 1);
	}
	return counts;
}
// The same, as a set of scenario IDs would read it: each name the IDs share.
const wanted = (...ids: Iterable<string>[]): Ran => {
	const out: Ran = new Map();
	for (const each of ids) {
		for (const id of each) {
			const name = subtestOf.get(id);
			assert.ok(name, `no feature file names ${id}`);
			out.set(name, (out.get(name) ?? 0) + 1);
		}
	}
	return out;
};
const same = (a: Ran, b: Ran, what: string) => {
	const missing = [...b].filter(([name, n]) => (a.get(name) ?? 0) !== n);
	const extra = [...a].filter(([name, n]) => (b.get(name) ?? 0) !== n);
	const show = (list: [string, number][]) =>
		list.map(([name, n]) => `${name} x${n}`).join(", ") || "none";
	assert.ok(
		missing.length === 0 && extra.length === 0,
		`${what}: missing ${show(missing)}; extra ${show(extra)}`,
	);
};

try {
	const whole = kind.run.whole;
	const { tests } = JSON.parse(itos(["tests", "list", "scenario", "--json"])) as {
		tests: { id: string; live: boolean }[];
	};
	const live = new Set(tests.filter((t) => t.live).map((t) => t.id));
	const smoke = new Set(itos(["tests", "smoke", "ids", "scenario"]).split("\n").filter(Boolean));
	same(ran(whole), wanted(live), "the whole run is not every live scenario");
	same(
		ran("tools/bin/itos tests smoke run scenario --"),
		wanted(smoke),
		"the smoke run is not the smoke set",
	);

	// A range naming one scenario: the smoke set and it, one outside the smoke
	// set.
	const named = [...live].find((x) => !smoke.has(x)) ?? [...live].at(-1)!;
	let plan = ciPlan([head, range(`feat: name a scenario\n\nScenarios: @${named}\n`)]);
	const step = featuresStep(plan, whole);
	assert.ok(step, "a push naming a scenario ran no features step");
	same(ran(step), wanted(smoke, [named]), `a range naming @${named}`);

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

	// A range naming tasks: one go test run, over the smoke set. What a task
	// check reads as a run of this kind, or as a step that has done it, or as
	// a check ci.nightly_only leaves out of a push, is the corpus's
	// (plans.yaml), over a configuration of its own: T-125 emptied this
	// repository's ledger of done tasks' checks, so there is no check here to
	// try each rule on.
	plan = ciPlan([head, range("test: name two tasks\n\nTask: T-016, T-009\n")]);
	const runs = plan.steps.filter((s) => s.startsWith(whole));
	assert.equal(runs.length, 1, `expected one go test run, got:\n${runs.join("\n")}`);
	same(ran(runs[0]!), wanted(smoke), "the run of a range naming tasks");

	// The smoke rule holds today.
	itos(["tests", "smoke", "check", "scenario"]);
} finally {
	rmSync(scratch, { recursive: true, force: true });
}

console.log(
	"features scope: a push runs the smoke set and what it names in one run; the nightly runs everything",
);
