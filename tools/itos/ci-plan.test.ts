// A task's checks keep their written order in CI's plan: a check that reads
// what the check above it writes must not be moved before it by cost order.
// The rule itself, and a check's own `cost:` over the patterns, are
// conformance/plans.yaml's; this holds the project's own ledger to it.
import { describe, expect, it } from "vite-plus/test";
import { ciPlan, type PlannedCheck } from "./ci-plan.ts";
import { loadTasks } from "./repo.ts";

// A check the run carries out itself: not one in the run of named tests, one
// a step has done, or one left to the nightly.
const runs = (item: PlannedCheck) => !item.merged && !item.coveredBy && !item.nightly;

describe("the ledger's checks in CI's plan", () => {
	it.each(loadTasks().map((task) => [task.id, task] as const))(
		"%s runs its checks in written order",
		(_, task) => {
			const plan = ciPlan({ known: true, tasks: [task], smoke: [] });
			const order = plan.order
				.filter((item): item is PlannedCheck => !("step" in item) && runs(item))
				.map((item) => item.index ?? 0);
			expect(order).toEqual([...order].sort((a, b) => a - b));
		},
	);
});
