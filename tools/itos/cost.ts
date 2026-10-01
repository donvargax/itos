// The cost rule, one for CI's plan (ci-plan.ts) and the commit-msg hook's run
// of the named tasks' checks (commit-tasks.ts): a step's or a check's cost
// class is its own `cost:`, else static when one of `ci.cost.static`'s
// patterns matches its command, else late; with `ci.cost.keep_written_order`,
// a check below a late check of its task is late too.
import { type Cost, config, matchesStatic } from "./config.ts";
import type { Check, Task } from "./repo.ts";

// Where a cost came from: the item's own `cost:`, a pattern, the default
// (late), or the order of the task's checks.
type CostFrom = "explicit" | "pattern" | "default" | "order";
export interface Costed {
	cost: Cost;
	costFrom: CostFrom;
}

// A command's cost by the config's patterns alone: the commands that need
// nothing built and take seconds. They run before the unit tests, the build
// and the run of named tests, so a push that fails one of them fails in its
// first minute, not after a long run of the scenarios. In doubt a command is
// late: it runs after everything it might need.
const isStatic = (command: string) => matchesStatic(command);

// A step's or check's cost: its own, else the patterns', else late.
export const costOf = (command: string, own?: Cost): Costed =>
	own
		? { cost: own, costFrom: "explicit" }
		: isStatic(command)
			? { cost: "static", costFrom: "pattern" }
			: { cost: "late", costFrom: "default" };

interface CostedCheck extends Costed {
	task: string;
	check: Check;
	// Its place in the task's `done_when`, from 0.
	index: number;
}

// A task's checks with their costs. With `keep_written_order`, a check below
// a late one is late, whatever its own class: authors write a task's checks
// in the order they depend on (a check that reads a file runs after the one
// above it that writes the file).
export function costedChecks(task: Task): CostedCheck[] {
	const keep = config().ci?.cost?.keep_written_order === true;
	let late = false;
	return task.done_when.map((check, index) => {
		let costed = costOf(check.run ?? check.fails!, check.cost);
		if (keep && late && costed.cost === "static") costed = { cost: "late", costFrom: "order" };
		if (costed.cost === "late") late = true;
		return { task: task.id, check, index, ...costed };
	});
}

// A task's checks that CI runs before any late one: its checks in written
// order, up to its first late one. The commit-msg hook runs these, so a
// commit is held only by what takes seconds.
export function checksBeforeLate(task: Task): CostedCheck[] {
	const costed = costedChecks(task);
	const late = costed.findIndex((planned) => planned.cost === "late");
	return late < 0 ? costed : costed.slice(0, late);
}
