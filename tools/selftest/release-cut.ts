// The release CI cuts (T-069) computes the right version and publishes what
// the launcher reads, proven without publishing anything:
//
//   1. The version, tools/bin/release-version, over scratch histories: since
//      the newest vX.Y.Z tag HEAD reaches, a feat makes a minor, a fix (scoped
//      or not) a patch, a feat and a fix a minor, and a breaking change a major,
//      whether a BREAKING-CHANGE: or BREAKING CHANGE: footer, a feat! or a
//      refactor!; docs, ci and test commits release nothing; a feat or a !
//      before the tag does not count, nor a BREAKING-CHANGE line outside the
//      last paragraph; a tag HEAD does not reach and a pre-release tag are not
//      the last release, and tags compare by version (v1.10.0 after v1.9.0);
//      with no tag a feat is 0.1.0; a shallow clone stops it (exit 2).
//   2. The release a version publishes, built as a GoReleaser snapshot of this
//      tree (tools/bin/build-go.ts --release, .goreleaser.yaml) stamped with
//      the version the commits since the last tag would release (the next
//      patch when they release nothing): tools/selftest/go-release.ts --dir
//      proves the folder (the five archives, their names, what each holds, each
//      binary's platform, itos.schema.json, checksums.txt and sha256sum -c,
//      this machine's binary saying that version), each archive's name in
//      checksums.txt gives the launcher that version (internal/launch's
//      archiveLine), and it is held to what the last release published: the
//      same names but for the version, the same entries with the same modes and
//      owners in the linux-amd64 and windows-amd64 archives, the same line
//      format in checksums.txt.
//   3. The notes tools/bin/release-notes writes for this tree's range and that
//      snapshot's checksums.txt, printed, and proven by
//      tools/selftest/release-notes.ts --notes: Upgrading last, the install
//      lines, every Upgrading: footer and Changes: entry, every default changed.
//
// It needs the network (GoReleaser and git-cliff through tools/bin/pinned,
// the last release's assets through gh) and Go.
//
//   node tools/selftest/release-cut.ts
import { spawnSync } from "node:child_process";
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { finish, outsideEnv, releaseRepos } from "./scratch.ts";

const root = resolve(".");
const tmp = mkdtempSync(join(tmpdir(), "release-cut-selftest-"));
const problems: string[] = [];
const expect = (ok: boolean, problem: string) => ok || problems.push(problem);
const env = outsideEnv();

function run(command: string, args: string[], cwd = root, extra: NodeJS.ProcessEnv = {}) {
	const r = spawnSync(command, args, {
		cwd,
		env: { ...env, ...extra },
		encoding: "utf8",
		maxBuffer: 64 * 1024 * 1024,
	});
	return {
		status: r.status ?? 1,
		stdout: r.stdout ?? "",
		stderr: r.stderr ?? r.error?.message ?? "",
	};
}

// The key=value lines tools/bin/release-version prints.
const fields = (stdout: string) =>
	Object.fromEntries(
		stdout
			.split("\n")
			.filter((line) => line.includes("="))
			.map((line) => [line.slice(0, line.indexOf("=")), line.slice(line.indexOf("=") + 1)]),
	) as Record<string, string>;

