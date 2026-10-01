// The task tooling's config: `itos.yaml` at the root, or the file
// `ITOS_CONFIG` names. Every table the tools read comes from it: the commit
// scopes and path sets, CI's steps, the prose paths and steps, the static
// command patterns, the checks steps cover, the nightly's steps, the ledger's
// files and the pre-push commands, so that a project changes its policy
// without changing the code.
//
// Validation (`itos config check`): the config, where an unknown key is an
// error that names it, then the ledger: duplicate or malformed IDs, unknown
// types, unknown keys, both or neither of run/fails, and a `cost: static`
// written below a late check of the same task, which written order would run
// late anyway.
//
// A section is optional when loading, so a fixture holds only what it tests;
// a tool that needs one it lacks fails saying so.
import { readdirSync, readFileSync } from "node:fs";
import { basename, dirname, join } from "node:path";
import { parse } from "yaml";
import { messages, problem, type Problem } from "./problem.ts";

export type Cost = "static" | "late";

export interface StepConfig {
	run?: string;
	tests?: string;
	whole?: boolean;
	cost?: Cost;
}
// One footer of `commits.footers` (footers.ts reads them).
export interface FooterConfig {
	source: "ledger" | { tests: string };
	strip_prefix?: string;
	required_for?: string | string[];
	validate_for?: string | string[];
	must_be_live?: boolean;
	read_at?: "commit" | "worktree";
}
// The providers (providers.ts).
export interface RangeConfig {
	provider?: "github" | "command" | "none";
	command?: string;
	github?: { workflow?: string; branch?: string; repository_env?: string; token_env?: string[] };
}
export interface IdentityConfig {
	provider?: "github" | "command" | "none";
	command?: string;
	hint?: string;
}
export interface PeopleConfig {
	source: "all-contributors-md" | "all-contributorsrc" | "yaml";
	file: string;
	login_from?: string;
}
export interface ScopeRule {
	only?: string[];
	never?: string[];
	must_touch?: string[];
}
export interface Config {
	version: number;
	requires?: string;
	shell?: string[];
	ledger?: {
		files: string;
		group?: { label?: string; pattern?: string; numeric?: boolean };
		id?: string;
		check?: { timeout?: number; pushed?: string };
	};
	commits?: {
		types?: string[];
		header_lint?: Record<string, string>;
		footers?: Record<string, FooterConfig>;
		path_sets?: Record<string, string[]>;
		scopes?: Record<string, ScopeRule>;
		reject_message?: string;
	};
	tests?: Record<
		string,
		{
			adapter?: string | { command: string; supports_at?: boolean };
			root?: string;
			id?: string;
			run?: { whole?: string; select?: string };
			smoke?: { file?: string };
		} & Record<string, unknown>
	>;
	ci?: {
		env?: Record<string, string>;
		steps: (string | StepConfig)[];
		prose?: { paths: string[]; steps: string[] };
		cost?: { static?: string[]; keep_written_order?: boolean };
		covers?: { by: string; matches: string }[];
		nightly_only?: string[];
		nightly?: { steps: (string | StepConfig)[] };
		wait_on_status?: string[];
		stop_at_first_failure?: boolean;
		range?: RangeConfig;
	};
	work?: {
		registry?: string;
		groups_key?: string;
		statuses?: string[];
		people?: PeopleConfig;
		identity?: IdentityConfig;
	};
	hooks?: { manager?: string; bin?: string; pre_push?: { per_base: string; whole: string } };
}

// The schema, strict: an object's keys are the ones listed, a map's are free.
type Spec =
	| "string"
	| "number"
	| "boolean"
	| "strings"
	| { enum: string[] }
	| { object: Record<string, Spec>; required?: string[] }
	| { map: Spec }
	| { list: Spec }
	| { either: Spec[] };

const str = "string" as const;
const strs = "strings" as const;
const bool = "boolean" as const;
const cost: Spec = { enum: ["static", "late"] };
const obj = (object: Record<string, Spec>, required: string[] = []): Spec => ({
	object,
	required,
});
const step: Spec = {
	either: [str, obj({ run: str, tests: str, whole: bool, cost })],
};
const typesOrAll: Spec = { either: [str, strs] };
const provider: Spec = { enum: ["github", "command", "none"] };

