// Repository readers shared by the hooks and the task runner.
import { execFileSync, spawnSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { basename, dirname } from "node:path";
import { parse } from "yaml";
import { config, type Cost, ledgerFiles, ledgerLayout } from "./config.ts";
import { problem, type Problem } from "./problem.ts";

export interface Check {
	run?: string;
	fails?: string;
	after?: "push";
	timeout?: number;
	// Its result can change with Markdown or `docs/**` alone, so a
	// prose-only push runs it though it is not static.
	prose?: boolean;
	// Its cost class, which wins over the config's patterns.
	cost?: Cost;
}
export interface Task {
	id: string;
	type: string;
	title: string;
	why?: string;
	done_when: Check[];
	phase: number;
}

export const git = (...args: string[]) =>
	execFileSync("git", args, { encoding: "utf8", stdio: ["ignore", "pipe", "ignore"] });

// The ledger's tasks: the files `ledger.files` names in its folder, or in
// `root` (a copy of the folder, a scratch ledger). The group is the phase.
export function loadTasks(root?: string): Task[] {
	const tasks: Task[] = [];
	const { numeric } = ledgerLayout();
	for (const { path, group } of ledgerFiles(root)) {
		const phase = (numeric ? Number(group) : group) as number;
		for (const task of (parse(readFileSync(path, "utf8")) ?? []) as Omit<Task, "phase">[])
			tasks.push({ ...task, done_when: task.done_when ?? [], phase });
	}
	return tasks;
}

// The files of a tree under `root` as { path: text }: a commit, or "index"
// for the staged tree. A tree that cannot be read (no HEAD yet) is empty.
export function treeTexts(
	tree: string,
	root: string,
	keep: (file: string) => boolean,
): Record<string, string> {
	const files: Record<string, string> = {};
	try {
		const listed =
			tree === "index"
				? git("ls-files", "--cached", "--", root)
				: git("ls-tree", "-r", "--name-only", tree, "--", root);
		for (const file of listed.split("\n").filter((f) => f && keep(f)))
			files[file] = git("show", tree === "index" ? `:${file}` : `${tree}:${file}`);
	} catch {
		return {};
	}
	return files;
}

// The ledger's task IDs: the working tree's, or a tree's when one is given
// (as a kind's tests are read, tests.ts). A ledger file that cannot be
// parsed at that tree gives none.
export function ledgerIds(at?: string): Set<string> {
	if (!at) return new Set(loadTasks().map((t) => t.id));
	const { dir, file } = ledgerLayout();
	const ids = new Set<string>();
	const texts = treeTexts(at, dir, (path) => dirname(path) === dir && file.test(basename(path)));
	for (const text of Object.values(texts)) {
		let tasks: unknown;
		try {
			tasks = parse(text);
		} catch {
			continue;
		}
		if (Array.isArray(tasks))
			for (const task of tasks) if (typeof task?.id === "string") ids.add(task.id);
	}
	return ids;
}

// commits.since, if the config names one: the commit where verification
// starts.
export const since = (): string | undefined => config().commits?.since;

// Whether the repository has the commit commits.since names, as a problem
// when it does not (a typo, a commit of another repository, a shallow clone).
export function sinceIssue(): Problem | undefined {
	const sha = since();
	if (!sha) return undefined;
	const found = spawnSync("git", ["cat-file", "-e", `${sha}^{commit}`], { stdio: "ignore" });
	if (found.status === 0) return undefined;
	return problem(
		"config-since-commit",
		`commits.since names ${sha}, which is not a commit of this repository`,
		"set commits.since to the full SHA of a commit this repository has, or fetch its history",
	);
}

// A range's commits as `git rev-list` takes them: `from..to`, or everything up
// to `to` when `from` is empty or all zeros (a new branch), less commits.since
// and its ancestors.
export function rangeArgs(from: string, to: string): string[] {
	const sha = since();
	return [!from || /^0+$/.test(from) ? to : `${from}..${to}`, ...(sha ? [`^${sha}`] : [])];
}

// Where a range starts for a range check, which takes one `{from}`: `from`,
// or commits.since when `from` is empty or one of its ancestors.
export function rangeStartAfterSince(from: string): string {
	const sha = since();
	if (!sha) return from;
	if (!from || /^0+$/.test(from)) return sha;
	const older = spawnSync("git", ["merge-base", "--is-ancestor", from, sha], { stdio: "ignore" });
	return older.status === 0 ? sha : from;
}

export function stagedFiles(): string[] {
	return git("diff", "--cached", "--name-only", "--diff-filter=ACMRD").split("\n").filter(Boolean);
}
