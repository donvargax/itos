// The titles of a project's itos work items and tasks, and the rule that
// writes one beside its ID in prose: `T-061` or a bare T-061 becomes
// `T-061: v2.0.0, Go only`. An ID already followed by its title, an ID inside
// a code span that holds more than the ID, and anything in a fenced block are
// left as written, so commands and code are never changed.

export type Titles = ReadonlyMap<string, string>;

// The registry's items, from tasks/work-items.yaml as itos writes it: each
// item a `  - id:` line followed by its `    title:` line. What the titles
// read when no itos answers.
export function parseRegistry(text: string): Map<string, string> {
	const titles = new Map<string, string>();
	const item = /^ {2}- id: *(\S+) *\n {4}title: *(.+?) *$/gm;
	for (const [, id, raw] of text.matchAll(item)) {
		const title = raw!.replace(/^(["'])(.*)\1$/, "$2");
		if (!titles.has(id!)) titles.set(id!, title);
	}
	return titles;
}

// The `id` and `title` of each entry of `list` in an itos command's JSON
// (`items` for itos work list, `tasks` for itos task list), or undefined when
// the answer has no such list: an itos older than v2.3.0 reads `work list` as
// `work` and prints the proposal, which has no `items`.
export function listed(json: unknown, list: "items" | "tasks"): Map<string, string> | undefined {
	const entries: unknown = Object(json)[list];
	if (!Array.isArray(entries)) return undefined;
	const pairs = entries.map((entry) => [Object(entry).id, Object(entry).title]).filter(isPair);
	return merge(pairs);
}

const isPair = (pair: unknown[]): pair is [string, string] =>
	typeof pair[0] === "string" && typeof pair[1] === "string";

// Every source's titles in one map, the first source to name an ID winning:
// the registry's items, then the ledger's tasks that have no item.
export function merge(...sources: (Iterable<[string, string]> | undefined)[]): Map<string, string> {
	const titles = new Map<string, string>();
	for (const source of sources)
		for (const [id, title] of source ?? []) if (!titles.has(id)) titles.set(id, title);
	return titles;
}

const escape = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

// One pattern over every known ID, longest first so T-0610 is never read as
// T-061, bounded so an ID inside a longer word or path is not one.
function idPattern(titles: Titles): RegExp | undefined {
	const ids = [...titles.keys()].sort((a, b) => b.length - a.length).map(escape);
	return ids.length ? new RegExp(`(?<![\\w/.-])(${ids.join("|")})(?![\\w/-])`, "g") : undefined;
}

const titled = (id: string, title: string) => `\`${id}: ${title}\``;

// Whether the text after an ID already gives its title: `ID: …` or `ID (…`,
// as a reply that named it properly writes it.
const alreadyTitled = (after: string, title: string) =>
	/^\s*[:(]/.test(after) || after.slice(0, title.length + 8).includes(title);

// A slice as prose names it, "slice 24" or "Slice 24", for the registry's
// slice-24.
const SLICE = /(?<![\w/.-])[Ss]lice (\d+)(?![\w/-])/g;

// Prose outside code spans: each bare ID gains its title, and so does a
// slice written with a space.
function annotateProse(prose: string, titles: Titles, pattern: RegExp): string {
	const ids = prose.replace(pattern, (id: string, _g: string, at: number, whole: string) => {
		const title = titles.get(id)!;
		return alreadyTitled(whole.slice(at + id.length), title) ? id : titled(id, title);
	});
	return ids.replace(SLICE, (words: string, n: string, at: number, whole: string) => {
		const id = `slice-${n}`;
		const title = titles.get(id);
		return !title || alreadyTitled(whole.slice(at + words.length), title)
			? words
			: titled(id, title);
	});
}

// One line of prose: code spans kept, but a span that is exactly an ID gains
// its title inside the backticks.
export function annotateLine(line: string, titles: Titles): string {
	const pattern = idPattern(titles);
	if (!pattern) return line;
	let out = "";
	let last = 0;
	for (const span of line.matchAll(/`[^`\n]*`/g)) {
		const start = span.index!;
		out += annotateProse(line.slice(last, start), titles, pattern);
		const inner = span[0].slice(1, -1);
		const title = titles.get(inner);
		out +=
			title && !alreadyTitled(line.slice(start + span[0].length), title)
				? titled(inner, title)
				: span[0];
		last = start + span[0].length;
	}
	return out + annotateProse(line.slice(last), titles, pattern);
}

// Text arriving in pieces: whole lines are annotated, a fence toggles the
// code state, and the unfinished tail waits for the next piece or the end.
export class Annotator {
	private pending = "";
	private fenced = false;
	constructor(private readonly titles: Titles) {}

	push(piece: string): string {
		this.pending += piece;
		const cut = this.pending.lastIndexOf("\n");
		if (cut < 0) return "";
		const ready = this.pending.slice(0, cut + 1);
		this.pending = this.pending.slice(cut + 1);
		return this.lines(ready);
	}

	flush(): string {
		const rest = this.pending;
		this.pending = "";
		return this.lines(rest);
	}

	private lines(text: string): string {
		return text
			.split("\n")
			.map((line) => {
				if (/^\s*(```|~~~)/.test(line)) {
					this.fenced = !this.fenced;
					return line;
				}
				return this.fenced ? line : annotateLine(line, this.titles);
			})
			.join("\n");
	}
}