const SCHEMA: Spec = obj(
	{
		version: "number",
		requires: str,
		shell: strs,
		ledger: obj(
			{
				files: str,
				group: obj({ label: str, pattern: str, numeric: bool }),
				id: str,
				check: obj({ timeout: "number", pushed: str }),
			},
			["files"],
		),
		commits: obj({
			types: strs,
			header_lint: obj({ use: str, hook: str, stdin: str, alongside: str }),
			footers: {
				map: obj(
					{
						source: { either: [str, obj({ tests: str }, ["tests"])] },
						strip_prefix: str,
						required_for: typesOrAll,
						validate_for: typesOrAll,
						must_be_live: bool,
						read_at: { enum: ["commit", "worktree"] },
					},
					["source"],
				),
			},
			path_sets: { map: strs },
			scopes: { map: obj({ only: strs, never: strs, must_touch: strs }) },
			reject_message: str,
		}),
		tests: {
			map: obj({
				// Built in (`gherkin`) or a command that speaks the adapter protocol (tests.ts).
				adapter: { either: [str, obj({ command: str, supports_at: bool }, ["command"])] },
				root: str,
				id: str,
				tag_prefix: str,
				wip_tag: str,
				run: obj({
					whole: str,
					select: str,
					ids_pattern: str,
					join: obj({ each: str, sep: str }),
				}),
				recognize: { list: obj({ command: str, as: str }, ["command", "as"]) },
				smoke: obj({ file: str, every_file: bool, add_hint: str }),
				range_checks: {
					list: obj({ name: str, except_types: strs, staged: str, range: str }, ["name"]),
				},
			}),
		},
		ci: obj(
			{
				env: { map: str },
				steps: { list: step },
				prose: obj({ paths: strs, steps: strs }, ["paths", "steps"]),
				cost: obj({ static: strs, keep_written_order: bool }),
				covers: { list: obj({ by: str, matches: str }, ["by", "matches"]) },
				nightly_only: strs,
				nightly: obj({ steps: { list: step } }, ["steps"]),
				wait_on_status: strs,
				stop_at_first_failure: bool,
				range: obj({
					provider,
					command: str,
					github: obj({ workflow: str, branch: str, repository_env: str, token_env: strs }),
				}),
			},
			["steps"],
		),
		work: obj({
			registry: str,
			groups_key: str,
			statuses: strs,
			people: obj(
				{
					source: { enum: ["all-contributors-md", "all-contributorsrc", "yaml"] },
					file: str,
					login_from: str,
				},
				["source", "file"],
			),
			identity: obj({ provider, command: str, hint: str }),
		}),
		hooks: obj({
			manager: str,
			bin: str,
			pre_push: obj({ per_base: str, whole: str }, ["per_base", "whole"]),
		}),
	},
	["version"],
);

const kind = (v: unknown) => (Array.isArray(v) ? "a list" : v === null ? "null" : typeof v);

// The config's problems carry a rule id and a fix, for `--json`; the sentence
// is what the text output prints.
const wrongType = (at: string, message: string, want: string) =>
	problem("config-type", message, `make ${at} ${want}`);

// What is wrong with a value against a spec, each problem naming its key path.
function problems(value: unknown, spec: Spec, path: string): Problem[] {
	if (typeof spec === "string") return scalarProblems(value, spec, path || "the file");
	if ("enum" in spec)
		return spec.enum.includes(value as string)
			? []
			: [
					problem(
						"config-enum",
						`${path} should be one of ${spec.enum.join(", ")}, not ${JSON.stringify(value)}`,
						`set ${path} to one of ${spec.enum.join(", ")}`,
					),
				];
	if ("either" in spec) {
		const each = spec.either.map((s) => problems(value, s, path));
		return each.some((p) => p.length === 0)
			? []
			: each.reduce((a, b) => (b.length < a.length ? b : a));
	}
	if ("list" in spec)
		return Array.isArray(value)
			? value.flatMap((v, i) => problems(v, spec.list, `${path}[${i}]`))
			: [wrongType(path, `${path} should be a list`, "a list")];
	return mappingProblems(value, spec, path);
}

