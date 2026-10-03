import type { On } from "claude-code";
import { expect, test, type Engine } from "claude-code/testing";

// The project the session runs in, and what its itos and its registry answer.
const ROOT = "/work/project";

const WORK_LIST = {
	schema: 1,
	file: "plans/registry.yaml",
	items: [
		{ id: "T-066", title: "The itos plugin for Claude Code", status: "doing" },
		{ id: "slice-43", title: "itos work list", status: "done" },
	],
};
const TASK_LIST = {
	schema: 1,
	tasks: [
		{ id: "T-066", title: "The ledger's own wording" },
		{ id: "T-007", title: "The commit rules", status: "no item" },
	],
};
// What itos work --json prints, and so what an itos older than v2.3.0 prints
// for work list: the proposal, with no items.
const PROPOSAL = { schema: 1, person: "someone", start: [], waiting: [] };
const REGISTRY = `items:
  - id: T-066
    title: "From the registry file"
  - id: slice-43
    title: itos work list
`;

type World = {
	root?: string; // the session's root; ROOT when absent
	top?: string; // what git rev-parse --show-toplevel prints; absent: not a repository
	pathItos?: "answers" | "too old"; // the itos on the PATH: v2.4.0 or later, or older; absent: none
	hooksBin?: string; // what itos config get hooks.bin prints
	executables?: string[]; // the paths test -x passes, the only ones that start
	onPath?: string[]; // commands on the PATH besides itos
	itos?: Record<string, unknown>; // stdout JSON by "work list" or "task list", whichever itos runs
	registry?: string;
	runs: string[][]; // every argv run, in order
	drawn: string[]; // every AssistantMessage text the engine was handed to draw
};

const ok = (stdout: string, exitCode = 0) => ({
	value: { exitCode, stdout, stderr: "", isStdoutTruncated: false, isStderrTruncated: false },
});
type Answer = ReturnType<typeof ok>;

// Whether a program starts: the itos on the PATH, an executable path, another command.
const found = (w: World, program: string) =>
	program === "itos"
		? !!w.pathItos
		: [...(w.executables ?? []), ...(w.onPath ?? [])].includes(program);

// itos config get hooks.bin: the value, or an older itos's usage error.
const configGet = (w: World) => (w.pathItos === "answers" ? ok(`${w.hooksBin}\n`) : ok("", 2));

// itos work list or task list, by the two words before --json.
function listed(w: World, argv: readonly string[]): Answer {
	const answer = w.itos?.[argv.slice(-3, -1).join(" ")];
	return ok(JSON.stringify(answer ?? {}), answer === undefined ? 1 : 0);
}

function itosRun(w: World, argv: readonly string[]): Answer {
	if (!found(w, argv[0]!)) throw new Error(`${argv[0]}: not found`);
	return argv[1] === "config" ? configGet(w) : listed(w, argv);
}

// The commands other than itos, by program.
const COMMANDS: Record<string, (w: World, argv: readonly string[]) => Answer> = {
	git: (w) => (w.top ? ok(`${w.top}\n`) : ok("", 128)),
	test: (w, argv) => ok("", w.executables?.includes(argv[2]!) ? 0 : 1),
};

// The engine beneath the plugin: the session's root, its processes, its files,
// and the bottoms of the events the plugin passes on.
function world(on: On, w: World): void {
	const root = w.root ?? ROOT;
	on("session.root", async () => ({ value: root }));
	on("process.run", async (_$, e) => {
		w.runs.push([...e.argv]);
		expect(e.init?.cwd).toBe(root);
		return (COMMANDS[e.argv[0]!] ?? itosRun)(w, e.argv);
	});
	on("fs.read", async (_$, e) => {
		expect(e.path).toBe(`${root}/tasks/work-items.yaml`);
		if (w.registry === undefined) throw new Error("ENOENT");
		return { value: w.registry };
	});
	on("session.start", async (_$, e) => ({ cwd: e.cwd }));
	on("turn.start", async (_$, e) => ({ turnId: e.turnId }));
	on("ui.render", async (_$, e) => {
		if (e.component === "AssistantMessage") w.drawn.push(e.props.text);
		return { type: "engine", ref: 0 };
	});
}

// The itos commands asked for JSON (work list, task list), in order.
const asked = (w: World) => w.runs.filter((argv) => argv.at(-1) === "--json");

// The words that started them: the itos the resolution chose.
const programs = (w: World) => asked(w).map((argv) => argv.slice(0, -3).join(" "));

// An itos on the PATH that answers config get, its hooks.bin naming itself.
const GLOBAL = { pathItos: "answers", hooksBin: "itos", top: ROOT } as const;

const start = ($: Engine) =>
	$.session.start({ cwd: ROOT, surface: "terminal", isInteractive: true });

const draw = ($: Engine, text: string) =>
	$.ui.render({
		surface: "terminal",
		component: "AssistantMessage",
		requestId: "m1",
		props: { text, isFirstOfReply: true },
	});

test("the titles come from the project's itos: every registry item, then the ledger's tasks", async ($, on) => {
	const w: World = {
		...GLOBAL,
		itos: { "work list": WORK_LIST, "task list": TASK_LIST },
		runs: [],
		drawn: [],
	};
	world(on, w);
	await start($);
	expect(asked(w)).toEqual([
		["itos", "work", "list", "--json"],
		["itos", "task", "list", "--json"],
	]);
	await draw($, "T-066 waits on slice 43; T-007 stays.");
	expect(w.drawn).toEqual([
		"`T-066: The itos plugin for Claude Code` waits on `slice-43: itos work list`; `T-007: The commit rules` stays.",
	]);
});

