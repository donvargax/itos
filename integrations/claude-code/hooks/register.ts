import type { EngineInterface, Register } from "claude-code";
import { Annotator, listed, merge, parseRegistry, type Titles } from "./titles";

// Where itos keeps the work registry by default, relative to the project:
// read only when the project's itos does not answer.
const REGISTRY = "tasks/work-items.yaml";

// The titles drawn beside the IDs, asked for at the session's start (a reload
// starts it again) and at each turn's, never while drawing.
let titles: Titles | undefined;

// What a command prints and its exit code in the session's root, or undefined
// when it cannot start.
async function run($: EngineInterface, root: string, argv: string[]) {
	try {
		return await $.process.run(argv, { cwd: root, timeoutMs: 15_000 });
	} catch {
		return undefined;
	}
}

// The repository's top, or undefined outside a repository.
async function topOf($: EngineInterface, root: string): Promise<string | undefined> {
	const git = await run($, root, ["git", "rev-parse", "--show-toplevel"]);
	return (git?.exitCode === 0 && git.stdout.trim()) || undefined;
}

// The effective hooks.bin's words, asked of the itos on the PATH (itos config
// get hooks.bin, from v2.4.0); none when nothing answers: no itos, or one
// older than v2.4.0, whose usage error for the unknown command exits 2.
async function hooksBin($: EngineInterface, root: string): Promise<string[]> {
	const asked = await run($, root, ["itos", "config", "get", "hooks.bin"]);
	return asked?.exitCode === 0 ? asked.stdout.split(/\s+/).filter(Boolean) : [];
}

const executable = async ($: EngineInterface, root: string, path: string | undefined) =>
	path !== undefined && (await run($, root, ["test", "-x", path]))?.exitCode === 0;

// The program a word names: a bare word (itos) as written, a command on the
// PATH; an absolute path as written; a relative one under the repository's
// top, and nothing outside a repository.
const placed = (word: string, top: string | undefined) =>
	!word.includes("/") || word.startsWith("/") ? word : top && `${top}/${word}`;

// Whether a program is taken: a command on the PATH as it is (running it says
// whether it is there), a path only when it is executable.
const runnable = async ($: EngineInterface, root: string, program: string | undefined) =>
	!!program && (!program.includes("/") || (await executable($, root, program)));

// The words hooks.bin answered, to run, or undefined when they name nothing
// runnable.
async function answered(
	$: EngineInterface,
	root: string,
	top: string | undefined,
	words: string[],
): Promise<string[] | undefined> {
	const program = words[0] && placed(words[0], top);
	return (await runnable($, root, program)) ? [program!, ...words.slice(1)] : undefined;
}

// The itos the repository's git hooks run, as the words that start it; the same
// rule as hooks/guard.sh, the guard's: the effective hooks.bin, a hooks.bin of
// several words starting with its first and passing the rest; with no answer,
// tools/bin/itos at the top when it is executable, the v2 default; else the
// itos on the PATH.
async function resolveItos($: EngineInterface, root: string): Promise<string[]> {
	const [words, top] = await Promise.all([hooksBin($, root), topOf($, root)]);
	const local = top && `${top}/tools/bin/itos`;
	const fallback = async () => ((await executable($, root, local)) ? [local!] : ["itos"]);
	return (await answered($, root, top, words)) ?? fallback();
}

// The JSON an itos command prints in the session's root, or undefined when no
// itos answers: none found, a non-zero exit, or output that is not JSON.
async function itos(
	$: EngineInterface,
	root: string,
	bin: string[],
	args: string[],
): Promise<unknown> {
	const answer = await run($, root, [...bin, ...args, "--json"]);
	try {
		return answer?.exitCode === 0 ? JSON.parse(answer.stdout) : undefined;
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
// item, both from the itos the repository's hooks run; the registry file read
// as the mod did when work list gives no items.
async function refresh($: EngineInterface): Promise<void> {
	const root = await $.session.root();
	const bin = await resolveItos($, root);
	const [work, tasks] = await Promise.all([
		itos($, root, bin, ["work", "list"]),
		itos($, root, bin, ["task", "list"]),
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
