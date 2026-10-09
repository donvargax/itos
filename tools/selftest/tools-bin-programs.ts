// The programs in tools/bin are shell scripts, which linux and macOS start by
// their #! line and windows cannot start at all: a spawn of one there ends
// with no exit code, and the gate that ran it fails without saying why. Every
// program under tools/ starts one of them through one resolver (T-135), so
// these checks run the real programs the way those programs run them, and a
// change that leaves windows out is caught here rather than in a gate's own
// output.
//
// The programs are the repository's, run in the repository: tools/bin/itos
// builds and runs the Go binary from this tree, and tools/bin/dev-version asks
// git what version this checkout is. Neither reaches the network.
import { strict as assert } from "node:assert";
import { spawnSync } from "node:child_process";
import { resolve } from "node:path";
import { repoProgram } from "../bin/repo-program.ts";

const ROOT = resolve(".");

// spawn is how a program under tools/ starts one of them: the resolver's
// command, with the arguments it gives.
function spawn(script: string, args: string[]) {
	const [program, argv] = repoProgram(resolve(ROOT, script), args);
	return spawnSync(program, argv, { cwd: ROOT, encoding: "utf8" });
}

function answers(name: string, run: ReturnType<typeof spawn>, wanted: RegExp): void {
	assert.equal(
		run.status,
		0,
		`${name} exited ${run.status ?? run.signal} (${run.error?.message ?? "no error"}):\n${run.stdout ?? ""}${run.stderr ?? ""}`,
	);
	assert.match(run.stdout, wanted, `${name} printed:\n${run.stdout}${run.stderr}`);
}

// tools/bin/itos is what every hook, every task check and every self-test runs.
answers("tools/bin/itos --version", spawn("tools/bin/itos", ["--version"]), /^itos \S/m);

// tools/bin/dev-version stamps the binary the wrapper above builds.
answers("tools/bin/dev-version", spawn("tools/bin/dev-version", [ROOT]), /\S/);

console.log("tools-bin-programs: the repository's programs answer here");
