import type { EngineInterface, Register } from "claude-code";
import { Annotator, listed, merge, parseRegistry, type Titles } from "./titles";

// Where itos keeps the work registry by default, relative to the project:
// read only when the project's itos does not answer.
const REGISTRY = "tasks/work-items.yaml";

// The titles drawn beside the IDs, asked for at the session's start (a reload
// starts it again) and at each turn's, never while drawing.
let titles: Titles | undefined;

// The JSON an itos command prints in the session's root, or undefined when no
// itos answers: none on the PATH, a non-zero exit, or output that is not JSON.
async function itos($: EngineInterface, root: string, args: string[]): Promise<unknown> {
	try {
		const { exitCode, stdout } = await $.process.run(["itos", ...args, "--json"], {
			cwd: root,
			timeoutMs: 15_000,
		});
		return exitCode === 0 ? JSON.parse(stdout) : undefined;
	} catch {
		return undefined;
	}
}

async function registryFile($: EngineInterface, root: string): Promise<Titles | undefined> {
	try {
		return parseRegistry(await $.fs.read(`${root}/${REGISTRY}`));
	} catch {
		return undefined;
	}
}

// Every registry item from itos work list, wherever work.registry puts the
// registry, and the ledger's tasks from itos task list, which may have no
// item; the registry file read as the mod did when work list gives no items.
async function refresh($: EngineInterface): Promise<void> {
	const root = await $.session.root();
	const [work, tasks] = await Promise.all([
		itos($, root, ["work", "list"]),
		itos($, root, ["task", "list"]),
	]);
	const items = listed(work, "items") ?? (await registryFile($, root));
	titles = merge(items, listed(tasks, "tasks"));
}

// A reply's text block with each bare ID titled, fenced code and longer code
// spans left as written.
function annotate(text: string, known: Titles): string {
	const a = new Annotator(known);
	return a.push(text) + a.flush();
}

export const register: Register = (on) => {
	on("session.start", async ($, e, next) => {
		await refresh($);
		return next(e);
	});

	on("turn.start", async ($, e, next) => {
		await refresh($);
		return next(e);
	});

	// The drawing alone: the stored reply, and what the model reads back, stay
	// as written, so the stream and the transcript are never touched.
	on("ui.render", { component: "AssistantMessage" }, async (_$, e, next) => {
		if (!titles?.size) return next(e);
		const text = annotate(e.props.text, titles);
		return text === e.props.text ? next(e) : next({ ...e, props: { ...e.props, text } });
	});
};
