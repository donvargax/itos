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
import { readFileSync } from "node:fs";
import { basename, dirname, join } from "node:path";
import { parse } from "yaml";
import { messages, problem, type Problem } from "./problem.ts";
import { current } from "./source.ts";

export type Cost = "static" | "late";

export interface StepConfig {
	run?: string;
	tests?: string;
	whole?: boolean;
	cost?: Cost;
	// The nightly's step that runs the checks of every task whose work item
	// is done; with cost: static, only their static ones.
	tasks?: "done";
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
// The hook managers `hooks install` writes or prints for (hooks.ts).
export const HOOK_MANAGERS = ["vp", "git", "husky", "lefthook", "pre-commit", "prek"] as const;
export type HookManager = (typeof HOOK_MANAGERS)[number];
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
		check?: { timeout?: number };
	};
	commits?: {
		types?: string[];
		// The header lint's delegate: the message file through `hook`, a
		// message on stdin through `stdin`.
		header_lint?: { hook?: string; stdin?: string };
		footers?: Record<string, FooterConfig>;
		path_sets?: Record<string, string[]>;
		scopes?: Record<string, ScopeRule>;
		reject_message?: string;
		// The commit where verification starts: verify and the range checks
		// leave it and its ancestors out.
		since?: string;
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
	hooks?: {
		// The hook manager `hooks install` writes for, over the one it detects.
		manager?: HookManager;
		bin?: string;
		pre_push?: { per_base: string; whole: string };
		// The commit-msg hook's run of the named tasks' static checks
		// (commit-tasks.ts): whether it runs, and each check's longest time.
		commit_msg?: { task_checks?: boolean; check_timeout?: number };
	};
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
	either: [str, obj({ run: str, tests: str, whole: bool, cost, tasks: { enum: ["done"] } })],
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
				check: obj({ timeout: "number" }),
			},
			["files"],
		),
		commits: obj({
			types: strs,
			header_lint: obj({ hook: str, stdin: str }),
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
			since: str,
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
			manager: { enum: [...HOOK_MANAGERS] },
			bin: str,
			pre_push: obj({ per_base: str, whole: str }, ["per_base", "whole"]),
			commit_msg: obj({ task_checks: bool, check_timeout: "number" }),
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
		// The specs of the value's shape are judged first, so a mapping with a
		// wrong value is told about that value, not that it is no string.
		const shaped = spec.either.filter((s) => fits(value, s));
		const each = (shaped.length ? shaped : spec.either).map((s) => problems(value, s, path));
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

// Whether a value has the shape a spec wants: a mapping for an object or a
// map, a list for a list, a scalar of the type for the rest.
function fits(value: unknown, spec: Spec): boolean {
	if (spec === "strings" || (typeof spec === "object" && "list" in spec))
		return Array.isArray(value);
	if (typeof spec === "string") return typeof value === spec;
	if ("enum" in spec) return typeof value === "string";
	if ("either" in spec) return spec.either.some((s) => fits(value, s));
	return !!value && typeof value === "object" && !Array.isArray(value);
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
	...sinceProblems(config),
	...scopeProblems(config),
	...footerProblems(config),
	...stepProblems(config),
	...patternProblems(config),
	...providerProblems(config),
	...hookProblems(config),
];

// A check_timeout of no seconds would be no timeout at all to the shell.
function hookProblems(config: Config): Problem[] {
	const seconds = config.hooks?.commit_msg?.check_timeout;
	if (seconds === undefined || seconds > 0) return [];
	return [
		problem(
			"config-check-timeout",
			`hooks.commit_msg.check_timeout is a number of seconds above 0, not ${seconds}`,
			"set hooks.commit_msg.check_timeout to the seconds a check may hold a commit, or set hooks.commit_msg.task_checks to false",
		),
	];
}

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

// A full commit SHA, SHA-1 or SHA-256, as git prints it.
export const FULL_SHA = /^(?:[0-9a-f]{40}|[0-9a-f]{64})$/;

// commits.since is a full SHA: an abbreviation can grow ambiguous, and a
// branch or tag can move. Whether the repository has that commit is
// `config check`'s question (config-check.ts), since a shallow clone may not.
function sinceProblems(config: Config): Problem[] {
	const since = config.commits?.since;
	if (since === undefined || FULL_SHA.test(since)) return [];
	return [
		problem(
			"config-since",
			`commits.since is not the full SHA of a commit: ${since}`,
			"set commits.since to the commit's full SHA, as `git rev-parse <commit>` prints it",
		),
	];
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

// A step runs a command, a kind's tests or, in the nightly alone, the checks
// of the done tasks: a push runs the checks of the tasks its commits name.
const stepProblems = (config: Config): Problem[] => [
	...(config.ci?.steps ?? []).flatMap((s) => oneStepProblems(config, s, false)),
	...(config.ci?.nightly?.steps ?? []).flatMap((s) => oneStepProblems(config, s, true)),
];

function oneStepProblems(config: Config, s: string | StepConfig, nightly: boolean): Problem[] {
	if (typeof s === "string") return [];
	if (!nightly && s.tasks !== undefined)
		return [
			problem(
				"config-step-tasks",
				`a CI step runs tasks: ${s.tasks}, which only a nightly step does: ${JSON.stringify(s)}`,
				"move the step to ci.nightly.steps; a push runs the checks of the tasks its commits name",
			),
		];
	const given = [s.run, s.tests, s.tasks].filter((v) => v !== undefined).length;
	if (given !== 1)
		return [
			nightly
				? problem(
						"config-step",
						`a nightly step needs exactly one of run, tests and tasks: ${JSON.stringify(s)}`,
						"give the step one of run: <command>, tests: <kind> or tasks: done",
					)
				: problem(
						"config-step",
						`a CI step needs exactly one of run and tests: ${JSON.stringify(s)}`,
						"give the step either run: <command> or tests: <kind>, not both",
					),
		];
	if (s.tasks !== undefined)
		return s.cost === "late"
			? [
					problem(
						"config-step-cost",
						`a nightly step runs tasks: ${s.tasks} with cost: late; its cost is static or left out`,
						"write cost: static to run only the static checks, or leave cost out to run them all",
					),
				]
			: [];
	if (s.tests === undefined || config.tests?.[s.tests]?.run?.whole) return [];
	return [
		problem(
			"config-step-tests",
			`a CI step runs tests: ${s.tests}, and tests.${s.tests}.run.whole is missing`,
			`add tests.${s.tests}.run.whole, the command that runs every ${s.tests}`,
		),
	];
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

// The config's text from the source itos reads its data from (source.ts); one
// that source does not hold, such as an ITOS_CONFIG outside the repository, is
// read where it is.
const configText = (file: string) =>
	current().has(file) ? current().read(file) : readFileSync(file, "utf8");

export function loadConfig(file = configPath()): Config {
	let raw: unknown;
	try {
		raw = parse(configText(file));
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
// The config, read once per file and per tree it is read from.
export function config(): Config {
	const key = `${current().tree}:${configPath()}`;
	if (!loaded.has(key)) loaded.set(key, loadConfig());
	return loaded.get(key)!;
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

// Where the work registry is when work.registry leaves it out: beside the
// ledger, since it is itos's data as the ledger is, and docs/ is prose.
export const DEFAULT_REGISTRY = "tasks/work-items.yaml";

// The registry's statuses, and the key its owners per group are under, when
// work.statuses and work.groups_key leave them out.
export const DEFAULT_STATUSES = ["todo", "doing", "done", "blocked"];
export const DEFAULT_GROUPS_KEY = "phases";

// The longest the commit-msg hook lets one task check run, in seconds, when
// hooks.commit_msg.check_timeout leaves it out. A static check takes seconds
// (the cost rule), so a minute leaves room for a slow machine while a commit
// is never held for minutes; a check that needs longer is late.
export const DEFAULT_COMMIT_CHECK_TIMEOUT = 60;

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

// The names in the ledger's folder. A project may configure a ledger before it
// makes the folder: the folder missing is one config error naming it, exit 2 (a
// folder the config names that cannot be read), in every command that reads
// the ledger, rather than the read's own error. A working tree's folder that is
// there and cannot be listed keeps its own error.
function ledgerNames(dir: string): string[] {
	const source = current();
	try {
		return source.list(dir);
	} catch (error) {
		if (source.tree === "worktree" && source.has(dir)) throw error;
		throw new ConfigError(configPath(), [
			problem(
				"ledger-folder-missing",
				`the ledger's folder ${dir} does not exist`,
				`create ${dir} with the ledger's files (ledger.files is ${section("ledger").files}), or point ledger.files at the folder that holds them`,
			),
		]);
	}
}

// The ledger's files in a folder (its own by default), sorted, with their group.
export function ledgerFiles(dir = ledgerLayout().dir): { path: string; group: string }[] {
	const { file } = ledgerLayout();
	return ledgerNames(dir)
		.map((name) => ({ name, m: file.exec(name) }))
		.filter(({ m }) => m)
		.sort((a, b) => a.name.localeCompare(b.name))
		.map(({ name, m }) => ({ path: join(dir, name), group: m![1]! }));
}

// The static command patterns: a check or step without a `cost:` of
// its own is static when one matches.
let patterns: { config: Config; rules: RegExp[] } | undefined;
export function staticPatterns(): RegExp[] {
	if (patterns?.config !== config())
		patterns = {
			config: config(),
			rules: (config().ci?.cost?.static ?? []).map((p) => new RegExp(p)),
		};
	return patterns.rules;
}

// How the project calls itos when hooks.bin leaves it out: the wrapper this
// repository and its template ship.
const DEFAULT_BIN = "tools/bin/itos";
export const itosBin = () => config().hooks?.bin ?? DEFAULT_BIN;

// A command as the cost rule reads it: its whitespace collapsed, and, when its
// first word is hooks.bin, that word read as `itos`. The ledger and ci.steps
// call itos the way the project does, so a pattern written `^itos work check`
// matches `tools/bin/itos work check` without spelling out the path. Only
// hooks.bin itself is read so: a command under any other path is read as
// written. (A hooks.bin of several words is read as its leading words.)
function asItos(command: string): string {
	const text = normal(command);
	const bin = normal(itosBin());
	return text === bin || text.startsWith(`${bin} `) ? `itos${text.slice(bin.length)}` : text;
}

// Whether a command is static by the patterns. Each pattern is tried on the
// command as written and as the cost rule reads it, so a pattern that names
// hooks.bin's path keeps matching: reading hooks.bin as itos only ever makes a
// command static, never late. The rewrite is for matching alone; the command
// runs as written. CI's plan, the commit-msg hook and config check's order
// rule all come here.
export function matchesStatic(command: string): boolean {
	const forms = [normal(command), asItos(command)];
	return staticPatterns().some((rule) => forms.some((form) => rule.test(form)));
}

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
		const tasks: unknown = parse(current().read(file)) ?? [];
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