// 1. The version over scratch histories.
function versions() {
	const tool = join(tmp, "release-version");
	const built = run("go", ["build", "-o", tool, "./tools/bin/release-version"]);
	if (built.status !== 0)
		return void problems.push(`go build ./tools/bin/release-version: ${built.stderr}`);
	const { git, commit, untagged, shallow } = releaseRepos(tmp);
	let n = 0;
	// A repository with a v1.2.3 tag on its first commit, the history given
	// before it (`before`) and after it.
	const history = (after: string[], before: string[] = []) => {
		const dir = join(tmp, `history-${++n}`);
		git(tmp, "init", "-q", "-b", "main", dir);
		commit(dir, "chore: the first");
		for (const m of before) commit(dir, m);
		git(dir, "tag", "v1.2.3");
		for (const m of after) commit(dir, m);
		return dir;
	};
	const cases: { name: string; dir: () => string; next: string; bump: string }[] = [
		{ name: "a feat", dir: () => history(["feat: a thing"]), next: "1.3.0", bump: "minor" },
		{ name: "a fix", dir: () => history(["fix: a bug"]), next: "1.2.4", bump: "patch" },
		{ name: "a scoped fix", dir: () => history(["fix(cli): a bug"]), next: "1.2.4", bump: "patch" },
		{
			name: "a feat and a fix",
			dir: () => history(["fix: a bug", "feat: a thing", "docs: say so"]),
			next: "1.3.0",
			bump: "minor",
		},
		{
			name: "a BREAKING-CHANGE: footer",
			dir: () => history(["feat: a thing", "fix: a bug\n\nWhy.\n\nBREAKING-CHANGE: a key goes"]),
			next: "2.0.0",
			bump: "major",
		},
		{
			name: "a BREAKING CHANGE: footer",
			dir: () => history(["fix: a bug\n\nBREAKING CHANGE: a key goes\nTask: T-001"]),
			next: "2.0.0",
			bump: "major",
		},
		{
			name: "a feat! header",
			dir: () => history(["feat!: a thing"]),
			next: "2.0.0",
			bump: "major",
		},
		{
			name: "a refactor! header",
			dir: () => history(["refactor(config)!: a key goes"]),
			next: "2.0.0",
			bump: "major",
		},
		{
			name: "docs, ci and test commits",
			dir: () => history(["docs: say so", "ci: run it", "test: prove it", "build: pin it"]),
			next: "",
			bump: "none",
		},
		{
			name: "a feat and a ! before the tag, docs after",
			dir: () => history(["docs: say so"], ["feat: a thing", "fix!: a bug"]),
			next: "",
			bump: "none",
		},
		{
			name: "a BREAKING-CHANGE line outside the last paragraph",
			dir: () => history(["fix: a bug\n\nBREAKING-CHANGE: not a footer here\n\nTask: T-001"]),
			next: "1.2.4",
			bump: "patch",
		},
		{
			name: "a tag HEAD does not reach, and a pre-release tag",
			dir: () => {
				const dir = history(["fix: a bug"]);
				git(dir, "tag", "v1.3.0-rc1");
				git(dir, "checkout", "-q", "-b", "side", "v1.2.3");
				commit(dir, "feat: elsewhere");
				git(dir, "tag", "v9.0.0");
				git(dir, "checkout", "-q", "main");
				return dir;
			},
			next: "1.2.4",
			bump: "patch",
		},
		{
			name: "tags compared by version",
			dir: () => {
				const dir = history(["feat: a thing"]);
				git(dir, "tag", "v1.9.0");
				commit(dir, "fix: a bug");
				git(dir, "tag", "v1.10.0");
				commit(dir, "fix: another");
				return dir;
			},
			next: "1.10.1",
			bump: "patch",
		},
		{ name: "no tag", dir: untagged, next: "0.1.0", bump: "minor" },
	];
	for (const c of cases) {
		const r = run(tool, [], c.dir());
		const got = fields(r.stdout);
		expect(
			r.status === 0 && got.next === c.next && got.bump === c.bump,
			`release-version over ${c.name}: exit ${r.status}, next=${got.next} bump=${got.bump}, not next=${c.next} bump=${c.bump}\n${r.stderr}`,
		);
	}
	const shallowRun = run(tool, [], shallow(history(["feat: a thing"])));
	expect(
		shallowRun.status === 2 && shallowRun.stderr.includes("shallow"),
		`release-version in a shallow clone: exit ${shallowRun.status}, not 2 saying so\n${shallowRun.stderr}`,
	);
	console.log(`== release-version: ${cases.length} histories and a shallow clone`);
}

// The version this tree's release would carry, the last release's tag, and
// the next patch when nothing is releasable.
function thisTree(): { version: string; last: string } {
	const r = run("go", ["run", "./tools/bin/release-version"]);
	const got = fields(r.stdout);
	if (r.status !== 0 || !got.last)
		throw new Error(`release-version on this tree: exit ${r.status}\n${r.stderr}`);
	if (got.next) return { version: got.next, last: got.last };
	const [major, minor, patch] = got.last.slice(1).split(".").map(Number);
	return { version: `${major}.${minor}.${patch! + 1}`, last: got.last };
}

// internal/launch's archiveLine: an archive's name in a release's
// checksums.txt, read for the version the launcher fetches.
const ARCHIVE_LINE =
	/^itos-(\d+\.\d+\.\d+(?:-[0-9A-Za-z.]+)?)-[0-9a-z]+-[0-9a-z]+\.(?:tar\.gz|zip)$/;
const SUM_LINE = /^[0-9a-f]{64} {2}\S+$/;

// An archive's entries as a consumer's tar or unzip lists them: each name with
// its mode, and for a tarball its owner by number; sizes and dates left out.
function entries(archive: string): string[] {
	if (archive.endsWith(".zip")) {
		const r = run("unzip", ["-Z", archive]);
		return r.stdout
			.split("\n")
			.filter((line) => /^[-d]/.test(line))
			.map((line) => {
				const words = line.trim().split(/\s+/);
				return `${words[0]} ${words.at(-1)}`;
			})
			.sort();
	}
	const r = run("tar", ["--numeric-owner", "-tvzf", archive]);
	return r.stdout
		.split("\n")
		.filter(Boolean)
		.map((line) => {
			const words = line.trim().split(/\s+/);
			return `${words[0]} ${words[1]} ${words.at(-1)}`;
		})
		.sort();
}

