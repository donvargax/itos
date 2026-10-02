// The Go build held to the whole suite (PLAN.md, "The port's proof"): builds
// the Go binary with tools/bin/build-go.ts into a scratch folder, as the
// release does, and runs the whole conformance corpus and every feature
// against it.
//
//   node tools/selftest/go-port.ts
//
// The corpus runs through tools/itos/conformance/run.ts with --bin pointed at
// the build, every file of it; the features run as
// `go test ./features -count=1` with ITOS_BIN pointed at the build, every live
// scenario. There is no list of what is ported: every command group is
// (T-039 to T-052). While the TypeScript was the implementation it was a step
// of every push (T-053); since it left (T-062), CI's corpus step and its one
// features run judge tools/bin/itos, the same tree built on demand, and this
// is the port's tasks' check: the whole of both against the release's build.
//
// Exits 1 when the build, the corpus or the features fail, naming which.
import { spawnSync } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { buildNative, ROOT } from "../bin/build-go.ts";
import { outsideEnv } from "./scratch.ts";

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
	run("the whole corpus", [process.execPath, "tools/itos/conformance/run.ts", "--bin", bin]);
	run("every feature", ["go", "test", "./features", "-count=1"], { ITOS_BIN: bin });
} catch (error) {
	failures.push(`the build: ${(error as Error).message}`);
} finally {
	rmSync(scratch, { recursive: true, force: true });
}

if (failures.length) {
	console.error(`\ngo port: FAIL\n${failures.map((f) => `  - ${f}`).join("\n")}`);
	process.exit(1);
}
console.log("\ngo port: the Go build passes the whole corpus and every feature");
