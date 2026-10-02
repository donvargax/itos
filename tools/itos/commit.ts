// `itos commit check-message <file|->`: one message through the header lint
// and the footer rules, as the commit-msg hook reads it. The header lint is
// `commits.header_lint`'s delegate (commitlint, say); its text output is
// printed as it comes. Under `--json` each line it reports becomes a problem
// with the delegate's rule id and a fix. The footer rules (`commits.footers`,
// footers.ts) are itos's and run here always, after the delegate, whatever it
// is: a delegate judges the header and body, and one without them never skips
// them. Both report before the exit, so a header problem does not hide a
// footer one.
import { readFileSync } from "node:fs";
import { config, ledgerLayout } from "./config.ts";
import { checkFooter, footers } from "./footers.ts";
import { emit, type Output, problem, type Problem, TEXT } from "./problem.ts";
import { inShell } from "./shell.ts";
import { shellWord } from "./tests.ts";

// What resolves each rule of @commitlint/config-conventional an agent meets.
const FIXES: Record<string, string> = {
	"type-enum": "start the header with one of the commit types, as in `docs: …`",
	"type-case": "write the type in lower case",
	"type-empty": "start the header with a type and a colon, as in `fix: …`",
	"subject-empty": "write a subject after the type's colon",
	"subject-case": "start the subject with a lower-case letter",
	"subject-full-stop": "remove the full stop at the end of the subject",
	"header-max-length": "shorten the header to 100 characters or fewer",
	"header-trim": "remove the spaces around the header",
	"body-leading-blank": "leave a blank line between the header and the body",
	"footer-leading-blank": "leave a blank line before the footers",
	"body-max-line-length": "wrap the body at 100 characters",
	"footer-max-line-length":
		"split the footer over several lines of 100 characters or fewer; a footer may repeat",
};

// A footer rule's fix, by what its sentence says.
function footerFix(rule: string, message: string): string | undefined {
	const f = footers().find((footer) => `${footer.key.toLowerCase()}-footer` === rule);
	if (!f) return undefined;
	const source = f.source === "ledger" ? `${ledgerLayout().dir}/` : `the ${f.source.tests} files`;
	if (/ need a /.test(message))
		return `add a line \`${f.key}: <id>\` after a blank line at the end, naming what the commit belongs to`;
	if (message.startsWith("unknown "))
		return `name an id ${source} has, or add it first in a docs commit`;
	return `make each id the ${f.key}: footer names live in the same commit`;
}

const fixFor = (rule: string, message: string) => FIXES[rule] ?? footerFix(rule, message);

// The delegate's report, one problem per `✖` (error) or `⚠` (warning) line
// that ends in `[rule]`; anything else it printed becomes one `header-lint`
// problem, so a failure never reads as clean.
function parseReport(text: string, failed: boolean): (Problem & { level: string })[] {
	const found: (Problem & { level: string })[] = [];
	for (const line of text.split("\n")) {
		const m = /^(✖|⚠)\s+(.*\S)\s+\[([\w-]+)\]$/.exec(line.trim());
		if (!m) continue;
		const [, mark, message = "", rule = ""] = m;
		const fix = fixFor(rule, message);
		found.push({ ...problem(rule, message, fix), level: mark === "✖" ? "error" : "warning" });
	}
	if (failed && !found.some((p) => p.level === "error"))
		found.push({ ...problem("header-lint", text.trim()), level: "error" });
	return found;
}

// The footer rules, which run beside any header lint delegate and alone
// without one.
function footerProblems(message: string): Problem[] {
	const type = /^(\w+)/.exec(message)?.[1] ?? "";
	return footers().flatMap((f) => {
		const [ok, why = ""] = checkFooter(f.key, type, message);
		const rule = `${f.key.toLowerCase()}-footer`;
		return ok ? [] : [problem(rule, why, fixFor(rule, why))];
	});
}

// Footer problems on stderr, printed as commitlint prints a problem.
const printFooters = (found: Problem[]) => {
	for (const p of found) console.error(`✖   ${p.message} [${p.rule}]`);
};

// With no delegate: the footer rules alone.
function footersOnly(message: string, out: Output): number {
	const found = footerProblems(message);
	if (out.json) emit({ ok: found.length === 0, problems: found });
	else printFooters(found);
	return found.length ? 1 : 0;
}

// The delegate, its report printed as it comes, or read into problems, then
// the footer rules; under `--json` one list, the delegate's problems first.
function delegated(delegate: string, message: string, out: Output): number {
	const stream = out.json ? "pipe" : "inherit";
	const run = inShell(delegate, {
		input: message,
		encoding: "utf8",
		stdio: ["pipe", stream, stream],
	});
	const linted = run.status === 0;
	const found = footerProblems(message);
	const ok = linted && found.length === 0;
	if (out.json)
		emit({
			ok,
			problems: [
				...parseReport(`${run.stdout}\n${run.stderr}`, !linted),
				...found.map((p) => ({ ...p, level: "error" })),
			],
		});
	else printFooters(found);
	return ok ? 0 : 1;
}

// `commit check-message <file|-> [--at <sha>]`: 0 when the message passes, 1
// when not. `--at` reads the footers' IDs at that commit.
export function checkMessage(source: string, at: string | undefined, out: Output = TEXT): number {
	const message = readFileSync(source === "-" ? 0 : source, "utf8");
	if (at) process.env.ITOS_AT = at;
	const delegate = config().commits?.header_lint?.stdin;
	return delegate ? delegated(delegate, message, out) : footersOnly(message, out);
}

// One message through the header lint and the footer rules as they were at a
// commit, its report on stderr under `--json`: the re-check of a
// pushed range (`itos verify`). The delegate reads the commit from ITOS_AT, as
// the footer rules do here, after it.
export function messageHoldsAt(message: string, at: string, out: Output = TEXT): boolean {
	const delegate = config().commits?.header_lint?.stdin;
	const linted =
		!delegate ||
		inShell(delegate, {
			input: message,
			env: { ...process.env, ITOS_AT: at },
			stdio: ["pipe", out.json ? 2 : "inherit", "inherit"],
		}).status === 0;
	const saved = process.env.ITOS_AT;
	process.env.ITOS_AT = at;
	try {
		const found = footerProblems(message);
		printFooters(found);
		return linted && found.length === 0;
	} finally {
		if (saved === undefined) delete process.env.ITOS_AT;
		else process.env.ITOS_AT = saved;
	}
}

// The commit-msg hook's second half (`itos hook commit-msg`): the
// message file through `commits.header_lint.hook` ({file} filled in), whose
// report is printed as it comes, then the footer rules here, always. A failing
// delegate's exit code is the hook's; footer problems alone exit 1.
export function lintMessageFile(file: string): number {
	const message = readFileSync(file, "utf8");
	const delegate = config().commits?.header_lint?.hook;
	if (!delegate) return footersOnly(message, TEXT);
	const command = delegate.replaceAll("{file}", shellWord(file));
	const status = inShell(command, { stdio: "inherit" }).status ?? 1;
	const found = footerProblems(message);
	printFooters(found);
	return status || (found.length ? 1 : 0);
}
