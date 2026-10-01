// What a CI run runs, decided once so that ci.ts, `itos ci plan` and the
// self-tests read the same answer.
//
//   - A push whose range CI can read runs one run of the kind of named tests
//     its `tests:` step names, over the kind's smoke set (e2e/smoke.yaml), the
//     tests its footers name (`Scenarios:`) and the subsets in the `done_when`
//     of the tasks its `Task:` footers name. A range it can't read runs every
//     test, and the nightly runs every test and nothing else. Which command a
//     check is and which command runs the selections are the kind's templates
//     (tests.ts).
//   - A named task's check that is one of the steps this run has just run
//     (`vp build`, `vp check`, …), or `vp test run` (whole or narrowed to
//     paths) when the whole unit suite has run with coverage, is not run again.
//     Every other check runs as before. `vp run task <id>` still runs them all.
//   - A check in `ci.nightly_only` (a slow self-test of the gates, say) runs
//     only in the nightly: a push skips it even when a named task's
//     `done_when` lists it.
//   - The run is in cost order: the static steps and the named tasks'
//     static checks first, then the unit tests, the build and the audit, then
//     the run of named tests, and last the checks that may need what came before.
//     `order` is that sequence; `steps` and `checks` are its two halves, for
//     what reads one of them. A step's or a check's cost is its own `cost:`,
//     else static when one of itos.yaml's `ci.cost.static` patterns matches
//     its command, else late; with `keep_written_order`, a check below
//     a late check of its task is late too, so it never runs before a check
//     written above it.
//   - A prose-only range runs the prose steps (`ci.prose.steps`), then the
//     named tasks' static checks and those whose `done_when` entry says
//     `prose: true` (a late check that reads Markdown or `docs/**`), and
//     nothing else: no build, no subset of named tests. What a check that
//     reads only code finds cannot change with prose; `leftOut` lists what the
//     plan dropped, so the log says so.
import {
	changedIn,
	ciSteps,
	docsOnly,
	PROSE_STEPS,
	readable,
	type Step,
	stepOf,
	tasksIn,
	testsNamedIn,
} from "./ci-scope.ts";
import { smokeIds } from "./e2e-scope.ts";
import { type Cost, config, matchesStatic, normal, section } from "./config.ts";
import { type Check, loadTasks, type Task } from "./repo.ts";
import { commandFor, recognize, type Selection } from "./tests.ts";
import { readFileSync } from "node:fs";
import { parse } from "yaml";

// Where a cost came from: the item's own `cost:`, a pattern, the default
// (late), or the order of the task's checks.
export type CostFrom = "explicit" | "pattern" | "default" | "order";
export interface Costed {
	cost: Cost;
	costFrom: CostFrom;
}

// A command's cost by the config's patterns alone: the commands that need
// nothing built and take seconds. They run before the unit tests, the build
// and the run of named tests, so a push that fails one of them fails in its
// first minute, not after a long run of the scenarios. In doubt a command is
// late: it runs after everything it might need.
export const isStatic = (command: string) => matchesStatic(command);

// A step's or check's cost: its own, else the patterns', else late.
export const costOf = (command: string, own?: Cost): Costed =>
	own
		? { cost: own, costFrom: "explicit" }
		: isStatic(command)
			? { cost: "static", costFrom: "pattern" }
			: { cost: "late", costFrom: "default" };

// The kind of named tests CI's `tests:` step runs, which a push narrows to a
// selection, if it has one.
const testsKind = () => ciSteps().find((step) => step.tests)?.tests;
// Checks a push leaves to the nightly, which runs them after the whole suite.
const nightlyOnly = () => section("ci").nightly_only ?? [];

