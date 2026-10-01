// The release tarball works as a consumer installs it. It proves the tarball,
// not the source: the conformance corpus and every feature run against the
// itos the tarball installs, so a file the bundle misses, a dependency it
// leaves out or a path resolved beside the source fails here.
//
//   node tools/selftest/release.ts                    pack (`vp pack`, then
//                                                     `npm pack`) and prove that
//   node tools/selftest/release.ts --tarball <file>   prove a tarball already
//                                                     packed (the release
//                                                     workflow's, the one it
//                                                     publishes)
//
// The tarball is installed into a scratch project with no network (`npm
// install --offline` with an empty cache), so a runtime dependency cannot
// come from anywhere; the scratch project must then hold itos and nothing
// else. Exits 1 on any failure.
import { spawnSync } from "node:child_process";
import { existsSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { outsideEnv } from "./scratch.ts";

const root = resolve(".");
const { version } = JSON.parse(readFileSync(join(root, "package.json"), "utf8")) as {
	version: string;
};
const scratch = mkdtempSync(join(tmpdir(), "release-selftest-"));
// Inside a hook git exports GIT_DIR and friends; nothing here is that repository.
const env = outsideEnv();

const failures: string[] = [];
function run(
	what: string,
	command: string[],
	options: { cwd?: string; env?: NodeJS.ProcessEnv } = {},
) {
	console.log(`\n== ${what}: ${command.join(" ")}`);
	const result = spawnSync(command[0]!, command.slice(1), {
		cwd: options.cwd ?? root,
		env: { ...env, ...options.env },
		stdio: "inherit",
	});
	if (result.status !== 0) failures.push(`${what} (exit ${result.status ?? result.signal})`);
	return result.status === 0;
}

function tarballOf(args: string[]): string | undefined {
	const at = args.indexOf("--tarball");
	if (at >= 0) return resolve(args[at + 1] ?? "");
	if (!run("bundle", ["vp", "pack"])) return undefined;
	if (!run("pack", ["npm", "pack", "--ignore-scripts", "--pack-destination", scratch]))
		return undefined;
	return join(scratch, `itos-${version}.tgz`);
}

function prove(tarball: string) {
	if (!existsSync(tarball)) return void failures.push(`no tarball at ${tarball}`);
	const project = join(scratch, "consumer");
	spawnSync("mkdir", ["-p", project]);
	writeFileSync(join(project, "package.json"), '{ "name": "consumer", "private": true }\n');
	const install = [
		"npm",
		"install",
		"--offline",
		"--cache",
		join(scratch, "npm-cache"),
		"--no-audit",
		"--no-fund",
		tarball,
	];
	if (!run("install, offline", install, { cwd: project })) return;

	const installed = readdirSync(join(project, "node_modules")).filter(
		(name) => !name.startsWith("."),
	);
	if (installed.join() !== "itos")
		failures.push(`the install holds more than itos: ${installed.join(", ")}`);

	const bin = join(project, "node_modules", ".bin", "itos");
	const said = spawnSync(bin, ["version"], { encoding: "utf8", env }).stdout?.trim();
	if (said !== `itos ${version}`)
		failures.push(`the installed itos says ${JSON.stringify(said)}, package.json ${version}`);

	run("conformance corpus", ["node", "tools/itos/conformance/run.ts", "--bin", bin]);
	run("every feature", ["go", "test", "./features", "-count=1"], { env: { ITOS_BIN: bin } });
}

const tarball = tarballOf(process.argv.slice(2));
if (tarball) prove(tarball);
rmSync(scratch, { recursive: true, force: true });

if (failures.length) {
	console.error(`\nrelease self-test: ${failures.length} failed`);
	for (const f of failures) console.error(`  - ${f}`);
	process.exit(1);
}
console.log(
	`\nrelease self-test: itos-${version}.tgz installs offline and passes the corpus and every feature`,
);
