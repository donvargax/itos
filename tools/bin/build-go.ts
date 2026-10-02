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
// The release build, the one the release workflow will upload at v1.0.0:
//
//   node tools/bin/build-go.ts --release <out dir>
//
// runs that same build once per platform a release publishes (PLAN.md §10)
// and writes into <out dir> one archive per platform, the config's JSON Schema
// and checksums.txt, then prints their paths. An archive is
// itos-<version>-<os>-<arch>.tar.gz, or .zip for windows, holding at its top
// level the binary (itos, or itos.exe), LICENSE and README.md. The schema is
// itos.schema.json, written by tools/bin/config-schema from the Go config's
// table, so an editor can check an itos.yaml against the release's
// (`# yaml-language-server: $schema=<its URL>`). checksums.txt is the SHA-256
// of each archive and of the schema in sha256sum's format, so
// `sha256sum -c checksums.txt` checks them all; it and the files it names are
// what the release publishes. The archives are written
// here, with Node's zlib, not by a tar or zip on the machine: the same bytes on
// every machine, the binary executable once unpacked, and no tool a runner
// might lack. Every entry's timestamp is HEAD's commit time, so a commit
// rebuilt by the same Go toolchain gives the same archives.
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { crc32, deflateRawSync, gzipSync } from "node:zlib";

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

// The platforms a release publishes (PLAN.md §10).
const PLATFORMS: Target[] = [
	{ goos: "linux", goarch: "amd64" },
	{ goos: "linux", goarch: "arm64" },
	{ goos: "darwin", goarch: "amd64" },
	{ goos: "darwin", goarch: "arm64" },
	{ goos: "windows", goarch: "amd64" },
];

// A file an archive holds: its name at the archive's top level, its bytes and
// its Unix mode.
interface Entry {
	name: string;
	data: Buffer;
	mode: number;
}

// HEAD's commit time, in seconds: every entry's timestamp.
function commitTime(): number {
	const shown = spawnSync("git", ["log", "-1", "--format=%ct"], { cwd: ROOT, encoding: "utf8" });
	const seconds = Number(shown.stdout?.trim());
	if (shown.status !== 0 || !Number.isInteger(seconds) || seconds <= 0)
		throw new Error(`git log -1 --format=%ct gave no commit time: ${shown.stderr ?? ""}`);
	return seconds;
}

// A ustar header field: `value` in octal, zero-padded to fill `width` less its
// terminating NUL.
const octal = (value: number, width: number) => `${value.toString(8).padStart(width - 1, "0")}\0`;

// One ustar header block; owner and group are root's, by number.
function tarHeader(entry: Entry, mtime: number): Buffer {
	const header = Buffer.alloc(512);
	header.write(entry.name, 0);
	header.write(octal(entry.mode, 8), 100);
	header.write(octal(0, 8), 108);
	header.write(octal(0, 8), 116);
	header.write(octal(entry.data.length, 12), 124);
	header.write(octal(mtime, 12), 136);
	header.write("        ", 148); // the checksum's field counts as spaces
	header.write("0", 156); // a regular file
	header.write("ustar\u000000", 257);
	const sum = header.reduce((total, byte) => total + byte, 0);
	header.write(`${sum.toString(8).padStart(6, "0")}\0 `, 148);
	return header;
}

// A gzipped ustar archive of the entries.
function tarGz(entries: Entry[], mtime: number): Buffer {
	const blocks = entries.flatMap((entry) => [
		tarHeader(entry, mtime),
		entry.data,
		Buffer.alloc((512 - (entry.data.length % 512)) % 512),
	]);
	const gz = gzipSync(Buffer.concat([...blocks, Buffer.alloc(1024)]), { level: 9 });
	gz[9] = 3; // the header's OS byte, which zlib sets by the machine: Unix, on every one
	return gz;
}

// A Unix time as MS-DOS's date and time, the only timestamp a zip entry needs.
function dosTime(seconds: number) {
	const t = new Date(seconds * 1000);
	return {
		date: ((t.getUTCFullYear() - 1980) << 9) | ((t.getUTCMonth() + 1) << 5) | t.getUTCDate(),
		time: (t.getUTCHours() << 11) | (t.getUTCMinutes() << 5) | (t.getUTCSeconds() >> 1),
	};
}

