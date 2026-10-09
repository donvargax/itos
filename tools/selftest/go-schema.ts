// The config's JSON Schema says what config check says (T-054): the schema
// a Go release publishes beside its archives, itos.schema.json, which an
// editor checks an itos.yaml against as it is written.
//
//   node tools/selftest/go-schema.ts
//
// Generates it into a scratch folder the way the release build does
// (build-go.ts's buildSchema, tools/bin/config-schema from the Go config's
// table), compiles it with ajv as JSON Schema draft 2020-12 in strict mode,
// and holds it to config check:
//
//   - this repository's itos.yaml passes;
//   - every config the corpus's config.yaml calls sound (config check exits 0,
//     or 1 for the ledger's, the registry's or the smoke set's problems)
//     passes, and so every config the schema refuses there is one config
//     check refuses (exit 2): the schema may say less than config check, never
//     something different;
//   - every config of every other corpus file the schema refuses is one its
//     case expects itos to refuse (exit 2);
//   - every corpus config config check refuses for what the schema itself
//     says (an unknown key, a wrong type, a value not allowed, a missing key)
//     the schema refuses too;
//   - this repository's itos.yaml with a misspelt key, a wrong type or a
//     value not allowed is refused, each for that reason, and so is it by
//     config check.
//
// Exits 1 on any failure, or when it validated nothing.
import { spawnSync } from "node:child_process";
import { mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import Ajv2020, { type ErrorObject } from "ajv/dist/2020.js";
import { parse } from "yaml";
import { buildSchema, ROOT, SCHEMA } from "../bin/build-go.ts";
import { repoProgram } from "../bin/repo-program.ts";
import { outsideEnv } from "./scratch.ts";

const DRAFT = "https://json-schema.org/draft/2020-12/schema";
const CORPUS = "tools/itos/conformance";

// config check's rules for what the schema itself says, and the words its
// text output gives them.
const SCHEMA_RULES = ["config-type", "config-enum", "config-unknown-key", "config-missing-key"];
const SCHEMA_WORDS = [/: unknown key /, /: \S+ should be /];

const scratch = mkdtempSync(join(tmpdir(), "go-schema-"));
const failures: string[] = [];
let sound = 0;
let refused = 0;
let corpusConfigs = 0;

interface Case {
	name: string;
	argv?: string[];
	cwd?: string;
	files?: Record<string, string | null>;
	exit: number;
	json?: { problems?: { rule: string }[] };
	stderr?: string;
}

const errorsOf = (errors: ErrorObject[] | null | undefined) =>
	(errors ?? []).map((e) => `${e.instancePath || "/"} ${e.message ?? e.keyword}`).join("; ");

try {
	console.log(`== the schema: go run ./tools/bin/config-schema ${join(scratch, SCHEMA)}`);
	const text = readFileSync(buildSchema(join(scratch, SCHEMA)), "utf8");
	const schema = JSON.parse(text) as Record<string, unknown>;
	if (schema.$schema !== DRAFT) failures.push(`the schema's $schema is ${String(schema.$schema)}`);
	const validate = new Ajv2020({ strict: true, allErrors: true }).compile(schema);

	// The repository's own config.
	const own = parse(readFileSync(join(ROOT, "itos.yaml"), "utf8")) as Record<string, any>;
	if (validate(own)) sound++;
	else failures.push(`this repository's itos.yaml is refused: ${errorsOf(validate.errors)}`);

	// The corpus's configs: config.yaml's by config check's verdict on each,
	// every other file's by its case's exit, 2 for a config itos refuses.
	for (const name of readdirSync(join(ROOT, CORPUS))
		.filter((f) => f.endsWith(".yaml"))
		.sort()) {
		const corpus = parse(readFileSync(join(ROOT, CORPUS, name), "utf8")) as {
			files?: Record<string, string>;
			cases?: Case[];
		};
		for (const c of corpus.cases ?? []) {
			const argv = c.argv ?? [];
			const at = argv.indexOf("--config");
			const file = (c.cwd ? `${c.cwd}/` : "") + (at >= 0 ? argv[at + 1]! : "itos.yaml");
			const written = c.files && file in c.files ? c.files[file] : corpus.files?.[file];
			if (typeof written !== "string") continue;
			// What the runner fills in: a commit's full SHA, or text.
			const filled = written
				.replaceAll(/\{\{sha\.[\w-]+\}\}/g, "0".repeat(40))
				.replaceAll(/\{\{[\w.-]+\}\}/g, "x");
			const config: unknown = parse(filled);
			corpusConfigs++;
			const passes = validate(config);
			const problems = errorsOf(validate.errors);
			const where = `${name}: "${c.name}"`;
			const configCheck =
				name === "config.yaml" &&
				argv[0] === "config" &&
				argv[1] === "check" &&
				!argv.includes("--print-defaults");
			if (passes) {
				if (configCheck && c.exit !== 2) sound++;
				// Refused by config check for what the schema itself says.
				const bySchema =
					c.json?.problems?.some((p) => SCHEMA_RULES.includes(p.rule)) ??
					SCHEMA_WORDS.some((words) => words.test(c.stderr ?? ""));
				if (configCheck && c.exit === 2 && bySchema)
					failures.push(`${where}: refused by config check for what the schema says, passes`);
				continue;
			}
			refused++;
			if (configCheck && c.exit !== 2)
				failures.push(`${where}: sound to config check, refused: ${problems}`);
			else if (c.exit !== 2)
				failures.push(`${where}: exits ${c.exit}, not 2, and the schema refuses it: ${problems}`);
		}
	}

	// This repository's config, each time with one thing wrong.
	const copy = () => structuredClone(own);
	const wrong: { what: string; config: Record<string, any>; keyword: string; path: string }[] = [
		{
			what: "a misspelt key, ci.cost.keep_writen_order",
			config: (() => {
				const c = copy();
				c.ci.cost.keep_writen_order = c.ci.cost.keep_written_order;
				delete c.ci.cost.keep_written_order;
				return c;
			})(),
			keyword: "additionalProperties",
			path: "/ci/cost",
		},
		{
			what: 'a wrong type, version: "1"',
			config: { ...copy(), version: "1" },
			keyword: "type",
			path: "/version",
		},
		{
			what: "a value not allowed, commits.header_lint.use: commitlint",
			config: (() => {
				const c = copy();
				c.commits.header_lint = { ...c.commits.header_lint, use: "commitlint" };
				return c;
			})(),
			keyword: "enum",
			path: "/commits/header_lint/use",
		},
	];
	const env = outsideEnv();
	for (const [i, w] of wrong.entries()) {
		if (validate(w.config)) {
			failures.push(`${w.what} passes`);
			continue;
		}
		refused++;
		if (!validate.errors?.some((e) => e.keyword === w.keyword && e.instancePath === w.path))
			failures.push(
				`${w.what} is refused, not by ${w.keyword} at ${w.path}: ${errorsOf(validate.errors)}`,
			);
		const file = join(scratch, `wrong-${i}.yaml`);
		writeFileSync(file, JSON.stringify(w.config));
		const [program, argv] = repoProgram("tools/bin/itos", [
			"config",
			"check",
			"-q",
			"--config",
			file,
		]);
		const check = spawnSync(program, argv, {
			cwd: ROOT,
			env,
			encoding: "utf8",
		});
		if (check.status !== 2)
			failures.push(`${w.what}: config check exits ${check.status}, not 2: ${check.stderr}`);
	}
} catch (error) {
	failures.push((error as Error).message);
} finally {
	rmSync(scratch, { recursive: true, force: true });
}

// The repository's own config is one sound config; the corpus must give more.
if (corpusConfigs === 0 || sound < 2 || refused === 0)
	failures.push("the schema validated nothing");
if (failures.length) {
	console.error(`\ngo schema: FAIL\n${failures.map((f) => `  - ${f}`).join("\n")}`);
	process.exit(1);
}
console.log(
	`\ngo schema: ${SCHEMA} (JSON Schema 2020-12) passes this repository's itos.yaml and the ` +
		`${sound - 1} configs config check calls sound, and refuses ${refused} configs, each one ` +
		`itos refuses too (${corpusConfigs} corpus configs in all)`,
);
