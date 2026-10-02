// Staged-file rules that a header lint can't see (it only reads the message):
// which paths each commit type may touch, and each kind's range checks on the
// staged tree: its staged commands, and the built-in moves rule (for
// scenarios, that outside feat/fix a feature file only gains or changes @wip
// scenarios, or has scenarios moved to or from it unchanged: moves.ts).
// `itos hook commit-msg` (hooks.ts) runs them before the header lint; `itos
// commit check-paths` runs the path rules alone, for planning a split.
import { readFileSync } from "node:fs";
import { config, matchesAny as matches, type RangeCheck, section } from "./config.ts";
import { stagedMoveIssues } from "./moves.ts";
import { emit, type Output, problem, type Problem, TEXT } from "./problem.ts";
import { stagedFiles } from "./repo.ts";
import { inShell } from "./shell.ts";
import { shellWord } from "./tests.ts";

// The rules come from itos.yaml: `commits.scopes`, whose `$config`
// is `commits.path_sets.config`, and `commits.reject_message`.
// only: every touched path must match; never: no touched path may match;
// must_touch: at least one touched path must match.
// The types whose path rules let a file through: where a rejected one goes.
function typesFor(file: string): string[] {
	return Object.entries(section("commits").scopes ?? {})
		.filter(([, r]) => !(r.only && !matches(file, r.only)) && !(r.never && matches(file, r.never)))
		.map(([type]) => type);
}
const split = (file: string) =>
	`split the commit: stage ${file} in a commit of another type (${typesFor(file).join(", ")})`;

// The path rules' problems for a type and its files; none for a type with no
// rule (merges, reverts and unknown types are commitlint's business).
export function scopeIssues(type: string, files: string[]): Problem[] {
	const rules = (section("commits").scopes ?? {})[type];
	if (!rules) return [];
	const found: Problem[] = [];
	for (const file of files) {
		if (rules.only && !matches(file, rules.only))
			found.push(problem("scope-only", `${type} commits may not touch ${file}`, split(file)));
		if (rules.never && matches(file, rules.never))
			found.push(problem("scope-never", `${type} commits may not touch ${file}`, split(file)));
	}
	if (rules.must_touch && !files.some((f) => matches(f, rules.must_touch!)))
		found.push(
			problem(
				"scope-must-touch",
				`${type} commits must change ${rules.must_touch.join(" or ")}`,
				`stage a change to ${rules.must_touch.join(" or ")}, or use the type that fits the change`,
			),
		);
	return found;
}

// The rejection, as the hook has always printed it.
export function reject(found: Problem[]) {
	console.error(section("commits").reject_message);
	for (const p of found) console.error(`  - ${p.message}`);
}

// `commit check-paths --type <t> <path>…`: the path rules only, for planning
// a split. 0 when they hold, 1 when not.
export function checkPaths(type: string, files: string[], out: Output = TEXT): number {
	const found = scopeIssues(type, files);
	if (out.json) emit({ type, files, ok: found.length === 0, problems: found });
	else if (found.length) reject(found);
	return found.length ? 1 : 0;
}

// The kinds' staged range checks (`tests.<kind>.range_checks[].staged`) that
// apply to a commit of a type, such as the scenario moves.
const stagedChecks = (type: string) =>
	Object.values(config().tests ?? {})
		.flatMap((k) => k.range_checks ?? [])
		.filter((c) => c.staged && !c.except_types?.includes(type));

// One staged range check, its {type} filled in, run on the index. Each
// `  - …` line it prints on stderr when it fails is one problem; a check that
// fails without one is a problem too.
function stagedCheckIssues(check: RangeCheck, type: string): Problem[] {
	const command = check.staged!.replaceAll("{type}", shellWord(type));
	const run = inShell(command, { encoding: "utf8", stdio: "pipe" });
	if (run.status === 0) return [];
	const lines = run.stderr.split("\n").flatMap((l) => /^\s+- (.*\S)/.exec(l)?.slice(1) ?? []);
	if (lines.length === 0)
		lines.push(`${check.name} failed: ${`${run.stdout}${run.stderr}`.trim()}`);
	return lines.map((line) => problem(check.name, line));
}

// The commit-msg rule on the staged files: the message's type against the
// staged paths, then the kinds' staged range checks, then their built-in
// moves rule (outside feat and fix, live scenarios may only move between
// files, unchanged). The paths and the staged commands are nothing for a type
// with no path rule (merges, reverts and unknown types are the header
// lint's); the moves rule judges the types it says it judges (moves.ts).
export function stagedIssues(message: string): Problem[] {
	const type = /^(\w+)/.exec(message)?.[1] ?? "";
	const ruled = !!(section("commits").scopes ?? {})[type];
	return [
		...(ruled ? scopeIssues(type, stagedFiles()) : []),
		...(ruled ? stagedChecks(type).flatMap((c) => stagedCheckIssues(c, type)) : []),
		...stagedMoveIssues(type),
	];
}

// The hook's first half (`itos hook commit-msg` runs it, then the header
// lint): 0 when the staged files hold, else the rejection and 1.
export function hook(messageFile: string): number {
	const found = stagedIssues(readFileSync(messageFile, "utf8"));
	if (found.length) reject(found);
	return found.length ? 1 : 0;
}
