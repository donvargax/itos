// Named tests behind an adapter: the one
// layer that knows how a kind of named test is listed and run. The rest of the
// tooling asks it which tests exist at a tree (a footer's IDs, the smoke set),
// what a task check that runs some of them selects, and the one command that
// runs several selections; everything a kind does is `itos.yaml`'s
// `tests.<kind>`.
//
// An adapter answers only which tests exist at a tree (`worktree`, `index` or
// a commit) and which are live, as one object (the adapter protocol):
// `{"protocol": 1, "tests": [{"id", "file", "live"}…], "files": […]}`, with
// `file` relative to the kind's root and `files` every file, test-less ones
// included. `itos tests list <kind> [--at <tree>]` prints it.
//
//   adapter: gherkin                     built in (gherkin.ts),
//                                        from `root`, `id`, `tag_prefix` (`@`)
//                                        and `wip_tag` (`@wip`)
//   adapter: { command: <cmd>,           `<cmd> list --at <tree>` prints the
//              supports_at: false }      JSON above and exits 0; without
//                                        `supports_at`, a tree other than the
//                                        working tree is read as the working
//                                        tree, with one warning per run
//
// A non-zero exit or output that is not the protocol fails whatever needed the
// list, with the adapter's stderr: the tool never guesses.
//
// The kind's `run` templates turn selections into a command (`whole` if any
// selection is whole; else each `ids` selection is one pattern by
// `ids_pattern`, the patterns deduplicated in order and combined by `join`),
// and its `recognize` templates read a task check back as a selection (a
// `{pattern}` is one shell word, quoted or bare; `as: smoke` is the smoke set).
import { spawnSync } from "node:child_process";
import { config, ConfigError, configPath } from "./config.ts";
import { gherkinList } from "./gherkin.ts";

export interface NamedTest {
	id: string;
	// Relative to the kind's root: the smoke rule's unit.
	file: string;
	live: boolean;
}
export interface TestList {
	protocol: 1;
	tests: NamedTest[];
	// Every file, the ones without a test included.
	files: string[];
}

// What one run of a kind selects: every test, some IDs, or a pattern from a
// task check.
export type Selection = { whole: true } | { ids: string[] } | { pattern: string };

export type Adapter = string | { command: string; supports_at?: boolean };
export interface Kind {
	adapter?: Adapter;
	root?: string;
	id?: string;
	tag_prefix?: string;
	wip_tag?: string;
	run?: {
		whole?: string;
		select?: string;
		ids_pattern?: string;
		join?: { each?: string; sep?: string };
	};
	recognize?: { command: string; as: string }[];
	smoke?: { file?: string; every_file?: boolean; add_hint?: string };
}

export function kind(name: string): Kind {
	const found = config().tests?.[name] as Kind | undefined;
	if (!found) throw new ConfigError(configPath(), [`tests.${name} is missing`]);
	return found;
}

// An ID as the kind's lists hold it: without its tag prefix.
export const bareId = (name: string, id: string) => {
	const prefix = kind(name).tag_prefix ?? "";
	return prefix && id.startsWith(prefix) ? id.slice(prefix.length) : id;
};

const warned = new Set<string>();

// The kind's tests at a tree: `worktree`, `index` or a commit. `root` stands
// in for the kind's own (a copy of its files, in a self-test).
export function listTests(
	name: string,
	{ at = "worktree", root }: { at?: string; root?: string } = {},
): TestList {
	const k = kind(name);
	const adapter = k.adapter ?? "gherkin";
	if (adapter === "gherkin") {
		// Gherkin's tags start with `@`; the root and the ID pattern are the kind's.
		const need = (key: "root" | "id") => {
			const value = key === "root" ? (root ?? k.root) : k[key];
			if (value === undefined)
				throw new ConfigError(configPath(), [`tests.${name}.${key} is missing`]);
			return value;
		};
		return gherkinList(
			{
				root: need("root"),
				id: need("id"),
				tag_prefix: k.tag_prefix ?? "@",
				wip_tag: k.wip_tag ?? "@wip",
			},
			at,
		);
	}
	if (typeof adapter === "string")
		throw new ConfigError(configPath(), [`tests.${name}.adapter ${adapter} is not built in`]);
	let tree = at;
	if (adapter.supports_at === false && tree !== "worktree") {
		if (!warned.has(name))
			console.warn(`tests.${name} cannot list a tree; ${tree} is read as the working tree`);
		warned.add(name);
		tree = "worktree";
	}
	return commandList(name, adapter.command, tree);
}

