// The commit-msg hook's check of itos's own data: the config, the ledger, the
// work registry and the smoke sets. A project's pre-commit hook passes them
// as prose, so the hook validates them itself when a commit stages any of
// them: `itos config check`'s problems (the registry's own check among them),
// read from the staged tree, since that is what the commit will hold and the
// working tree may differ from it. A commit that stages none of them is not
// checked here.
import { basename, dirname, posix } from "node:path";
import { config, configPath, hasSection, ledgerLayout, type Loaded } from "./config.ts";
import { configFindings } from "./config-check.ts";
import { problem, type Problem } from "./problem.ts";
import { stagedFiles } from "./repo.ts";
import { readingFrom, treeSource } from "./source.ts";

const norm = (path: string) => posix.normalize(path);

// Whether a staged file is the config, a ledger file, the registry or a smoke
// set, as the staged config names them. A config that does not load and is not
// staged leaves the commit to the hook's other rules, which read it too.
function stagesData(staged: string[]): boolean {
	const files = new Set(staged.map(norm));
	if (files.has(norm(configPath()))) return true;
	let read: Loaded;
	try {
		read = config();
	} catch {
		return false;
	}
	const ledger = hasSection("ledger", read) ? ledgerLayout() : undefined;
	const isLedger = (file: string) =>
		ledger !== undefined && dirname(file) === norm(ledger.dir) && ledger.file.test(basename(file));
	const named = [
		read.work.registry,
		...Object.values(read.tests ?? {}).flatMap((k) => (k.smoke?.file ? [k.smoke.file] : [])),
	];
	return [...files].some(isLedger) || named.some((file) => files.has(norm(file)));
}

// The staged data's problems, each a sentence the rejection prints, and the
// rejection's first line: commits.reject_message as staged, if it loads.
export function stagedDataIssues(staged = stagedFiles()): { found: Problem[]; header: string } {
	return readingFrom(treeSource("index"), () => {
		if (!stagesData(staged)) return { found: [], header: "" };
		let found: Problem[];
		try {
			found = configFindings().found;
		} catch (error) {
			found = [problem("data-unreadable", (error as Error).message)];
		}
		let header = "Commit rejected:";
		try {
			header = config().commits.reject_message;
		} catch {
			// The staged config is what is wrong: the default says so.
		}
		return { found, header };
	});
}

// The hook's first rule: 0 when the staged data is sound or none is staged,
// else the rejection, its problems one a line, and 1.
export function hook(): number {
	const { found, header } = stagedDataIssues();
	if (!found.length) return 0;
	console.error(header);
	for (const p of found) console.error(`  - ${p.message}`);
	return 1;
}
