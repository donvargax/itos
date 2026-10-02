// What a CI run runs, decided once so that ci.ts, `itos ci plan` and the
// self-tests read the same answer.
//
//   - A push whose range CI can read runs one run of the kind of named tests
//     its `tests:` step names, over the kind's smoke set (tests.<kind>.smoke.file), the
//     tests its footers name (`Scenarios:`) and the subsets in the `done_when`
//     of the tasks its ledger footers (`Task:`) name. A range it can't read runs every
//     test, and the nightly runs every test and nothing else. Which command a
//     check is and which command runs the selections are the kind's templates
//     (tests.ts). A task check that is a run of that kind merges into the
//     run, unless the plan has no such run (nothing at all selected): then
//     it runs as itself.
//   - A named task's check that is one of the steps this run has just run
//     (`vp build`, `vp check`, …), or `vp test run` (whole or narrowed to
//     paths) when the whole unit suite has run with coverage, is not run again.
//     Every other check runs as before. `tools/bin/itos task <id>` still runs them all.
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
//   - The nightly runs its own steps (`ci.nightly.steps`) in the order they
//     are written. Its `{ tasks: done }` step stands for the checks of every
//     task whose work item is `done` in the registry, in cost order where it
//     is written (with `cost: static`, only the static ones): a done task's
//     check runs though no push names it, so a change elsewhere that breaks
//     it shows the next morning. A task in progress, or with no work item, is
//     left out, as its checks may be red until it lands. A check one of the
//     nightly's steps has done is not run again, and one that is a run of
//     the kind the nightly runs whole is in that run.
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
import { smokeIds } from "./smoke-rule.ts";
import { loadSmoke } from "./smoke.ts";
import { config, normal, readings, section, type StepConfig } from "./config.ts";
import { type Costed, costedChecks, costOf } from "./cost.ts";
import { type Check, loadTasks, type Task } from "./repo.ts";
import { commandFor, recognize, type Selection } from "./tests.ts";
import { itemStatuses, registryPath } from "./work.ts";
import { readFileSync } from "node:fs";
import { parse } from "yaml";

// The kind of named tests CI's `tests:` step runs, which a push narrows to a
// selection, if it has one.
const testsKind = () => ciSteps().find((step) => step.tests)?.tests;
// That kind's smoke set, in the working tree or at a commit; none when no step
// runs named tests, so a CI without them needs no kind and no smoke set.
export const planSmoke = (at?: string): string[] => {
	const kind = testsKind();
	return kind ? smokeIds(loadSmoke(kind, { at }), kind) : [];
};
// Checks a push leaves to the nightly, which runs them after the whole suite.
const nightlyOnly = () => section("ci").nightly_only ?? [];

export interface PlannedCheck extends Costed {
	task: string;
	// The task's title, which a failure names.
	title?: string;
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
	// The tasks the range's ledger footers name; for the nightly, the tasks
	// whose work item is done.
	tasks?: Task[];
	nightly?: boolean;
	// The kind's smoke set (tests.<kind>.smoke.file), without the tag prefix.
	smoke?: string[];
}

// The step that has done what a check does: the same command, or one of
// `ci.covers` (`vp test run`, whole or narrowed to paths, is a part of the
// unit suite `vp run test:coverage` ran), its pattern tried on each of the
// check's readings, so one written for itos covers a check that calls hooks.bin.
function coveredBy(command: string, steps: string[]): string | undefined {
	const c = normal(command);
	if (steps.includes(c)) return c;
	const forms = readings(command).map(normal);
	return (section("ci").covers ?? []).find(
		(rule) => steps.includes(rule.by) && forms.some((form) => new RegExp(rule.matches).test(form)),
	)?.by;
}

// Whether a check is one `ci.nightly_only` lists, an entry written for itos
// naming a check that calls hooks.bin too.
const leftToNightly = (command: string) =>
	readings(command).some((form) => nightlyOnly().includes(normal(form)));

// Each task check that is a run of the kind, with its selection.
function runsOfKind(
	checks: PlannedCheck[],
	kind: string,
	smoke: string[],
): { planned: PlannedCheck; selection: Selection }[] {
	return checks.flatMap((planned) => {
		const selection = planned.check.run ? recognize(kind, planned.check.run, smoke) : undefined;
		return selection ? [{ planned, selection }] : [];
	});
}

