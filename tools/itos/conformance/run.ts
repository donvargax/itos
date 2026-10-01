// itos's conformance fixtures: the cases that say what the tool does, as
// data, run through its command line, so that the same files prove any
// implementation of it, whatever its language.
//
//   node tools/itos/conformance/run.ts --bin <command> [--only <file>…]
//       run every case of tools/itos/conformance/*.yaml (or of the files
//       given) through <command>; print each failure with its diff; exit 1
//       if any case fails, 2 if a fixture cannot be read
//
// A fixture file is YAML: `files`, `git` and `env` at the top are every case's
// defaults, and `cases` is a list. A case is:
//
//   name     what it shows (unique in its file)
//   argv     the arguments after `itos`
//   git      a scratch repository, built first in the case's folder (`git
//            init -b main`, fixed author and dates, so every SHA is fixed):
//            steps of `commit: <message>` (with `files:` to write first, and
//            `stage` those only), `stage: {files}`, `write: {files}`,
//            `branch: <new>`, `checkout: <ref>`, `run: <sh command>`; any
//            step may say `label: <name>`, the commit HEAD is then
//   files    written after the repository: path → text, or null to delete
//            it, or { text, executable: true }; a path ending in / is a folder.
//            A case's files are laid over the file's own.
//   stdin, env (laid over the file's), cwd (a folder of the case's)
//   hide     commands the PATH must not reach (a folder holding one is
//            replaced by a copy of links without it)
//   exit     the expected exit code (required)
//   stdout, stderr         the exact text; stdout_has, stderr_has: text each
//                          must contain (one, or a list); json: stdout's JSON
//   files_after            files as they must be after the run (null: absent)
//
// In every string, `{{dir}}` is the case's folder, `{{PATH}}` the runner's
// PATH, `{{sha.<label>}}` and `{{short.<label>}}` a labelled commit (40 and 7
// characters). The command runs in a clean environment: no GIT_*, ITOS_*,
// GITHUB_* or CI variable of the caller's, HOME an empty folder, no global or
// system git config. Nothing is shared between cases, so they run in parallel.
import { spawn, spawnSync } from "node:child_process";
import {
	chmodSync,
	existsSync,
	mkdirSync,
	mkdtempSync,
	readdirSync,
	readFileSync,
	rmSync,
	statSync,
	symlinkSync,
	writeFileSync,
} from "node:fs";
import { availableParallelism, tmpdir } from "node:os";
import { basename, delimiter, dirname, join, relative, resolve } from "node:path";
import { parse } from "yaml";

type FileValue = string | null | { text: string; executable?: boolean };
type Files = Record<string, FileValue>;
interface GitStep {
	commit?: string;
	stage?: Files;
	write?: Files;
	files?: Files;
	branch?: string;
	checkout?: string;
	run?: string;
	label?: string;
}
export interface Case {
	name: string;
	argv: string[];
	git?: GitStep[];
	files?: Files;
	stdin?: string;
	env?: Record<string, string>;
	cwd?: string;
	hide?: string[];
	exit: number;
	stdout?: string;
	stderr?: string;
	stdout_has?: string | string[];
	stderr_has?: string | string[];
	json?: unknown;
	files_after?: Files;
}
interface Fixture {
	file: string;
	cases: Case[];
}

const CASE_KEYS = new Set([
	"name",
	"argv",
	"git",
	"files",
	"stdin",
	"env",
	"cwd",
	"hide",
	"exit",
	"stdout",
	"stderr",
	"stdout_has",
	"stderr_has",
	"json",
	"files_after",
]);
const STEP_KEYS = new Set(["commit", "stage", "write", "files", "branch", "checkout", "run", "label"]);
const TOP_KEYS = new Set(["files", "git", "env", "cases"]);

class FixtureError extends Error {}

const unknownKey = (value: object, known: Set<string>) =>
	Object.keys(value).find((key) => !known.has(key));
const stepsProblem = (steps: GitStep[] = []) => {
	const key = steps.map((step) => unknownKey(step, STEP_KEYS)).find(Boolean);
	return key && `a git step has an unknown key ${key}`;
};

