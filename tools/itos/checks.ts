// Running one task check, for the task runner (cli.ts), for CI (ci.ts) and
// for the commit-msg hook (commit-tasks.ts).
import { closeSync, mkdtempSync, openSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { config } from "./config.ts";
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
const timeoutOf = (check: Check) => check.timeout ?? config().ledger?.check?.timeout ?? 600;

// `toStderr`: a verbose check's command line and output go to stderr, so that
// `--json` keeps stdout for its one object.
export function runCheck(check: Check, verbose: boolean, toStderr = false): Result {
	if (check.after === "push" && !pushed()) return "pending";
	const command = check.run ?? check.fails!;
	const line = `  $ ${command}${check.fails ? "   (must fail)" : ""}`;
	if (verbose) (toStderr ? console.error : console.log)(line);
	const result = inShell(command, {
		stdio: verbose ? ["inherit", toStderr ? 2 : "inherit", "inherit"] : "ignore",
		timeout: timeoutOf(check) * 1000,
	});
	const ok = result.status === 0;
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
