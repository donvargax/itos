// The Go build of itos, the one way every self-test and, later, the release
// workflow builds it:
//
//   node tools/bin/build-go.ts <out dir>
//
// builds ./cmd/itos into <out dir>/itos for this machine and prints its path.
// The binary is stamped with package.json's version (-ldflags -X), the one the
// TypeScript says and the conformance corpus's {{version}} expects, so a
// release still changes package.json alone; a binary built any other way says
// the module version Go records instead (internal/version). CGO_ENABLED=0, so
// the binary needs no C library, and -trimpath, so it holds no path of the
// machine that built it.
//
// `build` takes the target platform, so the release archives (PLAN.md §10)
// are this same build run once per platform.
import { spawnSync } from "node:child_process";
import { mkdirSync, readFileSync } from "node:fs";
import { join, resolve } from "node:path";

export const ROOT = resolve(import.meta.dirname, "../..");
const MODULE = "github.com/donvargax/itos";

const VERSION: string = (
	JSON.parse(readFileSync(join(ROOT, "package.json"), "utf8")) as { version: string }
).version;

// A platform Go builds for; undefined for this machine's.
interface Target {
	goos: string;
	goarch: string;
}

// Builds ./cmd/itos into `out` for the target, stamped with package.json's
// version, and returns the binary's path. Throws when go build fails, its
// output printed.
function build(out: string, target?: Target): string {
	const binary = resolve(out);
	mkdirSync(resolve(binary, ".."), { recursive: true });
	const env: NodeJS.ProcessEnv = { ...process.env, CGO_ENABLED: "0" };
	if (target) Object.assign(env, { GOOS: target.goos, GOARCH: target.goarch });
	const ldflags = `-s -w -X ${MODULE}/internal/version.stamp=${VERSION}`;
	const args = ["build", "-trimpath", "-ldflags", ldflags, "-o", binary, "./cmd/itos"];
	const result = spawnSync("go", args, { cwd: ROOT, env, stdio: "inherit" });
	if (result.status !== 0)
		throw new Error(`go ${args.join(" ")} failed (exit ${result.status ?? result.signal})`);
	return binary;
}

// The binary for this machine, `<dir>/itos`.
export const buildNative = (dir: string) =>
	build(join(dir, process.platform === "win32" ? "itos.exe" : "itos"));

if (import.meta.main) {
	const dir = process.argv[2];
	if (!dir) {
		console.error("usage: node tools/bin/build-go.ts <out dir>");
		process.exit(2);
	}
	try {
		console.log(buildNative(dir));
	} catch (error) {
		console.error(`build-go: ${(error as Error).message}`);
		process.exit(1);
	}
}
