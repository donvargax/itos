// The whole CI pipeline as one local command, so the workflows are thin
// wrappers and everything they do can be run and checked on a workstation.
//
//   itos ci run                      every step, the whole E2E suite included
//                                    (what `vp run ci` does)
//   itos ci run <from> <to>          a push's run: every step, with one run of
//                                    named tests over the smoke set and what
//                                    the range names, and the named tasks'
//                                    checks CI has not just run (ci-plan.ts),
//                                    in cost order: the static ones first, the
//                                    ones that may need the build or the push
//                                    last. A prose-only range runs the prose
//                                    steps and only the named tasks' static
//                                    and `prose: true` checks
//   itos ci run --nightly            the nightly: its steps (every scenario,
//                                    then the checks a push leaves to it)
//   itos ci scope <from> <to>        print the range's scope, for the workflow
//   itos ci plan <from> <to> | --nightly | --whole [--json] [--data-at <sha>]
//                                    print the plan the run would carry out,
//                                    and run nothing; `--data-at` reads the
//                                    ledger, the registry and the smoke set at
//                                    that commit
//   itos ci range --head <sha> [--base <sha>]
//                                    print `FROM=<sha>`, where the range
//                                    starts: the base of a pull request, else
//                                    what itos.yaml's `ci.range` provider says
//                                    if it is an ancestor of the head, else
//                                    empty (run everything)
import { spawnSync } from "node:child_process";
import { ciPlan, commandOf, type Plan, type PlannedCheck, planFor, planWith } from "./ci-plan.ts";
import { planData, planJson, rangeJson } from "./ci-plan-json.ts";
import { changedIn, docsOnly, rangeStart } from "./ci-scope.ts";
import { runCheck } from "./checks.ts";
import { section } from "./config.ts";
import { emit, logger, type Output, TEXT } from "./problem.ts";
import { rangeProvider } from "./providers.ts";

// `ci range --head <sha> [--base <sha>]`: where the range starts.
export async function ciRange(head: string, base: string, out: Output = TEXT): Promise<number> {
	const from = await rangeStart({ head, base, lastGreen: rangeProvider() });
	if (out.json) emit({ from });
	else console.log(`FROM=${from}`);
	return 0;
}

// `ci scope <from> <to>`: whether the range is prose only.
export function ciScope(from: string, to: string, out: Output = TEXT): number {
	const docs = docsOnly(changedIn(from, to));
	if (out.json) emit({ docs_only: docs });
	else console.log(`docs_only=${docs}`);
	return 0;
}

export interface PlanRequest {
	from: string;
	to: string;
	nightly: boolean;
	dataAt?: string;
}

// `ci plan`: the plan the run would carry out; runs nothing.
export async function ciPlanCommand(
	{ from, to, nightly, dataAt }: PlanRequest,
	out: Output = TEXT,
): Promise<number> {
	const plan = nightly
		? ciPlan({ known: false, nightly })
		: planWith(from, to, await planData(dataAt));
	const json = planJson(plan, rangeJson(from, to, nightly));
	if (out.json) console.log(JSON.stringify(json, null, 2));
	else
		for (const item of json.order)
			console.log(
				"step" in item
					? `${item.cost}  ${item.step}`
					: `${item.cost}  ${item.check.task}: ${item.check.command}   (${item.action})`,
			);
	return 0;
}

// Where a run stopped: a step (whose exit code is the run's) or a task check.
type Failure = { code: number } & (
	| { step: string }
	| { task: string; command: string }
	| { unknown: string[] }
);

// One of the run's steps: its exit code is the run's.
function runStep(step: string, out: Output): Failure | undefined {
	logger(out)(`\n$ ${step}`);
	const { status } = spawnSync("sh", ["-c", step], {
		stdio: ["inherit", out.json ? 2 : "inherit", "inherit"],
	});
	if (status === 0) return undefined;
	console.error(`\nCI failed at: ${step}`);
	return { step, code: status ?? 1 };
}

// One check of a task the range's commits name, unless a step of this run has
// just done it.
function runTaskCheck(planned: PlannedCheck, out: Output): Failure | undefined {
	const log = logger(out);
	const command = commandOf(planned);
	log(`\n${planned.task}`);
	if (planned.merged) log(`  = ${command}   (in the E2E run above)`);
	else if (planned.coveredBy) log(`  = ${command}   (ran above as \`${planned.coveredBy}\`)`);
	else if (planned.nightly) log(`  = ${command}   (runs in the nightly)`);
	else if (runCheck(planned.check, true, out.json) === "fail") {
		console.error(`\nCI failed at ${planned.task}'s check: ${command}`);
		return { task: planned.task, command, code: 1 };
	}
	return undefined;
}

// What the run says before its first item: unknown tasks (which end it),
// tasks not started, and what a prose-only range leaves out.
function preamble(plan: Plan, out: Output): Failure | undefined {
	const log = logger(out);
	// A footer naming a task no task file has is the cheapest failure there is.
	for (const id of plan.unknown) console.error(`No task ${id} in tasks/, though a footer names it`);
	if (plan.unknown.length) {
		console.error(`\nCI failed at the tasks named: ${plan.unknown.join(", ")}`);
		return { unknown: plan.unknown, code: 1 };
	}
	for (const id of plan.notStarted ?? [])
		log(`${id} is named but not started (todo in docs/work-items.yaml): its checks wait.`);
	if (plan.prose) {
		log("Only prose changed: `vp check`, and the named tasks' static and `prose: true` checks.");
		for (const planned of plan.leftOut)
			log(`  - ${planned.task}: ${commandOf(planned)}   (reads no prose; left out)`);
	}
	return undefined;
}

// `ci run [<from> <to>] | --nightly`: the plan, carried out in cost order.
// The first failure ends the run, so a static check that fails does so
// before the unit tests, the build and the run of named tests. A failing step
// exits with its own code.
export function ciRun(from: string, to: string, nightly: boolean, out: Output = TEXT): number {
	const plan: Plan = nightly ? ciPlan({ known: false, nightly }) : planFor(from, to);
	// Every step and every task check sees CI's settings (`ci.env`: Playwright's
	// full report, among others).
	Object.assign(process.env, section("ci").env ?? {});
	let failed = preamble(plan, out);
	for (const item of plan.order) {
		if (failed) break;
		failed = "step" in item ? runStep(item.step, out) : runTaskCheck(item, out);
	}
	if (!failed) logger(out)("\nCI passed");
	if (out.json) emit({ ok: !failed, ...(failed ? { failed_at: failed } : {}) });
	return failed?.code ?? 0;
}
