// itos's one command line: every command, with its exit codes and `--json`
// shapes, dispatched to the module that holds it (cli.ts, work.ts, ci.ts,
// e2e-scope.ts, verify-commits.ts, commit-scope.ts, commit.ts, hooks.ts,
// config-check.ts). `tools/bin/itos` wraps it, so the config and the ledger
// say `tools/bin/itos …`.
//
// The global flags are read first, wherever they stand (up to a `--`), and
// the commands are imported after, so `--config` and `--root` hold for
// everything a command loads.
import { emit } from "./problem.ts";
import { satisfies, VERSION } from "./version.ts";

export interface Globals {
	json: boolean;
	quiet: boolean;
	help: boolean;
	config?: string;
	root?: string;
	rest: string[];
}

const VALUED = new Set(["--config", "--root"]);
const FLAGS: Record<string, "json" | "quiet" | "help" | "color"> = {
	"--json": "json",
	"-q": "quiet",
	"--quiet": "quiet",
	"-h": "help",
	"--help": "help",
	"--no-color": "color",
};

// The global flags out of the arguments, up to a `--`, after which every
// argument is the command's.
export function parseGlobals(args: string[]): Globals {
	const g: Globals = { json: false, quiet: false, help: false, rest: [] };
	const stop = args.indexOf("--");
	const head = stop < 0 ? args : args.slice(0, stop);
	for (let i = 0; i < head.length; i++) {
		const arg = head[i]!;
		if (VALUED.has(arg)) g[arg === "--config" ? "config" : "root"] = head[++i];
		else if (arg in FLAGS) {
			const flag = FLAGS[arg]!;
			if (flag !== "color") g[flag] = true;
		} else g.rest.push(arg);
	}
	if (stop >= 0) g.rest.push("--", ...args.slice(stop + 1));
	return g;
}

class Usage extends Error {}
const usage = (message: string): never => {
	throw new Usage(message);
};

const flagValue = (args: string[], name: string) => {
	const i = args.indexOf(name);
	return i >= 0 ? args[i + 1] : undefined;
};
// The positional arguments: none of the flags, nor the values the named flags take.
const positional = (args: string[], valued: string[] = []) =>
	args.filter((a, i) => !a.startsWith("-") && !valued.includes(args[i - 1] ?? ""));

type Out = { json: boolean; quiet: boolean };
type Command = (args: string[], out: Out) => number | Promise<number>;

async function task(args: string[], out: Out) {
	const { listTasks, runTasks } = await import("./cli.ts");
	if (args[0] === "list") return listTasks(args.slice(1), out);
	return runTasks(args, { out });
}

async function work(args: string[], out: Out) {
	const { work, workCheck } = await import("./work.ts");
	if (args[0] === "check") return workCheck(args[1], out);
	return work(flagValue(args, "--as"), out);
}

async function commit(args: string[], out: Out) {
	const [sub, ...rest] = args;
	if (sub === "check-message") {
		const { checkMessage } = await import("./commit.ts");
		const [source] = positional(rest, ["--at"]);
		if (source === undefined && !rest.includes("-")) usage("commit check-message needs <file|->");
		return checkMessage(source ?? "-", flagValue(rest, "--at"), out);
	}
	if (sub === "check-paths") {
		const { checkPaths } = await import("./commit-scope.ts");
		const type = flagValue(rest, "--type") ?? usage("commit check-paths needs --type <type>");
		return checkPaths(type, positional(rest, ["--type"]), out);
	}
	return usage(`unknown command: commit ${sub ?? ""}`.trim());
}

async function verify(args: string[], out: Out) {
	const { verify } = await import("./verify-commits.ts");
	const [from, to] = args;
	if (to === undefined) usage("verify needs <from> <to>");
	return verify(from, to, out);
}

async function tests(args: string[], out: Out) {
	const [sub, ...rest] = args;
	if (sub === "list") {
		const { testsList } = await import("./tests-command.ts");
		const [name] = positional(rest, ["--at"]);
		return testsList(name ?? usage("tests list needs <kind>"), flagValue(rest, "--at"), out);
	}
	if (sub !== "smoke") return usage(`unknown command: tests ${sub ?? ""}`.trim());
	const scope = await import("./e2e-scope.ts");
	const [action, ...more] = rest;
	const [name] = positional(more, ["--features"]);
	if (!name) usage(`tests smoke ${action ?? "check|ids|run"} needs <kind>`);
	if (action === "check") return scope.smokeCheck(name!, flagValue(more, "--features"), out);
	if (action === "ids") return scope.smokeIdsCommand(name!, out);
	if (action === "run") {
		const dash = more.indexOf("--");
		return scope.smokeRun(name!, dash >= 0 ? more.slice(dash + 1) : []);
	}
	return usage(`unknown command: tests smoke ${action ?? ""}`.trim());
}

