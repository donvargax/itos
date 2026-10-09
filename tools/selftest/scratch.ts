// What the self-tests that run the real gates share: a scratch worktree of the
// current tree, HEAD plus every tracked edit and the untracked files a gate
// could run, committed as a base, so nothing they do touches the checkout
// they were started from.
import { spawn, spawnSync } from "node:child_process";
import { copyFileSync, mkdirSync, mkdtempSync, readFileSync, rmSync, symlinkSync } from "node:fs";
import { writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { repoProgram } from "../bin/repo-program.ts";

// Hooks export GIT_DIR and friends, and CI sets CI; a scratch repository must
// see neither.
const unhooked = () =>
	Object.fromEntries(
		Object.entries(process.env).filter(([k]) => !k.startsWith("GIT_") && k !== "CI"),
	) as NodeJS.ProcessEnv;

// The real git, as internal/git finds it (git.Bin), asked of
// tools/bin/real-git once rather than found by a second copy of the rule: the
// git ITOS_GIT names, else the first git on the PATH that is not an itos. The
// self-tests run it, never the name git, which may be an itos linked as git
// whose git commit is itos commit under its own policy (T-122).
let found = "";
export function realGit(): string {
	if (found) return found;
	const asked = spawnSync("go", ["run", "./tools/bin/real-git"], {
		cwd: resolve(import.meta.dirname, "../.."),
		env: unhooked(),
		encoding: "utf8",
	});
	if (asked.status !== 0)
		throw new Error(
			`go run ./tools/bin/real-git exited ${asked.status ?? asked.signal}:\n${asked.stdout}${asked.stderr}`,
		);
	found = asked.stdout.trim();
	return found;
}
// realGit, quoted for sh.
export const shGit = () => `'${realGit().replace(/'/g, `'\\''`)}'`;

// The environment the self-tests run everything in: no hook's GIT_ variables
// and no CI, and ITOS_GIT naming the real git, as an itos run hands its git to
// what it starts, so a hook, a gate or a script that runs the name git reaches
// an itos linked as git with ITOS_GIT set and passes straight to that git.
export const outsideEnv = (): NodeJS.ProcessEnv => ({ ...unhooked(), ITOS_GIT: realGit() });

// A command run asynchronously with its stdin given, so many can run at
// once: its exit code (null when a signal ended it) and stdout and stderr
// together.
export function spawnOutput(
	command: string,
	args: string[],
	options: { cwd: string; env: NodeJS.ProcessEnv; input: string },
): Promise<{ code: number | null; output: string }> {
	return new Promise((done, fail) => {
		const child = spawn(command, args, { cwd: options.cwd, env: options.env });
		let output = "";
		child.stdout.on("data", (d: Buffer) => (output += d.toString()));
		child.stderr.on("data", (d: Buffer) => (output += d.toString()));
		child.on("error", fail);
		child.on("close", (code) => done({ code, output }));
		child.stdin.end(options.input);
	});
}

// tools/bin/itos, the Go binary, run in cwd (the checkout by default): its
// stdout, or an error naming the command and what it printed.
export function itos(args: string[], cwd = resolve(".")): string {
	const [program, argv] = repoProgram(join(cwd, "tools/bin/itos"), args);
	const result = spawnSync(program, argv, {
		cwd,
		env: outsideEnv(),
		encoding: "utf8",
		maxBuffer: 64 * 1024 * 1024,
	});
	if (result.status !== 0)
		throw new Error(
			`tools/bin/itos ${args.join(" ")} exited ${result.status ?? result.signal}:\n${result.stdout}${result.stderr}`,
		);
	return result.stdout;
}

// What `itos ci plan --json` says a CI run does (its plan contract), as much
// of it as the self-tests read.
export interface Plan {
	prose: boolean;
	steps: string[];
	tasks: string[];
	order: { step?: string; check?: { task: string; command?: string }; action?: string }[];
}
export const ciPlan = (args: string[], cwd?: string) =>
	JSON.parse(itos(["ci", "plan", ...args, "--json"], cwd)) as Plan;
// The plan's run of the features, the step that starts with the kind's whole run.
export const featuresStep = (plan: Plan, whole = "go test ./features -count=1") =>
	plan.steps.find((step) => step.startsWith(whole));

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
		const run = sh(`${shGit()} ${command}`, undefined, cwd);
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
		const diff = sh(`${shGit()} diff HEAD --binary`, undefined, root).output;
		if (diff.trim()) {
			const applied = sh(`${shGit()} apply --whitespace=nowarn -`, diff);
			if (applied.status !== 0)
				throw new Error(`could not copy the working tree:\n${applied.output}`);
		}
		const untracked = sh(
			`${shGit()} ls-files --others --exclude-standard -- tools .vite-hooks features`,
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
	// itos's hooks, declared for the copy alone as itos hook install declares
	// them in a clone's git config (decision 37): the entries the copy's own
	// itos prints (hook install --print), in a file the environment includes
	// where the git dir is the copy's and nowhere else. The copy is a worktree
	// whose .git/config is the checkout's, which must not change; and git
	// hands its environment to the hooks and all they run, so an include
	// matched to the copy's git dir leaves every other repository, the unit
	// tests' among them, as it was. core.hooksPath, an empty folder there,
	// keeps the checkout's own hook folder (vp's core.hooksPath names it) from
	// running beside them, so a hook run is what the git config declares.
	// Made once, after open.
	let hooks = "";
	const declareHooks = () => {
		if (hooks) return hooks;
		const printed = JSON.parse(itos(["hook", "install", "--print", "--json"], dir)) as {
			hooks: { name: string; event: string; command: string }[];
		};
		if (!printed.hooks.length) throw new Error("itos hook install --print declares no hook");
		hooks = mkdtempSync(join(tmpdir(), `${name}-hooks-`));
		const none = join(hooks, "none");
		mkdirSync(none);
		const value = (v: string) => `"${v.replace(/\\/g, "\\\\").replace(/"/g, '\\"')}"`;
		const config = [`[core]\n\thooksPath = ${value(none)}\n`];
		for (const hook of printed.hooks)
			config.push(
				`[hook ${value(hook.name)}]\n\tevent = ${value(hook.event)}\n\tcommand = ${value(hook.command)}\n`,
			);
		const file = join(hooks, "config");
		writeFileSync(file, config.join(""));
		Object.assign(env, {
			GIT_CONFIG_COUNT: "1",
			GIT_CONFIG_KEY_0: `includeIf.gitdir:${git("rev-parse --absolute-git-dir")}.path`,
			GIT_CONFIG_VALUE_0: file,
		});
		return hooks;
	};
	// The command that runs the hooks the copy's git config declares for an
	// event (git hook run, which fails when none is declared), with its
	// arguments and, given, its stdin.
	const hookRun = (event: string, args: string[], input?: string) => {
		const folder = declareHooks();
		let stdin = "";
		if (input !== undefined) {
			const file = join(folder, `${event}.stdin`);
			writeFileSync(file, input);
			stdin = ` --to-stdin=${file}`;
		}
		return `${shGit()} hook run${stdin} ${event} -- ${args.join(" ")}`;
	};
	const remove = () => {
		sh(`${shGit()} worktree remove --force ${dir}`, undefined, root);
		rmSync(dir, { recursive: true, force: true });
		if (hooks) rmSync(hooks, { recursive: true, force: true });
	};
	return { root, dir, env, sh, git, edit, commit, open, hookRun, remove };
}

