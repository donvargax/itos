// The moves rule, a built-in range check of a Gherkin kind:
// `{ name, builtin: moves, except_types, allowed_renames }` in
// `tests.<kind>.range_checks`. A commit of a type except_types does not name
// may move scenarios between feature files, create feature files and delete
// the ones left empty, provided every live scenario keeps its ID, name, tags
// and steps exactly, none is lost or added, and a file with a live scenario
// keeps its header and Background. A @wip scenario may still be added,
// changed or removed: that is how a specification lands before its
// implementation. A rename the project allows is listed in allowed_renames,
// by ID (without its prefix) and new name. Comment lines (`#`) are not part of
// a scenario's ID, name, tags or steps, nor of a header, so they are dropped
// before anything is compared: a reason may be written beside a scenario in
// any commit.
//
// The commit-msg hook judges HEAD against the index (commit-scope.ts),
// verify each commit of its range against its parent (verify-commits.ts),
// and `itos tests moves <kind>` HEAD against the index by hand. The kind's
// root, ID pattern, tag prefix and wip tag are its own (tests.ts).
import { config, ConfigError, configPath, type RangeCheck } from "./config.ts";
import { type GherkinOptions, featureTexts, parseFeature } from "./gherkin.ts";
import { emit, type Output, problem, type Problem, TEXT } from "./problem.ts";
import { git } from "./repo.ts";
import { gherkinOptions, kind } from "./tests.ts";

const SCENARIO_LINE = /^(\s*Scenario(?: Outline)?:\s*)(.*?)\s*$/;
const COMMENT = /^\s*#/;
// The tree a commit with no parent is compared with.
const EMPTY_TREE = "4b825dc642cb6eb9a060e54bf8d69288fbee4904";

// The text without its comment lines, which the rule never compares.
const withoutComments = (text: string) =>
	text
		.split("\n")
		.filter((l) => !COMMENT.test(l))
		.join("\n")
		.trimEnd();

export interface Scenario {
	file: string;
	wip: boolean;
	// The block as written, less its comment lines: its tag line, its Scenario
	// line and its steps.
	body: string;
}
export interface FeatureSet {
	// Each file's header: everything before its first scenario, the Background
	// included.
	headers: Map<string, string>;
	scenarios: Map<string, Scenario>;
}

// The feature files given as { path: text }, read into one set by the kind's
// ID pattern, tag prefix and wip tag.
export function readFeatures(
	files: Record<string, string>,
	options: Omit<GherkinOptions, "root">,
): FeatureSet {
	const set: FeatureSet = { headers: new Map(), scenarios: new Map() };
	for (const [file, text] of Object.entries(files)) {
		// A file tagged @wip above its Feature line is @wip throughout.
		const { header, fileWip, blocks } = parseFeature(text, options);
		set.headers.set(file, withoutComments(header));
		for (const [id, block] of blocks)
			set.scenarios.set(id, {
				file,
				wip: fileWip || block.wip,
				body: withoutComments(block.body),
			});
	}
	return set;
}

const sets = new Map<string, FeatureSet>();
// The kind's feature files at a tree: a commit, or "index" for the staged
// tree, read once per run (verify reads each commit as a child and as a
// parent). A tree that cannot be read (no HEAD yet) is empty.
export function featureSet(name: string, tree: string): FeatureSet {
	const key = `${name}:${tree}`;
	if (!sets.has(key)) {
		const options = gherkinOptions(name);
		sets.set(key, readFeatures(featureTexts(tree, options.root), options));
	}
	return sets.get(key)!;
}

// Whether `after` is `before` with its Scenario line's name changed to `name`
// and nothing else.
function renamedTo(before: string, after: string, name: string): boolean {
	const a = before.split("\n");
	const b = after.split("\n");
	if (a.length !== b.length) return false;
	let renamed = false;
	for (let i = 0; i < a.length; i++) {
		if (a[i] === b[i]) continue;
		const from = SCENARIO_LINE.exec(a[i]!);
		const to = SCENARIO_LINE.exec(b[i]!);
		if (renamed || !from || !to || from[1] !== to[1] || to[2] !== name) return false;
		renamed = true;
	}
	return renamed;
}

const has = (set: FeatureSet, file: string) =>
	[...set.scenarios.values()].some((s) => s.file === file && !s.wip);