function scalarProblems(value: unknown, spec: Spec & string, at: string): Problem[] {
	if (spec === "strings")
		return Array.isArray(value) && value.every((v) => typeof v === "string")
			? []
			: [wrongType(at, `${at} should be a list of strings`, "a list of strings")];
	return typeof value === spec
		? []
		: [wrongType(at, `${at} should be a ${spec}, not ${kind(value)}`, `a ${spec}`)];
}

// The edit distance of two keys, for a misspelling's fix.
function distance(a: string, b: string): number {
	let row = Array.from({ length: b.length + 1 }, (_, j) => j);
	for (let i = 1; i <= a.length; i++) {
		const next = [i];
		for (let j = 1; j <= b.length; j++)
			next[j] = Math.min(
				row[j]! + 1,
				next[j - 1]! + 1,
				row[j - 1]! + (a[i - 1] === b[j - 1] ? 0 : 1),
			);
		row = next;
	}
	return row[b.length]!;
}

// An unknown key: renamed to the known key it misspells, else removed.
function unknownKey(at: string, k: string, known: string[]): Problem {
	const near = known
		.map((name) => ({ name, d: distance(k, name) }))
		.filter(({ d }) => d <= 2)
		.sort((a, b) => a.d - b.d)[0];
	return problem(
		"config-unknown-key",
		`unknown key ${at}`,
		near ? `rename ${at} to ${near.name}` : `remove ${at}; the keys here are ${known.join(", ")}`,
	);
}

function mappingProblems(
	value: unknown,
	spec: Extract<Spec, { map: Spec } | { object: Record<string, Spec> }>,
	path: string,
): Problem[] {
	if (!value || typeof value !== "object" || Array.isArray(value))
		return [
			wrongType(
				path || "the file",
				`${path || "the file"} should be a mapping, not ${kind(value)}`,
				"a mapping",
			),
		];
	const entries = Object.entries(value);
	const key = (k: string) => (path ? `${path}.${k}` : k);
	if ("map" in spec) return entries.flatMap(([k, v]) => problems(v, spec.map, key(k)));
	const known = Object.keys(spec.object);
	return [
		...entries.flatMap(([k, v]) =>
			k in spec.object ? problems(v, spec.object[k]!, key(k)) : [unknownKey(key(k), k, known)],
		),
		...(spec.required ?? [])
			.filter((k) => !(k in value))
			.map((k) => problem("config-missing-key", `${key(k)} is missing`, `add ${key(k)}`)),
	];
}

export class ConfigError extends Error {
	readonly file: string;
	readonly problems: string[];
	readonly issues: Problem[];
	constructor(file: string, found: (string | Problem)[]) {
		const issues = found.map((p) => (typeof p === "string" ? problem("config-invalid", p) : p));
		super(`${file}: ${messages(issues).join("; ")}`);
		this.file = file;
		this.issues = issues;
		this.problems = messages(issues);
	}
}

export const configPath = () => process.env.ITOS_CONFIG || "itos.yaml";

const tryRegExp = (source: string, where: string, found: Problem[]) => {
	try {
		new RegExp(source);
	} catch {
		found.push(
			problem(
				"config-regexp",
				`${where} is not a regular expression: ${source}`,
				`correct ${where} so that it compiles as a JavaScript regular expression`,
			),
		);
	}
};

// `$name` entries of a path list, replaced by `commits.path_sets.<name>`.
function expandSets(config: Config, found: Problem[]) {
	const sets = config.commits?.path_sets ?? {};
	const expand = (globs: string[], where: string) =>
		globs.flatMap((glob) => {
			if (!glob.startsWith("$")) return [glob];
			const set = sets[glob.slice(1)];
			if (!set)
				found.push(
					problem(
						"config-path-set",
						`${where} names ${glob}, which commits.path_sets does not have`,
						`add commits.path_sets.${glob.slice(1)}, or remove ${glob} from ${where}`,
					),
				);
			return set ?? [];
		});
	for (const [type, rule] of Object.entries(config.commits?.scopes ?? {}))
		for (const k of ["only", "never", "must_touch"] as const)
			if (rule[k]) rule[k] = expand(rule[k], `commits.scopes.${type}.${k}`);
	if (config.ci?.prose) config.ci.prose.paths = expand(config.ci.prose.paths, "ci.prose.paths");
}

