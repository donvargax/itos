// The schema contract (T-070, tools/bin/schema-contract) on schemas this
// test writes itself, in a scratch repository with a release tag, and no
// network:
//
//   - each breaking change (a key removed, a type narrowed, an enum value
//     removed, a key newly required, an object closed, a default changed) is
//     refused, naming the key, the change and the remedy, when no commit since
//     the tag marks one as breaking; a ! before the tag does not count;
//   - each passes with a commit since the tag that carries a BREAKING-CHANGE:
//     footer, and with one whose header has a !;
//   - each compatible change (a key added, a type widened, an enum value
//     added) passes, and is printed;
//   - -range-from checks nothing when the range has no feat or fix;
//   - the release's schema is downloaded from <release-url>/<tag>/ (here a
//     local server), and a download that fails (a 404, a closed port) stops
//     the check with exit 2, never a pass; no release tag passes, saying so; a
//     shallow clone stops it.
//
// Every proxy variable points at a closed port, so nothing reaches the network.
import { spawnSync } from "node:child_process";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { finish, outsideEnv, releaseRepos, spawnOutput } from "./scratch.ts";

const tmp = mkdtempSync(join(tmpdir(), "schema-contract-selftest-"));
const repo = join(tmp, "repo");
const bin = join(tmp, "schema-contract");
const problems: string[] = [];
const expect = (ok: boolean, problem: string) => ok || problems.push(problem);

const closed = "http://127.0.0.1:9";
const { env, git, commit, untagged, shallow } = releaseRepos(tmp, {
	GITHUB_REPOSITORY: "example/itos",
	HTTP_PROXY: closed,
	HTTPS_PROXY: closed,
	http_proxy: closed,
	https_proxy: closed,
	NO_PROXY: "",
	no_proxy: "",
});

// The release's schema, in the shape tools/bin/config-schema writes.
type Schema = Record<string, any>;
const release: Schema = {
	$schema: "https://json-schema.org/draft/2020-12/schema",
	title: "itos.yaml",
	type: "object",
	properties: {
		version: { description: "The config's format: 1.", type: "number" },
		shell: { type: "array", items: { type: "string" }, default: ["sh", "-c"] },
		hooks: {
			type: "object",
			properties: {
				manager: { type: "string", enum: ["husky", "lefthook", "vite-hooks"] },
				bin: { type: "string", default: "tools/bin/itos" },
				timeout: { anyOf: [{ type: "string" }, { type: "number" }] },
			},
			required: ["manager"],
			additionalProperties: false,
		},
		tests: {
			type: "object",
			additionalProperties: {
				type: "object",
				properties: { root: { type: "string", default: "features" } },
				additionalProperties: false,
			},
		},
		notes: { type: "object", properties: { text: { type: "string" } } },
	},
	required: ["version"],
	additionalProperties: false,
};
const changed = (edit: (s: Schema) => void) => {
	const s = structuredClone(release);
	edit(s);
	return s;
};

// Each change, the key the check must name and words its change must hold.
const breaking: { what: string; schema: Schema; key: string; words: string }[] = [
	{
		what: "a key removed",
		schema: changed((s) => delete s.properties.hooks.properties.bin),
		key: "hooks.bin",
		words: "key removed",
	},
	{
		what: "a type narrowed",
		schema: changed((s) => (s.properties.hooks.properties.timeout = { type: "string" })),
		key: "hooks.timeout",
		words: "type narrowed: a number is no longer accepted",
	},
	{
		what: "an enum value removed",
		schema: changed((s) => s.properties.hooks.properties.manager.enum.shift()),
		key: "hooks.manager",
		words: 'enum value "husky" removed',
	},
	{
		what: "a key newly required",
		schema: changed((s) => s.properties.hooks.required.push("bin")),
		key: "hooks.bin",
		words: "key newly required",
	},
	{
		what: "an object closed",
		schema: changed((s) => (s.properties.notes.additionalProperties = false)),
		key: "notes",
		words: "object closed",
	},
	{
		what: "a default changed",
		schema: changed((s) => (s.properties.shell.default = ["bash", "-c"])),
		key: "shell",
		words: 'default changed from ["sh","-c"] to ["bash","-c"]',
	},
	{
		what: "a default changed in a map's entries",
		schema: changed(
			(s) => (s.properties.tests.additionalProperties.properties.root.default = "spec"),
		),
		key: "tests.<key>.root",
		words: 'default changed from "features" to "spec"',
	},
];
const compatible: { what: string; schema: Schema; key: string; words: string }[] = [
	{
		what: "a key added",
		schema: changed((s) => (s.properties.hooks.properties.color = { type: "boolean" })),
		key: "hooks.color",
		words: "key added",
	},
	{
		what: "a type widened",
		schema: changed(
			(s) => (s.properties.version = { anyOf: [{ type: "number" }, { type: "string" }] }),
		),
		key: "version",
		words: "type widened: a string is now accepted",
	},
	{
		what: "an enum value added",
		schema: changed((s) => s.properties.hooks.properties.manager.enum.push("overcommit")),
		key: "hooks.manager",
		words: 'enum value "overcommit" added',
	},
];

