// The built-in header lint agrees with commitlint (T-063): every message of
// tools/selftest/header-agreement.json, the fixture header-agreement-record.ts
// wrote while commitlint was installed, goes through the Go binary
// (tools/bin/itos) with commits.header_lint.use: builtin, in both readings,
// and gets commitlint's verdict and commitlint's report lines:
//
//   - the hook's: `itos hook commit-msg <file>`, the message file read as
//     commitlint --edit reads it (git's comment lines and scissors out);
//   - stdin's: `itos commit check-message -`, the message as given, as
//     check-message and verify read it and commitlint reads stdin.
//
//   node tools/selftest/header-agreement.ts
//
// The scratch repository's config is the header lint alone: no
// commits.types, so type-enum's list is config-conventional's own, as
// commitlint's was, and no footers, paths or ledger, so nothing else
// reports. Verdicts must always agree. The report lines must too, but for
// the one known gap (p3-header-case-decomposition): subject-case may name
// other cases than commitlint for a subject with a precomposed letter
// outside Latin-1 and Latin Extended-A, which commitlint decomposes and Go's
// standard library cannot; there the subject-case lines are compared for
// being there, not for their words. Where commitlint refused a message as no
// input at all (a blank one on stdin: exit 1, "[input] is required", no
// report), it has no words, and the verdict alone is compared. Needs no commitlint and no Node
// package: only the fixture and the binary. Fails on any disagreement,
// naming the message, and when it compared nothing.
import { spawnSync } from "node:child_process";
import { existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { availableParallelism, tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { repoProgram } from "../bin/repo-program.ts";
import { outsideEnv, realGit, spawnOutput } from "./scratch.ts";

const ROOT = resolve(import.meta.dirname, "../..");
const FIXTURE = join(ROOT, "tools/selftest/header-agreement.json");
const ITOS = join(ROOT, "tools/bin/itos");
const REPORT = /^(✖|⚠)\s+.*\[[\w-]+\]$/u;

interface Reading {
	ok: boolean;
	report: string[];
}
interface Message {
	source: string;
	text: string;
	edit: Reading;
	stdin: Reading;
}
type Name = "edit" | "stdin";

if (!existsSync(FIXTURE)) {
	console.error(
		`FAIL ${FIXTURE} is missing: record it with node tools/selftest/header-agreement-record.ts while commitlint is installed`,
	);
	process.exit(1);
}
const { messages } = JSON.parse(readFileSync(FIXTURE, "utf8")) as { messages: Message[] };

const dir = mkdtempSync(join(tmpdir(), "header-agreement-"));
const env = {
	...outsideEnv(),
	GIT_CONFIG_GLOBAL: "/dev/null",
	GIT_CONFIG_NOSYSTEM: "1",
	GIT_AUTHOR_NAME: "header-agreement",
	GIT_AUTHOR_EMAIL: "selftest@localhost",
	GIT_COMMITTER_NAME: "header-agreement",
	GIT_COMMITTER_EMAIL: "selftest@localhost",
};

function setUp() {
	writeFileSync(join(dir, "itos.yaml"), "version: 1\ncommits:\n  header_lint:\n    use: builtin\n");
	for (const args of [
		["init", "-q", "-b", "main"],
		["add", "itos.yaml"],
		["commit", "-q", "--no-verify", "-m", "chore: start"],
	]) {
		const run = spawnSync(realGit(), args, { cwd: dir, env, encoding: "utf8" });
		if (run.status !== 0) throw new Error(`git ${args.join(" ")}: ${run.stderr}`);
	}
}

// The binary on one message in one reading: its verdict and report lines.
async function itos(text: string, reading: Name, i: number): Promise<Reading> {
	const file = join(dir, ".git", `MSG-${i}`);
	if (reading === "edit") writeFileSync(file, text);
	const args = reading === "edit" ? ["hook", "commit-msg", file] : ["commit", "check-message", "-"];
	const [program, argv] = repoProgram(ITOS, args);
	const run = await spawnOutput(program, argv, {
		cwd: dir,
		env,
		input: reading === "stdin" ? text : "",
	});
	if (run.code !== 0 && run.code !== 1)
		throw new Error(`itos ${args.join(" ")} exited ${run.code}:\n${run.output}`);
	return { ok: run.code === 0, report: run.output.split("\n").filter((l) => REPORT.test(l)) };
}

// The known gap: a subject with a precomposed letter beyond Latin Extended-A
// (U+017F), one that decomposes.
const decomposes = (text: string) => {
	const header = text.split(/\r?\n/u)[0] ?? "";
	return [...header].some((c) => c.codePointAt(0)! > 0x17f && c.normalize("NFD") !== c);
};
const caseLine = (l: string) => l.endsWith("[subject-case]");

// What differs between commitlint's reading and the binary's, or "".
function differs(m: Message, reading: Name, got: Reading): string {
	const want = m[reading];
	if (want.ok !== got.ok)
		return `commitlint ${want.ok ? "passes" : "rejects"} it, the built-in lint ${got.ok ? "passes" : "rejects"} it`;
	// commitlint refused the input, a blank message on stdin, with no report
	// to compare: the built-in lint names type-empty and subject-empty.
	if (!want.ok && want.report.length === 0) {
		refused.push(`${m.source} (${reading})`);
		return "";
	}
	let [a, b] = [want.report, got.report];
	if (decomposes(m.text) && a.some(caseLine) === b.some(caseLine)) {
		const gap = a.filter(caseLine).join() !== b.filter(caseLine).join();
		if (gap) known.push(`${m.source} (${reading})`);
		[a, b] = [a.filter((l) => !caseLine(l)), b.filter((l) => !caseLine(l))];
	}
	return a.join("\n") === b.join("\n") ? "" : "the report lines differ";
}

const problems: string[] = [];
const known: string[] = [];
const refused: string[] = [];
let compared = 0;

const lines = (report: string[]) =>
	report.map((l) => `\n    ${l}`).join("") || " (no report lines)";
const shown = (text: string) => JSON.stringify(text.length > 200 ? `${text.slice(0, 200)}…` : text);

// One message in both readings, each disagreement a problem naming it.
async function judge(m: Message, i: number) {
	for (const reading of ["edit", "stdin"] as const) {
		const got = await itos(m.text, reading, i);
		compared++;
		const why = differs(m, reading, got);
		if (!why) continue;
		const where = reading === "edit" ? "the hook's reading" : "stdin's";
		problems.push(
			`${m.source}, ${where}: ${why}\n  message: ${shown(m.text)}\n` +
				`  commitlint:${lines(m[reading].report)}\n  built-in:${lines(got.report)}`,
		);
	}
}

async function compare() {
	let next = 0;
	const worker = async () => {
		for (let i = next++; i < messages.length; i = next++) await judge(messages[i]!, i);
	};
	await Promise.all(Array.from({ length: availableParallelism() }, worker));
}

try {
	setUp();
	await compare();
} finally {
	rmSync(dir, { recursive: true, force: true });
}

for (const problem of problems) console.error(`FAIL ${problem}`);
const sources = (prefix: string) => messages.filter((m) => m.source.startsWith(prefix)).length;
const summary =
	`${messages.length} messages (${sources("history")} of this history, ${sources("corpus")} of the corpus, ` +
	`${sources("aimed")} aimed at the rules), ${compared} readings`;
if (compared === 0) {
	console.error("FAIL the fixture holds no message, so nothing was compared");
	process.exit(1);
}
if (problems.length) {
	console.error(`\n${problems.length} disagreement(s) with commitlint over ${summary}`);
	process.exit(1);
}
console.log(
	`The built-in header lint gives commitlint's verdict and report on ${summary}` +
		(known.length ? `; subject-case's known gap words ${known.length} of them differently` : "") +
		(refused.length
			? `; commitlint refused ${refused.length} as no input, with no report to compare`
			: ""),
);