// What the schema cannot say: names that must refer to something, patterns
// that must compile.
const crossProblems = (config: Config): Problem[] => [
	...scopeProblems(config),
	...footerProblems(config),
	...stepProblems(config),
	...patternProblems(config),
	...providerProblems(config),
];

// A `command` provider needs its command, and only the markdown table reads
// logins out of links.
function providerProblems(config: Config): Problem[] {
	const found: Problem[] = [];
	const needsCommand = (where: string, p?: { provider?: string; command?: string }) => {
		if (p?.provider === "command" && !p.command?.trim())
			found.push(
				problem(
					"config-provider-command",
					`${where}.provider is command, and ${where}.command is missing`,
					`add ${where}.command, or choose another ${where}.provider`,
				),
			);
	};
	needsCommand("ci.range", config.ci?.range);
	needsCommand("work.identity", config.work?.identity);
	const people = config.work?.people;
	if (people?.login_from !== undefined) {
		if (people.source !== "all-contributors-md")
			found.push(
				problem(
					"config-login-from",
					`work.people.login_from is read only by all-contributors-md`,
					"remove work.people.login_from",
				),
			);
		else if (people.login_from.split("{login}").length !== 2)
			found.push(
				problem(
					"config-login-from",
					`work.people.login_from needs one {login}: ${people.login_from}`,
					"write {login} once in work.people.login_from, where the login stands in the link",
				),
			);
	}
	return found;
}

function scopeProblems(config: Config): Problem[] {
	const types = config.commits?.types;
	if (!types) return [];
	return Object.keys(config.commits?.scopes ?? {})
		.filter((type) => !types.includes(type))
		.map((type) =>
			problem(
				"config-scope-type",
				`commits.scopes.${type} is not one of commits.types`,
				`add ${type} to commits.types, or remove commits.scopes.${type}`,
			),
		);
}

// A footer's source must be the ledger or a kind of tests the config has, and
// the types it names must be commit types.
const footerProblems = (config: Config): Problem[] =>
	Object.entries(config.commits?.footers ?? {}).flatMap(([key, f]) => [
		...sourceProblems(config, `commits.footers.${key}`, f.source),
		...namedTypeProblems(config, `commits.footers.${key}.required_for`, f.required_for),
		...namedTypeProblems(config, `commits.footers.${key}.validate_for`, f.validate_for),
	]);

function sourceProblems(config: Config, where: string, source: FooterConfig["source"]): Problem[] {
	if (typeof source === "string")
		return source === "ledger"
			? []
			: [
					problem(
						"config-footer-source",
						`${where}.source is ledger or { tests: <kind> }`,
						`set ${where}.source to ledger or { tests: <kind> }`,
					),
				];
	return config.tests?.[source.tests]
		? []
		: [
				problem(
					"config-footer-source",
					`${where}.source names tests.${source.tests}, which the config does not have`,
					`add tests.${source.tests}, or name a kind tests: has`,
				),
			];
}

function namedTypeProblems(config: Config, where: string, named?: string | string[]): Problem[] {
	if (named === undefined || named === "all") return [];
	if (typeof named === "string")
		return [
			problem(
				"config-footer-types",
				`${where} is a list of types or all`,
				`write ${where} as a list of commit types, or all`,
			),
		];
	const types = config.commits?.types ?? named;
	return named
		.filter((type) => !types.includes(type))
		.map((type) =>
			problem(
				"config-footer-types",
				`${where} names ${type}, which is not one of commits.types`,
				`add ${type} to commits.types, or remove it from ${where}`,
			),
		);
}

function stepProblems(config: Config): Problem[] {
	const steps = [...(config.ci?.steps ?? []), ...(config.ci?.nightly?.steps ?? [])];
	return steps.flatMap((s) => {
		if (typeof s === "string") return [];
		if ((s.run === undefined) === (s.tests === undefined))
			return [
				problem(
					"config-step",
					`a CI step needs exactly one of run and tests: ${JSON.stringify(s)}`,
					"give the step either run: <command> or tests: <kind>, not both",
				),
			];
		if (s.tests === undefined || config.tests?.[s.tests]?.run?.whole) return [];
		return [
			problem(
				"config-step-tests",
				`a CI step runs tests: ${s.tests}, and tests.${s.tests}.run.whole is missing`,
				`add tests.${s.tests}.run.whole, the command that runs every ${s.tests}`,
			),
		];
	});
}