export interface PlannedCheck extends Costed {
	task: string;
	check: Check;
	// Its place in the task's `done_when`, from 0.
	index?: number;
	// Its tests are in this run's one run of their kind.
	merged?: boolean;
	// That kind, when it is merged.
	kind?: string;
	// The step of this run that already did what it does.
	coveredBy?: string;
	// Left to the nightly, which runs it every night.
	nightly?: boolean;
}
export type PlanItem = ({ step: string } & Costed) | PlannedCheck;
export interface Plan {
	prose: boolean;
	// Everything the run does, in the order it does it.
	order: PlanItem[];
	steps: string[];
	tasks: string[];
	checks: PlannedCheck[];
	// The named tasks' checks a prose-only range does not run.
	leftOut: PlannedCheck[];
	// Task IDs a footer names that no task file has.
	unknown: string[];
	// Named tasks whose work item is still `todo`: nobody has started them, so
	// their checks cannot pass yet (a ledger commit that adds a task names it).
	notStarted?: string[];
}

export interface PlanInput {
	// Only prose changed: formatting is the gate.
	prose?: boolean;
	// The range was read; false runs every scenario.
	known: boolean;
	// The kind's test IDs the range's footers name (`Scenarios:`).
	scenarios?: string[];
	// The tasks the range's `Task:` footers name.
	tasks?: Task[];
	nightly?: boolean;
	// The kind's smoke set, without the tag prefix (e2e/smoke.yaml here).
	smoke?: string[];
}

// The step that has done what a check does: the same command, or one of
// `ci.covers` (`vp test run`, whole or narrowed to paths, is a part of the
// unit suite `vp run test:coverage` ran).
function coveredBy(command: string, steps: string[]): string | undefined {
	const c = normal(command);
	if (steps.includes(c)) return c;
	return (section("ci").covers ?? []).find(
		(rule) => new RegExp(rule.matches).test(c) && steps.includes(rule.by),
	)?.by;
}

// Mark each task check that is a run of the kind as merged, and return its
// selection.
function mergeRuns(checks: PlannedCheck[], kind: string, smoke: string[]): Selection[] {
	const selections: Selection[] = [];
	for (const planned of checks) {
		const selection = planned.check.run ? recognize(kind, planned.check.run, smoke) : undefined;
		if (!selection) continue;
		selections.push(selection);
		planned.merged = true;
		planned.kind = kind;
	}
	return selections;
}

// Mark each other check that a step has done, or that the nightly runs.
function markDone(checks: PlannedCheck[], steps: string[]) {
	for (const planned of checks) {
		if (planned.merged || !planned.check.run) continue;
		planned.coveredBy = coveredBy(planned.check.run, steps);
		planned.nightly = !planned.coveredBy && nightlyOnly().includes(normal(planned.check.run));
	}
}

// The command a check runs, whichever way it reads its exit code.
export const commandOf = (planned: PlannedCheck) => planned.check.run ?? planned.check.fails!;

// The run in cost order: the static steps, the static checks, the other
// steps (the unit tests, the build, the audit, the run of named tests), then the
// checks that may need any of them.
function inCostOrder(steps: ({ step: string } & Costed)[], checks: PlannedCheck[]): PlanItem[] {
	const isStaticItem = (item: Costed) => item.cost === "static";
	return [
		...steps.filter(isStaticItem),
		...checks.filter(isStaticItem),
		...steps.filter((item) => !isStaticItem(item)),
		...checks.filter((item) => !isStaticItem(item)),
	];
}

// A task's checks with their costs. With `keep_written_order`, a check below
// a late one is late, whatever its own class: authors write a task's checks
// in the order they depend on (a check that reads a file runs after the one
// above it that writes the file).
function costedChecks(task: Task): PlannedCheck[] {
	const keep = config().ci?.cost?.keep_written_order === true;
	let late = false;
	return task.done_when.map((check, index) => {
		let costed = costOf(check.run ?? check.fails!, check.cost);
		if (keep && late && costed.cost === "static") costed = { cost: "late", costFrom: "order" };
		if (costed.cost === "late") late = true;
		return { task: task.id, check, index, ...costed };
	});
}

// A check a prose-only range still runs: one that takes seconds, or
// one whose result prose can change.
const runsOnProse = (planned: PlannedCheck) =>
	planned.cost === "static" || planned.check.prose === true;

