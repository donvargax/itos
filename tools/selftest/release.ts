// The release tarball works as a consumer installs it. It proves the tarball,
// not the source: the conformance corpus and every feature run against the
// itos the tarball installs, so a file the bundle misses, a dependency it
// leaves out or a path resolved beside the source fails here.
//
//   node tools/selftest/release.ts                    pack (tools/bin/pack.ts, as
//                                                     the release workflow does)
//                                                     and prove that
//   node tools/selftest/release.ts --tarball <file>   prove a tarball already
//                                                     packed (the release
//                                                     workflow's, the one it
//                                                     publishes)
//
// The packed manifest must hold exactly name, version, type, bin, files and
// engines (node >=24): nothing of this repository's package.json a consumer
// does not need, its scripts above all. The tarball is installed into a
// scratch project with no network (`npm install --offline` with an empty
// cache), so a runtime dependency cannot come from anywhere; the scratch
// project must then hold itos and nothing else, and the install must report no
// install script. That report is npm's (`npm warn install-scripts …`), so a
// control package with a prepare script is installed the same way first, and
// an npm that reports nothing for it fails the test rather than passing it.
// Exits 1 on any failure.
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

// The packed manifest's keys, and what engines says.
const MANIFEST_KEYS = ["bin", "engines", "files", "name", "type", "version"];
const ENGINES = { node: ">=24" };

const failures: string[] = [];
// Runs a command, its output printed. With `capture`, `output` is that output,
// for a command whose report is judged.
function run(
	what: string,
	command: string[],
	options: { cwd?: string; env?: NodeJS.ProcessEnv; capture?: boolean } = {},
) {
	console.log(`\n== ${what}: ${command.join(" ")}`);
	const result = spawnSync(command[0]!, command.slice(1), {
		cwd: options.cwd ?? root,
		env: { ...env, ...options.env },
		encoding: "utf8",
		stdio: options.capture ? "pipe" : "inherit",
	});
	const output = `${result.stdout ?? ""}${result.stderr ?? ""}`;
	if (options.capture) process.stdout.write(output);
	if (result.status !== 0) failures.push(`${what} (exit ${result.status ?? result.signal})`);
	return { ok: result.status === 0, output };
}

function tarballOf(args: string[]): string | undefined {
	const at = args.indexOf("--tarball");
	if (at >= 0) return resolve(args[at + 1] ?? "");
	if (!run("pack", ["node", "tools/bin/pack.ts", scratch]).ok) return undefined;
	return join(scratch, `itos-${version}.tgz`);
}

// Installs a tarball, offline, into a new scratch project; the install's output.
function install(what: string, tarball: string, project: string) {
	spawnSync("mkdir", ["-p", project]);
	writeFileSync(join(project, "package.json"), '{ "name": "consumer", "private": true }\n');
	const command = [
		"npm",
		"install",
		"--offline",
		"--cache",
		join(scratch, "npm-cache"),
		"--no-audit",
		"--no-fund",
		tarball,
	];
	return run(what, command, { cwd: project, capture: true });
}

// npm's report of an install script it did not run.
const reportsScripts = (output: string) => /install-scripts?\b/i.test(output);

// The report is npm's; a package with a prepare script shows whether this npm makes one.
function npmReportsScripts(): boolean {
	const control = join(scratch, "control");
	spawnSync("mkdir", ["-p", control]);
	const manifest = { name: "control", version: "0.0.0", scripts: { prepare: "true" } };
	writeFileSync(join(control, "package.json"), JSON.stringify(manifest));
	if (
		!run("control, packed", ["npm", "pack", "--ignore-scripts", "--pack-destination", scratch], {
			cwd: control,
		}).ok
	)
		return false;
	const installed = install(
		"control, installed",
		join(scratch, "control-0.0.0.tgz"),
		join(scratch, "control-consumer"),
	);
	if (installed.ok && reportsScripts(installed.output)) return true;
	failures.push(
		"npm reported no install script for a package with a prepare script, so its report cannot show the tarball has none",
	);
	return false;
}

function proveManifest(tarball: string) {
	const shown = spawnSync("tar", ["-xzOf", tarball, "package/package.json"], { encoding: "utf8" });
	if (shown.status !== 0)
		return void failures.push(`the tarball has no package/package.json: ${shown.stderr}`);
	const manifest = JSON.parse(shown.stdout) as Record<string, unknown>;
	const keys = Object.keys(manifest).sort();
	if (keys.join() !== MANIFEST_KEYS.join())
		failures.push(
			`the packed manifest's keys are ${keys.join(", ")}, not ${MANIFEST_KEYS.join(", ")}`,
		);
	if (JSON.stringify(manifest.engines) !== JSON.stringify(ENGINES))
		failures.push(
			`the packed manifest's engines is ${JSON.stringify(manifest.engines)}, not ${JSON.stringify(ENGINES)}`,
		);
}

function prove(tarball: string) {
	if (!existsSync(tarball)) return void failures.push(`no tarball at ${tarball}`);
	proveManifest(tarball);
	const judged = npmReportsScripts();
	const project = join(scratch, "consumer");
	const installed = install("install, offline", tarball, project);
	if (!installed.ok) return;
	if (judged && reportsScripts(installed.output))
		failures.push("the install reports an install script");

	const held = readdirSync(join(project, "node_modules")).filter((name) => !name.startsWith("."));
	if (held.join() !== "itos") failures.push(`the install holds more than itos: ${held.join(", ")}`);

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
	`\nrelease self-test: itos-${version}.tgz packs only ${MANIFEST_KEYS.join(", ")}, installs ` +
		"offline with no install script and passes the corpus and every feature",
);
