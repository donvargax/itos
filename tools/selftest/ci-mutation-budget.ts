// The mutation budget of q-41 and q-52 (T-131), proved through the real CI planner and driver:
// a push's code range runs proof.code.check, one freshly judged mutation site of its changed Go
// functions, and a nightly runs twenty over cmd and internal with a seed of its own and without
// strict coverage (q-54), and neither plan runs any other mutation command. Each plan is this
// repository's own, whole, with only the configured shell replaced in a scratch config, so the
// driver dispatches every planned command to a fake that logs it, and nothing it names (itos-cc,
// the features, CI's other steps) runs. A provider's refusal, and a provider that cannot run,
// must still fail the run.
import { strict as assert } from "node:assert";
import { spawnSync } from "node:child_process";
import { chmodSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { parse, stringify } from "yaml";
import { repoProgram } from "../bin/repo-program.ts";
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

const log = join(scratch, "commands.jsonl");
const fakeShell = join(scratch, "fake-shell.mjs");
// The fake answers every command with exit 0, except a mutation command while CI_MUTATION_FAIL
// is set: "refuse" prints itos-cc's refusal of a survivor and exits 1, a number exits with it.
writeFileSync(
	fakeShell,
	`import { appendFileSync } from "node:fs";
const command = process.argv.at(-1);
appendFileSync(process.env.CI_MUTATION_COMMANDS, JSON.stringify({ command }) + "\\n");
const fail = process.env.CI_MUTATION_FAIL;
if (fail && command.includes("itos-cc mutation")) {
	if (fail === "refuse") {
		process.stdout.write(JSON.stringify({ schema: 1, ok: false, problems: [{ rule: "mutation.survived", message: "a mutant of ci.Run survived", fix: "test it" }] }));
		process.exit(1);
	}
	process.exit(Number(fail));
}
`,
);
chmodSync(fakeShell, 0o755);

const configPath = join(scratch, "itos.yaml");
const config = structuredClone(policy);
config.shell = [process.execPath, fakeShell];
writeFileSync(configPath, stringify(config));

const invocation = (args: string[], fail?: string) => {
	writeFileSync(log, "");
	const childEnv = {
		...env,
		ITOS_CONFIG: configPath,
		CI_MUTATION_COMMANDS: log,
		...(fail ? { CI_MUTATION_FAIL: fail } : {}),
	};
	const [program, argv] = repoProgram(join(root, "tools/bin/itos"), args);
	const run = spawnSync(program, argv, {
		cwd: root,
		env: childEnv,
		encoding: "utf8",
		maxBuffer: 64 * 1024 * 1024,
	});
	assert.equal(run.error, undefined, `tools/bin/itos could not start: ${run.error}`);
	const commands = readFileSync(log, "utf8")
		.split("\n")
		.filter(Boolean)
		.map((line) => JSON.parse(line).command as string);
	return {
		status: run.status,
		output: `${run.stdout}${run.stderr}`,
		json: JSON.parse(run.stdout),
		commands,
	};
};

// Every itos-cc mutation command a plan dispatched: the plan's budget is exactly these.
const mutations = (commands: string[]) =>
	commands.filter((command) => command.includes("itos-cc mutation"));

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

	// A push: one fresh mutant of the range's changed Go functions, strict coverage, cmd and
	// internal, and no other mutation command (the cached sample and complete check are gone).
	const push = invocation(["ci", "run", from, to, "--json"]);
	assert.equal(push.status, 0, `push CI driver did not pass with fake commands:\n${push.output}`);
	assert.equal(push.json.ok, true);
	const expectedPush = `tools/bin/pinned itos-cc mutation run --count 1 --since ${from} --fail-uncovered --json cmd internal`;
	assert.deepEqual(
		mutations(push.commands),
		[expectedPush],
		`push CI must run exactly the code proof ${expectedPush}:\n${push.commands.join("\n")}`,
	);

	// A nightly: twenty fresh mutants over cmd and internal, seeded by the run, not HEAD, with
	// no range, no strict coverage (q-54: with no range it judges every function), and no other
	// mutation command.
	const nightly = invocation(["ci", "run", "--nightly", "--json"]);
	assert.equal(
		nightly.status,
		0,
		`nightly CI driver did not pass with fake commands:\n${nightly.output}`,
	);
	assert.equal(nightly.json.ok, true);
	const expectedNightly =
		'tools/bin/pinned itos-cc mutation run --count 20 --seed "${GITHUB_RUN_ID:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}" --json cmd internal';
	assert.deepEqual(
		mutations(nightly.commands),
		[expectedNightly],
		`the nightly must run exactly ${expectedNightly}:\n${nightly.commands.join("\n")}`,
	);
	// The seed as the nightly's sh expands it: GitHub's run id, so two runs of one commit draw
	// differently; by hand, the time.
	const seedArg = (vars: Record<string, string>) => {
		const seed = expectedNightly.match(/--seed ("[^"]*")/)![1];
		const run = spawnSync("sh", ["-c", `printf '%s' ${seed}`], {
			env: { PATH: process.env.PATH ?? "", ...vars },
			encoding: "utf8",
		});
		assert.equal(run.status, 0, run.stderr);
		return run.stdout;
	};
	assert.equal(seedArg({ GITHUB_RUN_ID: "1234" }), "1234");
	assert.match(seedArg({}), /^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ$/);

	// A provider's refusal fails the push at the code proof, its problem kept.
	const refused = invocation(["ci", "run", from, to, "--json"], "refuse");
	assert.equal(refused.status, 1, `a refusing provider did not block CI:\n${refused.output}`);
	assert.equal(refused.json.ok, false);
	assert.equal(refused.json.failed_at.step, expectedPush);
	assert.equal(refused.json.failed_at.problems[0].rule, "mutation.survived");

	// A provider that cannot run (itos-cc's exit 3: not Linux, no Git, a tool missing) fails it
	// too, never a pass.
	const broken = invocation(["ci", "run", from, to, "--json"], "3");
	assert.equal(broken.status, 1, `a provider that cannot run did not block CI:\n${broken.output}`);
	assert.equal(broken.json.failed_at.step, expectedPush);
	assert.equal(broken.json.failed_at.problems[0].rule, "ci-code-proof");

	// And the nightly's step fails the night.
	const night = invocation(["ci", "run", "--nightly", "--json"], "1");
	assert.equal(night.status, 1, `a failing nightly provider did not block:\n${night.output}`);
	assert.equal(night.json.failed_at.step, expectedNightly);
	assert.equal(night.json.failed_at.code, 1);

	console.log(
		"ci mutation budget: a push runs one fresh mutant of its changed Go code, a night twenty with its own seed, and nothing else; a failing provider blocks both",
	);
} finally {
	rmSync(scratch, { recursive: true, force: true });
}
