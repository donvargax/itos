// A release's notes say what a consumer must change. It proves the parts a
// command can decide of docs/releases/v<version>.md, the file the release
// workflow publishes (PLAN.md, §10):
//
//   - the file is there;
//   - its last `##` section is "Upgrading", from which a consumer's session
//     updates alone;
//   - that section gives the pin to change. From v2.0.0, Go only, it is the
//     binary's install lines: the install script's `version=<version>`, its
//     download from releases/download/v<version>/ (or v$version/), and
//     checksums.txt, where each platform's hash is taken from; and it names no
//     itos-<version>.tgz, which no release publishes since the TypeScript left
//     (T-062). A version before 2.0.0 shipped the TypeScript tarball, and its
//     section has the pin line naming releases/download/v<version>/itos-<version>.tgz;
//   - every Upgrading footer of the commits since the last release, as
//     `itos commit footers Upgrading <last release> <tag, or HEAD> --json`
//     lists them (those saying none left out), appears in that section, its
//     text whitespace and case aside; a range whose commits carry none (any before
//     this repository required the footer, T-061) passes, and says so;
//   - every config key whose default differs, appears or disappears between
//     the last release and this version is named in that section (as its
//     dotted path, `work.registry`), the defaults being each one's
//     `itos config check --print-defaults --json`.
//
// A version whose tag v<version> exists is judged as released: its notes are
// the tag's docs/releases/v<version>.md, the file the release published, its
// footers the range up to its tag, and its defaults are its released
// binary's. So a release task's check stays green while later work moves the
// defaults or edits the tree's copy of the notes. Only a version not yet
// tagged is judged as this tree: its notes are the tree's file, its footers
// the range up to HEAD and its defaults tools/bin/itos's, which is how a
// release task proves its notes before its tag.
//
// The last release is the newest GitHub release of a v* tag older than this
// version. A release's defaults come from what it published, downloaded with
// its checksums.txt and verified, rather than from the tag's tree, which would
// build against today's toolchain: from v1.0.0 this machine's Go archive,
// unpacked; before it, the TypeScript tarball, installed offline into a
// scratch project with npm, the bundle those releases' consumers ran.
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

// This machine's platform, as the release names its archives.
const goarch: Record<string, string> = { x64: "amd64", arm64: "arm64" };
const platform = `${process.platform}-${goarch[process.arch] ?? process.arch}`;

// Whether version a is b or later.
const atLeast = (a: string, b: string) => !newer(b, a);

// The release's itos, from what it published: its asset verified against its
// checksums.txt, then this machine's binary unpacked (from v1.0.0) or the
// tarball installed offline (before it).
function releasedBin(tag: string): string | undefined {
	const v = tag.slice(1);
	const go = atLeast(v, "1.0.0");
	const asset = go ? `itos-${v}-${platform}.tar.gz` : `itos-${v}.tgz`;
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
		asset,
		"--pattern",
		"checksums.txt",
	]);
	// From v1.0.0 checksums.txt lists every asset; only one is downloaded, and
	// sha256sum fails when it verifies no file at all.
	if (
		got === undefined ||
		sh(["sha256sum", "--ignore-missing", "-c", "checksums.txt"], download) === undefined
	)
		return undefined;
	if (go) {
		const unpacked = join(scratch, tag, "bin");
		mkdirSync(unpacked, { recursive: true });
		return sh(["tar", "-xzf", join(download, asset), "-C", unpacked, "itos"]) === undefined
			? undefined
			: join(unpacked, "itos");
	}
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
			join(download, asset),
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

// Whitespace and case aside: the notes wrap and indent what a footer says on
// one line, and may start a sentence with it.
const flat = (text: string) => text.replace(/\s+/g, " ").trim().toLowerCase();

const section = upgrading();
const goOnly = atLeast(version, "2.0.0");
if (section !== undefined) {
	if (goOnly) {
		if (!section.includes(`version=${version}`))
			failures.push(`the Upgrading section has no install line setting version=${version}`);
		if (
			!new RegExp(`releases/download/v(\\$version|${version.replace(/\./g, "\\.")})/`).test(section)
		)
			failures.push(
				`the Upgrading section has no install line downloading from releases/download/v${version}/ (or v$version/)`,
			);
		if (!section.includes("checksums.txt"))
			failures.push(
				"the Upgrading section does not name checksums.txt, where the install script's hashes come from",
			);
		if (section.includes(`itos-${version}.tgz`))
			failures.push(
				`the Upgrading section names itos-${version}.tgz, a tarball no release publishes since the TypeScript left`,
			);
	} else {
		const tarball = `releases/download/v${version}/itos-${version}.tgz`;
		if (!section.includes(tarball))
			failures.push(`the Upgrading section has no pin line naming ${tarball}`);
	}
}
const tag = lastRelease();
const end = released ?? "HEAD";

// The range's Upgrading footers, each held to the section.
let footers: { sha: string; text: string }[] = [];
if (tag) {
	const listed = sh([
		join(root, "tools", "bin", "itos"),
		"commit",
		"footers",
		"Upgrading",
		tag,
		end,
		"--json",
	]);
	if (listed !== undefined) {
		footers = (JSON.parse(listed) as { footers: { sha: string; text: string }[] }).footers;
		if (section !== undefined)
			for (const f of footers)
				if (!flat(section).includes(flat(f.text)))
					failures.push(
						`the Upgrading section does not say what ${f.sha.slice(0, 7)}'s Upgrading footer asks: ${f.text}`,
					);
	}
}

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
const pin = goOnly
	? `gives the binary's install lines for ${version} (version, download, checksums.txt) and no tarball`
	: `pins itos-${version}.tgz`;
const held = footers.length
	? `holds the ${footers.length} Upgrading footer${footers.length === 1 ? "" : "s"} of ${tag}..${end} to it`
	: `has no Upgrading footer of ${tag}..${end} to hold to it (itos commit footers lists none for the range)`;
console.log(
	`release notes self-test: ${notesName} ends with Upgrading, ${pin}, ${held}, and names ` +
		`every default changed between ${tag} and ${released ?? "this tree"} (${changed.join(", ") || "none"})`,
);
