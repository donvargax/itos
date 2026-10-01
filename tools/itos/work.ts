// What to take next: reads the work registry (`work.registry`,
// docs/work-items.yaml), names the person a session works for, and proposes
// the items they can start — `todo`, every dependency done, theirs first,
// then unowned ones in phases nobody owns. Ideas (`kind: idea`, a gap with no
// scenarios or task entry yet) and deferred items (`deferred: <reason>`) are
// listed apart: neither can start until a coordinator specifies it or the
// reason goes. Who works on the project is the logins `work.people` lists
// (the All Contributors table in CONTRIBUTORS.md, say); who a session works
// for is what `work.identity` answers (the GitHub account `gh` is signed in
// as, say). Both are providers chosen in itos.yaml (providers.ts).
//
//   itos work [--as <handle>] [--json]   the proposal, for the person
//   itos work check [<file>]             validate the registry, exit 1 on a problem
//
// Every problem carries a rule id and a fix for `--json`.
import { readFileSync } from "node:fs";
import { parse } from "yaml";
import { config, type PeopleConfig } from "./config.ts";
import { emit, messages, type Output, problem, type Problem, TEXT } from "./problem.ts";
import { type Answer, DEFAULT_PEOPLE, identityProvider, people } from "./providers.ts";

const STATUSES = ["todo", "doing", "done", "blocked"] as const;
type Status = (typeof STATUSES)[number];
const KINDS = ["slice", "task", "idea"] as const;
type Kind = (typeof KINDS)[number];

interface Item {
	id: string;
	title: string;
	phase: number;
	owner: string | null;
	status: Status;
	depends_on: string[];
	kind?: Kind;
	why?: string;
	deferred?: string;
	issue?: number;
	refs?: string[];
	blocked_by?: string;
}

interface Registry {
	logins: string[];
	phases: Record<string, string | null>;
	items: Item[];
}

const registryPath = () => config().work?.registry ?? "docs/work-items.yaml";
const peopleSource = () => config().work?.people ?? DEFAULT_PEOPLE;
// The file the people come from, as the messages name it.
const listedIn = () => peopleSource().file;

export function load(path = registryPath(), source: PeopleConfig = peopleSource()): Registry {
	const raw = parse(readFileSync(path, "utf8")) as Partial<Registry>;
	return {
		logins: people(source),
		phases: raw.phases ?? {},
		items: (raw.items ?? []).map((item) => ({ ...item, depends_on: item.depends_on ?? [] })),
	};
}

const text = (value: unknown) => typeof value === "string" && value.trim() !== "";

// Each problem is a rule id, the sentence `--check` prints and, where one
// exists, the edit that resolves it.
const when = (holds: boolean, rule: string, message: string, fix?: string): Problem[] =>
	holds ? [problem(rule, message, fix)] : [];

const statusIssues = (item: Item): Problem[] => [
	...when(
		!STATUSES.includes(item.status),
		"work-unknown-status",
		`${item.id}: unknown status "${item.status}"`,
		`set ${item.id}'s status to one of ${STATUSES.join(", ")}`,
	),
	...when(
		item.kind !== undefined && !KINDS.includes(item.kind),
		"work-unknown-kind",
		`${item.id}: unknown kind "${item.kind}"`,
		`set ${item.id}'s kind to one of ${KINDS.join(", ")}`,
	),
	// An idea is specified (kind slice or task) before anyone takes it.
	...when(
		item.kind === "idea" && item.status !== "todo",
		"work-idea-started",
		`${item.id}: an idea is ${item.status}; specify it first (kind slice or task)`,
		`specify ${item.id} (kind: slice or task, with its scenarios or ledger entry), or set it back to todo`,
	),
];

const textIssues = (item: Item): Problem[] => [
	...when(
		item.why !== undefined && !text(item.why),
		"work-why-not-text",
		`${item.id}: why is not a text`,
		`write ${item.id}'s why as text, or remove it`,
	),
	...when(
		item.deferred !== undefined && !text(item.deferred),
		"work-deferred-no-reason",
		`${item.id}: deferred needs its reason`,
		`write why ${item.id} is deferred in its deferred:, or remove the key`,
	),
	...when(
		item.deferred !== undefined && item.status !== "todo",
		"work-deferred-started",
		`${item.id}: deferred, but ${item.status}`,
		`remove ${item.id}'s deferred:, or set it back to todo`,
	),
];

