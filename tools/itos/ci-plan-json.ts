// The CI plan as JSON, which `itos ci plan --json` prints: the contract a
// second implementation of the tool is compared on. Keys are only ever added.
//
// Each item of `order` has a `cost` (`static` or `late`) and where it came
// from: `explicit` when its own `cost:` said so, `pattern` when one of
// the config's static patterns matched it, `default` when none did, `order`
// when a late check written above it made it late. A task check also has its `action`:
// `run`, `merged` (its tests are in the run's one run of their kind, `kind`), `covered` (a step did it, `covered_by`) or `nightly`.
import { execFileSync } from "node:child_process";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import {
	commandOf,
	type Costed,
	notStartedIn,
	type Plan,
	type PlanData,
	type PlannedCheck,
} from "./ci-plan.ts";
import { readable } from "./ci-scope.ts";
import { smokeIds } from "./e2e-scope.ts";
import { ledgerLayout, section } from "./config.ts";
import { git, loadTasks } from "./repo.ts";
import { loadSmoke } from "./smoke.ts";

export interface RangeJson {
	from: string;
	to: string;
	readable: boolean;
	nightly: boolean;
}

// A commit's full SHA, or what was given when git can't resolve it.
function fullSha(ref: string): string {
	if (!ref) return ref;
	try {
		return git("rev-parse", "--verify", "--quiet", `${ref}^{commit}`).trim() || ref;
	} catch {
		return ref;
	}
}

export const rangeJson = (from: string, to: string, nightly = false): RangeJson => ({
	from: fullSha(from),
	to: fullSha(to),
	readable: !nightly && readable(from, to),
	nightly,
});

const cost = (item: Costed) => ({ cost: item.cost, cost_from: item.costFrom });

const checkJson = (planned: PlannedCheck) => ({
	task: planned.task,
	index: planned.index ?? 0,
	command: commandOf(planned),
	...(planned.check.fails === undefined ? {} : { fails: true }),
});

function action(planned: PlannedCheck) {
	if (planned.merged) return { action: "merged", kind: planned.kind };
	if (planned.coveredBy) return { action: "covered", covered_by: planned.coveredBy };
	if (planned.nightly) return { action: "nightly" };
	return { action: "run" };
}

export function planJson(plan: Plan, range: RangeJson) {
	return {
		schema: 1,
		range,
		prose: plan.prose,
		steps: plan.steps,
		order: plan.order.map((item) =>
			"step" in item
				? { step: item.step, ...cost(item) }
				: { check: checkJson(item), ...cost(item), ...action(item) },
		),
		tasks: plan.tasks,
		left_out: plan.leftOut.map(checkJson),
		unknown: plan.unknown,
		not_started: plan.notStarted ?? [],
	};
}

// What a plan reads besides the range's commits, from the working tree, or
// with `at` from that commit's tree: the ledger (`ledger.files`), the registry
// (`work.registry`) and the smoke set (the scenario kind's `smoke.file`,
// through smoke.ts). The feature
// files are not read by the plan; the config is always the working tree's.
export async function planData(at?: string): Promise<PlanData> {
	const registry = section("work").registry ?? "docs/work-items.yaml";
	if (!at) return { tasks: loadTasks(), todo: notStartedIn(registry), smoke: smokeIds() };
	const dir = mkdtempSync(join(tmpdir(), "plan-data-"));
	const ledger = ledgerLayout();
	try {
		const paths = git("ls-tree", "-r", "--name-only", at, "--", ledger.dir, registry)
			.split("\n")
			.filter(
				(p) =>
					p === registry ||
					(dirname(p) === ledger.dir && ledger.file.test(p.slice(ledger.dir.length + 1))),
			);
		for (const path of paths) {
			mkdirSync(join(dir, dirname(path)), { recursive: true });
			writeFileSync(join(dir, path), execFileSync("git", ["show", `${at}:${path}`]));
		}
		return {
			tasks: loadTasks(join(dir, ledger.dir)),
			todo: notStartedIn(join(dir, registry)),
			smoke: smokeIds(loadSmoke("scenario", { at })),
		};
	} finally {
		rmSync(dir, { recursive: true, force: true });
	}
}
