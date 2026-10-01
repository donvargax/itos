// The task runner: `itos task <id>… | --group <g> [--skip <ids>] | --pending`
// and `itos task list` (main.ts dispatches them; `vp run task` runs the first).
import { runCheck } from "./checks.ts";
import { ledgerLayout, section } from "./config.ts";
import { emit, type Output, TEXT } from "./problem.ts";
import { loadTasks, type Task } from "./repo.ts";

type Status = "done" | "pending" | "failing" | "review";
type Result = "pass" | "fail" | "pending";

function runTask(task: Task, verbose: boolean, out: Output): { status: Status; results: Result[] } {
	if (task.done_when.length === 0) return { status: "review", results: [] };
	if (verbose) (out.json ? console.error : console.log)(`\n${task.id} ${task.title}`);
	const results = task.done_when.map((c) => runCheck(c, verbose, out.json));
	const status = results.includes("fail")
		? "failing"
		: results.includes("pending")
			? "pending"
			: "done";
	return { status, results };
}

const flag = (args: string[], name: string) => {
	const i = args.indexOf(name);
	return i >= 0 ? args[i + 1] : undefined;
};

// The group a `--group` (or `--phase`) names, as the ledger's tasks hold it.
const groupOf = (value: string) => (ledgerLayout().numeric ? Number(value) : value);

interface TaskOptions {
	out?: Output;
}

function select(args: string[], tasks: Task[]): Task[] {
	const group = flag(args, "--group") ?? flag(args, "--phase");
	const skip = new Set(flag(args, "--skip")?.split(",") ?? []);
	let selected: Task[];
	if (group !== undefined) selected = tasks.filter((t) => t.phase === groupOf(group));
	else if (args.includes("--pending")) selected = tasks;
	else selected = tasks.filter((t) => args.includes(t.id));
	return selected.filter((t) => !skip.has(t.id));
}

// The IDs named on the command line that the ledger does not have.
function unknownIds(args: string[], tasks: Task[]): string[] {
	const pattern = new RegExp(`^(?:${section("ledger").id ?? "T-\\d+"})$`);
	const values = new Set(["--group", "--phase", "--skip"].map((f) => flag(args, f)));
	return args.filter((a) => pattern.test(a) && !values.has(a) && !tasks.some((t) => t.id === a));
}

// `task <id>…`: runs the tasks' checks in written order, verbose for one task,
// and prints the status table. 1 when a check fails or an ID is unknown, 2
// when nothing matches.
export function runTasks(args: string[], { out = TEXT }: TaskOptions = {}): number {
	const tasks = loadTasks();
	const unknown = unknownIds(args, tasks);
	for (const id of unknown) console.error(`No task ${id} in ${ledgerLayout().dir}/`);
	if (unknown.length) return 1;
	const selected = select(args, tasks);
	if (selected.length === 0) {
		console.error(
			"No matching tasks. Usage: itos task <id>… | --group <g> [--skip <ids>] | --pending",
		);
		return 2;
	}
	const verbose = selected.length === 1;
	const rows = selected.map((t) => ({ task: t, ...runTask(t, verbose, out) }));
	const shown = args.includes("--pending") ? rows.filter((r) => r.status !== "done") : rows;
	if (out.json)
		emit({
			tasks: shown.map(({ task, status, results }) => ({
				id: task.id,
				type: task.type,
				title: task.title,
				group: task.phase,
				status,
				checks: task.done_when.map((check, i) => ({
					command: check.run ?? check.fails,
					...(check.fails === undefined ? {} : { fails: true }),
					result: results[i] ?? "review",
				})),
			})),
		});
	else {
		console.log("");
		for (const { task, status } of shown)
			console.log(`${status.padEnd(8)} ${task.id}  ${task.type.padEnd(8)} ${task.title}`);
	}
	return rows.some((r) => r.status === "failing") ? 1 : 0;
}

// `task list [--group <g>]`: the tasks, running nothing.
export function listTasks(args: string[], out: Output = TEXT): number {
	const group = flag(args, "--group") ?? flag(args, "--phase");
	const tasks = loadTasks().filter((t) => group === undefined || t.phase === groupOf(group));
	if (out.json)
		emit({
			tasks: tasks.map((t) => ({
				id: t.id,
				type: t.type,
				title: t.title,
				group: t.phase,
				checks: t.done_when.length,
			})),
		});
	else
		for (const t of tasks)
			console.log(`${t.id}  ${String(t.phase).padEnd(3)} ${t.type.padEnd(8)} ${t.title}`);
	return 0;
}
