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
	itos?: Record<string, unknown>; // stdout JSON by "work list" or "task list"; absent: no itos
	registry?: string;
	runs: string[][]; // every argv run, in order
	drawn: string[]; // every AssistantMessage text the engine was handed to draw
};

// The engine beneath the plugin: the session's root, its processes, its files,
// and the bottoms of the events the plugin passes on.
function world(on: On, w: World): void {
	on("session.root", async () => ({ value: ROOT }));
	on("process.run", async (_$, e) => {
		w.runs.push([...e.argv]);
		expect(e.init?.cwd).toBe(ROOT);
		if (!w.itos) throw new Error("itos: not found");
		const answer = w.itos[e.argv.slice(1, 3).join(" ")];
		const value = {
			exitCode: answer === undefined ? 1 : 0,
			stdout: JSON.stringify(answer ?? {}),
			stderr: "",
			isStdoutTruncated: false,
			isStderrTruncated: false,
		};
		return { value };
	});
	on("fs.read", async (_$, e) => {
		expect(e.path).toBe(`${ROOT}/tasks/work-items.yaml`);
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
		itos: { "work list": WORK_LIST, "task list": TASK_LIST },
		runs: [],
		drawn: [],
	};
	world(on, w);
	await start($);
	expect(w.runs).toEqual([
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
		itos: { "work list": WORK_LIST, "task list": TASK_LIST },
		runs: [],
		drawn: [],
	};
	world(on, w);
	await start($);
	await draw($, "T-066");
	await draw($, "T-066 again");
	expect(w.runs).toHaveLength(2);
	w.itos = { "work list": { ...WORK_LIST, items: [{ id: "T-066", title: "Renamed" }] } };
	await $.turn.start({ text: "next", turnId: "t2" });
	expect(w.runs).toHaveLength(4);
	await draw($, "T-066");
	expect(w.drawn.at(-1)).toBe("`T-066: Renamed`");
});

test("with no itos on the PATH the registry file gives the titles", async ($, on) => {
	const w: World = { registry: REGISTRY, runs: [], drawn: [] };
	world(on, w);
	await start($);
	await draw($, "T-066 and `slice-43`.");
	expect(w.drawn).toEqual(["`T-066: From the registry file` and `slice-43: itos work list`."]);
});

test("an itos older than v2.3.0, whose work list prints the proposal, reads as no itos for the items", async ($, on) => {
	const w: World = {
		itos: { "work list": PROPOSAL, "task list": TASK_LIST },
		registry: REGISTRY,
		runs: [],
		drawn: [],
	};
	world(on, w);
	await start($);
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
