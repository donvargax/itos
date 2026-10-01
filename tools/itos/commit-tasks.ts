// The commit-msg hook's last rule: the static checks of the tasks a commit
// names. For each task a ledger footer (`Task:`) names, its checks run in
// written order up to its first late one, by the cost rule CI uses
// (cost.ts), so a commit is held only by what takes
// seconds. Each runs through the shell, as `itos task` runs it, quietly, its
// timeout capped by `hooks.commit_msg.check_timeout`; an `after: push` check
// is pending, since the commit being made is not pushed. A task stops at its
// first failure.
//
// A failure rejects the commit when the task's work item is done: a finished
// task that fails is a regression. For any other status, or a task with no
// item, the failure is reported and the commit goes through: a task in
// progress is committed in steps, and CI judges what is pushed. The ledger
// and the registry are read from the staged tree, as the footers and the
// data check read them (source.ts); the hook's own keys from the config the
// other rules read. `hooks.commit_msg.task_checks: false` turns it off.
import { readFileSync } from "node:fs";
import { type Captured, runCheckCaptured } from "./checks.ts";
import { config, DEFAULT_COMMIT_CHECK_TIMEOUT } from "./config.ts";
import { checksBeforeLate } from "./cost.ts";
import { footerIds, footers } from "./footers.ts";
import { type Check, loadTasks, type Task } from "./repo.ts";
import { readingFrom, treeSource } from "./source.ts";
import { itemStatus, registryPath } from "./work.ts";

// The task IDs the message's ledger footers name, each once, in order.
const namedTasks = (message: string): string[] => [
	...new Set(
		footers()
			.filter((f) => f.source === "ledger")
			.flatMap((f) => footerIds(message, f.key)),
	),
];

interface Named {
	task: Task;
	// Its work item's status, as staged; undefined when it has none.
	status?: string;
	registry: string;
}

// The named tasks the staged ledger has, with their items' statuses. A
// staged tree with no ledger names none (the header lint has judged the
// footer by then).
function stagedTasks(ids: string[]): Named[] {
	return readingFrom(treeSource("index"), () => {
		let tasks: Task[];
		try {
			tasks = loadTasks();
		} catch {
			return [];
		}
		const registry = registryPath();
		return ids.flatMap((id) => {
			const task = tasks.find((t) => t.id === id);
			return task ? [{ task, status: itemStatus(id, registry), registry }] : [];
		});
	});
}

// The environment a check runs in: the hook's, less the index git made the
// commit from, so that a check's own git commands (in a scratch repository
// too) never read or write the commit's index, as under `itos task`.
function checkEnv(): NodeJS.ProcessEnv {
	const env = { ...process.env };
	delete env.GIT_INDEX_FILE;
	return env;
}

// What went wrong with a failed check, in a few words.
function failure(check: Check, run: Captured): string {
	const command = `\`${check.run ?? check.fails}\``;
	if (run.timedOut)
		return run.capped
			? `${command} ran past hooks.commit_msg.check_timeout (${run.seconds}s)`
			: `${command} ran past its timeout (${run.seconds}s)`;
	if (check.fails && run.status === 0) return `${command} passed, and must fail`;
	return run.status === null ? `${command} was stopped` : `${command} exited ${run.status}`;
}

// One task's checks before its first late one; the first failure, if any,
// with its output printed under the task's title and the check, as `itos
// task` prints them.
function firstFailure({ task }: Named, cap: number, env: NodeJS.ProcessEnv): string | undefined {
	for (const { check } of checksBeforeLate(task)) {
		if (check.after === "push") continue;
		const run = runCheckCaptured(check, cap, env);
		if (run.result === "pass") continue;
		console.error(`${task.id} ${task.title}`);
		console.error(`  $ ${check.run ?? check.fails}${check.fails ? "   (must fail)" : ""}`);
		if (run.output)
			process.stderr.write(run.output.endsWith("\n") ? run.output : `${run.output}\n`);
		return failure(check, run);
	}
	return undefined;
}

// What one failing task does to the commit: when its item is done, the line it
// adds to the rejection; else its report, printed now, and nothing.
function verdict({ task, status, registry }: Named, why: string): string[] {
	if (status === "done")
		return [`failing ${task.id}: ${why}, and ${registry} says ${task.id} is done`];
	const where = `${status ? `is ${status}` : "has no item"} in ${registry}`;
	console.error(
		`failing ${task.id}: ${why}; ${task.id} ${where}, so the commit goes through, and CI judges the push`,
	);
	return [];
}

// The rule: 0 when every named task's checks pass, or every one that fails is
// not done; else the rejection, one line per done task that fails, and 1.
export function hook(messageFile: string): number {
	const settings = config().hooks?.commit_msg ?? {};
	const ids = settings.task_checks === false ? [] : namedTasks(readFileSync(messageFile, "utf8"));
	if (!ids.length) return 0;
	const cap = settings.check_timeout ?? DEFAULT_COMMIT_CHECK_TIMEOUT;
	const env = checkEnv();
	const rejected = stagedTasks(ids).flatMap((named) => {
		const why = firstFailure(named, cap, env);
		return why ? verdict(named, why) : [];
	});
	if (!rejected.length) return 0;
	console.error(config().commits?.reject_message ?? "Commit rejected:");
	for (const line of rejected) console.error(`  - ${line}`);
	return 1;
}
