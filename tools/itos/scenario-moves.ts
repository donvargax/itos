// The rule for a commit outside feat and fix: it may move scenarios
// between feature files, create feature files and delete the ones left empty,
// provided every live scenario keeps its ID, name, tags and steps exactly,
// none is lost or added, and a file's header and Background stay as they
// were. A @wip scenario may still be added, changed or removed: that is how a
// specification lands before its implementation. A rename the project allows
// outside feat and fix is listed in ALLOWED_RENAMES, by ID and new name.
// Comment lines
// (`#`) are not part of a scenario's ID, name, tags or steps, nor of a
// header, so they are dropped before anything is compared: a reason may be
// written beside a scenario in any commit.
//
//   node tools/itos/scenario-moves.ts               HEAD against the index, as
//   node tools/itos/scenario-moves.ts --type <t>    the commit-msg hook checks it
//                                                   (with --type, worded for a <t> commit)
//   node tools/itos/scenario-moves.ts <from> <to>   every commit in from..to that
//                                                   is not a feat or a fix (CI)
//
// itos.yaml's `tests.scenario.range_checks` names these commands.
import { featureTexts, parseFeature } from "./gherkin.ts";
import { git } from "./repo.ts";
import { kind } from "./tests.ts";

// The scenario kind of itos.yaml, whose range check this is: its
// feature files, IDs and wip tag.
const KIND = "scenario";
const options = () => {
	const { id = "", tag_prefix = "@", wip_tag = "@wip" } = kind(KIND);
	return { id, tag_prefix, wip_tag };
};
const SCENARIO_LINE = /^(\s*Scenario(?: Outline)?:\s*)(.*?)\s*$/;
const COMMENT = /^\s*#/;

// The text without its comment lines, which the rule never compares.
const withoutComments = (text: string) =>
	text
		.split("\n")
		.filter((l) => !COMMENT.test(l))
		.join("\n")
		.trimEnd();

// A live scenario's name may not change outside feat and fix; these renames
// are allowed anyway, for exactly this ID and new name: `"ID-…": "the new
// name"`. Each one is a decision the project owner takes; record why in the
// task that allows it.
const ALLOWED_RENAMES: Record<string, string> = {};

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

// The feature files given as { path: text }, read into one set.
export function readFeatures(files: Record<string, string>): FeatureSet {
	const set: FeatureSet = { headers: new Map(), scenarios: new Map() };
	for (const [file, text] of Object.entries(files)) {
		// A file tagged @wip above its Feature line is @wip throughout.
		const { header, fileWip, blocks } = parseFeature(text, options());
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

// The feature files of a tree: a commit, or "index" for the staged tree. A
// tree that cannot be read (no HEAD yet) is empty.
export const featureSet = (tree: string): FeatureSet =>
	readFeatures(featureTexts(tree, kind(KIND).root ?? "e2e/features"));

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
// commit …".
// `renames` is the allowed renames, ALLOWED_RENAMES unless a test gives its own.
export function moveProblems(
	before: FeatureSet,
	after: FeatureSet,
	renames: Record<string, string> = ALLOWED_RENAMES,
): string[] {
	const problems: string[] = [];
	for (const [id, now] of after.scenarios) {
		const problem = scenarioProblem(id, before.scenarios.get(id), now, renames);
		if (problem) problems.push(problem);
	}
	for (const [id, was] of before.scenarios)
		if (!after.scenarios.has(id) && !was.wip)
			problems.push(`loses the scenario ${id} of ${was.file}`);
	for (const file of after.headers.keys()) {
		const problem = headerProblem(file, before, after);
		if (problem) problems.push(problem);
	}
	return problems;
}

// The commits of a range, oldest first; an empty or all-zero start (a new
// branch) means everything up to `to`.
const commitsIn = (from: string, to: string) =>
	git("rev-list", "--no-merges", "--reverse", !from || /^0+$/.test(from) ? to : `${from}..${to}`)
		.split("\n")
		.filter(Boolean);

const typeOf = (message: string) => /^(\w+)/.exec(message)?.[1] ?? "";

if (import.meta.main) {
	const args = process.argv.slice(2);
	const staged = args[0] === "--type" ? `a ${args[1]} commit` : "the index";
	const [from, to] = args[0] === "--type" ? [] : args;
	let failed = 0;
	if (from === undefined) {
		const problems = moveProblems(featureSet("HEAD"), featureSet("index"));
		for (const p of problems) console.error(`  - ${staged} ${p}`);
		failed = problems.length;
		if (!failed) console.log("The staged feature files only move scenarios, if anything");
	} else {
		const commits = commitsIn(from, to ?? "HEAD");
		let checked = 0;
		for (const sha of commits) {
			const message = git("log", "-1", "--format=%B", sha);
			const type = typeOf(message);
			if (type === "feat" || type === "fix") continue;
			checked++;
			let parent = "";
			try {
				parent = git("rev-parse", "--verify", `${sha}^`).trim();
			} catch {
				parent = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"; // the empty tree
			}
			const problems = moveProblems(featureSet(parent), featureSet(sha));
			if (problems.length === 0) continue;
			failed++;
			console.error(`${sha.slice(0, 7)} ${message.split("\n")[0]}`);
			for (const p of problems) console.error(`  - a ${type} commit ${p}`);
		}
		console.log(
			`${checked - failed}/${checked} commits outside feat and fix only move scenarios, if anything (${commits.length} in the range)`,
		);
	}
	process.exit(failed ? 1 : 0);
}
