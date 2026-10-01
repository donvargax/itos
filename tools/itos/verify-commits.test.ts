// The commitlint plugin (commitlint.config.ts), the project's header lint
// delegate: one rule per footer of itos.yaml, each the footer rule of
// footers.ts, read at the commit ITOS_AT names as the re-check of a pushed
// range sets it, else the staged tree. What the footer rules decide, at which
// tree, is conformance/messages.yaml's and verify.yaml's.
import { afterEach, describe, expect, it } from "vite-plus/test";
import { footerRule } from "../../commitlint.config.ts";

// A commit as the rule sees it: only its type and raw text matter here.
const commit = (type: string, footers: string) =>
	({ type, raw: `${type}: add a thing\n\n${footers}\n` }) as unknown as Parameters<
		ReturnType<typeof footerRule>
	>[0];

afterEach(() => {
	delete process.env.ITOS_AT;
});

describe("the commitlint plugin's footer rules", () => {
	it("read the project's ledger", () => {
		process.env.ITOS_AT = "HEAD";
		expect(footerRule("Task")(commit("build", "Task: T-001"))).toEqual([true]);
		expect(footerRule("Task")(commit("build", "Task: T-0000"))).toEqual([
			false,
			"unknown tasks: T-0000",
		]);
		expect(footerRule("Task")(commit("chore", ""))).toEqual([
			false,
			'chore commits need a "Task: T-…" footer',
		]);
	});
});
