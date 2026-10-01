// The release tarball, itos-<version>.tgz, packed the one way the release
// workflow and tools/selftest/release.ts both use:
//
//   node tools/bin/pack.ts <destination>
//
// `vp pack` bundles tools/itos/ into dist/itos.mjs (vite.config.ts's `pack`);
// then the files package.json's `files` lists are copied into a scratch folder
// beside a manifest written for consumers, and `npm pack` packs that folder
// into <destination>. Prints the tarball's path.
//
// The manifest is package.json's name, version, type, bin and files, plus
// engines, and nothing else. This repository's package.json is not packed: its
// scripts (prepare: vp config) would be listed on every consumer's install as
// an install script awaiting approval, and would fail without Vite+; its
// devDependencies and packageManager are this repository's own. engines says
// Node 24, the bundle's target, which it needs for import.meta.main.
import { spawnSync } from "node:child_process";
import { cpSync, existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

const ENGINES = { node: ">=24" };

interface Manifest {
	name: string;
	version: string;
	type: string;
	bin: Record<string, string>;
	files: string[];
}

const root = resolve(".");
const destination = process.argv[2];
if (!destination) {
	console.error("usage: node tools/bin/pack.ts <destination>");
	process.exit(2);
}

function run(command: string[], cwd: string) {
	const result = spawnSync(command[0]!, command.slice(1), { cwd, stdio: "inherit" });
	if (result.status !== 0) {
		console.error(`pack: ${command.join(" ")} failed (exit ${result.status ?? result.signal})`);
		process.exit(1);
	}
}

const { name, version, type, bin, files } = JSON.parse(
	readFileSync(join(root, "package.json"), "utf8"),
) as Manifest;

run(["vp", "pack"], root);
const stage = mkdtempSync(join(tmpdir(), "itos-pack-"));
for (const file of files) cpSync(join(root, file), join(stage, file), { recursive: true });
const manifest = { name, version, type, bin, files, engines: ENGINES };
writeFileSync(join(stage, "package.json"), `${JSON.stringify(manifest, null, "\t")}\n`);
run(["npm", "pack", "--pack-destination", resolve(destination)], stage);
rmSync(stage, { recursive: true, force: true });

const tarball = resolve(destination, `${name}-${version}.tgz`);
if (!existsSync(tarball)) {
	console.error(`pack: npm pack left no ${tarball}`);
	process.exit(1);
}
console.log(tarball);
