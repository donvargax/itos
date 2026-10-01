// The footer rules (footers.ts's checkFooter), which itos runs beside any
// header lint delegate: one rule per footer of itos.yaml, read at the commit
// ITOS_AT names as the re-check of a pushed range sets it, else the staged
// tree. What the footer rules decide, at which tree, is
// conformance/messages.yaml's and verify.yaml's.
import { afterEach, describe, expect, it } from "vite-plus/test";
import { checkFooter } from "./footers.ts";

const message = (type: string, footers: string) => `${type}: add a thing\n\n${footers}\n`;

afterEach(() => {
	delete process.env.ITOS_AT;
});

describe("the footer rules", () => {
	it("read the project's ledger", () => {
		process.env.ITOS_AT = "HEAD";
		expect(checkFooter("Task", "build", message("build", "Task: T-001"))).toEqual([true]);
		expect(checkFooter("Task", "build", message("build", "Task: T-0000"))).toEqual([
			false,
			"unknown tasks: T-0000",
		]);
		expect(checkFooter("Task", "chore", message("chore", ""))).toEqual([
			false,
			'chore commits need a "Task: T-…" footer',
		]);
	});
});
