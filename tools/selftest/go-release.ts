// The Go release, built as the release workflow builds it, works: the
// archives, the config's JSON Schema and checksums.txt GoReleaser writes
// (.goreleaser.yaml) are what the release publishes and the launcher, a pin
// and a consumer's pinned install script download (PLAN.md §10), so they are
// proven before anyone relies on them.
//
//   node tools/selftest/go-release.ts              build the release from the
//                                                  checkout into a scratch
//                                                  folder, through the command
//                                                  line the self-tests call
//                                                  (build-go.ts --release, a
//                                                  GoReleaser snapshot), and
//                                                  prove that
//   node tools/selftest/go-release.ts --dir <dir>  prove a release already
//        [--version <version>]                     built into <dir>, building
//                                                  nothing; with --version,
//                                                  that it is that version's
//
// The version is the one the archives are named with, which every archive
// must share (and --version must be). Either way it reads the folder only with
// the system's own tools, not the code that wrote it:
//
//   - the folder holds one archive per platform of PLATFORMS below, named as
//     the release names them, itos.schema.json and checksums.txt, and nothing
//     else;
//   - every archive lists exactly its binary (itos, or itos.exe), LICENSE and
//     README.md (`tar -tzf`, `unzip -Z1`), and its binary is an executable of
//     its platform's format and architecture;
//   - itos.schema.json is JSON (tools/selftest/go-schema.ts proves what it
//     says);
//   - checksums.txt names every archive and the schema once and passes
//     `sha256sum -c`;
//   - this machine's archive, unpacked with tar, says `itos <version>` for
//     `itos version`.
//
// The Claude Code plugin's version is its own since T-069, so its manifest is
// no longer compared with itos's.
//
// Exits 1 on any failure, or when the folder is empty or missing. The nightly
// runs it without --dir (itos.yaml's `ci.nightly.steps`), so the release build
// cannot rot between releases; tools/selftest/release-cut.ts runs it with
// --dir and --version on the release a version would publish.
import { spawnSync } from "node:child_process";
import { existsSync, mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { ROOT } from "../bin/build-go.ts";
import { outsideEnv } from "./scratch.ts";

// What a release publishes for each platform (PLAN.md §10): the expectation,
// written here rather than read from build-go.ts, so a platform dropped there
// fails here.
const PLATFORMS = ["linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64", "windows-amd64"];

// The version proven, set once the folder is read: the one its archives are
// named with.
let version = "";
const archiveOf = (platform: string) =>
	`itos-${version}-${platform}.${platform.startsWith("windows") ? "zip" : "tar.gz"}`;
const binaryOf = (platform: string) => (platform.startsWith("windows") ? "itos.exe" : "itos");
// The config's JSON Schema, beside the archives.
const SCHEMA = "itos.schema.json";

const args = process.argv.slice(2);
const option = (name: string) => {
	const at = args.indexOf(name);
	return at >= 0 ? (args.splice(at, 2)[1] ?? "") : undefined;
};
const dirArg = option("--dir");
const wanted = option("--version");
if (args.length || dirArg === "" || wanted === "" || (wanted && dirArg === undefined)) {
	console.error("usage: node tools/selftest/go-release.ts [--dir <dir> [--version <version>]]");
	process.exit(2);
}
// The folder proven: the one given, or the one built here.
const given = dirArg === undefined ? undefined : resolve(dirArg);
const scratch = mkdtempSync(join(tmpdir(), "go-release-"));
const out = given ?? join(scratch, "release");
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

// checksums.txt names every archive and the schema once, and sha256sum
// agrees with it.
function proveChecksums(named: string[]) {
	const sums = join(out, "checksums.txt");
	if (!existsSync(sums)) return void failures.push("the release has no checksums.txt");
	const listed = lines(readFileSync(sums)).map((line) => line.split(/\s+\*?/)[1]);
	if (listed.sort().join() !== [...named].sort().join())
		failures.push(`checksums.txt names ${listed.join(", ")}, not ${named.join(", ")}`);
	run(["sha256sum", "-c", "checksums.txt"], out);
}

// This machine's archive, unpacked, says the version its name gives.
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

// The version the folder's archives are named with: this machine's
// platform's name read back, which every other archive must then match.
function versionOf(held: string[]): string {
	const named = held
		.map((name) => /^itos-(.+)-linux-amd64\.tar\.gz$/.exec(name)?.[1])
		.find((v) => v !== undefined);
	if (named === undefined) failures.push("no archive is named itos-<version>-linux-amd64.tar.gz");
	else if (wanted && named !== wanted)
		failures.push(`the archives are of itos ${named}, not ${wanted}`);
	return named ?? wanted ?? "<none>";
}

try {
	if (given) console.log(`== the release built into ${out}`);
	else {
		console.log(`== the release: node tools/bin/build-go.ts --release ${out}`);
		const build = spawnSync(process.execPath, ["tools/bin/build-go.ts", "--release", out], {
			cwd: ROOT,
			stdio: "inherit",
		});
		if (build.status !== 0)
			failures.push(`the release build (exit ${build.status ?? build.signal})`);
	}
	const held = existsSync(out) ? readdirSync(out).sort() : [];
	version = versionOf(held);
	const archives = PLATFORMS.map(archiveOf);
	const expected = [...archives, SCHEMA, "checksums.txt"].sort();
	if (held.length === 0)
		failures.push(given ? `${out} holds nothing` : "the release build wrote nothing");
	else if (held.join() !== expected.join())
		failures.push(`the release holds ${held.join(", ")}, not ${expected.join(", ")}`);
	for (const platform of PLATFORMS) if (held.includes(archiveOf(platform))) proveArchive(platform);
	if (held.includes(SCHEMA)) {
		try {
			JSON.parse(readFileSync(join(out, SCHEMA), "utf8"));
		} catch (error) {
			failures.push(`${SCHEMA} is not JSON: ${(error as Error).message}`);
		}
	}
	if (held.length) proveChecksums([...archives, SCHEMA]);
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
		`LICENSE and README.md, and ${SCHEMA} pass sha256sum -c, and this machine's says its version`,
);
