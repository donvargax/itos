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

// What a repository ships to answer in itos's place: an executable
// tools/bin/itos at its top, and a hooks.bin naming a script of its own. Each
// answers with titles of its own, so a reply drawn with them would show it ran.
const SHIPPED_TOOLS = `${ROOT}/tools/bin/itos`;
const SHIPPED_HOOKS_BIN = `${ROOT}/.tools/bin/itos`;
const SHIPPED_LIST = {
	schema: 1,
	items: [{ id: "T-066", title: "From a program the repository ships" }],
};

type World = {
	root?: string; // the session's root; ROOT when absent
	pathItos?: boolean; // whether an itos is on the PATH
	shipped?: boolean; // whether the repository ships SHIPPED_TOOLS and SHIPPED_HOOKS_BIN
	itos?: Record<string, unknown>; // stdout JSON by "work list --all", "work list" or "task list"
	registry?: string;
	runs: string[][]; // every argv run, in order
	drawn: string[]; // every AssistantMessage text the engine was handed to draw
};

const ok = (stdout: string, exitCode = 0) => ({
	value: { exitCode, stdout, stderr: "", isStdoutTruncated: false, isStderrTruncated: false },
});
type Answer = ReturnType<typeof ok>;

// The itos command in an argv: the words between the program and --json.
const command = (argv: readonly string[]) => argv.slice(1, -1).join(" ");

// itos work list [--all] or task list; one World.itos does not answer is
// refused as a usage error, exit 2, as an itos older than slice 77 refuses
// work list --all.
function listed(w: World, argv: readonly string[]): Answer {
	const answer = w.itos?.[command(argv)];
	return answer === undefined ? ok("", 2) : ok(JSON.stringify(answer));
}

// The itos on the PATH, were it asked for hooks.bin naming the repository's
// own script, and its lists.
const pathItosRun = (w: World, argv: readonly string[]) =>
	argv[1] === "config" ? ok(".tools/bin/itos\n") : listed(w, argv);

// Every program the session could start, by name: the itos on the PATH, the
// repository's top from git, test -x passing each shipped path, and the
// shipped programs themselves; anything else, or one absent, does not start.
const PROGRAMS: Record<string, (w: World, argv: readonly string[]) => Answer | undefined> = {
	itos: (w, argv) => (w.pathItos ? pathItosRun(w, argv) : undefined),
	git: () => ok(`${ROOT}\n`),
	test: (w) => ok("", w.shipped ? 0 : 1),
	[SHIPPED_TOOLS]: (w) => (w.shipped ? ok(JSON.stringify(SHIPPED_LIST)) : undefined),
	[SHIPPED_HOOKS_BIN]: (w) => (w.shipped ? ok(JSON.stringify(SHIPPED_LIST)) : undefined),
};

function processRun(w: World, argv: readonly string[]): Answer {
	const answer = PROGRAMS[argv[0]!]?.(w, argv);
	if (!answer) throw new Error(`${argv[0]}: not found`);
	return answer;
}

