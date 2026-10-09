import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtempSync, readFileSync, readdirSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { CaseTimeoutError, runCase } from "./run.ts";

type Regression = { name: string; run: (nativeGit: string) => Promise<void> };

const scratchRoot = () => mkdtempSync(join(tmpdir(), "itos-conformance-runner-test-"));
const assertScratchEmpty = (root: string) => assert.deepEqual(readdirSync(root), [], "runCase left its own scratch directory behind");

function processIsRunning(pid: number): boolean {
	try {
		process.kill(pid, 0);
	} catch (error) {
		if ((error as NodeJS.ErrnoException).code === "ESRCH") return false;
		throw error;
	}
	if (process.platform === "win32") return true;
	const status = spawnSync("ps", ["-o", "stat=", "-p", String(pid)], { encoding: "utf8" });
	if (status.status !== 0) return false;
	const state = status.stdout.trim();
	return state !== "" && !state.startsWith("Z");
}

const fakeGitAndNativeInspection: Regression = {
	name: "a PATH-first Git shim remains fake for the command while config inspection uses native Git",
	async run(nativeGit) {
		const root = scratchRoot();
		try {
			const { outcome } = await runCase(
				"sh",
				{
					name: "native inspection does not inherit fake Git",
					argv: ["-c", "git --version"],
					git: [],
					files: {
						"old-git": {
							text: `#!/bin/sh\nif [ "$1" = "--version" ]; then echo "git version 2.30.0"; exit 0; fi\nexec "{{git}}" "$@"\n`,
							executable: true,
						},
						"shim-bin/git": { text: "#!/bin/sh\nexec \"$ITOS_GIT\" \"$@\"\n", executable: true },
					},
					env: {
						ITOS_GIT: "{{dir}}/old-git",
						PATH: "{{dir}}/shim-bin{{PATH_SEP}}{{PATH}}",
					},
					git_config_after: {
						"hook.itos-commit-msg.event": "commit-msg",
						"hook.itos-pre-push.event": "pre-push",
					},
					exit: 0,
				},
				{ nativeGit, deadlineMs: 10_000, tempRoot: root },
			);
			assert.equal(outcome.exit, 0);
			assert.equal(outcome.stdout, "git version 2.30.0\n", "the command under test must still reach fixture Git");
			assert.deepEqual(outcome.gitConfig, {
				"hook.itos-commit-msg.event": "commit-msg",
				"hook.itos-pre-push.event": "pre-push",
			});
			assertScratchEmpty(root);
		} finally {
			rmSync(root, { recursive: true, force: true });
		}
	},
};

const legacyGitFallbackUsesNativePath: Regression = {
	name: "an old fixture's bare Git fallback reaches native Git without replacing its fake version",
	async run(nativeGit) {
		const root = scratchRoot();
		try {
			const { outcome } = await runCase(
				"sh",
				{
					name: "legacy old-Git fallback",
					argv: ["-c", `"$ITOS_GIT" --version; "$ITOS_GIT" config --get-all hook.itos-commit-msg.event`],
					git: [],
					files: {
						"old-git": {
							text: `#!/bin/sh\nif [ "$1" = --version ]; then echo "git version 2.30.0"; exit 0; fi\nexec git "$@"\n`,
							executable: true,
						},
					},
					env: { ITOS_GIT: "{{dir}}/old-git" },
					git_config_after: { "hook.itos-commit-msg.event": "commit-msg" },
					exit: 0,
				},
				{ nativeGit, deadlineMs: 5_000, tempRoot: root },
			);
			assert.equal(outcome.exit, 0);
			assert.equal(outcome.stdout, "git version 2.30.0\ncommit-msg\n");
			assert.deepEqual(outcome.gitConfig, { "hook.itos-commit-msg.event": "commit-msg" });
			assertScratchEmpty(root);
		} finally {
			rmSync(root, { recursive: true, force: true });
		}
	},
};

