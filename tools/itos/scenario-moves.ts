// This repository's moving rule as a script, until itos.yaml's
// `tests.scenario.range_checks` takes the built-in (moves.ts, T-037). A rename
// allowed outside feat and fix is listed in ALLOWED_RENAMES, by ID and new
// name.
//
//   node tools/itos/scenario-moves.ts               HEAD against the index, as
//   node tools/itos/scenario-moves.ts --type <t>    the commit-msg hook checks it
//                                                   (with --type, worded for a <t> commit)
//   node tools/itos/scenario-moves.ts <from> <to>   every commit in from..to that
//                                                   is not a feat or a fix (CI)
import { featureSet, moveProblems } from "./moves.ts";
import { git, rangeArgs } from "./repo.ts";

// The scenario kind of itos.yaml, whose range check this is.
const KIND = "scenario";

// A live scenario's name may not change outside feat and fix; these renames
// are allowed anyway, for exactly this ID and new name: `"ID-…": "the new
// name"`. Each one is a decision the project owner takes; record why in the
// task that allows it.
const ALLOWED_RENAMES: Record<string, string> = {};

// The commits of a range, oldest first; an empty or all-zero start (a new
// branch) means everything up to `to`. commits.since and its ancestors are
// left out, as verify leaves them.
const commitsIn = (from: string, to: string) =>
	git("rev-list", "--no-merges", "--reverse", ...rangeArgs(from, to))
		.split("\n")
		.filter(Boolean);

const typeOf = (message: string) => /^(\w+)/.exec(message)?.[1] ?? "";

const problemsBetween = (before: string, after: string) =>
	moveProblems(featureSet(KIND, before), featureSet(KIND, after), ALLOWED_RENAMES);

if (import.meta.main) {
	const args = process.argv.slice(2);
	const staged = args[0] === "--type" ? `a ${args[1]} commit` : "the index";
	const [from, to] = args[0] === "--type" ? [] : args;
	let failed = 0;
	if (from === undefined) {
		const problems = problemsBetween("HEAD", "index");
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
			const problems = problemsBetween(parent, sha);
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