// What is wrong with one case, if anything.
function caseProblem(c: Case, names: Set<string>): string | undefined {
	if (!c || typeof c !== "object") return "is not a mapping";
	const key = unknownKey(c, CASE_KEYS);
	if (key) return `unknown key ${key}`;
	if (typeof c.name !== "string") return "has no name";
	if (names.has(c.name)) return "uses a name twice";
	if (!Array.isArray(c.argv) || !c.argv.every((a) => typeof a === "string"))
		return "has an argv that is not a list of strings";
	if (typeof c.exit !== "number") return "has no exit";
	return stepsProblem(c.git);
}

// A fixture file, its defaults laid under each case, held to the format.
export function readFixture(file: string): Fixture {
	const doc = parse(readFileSync(file, "utf8")) as Record<string, unknown> | null;
	const fail = (why: string): never => {
		throw new FixtureError(`${file}: ${why}`);
	};
	if (!doc || typeof doc !== "object" || !Array.isArray(doc.cases)) return fail("has no list of cases");
	const top = doc as { files?: Files; git?: GitStep[]; env?: Record<string, string>; cases: Case[] };
	const problem = (unknownKey(top, TOP_KEYS) && `unknown key ${unknownKey(top, TOP_KEYS)}`) || stepsProblem(top.git);
	if (problem) fail(problem);
	const names = new Set<string>();
	const cases = top.cases.map((c, i) => {
		const why = caseProblem(c, names);
		if (why) fail(`case ${i + 1}${c?.name ? ` (${c.name})` : ""} ${why}`);
		names.add(c.name);
		return { ...c, git: c.git ?? top.git, files: { ...top.files, ...c.files }, env: { ...top.env, ...c.env } };
	});
	return { file, cases };
}

// The caller's environment without what would reach into the caller's repository or
// change what the tool does, and a git that reads no config but the scratch
// repository's.
function cleanEnv(home: string): NodeJS.ProcessEnv {
	const env: NodeJS.ProcessEnv = {};
	for (const [key, value] of Object.entries(process.env))
		if (!/^(GIT_|ITOS_|GITHUB_|RUNNER_|PRE_COMMIT_|GH_)|^(CI|NO_COLOR|FORCE_COLOR)$/.test(key))
			env[key] = value;
	return {
		...env,
		HOME: home,
		GIT_CONFIG_NOSYSTEM: "1",
		GIT_CONFIG_GLOBAL: join(home, ".gitconfig"),
		GIT_AUTHOR_NAME: "itos conformance",
		GIT_AUTHOR_EMAIL: "conformance@localhost",
		GIT_COMMITTER_NAME: "itos conformance",
		GIT_COMMITTER_EMAIL: "conformance@localhost",
		// A fixed date, so every scratch commit's SHA is the same on every run.
		GIT_AUTHOR_DATE: "2000-01-01T00:00:00Z",
		GIT_COMMITTER_DATE: "2000-01-01T00:00:00Z",
	};
}

type Subst = (s: string) => string;

function substituter(dir: string, labels: Map<string, string>): Subst {
	return (s) =>
		s.replace(/\{\{(dir|PATH|sha\.[\w-]+|short\.[\w-]+)\}\}/g, (whole, name: string) => {
			if (name === "dir") return dir;
			if (name === "PATH") return process.env.PATH ?? "";
			const [form, label] = name.split(".") as [string, string];
			const sha = labels.get(label);
			if (sha === undefined) throw new FixtureError(`no commit is labelled ${label} (${whole})`);
			return form === "sha" ? sha : sha.slice(0, 7);
		});
}

const deep = (value: unknown, subst: Subst): unknown =>
	typeof value === "string"
		? subst(value)
		: Array.isArray(value)
			? value.map((v) => deep(v, subst))
			: value && typeof value === "object"
				? Object.fromEntries(Object.entries(value).map(([k, v]) => [k, deep(v, subst)]))
				: value;

function writeFiles(dir: string, files: Files, subst: Subst): string[] {
	const written: string[] = [];
	for (const [path, value] of Object.entries(files)) {
		const full = join(dir, path);
		written.push(path.replace(/\/$/, ""));
		if (value === null) rmSync(full, { recursive: true, force: true });
		else if (path.endsWith("/")) mkdirSync(full, { recursive: true });
		else {
			mkdirSync(dirname(full), { recursive: true });
			const text = typeof value === "string" ? value : value.text;
			writeFileSync(full, subst(text));
			if (typeof value === "object" && value.executable) chmodSync(full, 0o755);
		}
	}
	return written;
}

