// The Go build of itos, the one way every self-test builds it:
//
//   node tools/bin/build-go.ts <out dir>
//
// builds ./cmd/itos into <out dir>/itos for this machine and prints its path.
// The binary is stamped (-ldflags -X) with the version tools/bin/dev-version
// gives this checkout from git describe, as tools/bin/itos stamps it: the
// release's at its tag, 2.3.1-dev.5.g1234abc five commits after v2.3.0 (T-069;
// the tag is the version, and nothing in the tree holds one). A binary built
// any other way says the module version Go records instead (internal/version).
// CGO_ENABLED=0, so the binary needs no C library, and -trimpath, so it holds
// no path of the machine that built it.
//
// The release build:
//
//   node tools/bin/build-go.ts --release <out dir>
//
// is GoReleaser's (.goreleaser.yaml, run by tools/bin/pinned), as a snapshot
// that publishes nothing, stamped with that same version: it copies into <out
// dir> what a release publishes, one archive per platform
// (docs/decisions/0022-itos-is-distributed-as-release-archives-with-checksums-installed-pinned.md), the
// config's JSON Schema and checksums.txt, then prints their paths. An archive
// is itos-<version>-<os>-<arch>.tar.gz, or .zip for windows, holding at its top
// level the binary (itos, or itos.exe), LICENSE and README.md. The schema is
// itos.schema.json, written by tools/bin/config-schema from the Go config's
// table, so an editor can check an itos.yaml against the release's
// (`# yaml-language-server: $schema=<its URL>`). checksums.txt is the SHA-256
// of each archive and of the schema in sha256sum's format, so
// `sha256sum -c checksums.txt` checks them all. The release workflow runs the
// same configuration at its tag (.github/workflows/release.yml), so what this
// builds is what a release publishes.
import { spawnSync } from "node:child_process";
import { copyFileSync, mkdirSync, readFileSync } from "node:fs";
import { join, resolve } from "node:path";

export const ROOT = resolve(import.meta.dirname, "../..");
const MODULE = "github.com/donvargax/itos/v6";

// The version this checkout's builds say they are (tools/bin/dev-version).
function devVersion(): string {
	const said = spawnSync(join(ROOT, "tools/bin/dev-version"), [ROOT], { encoding: "utf8" });
	const version = said.stdout?.trim();
	if (said.status !== 0 || !version)
		throw new Error(`tools/bin/dev-version says no version: ${said.stderr ?? said.error?.message}`);
	return version;
}

// Builds ./cmd/itos into `out` for this machine, stamped with the checkout's
// version, and returns the binary's path. Throws when go build fails, its
// output printed.
function build(out: string): string {
	const binary = resolve(out);
	mkdirSync(resolve(binary, ".."), { recursive: true });
	const env: NodeJS.ProcessEnv = { ...process.env, CGO_ENABLED: "0" };
	const ldflags = `-s -w -X ${MODULE}/internal/version.stamp=${devVersion()}`;
	const args = ["build", "-trimpath", "-ldflags", ldflags, "-o", binary, "./cmd/itos"];
	const result = spawnSync("go", args, { cwd: ROOT, env, stdio: "inherit" });
	if (result.status !== 0)
		throw new Error(`go ${args.join(" ")} failed (exit ${result.status ?? result.signal})`);
	return binary;
}

// The binary for this machine, `<dir>/itos`.
export const buildNative = (dir: string) =>
	build(join(dir, process.platform === "win32" ? "itos.exe" : "itos"));

// The config's JSON Schema, as the release publishes it.
export const SCHEMA = "itos.schema.json";

// Writes the config's JSON Schema to `path`, generated from the Go config's
// table by tools/bin/config-schema, and returns the path. Throws when the
// generator fails, its output printed.
export function buildSchema(path: string): string {
	const out = resolve(path);
	mkdirSync(resolve(out, ".."), { recursive: true });
	const args = ["run", "./tools/bin/config-schema", out];
	const result = spawnSync("go", args, { cwd: ROOT, stdio: "inherit" });
	if (result.status !== 0)
		throw new Error(`go ${args.join(" ")} failed (exit ${result.status ?? result.signal})`);
	return out;
}

// What a release publishes, by name: the archives and the schema, as
// checksums.txt lists them, and checksums.txt.
function published(dist: string): string[] {
	const listed = readFileSync(join(dist, "checksums.txt"), "utf8")
		.split("\n")
		.filter(Boolean)
		.map((line) => line.split(/\s+\*?/)[1]!);
	return [...listed, "checksums.txt"];
}

// Builds the release as a GoReleaser snapshot stamped with the checkout's
// version, or ITOS_SNAPSHOT_VERSION when it is set (a self-test building the
// release a version would publish), copies what a release publishes into
// `dir`, and returns their paths, checksums.txt last.
function buildRelease(dir: string): string[] {
	const version = process.env.ITOS_SNAPSHOT_VERSION || devVersion();
	const out = resolve(dir);
	mkdirSync(out, { recursive: true });
	const args = ["goreleaser", "release", "--snapshot", "--clean"];
	const result = spawnSync(join(ROOT, "tools/bin/pinned"), args, {
		cwd: ROOT,
		env: { ...process.env, ITOS_SNAPSHOT_VERSION: version },
		stdio: ["ignore", 2, 2], // GoReleaser's log, off this command's stdout
	});
	if (result.status !== 0)
		throw new Error(
			`tools/bin/pinned ${args.join(" ")} failed (exit ${result.status ?? result.signal})`,
		);
	const dist = join(ROOT, "dist/goreleaser");
	return published(dist).map((name) => {
		const from = name === SCHEMA ? join(ROOT, ".tools/release", SCHEMA) : join(dist, name);
		copyFileSync(from, join(out, name));
		return join(out, name);
	});
}

if (import.meta.main) {
	const release = process.argv[2] === "--release";
	const dir = process.argv[release ? 3 : 2];
	if (!dir) {
		console.error("usage: node tools/bin/build-go.ts [--release] <out dir>");
		process.exit(2);
	}
	try {
		console.log((release ? buildRelease(dir) : [buildNative(dir)]).join("\n"));
	} catch (error) {
		console.error(`build-go: ${(error as Error).message}`);
		process.exit(1);
	}
}
