// Running one task check, for the task runner (cli.ts) and for CI (ci.ts).
import { spawnSync } from "node:child_process";
import { config } from "./config.ts";
import { git, type Check } from "./repo.ts";

function pushed(): boolean {
	try {
		return git("branch", "-r", "--contains", "HEAD").trim().length > 0;
	} catch {
		return false;
	}
}

// `toStderr`: a verbose check's command line and output go to stderr, so that
// `--json` keeps stdout for its one object.
export function runCheck(
	check: Check,
	verbose: boolean,
	toStderr = false,
): "pass" | "fail" | "pending" {
	if (check.after === "push" && !pushed()) return "pending";
	const command = check.run ?? check.fails!;
	const line = `  $ ${command}${check.fails ? "   (must fail)" : ""}`;
	if (verbose) (toStderr ? console.error : console.log)(line);
	const result = spawnSync("sh", ["-c", command], {
		stdio: verbose ? ["inherit", toStderr ? 2 : "inherit", "inherit"] : "ignore",
		timeout: (check.timeout ?? config().ledger?.check?.timeout ?? 600) * 1000,
	});
	const ok = result.status === 0;
	return ok === !check.fails ? "pass" : "fail";
}