function git(dir: string, env: NodeJS.ProcessEnv, args: string[], input?: string): string {
	const run = spawnSync("git", args, { cwd: dir, env, input, encoding: "utf8" });
	if (run.status !== 0)
		throw new FixtureError(`git ${args.join(" ")} failed:\n${run.stderr}${run.stdout}`.trimEnd());
	return run.stdout.trim();
}

function sh(dir: string, env: NodeJS.ProcessEnv, command: string) {
	const run = spawnSync("sh", ["-c", command], { cwd: dir, env, encoding: "utf8" });
	if (run.status !== 0)
		throw new FixtureError(`\`${command}\` failed:\n${run.stderr}${run.stdout}`.trimEnd());
}

// One step of a scratch repository, in the order its keys are read.
function applyStep(dir: string, step: GitStep, env: NodeJS.ProcessEnv, subst: Subst) {
	if (step.write) writeFiles(dir, step.write, subst);
	if (step.stage) git(dir, env, ["add", "-A", "--", ...writeFiles(dir, step.stage, subst)]);
	if (step.commit !== undefined) {
		const paths = writeFiles(dir, step.files ?? {}, subst);
		if (paths.length) git(dir, env, ["add", "-A", "--", ...paths]);
		git(dir, env, ["commit", "-q", "--allow-empty", "-F", "-"], subst(step.commit));
	}
	if (step.branch) git(dir, env, ["checkout", "-q", "-b", step.branch]);
	if (step.checkout) git(dir, env, ["checkout", "-q", subst(step.checkout)]);
	if (step.run) sh(dir, env, subst(step.run));
}

// The scratch repository a case's `git` steps describe.
function buildRepo(dir: string, steps: GitStep[], env: NodeJS.ProcessEnv, labels: Map<string, string>) {
	const subst = substituter(dir, labels);
	git(dir, env, ["init", "-q", "-b", "main"]);
	git(dir, env, ["config", "commit.gpgsign", "false"]);
	for (const step of steps) {
		applyStep(dir, step, env, subst);
		if (step.label) labels.set(step.label, git(dir, env, ["rev-parse", "HEAD"]));
	}
}

// A PATH on which none of `hide` is found: each folder that holds one is
// replaced by a folder of links to everything else in it.
function hiddenPath(path: string, hide: string[], scratch: string): string {
	return path
		.split(delimiter)
		.map((folder, i) => {
			if (!hide.some((name) => existsSync(join(folder, name)))) return folder;
			const copy = join(scratch, `path-${i}`);
			mkdirSync(copy, { recursive: true });
			for (const name of readdirSync(folder))
				if (!hide.includes(name)) symlinkSync(join(folder, name), join(copy, name));
			return copy;
		})
		.join(delimiter);
}

export interface Outcome {
	exit: number | null;
	stdout: string;
	stderr: string;
	files: Record<string, { text: string; executable: boolean } | null>;
}

function execute(bin: string, argv: string[], options: { cwd: string; env: NodeJS.ProcessEnv; stdin?: string }) {
	return new Promise<{ exit: number | null; stdout: string; stderr: string }>((done) => {
		const child = spawn(bin, argv, { cwd: options.cwd, env: options.env });
		let stdout = "";
		let stderr = "";
		child.stdout.setEncoding("utf8").on("data", (d: string) => (stdout += d));
		child.stderr.setEncoding("utf8").on("data", (d: string) => (stderr += d));
		const timer = setTimeout(() => child.kill("SIGKILL"), 60_000);
		child.on("error", (error) => {
			clearTimeout(timer);
			done({ exit: null, stdout, stderr: `${stderr}${error.message}\n` });
		});
		child.on("close", (code) => {
			clearTimeout(timer);
			done({ exit: code, stdout, stderr });
		});
		child.stdin.on("error", () => {});
		child.stdin.end(options.stdin ?? "");
	});
}