const write = (name: string, schema: Schema) => {
	const file = join(tmp, name);
	writeFileSync(file, `${JSON.stringify(schema, null, 2)}\n`);
	return file;
};
const releaseFile = write("release.json", release);
const check = (cwd: string, args: string[]) => {
	const r = spawnSync(bin, args, { cwd, env, encoding: "utf8" });
	return { status: r.status, output: `${r.stdout}${r.stderr}` };
};

try {
	const built = spawnSync("go", ["build", "-o", bin, "./tools/bin/schema-contract"], {
		cwd: resolve("."),
		env: outsideEnv(),
		encoding: "utf8",
	});
	if (built.status !== 0) throw new Error(`go build ./tools/bin/schema-contract:\n${built.stderr}`);

	// The scratch history: a breaking change before the release, the release's
	// tag, then a feat that marks nothing; branches add a commit that does.
	git(tmp, "init", "-q", "-b", "main", repo);
	commit(repo, "feat!: an old break, before the release");
	commit(repo, "build: version 1.0.0");
	git(repo, "tag", "v1.0.0");
	const released = git(repo, "rev-parse", "HEAD");
	commit(repo, "feat: change the config\n\nNo breaking change said.\n\nTask: T-1");
	const feat = git(repo, "rev-parse", "HEAD");
	git(repo, "checkout", "-q", "-b", "footer");
	commit(repo, "feat: change it again\n\nBody.\n\nBREAKING-CHANGE: rename hooks.bin in itos.yaml");
	git(repo, "checkout", "-q", "-b", "bang", feat);
	commit(repo, "fix!: change it again");
	git(repo, "checkout", "-q", "-b", "docs", released);
	commit(repo, "docs: say something");
	git(repo, "checkout", "-q", "main");
	const on = (branch: string) => git(repo, "checkout", "-q", branch);

	// 1. No change: passes, saying so.
	let r = check(repo, ["-old", releaseFile, "-new", releaseFile]);
	expect(
		r.status === 0 && r.output.includes("no change"),
		`the release's own schema should pass with no change, exited ${r.status}:\n${r.output}`,
	);

	for (const [i, c] of breaking.entries()) {
		const file = write(`breaking-${i}.json`, c.schema);
		// 2. Refused with no breaking-change commit since the tag, naming the
		// key, the change and the remedy.
		on("main");
		r = check(repo, ["-old", releaseFile, "-new", file]);
		expect(
			r.status === 1,
			`${c.what} with no breaking-change commit should be refused (exit 1), exited ${r.status}:\n${r.output}`,
		);
		expect(
			r.output.includes(`${c.key}: ${c.words}`) &&
				r.output.includes("BREAKING-CHANGE:") &&
				r.output.includes("v1.0.0"),
			`${c.what}: the refusal should name ${c.key}, "${c.words}", the release and the remedy:\n${r.output}`,
		);
		// 3. Passed with a BREAKING-CHANGE footer, and with a ! in a header.
		for (const branch of ["footer", "bang"]) {
			on(branch);
			r = check(repo, ["-old", releaseFile, "-new", file]);
			expect(
				r.status === 0 &&
					r.output.includes(`${c.key}: ${c.words}`) &&
					r.output.includes("marked by"),
				`${c.what} with a breaking-change commit (${branch}) should pass, naming it, exited ${r.status}:\n${r.output}`,
			);
		}
	}

	// 4. Each compatible change passes with no breaking-change commit, printed.
	on("main");
	for (const [i, c] of compatible.entries()) {
		r = check(repo, ["-old", releaseFile, "-new", write(`compatible-${i}.json`, c.schema)]);
		expect(
			r.status === 0 && r.output.includes(`compatible since v1.0.0: ${c.key}: ${c.words}`),
			`${c.what} should pass and be printed, exited ${r.status}:\n${r.output}`,
		);
	}

	// 5. -range-from: a range with no feat or fix checks nothing; one with a
	// feat checks.
	const removed = join(tmp, "breaking-0.json");
	on("docs");
	r = check(repo, ["-range-from", released, "-old", releaseFile, "-new", removed]);
	expect(
		r.status === 0 && r.output.includes("no feat or fix"),
		`a range of a docs commit should check nothing, exited ${r.status}:\n${r.output}`,
	);
	on("main");
	r = check(repo, ["-range-from", released, "-old", releaseFile, "-new", removed]);
	expect(
		r.status === 1 && r.output.includes("hooks.bin: key removed"),
		`a range with a feat should check, and refuse a key removed, exited ${r.status}:\n${r.output}`,
	);

	// 6. A schema keyword the comparison does not read stops it.
	r = check(repo, [
		"-old",
		releaseFile,
		"-new",
		write(
			"pattern.json",
			changed((s) => (s.properties.hooks.properties.bin.pattern = "^t")),
		),
	]);
	expect(
		r.status === 2 && r.output.includes('"pattern"'),
		`a keyword the comparison does not read should stop it (exit 2), exited ${r.status}:\n${r.output}`,
	);

	// 7. The download: the release's schema served at <url>/v1.0.0/itos.schema.json
	// is compared; a 404 and a closed port stop the check with exit 2.
	const server = createServer((req, res) => {
		if (req.url === "/download/v1.0.0/itos.schema.json") {
			res.writeHead(200, { "content-type": "application/json" });
			res.end(JSON.stringify(release));
		} else {
			res.writeHead(404);
			res.end("Not Found");
		}
	});
	await new Promise<void>((done) => server.listen(0, "127.0.0.1", done));
	const port = (server.address() as AddressInfo).port;
	const fetched = (url: string) =>
		spawnOutput(bin, ["-release-url", url, "-new", removed], { cwd: repo, env, input: "" });
	let d = await fetched(`http://127.0.0.1:${port}/download`);
	expect(
		d.code === 1 && d.output.includes("hooks.bin: key removed"),
		`the downloaded schema should be compared, and a key removed refused, exited ${d.code}:\n${d.output}`,
	);
	d = await fetched(`http://127.0.0.1:${port}/elsewhere`);
	expect(
		d.code === 2 && d.output.includes("404"),
		`a release without the schema (404) should stop the check (exit 2), exited ${d.code}:\n${d.output}`,
	);
	server.close();
	d = await fetched(closed);
	expect(
		d.code === 2 && d.output.includes("cannot download"),
		`no network should stop the check (exit 2), exited ${d.code}:\n${d.output}`,
	);

	// 8. No release tag: nothing to hold the tree to, said, and passed.
	r = check(untagged(), ["-release-url", closed, "-new", removed]);
	expect(
		r.status === 0 && r.output.includes("no vX.Y.Z tag"),
		`a repository with no release tag should pass, saying so, exited ${r.status}:\n${r.output}`,
	);

	// 9. A shallow clone, which may hide the tag, stops it.
	r = check(shallow(repo), ["-old", releaseFile, "-new", removed]);
	expect(
		r.status === 2 && r.output.includes("shallow"),
		`a shallow clone should stop the check (exit 2), exited ${r.status}:\n${r.output}`,
	);
} catch (error) {
	problems.push((error as Error).message);
} finally {
	rmSync(tmp, { recursive: true, force: true });
}

finish(
	problems,
	"schema contract",
	`The schema contract refuses each of ${breaking.length} breaking changes unless a commit since the release marks one, passes each of ${compatible.length} compatible ones, and never passes a failed download`,
);