// Mark each other check that a step has done, or, in a push, that the
// nightly runs.
function markDone(checks: PlannedCheck[], steps: string[], nightly = false) {
	for (const planned of checks) {
		if (planned.merged || !planned.check.run) continue;
		planned.coveredBy = coveredBy(planned.check.run, steps);
		planned.nightly = !nightly && !planned.coveredBy && leftToNightly(planned.check.run);
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
	smoke = planSmoke(),
}: PlanInput): Plan {
	const base = { prose, tasks: tasks.map((t) => t.id), leftOut: [], unknown: [] };
	const costed = (step: Step) => ({ step: step.command, ...costOf(step.command, step.cost) });
	if (nightly) return nightlyPlan(base, tasks, smoke);
	const named: PlannedCheck[] = tasks.flatMap(costedChecks);
	const checks = prose ? named.filter(runsOnProse) : named;
	const leftOut = prose ? named.filter((planned) => !runsOnProse(planned)) : [];
	const kind = testsKind();
	const merging = kind ? runsOfKind(checks, kind, smoke) : [];
	const testsRun = kind
		? commandFor(kind, [
				...ownSelection(prose, known, scenarios, smoke),
				...merging.map((m) => m.selection),
			])
		: undefined;
	// A check is merged only into a run that happens: with nothing selected
	// (an empty smoke set, a range that names no test) it runs as itself.
	if (testsRun)
		for (const { planned } of merging) {
			planned.merged = true;
			planned.kind = kind;
		}
	// The kind's run takes the merged selection's command, and keeps its cost.
	const runs: Step[] = (
		prose ? PROSE_STEPS().map((command): Step => ({ command })) : ciSteps()
	).flatMap((step) => (step.tests ? (testsRun ? [{ ...step, command: testsRun }] : []) : [step]));
	if (prose && testsRun) runs.push({ command: testsRun, tests: kind });
	const steps = runs.map((s) => s.command);
	markDone(checks, steps);
	return { ...base, order: inCostOrder(runs.map(costed), checks), steps, checks, leftOut };
}

// Whether a nightly step is the one that runs the done tasks' checks.
const isTasksStep = (step: string | StepConfig): step is StepConfig =>
	typeof step === "object" && step.tasks !== undefined;

// The nightly's plan: its steps in written order, its tasks step standing for
// the checks of `done`, static then late (only the static ones with
// `cost: static`). Each is merged into the nightly's whole run of its kind,
// covered by one of its steps, or run.
function nightlyPlan(base: Pick<Plan, "leftOut" | "unknown">, done: Task[], smoke: string[]): Plan {
	const order: PlanItem[] = [];
	const steps: string[] = [];
	const checks: PlannedCheck[] = [];
	let kind: string | undefined;
	for (const configured of section("ci").nightly?.steps ?? []) {
		if (isTasksStep(configured)) {
			const planned: PlannedCheck[] = done
				.flatMap(costedChecks)
				.filter((p) => configured.cost !== "static" || p.cost === "static");
			checks.push(...planned);
			order.push(...inCostOrder([], planned));
			continue;
		}
		const step = stepOf(configured);
		kind ??= step.tests;
		steps.push(step.command);
		order.push({ step: step.command, ...costOf(step.command, step.cost) });
	}
	if (kind)
		for (const { planned } of runsOfKind(checks, kind, smoke)) {
			planned.merged = true;
			planned.kind = kind;
		}
	markDone(checks, steps, true);
	const tasks = [...new Set(checks.map((p) => p.task))];
	return { ...base, prose: false, order, steps, tasks, checks };
}

// The tasks whose work item is `done` in the registry, for the nightly's
// tasks step; none, and the ledger unread, when the nightly has no such step.
export function doneTasks(registry = registryPath()): Task[] {
	if (!(section("ci").nightly?.steps ?? []).some(isTasksStep)) return [];
	const statuses = itemStatuses(registry);
	return loadTasks().filter((task) => statuses.get(task.id) === "done");
}

// The nightly's plan, the done tasks read from the ledger and the registry.
export const planNightly = (): Plan => ciPlan({ known: false, nightly: true, tasks: doneTasks() });

// The plan for a pushed range, read from git and the task files.
export const planFor = (
	from: string,
	to: string,
	root?: string,
	registry = section("work").registry,
): Plan =>
	planWith(from, to, { tasks: loadTasks(root), todo: notStartedIn(registry), smoke: planSmoke() });

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
		const waiting = config().ci.wait_on_status;
		return new Set((items ?? []).filter((i) => waiting.includes(i.status)).map((i) => i.id));
	} catch {
		return new Set();
	}
}

// The step that is the run's one run of named tests, if it has one.
export const namedTestsStep = (plan: Plan) => {
	const kind = testsKind();
	const whole = kind && section("tests")[kind]?.run?.whole;
	return whole ? plan.steps.find((s) => s.startsWith(whole)) : undefined;
};
