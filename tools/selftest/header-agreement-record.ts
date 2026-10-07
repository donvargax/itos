// Records commitlint's verdicts for tools/selftest/header-agreement.ts (T-063),
// which holds itos's built-in header lint to them without commitlint. Run once,
// by hand, while commitlint is installed; not a check:
//
//   node tools/selftest/header-agreement-record.ts
//
// commitlint left this repository when it switched to the built-in lint, so
// rerunning this needs it back first, in a scratch checkout of your own
// (`vp add -D @commitlint/cli @commitlint/config-conventional`), and the
// fixture it writes is the one header-agreement.ts reads,
// tools/selftest/header-agreement.json.
//
// The messages are every commit message of this repository after
// commits.since (as verify reads one: `git log -1 --format=%B`), every
// message of the conformance corpus (what check-message and the commit-msg
// hook are given, and its repositories' commits), and messages aimed at each
// of config-conventional's rules and at the parser's edge cases (AIMED
// below). Each goes through commitlint with config-conventional alone, as
// this repository delegated to it, in both readings: `--edit` on a message
// file, the commit-msg hook's, and the message on stdin, check-message's and
// verify's. The fixture keeps, per message and reading, commitlint's verdict
// (its exit code, 0 or not) and its report lines (`✖` or `⚠`, the sentence,
// the rule in brackets), the lines the built-in lint prints the same way.
import { spawnSync } from "node:child_process";
import { existsSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { availableParallelism, tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { parse } from "yaml";
import { outsideEnv, realGit, spawnOutput } from "./scratch.ts";

const ROOT = resolve(import.meta.dirname, "../..");
const FIXTURE = join(ROOT, "tools/selftest/header-agreement.json");
const COMMITLINT = join(ROOT, "node_modules/.bin/commitlint");
const CORPUS = join(ROOT, "tools/itos/conformance");

// A report line, as commitlint prints a problem.
const REPORT = /^(✖|⚠)\s+.*\[[\w-]+\]$/u;

const long = (n: number, c = "a") => c.repeat(n);
const url = `https://example.com/${long(100, "u")}`;

// Messages aimed at each rule of config-conventional, at the messages
// is-ignored passes and at the parser's edge cases: each a name and a text.
const AIMED: [string, string][] = [
	// type-empty, type-enum, type-case
	["no type", "update things\n"],
	["a colon and no type", ": no type\n"],
	["a scope and no type", "(scope): no type\n"],
	["an unknown type", "feet: x\n"],
	["a type with a digit", "feat2: x\n"],
	["a type in sentence case", "Fix: x\n"],
	["a type in upper case", "FEAT: x\n"],
	["a type in mixed case", "fIX: x\n"],
	["a type with a hyphen", "fix-up: x\n"],
	["a type with an underscore", "fix_up: x\n"],
	["a type with a non-ASCII letter", "fíx: x\n"],
	// subject-empty
	["no subject", "feat:\n"],
	["a space and no subject", "feat: \n"],
	["a scope and no subject", "feat(scope):\n"],
	["two spaces and no subject", "feat:  \n"],
	["no space after the colon", "feat:x\n"],
	["a space before the colon", "feat :x\n"],
	["a space between the type and the scope", "feat (scope): x\n"],
	// the scope
	["an empty scope", "feat(): x\n"],
	["two scopes", "feat(a)(b): x\n"],
	["a scope with a space", "feat(a b): x\n"],
	["a scope with a colon", "feat(sc:ope): x\n"],
	["a scope in upper case", "feat(API): add x\n"],
	["a scope with a slash and a comma", "feat(a/b,c): add x\n"],
	["an unclosed scope", "feat(scope: x\n"],
	// breaking changes
	["a breaking mark", "feat!: drop x\n"],
	["a breaking mark after a scope", "feat(api)!: drop x\n"],
	["a breaking mark and no subject", "feat!:\n"],
	["a breaking-change footer", "feat: drop x\n\nBREAKING CHANGE: y goes\n"],
	["a breaking-change footer with a hyphen", "feat: drop x\n\nBREAKING-CHANGE: y goes\n"],
	[
		"a breaking-change note in lower case, continued, then a footer",
		"feat: drop x\n\nbreaking change: y goes\nand z\nRefs: #1\n",
	],
	["a breaking-change note in a list", "feat: drop x\n\n* BREAKING CHANGE: y goes\n"],
	["a breaking-change note after the body", "feat: drop x\n\nThe body.\n\nBREAKING CHANGE: y\n"],
	// subject-case
	["a subject in sentence case", "feat: Add x\n"],
	["a subject in upper case", "feat: ADD X\n"],
	["a subject in start case", "feat: Add The Thing\n"],
	["a subject in pascal case", "feat: AddThing\n"],
	["a subject in lower case with a proper name", "feat: add The Thing\n"],
	["a subject starting with a quoted name", "feat: `Foo` bar\n"],
	["a subject starting with a double-quoted name", 'feat: "Quoted" thing\n'],
	["a subject starting with a single-quoted name", "feat: 'Single' thing\n"],
	["a subject all quoted", "feat: `Foo Bar`\n"],
	["a subject starting with a digit", "feat: 1st thing\n"],
	["a subject starting with an underscore", "feat: _private thing\n"],
	["a subject in camel case", "feat: iPhone support\n"],
	["a subject with a hyphenated capital", "feat: Xyz-abc\n"],
	["a subject of one capital letter", "feat: X\n"],
	["a subject of one capital word", "feat: README\n"],
	["a subject starting with a capital word", "feat: README edits\n"],
	["a subject starting with a Latin-1 capital", "feat: Éclair\n"],
	["a subject in Latin-1 start case", "feat: Éclair Über\n"],
	["a subject starting with a Latin Extended-A capital", "feat: Ārvalds\n"],
	["a subject in Latin Extended-A start case", "feat: Ārvalds Ōtaki\n"],
	["a subject starting with a Greek capital", "feat: Ωmega\n"],
	["a subject in Greek start case", "feat: Ἀθήνα Πόλη\n"],
	["a subject in Greek upper case", "feat: ΑΘΗΝΑ\n"],
	["a subject starting with a title-case digraph", "feat: ǅemal\n"],
	["a subject starting with a capital dotted I", "feat: İstanbul\n"],
	["a subject starting with sharp s", "feat: ßtraße\n"],
	["a subject starting with a ligature", "feat: ﬀ ligature\n"],
	["a subject starting with an astral capital", "feat: 𝐀bc\n"],
	["a subject starting with Cyrillic", "feat: Привет мир\n"],
	["a subject starting with a CJK character", "feat: 漢字\n"],
	["a subject starting with an emoji", "feat: 🎉 party\n"],
	["a subject starting with a combining mark", "feat: ͅx\n"],
	["a subject starting with a space and a capital", "feat:  Add x\n"],
	["a subject in sentence case ending with a full stop", "feat: Add x.\n"],
	// subject-full-stop
	["a subject ending with a full stop", "feat: x.\n"],
	["a subject ending with an ellipsis", "feat: x...\n"],
	["a subject ending with two full stops", "feat: x..\n"],
	["a header ending with a colon then a full stop", "feat:.\n"],
	["a subject ending with an ideographic full stop", "feat: x。\n"],
	["a subject ending with a question mark", "feat: x?\n"],
	// header-max-length
	["a header of 100 characters", `feat: ${long(94)}\n`],
	["a header of 101 characters", `feat: ${long(95)}\n`],
	["a header of 100 UTF-16 units with an astral character", `feat: ${long(92)}𝐀\n`],
	["a header of 101 UTF-16 units with an astral character", `feat: ${long(93)}𝐀\n`],
	["a header of 100 accented characters", `feat: ${long(94, "é")}\n`],
	["a long header with a URL", `feat: ${url}\n`],
	// header-trim
	["a header with a leading space", " feat: x\n"],
	["a header with a trailing space", "feat: x \n"],
	["a header with spaces at both ends", " feat: x \n"],
	["a header with a leading tab", "\tfeat: x\n"],
	["a header with a trailing no-break space", "feat: x \n"],
	["a header with a leading ideographic space", "　feat: x\n"],
	["a header with a trailing zero-width space", "feat: x​\n"],
	["a header with a trailing byte-order mark", "feat: x﻿\n"],
	// body-leading-blank, body-max-line-length
	["a body without a blank line", "feat: x\nThe body.\n"],
	["a body after a blank line", "feat: x\n\nThe body.\n"],
	["a body after two blank lines", "feat: x\n\n\nThe body.\n"],
	["a body line of 100 characters", `feat: x\n\n${long(100)}\n`],
	["a body line of 101 characters", `feat: x\n\n${long(101)}\n`],
	["a body line of 101 characters with a URL", `feat: x\n\nsee ${url}\n`],
	["a body line of 101 with an astral character", `feat: x\n\n${long(99)}𝐀\n`],
	["a body line too long without a blank line", `feat: x\n${long(101)}\n`],
	// footer-leading-blank, footer-max-line-length
	["a footer without a blank line", "feat: x\nTask: T-001\n"],
	["a footer after a body without a blank line", "feat: x\n\nThe body.\nTask: T-001\n"],
	["a footer after a blank line", "feat: x\n\nThe body.\n\nTask: T-001\n"],
	["two footers", "feat: x\n\nTask: T-001\nScenarios: @ID-A-01\n"],
	["an issue footer", "fix: x\n\nCloses #12\n"],
	["a hyphenated footer", "fix: x\n\nReviewed-by: Some One <one@example.com>\n"],
	["a footer line of 101 characters", `feat: x\n\nTask: ${long(95)}\n`],
	["a footer line of 101 characters with a URL", `feat: x\n\nRefs: ${url}\n`],
	["a footer line too long after a body", `feat: x\n\nThe body.\n\nRefs: ${long(101)}\n`],
	["a footer continued over lines", "feat: x\n\nRefs: one\n  two\n  three\n"],
	["a footer then a body line", "feat: x\n\nRefs: one\n\nMore body.\n"],
	["a footer whose line repeats in the body", "feat: x\n\nTask: T-001\nbody\n\nTask: T-001\n"],
	["a footer token without a value", "feat: x\n\nTask:\n"],
	["a footer token with a tab", "feat: x\n\nTask:\tT-001\n"],
	// the messages is-ignored passes
	["a merge of a branch", "Merge branch 'main' into topic\n"],
	["a merge of a pull request", "Merge pull request #1 from a/b\n\nFix things.\n"],
	["a merge of a tag", "Merge tag 'v1.0.0'\n"],
	["a merge of a remote-tracking branch", "Merge remote-tracking branch 'origin/main'\n"],
	["a merge line in the body", "Squashed: x\n\nMerge branch 'a' into b\n"],
	["a host's merge", "Merged PR 12: x\n"],
	["a host's merge into a branch", "Merged x in main\n"],
	["an automatic merge", "Automatic merge from x\n"],
	["an auto-merge", "Auto-merged x into y\n"],
	["a revert", 'Revert "feat: x"\n\nThis reverts commit abc.\n'],
	["a revert in lower case", 'revert "feat: x"\n'],
	["a revert type", "revert: feat: x\n"],
	["a reapply", 'Reapply "feat: x"\n'],
	["a fixup", "fixup! feat: x\n"],
	["a squash", "squash! Add X.\n"],
	["an amend", "amend! feat: x\n"],
	["a version", "v1.2.3\n"],
	["a version without a v", "1.2.3\n"],
	["a pre-release version", "1.2.3-rc.1\n"],
	["a chore version", "chore: 1.2.3\n"],
	["a chore release version with skip ci", "chore(release): v1.2.3 [skip ci]\n"],
	["a version with a skip mark in parentheses", "v1.2.3 (ci skip)\n"],
	["not quite a version", "v1.2\n"],
	["a version with a leading zero", "01.2.3\n"],
	["an initial commit", "Initial commit\n"],
	// comment lines and git's scissors (the --edit reading leaves them out)
	["a comment line after the header", "feat: x\n# a comment\n"],
	["a comment line before the header", "# a comment\nfeat: x\n"],
	["a comment line with a bad header after it", "# a comment\nFeat: X.\n"],
	["a bad header with a comment after it", "Feat: x\n# a comment\n"],
	["a header that is a comment", "#feat: x\n"],
	["only comment lines", "# a comment\n# another\n"],
	[
		"git's template under the scissors",
		"feat: x\n\nThe body.\n# ------------------------ >8 ------------------------\nFoo Bar.\n",
	],
	["a body with a hash that is not a comment", "feat: x\n\n#123 is fixed\n"],
	["a gpg line", "feat: x\ngpg: Signature made\n"],
	// line breaks
	["CRLF line breaks", "feat: x\r\n\r\nThe body.\r\n"],
	["CRLF with a bad header", "Feat: x\r\n"],
	["CRLF with a footer", "feat: x\r\n\r\nTask: T-001\r\n"],
	["a line separator in the header", "feat: x y\n"],
	["a carriage return in the header", "feat: x\ry\n"],
	["no final line break", "feat: x"],
	["two final line breaks", "feat: x\n\n"],
	["leading blank lines", "\n\nfeat: x\n"],
	// blank messages
	["an empty message", ""],
	["a line break alone", "\n"],
	["spaces and line breaks", "  \n\n"],
];

interface Reading {
	ok: boolean;
	report: string[];
}
interface Message {
	source: string;
	text: string;
	edit?: Reading;
	stdin?: Reading;
}

const git = (...args: string[]) => {
	const run = spawnSync(realGit(), args, { cwd: ROOT, encoding: "utf8", maxBuffer: 1 << 26 });
	if (run.status !== 0) throw new Error(`git ${args.join(" ")}: ${run.stderr}`);
	return run.stdout;
};

// Every commit after commits.since, oldest first, as verify reads it.
function history(): Message[] {
	const since = /^\s*since:\s*"([0-9a-f]{40})"/m.exec(
		readFileSync(join(ROOT, "itos.yaml"), "utf8"),
	);
	if (!since) throw new Error("itos.yaml names no commits.since");
	const shas = git("rev-list", "--reverse", `${since[1]}..HEAD`).split("\n").filter(Boolean);
	return shas.map((sha) => ({
		source: `history ${sha}`,
		text: git("log", "-1", "--format=%B", sha),
	}));
}

type Files = Record<string, unknown>;
interface Case {
	name?: string;
	argv?: unknown[];
	stdin?: string;
	files?: Files;
	git?: { commit?: string }[];
}

const fileText = (value: unknown) =>
	typeof value === "string"
		? value
		: value && typeof value === "object" && "text" in value
			? String((value as { text: unknown }).text)
			: undefined;

// A case's messages: what check-message or the commit-msg hook is given (its
// stdin, or the files its argv names), and its repository's commits.
function caseMessages(file: string, c: Case, defaults: Files): Message[] {
	const found: Message[] = [];
	const argv = (c.argv ?? []).map(String);
	const judged = argv.includes("check-message") || argv.includes("commit-msg");
	const source = `corpus ${file}: ${c.name ?? "(top)"}`;
	if (judged && typeof c.stdin === "string") found.push({ source, text: c.stdin });
	const files = { ...defaults, ...c.files };
	for (const arg of judged ? argv : []) {
		const text = fileText(files[arg]);
		if (text !== undefined) found.push({ source: `${source} (${arg})`, text });
	}
	for (const step of c.git ?? [])
		if (typeof step.commit === "string")
			found.push({ source: `${source} (commit)`, text: step.commit });
	return found;
}

function corpus(): Message[] {
	const found: Message[] = [];
	for (const file of readdirSync(CORPUS)
		.filter((f) => f.endsWith(".yaml"))
		.sort()) {
		const doc = parse(readFileSync(join(CORPUS, file), "utf8")) as Case & { cases?: Case[] };
		found.push(...caseMessages(file, { git: doc.git }, {}));
		for (const c of doc.cases ?? []) found.push(...caseMessages(file, c, doc.files ?? {}));
	}
	return found;
}

// Each text once, under the first source that has it.
function unique(messages: Message[]): Message[] {
	const seen = new Set<string>();
	return messages.filter((m) => !seen.has(m.text) && (seen.add(m.text), true));
}

const work = mkdtempSync(join(tmpdir(), "header-agreement-record-"));
const config = join(work, "commitlint.config.mjs");
writeFileSync(config, 'export default { extends: ["@commitlint/config-conventional"] };\n');

// commitlint on one message in one reading, from the checkout so the extends
// resolve, outside any hook's environment.
async function commitlint(text: string, edit: string | undefined): Promise<Reading> {
	const args = ["--cwd", ROOT, "--config", config, ...(edit ? ["--edit", edit] : [])];
	const run = await spawnOutput(COMMITLINT, args, {
		cwd: ROOT,
		env: outsideEnv(),
		input: edit ? "" : text,
	});
	return { ok: run.code === 0, report: run.output.split("\n").filter((l) => REPORT.test(l)) };
}

async function record(messages: Message[]) {
	let next = 0;
	const worker = async () => {
		for (let i = next++; i < messages.length; i = next++) {
			const m = messages[i]!;
			const file = join(work, `msg-${i}`);
			writeFileSync(file, m.text);
			m.edit = await commitlint(m.text, file);
			m.stdin = await commitlint(m.text, undefined);
		}
	};
	await Promise.all(Array.from({ length: availableParallelism() }, worker));
}

if (!existsSync(COMMITLINT)) {
	console.error(
		`header-agreement-record: ${COMMITLINT} is missing; install @commitlint/cli and @commitlint/config-conventional first`,
	);
	process.exit(2);
}
try {
	const messages = unique([
		...history(),
		...corpus(),
		...AIMED.map(([name, text]) => ({ source: `aimed: ${name}`, text })),
	]);
	await record(messages);
	const version = spawnSync(COMMITLINT, ["--version"], { encoding: "utf8" }).stdout.trim();
	writeFileSync(
		FIXTURE,
		`${JSON.stringify(
			{
				recorded: {
					commitlint: version,
					config: "@commitlint/config-conventional",
					head: git("rev-parse", "HEAD").trim(),
				},
				messages,
			},
			null,
			"\t",
		)}\n`,
	);
	const failing = (r: "edit" | "stdin") => messages.filter((m) => !m[r]!.ok).length;
	console.log(
		`recorded ${messages.length} messages in both readings: commitlint rejects ${failing("edit")} by --edit, ${failing("stdin")} on stdin`,
	);
} finally {
	rmSync(work, { recursive: true, force: true });
}
