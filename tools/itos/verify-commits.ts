// CI: re-check the commits in a pushed range with the commit-msg hook's rules,
// so a commit made with the hooks bypassed still fails the build: each commit's
// message (against the feature files and the ledger of that commit) and
// paths, then each kind's range check over the range (the scenario-moves
// rule). `itos verify <from> <to>` runs it; `from` may be empty or all zeros
// (a new branch): every commit up to `to`.
import { spawnSync } from "node:child_process";
import { messageHoldsAt } from "./commit.ts";
import { checkPaths } from "./commit-scope.ts";
import { config } from "./config.ts";
import { emit, type Output, TEXT } from "./problem.ts";
import { git } from "./repo.ts";
import { shellWord } from "./tests.ts";

// A range check, its output where the logs go: stderr under `--json`.
const run = (command: string, out: Output) =>
	spawnSync("sh", ["-c", command], { stdio: ["ignore", out.json ? 2 : "inherit", "inherit"] })
		.status === 0;

// One commit: its message at that commit, then its paths.
function verifyCommit(sha: string, out: Output): { sha: string; header: string; ok: boolean } {
	const message = git("log", "-1", "--format=%B", sha);
	const header = message.split("\n")[0]!;
	const type = /^(\w+)/.exec(message)?.[1] ?? "";
	const files = git("diff-tree", "--no-commit-id", "--name-only", "-r", "--root", sha)
		.split("\n")
		.filter(Boolean);
	// The message against the IDs as they were at that commit, for each footer
	// read `at: commit`: a later commit may have set a scenario back to @wip,
	// as a revert does, or taken a task out of the ledger. The header lint is
	// the config's delegate (commitlint, say), else the footer rules alone.
	const ok = messageHoldsAt(message, sha, out) && checkPaths(type, files) === 0;
	if (!ok) console.error(`  ^ ${sha.slice(0, 7)} ${header}`);
	return { sha, header, ok };
}

// Each kind's range checks (`tests.<kind>.range_checks[].range`), their
// {from} and {to} filled in, such as the scenario moves.
const rangeChecks = (from: string, to: string) =>
	Object.values(config().tests ?? {}).flatMap((k) =>
		((k.range_checks ?? []) as { range?: string }[]).flatMap((c) =>
			c.range
				? [c.range.replaceAll("{from}", shellWord(from)).replaceAll("{to}", shellWord(to))]
				: [],
		),
	);

// `verify <from> <to>`: 0 when every non-merge commit of the range passes and
// its range checks hold, else 1.
export function verify(from = "", to = "HEAD", out: Output = TEXT): number {
	const range = !from || /^0+$/.test(from) ? to : `${from}..${to}`;
	const commits = git("rev-list", "--no-merges", "--reverse", range).split("\n").filter(Boolean);
	const results = commits.map((sha) => verifyCommit(sha, out));
	const passed = results.filter((r) => r.ok).length;
	const summary = `${passed}/${commits.length} commits pass the commit rules`;
	(out.json ? console.error : console.log)(summary);
	const ranged = rangeChecks(from, to).every((command) => run(command, out));
	if (out.json)
		emit({
			range: { from, to },
			commits: results,
			passed,
			total: commits.length,
			range_checks: ranged,
		});
	return passed < commits.length || !ranged ? 1 : 0;
}
