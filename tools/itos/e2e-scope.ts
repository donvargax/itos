// The smoke set's rule and run. A push's CI runs the smoke set (read through
// smoke.ts), the tests its commits name and the subsets of the tasks they
// name, in one run of the kind; the nightly runs every test. Which tests
// exist and which are live is the kind's adapter's answer, and the command
// that runs them its templates' (tests.ts).
//
//   itos tests smoke check <kind> [--features <dir>]
//                       every file with a live test has a smoke test, and
//                       every smoke ID is live
//   itos tests smoke ids <kind>
//                       the smoke IDs, one a line
//   itos tests smoke run <kind> [-- <runner args>…]
//                       run exactly the smoke set (`vp run e2e:smoke`)
import { spawnSync } from "node:child_process";
import { emit, messages, type Output, problem, type Problem, TEXT } from "./problem.ts";
import { loadSmoke, type SmokeFile } from "./smoke.ts";
import { bareId, commandFor, kind, listTests } from "./tests.ts";

// The kind a caller means when it names none: itos.yaml's `tests.scenario`.
const KIND = "scenario";

// Each kind's working-tree smoke set, read once.
const working = new Map<string, SmokeFile[]>();
const smokeSet = (name = KIND) => {
	if (!working.has(name)) working.set(name, loadSmoke(name));
	return working.get(name)!;
};

const addHint = (name: string) => {
	const smoke = kind(name).smoke;
	return smoke?.add_hint ?? `add one to ${smoke?.file}`;
};

// Each file's live test IDs (without the tag prefix), keyed by its path
// relative to the kind's root, which `root` stands in for. A file with no live
// test, or none at all, is there with none.
export function liveScenarios(root?: string, name = KIND): Map<string, Set<string>> {
	const { tests, files } = listTests(name, { root });
	const live = new Map(files.map((file) => [file, new Set<string>()]));
	for (const t of tests) if (t.live) live.get(t.file)?.add(t.id);
	return live;
}

function entryProblems(entry: SmokeFile, live: Set<string> | undefined, name: string): Problem[] {
	const smokeFile = kind(name).smoke?.file;
	const edit = `edit ${smokeFile}`;
	if (!live)
		return [
			problem(
				"smoke-not-a-file",
				`the smoke list names ${entry.file}, which is not a feature file`,
				`${edit}: remove the entry for ${entry.file}, or correct its path`,
			),
		];
	const found = entry.scenarios
		.filter((s) => !live.has(bareId(name, s.id)))
		.map((s) =>
			problem(
				"smoke-not-live",
				`the smoke list names ${s.id}, which is not a live scenario of ${entry.file}`,
				`${edit}: name a live scenario of ${entry.file} in place of ${s.id}`,
			),
		);
	if (entry.scenarios.some((s) => !s.why.trim()))
		found.push(
			problem(
				"smoke-no-why",
				`a smoke scenario of ${entry.file} does not say why it is there`,
				`${edit}: give each scenario of ${entry.file} a why`,
			),
		);
	if (entry.scenarios.length > 1 && !entry.more?.trim())
		found.push(
			problem(
				"smoke-more-no-why",
				`${entry.file} has more than one smoke scenario and does not say why`,
				`${edit}: give ${entry.file} a more: that says why it has more than one`,
			),
		);
	return found;
}

// What breaks the rule, with its rule id: a feature file with a live scenario
// and no smoke one, a smoke ID that is not a live scenario of its file, a
// missing reason.
export function smokeIssues(smoke: SmokeFile[], root?: string, name = KIND): Problem[] {
	const files = liveScenarios(root, name);
	const listed = new Set(smoke.filter((e) => e.scenarios.length > 0).map((e) => e.file));
	const found = [...files]
		.filter(([file, live]) => live.size > 0 && !listed.has(file))
		.map(([file]) =>
			problem("smoke-missing", `${file} has no smoke scenario (${addHint(name)})`, addHint(name)),
		);
	for (const entry of smoke) found.push(...entryProblems(entry, files.get(entry.file), name));
	return found;
}

// The same, as the sentences the check prints.
export const smokeProblems = (smoke: SmokeFile[] = smokeSet(), root?: string): string[] =>
	messages(smokeIssues(smoke, root));

export const smokeIds = (smoke: SmokeFile[] = smokeSet(), name = KIND) =>
	smoke.flatMap((e) => e.scenarios.map((s) => bareId(name, s.id)));

// `tests smoke check <kind>`: the rule over the working tree, or a copy of the
// kind's files in `root`. 0 when it holds, 1 when it does not.
export function smokeCheck(name: string, root?: string, out: Output = TEXT): number {
	const found = smokeIssues(smokeSet(name), root, name);
	const count = smokeIds(smokeSet(name), name).length;
	if (out.json) emit({ kind: name, ok: found.length === 0, ids: count, problems: found });
	else for (const p of found) console.error(`FAIL ${p.message}`);
	if (found.length) return 1;
	if (!out.json && !out.quiet)
		console.log(`Every feature file has a smoke scenario (${count} in all)`);
	return 0;
}

// `tests smoke ids <kind>`: the smoke set's IDs, one a line.
export function smokeIdsCommand(name: string, out: Output = TEXT): number {
	const ids = smokeIds(smokeSet(name), name);
	if (out.json) emit({ kind: name, ids });
	else for (const id of ids) console.log(id);
	return 0;
}

// `tests smoke run <kind>`: the kind's run of exactly the smoke set, with the
// arguments given; its exit code is the runner's.
export function smokeRun(name: string, args: string[]): number {
	const run = commandFor(name, [{ ids: smokeIds(smokeSet(name), name) }])!;
	return spawnSync("sh", ["-c", `${run} "$@"`, "sh", ...args], { stdio: "inherit" }).status ?? 1;
}
