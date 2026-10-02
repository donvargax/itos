// tools/bin/itos, which the hooks, CI's steps, the ledger's checks and the
// nightly call (itos.yaml's hooks.bin), runs the Go binary and needs no Node
// (T-060). Run in a scratch worktree of the current tree (uncommitted edits
// included, scratch.ts), with a PATH that holds go, git and sh but no node:
//
//   - `version`, `config check` and `task list` run on this repository, the
//     first call building the binary into .tools/bin/itos, stamped with
//     package.json's version;
//   - a call with nothing changed builds nothing;
//   - a Go source added, a Go source removed and package.json's version changed
//     each rebuild it, and the next call runs the new build;
//   - a Go source that does not compile fails the call with exit 3 and one
//     line naming the build, never by running the old binary.
//
//   node tools/selftest/go-dogfood.ts
import { spawnSync } from "node:child_process";
import {
	existsSync,
	mkdirSync,
	mkdtempSync,
	readdirSync,
	readFileSync,
	rmSync,
	statSync,
	symlinkSync,
	utimesSync,
	writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { delimiter, join } from "node:path";
import { scratchRepo } from "./scratch.ts";

const repo = scratchRepo("go-dogfood-selftest");
const { dir, env } = repo;
const links = mkdtempSync(join(tmpdir(), "go-dogfood-path-"));
const problems: string[] = [];
const expect = (ok: boolean, problem: string) => {
	if (!ok) problems.push(problem);
};

// The PATH less Node: each folder that holds node is replaced by a folder of
// links to everything else in it, as the corpus runner's `hide` does.
const HIDDEN = ["node", "nodejs"];
const noNode = (env.PATH ?? "")
	.split(delimiter)
	.filter(Boolean)
	.map((folder, i) => {
		if (!HIDDEN.some((name) => existsSync(join(folder, name)))) return folder;
		const copy = join(links, `path-${i}`);
		mkdirSync(copy);
		for (const name of readdirSync(folder))
			if (!HIDDEN.includes(name)) symlinkSync(join(folder, name), join(copy, name));
		return copy;
	})
	.join(delimiter);
const runEnv = { ...env, PATH: noNode };

const found = (command: string) =>
	spawnSync("/bin/sh", ["-c", `command -v ${command}`], { env: runEnv }).status === 0;

const itos = (...args: string[]) => {
	const run = spawnSync(join(dir, "tools/bin/itos"), args, {
		cwd: dir,
		env: runEnv,
		encoding: "utf8",
	});
	return {
		status: run.status ?? 1,
		stdout: run.stdout ?? "",
		stderr: run.stderr ?? `${run.error?.message ?? ""}\n`,
	};
};
const binary = () => join(dir, ".tools/bin/itos");
const built = () => (existsSync(binary()) ? statSync(binary()).mtimeMs : 0);
// The binary as if built a minute ago, so the next edit is newer whatever the
// file system's clock resolution.
const age = () => {
	const then = new Date(Date.now() - 60_000);
	if (existsSync(binary())) utimesSync(binary(), then, then);
};

const version = () =>
	(JSON.parse(readFileSync(join(dir, "package.json"), "utf8")) as { version: string }).version;

try {
	repo.open();
	expect(!found("node"), `the PATH built without node still finds it: ${noNode}`);
	for (const tool of ["go", "git", "sh"])
		expect(found(tool), `the PATH built without node lost ${tool}`);
	expect(!existsSync(binary()), "the scratch copy already holds .tools/bin/itos");

	// 1. The commands, on this repository, with no Node.
	let run = itos("version");
	expect(
		run.status === 0 && run.stdout === `itos ${version()}\n`,
		`tools/bin/itos version with no node: exit ${run.status}, ${JSON.stringify(run.stdout)}\n${run.stderr}`,
	);
	expect(built() > 0, "the first call left no binary at .tools/bin/itos");
	run = itos("config", "check");
	expect(
		run.status === 0,
		`tools/bin/itos config check with no node: exit ${run.status}\n${run.stderr}`,
	);
	run = itos("task", "list");
	expect(
		run.status === 0 && run.stdout.includes("T-060"),
		`tools/bin/itos task list with no node: exit ${run.status}\n${run.stdout}${run.stderr}`,
	);

	// 2. Nothing changed, nothing built.
	let before = built();
	run = itos("version");
	expect(run.status === 0 && built() === before, "a call with nothing changed rebuilt the binary");

	// 3. A Go source added: rebuilt, and the new build is what runs.
	const marker = "go-dogfood: the new build";
	const source = join(dir, "cmd/itos/dogfood.go");
	age();
	before = built();
	writeFileSync(
		source,
		`package main\n\nimport "os"\n\nfunc init() { os.Stderr.WriteString("${marker}\\n") }\n`,
	);
	run = itos("version");
	expect(
		run.status === 0 && built() !== before && run.stderr.includes(marker),
		`a Go source added did not rebuild the binary: exit ${run.status}\n${run.stderr}`,
	);

	// 4. A Go source removed: rebuilt without it.
	age();
	before = built();
	rmSync(source);
	run = itos("version");
	expect(
		run.status === 0 && built() !== before && !run.stderr.includes(marker),
		`a Go source removed did not rebuild the binary: exit ${run.status}\n${run.stderr}`,
	);

	// 5. package.json's version changed: rebuilt, stamped with it.
	age();
	const packageJson = join(dir, "package.json");
	const original = readFileSync(packageJson, "utf8");
	writeFileSync(packageJson, original.replace(`"version": "${version()}"`, '"version": "9.9.9"'));
	run = itos("version");
	expect(
		run.status === 0 && run.stdout === "itos 9.9.9\n",
		`a new version in package.json was not stamped: exit ${run.status}, ${JSON.stringify(run.stdout)}\n${run.stderr}`,
	);
	writeFileSync(packageJson, original);

	// 6. A Go source that does not compile: exit 3 and one line naming the
	// build, the old binary not run.
	const broken = join(dir, "cmd/itos/broken.go");
	writeFileSync(broken, "package main\n\nfunc broken( {}\n");
	run = itos("version");
	const first = run.stderr.split("\n")[0] ?? "";
	expect(
		run.status === 3 &&
			run.stdout === "" &&
			first.startsWith("itos: the Go binary does not build (go build ./cmd/itos): "),
		`a Go source that does not compile: exit ${run.status}, stdout ${JSON.stringify(run.stdout)}\n${run.stderr}`,
	);
	rmSync(broken);
	run = itos("version");
	expect(
		run.status === 0 && run.stdout === `itos ${version()}\n`,
		`the build fixed did not run again: exit ${run.status}\n${run.stderr}`,
	);
} finally {
	repo.remove();
	rmSync(links, { recursive: true, force: true });
}

for (const problem of problems) console.error(`FAIL ${problem}`);
if (problems.length) {
	console.error(`\n${problems.length} dogfood check(s) failed`);
	process.exit(1);
}
console.log(
	"go dogfood: tools/bin/itos runs the Go binary with no node, rebuilt when its sources change",
);