function patternProblems(config: Config): Problem[] {
	const found: Problem[] = [];
	for (const [i, pattern] of (config.ci?.cost?.static ?? []).entries())
		tryRegExp(pattern, `ci.cost.static[${i}]`, found);
	for (const [i, rule] of (config.ci?.covers ?? []).entries())
		tryRegExp(rule.matches, `ci.covers[${i}].matches`, found);
	const ledger = config.ledger;
	if (!ledger) return found;
	if (ledger.id) tryRegExp(ledger.id, "ledger.id", found);
	if (ledger.group?.pattern) tryRegExp(ledger.group.pattern, "ledger.group.pattern", found);
	if (!ledger.files.includes("{group}"))
		found.push(
			problem(
				"config-ledger-files",
				`ledger.files has no {group}: ${ledger.files}`,
				"write {group} in ledger.files where a file's group stands, as in tasks/phase-{group}.yaml",
			),
		);
	return found;
}

export function loadConfig(file = configPath()): Config {
	let raw: unknown;
	try {
		raw = parse(readFileSync(file, "utf8"));
	} catch (error) {
		throw new ConfigError(file, [
			problem(
				"config-unreadable",
				`cannot be read: ${(error as Error).message}`,
				`create ${file}, or correct its YAML`,
			),
		]);
	}
	const found = problems(raw, SCHEMA, "");
	if (found.length) throw new ConfigError(file, found);
	const config = raw as Config;
	if (config.version !== 1)
		throw new ConfigError(file, [
			problem("config-version", `version ${config.version} is not 1`, "set version: 1"),
		]);
	expandSets(config, found);
	found.push(...crossProblems(config));
	if (found.length) throw new ConfigError(file, found);
	return config;
}

const loaded = new Map<string, Config>();
// The config, read once per file.
export function config(): Config {
	const file = configPath();
	if (!loaded.has(file)) loaded.set(file, loadConfig(file));
	return loaded.get(file)!;
}

// A section a tool cannot work without.
export function section<K extends keyof Config>(key: K): NonNullable<Config[K]> {
	const value = config()[key];
	if (value === undefined)
		throw new ConfigError(configPath(), [
			problem("config-missing-section", `${key} is missing`, `add a ${key}: section`),
		]);
	return value!;
}

// The globs: `*` does not cross `/`, `**`
// does, `**/` may match nothing, `{a,b}`, the whole path; no `/` is the root.
export const globToRegExp = (glob: string) =>
	new RegExp(
		`^${glob
			.replace(/[.+^$()|[\]\\]/g, "\\$&")
			.replace(/\{([^}]*)\}/g, (_, options: string) => `(?:${options.split(",").join("|")})`)
			.replace(/\*\*\//g, "(?:.*/)?")
			.replace(/\*\*/g, ".*")
			.replace(/(?<!\.)\*/g, "[^/]*")}$`,
	);
export const matchesAny = (file: string, globs: string[]) =>
	globs.some((g) => globToRegExp(g).test(file));

// A command with its whitespace collapsed, as the command patterns read it.
export const normal = (command: string) => command.trim().replace(/\s+/g, " ");

// The ledger's folder and how its files are named: `tasks/phase-{group}.yaml`
// gives `tasks` and a pattern whose group is the phase.
export function ledgerLayout(): { dir: string; file: RegExp; numeric: boolean } {
	const ledger = section("ledger");
	const [before, after] = basename(ledger.files).split("{group}") as [string, string];
	const quote = (s: string) => s.replace(/[.*+?^$()|[\]\\{}]/g, "\\$&");
	const group = ledger.group?.pattern ?? "[^/]+";
	return {
		dir: dirname(ledger.files),
		file: new RegExp(`^${quote(before)}(${group})${quote(after)}$`),
		numeric: ledger.group?.numeric ?? false,
	};
}