const hungPostCheckIsBounded: Regression = {
	name: "a hanging git_config_after inspection is bounded by the case deadline",
	async run(nativeGit) {
		const root = scratchRoot();
		const started = join(root, "inspection-started");
		const script = [
			`const fs = require("node:fs");`,
			`const { spawnSync } = require("node:child_process");`,
			`const args = process.argv.slice(1);`,
			`if (args[0] === "config" && args[1] === "--get-all") { fs.writeFileSync(${JSON.stringify(started)}, "started"); setInterval(() => {}, 1000); }`,
			`else { const result = spawnSync(${JSON.stringify(nativeGit)}, args, { stdio: "inherit" }); process.exit(result.status ?? 1); }`,
		].join("\n");
		try {
			const began = performance.now();
			await assert.rejects(
				runCase(
					process.execPath,
					{
						name: "hung post-check",
						argv: ["-e", "process.exit(0)"],
						files: {
							".git/HEAD": "ref: refs/heads/main\n",
							".git/config": '[core]\n\trepositoryformatversion = 0\n\tbare = false\n',
						},
						git_config_after: { "hook.itos-commit-msg.event": "commit-msg" },
						exit: 0,
					},
					{ nativeGit: process.execPath, nativeGitArgs: ["-e", script], deadlineMs: 2_000, tempRoot: root },
				),
				(error) => error instanceof CaseTimeoutError,
			);
			assert.ok(readFileSync(started, "utf8") === "started", "the timeout must reach the post-case Git inspection");
			assert.ok(performance.now() - began < 5_000, "the post-check outlived its injected deadline");
			assert.deepEqual(readdirSync(root), ["inspection-started"], "runCase left its private scratch directory behind");
		} finally {
			rmSync(root, { recursive: true, force: true });
		}
	},
};

const commandDescendantIsReaped: Regression = {
	name: "a command descendant holding output open is killed with its case process group",
	async run(nativeGit) {
		const root = scratchRoot();
		const pidFile = join(root, "descendant.pid");
		const command = [
			`const { spawn } = require("node:child_process");`,
			`const { writeFileSync } = require("node:fs");`,
			`const child = spawn(process.execPath, ["-e", "setInterval(() => {}, 1000)"], { stdio: "inherit" });`,
			`writeFileSync(process.env.T129_PID_FILE, String(child.pid));`,
			`setInterval(() => {}, 1000);`,
		].join("\n");
		try {
			const began = performance.now();
			await assert.rejects(
				runCase(
					process.execPath,
					{
						name: "descendant holds output open",
						argv: ["-e", command],
						env: { T129_PID_FILE: pidFile },
						exit: 0,
					},
					{ nativeGit, deadlineMs: 400, tempRoot: root },
				),
				(error) => error instanceof CaseTimeoutError,
			);
			assert.ok(performance.now() - began < 3_000, "the descendant held the case beyond its deadline");
			const pid = Number(readFileSync(pidFile, "utf8"));
			assert.ok(Number.isInteger(pid) && pid > 0, "the command did not record its descendant pid");
			assert.equal(processIsRunning(pid), false, "the descendant remained live after case cleanup");
			assert.deepEqual(readdirSync(root), ["descendant.pid"], "runCase left its private scratch directory behind");
		} finally {
			rmSync(root, { recursive: true, force: true });
		}
	},
};

export const runnerRegressions: Regression[] = [
	fakeGitAndNativeInspection,
	legacyGitFallbackUsesNativePath,
	hungPostCheckIsBounded,
	commandDescendantIsReaped,
];

// The full corpus entry point runs these same behavioral checks so they remain
// covered after the task's temporary progress checks are eventually removed.
export async function runRunnerRegressions(nativeGit: string) {
	for (const regression of runnerRegressions) {
		try {
			await regression.run(nativeGit);
		} catch (error) {
			throw new Error(`${regression.name}: ${(error as Error).message}`, { cause: error });
		}
	}
}