// One case, run in a folder of its own; its outcome and the substitution its
// expectations are read with.
export async function runCase(
	bin: string,
	c: Case,
): Promise<{ outcome: Outcome; subst: Subst; labels: Map<string, string> }> {
	const scratch = mkdtempSync(join(tmpdir(), "itos-conformance-"));
	const dir = join(scratch, "case");
	const home = join(scratch, "home");
	mkdirSync(dir);
	mkdirSync(home);
	try {
		const env = cleanEnv(home);
		const labels = new Map<string, string>();
		if (c.git) buildRepo(dir, c.git, env, labels);
		const subst = substituter(dir, labels);
		writeFiles(dir, c.files ?? {}, subst);
		for (const [key, value] of Object.entries(c.env ?? {})) env[key] = subst(value);
		if (c.hide?.length) env.PATH = hiddenPath(env.PATH ?? "", c.hide, scratch);
		const run = await execute(bin, c.argv.map(subst), {
			cwd: c.cwd ? join(dir, c.cwd) : dir,
			env,
			stdin: c.stdin === undefined ? undefined : subst(c.stdin),
		});
		const files: Outcome["files"] = {};
		for (const path of Object.keys(c.files_after ?? {})) {
			const full = join(dir, path);
			files[path] = existsSync(full)
				? { text: readFileSync(full, "utf8"), executable: (statSync(full).mode & 0o111) !== 0 }
				: null;
		}
		return { outcome: { ...run, files }, subst, labels };
	} finally {
		rmSync(scratch, { recursive: true, force: true });
	}
}

// Two spaces, keys sorted at every level: two values compare line by line.
const byKey = (_: string, value: unknown) =>
	value && typeof value === "object" && !Array.isArray(value)
		? Object.fromEntries(Object.entries(value).sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0)))
		: value;
const pretty = (value: unknown) => `${JSON.stringify(value, byKey, 2)}\n`;

// The longest common subsequence's lengths of every pair of suffixes.
function suffixes(a: string[], b: string[]): number[][] {
	const lcs = Array.from({ length: a.length + 1 }, () => new Array<number>(b.length + 1).fill(0));
	for (let i = a.length - 1; i >= 0; i--)
		for (let j = b.length - 1; j >= 0; j--)
			lcs[i]![j] = a[i] === b[j] ? lcs[i + 1]![j + 1]! + 1 : Math.max(lcs[i + 1]![j]!, lcs[i]![j + 1]!);
	return lcs;
}

// A line diff of expected and actual: `-` expected only, `+` actual only.
export function diff(expected: string, actual: string): string[] {
	const a = expected.split("\n");
	const b = actual.split("\n");
	const lcs = suffixes(a, b);
	const lines: string[] = [];
	let [i, j] = [0, 0];
	while (i < a.length || j < b.length) {
		const drop = i < a.length && (j >= b.length || lcs[i + 1]![j]! >= lcs[i]![j + 1]!);
		if (i < a.length && j < b.length && a[i] === b[j]) {
			lines.push(`  ${a[i]}`);
			i++;
			j++;
		} else if (drop) lines.push(`- ${a[i++]}`);
		else lines.push(`+ ${b[j++]}`);
	}
	// Both ending in a newline end in the same empty line: not worth a line.
	return lines.at(-1) === "  " ? lines.slice(0, -1) : lines;
}

const list = (v: string | string[] | undefined) => (v === undefined ? [] : Array.isArray(v) ? v : [v]);

const lines = (why: string, text: string) => [why, ...text.split("\n").map((l) => `    | ${l}`)];

function textProblems(what: string, expected: string, actual: string): string[] {
	if (expected === actual) return [];
	return [`${what} (- expected, + got):`, ...diff(expected, actual).map((l) => `    ${l}`)];
}

// The parts a stream must contain.
function hasProblems(stream: "stdout" | "stderr", wanted: string[], outcome: Outcome): string[] {
	const missing = wanted.find((part) => !outcome[stream].includes(part));
	return missing === undefined ? [] : lines(`${stream} lacks ${JSON.stringify(missing)}:`, outcome[stream]);
}

function jsonProblems(expected: unknown, stdout: string): string[] {
	try {
		return textProblems("json", pretty(expected), pretty(JSON.parse(stdout)));
	} catch {
		return lines("stdout is not JSON:", stdout);
	}
}

