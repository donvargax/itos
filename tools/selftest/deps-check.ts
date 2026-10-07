// The dependency check (T-067, tools/bin/deps-check) against a module proxy of
// local files, with no network: a scratch module requires modules the proxy
// says were published a day ago and a month ago, and the check is run on it.
//
//   - a module a day old is refused, naming it and how to except it, and
//     govulncheck is not run;
//   - excepted with a reason, it passes;
//   - a module a month old passes;
//   - an exception that excuses nothing (a version not in the build, or one
//     now old enough) fails, and one without a reason is refused;
//   - a govulncheck finding (its exit status 3) fails the check;
//   - -changed-since checks nothing when go.mod and go.sum did not change.
//
// govulncheck itself needs the network (the vulnerability database, and its
// own source on a cold module cache), so here it is a stub the check is told
// to run, which proves what the check does with its answer; the real one runs
// wherever the check does, on every commit that stages go.mod or go.sum.
// Every proxy variable points at a closed port, so any request that tried the
// network would fail.
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { existsSync, mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { outsideEnv, realGit } from "./scratch.ts";

const DAY = 24 * 60 * 60 * 1000;
const tmp = mkdtempSync(join(tmpdir(), "deps-check-selftest-"));
const proxy = join(tmp, "proxy");
const app = join(tmp, "app");
const marker = join(tmp, "govulncheck-ran");
const bin = join(tmp, "deps-check");
const problems: string[] = [];
const expect = (ok: boolean, problem: string) => ok || problems.push(problem);

const closed = "http://127.0.0.1:9";
const env: NodeJS.ProcessEnv = {
	...outsideEnv(),
	GOPROXY: `file://${proxy}`,
	GOSUMDB: "off",
	GONOPROXY: "",
	GOPRIVATE: "",
	GONOSUMDB: "",
	GOMODCACHE: join(tmp, "modcache"),
	GOFLAGS: "-modcacherw",
	GOTOOLCHAIN: "local",
	GOWORK: "off",
	HTTP_PROXY: closed,
	HTTPS_PROXY: closed,
	http_proxy: closed,
	https_proxy: closed,
	NO_PROXY: "",
	no_proxy: "",
};

const run = (cmd: string, args: string[], cwd: string, extra: NodeJS.ProcessEnv = {}) => {
	const r = spawnSync(cmd, args, { cwd, env: { ...env, ...extra }, encoding: "utf8" });
	return { status: r.status, output: `${r.stdout}${r.stderr}` };
};

// A module version the proxy serves: its .info, published `ago` milliseconds
// before now, and its go.mod, whose hash go.sum holds (dirhash's h1 over the
// one file go.mod).
const modFile = (path: string) => `module ${path}\n\ngo 1.21\n`;
const sum = (path: string, version: string) => {
	const fileHash = createHash("sha256").update(modFile(path)).digest("hex");
	const h1 = createHash("sha256").update(`${fileHash}  go.mod\n`).digest("base64");
	return `${path} ${version}/go.mod h1:${h1}\n`;
};
const serve = (path: string, version: string, ago: number) => {
	const dir = join(proxy, path, "@v");
	mkdirSync(dir, { recursive: true });
	const time = new Date(Date.now() - ago).toISOString();
	writeFileSync(join(dir, "list"), `${version}\n`);
	writeFileSync(join(dir, `${version}.info`), JSON.stringify({ Version: version, Time: time }));
	writeFileSync(join(dir, `${version}.mod`), modFile(path));
};

const YOUNG = "example.com/young";
const OLD = "example.com/old";
serve(YOUNG, "v1.0.0", DAY);
serve(OLD, "v1.0.0", 30 * DAY);

// The scratch module, requiring `requires` at v1.0.0, with these exceptions.
const setUp = (requires: string[], exceptions?: object[]) => {
	rmSync(join(app, "deps-check.json"), { force: true });
	rmSync(marker, { force: true });
	const lines = requires.map((r) => `\t${r} v1.0.0\n`).join("");
	writeFileSync(join(app, "go.mod"), `module example.com/app\n\ngo 1.21\n\nrequire (\n${lines})\n`);
	writeFileSync(join(app, "go.sum"), requires.map((r) => sum(r, "v1.0.0")).join(""));
	if (exceptions)
		writeFileSync(join(app, "deps-check.json"), JSON.stringify({ exceptions }, null, "\t"));
};
const stub = join(tmp, "govulncheck.sh");
writeFileSync(stub, `touch '${marker}'\necho 'stub govulncheck'\nexit "\${STUB_EXIT:-0}"\n`);
const check = (args: string[] = [], vulnExit = 0) =>
	run(bin, ["-govulncheck", `sh ${stub}`, ...args], app, { STUB_EXIT: String(vulnExit) });
const reason = "the fix for a vulnerability that cannot wait a week";

try {
	mkdirSync(app, { recursive: true });
	const built = spawnSync("go", ["build", "-o", bin, "./tools/bin/deps-check"], {
		cwd: resolve("."),
		env: outsideEnv(),
		encoding: "utf8",
	});
	if (built.status !== 0) throw new Error(`go build ./tools/bin/deps-check:\n${built.stderr}`);

	// 1. A module a day old is refused, named with its version and how to
	// except it, and govulncheck is not run.
	setUp([YOUNG, OLD]);
	let r = check();
	expect(
		r.status === 1,
		`a module a day old should be refused (exit 1), exited ${r.status}:\n${r.output}`,
	);
	expect(
		r.output.includes(`${YOUNG} v1.0.0 was published`) && r.output.includes("deps-check.json"),
		`the refusal should name ${YOUNG} v1.0.0 and the exceptions file:\n${r.output}`,
	);
	expect(!r.output.includes(OLD), `a module a month old should not be named:\n${r.output}`);
	expect(!existsSync(marker), "govulncheck ran though a module was refused");

	// 2. Excepted with a reason, it passes, and govulncheck runs.
	setUp([YOUNG, OLD], [{ module: YOUNG, version: "v1.0.0", reason }]);
	r = check();
	expect(
		r.status === 0,
		`an excepted module a day old should pass, exited ${r.status}:\n${r.output}`,
	);
	expect(r.output.includes(reason), `the pass should print the exception's reason:\n${r.output}`);
	expect(existsSync(marker), "govulncheck did not run once every module had passed");

	// 3. A module a month old passes with no exception.
	setUp([OLD]);
	r = check();
	expect(r.status === 0, `a module a month old should pass, exited ${r.status}:\n${r.output}`);
	expect(
		r.output.includes(`${OLD} v1.0.0`),
		`the pass should name the newest module:\n${r.output}`,
	);

	// 4. An exception that excuses nothing fails: its version is not in the
	// build, or it is old enough without it.
	setUp([OLD], [{ module: YOUNG, version: "v1.0.0", reason }]);
	r = check();
	expect(
		r.status === 1 && r.output.includes("names a version not in the build"),
		`an exception for a version not in the build should fail, exited ${r.status}:\n${r.output}`,
	);
	setUp([OLD], [{ module: OLD, version: "v1.0.0", reason }]);
	r = check();
	expect(
		r.status === 1 && r.output.includes("no longer needed"),
		`an exception for a module old enough should fail, exited ${r.status}:\n${r.output}`,
	);

	// 5. An exception without a reason is refused.
	setUp([YOUNG], [{ module: YOUNG, version: "v1.0.0", reason: "" }]);
	r = check();
	expect(
		r.status === 2 && r.output.includes("reason"),
		`an exception without a reason should be refused (exit 2), exited ${r.status}:\n${r.output}`,
	);

	// 6. A govulncheck finding fails the check.
	setUp([OLD]);
	r = check([], 3);
	expect(
		r.status === 1 && r.output.includes("govulncheck found"),
		`a govulncheck finding should fail the check, exited ${r.status}:\n${r.output}`,
	);

	// 7. -changed-since: nothing to check when go.mod and go.sum did not change
	// since the revision, the whole check when they did.
	setUp([OLD]);
	const git = (...args: string[]) => run(realGit(), args, app);
	git("init", "-q");
	git("add", "go.mod", "go.sum");
	git("-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "old");
	setUp([YOUNG, OLD]);
	git("add", "go.mod", "go.sum");
	git("-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "young");
	r = check(["-changed-since", "HEAD"]);
	expect(
		r.status === 0 && r.output.includes("nothing to check"),
		`-changed-since HEAD should check nothing, exited ${r.status}:\n${r.output}`,
	);
	r = check(["-changed-since", "HEAD~1"]);
	expect(
		r.status === 1 && r.output.includes(`${YOUNG} v1.0.0`),
		`-changed-since HEAD~1 should check and refuse ${YOUNG}, exited ${r.status}:\n${r.output}`,
	);
} finally {
	rmSync(tmp, { recursive: true, force: true });
}

for (const problem of problems) console.error(`FAIL ${problem}`);
console.log(
	problems.length
		? `\n${problems.length} dependency check(s) failed`
		: "\nThe dependency check refuses a module a day old unless excepted, passes an old one, and fails on a govulncheck finding",
);
process.exit(problems.length ? 1 : 0);
