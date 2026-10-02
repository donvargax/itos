// Running one task check, for the task runner (cli.ts), for CI (ci.ts) and
// for the commit-msg hook (commit-tasks.ts).
import { closeSync, mkdtempSync, openSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { config, normal } from "./config.ts";
import { git, type Check } from "./repo.ts";
import { inShell } from "./shell.ts";

type Result = "pass" | "fail" | "pending";

function pushed(): boolean {
	try {
		return git("branch", "-r", "--contains", "HEAD").trim().length > 0;
	} catch {
		return false;
	}
}

// A check's own timeout, in seconds: its `timeout:`, else the ledger's.
const timeoutOf = (check: Check) => check.timeout ?? config().ledger.check.timeout;

// One invocation's runs, so that a check several tasks list runs once: each
// distinct check's exit status, 0 or not, by its key. Kept for one
// invocation only; nothing is cached across runs.
export type Runs = Map<string, boolean>;

// What makes two checks the same check: the same command, its whitespace
// collapsed as the config's patterns read it, and the same timeout. Whether it
// must pass or fail is not part of it: each task reads the one exit status by
// its own `run:` or `fails:`.
const runKey = (check: Check) => `${normal(check.run ?? check.fails!)}\0${timeoutOf(check)}`;

// `toStderr`: a verbose check's command line and output go to stderr, so that
// `--json` keeps stdout for its one object. `runs`: the invocation's runs
// so far; a check already in it is not run again, and its command line, when
// verbose, says so.
export function runCheck(check: Check, verbose: boolean, toStderr = false, runs?: Runs): Result {
	if (check.after === "push" && !pushed()) return "pending";
	const command = check.run ?? check.fails!;
	const key = runKey(check);
	const reused = runs?.get(key);
	const mode = check.fails ? "   (must fail)" : "";
	const note = reused === undefined ? "" : "   (ran above; its exit status reused)";
	if (verbose) (toStderr ? console.error : console.log)(`  $ ${command}${mode}${note}`);
	const run = () =>
		inShell(command, {
			stdio: verbose ? ["inherit", toStderr ? 2 : "inherit", "inherit"] : "ignore",
			timeout: timeoutOf(check) * 1000,
		}).status === 0;
	const ok = reused ?? run();
	runs?.set(key, ok);
	return ok === !check.fails ? "pass" : "fail";
}

export interface Captured {
	result: "pass" | "fail";
	// What it printed, stdout and stderr together.
	output: string;
	// Its exit code; null when it was stopped.
	status: number | null;
	// The seconds it was given, whether the cap set them, and whether it
	// outlasted them.
	seconds: number;
	capped: boolean;
	timedOut: boolean;
}

// One check run quietly, its output kept to print when it fails, with its
// timeout capped at `cap` seconds. The output goes to a file rather than a
// pipe, so a command it leaves running cannot hold the run past its timeout.
// `env` is the environment it runs in.
export function runCheckCaptured(
	check: Check,
	cap = Infinity,
	env: NodeJS.ProcessEnv = process.env,
): Captured {
	const command = check.run ?? check.fails!;
	const own = timeoutOf(check);
	const seconds = Math.min(own, cap);
	const dir = mkdtempSync(join(tmpdir(), "itos-check-"));
	const file = join(dir, "output");
	const fd = openSync(file, "w");
	try {
		let run;
		try {
			run = inShell(command, { stdio: ["ignore", fd, fd], timeout: seconds * 1000, env });
		} finally {
			closeSync(fd);
		}
		const timedOut = (run.error as NodeJS.ErrnoException | undefined)?.code === "ETIMEDOUT";
		const ok = run.status === 0;
		return {
			result: !timedOut && ok === !check.fails ? "pass" : "fail",
			output: readFileSync(file, "utf8"),
			status: run.status,
			seconds,
			capped: cap < own,
			timedOut,
		};
	} finally {
		rmSync(dir, { recursive: true, force: true });
	}
}