// The ledger's files in a folder (its own by default), sorted, with their group.
export function ledgerFiles(dir = ledgerLayout().dir): { path: string; group: string }[] {
	const { file } = ledgerLayout();
	return readdirSync(dir)
		.map((name) => ({ name, m: file.exec(name) }))
		.filter(({ m }) => m)
		.sort((a, b) => a.name.localeCompare(b.name))
		.map(({ name, m }) => ({ path: join(dir, name), group: m![1]! }));
}

// The static command patterns: a check or step without a `cost:` of
// its own is static when one matches.
let patterns: { file: string; rules: RegExp[] } | undefined;
export function staticPatterns(): RegExp[] {
	if (patterns?.file !== configPath())
		patterns = {
			file: configPath(),
			rules: (config().ci?.cost?.static ?? []).map((p) => new RegExp(p)),
		};
	return patterns.rules;
}
export const matchesStatic = (command: string) =>
	staticPatterns().some((rule) => rule.test(normal(command)));

const TASK_KEYS = ["id", "type", "title", "why", "done_when"];
const CHECK_KEYS = ["run", "fails", "after", "timeout", "prose", "cost"];

interface RawCheck {
	run?: unknown;
	fails?: unknown;
	after?: unknown;
	timeout?: unknown;
	prose?: unknown;
	cost?: unknown;
}

// What each optional key of a check may hold, and the fix.
const CHECK_VALUES: [keyof RawCheck, (v: unknown) => boolean, string, string][] = [
	[
		"after",
		(v) => v === "push",
		"after: push is its only value",
		"write after: push, or remove it",
	],
	[
		"timeout",
		(v) => typeof v === "number",
		"its timeout is a number of seconds",
		"write the timeout as a number of seconds",
	],
	["prose", (v) => typeof v === "boolean", "prose is true or false", "write prose: true or false"],
	[
		"cost",
		(v) => v === "static" || v === "late",
		"cost is static or late",
		"write cost: static or cost: late",
	],
];

// One check's problems.
function checkProblems(check: unknown, where: string): Problem[] {
	if (!check || typeof check !== "object" || Array.isArray(check))
		return [
			problem(
				"ledger-check-shape",
				`${where} is not a mapping`,
				`write ${where} as run: <command> or fails: <command>`,
			),
		];
	const c = check as RawCheck;
	const found = Object.keys(c)
		.filter((k) => !CHECK_KEYS.includes(k))
		.map((k) =>
			problem(
				"ledger-check-unknown-key",
				`${where} has an unknown key ${k}`,
				`remove ${k}; a check's keys are ${CHECK_KEYS.join(", ")}`,
			),
		);
	if ((c.run === undefined) === (c.fails === undefined))
		found.push(
			problem(
				"ledger-check-run-or-fails",
				`${where} needs exactly one of run and fails`,
				`give ${where} either run: or fails:, not both`,
			),
		);
	else if (typeof (c.run ?? c.fails) !== "string")
		found.push(
			problem(
				"ledger-check-command",
				`${where}'s command is not text`,
				`quote ${where}'s command as one string`,
			),
		);
	for (const [key, valid, rule, fix] of CHECK_VALUES)
		if (c[key] !== undefined && !valid(c[key]))
			found.push(
				problem(
					"ledger-check-value",
					`${where} says ${key}: ${JSON.stringify(c[key])}; ${rule}`,
					`${fix} in ${where}`,
				),
			);
	return found;
}

// Whether a check is late by itself: its own `cost:`, else the patterns.
function lateByItself(check: unknown): boolean {
	const c = (check ?? {}) as RawCheck;
	if (c.cost === "late" || c.cost === "static") return c.cost === "late";
	const command = c.run ?? c.fails;
	return !(typeof command === "string" && matchesStatic(command));
}

// A `cost: static` written below a late check, which written order runs late.
function orderProblems(checks: unknown[]): Problem[] {
	const firstLate = checks.findIndex(lateByItself);
	if (firstLate < 0 || config().ci?.cost?.keep_written_order !== true) return [];
	return checks
		.map((check, n) => ({ n, cost: (check as RawCheck | null)?.cost }))
		.filter(({ n, cost }) => n > firstLate && cost === "static")
		.map(({ n }) =>
			problem(
				"ledger-static-after-late",
				`check ${n} says cost: static below check ${firstLate}, which is late: ` +
					"written order runs it late",
				`move check ${n} above check ${firstLate}, or remove its cost: static`,
			),
		);
}

