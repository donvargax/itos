// The Go port held to what it has ported, on every push (PLAN.md, "The port's
// proof"): builds the Go binary with tools/bin/build-go.ts into a scratch
// folder and runs the ported set against it.
//
//   node tools/selftest/go-port.ts
//
// The ported set is PORTED below, the one list of it: the conformance corpus
// files and the scenario selections each landed command group has added.
// A corpus file runs through tools/itos/conformance/run.ts with --bin pointed
// at the build; a selection is a -scenarios= regular expression over the
// features' tags, run as `go test ./features -count=1 -scenarios=<it>` with
// ITOS_BIN pointed at the build. So a later change to a ported group must
// land in both implementations in one push: the corpus and the features
// judge the TypeScript in the same CI run.
//
// Exits 1 when the set is empty, since a guard that runs nothing proves
// nothing, or when anything in it fails, the build included.
import { spawnSync } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { buildNative, ROOT } from "../bin/build-go.ts";
import { outsideEnv } from "./scratch.ts";

// Each command group adds its part here when it lands (tasks/phase-2.yaml).
const PORTED = {
	// Corpus files, relative to the root: the command line itself (T-039), its
	// help texts (T-041), the config, the ledger, the registry and the smoke
	// sets as config check reads them (T-042), the task runner, task and task
	// list (T-043), the globs and path rules, commit check-paths (T-044), the
	// footer rules, commit check-message (T-045), the named tests, tests list
	// and tests smoke (T-046), verify (T-047; moves.yaml joins with the
	// commit-msg hook, T-052), and CI's plan, ci plan and ci scope (T-048).
	corpus: [
		"tools/itos/conformance/cli.yaml",
		"tools/itos/conformance/help.yaml",
		"tools/itos/conformance/config.yaml",
		"tools/itos/conformance/tasks.yaml",
		"tools/itos/conformance/globs.yaml",
		"tools/itos/conformance/scopes.yaml",
		"tools/itos/conformance/messages.yaml",
		"tools/itos/conformance/tests.yaml",
		"tools/itos/conformance/smoke.yaml",
		"tools/itos/conformance/verify.yaml",
		"tools/itos/conformance/plans.yaml",
	],
	// -scenarios= regular expressions, as a group's checks write them: the
	// scenarios that run config check (T-042), itos task (T-043), tests smoke
	// check (T-046), verify or tests moves (T-047), and ci plan (T-048).
	scenarios: [
		"^@ID-(CONFIG-1[2346]|LEDGER-0[12]|SINCE-0[4-7])$",
		"^@ID-(TASK-0[1-4]|LEDGER-0[34]|CONFIG-(01|18))$",
		"^@ID-CONFIG-(09|10|15)$",
		"^@ID-(SINCE-0[1238]|FOOT-04|CONFIG-04|MOVES-0[56])$",
		"^@ID-LEDGER-05$",
	],
};

if (PORTED.corpus.length + PORTED.scenarios.length === 0) {
	console.error("go port: the ported set is empty, so it proves nothing");
	process.exit(1);
}

const scratch = mkdtempSync(join(tmpdir(), "go-port-"));
// Inside a hook git exports GIT_DIR and friends; nothing here is that repository.
const env = outsideEnv();
const failures: string[] = [];

function run(what: string, command: string[], extra: NodeJS.ProcessEnv = {}) {
	console.log(`\n== ${what}: ${command.join(" ")}`);
	const result = spawnSync(command[0]!, command.slice(1), {
		cwd: ROOT,
		env: { ...env, ...extra },
		stdio: "inherit",
	});
	if (result.status !== 0) failures.push(`${what} (exit ${result.status ?? result.signal})`);
}

try {
	const bin = buildNative(scratch);
	if (PORTED.corpus.length)
		run("the ported corpus", [
			process.execPath,
			"tools/itos/conformance/run.ts",
			"--bin",
			bin,
			"--only",
			...PORTED.corpus,
		]);
	for (const pattern of PORTED.scenarios)
		run(
			`the scenarios ${pattern}`,
			["go", "test", "./features", "-count=1", `-scenarios=${pattern}`],
			{ ITOS_BIN: bin },
		);
} catch (error) {
	failures.push(`the build: ${(error as Error).message}`);
} finally {
	rmSync(scratch, { recursive: true, force: true });
}

if (failures.length) {
	console.error(`\ngo port: FAIL\n${failures.map((f) => `  - ${f}`).join("\n")}`);
	process.exit(1);
}
const parts = [...PORTED.corpus, ...PORTED.scenarios.map((p) => `scenarios ${p}`)];
console.log(`\ngo port: the Go build passes the ported set (${parts.join(", ")})`);
