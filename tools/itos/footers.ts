// The one footer reader: a message's footers as `itos.yaml`'s
// `commits.footers` defines them. It answers the commit-msg rule (a
// commitlint config makes one rule of `checkFooter` per key) and CI's lists
// of the IDs a pushed range names (`ci-scope.ts`'s `tasksIn` and
// `testsNamedIn`). No key, type or rule is written here.
//
// A footer is `<Key>: <id> <id>, …` at the start of a line, and may repeat
// over several lines to stay within the line length limit. Per key:
//
//   source        `ledger` (the task IDs) or `{ tests: <kind> }` (a kind's
//                 named tests, as its adapter lists them: tests.ts)
//   strip_prefix  what an ID may start with and is read without (`@`)
//   required_for  the types whose commits must carry it, or `all`
//   validate_for  the types whose IDs must exist, or `all`
//   must_be_live  a named test must be live, as its adapter says
//   read_at       `commit`: the IDs that exist are the ones of the commit
//                 being checked — the staged tree in the commit-msg hook,
//                 the commit ITOS_AT names in the re-check of a pushed range
//                 (verify-commits.ts), so a later commit that sets a test
//                 back to wip or drops a task does not fail an older one.
//                 `worktree`: the working tree.
import { spawnSync } from "node:child_process";
import { config, type FooterConfig, section } from "./config.ts";
import { ledgerIds } from "./repo.ts";
import { listTests } from "./tests.ts";

export type Footer = FooterConfig & { key: string };

export const footers = (): Footer[] =>
	Object.entries(config().commits?.footers ?? {}).map(([key, rule]) => ({ ...rule, key }));

export function footer(key: string): Footer {
	const found = footers().find((f) => f.key === key);
	if (!found) throw new Error(`itos.yaml has no commits.footers.${key}`);
	return found;
}

const applies = (types: string | string[] | undefined, type: string) =>
	types === "all" || (Array.isArray(types) && types.includes(type));

// The ID pattern a footer's IDs match: the ledger's, or its kind's.
const idPattern = (f: Footer): string | undefined =>
	f.source === "ledger" ? config().ledger?.id : config().tests?.[f.source.tests]?.id;

// The IDs one footer gives in a text (a message, or a range's messages), in
// order, the prefix stripped, not deduplicated.
export function footerIds(text: string, key: string, strip = footer(key).strip_prefix): string[] {
	return text
		.split("\n")
		.filter((line) => line.startsWith(`${key}:`))
		.flatMap((line) =>
			line
				.slice(key.length + 1)
				.split(/[\s,]+/)
				.filter(Boolean),
		)
		.map((id) => (strip && id.startsWith(strip) ? id.slice(strip.length) : id));
}

// The tree a footer's IDs are read at: a commit, "index", or the working tree.
const treeOf = (f: Footer): string | undefined =>
	f.read_at === "commit" ? process.env.ITOS_AT || "index" : undefined;

// The IDs that exist for a footer at a tree (the working tree without one),
// and the ones not live.
function knownAt(f: Footer, tree?: string): { all: Set<string>; notLive: Set<string> } {
	if (f.source === "ledger") return { all: ledgerIds(tree), notLive: new Set() };
	const { tests } = listTests(f.source.tests, { at: tree ?? "worktree" });
	return {
		all: new Set(tests.map((t) => t.id)),
		notLive: new Set(tests.filter((t) => !t.live).map((t) => t.id)),
	};
}

// The IDs that exist for a footer where its `read_at` says. A commit with none
// at all predates its source (a project's first commits may name tasks
// before the ledger is committed), so there is nothing to read at it: the
// working tree is read instead, and a warning says so.
function known(f: Footer) {
	const tree = treeOf(f);
	const found = knownAt(f, tree);
	if (!tree || found.all.size > 0) return found;
	console.warn(`${tree} has no ${noun(f)}; its ${f.key}: footer is read against the working tree`);
	return knownAt(f);
}

// How the messages name a footer and its IDs: `"Task: T-…"`, `tasks`.
const example = (f: Footer) =>
	`"${f.key}: ${f.strip_prefix ?? ""}${/^[\w-]*/.exec(idPattern(f) ?? "")![0]}…"`;
const noun = (f: Footer) => `${f.source === "ledger" ? "task" : f.source.tests}s`;
// Why a named test is not live: its kind's wip tag, when it has one.
const notLiveWhy = (f: Footer) => {
	const tag = f.source !== "ledger" && section("tests")[f.source.tests]?.wip_tag;
	return typeof tag === "string" ? `still tagged ${tag}` : "not live";
};

// One footer's rule over one message: [true], or [false, why].
export function checkFooter(
	key: string,
	type: string | null | undefined,
	message: string,
): [boolean, string?] {
	const f = footer(key);
	const ids = footerIds(message, key);
	if (applies(f.required_for, type ?? "") && ids.length === 0)
		return [false, `${type} commits need a ${example(f)} footer`];
	if (!applies(f.validate_for, type ?? "")) return [true];
	const { all, notLive } = known(f);
	const unknown = ids.filter((id) => !all.has(id));
	if (unknown.length) return [false, `unknown ${noun(f)}: ${unknown.join(", ")}`];
	const pending = f.must_be_live ? ids.filter((id) => notLive.has(id)) : [];
	if (pending.length) return [false, `${notLiveWhy(f)}: ${pending.join(", ")}`];
	return [true];
}

// The IDs a pushed range's commits give in one footer, each once, in the
// order first given, and only those that match the footer's ID pattern. A
// range that cannot be read gives none.
export function footerIdsIn(from: string, to: string, key: string): string[] {
	if (!from || !to) return [];
	const { status, stdout } = spawnSync("git", ["log", "--format=%B", `${from}..${to}`], {
		encoding: "utf8",
	});
	if (status !== 0) return [];
	const f = footer(key);
	const pattern = new RegExp(`^(?:${idPattern(f) ?? ".+"})$`);
	return [...new Set(footerIds(stdout, key, ""))]
		.map((id) =>
			f.strip_prefix && id.startsWith(f.strip_prefix) ? id.slice(f.strip_prefix.length) : id,
		)
		.filter((id) => pattern.test(id));
}