// A task's id, type, title and why.
function fieldProblems(task: Record<string, unknown>): Problem[] {
	const cfg = config();
	const found: Problem[] = [];
	const idPattern = new RegExp(`^${cfg.ledger?.id ?? ".+"}$`);
	if (typeof task.id !== "string")
		found.push(problem("ledger-no-id", "no id", "give the task an id: that matches ledger.id"));
	else if (!idPattern.test(task.id))
		found.push(
			problem(
				"ledger-id-pattern",
				`the id does not match ${idPattern.source}`,
				`rename the task to an id that matches ${idPattern.source}`,
			),
		);
	const types = cfg.commits?.types;
	if (typeof task.type !== "string")
		found.push(problem("ledger-no-type", "no type", "give the task a type: (a commit type)"));
	else if (types && !types.includes(task.type))
		found.push(
			problem(
				"ledger-type",
				`type ${task.type} is not a commit type`,
				`set type: to one of ${types.join(", ")}`,
			),
		);
	if (typeof task.title !== "string")
		found.push(problem("ledger-no-title", "no title", "give the task a title:"));
	if (task.why !== undefined && typeof task.why !== "string")
		found.push(problem("ledger-why", "its why is not text", "write the why as text"));
	return found;
}

// One task's own problems, the ones of its checks included.
function taskProblems(task: Record<string, unknown>): Problem[] {
	const found = [
		...Object.keys(task)
			.filter((k) => !TASK_KEYS.includes(k))
			.map((k) =>
				problem(
					"ledger-unknown-key",
					`unknown key ${k}`,
					`remove ${k}; a task's keys are ${TASK_KEYS.join(", ")}`,
				),
			),
		...fieldProblems(task),
	];
	const checks = task.done_when ?? [];
	if (!Array.isArray(checks))
		return [
			...found,
			problem("ledger-done-when", "done_when is not a list", "write done_when as a list of checks"),
		];
	return [
		...found,
		...checks.flatMap((check, n) => checkProblems(check, `check ${n}`)),
		...orderProblems(checks),
	];
}

// A ledger file's tasks, or the problem reading it.
function readLedger(file: string): { tasks: Record<string, unknown>[] } | { problem: Problem } {
	try {
		const tasks: unknown = parse(readFileSync(file, "utf8")) ?? [];
		return Array.isArray(tasks)
			? { tasks: tasks.map((t) => (t ?? {}) as Record<string, unknown>) }
			: {
					problem: problem(
						"ledger-not-a-list",
						`${file} is not a list of tasks`,
						`write ${file} as a YAML list of tasks`,
					),
				};
	} catch (error) {
		return {
			problem: problem(
				"ledger-unreadable",
				`${file} cannot be read: ${(error as Error).message}`,
				`correct ${file}'s YAML (a value holding ": " must be quoted)`,
			),
		};
	}
}

// The ledger's problems over the given files,
// with their rule ids.
export function ledgerIssues(files: string[]): Problem[] {
	const seen = new Map<string, string>();
	const found: Problem[] = [];
	for (const file of files) {
		const read = readLedger(file);
		if ("problem" in read) {
			found.push(read.problem);
			continue;
		}
		for (const [i, task] of read.tasks.entries()) {
			const id = typeof task.id === "string" ? task.id : `task ${i + 1} of ${file}`;
			const own = taskProblems(task);
			if (seen.has(id))
				own.push(
					problem(
						"ledger-duplicate-id",
						`also in ${seen.get(id)}`,
						`give one of the two ${id} tasks another id`,
					),
				);
			seen.set(id, file);
			found.push(...own.map((p) => ({ ...p, message: `${id}: ${p.message}` })));
		}
	}
	return found;
}

// The config's problems, or none: a config that cannot be loaded.
export function configIssues(): { file: string; problems: Problem[] } {
	try {
		config();
		return { file: configPath(), problems: [] };
	} catch (error) {
		if (!(error instanceof ConfigError)) throw error;
		return { file: error.file, problems: error.issues };
	}
}
