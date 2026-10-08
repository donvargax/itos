// itos-cc's listed-test protocol: one live scenario ID, a tab, then its path
// from the repository root. `itos tests list --json` is the source of truth;
// only this adapter reshapes its JSON for the provider.
import { spawnSync } from "node:child_process";

const listed = spawnSync("tools/bin/itos", ["tests", "list", "scenario", "--json"], {
	cwd: process.cwd(),
	encoding: "utf8",
});

if (listed.status !== 0) {
	process.stderr.write(
		listed.stderr || `tools/bin/itos tests list scenario exited ${listed.status}`,
	);
	process.exit(listed.status ?? 1);
}

let report: unknown;
try {
	report = JSON.parse(listed.stdout);
} catch (error) {
	console.error(
		`tools/bin/itos tests list scenario returned invalid JSON: ${(error as Error).message}`,
	);
	process.exit(1);
}

if (
	typeof report !== "object" ||
	report === null ||
	!("tests" in report) ||
	!Array.isArray(report.tests)
) {
	console.error("tools/bin/itos tests list scenario returned no tests list");
	process.exit(1);
}

for (const test of report.tests) {
	if (
		typeof test !== "object" ||
		test === null ||
		typeof test.id !== "string" ||
		typeof test.file !== "string" ||
		typeof test.live !== "boolean"
	) {
		console.error("tools/bin/itos tests list scenario returned an invalid test");
		process.exit(1);
	}
	if (test.live) process.stdout.write(`${test.id}\tfeatures/${test.file}\n`);
}
