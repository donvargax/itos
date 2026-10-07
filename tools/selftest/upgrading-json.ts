// A release's upgrading.json is its Upgrading section as data (T-091): what
// itos upgrade (slice-75) reads of each release between a project's pin and the
// new one, walking back by its previous. It proves the asset
// `tools/bin/release-notes -json` writes for a range, as
// tools/selftest/release-notes.ts proves the notes:
//
//   - it is one JSON object of exactly schema (1), version (the version
//     released), previous, breaking, upgrading, changes and config, each list
//     an array, never null;
//   - previous is the last release, X.Y.Z with no v: the newest vX.Y.Z tag the
//     range's end reaches other than the version's own, "" with none, which is
//     what the release workflow passes as -from (tools/bin/release-version's
//     last=);
//   - upgrading is every Upgrading footer
//     `itos commit footers Upgrading <last release> <end> --json` lists (those
//     saying none left out), in its order, each {commit, header, text} its sha,
//     its subject and its text, that text continued by the lines a wrapped
//     footer runs onto;
//   - changes is every Changes entry `itos commit footers Changes` lists, the
//     same way;
//   - breaking is every commit of the range marked breaking (a BREAKING-CHANGE:
//     or BREAKING CHANGE: footer in its message's last paragraph, or a ! in its
//     header), oldest first, each with its header and what it asks;
//   - config is every finding of
//     `go run ./tools/bin/schema-contract -json -release <last release>`, in
//     its order, each {key, change} its path and its change; none with no last
//     release.
//
//   node tools/selftest/upgrading-json.ts             the range since the last
//                                                     release up to HEAD, as the
//                                                     version tools/bin/release-version
//                                                     computes (the next patch when
//                                                     the range releases nothing, as
//                                                     tools/selftest/release-cut.ts
//                                                     stamps its snapshot)
//   node tools/selftest/upgrading-json.ts <version>   a released version: the range
//                                                     from the release before it to
//                                                     its tag
//
// It needs Go and the network (the schema contract downloads the last
// release's itos.schema.json). Exits 1 on any failure.
import { spawnSync } from "node:child_process";
import { join, resolve } from "node:path";
import { finish, outsideEnv, realGit } from "./scratch.ts";

const root = resolve(".");
const env = outsideEnv();
const problems: string[] = [];
const expect = (ok: boolean, problem: string) => ok || problems.push(problem);

function run(command: string, args: string[]) {
	const r = spawnSync(command, args, {
		cwd: root,
		env,
		encoding: "utf8",
		maxBuffer: 64 * 1024 * 1024,
	});
	return {
		status: r.status ?? 1,
		stdout: r.stdout ?? "",
		stderr: r.stderr ?? r.error?.message ?? "",
	};
}

// A command's stdout, or a problem naming it and undefined.
function sh(command: string, args: string[]): string | undefined {
	const r = run(command, args);
	if (r.status === 0) return r.stdout;
	problems.push(`${command} ${args.join(" ")} (exit ${r.status}): ${r.stderr}`);
	return undefined;
}

const newer = (a: string, b: string) => {
	const [x, y] = [a, b].map((v) => v.split(".").map(Number));
	for (let i = 0; i < 3; i++) if (x![i] !== y![i]) return (x![i] ?? 0) > (y![i] ?? 0);
	return false;
};

// The newest vX.Y.Z tag end reaches other than own, or "" with none.
function lastTag(end: string, own: string): string {
	const tags = (sh(realGit(), ["tag", "--merged", end, "--list", "v*"]) ?? "")
		.split("\n")
		.filter((t) => /^v\d+\.\d+\.\d+$/.test(t) && t !== own)
		.map((t) => t.slice(1))
		.sort((a, b) => (newer(a, b) ? -1 : 1));
	return tags.length ? `v${tags[0]}` : "";
}

// The version and range judged: a given version's, ending at its tag; else
// this tree's, as the release workflow would cut it.
function judged(): { version: string; end: string; last: string } {
	const given = process.argv[2]?.replace(/^v/, "");
	if (given) {
		const tag = `v${given}`;
		expect(
			run(realGit(), ["rev-parse", "--quiet", "--verify", `refs/tags/${tag}^{commit}`]).status ===
				0,
			`no tag ${tag} to judge`,
		);
		return { version: given, end: tag, last: lastTag(tag, tag) };
	}
	const r = run("go", ["run", "./tools/bin/release-version"]);
	const got = Object.fromEntries(
		r.stdout
			.split("\n")
			.filter((line) => line.includes("="))
			.map((line) => [line.slice(0, line.indexOf("=")), line.slice(line.indexOf("=") + 1)]),
	) as Record<string, string>;
	if (r.status !== 0) {
		problems.push(`go run ./tools/bin/release-version (exit ${r.status}): ${r.stderr}`);
		finish(problems, "upgrading.json", "");
	}
	if (got.next) return { version: got.next, end: "HEAD", last: got.last ?? "" };
	if (!got.last) return { version: "0.1.0", end: "HEAD", last: "" };
	const [major, minor, patch] = got.last.slice(1).split(".").map(Number);
	return { version: `${major}.${minor}.${patch! + 1}`, end: "HEAD", last: got.last };
}

interface Entry {
	commit: string;
	header: string;
	text: string;
}
interface Asset {
	schema: number;
	version: string;
	previous: string;
	breaking: Entry[];
	upgrading: Entry[];
	changes: Entry[];
	config: { key: string; change: string }[];
}

