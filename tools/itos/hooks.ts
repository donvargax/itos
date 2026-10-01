// itos's hook entry points and their shims.
//
//   itos hook commit-msg <file>      the path rules and the kinds' staged range
//                                    checks (commit-scope.ts), then the header
//                                    lint with the footer rules (commit.ts)
//   itos hook pre-push <remote> <url> the unit tests the pushed commits reach
//                                    (pre-push.ts, itos.yaml's hooks.pre_push)
//   itos hooks install [--manager <m>] [--print] [--force]
//                                    the one-line shims calling the two, for
//                                    the hook manager in use
//
// A hook manager is found by its markers, in this order: Vite+ (a
// `.vite-hooks/` folder, or core.hooksPath set to `.vite-hooks/_` by `vp
// config`), husky (`.husky/`), lefthook (`lefthook.yml`), pre-commit or prek
// (`.pre-commit-config.yaml`), else plain git. Vite+, husky and git keep hooks as files, so their shims are
// written; lefthook and pre-commit keep them in their config, so their snippet
// is printed to add there. A hook file that is not a shim is never replaced
// without --force: the pre-commit hook, say, is the project's own.
import { execFileSync } from "node:child_process";
import { existsSync, mkdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { lintMessageFile } from "./commit.ts";
import { hook as stagedRule } from "./commit-scope.ts";
import { config } from "./config.ts";
import { emit, type Output, TEXT } from "./problem.ts";
import { prePush } from "./pre-push.ts";

// `hook commit-msg <file>`: the staged files' rules, then the header lint,
// in the order and wording the two-line hook had; the first to fail decides.
export function hookCommitMsg(file: string): number {
	return stagedRule(file) || lintMessageFile(file);
}

// `hook pre-push <remote> <url>`: git's ref lines on stdin. Under pre-commit
// or prek, which keep stdin, the refs come from their environment instead.
export function hookPrePush(env: NodeJS.ProcessEnv = process.env): number {
	const to = env.PRE_COMMIT_TO_REF;
	const input =
		to === undefined
			? readFileSync(0, "utf8")
			: `${env.PRE_COMMIT_LOCAL_BRANCH ?? "HEAD"} ${to} ${env.PRE_COMMIT_REMOTE_BRANCH ?? "-"} ${env.PRE_COMMIT_FROM_REF || "0".repeat(40)}\n`;
	return prePush(input);
}

export const MANAGERS = ["vp", "git", "husky", "lefthook", "pre-commit", "prek"] as const;
export type Manager = (typeof MANAGERS)[number];
export const isManager = (m: string): m is Manager => (MANAGERS as readonly string[]).includes(m);

const NAMES: Record<Manager, string> = {
	vp: "Vite+",
	git: "plain git",
	husky: "husky",
	lefthook: "lefthook",
	"pre-commit": "pre-commit",
	prek: "prek",
};

// The repository's own core.hooksPath, if it has one.
function hooksPath(root: string): string | undefined {
	try {
		const value = execFileSync("git", ["-C", root, "config", "--local", "core.hooksPath"], {
			encoding: "utf8",
			stdio: ["ignore", "pipe", "ignore"],
		}).trim();
		return value || undefined;
	} catch {
		return undefined;
	}
}

const isDir = (path: string) => existsSync(path) && statSync(path).isDirectory();
const pointsAt = (path: string | undefined, root: string, dir: string) =>
	path !== undefined &&
	(resolve(root, path) === resolve(root, dir) || path.replace(/\/+$/, "").endsWith(dir));

const LEFTHOOK = ["lefthook.yml", "lefthook.yaml", ".lefthook.yml", ".lefthook.yaml"] as const;

// The hook manager in use under `root`, and the marker that says so.
export function detectManager(root = "."): { manager: Manager; marker: string } {
	const path = hooksPath(root);
	if (isDir(join(root, ".vite-hooks"))) return { manager: "vp", marker: ".vite-hooks/" };
	if (pointsAt(path, root, ".vite-hooks/_"))
		return { manager: "vp", marker: `core.hooksPath ${path}` };
	if (isDir(join(root, ".husky"))) return { manager: "husky", marker: ".husky/" };
	if (pointsAt(path, root, ".husky/_"))
		return { manager: "husky", marker: `core.hooksPath ${path}` };
	for (const file of LEFTHOOK)
		if (existsSync(join(root, file))) return { manager: "lefthook", marker: file };
	if (existsSync(join(root, ".pre-commit-config.yaml")))
		return { manager: "pre-commit", marker: ".pre-commit-config.yaml" };
	return { manager: "git", marker: "no hook manager's marker" };
}

// The two hooks' one-line bodies.
export function shimLines(bin = config().hooks?.bin ?? "tools/bin/itos") {
	return {
		"commit-msg": `exec ${bin} hook commit-msg "$1"`,
		"pre-push": `exec ${bin} hook pre-push "$@"`,
	};
}

// A hook file that only calls itos: one line, besides comments and a shebang.
export function isShim(text: string): boolean {
	const lines = text.split("\n").filter((l) => l.trim() && !l.trim().startsWith("#"));
	return lines.length === 1 && /\bitos hook (commit-msg|pre-push)\b/.test(lines[0]!);
}

// Where a file manager keeps its hooks, relative to `root`.
function hookDir(manager: "vp" | "husky" | "git", root: string): string {
	if (manager === "vp") return ".vite-hooks";
	if (manager === "husky") return ".husky";
	return execFileSync("git", ["-C", root, "rev-parse", "--git-path", "hooks"], {
		encoding: "utf8",
	}).trim();
}

// The snippet a config-file manager takes.
function snippet(manager: "lefthook" | "pre-commit" | "prek", bin: string): string {
	if (manager === "lefthook")
		return `commit-msg:
  commands:
    itos:
      run: ${bin} hook commit-msg {1}
pre-push:
  commands:
    itos:
      run: ${bin} hook pre-push {1} {2}
      use_stdin: true`;
	return `default_install_hook_types: [pre-commit, commit-msg, pre-push]
repos:
  - repo: local
    hooks:
      - id: itos-commit-msg
        name: itos hook commit-msg
        entry: ${bin} hook commit-msg
        language: system
        stages: [commit-msg]
      - id: itos-pre-push
        name: itos hook pre-push
        entry: ${bin} hook pre-push
        language: system
        stages: [pre-push]
        pass_filenames: false
        always_run: true`;
}

export interface InstallOptions {
	manager?: Manager;
	print?: boolean;
	force?: boolean;
	root?: string;
	bin?: string;
}
type Action = "wrote" | "unchanged" | "replaced" | "refused" | "printed";
type Found = { manager: Manager; marker: string };
type Say = (line: string) => void;
const readIf = (path: string) => (existsSync(path) ? readFileSync(path, "utf8") : undefined);

// lefthook, pre-commit and prek: the snippet to add to their config.
function printSnippet(found: Found, root: string, bin: string, out: Output, say: Say): number {
	const manager = found.manager as "lefthook" | "pre-commit" | "prek";
	const file =
		manager === "lefthook"
			? (LEFTHOOK.find((f) => existsSync(join(root, f))) ?? LEFTHOOK[0])
			: ".pre-commit-config.yaml";
	const text = snippet(manager, bin);
	const installed = readIf(join(root, file))?.includes(`${bin} hook commit-msg`) ?? false;
	if (out.json) emit({ ...found, file, snippet: text, installed });
	else {
		say(installed ? `${file} already calls itos; its snippet:` : `Add this to ${file}:`);
		console.log(text);
	}
	return 0;
}

// What becomes of one hook file: printed, left as it is, refused, or written.
function place(full: string, content: string, options: InstallOptions, git: boolean): Action {
	const current = readIf(full);
	if (options.print) return "printed";
	if (current === content) return "unchanged";
	const foreign = current !== undefined && !isShim(current);
	if (foreign && !options.force) return "refused";
	mkdirSync(dirname(full), { recursive: true });
	writeFileSync(full, content, git ? { mode: 0o755 } : {});
	return foreign ? "replaced" : "wrote";
}

// One line per file, as its action says.
function report(files: { path: string; content: string; action: Action }[], out: Output, say: Say) {
	for (const f of files) {
		if (f.action === "refused")
			console.error(`${f.path} is not an itos shim; pass --force to replace it`);
		else if (f.action !== "printed") say(`${f.action} ${f.path}`);
		else if (!out.json) console.log(`==> ${f.path}\n${f.content.trimEnd()}`);
	}
}

// `hooks install`: 0 when every shim is in place (or printed), 1 when a hook
// that is not a shim stood in the way and --force was not given.
export function hooksInstall(options: InstallOptions = {}, out: Output = TEXT): number {
	const root = options.root ?? ".";
	const bin = options.bin ?? config().hooks?.bin ?? "tools/bin/itos";
	const found: Found = options.manager
		? { manager: options.manager, marker: `--manager ${options.manager}` }
		: detectManager(root);
	const { manager } = found;
	const say: Say = (line) => (out.json ? console.error(line) : console.log(line));
	say(`${options.manager ? "Using" : "Found"} ${NAMES[manager]} (${found.marker})`);
	if (manager !== "vp" && manager !== "husky" && manager !== "git")
		return printSnippet(found, root, bin, out, say);
	const dir = hookDir(manager, root);
	const git = manager === "git";
	const files = Object.entries(shimLines(bin)).map(([name, line]) => {
		const path = join(dir, name);
		const content = git ? `#!/bin/sh\n${line}\n` : `${line}\n`;
		return { path, content, action: place(join(root, path), content, options, git) };
	});
	if (out.json) emit({ ...found, files });
	report(files, out, say);
	return files.some((f) => f.action === "refused") ? 1 : 0;
}
