// `itos tests list <kind> [--at <tree>]`: the kind's adapter's listing
// (tests.ts), at the working tree, the index or a commit.
import { emit, type Output, TEXT } from "./problem.ts";
import { listTests } from "./tests.ts";

export function testsList(name: string, at: string | undefined, out: Output = TEXT): number {
	const list = listTests(name, { at: at ?? "worktree" });
	if (out.json) emit(list);
	else for (const t of list.tests) console.log(`${t.id}  ${t.file}${t.live ? "" : "  (not live)"}`);
	return 0;
}