// 2. The snapshot, proven and held to the last release.
function snapshot(version: string, last: string): string | undefined {
	const out = join(tmp, "release");
	console.log(
		`== GoReleaser snapshot of this tree as ${version}: node tools/bin/build-go.ts --release`,
	);
	const built = spawnSync(process.execPath, ["tools/bin/build-go.ts", "--release", out], {
		cwd: root,
		env: { ...env, ITOS_SNAPSHOT_VERSION: version },
		stdio: ["ignore", "ignore", "inherit"],
	});
	if (built.status !== 0)
		return void problems.push(`the snapshot build (exit ${built.status ?? built.signal})`);
	const proven = spawnSync(
		process.execPath,
		["tools/selftest/go-release.ts", "--dir", out, "--version", version],
		{ cwd: root, env, stdio: "inherit" },
	);
	expect(
		proven.status === 0,
		`go-release.ts --dir ${out} --version ${version} (exit ${proven.status})`,
	);

	const sums = readFileSync(join(out, "checksums.txt"), "utf8").split("\n").filter(Boolean);
	for (const line of sums) {
		expect(SUM_LINE.test(line), `checksums.txt's line is not "<sha256>  <name>": ${line}`);
		const name = line.split(/\s+/)[1]!;
		if (name === "itos.schema.json") continue;
		const read = ARCHIVE_LINE.exec(name)?.[1];
		expect(
			read === version,
			`the launcher reads ${name} as itos ${read ?? "nothing"}, not ${version}`,
		);
	}

	// What the last release published, its version put in place of its own.
	const was = join(tmp, last);
	mkdirSync(was);
	const v = last.slice(1);
	const got = run("gh", [
		"release",
		"download",
		last,
		"--dir",
		was,
		"--pattern",
		"checksums.txt",
		"--pattern",
		`itos-${v}-linux-amd64.tar.gz`,
		"--pattern",
		`itos-${v}-windows-amd64.zip`,
	]);
	if (got.status !== 0) return void problems.push(`gh release download ${last}: ${got.stderr}`);
	const wasSums = readFileSync(join(was, "checksums.txt"), "utf8").split("\n").filter(Boolean);
	for (const line of wasSums) expect(SUM_LINE.test(line), `${last}'s checksums.txt line: ${line}`);
	const names = (lines: string[], of: string) =>
		lines
			.map((line) => line.split(/\s+/)[1]!.replace(`itos-${of}-`, "itos-<version>-"))
			.sort()
			.join(", ");
	expect(
		names(sums, version) === names(wasSums, v),
		`the snapshot publishes ${names(sums, version)}, ${last} published ${names(wasSums, v)}`,
	);
	for (const platform of ["linux-amd64.tar.gz", "windows-amd64.zip"]) {
		const now = entries(join(out, `itos-${version}-${platform}`));
		const then = entries(join(was, `itos-${v}-${platform}`));
		expect(
			now.length > 0 && now.join("\n") === then.join("\n"),
			`the ${platform} archive holds\n    ${now.join("\n    ")}\n  where ${last}'s held\n    ${then.join("\n    ")}`,
		);
	}
	console.log(
		`== the snapshot's assets are ${last}'s, but for the version: ${names(sums, version)}`,
	);
	return join(out, "checksums.txt");
}

// 3. The notes, written and proven.
function notes(version: string, last: string, checksums: string) {
	const file = join(tmp, "notes.md");
	const r = run("go", [
		"run",
		"./tools/bin/release-notes",
		"-version",
		version,
		"-from",
		last,
		"-checksums",
		checksums,
	]);
	if (r.status !== 0)
		return void problems.push(`tools/bin/release-notes (exit ${r.status}): ${r.stderr}`);
	writeFileSync(file, r.stdout);
	console.log(`== the notes tools/bin/release-notes writes for ${last}..HEAD as ${version}:\n`);
	console.log(r.stdout);
	const proven = spawnSync(
		process.execPath,
		["tools/selftest/release-notes.ts", version, "--notes", file],
		{
			cwd: root,
			env,
			stdio: "inherit",
		},
	);
	expect(proven.status === 0, `release-notes.ts ${version} --notes (exit ${proven.status})`);
}

try {
	versions();
	const { version, last } = thisTree();
	const checksums = snapshot(version, last);
	if (checksums) notes(version, last, checksums);
} catch (error) {
	problems.push((error as Error).message);
} finally {
	rmSync(tmp, { recursive: true, force: true });
}
finish(
	problems,
	"release cut",
	"release cut: the version follows the commits since the last tag, the snapshot publishes what the last release did but for its version, and its notes pass the notes check",
);
