// What the self-tests that run the real gates share: a scratch worktree of the
// current tree, HEAD plus every tracked edit and the untracked files a gate
// could run, committed as a base, so nothing they do touches the checkout
// they were started from.
import { spawnSync } from "node:child_process";
import { copyFileSync, mkdirSync, mkdtempSync, readFileSync, rmSync, symlinkSync } from "node:fs";
import { writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";

// Hooks export GIT_DIR and friends, and CI sets CI; a scratch repository must
// see neither.
export const outsideEnv = () =>
	Object.fromEntries(
		Object.entries(process.env).filter(([k]) => !k.startsWith("GIT_") && k !== "CI"),
	) as NodeJS.ProcessEnv;

export interface Run {
	status: number;
	output: string;
	seconds: number;
}

export function scratchRepo(name: string) {
	const root = resolve(".");
	const dir = mkdtempSync(join(tmpdir(), `${name}-`));
	const env = outsideEnv();
	Object.assign(env, {
		GIT_AUTHOR_NAME: name,
		GIT_AUTHOR_EMAIL: "selftest@localhost",
		GIT_COMMITTER_NAME: name,
		GIT_COMMITTER_EMAIL: "selftest@localhost",
	});
	const sh = (command: string, input?: string, cwd = dir): Run => {
		const started = performance.now();
		const result = spawnSync("sh", ["-c", command], {
			cwd,
			env,
			input,
			encoding: "utf8",
			maxBuffer: 64 * 1024 * 1024,
		});
		return {
			status: result.status ?? 1,
			output: `${result.stdout}${result.stderr}`,
			seconds: (performance.now() - started) / 1000,
		};
	};
	const git = (command: string, cwd = dir) => {
		const run = sh(`git ${command}`, undefined, cwd);
		if (run.status !== 0) throw new Error(`git ${command} failed:\n${run.output}`);
		return run.output.trim();
	};
	const edit = (file: string, from: string, to: string) => {
		const path = join(dir, file);
		const text = readFileSync(path, "utf8");
		if (!text.includes(from)) throw new Error(`${file} no longer contains ${JSON.stringify(from)}`);
		writeFileSync(path, text.replace(from, to));
	};
	// Everything in the tree, committed without the hooks.
	const commit = (message: string) => {
		git("add -A");
		const sha = git(`commit-tree ${git("write-tree")} -p HEAD -m "${message}"`);
		git(`reset -q --hard ${sha}`);
		return sha;
	};
	// Builds the copy and returns its base commit.
	const open = () => {
		git(`worktree add -q --detach ${dir} HEAD`, root);
		const diff = sh("git diff HEAD --binary", undefined, root).output;
		if (diff.trim()) {
			const applied = sh("git apply --whitespace=nowarn -", diff);
			if (applied.status !== 0)
				throw new Error(`could not copy the working tree:\n${applied.output}`);
		}
		const untracked = sh(
			"git ls-files --others --exclude-standard -- tools .vite-hooks features",
			undefined,
			root,
		);
		for (const file of untracked.output.split("\n").filter(Boolean)) {
			mkdirSync(dirname(join(dir, file)), { recursive: true });
			copyFileSync(join(root, file), join(dir, file));
		}
		symlinkSync(join(root, "node_modules"), join(dir, "node_modules"));
		return commit(`${name} base`);
	};
	const remove = () => {
		sh(`git worktree remove --force ${dir}`, undefined, root);
		rmSync(dir, { recursive: true, force: true });
	};
	return { root, dir, env, sh, git, edit, commit, open, remove };
}
