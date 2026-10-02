// The Go release, built as the release workflow will build it, works: the
// archives and checksums.txt `node tools/bin/build-go.ts --release` writes are
// what a consumer's pinned install script downloads (PLAN.md §10), so they are
// proven before the first Go release ever publishes them.
//
//   node tools/selftest/go-release.ts
//
// Builds the release from the checkout into a scratch folder, through the
// command line the workflow will call, and then reads it only with the system's
// own tools, not the code that wrote it:
//
//   - the folder holds one archive per platform of PLATFORMS below, named as
//     the release names them, and checksums.txt, and nothing else;
//   - every archive lists exactly its binary (itos, or itos.exe), LICENSE and
//     README.md (`tar -tzf`, `unzip -Z1`), and its binary is an executable of
//     its platform's format and architecture;
//   - checksums.txt names every archive once and passes `sha256sum -c`;
//   - this machine's archive, unpacked with tar, says `itos <package.json's
//     version>` for `itos version`.
//
// Exits 1 on any failure, or when it built nothing. The nightly runs it
// (itos.yaml's `ci.nightly.steps`), so the release build cannot rot before the
// first Go release.
import { spawnSync } from "node:child_process";
import { existsSync, mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { ROOT } from "../bin/build-go.ts";
import { outsideEnv } from "./scratch.ts";

// What a release publishes for each platform (PLAN.md §10): the expectation,
// written here rather than read from build-go.ts, so a platform dropped there
// fails here.
const PLATFORMS = ["linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64", "windows-amd64"];

const { version } = JSON.parse(readFileSync(join(ROOT, "package.json"), "utf8")) as {
	version: string;
};
const archiveOf = (platform: string) =>
	`itos-${version}-${platform}.${platform.startsWith("windows") ? "zip" : "tar.gz"}`;
const binaryOf = (platform: string) => (platform.startsWith("windows") ? "itos.exe" : "itos");

const scratch = mkdtempSync(join(tmpdir(), "go-release-"));
const out = join(scratch, "release");
// Inside a hook git exports GIT_DIR and friends; nothing here is that repository.
const env = outsideEnv();
const failures: string[] = [];

// Runs a command and returns its output as bytes, or undefined (the failure
// recorded) when it fails or cannot be started.
function run(command: string[], cwd = ROOT): Buffer | undefined {
	const result = spawnSync(command[0]!, command.slice(1), {
		cwd,
		env,
		maxBuffer: 256 * 1024 * 1024,
	});
	if (result.status === 0) return result.stdout;
	const why = result.error?.message ?? `exit ${result.status ?? result.signal}`;
	failures.push(`${command.join(" ")} failed (${why}): ${result.stderr?.toString().trim() ?? ""}`);
	return undefined;
}

const lines = (output: Buffer | undefined) =>
	output?.toString().split("\n").filter(Boolean).sort() ?? [];

// An executable's format and architecture, read from its first bytes: ELF's
// e_machine, Mach-O's cputype, PE's machine.
const ELF: Record<number, string> = { 0x3e: "linux-amd64", 0xb7: "linux-arm64" };
const MACHO: Record<number, string> = { 0x01000007: "darwin-amd64", 0x0100000c: "darwin-arm64" };
const PE: Record<number, string> = { 0x8664: "windows-amd64", 0xaa64: "windows-arm64" };
function platformOf(binary: Buffer): string | undefined {
	if (binary.subarray(0, 4).toString("latin1") === "\x7fELF") return ELF[binary.readUInt16LE(18)];
	if (binary.readUInt32LE(0) === 0xfeedfacf) return MACHO[binary.readUInt32LE(4)];
	if (binary.subarray(0, 2).toString("latin1") !== "MZ") return undefined;
	const pe = binary.readUInt32LE(0x3c);
	if (binary.subarray(pe, pe + 4).toString("latin1") !== "PE\0\0") return undefined;
	return PE[binary.readUInt16LE(pe + 4)];
}

// One archive: its files, and its binary's platform.
function proveArchive(platform: string) {
	const archive = join(out, archiveOf(platform));
	const zip = archive.endsWith(".zip");
	const listed = lines(run(zip ? ["unzip", "-Z1", archive] : ["tar", "-tzf", archive]));
	const expected = [binaryOf(platform), "LICENSE", "README.md"].sort();
	if (listed.join() !== expected.join())
		failures.push(`${archiveOf(platform)} holds ${listed.join(", ")}, not ${expected.join(", ")}`);
	const binary = run(
		zip ? ["unzip", "-p", archive, binaryOf(platform)] : ["tar", "-xzOf", archive, "itos"],
	);
	const built = binary && platformOf(binary);
	if (binary && built !== platform)
		failures.push(`${archiveOf(platform)}'s binary is built for ${built ?? "no known platform"}`);
}

// checksums.txt names every archive once, and sha256sum agrees with it.
function proveChecksums(archives: string[]) {
	const sums = join(out, "checksums.txt");
	if (!existsSync(sums)) return void failures.push("the release has no checksums.txt");
	const named = lines(readFileSync(sums)).map((line) => line.split(/\s+\*?/)[1]);
	if (named.sort().join() !== [...archives].sort().join())
		failures.push(`checksums.txt names ${named.join(", ")}, not ${archives.join(", ")}`);
	run(["sha256sum", "-c", "checksums.txt"], out);
}

// This machine's archive, unpacked, says package.json's version.
function proveNative() {
	const goarch: Record<string, string> = { x64: "amd64", arm64: "arm64" };
	const platform = `${process.platform}-${goarch[process.arch] ?? process.arch}`;
	if (!PLATFORMS.includes(platform))
		return void failures.push(
			`no archive is for this machine (${process.platform}/${process.arch})`,
		);
	const unpacked = join(scratch, "native");
	mkdirSync(unpacked);
	if (!run(["tar", "-xzf", join(out, archiveOf(platform)), "-C", unpacked])) return;
	// From the checkout, whose itos.yaml `itos version` reads.
	const said = run([join(unpacked, "itos"), "version"])
		?.toString()
		.trim();
	if (said !== undefined && said !== `itos ${version}`)
		failures.push(
			`${archiveOf(platform)}'s itos says ${JSON.stringify(said)}, not itos ${version}`,
		);
}

try {
	console.log(`== the release: node tools/bin/build-go.ts --release ${out}`);
	const build = spawnSync(process.execPath, ["tools/bin/build-go.ts", "--release", out], {
		cwd: ROOT,
		stdio: "inherit",
	});
	if (build.status !== 0) failures.push(`the release build (exit ${build.status ?? build.signal})`);
	const archives = PLATFORMS.map(archiveOf);
	const held = existsSync(out) ? readdirSync(out).sort() : [];
	const expected = [...archives, "checksums.txt"].sort();
	if (held.length === 0) failures.push("the release build wrote nothing");
	else if (held.join() !== expected.join())
		failures.push(`the release holds ${held.join(", ")}, not ${expected.join(", ")}`);
	for (const platform of PLATFORMS) if (held.includes(archiveOf(platform))) proveArchive(platform);
	if (held.length) proveChecksums(archives);
	if (held.length) proveNative();
} finally {
	rmSync(scratch, { recursive: true, force: true });
}

if (failures.length) {
	console.error(`\ngo release: FAIL\n${failures.map((f) => `  - ${f}`).join("\n")}`);
	process.exit(1);
}
console.log(
	`\ngo release: ${PLATFORMS.length} archives of itos ${version}, each holding its binary, ` +
		"LICENSE and README.md, pass sha256sum -c, and this machine's says its version",
);
