// Exercise the configured mutation samples through the real CI planner and
// driver. The configured shell is replaced only in a scratch config, so the
// driver dispatches the planned commands without running itos-cc or CI's other
// expensive steps.
import { strict as assert } from "node:assert";
import { spawnSync } from "node:child_process";
import { chmodSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { parse, stringify } from "yaml";
import { outsideEnv, realGit } from "./scratch.ts";

const root = resolve(".");
const scratch = mkdtempSync(join(root, ".ci-mutation-budget-"));
const env = { ...outsideEnv(), GIT_INDEX_FILE: join(scratch, "index") };
const git = (args: string[], input?: string) => {
	const run = spawnSync(realGit(), args, { cwd: root, env, input, encoding: "utf8" });
	assert.equal(run.status, 0, `git ${args.join(" ")} failed:\n${run.stdout}${run.stderr}`);
	return run.stdout.trim();
};

const policy = parse(readFileSync(join(root, "itos.yaml"), "utf8")) as any;
const pushSample = policy.ci.steps.find(
	(step: { run?: unknown }) => typeof step.run === "string" && step.run.includes("mutation sample"),
);
const nightlySample = policy.ci.nightly.steps.find((step: string | { run?: unknown }) => {
	const command = typeof step === "string" ? step : step.run;
	return typeof command === "string" && command.includes("mutation sample");
});
assert.ok(pushSample, "the repository's push CI plan must configure its mutation sample");
assert.ok(nightlySample, "the repository's nightly plan must configure its mutation sample");

const log = join(scratch, "commands.jsonl");
const fakeShell = join(scratch, "fake-shell.mjs");
writeFileSync(
	fakeShell,
	`import { appendFileSync } from "node:fs";
const command = process.argv.at(-1);
appendFileSync(process.env.CI_MUTATION_COMMANDS, JSON.stringify({ command }) + "\\n");
if (command.includes("mutation sample")) process.exit(Number(process.env.CI_MUTATION_FAIL ?? 0));
if (command.includes("mutation check")) process.stdout.write(JSON.stringify({ schema: 1, ok: true }));
`,
);
chmodSync(fakeShell, 0o755);

const fixture = (nightly: boolean) => {
	const config = structuredClone(policy);
	config.shell = [process.execPath, fakeShell];
	config.ci.steps = [structuredClone(pushSample)];
	config.ci.nightly.steps = [structuredClone(nightlySample)];
	const path = join(scratch, nightly ? "nightly.yaml" : "push.yaml");
	writeFileSync(path, stringify(config));
	return path;
};

const invocation = (args: string[], configPath: string, failSample = false) => {
	const childEnv = {
		...env,
		ITOS_CONFIG: configPath,
		CI_MUTATION_COMMANDS: log,
		...(failSample ? { CI_MUTATION_FAIL: "41" } : {}),
	};
	const run = spawnSync(join(root, "tools/bin/itos"), args, {
		cwd: root,
		env: childEnv,
		encoding: "utf8",
		maxBuffer: 64 * 1024 * 1024,
	});
	assert.equal(run.error, undefined, `tools/bin/itos could not start: ${run.error}`);
	return {
		status: run.status,
		output: `${run.stdout}${run.stderr}`,
		json: JSON.parse(run.stdout),
	};
};

const dispatched = () =>
	readFileSync(log, "utf8")
		.split("\n")
		.filter(Boolean)
		.map((line) => JSON.parse(line).command as string);

try {
	// Give the planner a real code-path range without moving HEAD, the shared
	// index, or any branch. All Git environment routing is cleared by outsideEnv.
	git(["read-tree", "HEAD"]);
	const blob = git(["hash-object", "-w", "--stdin"], "ci mutation budget self-test\n");
	git(["update-index", "--add", "--cacheinfo", `100644,${blob},internal/ci/ci.go`]);
	const tree = git(["write-tree"]);
	const to = git([
		"-c",
		"user.name=ci-mutation-budget",
		"-c",
		"user.email=selftest@localhost",
		"commit-tree",
		tree,
		"-p",
		"HEAD",
		"-m",
		"ci: mutation budget self-test range",
	]);
	const from = git(["rev-parse", "HEAD"]);

	const push = invocation(["ci", "run", from, to, "--json"], fixture(false));
	assert.equal(push.status, 0, `push CI driver did not pass with fake commands:\n${push.output}`);
	assert.equal(push.json.ok, true);
	const pushCommands = dispatched();
	const expectedPush = `tools/bin/pinned itos-cc mutation sample --since '${from}' --count 1 --json cmd internal`;
	assert.ok(
		pushCommands.includes(expectedPush),
		`push CI did not dispatch ${expectedPush}:\n${pushCommands.join("\n")}`,
	);
	assert.ok(
		pushCommands.some(
			(command) =>
				command ===
				`tools/bin/pinned itos-cc mutation check --since ${from} --fail-uncovered --json`,
		),
		`the complete-cache proof was not dispatched for the code range:\n${pushCommands.join("\n")}`,
	);

	writeFileSync(log, "");
	const nightly = invocation(["ci", "run", "--nightly", "--json"], fixture(true));
	assert.equal(
		nightly.status,
		0,
		`nightly CI driver did not pass with fake commands:\n${nightly.output}`,
	);
	assert.equal(nightly.json.ok, true);
	const nightlyCommands = dispatched();
	const expectedNightly = "tools/bin/pinned itos-cc mutation sample --count 20 --json cmd internal";
	assert.ok(
		nightlyCommands.includes(expectedNightly),
		`nightly did not dispatch ${expectedNightly}:\n${nightlyCommands.join("\n")}`,
	);
	assert.ok(
		!nightlyCommands.some(
			(command) => command.includes("mutation sample") && command.includes("--since"),
		),
	);

	writeFileSync(log, "");
	const refused = invocation(["ci", "run", from, to, "--json"], fixture(false), true);
	assert.equal(refused.status, 1, `a failing sample provider did not block CI:\n${refused.output}`);
	assert.equal(refused.json.ok, false);
	assert.equal(refused.json.failed_at.code, 41);
	assert.equal(refused.json.failed_at.step, expectedPush);

	console.log(
		"ci mutation budget: push and nightly dispatch their bounded cached samples; provider failure blocks",
	);
} finally {
	rmSync(scratch, { recursive: true, force: true });
}