// A scenario of the later set against its earlier self, if any.
function scenarioProblem(
	id: string,
	was: Scenario | undefined,
	now: Scenario,
	renames: Record<string, string>,
): string | undefined {
	if (!was) return now.wip ? undefined : `adds the live scenario ${id} to ${now.file}`;
	if (was.body === now.body || now.wip) return undefined;
	const allowed = renames[id];
	if (allowed && renamedTo(was.body, now.body, allowed)) return undefined;
	return `changes the live scenario ${id} in ${now.file}: a moved scenario keeps its ID, name, tags and steps exactly`;
}

// A file of the later set whose header changed while it held a live scenario.
function headerProblem(file: string, before: FeatureSet, after: FeatureSet): string | undefined {
	const was = before.headers.get(file);
	if (was === undefined || was === after.headers.get(file)) return undefined;
	if (!has(before, file) && !has(after, file)) return undefined;
	return `changes the header or Background of ${file}`;
}

// What breaks the rule between two sets, each problem a phrase for "a <type>
// commit …". `renames` is the allowed renames, by ID and new name.
export function moveProblems(
	before: FeatureSet,
	after: FeatureSet,
	renames: Record<string, string> = {},
): string[] {
	const problems: string[] = [];
	for (const [id, now] of after.scenarios) {
		const found = scenarioProblem(id, before.scenarios.get(id), now, renames);
		if (found) problems.push(found);
	}
	for (const [id, was] of before.scenarios)
		if (!after.scenarios.has(id) && !was.wip)
			problems.push(`loses the scenario ${id} of ${was.file}`);
	for (const file of after.headers.keys()) {
		const found = headerProblem(file, before, after);
		if (found) problems.push(found);
	}
	return problems;
}

// Each kind's built-in moves checks.
const movesChecks = (): { kind: string; check: RangeCheck }[] =>
	Object.entries(config().tests ?? {}).flatMap(([name, k]) =>
		(k.range_checks ?? [])
			.filter((c) => c.builtin === "moves")
			.map((check) => ({ kind: name, check })),
	);

// Whether the rule judges a commit of this type: one except_types does not
// name and, when the config lists the commit types, one of them, so a merge's
// or git's own revert's message ("Merge …", "Revert …") is the header lint's.
const judges = (check: RangeCheck, type: string) => {
	const types = config().commits?.types;
	return !check.except_types?.includes(type) && (!types || types.includes(type));
};

// The moves checks' problems for a commit of a type, from the tree `before`
// to the tree `after`, each worded for "a <type> commit".
function issuesBetween(type: string, before: string, after: string): Problem[] {
	return movesChecks()
		.filter(({ check }) => judges(check, type))
		.flatMap(({ kind: name, check }) =>
			moveProblems(featureSet(name, before), featureSet(name, after), check.allowed_renames).map(
				(p) => problem(check.name, `a ${type} commit ${p}`),
			),
		);
}

// The commit-msg hook's moves rule: HEAD against the index.
export const stagedMoveIssues = (type: string): Problem[] => issuesBetween(type, "HEAD", "index");

// verify's moves rule for one commit: the commit against its parent, or the
// empty tree for a root commit.
export function commitMoveIssues(sha: string, type: string): Problem[] {
	if (!movesChecks().some(({ check }) => judges(check, type))) return [];
	let parent = EMPTY_TREE;
	try {
		parent = git("rev-parse", "--verify", "--quiet", `${sha}^`).trim() || EMPTY_TREE;
	} catch {
		// a root commit
	}
	return issuesBetween(type, parent, sha);
}

// `itos tests moves <kind>`: HEAD against the index, by the kind's moves
// check's allowed renames (none when it has no moves check). 0 when the staged
// feature files only move scenarios, if anything, 1 with each problem.
export function testsMoves(name: string, out: Output = TEXT): number {
	const { adapter } = kind(name);
	if (adapter !== "gherkin")
		throw new ConfigError(configPath(), [
			problem(
				"moves-not-gherkin",
				`tests.${name}.adapter is not gherkin, and the moves rule reads feature files`,
				"name a kind whose adapter is gherkin",
			),
		]);
	const renames = Object.assign(
		{},
		...movesChecks()
			.filter((m) => m.kind === name)
			.map((m) => m.check.allowed_renames ?? {}),
	) as Record<string, string>;
	const found = moveProblems(featureSet(name, "HEAD"), featureSet(name, "index"), renames).map(
		(p) => problem("moves", `the index ${p}`),
	);
	if (out.json) emit({ kind: name, ok: found.length === 0, problems: found });
	else if (found.length) for (const p of found) console.error(`  - ${p.message}`);
	else if (!out.quiet) console.log("The staged feature files only move scenarios, if anything");
	return found.length ? 1 : 0;
}