// The run's own selection: the smoke set and the named tests, every test for
// a range it can't read, none for prose.
const ownSelection = (
	prose: boolean,
	known: boolean,
	scenarios: string[],
	smoke: string[],
): Selection[] => (prose ? [] : [known ? { ids: [...smoke, ...scenarios] } : { whole: true }]);

export function ciPlan({
	prose = false,
	known,
	scenarios = [],
	tasks = [],
	nightly = false,
	smoke = smokeIds(),
}: PlanInput): Plan {
	const base = { prose, tasks: tasks.map((t) => t.id), leftOut: [], unknown: [] };
	const costed = (step: Step) => ({ step: step.command, ...costOf(step.command, step.cost) });
	if (nightly) {
		const steps = (section("ci").nightly?.steps ?? []).map(stepOf);
		return {
			...base,
			prose: false,
			order: steps.map(costed),
			steps: steps.map((s) => s.command),
			tasks: [],
			checks: [],
		};
	}
	const named: PlannedCheck[] = tasks.flatMap(costedChecks);
	const checks = prose ? named.filter(runsOnProse) : named;
	const leftOut = prose ? named.filter((planned) => !runsOnProse(planned)) : [];
	const kind = testsKind();
	const e2e = kind
		? commandFor(kind, [
				...ownSelection(prose, known, scenarios, smoke),
				...mergeRuns(checks, kind, smoke),
			])
		: undefined;
	// The kind's run takes the merged selection's command, and keeps its cost.
	const runs: Step[] = (
		prose ? PROSE_STEPS().map((command): Step => ({ command })) : ciSteps()
	).flatMap((step) => (step.tests ? (e2e ? [{ ...step, command: e2e }] : []) : [step]));
	if (prose && e2e) runs.push({ command: e2e, tests: kind });
	const steps = runs.map((s) => s.command);
	markDone(checks, steps);
	return { ...base, order: inCostOrder(runs.map(costed), checks), steps, checks, leftOut };
}

// The plan for a pushed range, read from git and the task files.
export const planFor = (
	from: string,
	to: string,
	root?: string,
	registry = section("work").registry ?? "docs/work-items.yaml",
): Plan =>
	planWith(from, to, { tasks: loadTasks(root), todo: notStartedIn(registry), smoke: smokeIds() });

// What a plan reads besides the range's commits: the ledger, the registry's
// `todo` items and the smoke set. ci-plan-json.ts reads them at another
// commit for `ci plan --data-at`.
export interface PlanData {
	tasks: Task[];
	todo: Set<string>;
	smoke: string[];
}

// The plan for a pushed range: its commits read from git, the rest from `data`.
export function planWith(from: string, to: string, { tasks: all, todo, smoke }: PlanData): Plan {
	const ids = tasksIn(from, to);
	const plan = ciPlan({
		prose: docsOnly(changedIn(from, to)),
		known: readable(from, to),
		scenarios: testsNamedIn(from, to, testsKind() ?? ""),
		tasks: all.filter((t) => ids.includes(t.id) && !todo.has(t.id)),
		smoke,
	});
	return {
		...plan,
		unknown: ids.filter((id) => !all.some((t) => t.id === id)),
		notStarted: ids.filter((id) => todo.has(id)),
	};
}

// The work items still `todo` in the registry, or none if it can't be read.
export function notStartedIn(registry: string): Set<string> {
	try {
		const items = (
			parse(readFileSync(registry, "utf8")) as { items?: { id: string; status: string }[] }
		).items;
		const waiting = config().ci?.wait_on_status ?? ["todo"];
		return new Set((items ?? []).filter((i) => waiting.includes(i.status)).map((i) => i.id));
	} catch {
		return new Set();
	}
}

// The step that is the run's one run of named tests, if it has one.
export const e2eStep = (plan: Plan) => {
	const kind = testsKind();
	const whole = kind && section("tests")[kind]?.run?.whole;
	return whole ? plan.steps.find((s) => s.startsWith(whole)) : undefined;
};