// Whitespace aside: a wrapped footer's lines are joined with newlines.
const flat = (text: string) => text.replace(/\s+/g, " ").trim();

const { version, end, last } = judged();
const written = sh("go", [
	"run",
	"./tools/bin/release-notes",
	"-json",
	"-version",
	version,
	"-to",
	end,
]);
let asset: Asset | undefined;
if (written !== undefined) {
	try {
		asset = JSON.parse(written) as Asset;
	} catch (error) {
		problems.push(`release-notes -json wrote no JSON: ${(error as Error).message}`);
	}
}

const keys = ["schema", "version", "previous", "breaking", "upgrading", "changes", "config"];
const lists = ["breaking", "upgrading", "changes", "config"] as const;

// A list of entries held to what a footer listing says, in its order.
function sameFooters(name: "upgrading" | "changes", footer: string, got: Entry[]) {
	if (!last) return expect(got.length === 0, `${name} lists ${got.length} with no last release`);
	const listed = sh(join(root, "tools", "bin", "itos"), [
		"commit",
		"footers",
		footer,
		last,
		end,
		"--json",
	]);
	if (listed === undefined) return;
	const want = (JSON.parse(listed) as { footers: { sha: string; subject: string; text: string }[] })
		.footers;
	expect(
		got.length === want.length,
		`${name} lists ${got.length}, itos commit footers ${footer} ${last} ${end} ${want.length}`,
	);
	want.forEach((f, i) => {
		const e = got[i];
		expect(
			e !== undefined &&
				e.commit === f.sha &&
				e.header === f.subject &&
				flat(e.text).startsWith(flat(f.text)),
			`${name}[${i}] is ${JSON.stringify(e)}, not ${f.sha.slice(0, 7)}'s ${footer} footer: ${f.text}`,
		);
	});
	for (const e of got)
		expect(e.text.trim().toLowerCase() !== "none", `${name} keeps ${e.commit.slice(0, 7)}'s none`);
}

// The range's breaking commits, oldest first: its marked footer or its ! header.
function breakingCommits(): { sha: string; header: string }[] {
	const range = last ? `${last}..${end}` : end;
	const log = sh(realGit(), ["log", "--reverse", "--format=%H%x1f%B%x1e", range]) ?? "";
	const found: { sha: string; header: string }[] = [];
	for (const record of log.split("\x1e")) {
		const [sha, message] = record.replace(/^\n+/, "").split("\x1f");
		if (!sha || message === undefined) continue;
		const text = message.trim();
		const header = text.split("\n")[0]!;
		const paragraphs = text.split("\n").slice(1).join("\n").trim().split("\n\n");
		const footers = text.includes("\n") ? paragraphs.at(-1)! : "";
		if (/^[a-zA-Z]+(\([^)]*\))?!: /.test(header) || /^BREAKING[- ]CHANGE: /m.test(footers))
			found.push({ sha, header });
	}
	return found;
}

if (asset !== undefined) {
	expect(
		JSON.stringify(Object.keys(asset)) === JSON.stringify(keys),
		`its keys are ${Object.keys(asset).join(", ")}, not ${keys.join(", ")}`,
	);
	expect(asset.schema === 1, `its schema is ${asset.schema}, not 1`);
	expect(asset.version === version, `its version is ${asset.version}, not ${version}`);
	expect(
		asset.previous === last.replace(/^v/, ""),
		`its previous is "${asset.previous}", not the last release "${last.replace(/^v/, "")}"`,
	);
	for (const list of lists) expect(Array.isArray(asset[list]), `its ${list} is not a list`);
	if (lists.every((list) => Array.isArray(asset![list]))) {
		sameFooters("upgrading", "Upgrading", asset.upgrading);
		sameFooters("changes", "Changes", asset.changes);
		const want = breakingCommits();
		expect(
			JSON.stringify(asset.breaking.map((b) => [b.commit, b.header])) ===
				JSON.stringify(want.map((b) => [b.sha, b.header])),
			`breaking lists ${asset.breaking.map((b) => b.commit.slice(0, 7)).join(", ") || "none"}, ` +
				`the range's breaking commits are ${want.map((b) => b.sha.slice(0, 7)).join(", ") || "none"}`,
		);
		for (const b of asset.breaking)
			expect(b.text.trim() !== "", `breaking ${b.commit.slice(0, 7)} says nothing it asks`);
		if (last) {
			const found = sh("go", ["run", "./tools/bin/schema-contract", "-json", "-release", last]);
			if (found !== undefined) {
				const findings = (JSON.parse(found) as { findings: { path: string; change: string }[] })
					.findings;
				expect(
					JSON.stringify(asset.config) ===
						JSON.stringify(findings.map((f) => ({ key: f.path, change: f.change }))),
					`config is ${JSON.stringify(asset.config)}, schema-contract -release ${last} names ` +
						JSON.stringify(findings.map((f) => f.path)),
				);
			}
		} else
			expect(asset.config.length === 0, `config lists ${asset.config.length} with no last release`);
	}
}

finish(
	problems,
	"upgrading.json",
	asset === undefined
		? ""
		: `upgrading.json self-test: the asset for ${last || "the first commit"}..${end} as ${version} ` +
				`has previous "${asset.previous}", ${asset.breaking.length} breaking, ` +
				`${asset.upgrading.length} Upgrading footer(s), ${asset.changes.length} Changes entr(ies) ` +
				`and ${asset.config.length} config change(s), each as its source lists it`,
);
