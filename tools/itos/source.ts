// Where itos reads its own data: the config, the ledger, the work registry,
// the people and the smoke sets. By default that is the working tree's files;
// `readingFrom` reads them from a tree git holds instead, the index (the
// staged tree, which the commit-msg hook judges, since it is what the commit
// will hold) or a commit, for as long as a function runs. The readers in
// config.ts, work.ts, providers.ts and smoke.ts all go through `current()`, so
// `itos config check` and the hook's check of the staged data are one check.
import { execFileSync, spawnSync } from "node:child_process";
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { posix, relative, resolve, sep } from "node:path";

export interface Source {
	// "worktree", "index" or a commit, as a kind's adapter takes a tree.
	tree: string;
	has(path: string): boolean;
	// The file's text; throws when the source does not hold it.
	read(path: string): string;
	// The names of the files directly in a folder; throws when there is none.
	list(dir: string): string[];
}

export const WORKTREE: Source = {
	tree: "worktree",
	has: existsSync,
	read: (path) => readFileSync(path, "utf8"),
	list: (dir) => readdirSync(dir),
};

const git = (args: string[]) =>
	execFileSync("git", args, { encoding: "utf8", stdio: ["ignore", "pipe", "ignore"] });

// A tree git holds: "index" for the staged tree, else a commit. Paths are the
// working tree's, relative to the current folder, as everywhere else.
export function treeSource(tree: string): Source {
	const top = git(["rev-parse", "--show-toplevel"]).trim();
	const inRepo = (path: string) => relative(top, resolve(path)).split(sep).join("/") || ".";
	const spec = (path: string) => `${tree === "index" ? "" : tree}:${inRepo(path)}`;
	const name = tree === "index" ? "the index" : tree;
	return {
		tree,
		has: (path) =>
			spawnSync("git", ["cat-file", "-e", spec(path)], { stdio: "ignore" }).status === 0,
		read(path) {
			try {
				return git(["show", spec(path)]);
			} catch {
				throw new Error(`${name} holds no ${path}`);
			}
		},
		list(dir) {
			const at = inRepo(dir);
			const listed =
				tree === "index"
					? git(["ls-files", "--cached", "--", at])
					: git(["ls-tree", "-r", "--name-only", tree, "--", at]);
			const files = listed
				.split("\n")
				.filter((f) => f && posix.dirname(f) === posix.normalize(at))
				.map((f) => posix.basename(f));
			if (!files.length && !listed.trim()) throw new Error(`${name} holds no folder ${dir}`);
			return files;
		},
	};
}

let reading: Source = WORKTREE;

// The source itos's data is read from now.
export const current = (): Source => reading;

// `fn`'s result, with itos's data read from `source` while it runs.
export function readingFrom<T>(source: Source, fn: () => T): T {
	const before = reading;
	reading = source;
	try {
		return fn();
	} finally {
		reading = before;
	}
}
