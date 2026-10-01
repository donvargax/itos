// Every conformance fixture (tools/itos/conformance/*.yaml) through
// tools/bin/itos, so the unit suite, which CI runs whole on every push, holds
// the tool to the cases any implementation of it is held to. One test per
// file, each file's cases run by run.ts in scratch folders of their own.
import { spawnSync } from "node:child_process";
import { basename, relative } from "node:path";
import { describe, expect, it } from "vite-plus/test";
import { fixtureFiles } from "./run.ts";

describe("itos's conformance fixtures", () => {
	it.each(fixtureFiles().map((file) => relative(process.cwd(), file)))(
		"%s passes",
		(file) => {
			const run = spawnSync(
				process.execPath,
				["tools/itos/conformance/run.ts", "--bin", "tools/bin/itos", "--only", file],
				{ encoding: "utf8" },
			);
			expect(run.status, `${basename(file)}:\n${run.stderr}${run.stdout}`).toBe(0);
		},
		120_000,
	);
});
