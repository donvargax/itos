// The Gherkin adapter, built in: which scenarios the `*.feature` files under
// a kind's `root` hold at a tree, and which are live. It is the only module
// that reads a feature file; the tools ask `tests.ts` for a kind's list, and
// the scenario-moves range check reads the blocks through `parseFeature`.
//
// A scenario is the block starting at a line that begins with a tag and holds
// `<tag_prefix><id>` as a whole word, up to the next such line; the header is
// everything before the first one, the Background included. A block is wip if
// its tag line holds `wip_tag`, a file if a tag line of its header does, and a
// scenario is live when neither is: one definition for the footer rule, the
// smoke set and the moves alike.
import { readdirSync, readFileSync } from "node:fs";
import { join, relative } from "node:path";
import { treeTexts } from "./repo.ts";
import type { NamedTest, TestList } from "./tests.ts";

export interface GherkinOptions {
	// The folder of the feature files; a test's `file` is relative to it.
	root: string;
	// A scenario ID's pattern, without its tag prefix.
	id: string;
	tag_prefix: string;
	wip_tag: string;
}

export interface Block {
	wip: boolean;
	// The block as written: its tag line, its Scenario line and its steps.
	body: string;
}
export interface Feature {
	header: string;
	// Tagged wip above its Feature line: wip throughout.
	fileWip: boolean;
	blocks: Map<string, Block>;
}

const quote = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
const tagged = (line: string, tag: RegExp) => line.trim().startsWith("@") && tag.test(line);

// A feature file's header and its scenario blocks, keyed by ID.
export function parseFeature(text: string, options: Omit<GherkinOptions, "root">): Feature {
	const idTag = new RegExp(`${quote(options.tag_prefix)}(${options.id})\\b`);
	const wipTag = new RegExp(`(^|\\s)${quote(options.wip_tag)}(\\s|$)`);
	const blocks = new Map<string, Block>();
	const header: string[] = [];
	let current: { id: string; lines: string[] } | null = null;
	const flush = () => {
		if (current)
			blocks.set(current.id, {
				wip: wipTag.test(current.lines[0]!),
				body: current.lines.join("\n").trimEnd(),
			});
	};
	for (const line of text.split("\n")) {
		const id = idTag.exec(line)?.[1];
		if (id && line.trim().startsWith("@")) {
			flush();
			current = { id, lines: [line] };
		} else if (current) current.lines.push(line);
		else header.push(line);
	}
	flush();
	return {
		header: header.join("\n").trimEnd(),
		fileWip: header.some((line) => tagged(line, wipTag)),
		blocks,
	};
}

const isFeature = (file: string) => file.endsWith(".feature");

function featureFiles(dir: string): string[] {
	let entries;
	try {
		entries = readdirSync(dir, { withFileTypes: true });
	} catch {
		return [];
	}
	return entries.flatMap((e) =>
		e.isDirectory()
			? featureFiles(join(dir, e.name))
			: isFeature(e.name)
				? [join(dir, e.name)]
				: [],
	);
}

// The feature files under `root` as { path: text }, paths as git gives them:
// from the working tree's file system, the index, or a commit. A tree that
// cannot be read has none.
export function featureTexts(tree: string, root: string): Record<string, string> {
	if (tree !== "worktree") return treeTexts(tree, root, isFeature);
	return Object.fromEntries(featureFiles(root).map((path) => [path, readFileSync(path, "utf8")]));
}

// The adapter protocol's list of a tree's scenarios (tests.ts).
export function gherkinList(options: GherkinOptions, at: string): TestList {
	const texts = new Map(
		Object.entries(featureTexts(at, options.root)).map(([path, text]) => [
			relative(options.root, path),
			text,
		]),
	);
	const files = [...texts.keys()].sort();
	const tests: NamedTest[] = [];
	for (const file of files) {
		const { fileWip, blocks } = parseFeature(texts.get(file)!, options);
		for (const [id, block] of blocks) tests.push({ id, file, live: !fileWip && !block.wip });
	}
	return { protocol: 1, tests, files };
}