// The real hooks, run in a scratch copy and timed for the report, and the
// problems a self-test collects instead of stopping at the first: pre-commit,
// vp's, from its file; commit-msg and pre-push, itos's, as git runs them from
// the copy's git config (hookRun).
export function hookGates({ sh, git, hookRun }: ReturnType<typeof scratchRepo>) {
	const problems: string[] = [];
	const timings: string[] = [];
	const expect = (ok: boolean, problem: string) => {
		if (!ok) problems.push(problem);
	};
	const gate = (label: string, command: string, input?: string): Run => {
		const run = sh(command, input);
		timings.push(
			`${label.padEnd(58)} ${run.status === 0 ? "pass" : "FAIL"}  ${run.seconds.toFixed(1)} s`,
		);
		return run;
	};
	const preCommit = (label: string) => {
		git("add -A");
		return gate(`pre-commit, ${label}`, "sh .vite-hooks/pre-commit");
	};
	const commitMsg = (label: string, file: string) =>
		gate(`commit-msg, ${label}`, hookRun("commit-msg", [file]));
	const prePush = (label: string, base: string, sha: string) =>
		gate(
			`pre-push, ${label}`,
			hookRun(
				"pre-push",
				["upstream", "git@example.invalid:upstream.git"],
				`refs/heads/main ${sha} refs/heads/main ${base}\n`,
			),
		);
	return { problems, timings, expect, gate, preCommit, commitMsg, prePush };
}

// The scratch repositories of a check held to the last release
// (tools/bin/schema-contract, tools/bin/previous-release): git run in a folder
// with a fixed author, in outsideEnv() with extra laid over it; an empty
// commit; and the two repositories every such check must not pass blindly, one
// with no release tag and a shallow clone of repo.
export function releaseRepos(tmp: string, extra: NodeJS.ProcessEnv = {}) {
	const env: NodeJS.ProcessEnv = {
		...outsideEnv(),
		...extra,
		GIT_AUTHOR_NAME: "selftest",
		GIT_AUTHOR_EMAIL: "selftest@localhost",
		GIT_COMMITTER_NAME: "selftest",
		GIT_COMMITTER_EMAIL: "selftest@localhost",
	};
	const git = (cwd: string, ...args: string[]) => {
		const r = spawnSync(realGit(), args, { cwd, env, encoding: "utf8" });
		if (r.status !== 0) throw new Error(`git ${args.join(" ")}:\n${r.stderr}`);
		return r.stdout.trim();
	};
	const commit = (cwd: string, message: string) =>
		git(cwd, "commit", "-q", "--allow-empty", "-m", message);
	const untagged = () => {
		const dir = join(tmp, "untagged");
		git(tmp, "init", "-q", "-b", "main", dir);
		commit(dir, "feat: the first");
		return dir;
	};
	const shallow = (repo: string) => {
		const dir = join(tmp, "shallow");
		git(tmp, "clone", "-q", "--depth", "1", "--no-tags", `file://${repo}`, dir);
		return dir;
	};
	return { env, git, commit, untagged, shallow };
}

// A self-test's end: each problem, then one line saying how it went; exit 1
// on a problem.
export function finish(problems: string[], what: string, passed: string): never {
	for (const problem of problems) console.error(`FAIL ${problem}`);
	console.log(problems.length ? `\n${problems.length} ${what} check(s) failed` : `\n${passed}`);
	process.exit(problems.length ? 1 : 0);
}