// A zip archive of the entries, each deflated, made by Unix (so unzip keeps
// their modes) with no extra fields.
function zip(entries: Entry[], mtime: number): Buffer {
	const { date, time } = dosTime(mtime);
	const locals: Buffer[] = [];
	const centrals: Buffer[] = [];
	let offset = 0;
	for (const entry of entries) {
		const name = Buffer.from(entry.name);
		const packed = deflateRawSync(entry.data, { level: 9 });
		// The fields a local header and its central record share, from "version needed".
		const shared = Buffer.alloc(26);
		shared.writeUInt16LE(20, 0); // version needed: deflate
		shared.writeUInt16LE(8, 4); // method: deflate
		shared.writeUInt16LE(time, 6);
		shared.writeUInt16LE(date, 8);
		shared.writeUInt32LE(crc32(entry.data), 10);
		shared.writeUInt32LE(packed.length, 14);
		shared.writeUInt32LE(entry.data.length, 18);
		shared.writeUInt16LE(name.length, 22);
		const local = Buffer.concat([u32(0x04034b50), shared, name, packed]);
		const central = Buffer.alloc(46);
		central.writeUInt32LE(0x02014b50, 0);
		central.writeUInt16LE((3 << 8) | 20, 4); // made by Unix, zip 2.0
		shared.copy(central, 6);
		central.writeUInt32LE(((0o100000 | entry.mode) << 16) >>> 0, 38);
		central.writeUInt32LE(offset, 42);
		locals.push(local);
		centrals.push(central, name);
		offset += local.length;
	}
	const directory = Buffer.concat(centrals);
	const end = Buffer.alloc(22);
	end.writeUInt32LE(0x06054b50, 0);
	end.writeUInt16LE(entries.length, 8);
	end.writeUInt16LE(entries.length, 10);
	end.writeUInt32LE(directory.length, 12);
	end.writeUInt32LE(offset, 16);
	return Buffer.concat([...locals, directory, end]);
}

const u32 = (value: number) => {
	const bytes = Buffer.alloc(4);
	bytes.writeUInt32LE(value);
	return bytes;
};

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

const sha256 = (data: Buffer) => createHash("sha256").update(data).digest("hex");

// The archive's name, as the release publishes it.
const archiveName = ({ goos, goarch }: Target) =>
	`itos-${VERSION}-${goos}-${goarch}.${goos === "windows" ? "zip" : "tar.gz"}`;

// Builds every platform's archive, the schema and checksums.txt into `dir`,
// and returns their paths, checksums.txt last.
export function buildRelease(dir: string): string[] {
	const out = resolve(dir);
	mkdirSync(out, { recursive: true });
	const mtime = commitTime();
	const docs = ["LICENSE", "README.md"].map((name) => ({
		name,
		data: readFileSync(join(ROOT, name)),
		mode: 0o644,
	}));
	const stage = mkdtempSync(join(tmpdir(), "itos-release-"));
	const sums: string[] = [];
	const paths: string[] = [];
	try {
		for (const target of PLATFORMS) {
			const exe = target.goos === "windows" ? "itos.exe" : "itos";
			const binary = build(join(stage, `${target.goos}-${target.goarch}`, exe), target);
			const entries = [{ name: exe, data: readFileSync(binary), mode: 0o755 }, ...docs];
			const archive = target.goos === "windows" ? zip(entries, mtime) : tarGz(entries, mtime);
			const name = archiveName(target);
			writeFileSync(join(out, name), archive);
			sums.push(`${sha256(archive)}  ${name}\n`);
			paths.push(join(out, name));
		}
	} finally {
		rmSync(stage, { recursive: true, force: true });
	}
	const schema = buildSchema(join(out, SCHEMA));
	sums.push(`${sha256(readFileSync(schema))}  ${SCHEMA}\n`);
	paths.push(schema);
	writeFileSync(join(out, "checksums.txt"), sums.join(""));
	return [...paths, join(out, "checksums.txt")];
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