function fileProblems(path: string, value: FileValue, actual: Outcome["files"][string], subst: Subst): string[] {
	if (value === null) return actual === null ? [] : [`${path}: expected no file, found one`];
	if (!actual) return [`${path}: expected a file, found none`];
	const want = typeof value === "string" ? { text: value } : value;
	const mode =
		want.executable === undefined || want.executable === actual.executable
			? []
			: [`${path}: expected ${want.executable ? "" : "not "}executable`];
	return [...textProblems(path, subst(want.text), actual.text), ...mode];
}

// What differs between a case's expectations and its outcome, as lines to print.
export function compare(c: Case, { outcome, subst }: { outcome: Outcome; subst: Subst }): string[] {
	const text = (what: "stdout" | "stderr") =>
		c[what] === undefined ? [] : textProblems(what, subst(c[what]!), outcome[what]);
	return [
		...(outcome.exit === c.exit ? [] : [`exit: expected ${c.exit}, got ${outcome.exit}`]),
		...text("stdout"),
		...text("stderr"),
		...hasProblems("stdout", list(c.stdout_has).map(subst), outcome),
		...hasProblems("stderr", list(c.stderr_has).map(subst), outcome),
		...(c.json === undefined ? [] : jsonProblems(deep(c.json, subst), outcome.stdout)),
		...Object.entries(c.files_after ?? {}).flatMap(([path, value]) =>
			fileProblems(path, value, outcome.files[path], subst),
		),
	];
}

// Run `jobs` at a time, keeping the results in order.
async function pool<T, R>(items: T[], jobs: number, work: (item: T) => Promise<R>): Promise<R[]> {
	const results: R[] = new Array(items.length);
	let next = 0;
	const worker = async () => {
		while (next < items.length) {
			const i = next++;
			results[i] = await work(items[i]!);
		}
	};
	await Promise.all(Array.from({ length: Math.min(jobs, items.length) }, worker));
	return results;
}

export const HERE = dirname(new URL(import.meta.url).pathname);

// Every fixture file of the folder, sorted.
export const fixtureFiles = () =>
	readdirSync(HERE)
		.filter((f) => f.endsWith(".yaml"))
		.sort()
		.map((f) => join(HERE, f));

// The command to run: a path if it has a slash, else a name on the PATH.
const binary = (bin: string) => (bin.includes("/") ? resolve(bin) : bin);

async function main(args: string[]): Promise<number> {
	let bin: string | undefined;
	const only: string[] = [];
	let listing = false;
	for (let i = 0; i < args.length; i++) {
		if (args[i] === "--bin") bin = args[++i];
		else if (args[i] === "--only") listing = true;
		else if (listing && !args[i]!.startsWith("--")) only.push(args[i]!);
		else listing = false;
	}
	if (!bin) {
		console.error("Usage: node tools/itos/conformance/run.ts --bin <command> [--only <file>…]");
		return 2;
	}
	const started = performance.now();
	let fixtures: Fixture[];
	try {
		fixtures = (only.length ? only : fixtureFiles()).map(readFixture);
	} catch (error) {
		console.error(`FAIL ${(error as Error).message}`);
		return 2;
	}
	const cases = fixtures.flatMap((f) => f.cases.map((c) => ({ file: f.file, c })));
	const jobs = Math.max(1, Math.min(4, availableParallelism() - 1));
	const results = await pool(cases, jobs, async ({ file, c }) => {
		try {
			return { file, c, problems: compare(c, await runCase(binary(bin), c)) };
		} catch (error) {
			return { file, c, problems: [`could not run: ${(error as Error).message}`] };
		}
	});
	const failed = results.filter((r) => r.problems.length);
	for (const { file, c, problems } of failed) {
		console.error(`\nFAIL ${relative(process.cwd(), file) || basename(file)}: ${c.name}`);
		console.error(`  $ itos ${c.argv.join(" ")}`);
		for (const line of problems) console.error(`  ${line}`);
	}
	const seconds = ((performance.now() - started) / 1000).toFixed(1);
	const counts = fixtures.map((f) => `${basename(f.file, ".yaml")} ${f.cases.length}`).join(", ");
	console.log(`\n${cases.length - failed.length}/${cases.length} conformance cases pass (${counts}) in ${seconds} s`);
	return failed.length ? 1 : 0;
}

if (import.meta.main) process.exitCode = await main(process.argv.slice(2));
