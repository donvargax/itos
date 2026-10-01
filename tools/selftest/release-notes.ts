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
//     the last release and this version is named in that section (as its
//     dotted path, `work.registry`), the defaults being each one's
//     `itos config check --print-defaults --json`.
//
// A version whose tag v<version> exists is judged as released: its notes are
// the tag's docs/releases/v<version>.md, the file the release published, and
// its defaults are its released tarball's. So a release task's check stays
// green while later work moves the defaults or edits the tree's copy of the
// notes. Only a version not yet tagged is judged as this tree: its notes are
// the tree's file and its defaults tools/bin/itos's, which is how a release
// task proves its notes before its tag.
//
// The last release is the newest GitHub release of a v* tag older than this
// version. A release's defaults come from its tarball, downloaded with its
// checksums.txt, verified, and installed offline into a scratch project, as
// tools/selftest/release.ts installs one, rather than from the tag's tree: the
// tarball is the bundle consumers run, needing nothing from this checkout,
// while the tag's source would run against this checkout's node_modules, which
// move with the lockfile, so a finished release could go red when a dependency
// does.
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
const env = outsideEnv();
const scratch = mkdtempSync(join(tmpdir(), "release-notes-selftest-"));
const failures: string[] = [];

function sh(command: string[], cwd = root): string | undefined {
	const result = spawnSync(command[0]!, command.slice(1), { cwd, env, encoding: "utf8" });
	if (result.status === 0) return result.stdout;
	failures.push(`${command.join(" ")} (exit ${result.status ?? result.signal}): ${result.stderr}`);
	return undefined;
}

// This version's tag, when it is cut: the release it judges.
const released =
	spawnSync("git", ["rev-parse", "--quiet", "--verify", `refs/tags/v${version}^{commit}`], {
		cwd: root,
		env,
		encoding: "utf8",
	}).status === 0
		? `v${version}`
		: undefined;
const notesPath = `docs/releases/v${version}.md`;
const notesName = notesArg ?? (released ? `${released}:${notesPath}` : notesPath);

// The notes' text, or undefined after saying why there is none.
function notes(): string | undefined {
	if (notesArg !== undefined || !released) {
		const file = resolve(notesArg ?? join(root, notesPath));
		if (!existsSync(file)) return void failures.push(`no release notes at ${file}`);
		return readFileSync(file, "utf8");
	}
	const shown = spawnSync("git", ["show", `${released}:${notesPath}`], {
		cwd: root,
		env,
		encoding: "utf8",
	});
	if (shown.status !== 0) return void failures.push(`no release notes at ${notesName}`);
	return shown.stdout;
}

// The Upgrading section's text, or undefined after saying why there is none.
function upgrading(): string | undefined {
	const text = notes();
	if (text === undefined) return undefined;
	const sections = text.split(/^(?=## )/m);
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
	const download = join(scratch, tag, "download");
	mkdirSync(download, { recursive: true });
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
	const project = join(scratch, tag, "consumer");
	mkdirSync(project, { recursive: true });
	writeFileSync(join(project, "package.json"), '{ "name": "consumer", "private": true }\n');
	const installed = sh(
		[
			"npm",
			"install",
			"--offline",
			"--cache",
			join(scratch, tag, "npm-cache"),
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
// A tagged version's defaults are its release's; only an untagged one is this tree's.
const afterBin = released ? releasedBin(released) : join(root, "tools", "bin", "itos");
const after = afterBin ? defaultsOf(afterBin) : undefined;
let changed: string[] = [];
if (before && after) {
	changed = changedKeys(before, after);
	if (section !== undefined)
		for (const key of changed)
			if (!section.includes(key))
				failures.push(
					`the Upgrading section does not name ${key}, whose default changed between ${tag} and ${released ?? "this tree"}`,
				);
}
rmSync(scratch, { recursive: true, force: true });

if (failures.length) {
	console.error(`release notes self-test: ${failures.length} failed`);
	for (const f of failures) console.error(`  - ${f}`);
	process.exit(1);
}
console.log(
	`release notes self-test: ${notesName} ends with Upgrading, pins itos-${version}.tgz and names ` +
		`every default changed between ${tag} and ${released ?? "this tree"} (${changed.join(", ") || "none"})`,
);