async function ci(args: string[], out: Out) {
	const ciCommands = await import("./ci.ts");
	const [sub, ...rest] = args;
	const [from = "", to = ""] = positional(rest, ["--data-at", "--head", "--base"]);
	const nightly = rest.includes("--nightly");
	if (sub === "plan") {
		if (!nightly && !rest.includes("--whole") && !to)
			usage("ci plan needs <from> <to>, --nightly or --whole");
		const dataAt = flagValue(rest, "--data-at");
		return ciCommands.ciPlanCommand({ from, to, nightly, dataAt }, out);
	}
	if (sub === "run") return ciCommands.ciRun(from, to, nightly, out);
	if (sub === "scope") {
		if (!to) usage("ci scope needs <from> <to>");
		return ciCommands.ciScope(from, to, out);
	}
	if (sub === "range") {
		const head = flagValue(rest, "--head") ?? usage("ci range needs --head <sha>");
		return ciCommands.ciRange(head, flagValue(rest, "--base") ?? "", out);
	}
	return usage(`unknown command: ci ${sub ?? ""}`.trim());
}

async function hook(args: string[]) {
	const [sub, ...rest] = args;
	const hooks = await import("./hooks.ts");
	if (sub === "commit-msg")
		return hooks.hookCommitMsg(rest[0] ?? usage("hook commit-msg needs <file>"));
	if (sub === "pre-push") return hooks.hookPrePush();
	return usage(`unknown command: hook ${sub ?? ""}`.trim());
}

async function hooksCommand(args: string[], out: Out) {
	if (args[0] !== "install") return usage(`unknown command: hooks ${args[0] ?? ""}`.trim());
	const { hooksInstall, isManager, MANAGERS } = await import("./hooks.ts");
	const manager = flagValue(args, "--manager");
	const options = { print: args.includes("--print"), force: args.includes("--force") };
	if (manager === undefined) return hooksInstall(options, out);
	if (!isManager(manager)) return usage(`hooks install --manager takes ${MANAGERS.join("|")}`);
	return hooksInstall({ ...options, manager }, out);
}

async function configCommand(args: string[], out: Out) {
	if (args[0] !== "check") return usage(`unknown command: config ${args[0] ?? ""}`.trim());
	const { configCheck, printDefaults } = await import("./config-check.ts");
	if (args.includes("--print-defaults")) return printDefaults(out);
	return configCheck(flagValue(args, "--ledger"), out);
}

async function version(args: string[], out: Out) {
	const { config } = await import("./config.ts");
	const requires = config().requires;
	const ok = requires === undefined || satisfies(VERSION, requires);
	if (out.json)
		emit({ version: VERSION, ...(requires === undefined ? {} : { requires, satisfied: ok }) });
	else console.log(`itos ${VERSION}`);
	if (!args.includes("--check") || ok) return 0;
	console.error(`itos ${VERSION} does not satisfy ${requires} (the config's requires)`);
	return 1;
}

const COMMANDS: Record<string, Command> = {
	task,
	work,
	commit,
	verify,
	tests,
	ci,
	hook,
	hooks: hooksCommand,
	config: configCommand,
	version,
};

// The command path a `--help` asks about: the command and its subcommands.
const helpPath = (rest: string[]) => rest.filter((a) => !a.startsWith("-")).slice(0, 3);

// A config error, as the config check prints it: exit 2.
async function configFailure(error: unknown, out: Out): Promise<number | undefined> {
	const { ConfigError } = await import("./config.ts");
	if (!(error instanceof ConfigError)) return undefined;
	if (out.json)
		emit({
			config: error.file,
			valid: false,
			problems: error.issues.map((p) => ({ ...p, message: `${error.file}: ${p.message}` })),
		});
	else for (const p of error.problems) console.error(`FAIL ${error.file}: ${p}`);
	return 2;
}

// The environment the global flags set, before any command module loads.
function applyGlobals(g: Globals, argv: string[]) {
	if (g.config) process.env.ITOS_CONFIG = g.config;
	if (g.root) process.chdir(g.root);
	if (argv.includes("--no-color")) process.env.NO_COLOR = "1";
}

// `itos`, `itos help <command>` and any `--help`: 2 for a bare `itos`.
async function help(g: Globals): Promise<number> {
	const [name = "", ...args] = g.rest;
	const { helpFor } = await import("./help.ts");
	console.log(helpFor(name === "help" ? args : helpPath(g.rest)));
	return name === "" && !g.help ? 2 : 0;
}

// A usage error (2), a config error (2), or a file or command the config
// names that cannot be read (2: a people file, a smoke set, an adapter's
// output), its message alone. ITOS_DEBUG=1 rethrows it with its stack.
async function failure(error: unknown, out: Out): Promise<number> {
	if (error instanceof Usage) {
		console.error(`itos: ${error.message} (itos --help)`);
		return 2;
	}
	const code = await configFailure(error, out);
	if (code !== undefined) return code;
	if (!(error instanceof Error) || process.env.ITOS_DEBUG) throw error;
	console.error(`itos: ${error.message}`);
	return 2;
}

export async function main(argv: string[]): Promise<number> {
	const g = parseGlobals(argv);
	applyGlobals(g, argv);
	const [name = "", ...args] = g.rest;
	if (g.help || name === "" || name === "help") return help(g);
	const out = { json: g.json, quiet: g.quiet };
	try {
		const command = COMMANDS[name] ?? usage(`unknown command: ${name}`);
		return await command(args, out);
	} catch (error) {
		return failure(error, out);
	}
}

if (import.meta.main) process.exitCode = await main(process.argv.slice(2));