// The engine beneath the plugin: the session's root, its processes, its files,
// and the bottoms of the events the plugin passes on.
function world(on: On, w: World): void {
	const root = w.root ?? ROOT;
	on("session.root", async () => ({ value: root }));
	on("process.run", async (_$, e) => {
		w.runs.push([...e.argv]);
		expect(e.init?.cwd).toBe(root);
		return processRun(w, e.argv);
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

// Every program started, in order.
const programs = (w: World) => w.runs.map((argv) => argv[0]);

const start = ($: Engine) =>
	$.session.start({ cwd: ROOT, surface: "terminal", isInteractive: true });

const draw = ($: Engine, text: string) =>
	$.ui.render({
		surface: "terminal",
		component: "AssistantMessage",
		requestId: "m1",
		props: { text, isFirstOfReply: true },
	});

// An itos on the PATH that answers both lists.
const ANSWERS = { pathItos: true, itos: { "work list --all": WORK_LIST, "task list": TASK_LIST } };

test("the titles come from the itos on the PATH: every registry item, then the ledger's tasks", async ($, on) => {
	const w: World = { ...ANSWERS, runs: [], drawn: [] };
	world(on, w);
	await start($);
	expect(w.runs).toEqual([
		["itos", "work", "list", "--all", "--json"],
		["itos", "task", "list", "--json"],
	]);
	await draw($, "T-066 waits on slice 43; T-007 stays.");
	expect(w.drawn).toEqual([
		"`T-066: The itos plugin for Claude Code` waits on `slice-43: itos work list`; `T-007: The commit rules` stays.",
	]);
});

test("an itos older than slice 77, which refuses work list --all, is asked for plain work list", async ($, on) => {
	const w: World = {
		pathItos: true,
		itos: { "work list": WORK_LIST, "task list": TASK_LIST },
		runs: [],
		drawn: [],
	};
	world(on, w);
	await start($);
	expect(asked(w)).toContainEqual(["itos", "work", "list", "--all", "--json"]);
	expect(asked(w).at(-1)).toEqual(["itos", "work", "list", "--json"]);
	await draw($, "T-066 waits on slice 43.");
	expect(w.drawn).toEqual([
		"`T-066: The itos plugin for Claude Code` waits on `slice-43: itos work list`.",
	]);
});

test("drawing runs nothing; the titles are asked for again at each turn's start", async ($, on) => {
	const w: World = { ...ANSWERS, runs: [], drawn: [] };
	world(on, w);
	await start($);
	const ran = w.runs.length;
	await draw($, "T-066");
	await draw($, "T-066 again");
	expect(w.runs).toHaveLength(ran);
	expect(asked(w)).toHaveLength(2);
	w.itos = { "work list --all": { ...WORK_LIST, items: [{ id: "T-066", title: "Renamed" }] } };
	await $.turn.start({ text: "next", turnId: "t2" });
	expect(asked(w)).toHaveLength(4);
	await draw($, "T-066");
	expect(w.drawn.at(-1)).toBe("`T-066: Renamed`");
});

// T-097: a cloned repository's tools/bin/itos, or a script its hooks.bin
// names, would run on every reply for anyone with the plugin enabled. Both are
// there and executable, and the itos on the PATH would name the script if
// asked for hooks.bin; only the itos on the PATH runs, never asked for it.
test("a repository's executable tools/bin/itos and its hooks.bin never run: the itos on the PATH does", async ($, on) => {
	const w: World = { ...ANSWERS, shipped: true, runs: [], drawn: [] };
	world(on, w);
	await start($);
	await $.turn.start({ text: "next", turnId: "t2" });
	expect(new Set(programs(w))).toEqual(new Set(["itos"]));
	expect(w.runs.filter((argv) => argv[1] === "config")).toEqual([]);
	await draw($, "T-066");
	expect(w.drawn).toEqual(["`T-066: The itos plugin for Claude Code`"]);
});

test("from a subfolder too, only the itos on the PATH runs, in the session's root", async ($, on) => {
	const w: World = { ...ANSWERS, root: `${ROOT}/internal/cli`, shipped: true, runs: [], drawn: [] };
	world(on, w);
	await start($);
	expect(new Set(programs(w))).toEqual(new Set(["itos"]));
});

test("with no itos on the PATH, a repository's tools/bin/itos still never runs: the registry file gives the titles", async ($, on) => {
	const w: World = { shipped: true, registry: REGISTRY, runs: [], drawn: [] };
	world(on, w);
	await start($);
	expect(programs(w)).toEqual(["itos", "itos", "itos"]); // work list --all, task list, work list
	await draw($, "T-066 and `slice-43`.");
	expect(w.drawn).toEqual(["`T-066: From the registry file` and `slice-43: itos work list`."]);
});

test("an itos older than v2.3.0, whose work list prints the proposal, reads as no itos for the items", async ($, on) => {
	const w: World = {
		pathItos: true,
		itos: { "work list": PROPOSAL, "task list": TASK_LIST },
		registry: REGISTRY,
		runs: [],
		drawn: [],
	};
	world(on, w);
	await start($);
	expect(programs(w)).toEqual(["itos", "itos", "itos"]); // work list --all, task list, work list
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