const ownerIssues = (item: Item, handles: Set<string>): Problem[] =>
	when(
		item.owner !== null && !handles.has(item.owner),
		"work-unknown-owner",
		`${item.id}: owner "${item.owner}" is not in ${listedIn()}`,
		`add ${item.owner} to ${listedIn()}, or set ${item.id}'s owner to one of its logins`,
	);

const dependencyIssues = (item: Item, byId: Map<string, Item>): Problem[] =>
	item.depends_on.flatMap((dep) => {
		const other = byId.get(dep);
		if (!other)
			return [
				problem(
					"work-unknown-dependency",
					`${item.id}: depends on unknown "${dep}"`,
					`remove "${dep}" from ${item.id}'s depends_on, or add an item ${dep}`,
				),
			];
		return when(
			item.status === "done" && other.status !== "done",
			"work-done-before-dependency",
			`${item.id}: done, but "${dep}" is ${other.status}`,
			`finish ${dep} first, or set ${item.id} back to doing`,
		);
	});

const itemIssues = (item: Item, byId: Map<string, Item>, handles: Set<string>): Problem[] => [
	...when(!item.title, "work-no-title", `${item.id}: no title`, `give ${item.id} a title`),
	...statusIssues(item),
	...textIssues(item),
	...ownerIssues(item, handles),
	...dependencyIssues(item, byId),
];

// Every cycle: an item reached again while walking its own dependencies.
function cycles(byId: Map<string, Item>, file: string): Problem[] {
	const found: Problem[] = [];
	const state = new Map<string, "walking" | "done">();
	const walk = (id: string, path: string[]) => {
		if (state.get(id) === "walking") {
			const loop = [...path.slice(path.indexOf(id)), id];
			found.push(
				problem(
					"work-cycle",
					`cycle: ${loop.join(" → ")}`,
					`remove one of the depends_on links ${loop.join(" → ")} in ${file}`,
				),
			);
		}
		if (state.has(id)) return;
		state.set(id, "walking");
		for (const dep of byId.get(id)?.depends_on ?? []) walk(dep, [...path, id]);
		state.set(id, "done");
	};
	for (const id of byId.keys()) walk(id, []);
	return found;
}

// Every problem with the registry, with its rule id; none means it is sound.
export function registryIssues(
	{ logins, phases, items }: Registry,
	file = registryPath(),
): Problem[] {
	const handles = new Set(logins);
	const byId = new Map(items.map((item) => [item.id, item]));
	const twice = items.filter((item, i) => items.findIndex((o) => o.id === item.id) !== i);
	const unlisted = items.filter((item) => !(String(item.phase) in phases));
	const strangers = Object.entries(phases).filter(([, o]) => o !== null && !handles.has(o));
	return [
		...twice.map((item) =>
			problem(
				"work-duplicate-id",
				`${item.id}: listed twice`,
				`rename or remove one of the two ${item.id} items`,
			),
		),
		...unlisted.map((item) =>
			problem(
				"work-unknown-phase",
				`${item.id}: phase ${item.phase} is not listed`,
				`add phase ${item.phase} to phases: in ${file}, or move ${item.id} to a listed one`,
			),
		),
		...items.flatMap((item) => itemIssues(item, byId, handles)),
		...strangers.map(([phase, owner]) =>
			problem(
				"work-unknown-phase-owner",
				`phase ${phase}: owner "${owner}" is not in ${listedIn()}`,
				`add ${owner} to ${listedIn()}, or set phase ${phase}'s owner to one of its logins`,
			),
		),
		...cycles(byId, file),
	];
}

// The same, as one line each.
export const problems = (registry: Registry): string[] => messages(registryIssues(registry));

export type Identity = { handle: string; listed: boolean } | { problem: string };

// Who a session works for: `--as`, else what the identity provider answers.
export function whoami(
	logins: string[],
	as?: string,
	identify: () => Answer = identityProvider(),
): Identity {
	if (as)
		return logins.includes(as)
			? { handle: as, listed: true }
			: { problem: `"${as}" is not in ${listedIn()}` };
	const answer = identify();
	return "handle" in answer ? { ...answer, listed: logins.includes(answer.handle) } : answer;
}