test("drawing runs nothing; the titles are asked for again at each turn's start", async ($, on) => {
	const w: World = {
		...GLOBAL,
		itos: { "work list": WORK_LIST, "task list": TASK_LIST },
		runs: [],
		drawn: [],
	};
	world(on, w);
	await start($);
	const ran = w.runs.length;
	await draw($, "T-066");
	await draw($, "T-066 again");
	expect(w.runs).toHaveLength(ran);
	expect(asked(w)).toHaveLength(2);
	w.itos = { "work list": { ...WORK_LIST, items: [{ id: "T-066", title: "Renamed" }] } };
	await $.turn.start({ text: "next", turnId: "t2" });
	expect(asked(w)).toHaveLength(4);
	await draw($, "T-066");
	expect(w.drawn.at(-1)).toBe("`T-066: Renamed`");
});

test("the itos the repository's hooks.bin names runs, a path read from the repository's top", async ($, on) => {
	const w: World = {
		...GLOBAL,
		hooksBin: ".tools/bin/itos",
		executables: [`${ROOT}/.tools/bin/itos`],
		itos: { "work list": WORK_LIST, "task list": TASK_LIST },
		runs: [],
		drawn: [],
	};
	world(on, w);
	await start($);
	expect(w.runs[0]).toEqual(["itos", "config", "get", "hooks.bin"]);
	expect(programs(w)).toEqual([`${ROOT}/.tools/bin/itos`, `${ROOT}/.tools/bin/itos`]);
	await draw($, "T-066");
	expect(w.drawn).toEqual(["`T-066: The itos plugin for Claude Code`"]);
});

test("from a subfolder, a relative hooks.bin is resolved against the repository's top", async ($, on) => {
	const w: World = {
		...GLOBAL,
		root: `${ROOT}/internal/cli`,
		hooksBin: "tools/bin/itos",
		executables: [`${ROOT}/tools/bin/itos`],
		itos: { "work list": WORK_LIST, "task list": TASK_LIST },
		runs: [],
		drawn: [],
	};
	world(on, w);
	await start($);
	expect(programs(w)).toEqual([`${ROOT}/tools/bin/itos`, `${ROOT}/tools/bin/itos`]);
});

test("a hooks.bin of several words starts with its first, the rest passed before the command", async ($, on) => {
	const w: World = {
		...GLOBAL,
		hooksBin: "go run ./cmd/itos",
		onPath: ["go"],
		itos: { "work list": WORK_LIST, "task list": TASK_LIST },
		runs: [],
		drawn: [],
	};
	world(on, w);
	await start($);
	expect(asked(w)[0]).toEqual(["go", "run", "./cmd/itos", "work", "list", "--json"]);
});

test("a hooks.bin path that is not executable is no answer: tools/bin/itos runs", async ($, on) => {
	const w: World = {
		...GLOBAL,
		hooksBin: ".tools/bin/itos",
		executables: [`${ROOT}/tools/bin/itos`],
		itos: { "work list": WORK_LIST, "task list": TASK_LIST },
		runs: [],
		drawn: [],
	};
	world(on, w);
	await start($);
	expect(programs(w)).toEqual([`${ROOT}/tools/bin/itos`, `${ROOT}/tools/bin/itos`]);
});

test("an itos older than v2.4.0, whose config get exits 2, is no answer: tools/bin/itos at the top runs", async ($, on) => {
	const w: World = {
		top: ROOT,
		pathItos: "too old",
		executables: [`${ROOT}/tools/bin/itos`],
		itos: { "work list": WORK_LIST, "task list": TASK_LIST },
		runs: [],
		drawn: [],
	};
	world(on, w);
	await start($);
	expect(programs(w)).toEqual([`${ROOT}/tools/bin/itos`, `${ROOT}/tools/bin/itos`]);
	await draw($, "T-007");
	expect(w.drawn).toEqual(["`T-007: The commit rules`"]);
});

test("with no itos on the PATH, tools/bin/itos at the top runs", async ($, on) => {
	const w: World = {
		top: ROOT,
		executables: [`${ROOT}/tools/bin/itos`],
		itos: { "work list": WORK_LIST, "task list": TASK_LIST },
		runs: [],
		drawn: [],
	};
	world(on, w);
	await start($);
	expect(programs(w)).toEqual([`${ROOT}/tools/bin/itos`, `${ROOT}/tools/bin/itos`]);
	await draw($, "T-066");
	expect(w.drawn).toEqual(["`T-066: The itos plugin for Claude Code`"]);
});

test("with neither itos nor tools/bin/itos, the itos on the PATH is tried and the registry file gives the titles", async ($, on) => {
	const w: World = { top: ROOT, registry: REGISTRY, runs: [], drawn: [] };
	world(on, w);
	await start($);
	expect(programs(w)).toEqual(["itos", "itos"]);
	await draw($, "T-066 and `slice-43`.");
	expect(w.drawn).toEqual(["`T-066: From the registry file` and `slice-43: itos work list`."]);
});

test("an itos older than v2.3.0, whose work list prints the proposal, reads as no itos for the items", async ($, on) => {
	const w: World = {
		top: ROOT,
		pathItos: "too old",
		itos: { "work list": PROPOSAL, "task list": TASK_LIST },
		registry: REGISTRY,
		runs: [],
		drawn: [],
	};
	world(on, w);
	await start($);
	expect(programs(w)).toEqual(["itos", "itos"]);
	await draw($, "T-066, T-007");
	expect(w.drawn).toEqual(["`T-066: From the registry file`, `T-007: The commit rules`"]);
});

test("with neither itos nor a registry, a reply is drawn as written", async ($, on) => {
	const w: World = { runs: [], drawn: [] };
	world(on, w);
	await start($);
	await draw($, "T-066 lands");
	expect(w.drawn).toEqual(["T-066 lands"]);
});
