// pre-push hook: run what the pushed commits affect, and nothing slow — the
// unit tests their changes reach. The scenarios their `Scenarios:` footers
// name and the checks of the tasks their `Task:` footers name are CI's, with
// the whole unit and E2E suites (ci.ts): on a shared machine they take
// minutes, and CI runs them anyway. `itos hook pre-push <remote> <url>`
// (hooks.ts) runs it, with git's lines on stdin.
import { spawnSync } from "node:child_process";
import { section } from "./config.ts";

const ZERO = /^0+$/;
const known = (sha: string) => spawnSync("git", ["cat-file", "-e", `${sha}^{commit}`]).status === 0;

// The remote commits the push builds on, from git's `<local ref> <local sha>
// <remote ref> <remote sha>` lines. A new branch, or a remote commit this
// clone doesn't have, gives no base to compare with, so the unit suite runs
// whole; a deleted branch (zero local SHA) runs nothing.
export function pushBases(input: string): { bases: string[]; whole: boolean } {
	const bases = new Set<string>();
	let whole = false;
	for (const line of input.split("\n").filter(Boolean)) {
		const [, localSha, , remoteSha] = line.split(" ");
		if (!localSha || ZERO.test(localSha)) continue;
		if (remoteSha && !ZERO.test(remoteSha) && known(remoteSha)) bases.add(remoteSha);
		else whole = true;
	}
	return { bases: [...bases], whole };
}

const run = (command: string) => {
	console.log(`$ ${command}`);
	return spawnSync("sh", ["-c", command], { stdio: "inherit" }).status === 0;
};

// Vitest follows the imports from every file changed since the base (plus
// vite.config.ts's forceRerunTriggers), so this is every unit test the pushed
// changes can reach. The commands are itos.yaml's `hooks.pre_push`:
// `per_base` once per base, else `whole`. 0 when they pass, 1 when not.
export function prePush(input: string): number {
	const { bases, whole } = pushBases(input);
	const commands = section("hooks").pre_push!;
	let ok = true;
	if (whole) ok = run(commands.whole) && ok;
	else for (const base of bases) ok = run(commands.per_base.replaceAll("{base}", base)) && ok;
	return ok ? 0 : 1;
}
