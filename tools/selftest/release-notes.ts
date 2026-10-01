// A release's notes say what a consumer must change. It proves the parts a
// command can decide of docs/releases/v<version>.md, the file the release
// workflow publishes (PLAN.md, §10):
//
//   - the file is there;
//   - its last `##` section is "Upgrading", from which a consumer's session
//     updates alone;
//   - that section has the pin line, naming
//     releases/download/v<version>/itos-<version>.tgz;
//   - every config key whose default differs, appears or disappears between
//     the last release and this tree is named in that section (as its dotted
//     path, `work.registry`), the defaults being each one's
//     `itos config check --print-defaults --json`.
//
// The last release is the newest GitHub release of a v* tag older than this
// version; its tarball is downloaded with its checksums.txt, verified, and
// installed offline into a scratch project, as tools/selftest/release.ts
// installs one. This tree's defaults are tools/bin/itos's.
//
//   node tools/selftest/release-notes.ts                  package.json's version
//   node tools/selftest/release-notes.ts <version>        another version's notes
//   node tools/selftest/release-notes.ts --notes <file>   judge <file> as the notes
//
// Exits 1 on any failure.
import { spawnSync } from "node:child_process";
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { outsideEnv } from "./scratch.ts";

const root = resolve(".");
const args = process.argv.slice(2);
const at = args.indexOf("--notes");
const notesArg = at >= 0 ? args.splice(at, 2)[1] : undefined;
const version =
	args[0]?.replace(/^v/, "") ??
	(JSON.parse(readFileSync(join(root, "package.json"), "utf8")) as { version: string }).version;
const notesFile = resolve(notesArg ?? join(root, "docs", "releases", `v${version}.md`));
const env = outsideEnv();
const scratch = mkdtempSync(join(tmpdir(), "release-notes-selftest-"));
const failures: string[] = [];

function sh(command: string[], cwd = root): string | undefined {
	const result = spawnSync(command[0]!, command.slice(1), { cwd, env, encoding: "utf8" });
	if (result.status === 0) return result.stdout;
	failures.push(`${command.join(" ")} (exit ${result.status ?? result.signal}): ${result.stderr}`);
	return undefined;
}

// The Upgrading section's text, or undefined after saying why there is none.
function upgrading(): string | undefined {
	if (!existsSync(notesFile)) return void failures.push(`no release notes at ${notesFile}`);
	const sections = readFileSync(notesFile, "utf8").split(/^(?=## )/m);
	const last = sections.at(-1)!;
	const heading = /^## (.*)/.exec(last)?.[1]?.trim();
	if (heading !== "Upgrading")
		return void failures.push(
			`the last ## section is ${heading ? `"${heading}"` : "missing"}, not "Upgrading"`,
		);
	return last;
}

const newer = (a: string, b: string) => {
	const [x, y] = [a, b].map((v) => v.split(".").map(Number));
	for (let i = 0; i < 3; i++) if (x![i] !== y![i]) return (x![i] ?? 0) > (y![i] ?? 0);
	return false;
};

// The newest release before this version, as its tag.
function lastRelease(): string | undefined {
	const listed = sh(["gh", "release", "list", "--limit", "100", "--json", "tagName,isDraft"]);
	if (listed === undefined) return undefined;
	const tags = (JSON.parse(listed) as { tagName: string; isDraft: boolean }[])
		.filter((r) => !r.isDraft && /^v\d+\.\d+\.\d+$/.test(r.tagName))
		.map((r) => r.tagName.slice(1))
		.filter((v) => newer(version, v))
		.sort((a, b) => (newer(a, b) ? -1 : 1));
	if (!tags.length) return void failures.push(`no release older than ${version} to compare with`);
	return `v${tags[0]}`;
}

type Defaults = Record<string, unknown>;

function defaultsOf(bin: string): Defaults | undefined {
	const printed = sh([bin, "config", "check", "--print-defaults", "--json"], scratch);
	return printed === undefined
		? undefined
		: (JSON.parse(printed) as { defaults: Defaults }).defaults;
}

// The release's itos, installed offline from its verified tarball.
function releasedBin(tag: string): string | undefined {
	const v = tag.slice(1);
	const download = join(scratch, "download");
	mkdirSync(download);
	const got = sh([
		"gh",
		"release",
		"download",
		tag,
		"--dir",
		download,
		"--pattern",
		`itos-${v}.tgz`,
		"--pattern",
		"checksums.txt",
	]);
	if (got === undefined || sh(["sha256sum", "-c", "checksums.txt"], download) === undefined)
		return undefined;
	const project = join(scratch, "consumer");
	mkdirSync(project);
	writeFileSync(join(project, "package.json"), '{ "name": "consumer", "private": true }\n');
	const installed = sh(
		[
			"npm",
			"install",
			"--offline",
			"--cache",
			join(scratch, "npm-cache"),
			"--no-audit",
			"--no-fund",
			join(download, `itos-${v}.tgz`),
		],
		project,
	);
	return installed === undefined ? undefined : join(project, "node_modules", ".bin", "itos");
}

// Every leaf of the defaults as its dotted path; a list is one leaf.
function leaves(value: unknown, path = "", into = new Map<string, string>()) {
	if (value !== null && typeof value === "object" && !Array.isArray(value))
		for (const [k, v] of Object.entries(value)) leaves(v, path ? `${path}.${k}` : k, into);
	else into.set(path, JSON.stringify(value));
	return into;
}

function changedKeys(before: Defaults, after: Defaults): string[] {
	const [was, now] = [leaves(before), leaves(after)];
	return [...new Set([...was.keys(), ...now.keys()])].filter((k) => was.get(k) !== now.get(k));
}

const section = upgrading();
if (section !== undefined) {
	const tarball = `releases/download/v${version}/itos-${version}.tgz`;
	if (!section.includes(tarball))
		failures.push(`the Upgrading section has no pin line naming ${tarball}`);
}
const tag = lastRelease();
const bin = tag && releasedBin(tag);
const before = bin && defaultsOf(bin);
const after = defaultsOf(join(root, "tools", "bin", "itos"));
let changed: string[] = [];
if (before && after) {
	changed = changedKeys(before, after);
	if (section !== undefined)
		for (const key of changed)
			if (!section.includes(key))
				failures.push(
					`the Upgrading section does not name ${key}, whose default changed since ${tag}`,
				);
}
rmSync(scratch, { recursive: true, force: true });

if (failures.length) {
	console.error(`release notes self-test: ${failures.length} failed`);
	for (const f of failures) console.error(`  - ${f}`);
	process.exit(1);
}
console.log(
	`release notes self-test: ${notesFile} ends with Upgrading, pins itos-${version}.tgz and names ` +
		`every default changed since ${tag} (${changed.join(", ") || "none"})`,
);
