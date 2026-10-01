// How much of CI a pushed range has to run. A change that can only break its
// own formatting doesn't need the build, the suites, the audit or a browser.
import { spawnSync } from "node:child_process";
import { type Cost, matchesAny, section, type StepConfig } from "./config.ts";
import { footerIdsIn, footers } from "./footers.ts";

// Every step of CI, in order (what `vp run ci` runs), from itos.yaml's
// `ci.steps`. The local hooks run only what a change affects and leave the
// whole suites to these. A `tests:` step is its kind's whole run
// (`run.whole`); a push whose range CI can read runs a selection in its place
// (ci-plan.ts), and the nightly runs it whole.
export interface Step {
	command: string;
	// The named tests it runs, if it is a kind's run.
	tests?: string;
	cost?: Cost;
}
export const stepOf = (step: string | StepConfig): Step =>
	typeof step === "string"
		? { command: step }
		: step.tests !== undefined
			? { command: section("tests")[step.tests]!.run!.whole!, tests: step.tests, cost: step.cost }
			: { command: step.run!, cost: step.cost };
export const ciSteps = (): Step[] => section("ci").steps.map(stepOf);
export const STEPS = ciSteps().map((s) => s.command);

// Prose can only break its own formatting, and the prose steps
// (`ci.prose.steps`) are what read it. Generated documentation is written on
// demand into an ignored folder, not committed, so no committed prose can
// disagree with the code.
export const PROSE_STEPS = (): string[] => section("ci").prose?.steps ?? [];

// Prose (`ci.prose.paths`): Markdown anywhere, and everything under docs/.
// `tasks/**` and `e2e/features/**` are deliberately absent, though the
// pre-commit hook skips them as well — `vp run ci` runs the task checks and
// every scenario, so a change to either can turn CI red and has to be checked.
export const docsOnly = (paths: string[]) => {
	const prose = section("ci").prose?.paths;
	return !!prose && paths.length > 0 && paths.every((p) => matchesAny(p, prose));
};

// The files a pushed range touched. Anything we can't read — a first push, a
// shallow clone, a rewritten history — comes back empty, and empty means "run
// everything", so the shortcut is never taken on a guess.
export function changedIn(from: string, to: string): string[] {
	if (!from || !to) return [];
	const { status, stdout } = spawnSync("git", ["diff", "--name-only", from, to], {
		encoding: "utf8",
	});
	return status === 0 ? stdout.split("\n").filter(Boolean) : [];
}

// The tasks a pushed range's commits name in their `Task:` footers, read by
// the footer reader (footers.ts). Their checks are CI's: the pre-push hook
// does not run them, so the push stays quick on a shared machine. A range we
// can't read gives none.
export const tasksIn = (from: string, to: string) => footerIdsIn(from, to, "Task");

// The tests of a kind a pushed range's footers name (`Scenarios:` for the
// scenario kind), without their prefix, each once: a push's run
// of the kind is its smoke set and these.
export const testsNamedIn = (from: string, to: string, kind: string) => [
	...new Set(
		footers()
			.filter((f) => typeof f.source === "object" && f.source.tests === kind)
			.flatMap((f) => footerIdsIn(from, to, f.key)),
	),
];

// Whether a range can be read at all. One that can't (a first push, an empty
// start, a rewritten history) runs every scenario, as it runs everything else.
export function readable(from: string, to: string): boolean {
	if (!from || !to) return false;
	return spawnSync("git", ["rev-list", "--quiet", `${from}..${to}`]).status === 0;
}

// Where a CI run's range starts. A newer push cancels a waiting run, so
// a push to main is checked from the head of the last green run on main, not
// from the push before it: the newest run covers every cancelled one's commits.
// A pull request keeps its base. With no green run, or one whose commit is not
// an ancestor of the head (a rewritten history), the start is empty, and empty
// means "run everything" to changedIn, tasksIn and verify-commits alike.
export interface RangeInput {
	head: string;
	// The pull request's base commit; empty for a push.
	base?: string;
	// The start the range provider proposes (`ci.range`, providers.ts):
	// for `github` the head commit of the last successful run on main. Undefined
	// runs everything.
	lastGreen: () => Promise<string | undefined>;
	// The repository to read the ancestry in (the current directory by default).
	cwd?: string;
}
export async function rangeStart({ head, base, lastGreen, cwd }: RangeInput): Promise<string> {
	if (base) return base;
	const green = await lastGreen().catch(() => undefined);
	if (!green || !head) return "";
	const { status } = spawnSync("git", ["merge-base", "--is-ancestor", green, head], { cwd });
	return status === 0 ? green : "";
}