export interface Proposal {
	person: string | null;
	doing: Item[];
	next: Item[];
	unowned: Item[];
	waiting: { item: Item; on: string[] }[];
	ideas: { item: Item; on: string[] }[];
	deferred: Item[];
}

export function propose(registry: Registry, handle: string | null): Proposal {
	const done = new Set(registry.items.filter((i) => i.status === "done").map((i) => i.id));
	const open = (item: Item) => item.depends_on.filter((dep) => !done.has(dep));
	const ownerOf = (item: Item) => item.owner ?? registry.phases[String(item.phase)] ?? null;
	const mine = (item: Item) => handle !== null && ownerOf(item) === handle;
	const todoItems = registry.items.filter((i) => i.status === "todo");
	// What a person may look at: theirs, or nobody's.
	const theirs = todoItems.filter((i) => mine(i) || ownerOf(i) === null);
	const deferred = theirs.filter((i) => i.deferred !== undefined);
	const ideas = theirs.filter((i) => i.kind === "idea" && i.deferred === undefined);
	const todo = todoItems.filter((i) => i.kind !== "idea" && i.deferred === undefined);
	return {
		person: handle,
		doing: registry.items.filter((i) => i.status === "doing" && mine(i)),
		next: todo.filter((i) => mine(i) && open(i).length === 0),
		unowned: todo.filter((i) => ownerOf(i) === null && open(i).length === 0),
		waiting: todo.filter(mine).flatMap((item) => {
			const on = open(item);
			return on.length ? [{ item, on }] : [];
		}),
		ideas: ideas.map((item) => ({ item, on: open(item) })),
		deferred,
	};
}

const line = (item: Item) => `  ${item.id}  ${item.title}${item.issue ? `  (#${item.issue})` : ""}`;

function print(p: Proposal) {
	console.log(p.person ? `Working for ${p.person}.` : "Working for nobody.");
	const section = (title: string, items: Item[]) => {
		if (!items.length) return;
		console.log(`\n${title}`);
		for (const item of items) console.log(line(item));
	};
	section("In progress:", p.doing);
	section("Can start now:", p.next);
	section("Unowned, can start now (agree an owner first):", p.unowned);
	if (p.waiting.length) {
		console.log("\nWaiting:");
		for (const { item, on } of p.waiting) console.log(`${line(item)}  ← ${on.join(", ")}`);
	}
	if (p.ideas.length) {
		console.log("\nIdeas, not yet specified (a coordinator specifies one before it starts):");
		for (const { item, on } of p.ideas)
			console.log(`${line(item)}${on.length ? `  ← ${on.join(", ")}` : ""}`);
	}
	if (p.deferred.length) {
		console.log("\nDeferred:");
		for (const item of p.deferred) console.log(`${line(item)}  — ${item.deferred?.trim()}`);
	}
}

// `work check [<file>]`: 0 when the registry is sound, 1 with its problems.
export function workCheck(path?: string, out: Output = TEXT): number {
	const file = path ?? registryPath();
	const found = registryIssues(load(path), file);
	if (out.json) emit({ file, sound: found.length === 0, problems: found });
	else for (const p of found) console.error(p.message);
	if (found.length) return 1;
	if (!out.json && !out.quiet) console.log(`${file}: sound`);
	return 0;
}

// `work [--as <h>]`: what the person can start. 1 when the registry is not
// sound; 3 (a missing environment) when `--as` is not among the people.
export function work(as: string | undefined, out: Output = TEXT): number {
	const sound = workCheck(undefined, { json: false, quiet: true });
	if (sound !== 0) return sound;
	const registry = load();
	const who = whoami(registry.logins, as);
	if ("problem" in who) {
		console.error(who.problem);
		if (as) return 3;
	} else if (!who.listed)
		console.error(`${who.handle} is not in ${listedIn()}: nothing is theirs yet`);
	const proposal = propose(registry, "handle" in who ? who.handle : null);
	if (out.json) emit(proposal);
	else print(proposal);
	return 0;
}
