// `itos config check [--ledger <file>] [--print-defaults]`: the config, then
// the ledger, the work registry and each kind's smoke set, every problem with
// its rule id and, where one exists, a fix. Exit 2 when the config is invalid
// (nothing else can be read), 1 for any other problem.
import { stringify } from "yaml";
import { smokeIds, smokeIssues } from "./e2e-scope.ts";
import { load, registryIssues } from "./work.ts";
import { config, configIssues, configPath, ledgerFiles, ledgerIssues } from "./config.ts";
import { emit, type Output, problem, type Problem, TEXT } from "./problem.ts";
import { DEFAULT_PEOPLE } from "./providers.ts";
import { loadSmoke } from "./smoke.ts";

// The values the tools take when the config leaves a key out.
export const DEFAULTS = {
	shell: ["sh", "-c"],
	ledger: { group: { pattern: "[^/]+", numeric: false }, check: { timeout: 600 } },
	commits: { reject_message: "Commit rejected:" },
	tests: { "<kind>": { adapter: "gherkin", tag_prefix: "@", wip_tag: "@wip" } },
	ci: { wait_on_status: ["todo"], range: { provider: "github" } },
	work: {
		registry: "docs/work-items.yaml",
		people: DEFAULT_PEOPLE,
		identity: { provider: "github", hint: "pass --as <handle>" },
	},
};

type Area = "config" | "ledger" | "registry" | "smoke";
type Found = Problem & { area: Area };

const tagged = (area: Area, found: Problem[]): Found[] => found.map((p) => ({ ...p, area }));

// Each kind's smoke set against its tests; a set that cannot be read is one problem.
function smokeProblems(): { lines: string[]; found: Found[] } {
	const lines: string[] = [];
	const found: Found[] = [];
	for (const [name, k] of Object.entries(config().tests ?? {})) {
		const file = (k.smoke as { file?: string } | undefined)?.file;
		if (!file) continue;
		try {
			const smoke = loadSmoke(name);
			const own = smokeIssues(smoke, undefined, name);
			found.push(...tagged("smoke", own));
			lines.push(`${file}: every file has a smoke test (${smokeIds(smoke, name).length} in all)`);
		} catch (error) {
			const message = (error as Error).message;
			found.push({ ...problem("smoke-unreadable", message, `correct ${file}`), area: "smoke" });
		}
	}
	return { lines, found };
}

function report(found: Found[], lines: string[], out: Output) {
	if (out.json) emit({ config: configPath(), valid: found.length === 0, problems: found });
	else for (const p of found) console.error(`FAIL ${p.message}`);
	if (!found.length && !out.json && !out.quiet) for (const line of lines) console.log(line);
}

export function configCheck(ledger: string | undefined, out: Output = TEXT): number {
	const loaded = configIssues();
	if (loaded.problems.length) {
		const found = loaded.problems.map((p) => ({
			...p,
			message: `${loaded.file}: ${p.message}`,
			area: "config" as const,
		}));
		report(found, [], out);
		return 2;
	}
	const files = ledger ? [ledger] : ledgerFiles().map((f) => f.path);
	const registry = config().work?.registry ?? DEFAULTS.work.registry;
	const smoke = smokeProblems();
	const found = [
		...tagged("ledger", ledgerIssues(files)),
		...tagged("registry", registryIssues(load())),
		...smoke.found,
	];
	report(
		found,
		[
			`${configPath()} is valid, and so are the ${files.length} ledger files it reads`,
			`${registry}: sound`,
			...smoke.lines,
		],
		out,
	);
	return found.length ? 1 : 0;
}

// `config check --print-defaults`: the defaults, as YAML or JSON.
export function printDefaults(out: Output = TEXT): number {
	if (out.json) emit({ defaults: DEFAULTS });
	else process.stdout.write(stringify(DEFAULTS));
	return 0;
}