// A command adapter's list, held to the protocol.
function commandList(name: string, command: string, at: string): TestList {
	const [shell = "sh", ...flags] = config().shell ?? ["sh", "-c"];
	const run = spawnSync(shell, [...flags, `${command} list --at ${shellWord(at)}`], {
		encoding: "utf8",
		maxBuffer: 64 * 1024 * 1024,
	});
	const fail = (why: string): never => {
		throw new Error(
			`tests.${name}: \`${command} list --at ${at}\` ${why}\n${run.stderr ?? ""}`.trimEnd(),
		);
	};
	if (run.status !== 0) fail(`exited ${run.status ?? run.signal}`);
	let list: unknown;
	try {
		list = JSON.parse(run.stdout);
	} catch {
		fail("did not print JSON");
	}
	const problem = protocolProblem(list);
	if (problem) fail(problem);
	return list as TestList;
}

function protocolProblem(list: unknown): string | undefined {
	const l = list as Partial<TestList> | null;
	if (!l || typeof l !== "object") return "printed no object";
	if (l.protocol !== 1) return `speaks protocol ${JSON.stringify(l.protocol)}, not 1`;
	if (!Array.isArray(l.files) || !l.files.every((f) => typeof f === "string"))
		return "printed no list of files";
	if (!Array.isArray(l.tests)) return "printed no list of tests";
	const bad = l.tests.find(
		(t) =>
			!t || typeof t.id !== "string" || typeof t.file !== "string" || typeof t.live !== "boolean",
	);
	return bad ? `printed a test that is not { id, file, live }: ${JSON.stringify(bad)}` : undefined;
}

// A string as one shell word.
export const shellWord = (s: string) => `'${s.replaceAll("'", `'\\''`)}'`;

const template = (name: string, key: "whole" | "select" | "ids_pattern") => {
	const value = kind(name).run?.[key];
	if (value === undefined)
		throw new ConfigError(configPath(), [`tests.${name}.run.${key} is missing`]);
	return value;
};

// One command that runs every selection, or none for no selection.
export function commandFor(name: string, selections: Selection[]): string | undefined {
	if (selections.length === 0) return undefined;
	if (selections.some((s) => "whole" in s)) return template(name, "whole");
	const patterns = [
		...new Set(
			selections.map((s) =>
				"ids" in s
					? template(name, "ids_pattern").replace("{ids}", () =>
							[...new Set(s.ids.map((id) => bareId(name, id)))].join("|"),
						)
					: (s as { pattern: string }).pattern,
			),
		),
	];
	const join = kind(name).run?.join ?? {};
	const pattern =
		patterns.length === 1
			? patterns[0]!
			: patterns.map((p) => (join.each ?? "{p}").replace("{p}", () => p)).join(join.sep ?? "|");
	return template(name, "select").replace("{pattern}", () => shellWord(pattern));
}

// A `{pattern}` in a recognize template: one shell word, double-quoted
// without `$`, `` ` ``, `"` or `\`, single-quoted, or bare.
const WORD = `(?:"([^"$\`\\\\]+)"|'([^']+)'|([^\\s"'$\`\\\\]+))`;
const templateRegExp = (command: string) =>
	new RegExp(
		`^${command
			.trim()
			.split(/\s+/)
			.map((word) =>
				word
					.split("{pattern}")
					.map((part) => part.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"))
					.join(WORD),
			)
			.join("\\s+")}$`,
	);

// A task check read as a selection of the kind, or undefined for a check that
// is not one of its runs (other flags, a pattern the shell would expand): that
// check runs as it is. `smoke` is the kind's smoke set, for `as: smoke`.
export function recognize(
	name: string,
	command: string,
	smoke: string[] = [],
): Selection | undefined {
	const c = command.trim();
	for (const rule of kind(name).recognize ?? []) {
		const m = templateRegExp(rule.command).exec(c);
		if (!m) continue;
		if (rule.as === "whole") return { whole: true };
		if (rule.as === "smoke") return { ids: smoke };
		const pattern = m[1] ?? m[2] ?? m[3];
		if (pattern !== undefined) return { pattern };
	}
	return undefined;
}
