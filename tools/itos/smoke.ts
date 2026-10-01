// A kind's smoke set: the tests every push's CI runs, beside the ones its
// commits name. This is the one loader every reader goes through. The file is
// the kind's `smoke.file` in itos.yaml (`e2e/smoke.yaml` for scenarios), a
// list in this shape:
//
//   - file: app.feature               relative to the kind's root
//     more: why it has more than one  required when it does
//     scenarios:
//       - id: "@ID-APP-01"
//         why: what it is there for
import { execFileSync } from "node:child_process";
import { existsSync, readFileSync } from "node:fs";
import { parse } from "yaml";
import { kind } from "./tests.ts";

interface SmokeScenario {
	id: string;
	why: string;
}
export interface SmokeFile {
	// Relative to the kind's root.
	file: string;
	// Why the file has more than one, required when it does.
	more?: string;
	scenarios: SmokeScenario[];
}

// The kind's smoke file, as itos.yaml names it.
function smokeFile(name: string): string {
	const file = kind(name).smoke?.file;
	if (!file) throw new Error(`tests.${name}.smoke.file is missing`);
	return file;
}

const isString = (v: unknown): v is string => typeof v === "string";

type Fields = Record<string, unknown>;
const isMap = (v: unknown): v is Fields => !!v && typeof v === "object" && !Array.isArray(v);
const unknownKey = (map: Fields, keys: string[]) => Object.keys(map).find((k) => !keys.includes(k));

// What is wrong with one scenario of an entry, if anything. A missing why is
// the smoke rule's to report, as it always was, so it reads as empty.
function scenarioProblem(s: unknown, at: string): string | undefined {
	if (!isMap(s) || !isString(s.id)) return `${at} has a scenario without an id`;
	const extra = unknownKey(s, ["id", "why"]);
	if (extra) return `${at}: ${s.id} has an unknown key ${extra}`;
	if (s.why === undefined) s.why = "";
	return isString(s.why) ? undefined : `${at}: ${s.id}'s why is not text`;
}

// What is wrong with one entry of the list, if anything.
function entryProblem(e: unknown, n: number): string | undefined {
	if (!isMap(e)) return `entry ${n} is not a map`;
	if (!isString(e.file)) return `entry ${n} has no file`;
	const at = `entry ${n} (${e.file})`;
	const extra = unknownKey(e, ["file", "more", "scenarios"]);
	if (extra) return `${at} has an unknown key ${extra}`;
	if (e.more !== undefined && !isString(e.more)) return `${at}: more is not text`;
	if (!Array.isArray(e.scenarios)) return `${at} has no list of scenarios`;
	for (const s of e.scenarios) {
		const problem = scenarioProblem(s, at);
		if (problem) return problem;
	}
	return undefined;
}

// The list held to its shape, or an error naming the file and the entry.
function checkShape(list: unknown, where: string): SmokeFile[] {
	if (!Array.isArray(list)) throw new Error(`${where}: is not a list of files`);
	list.forEach((entry, i) => {
		const problem = entryProblem(entry, i + 1);
		if (problem) throw new Error(`${where}: ${problem}`);
	});
	return list as SmokeFile[];
}

// A smoke list's text, held to its shape.
function parseSmoke(text: string, path: string): SmokeFile[] {
	return checkShape(parse(text) ?? [], path);
}

// A file's text at a commit, or undefined when the commit has none.
function showAt(at: string, path: string): string | undefined {
	try {
		return execFileSync("git", ["show", `${at}:${path}`], {
			encoding: "utf8",
			stdio: ["ignore", "pipe", "ignore"],
		});
	} catch {
		return undefined;
	}
}

// The kind's smoke set in the working tree, or with `at` at that commit.
export function loadSmoke(name = "scenario", { at }: { at?: string } = {}): SmokeFile[] {
	const path = smokeFile(name);
	if (!at) {
		if (!existsSync(path)) throw new Error(`${path} is missing (tests.${name}.smoke.file)`);
		return parseSmoke(readFileSync(path, "utf8"), path);
	}
	const text = showAt(at, path);
	if (text === undefined) throw new Error(`${at} holds no ${path}`);
	return parseSmoke(text, `${at}:${path}`);
}
