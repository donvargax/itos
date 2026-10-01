// The three things the tooling asks of the world outside the repository, each
// a provider chosen in itos.yaml:
//
//   ci.range       where a push's range starts: `github` (the last green run
//                  of the workflow), `command` (its stdout is the
//                  commit) or `none` (always run everything)
//   work.identity  who a session works for: `github` (`gh api user`),
//                  `command` (its stdout is the handle) or `none` (only --as)
//   work.people    who may own work: `all-contributors-md` (the table in
//                  CONTRIBUTORS.md), `all-contributorsrc` (the
//                  specification's JSON) or `yaml` (a list of logins)
//
// Every provider answers or says it cannot; none of them throws for a lookup
// that fails, since a failed range lookup means "run everything" and a failed
// identity means "nobody".
import { execFileSync, spawnSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { parse } from "yaml";
import { config, type IdentityConfig, type PeopleConfig, type RangeConfig } from "./config.ts";

// A command run through the config's shell, and its trimmed first line of
// output; undefined when it fails or prints nothing.
function firstLine(command: string): string | undefined {
	const [shell = "sh", ...flags] = config().shell ?? ["sh", "-c"];
	const run = spawnSync(shell, [...flags, command], {
		encoding: "utf8",
		stdio: ["ignore", "pipe", "inherit"],
	});
	if (run.status !== 0) return undefined;
	return run.stdout.trim().split("\n")[0]?.trim() || undefined;
}

// The start commit a range provider proposes, before rangeStart holds it to
// the head's ancestry. Undefined means "run everything".
export function rangeProvider(
	range: RangeConfig = config().ci?.range ?? {},
	env: NodeJS.ProcessEnv = process.env,
): () => Promise<string | undefined> {
	const provider = range.provider ?? "github";
	if (provider === "none") return async () => undefined;
	if (provider === "command") return async () => firstLine(range.command ?? "");
	const github = range.github ?? {};
	const tokens = github.token_env ?? ["GITHUB_TOKEN", "GH_TOKEN"];
	return () =>
		lastGreenRun({
			repository: env[github.repository_env ?? "GITHUB_REPOSITORY"] ?? "",
			token: tokens.map((name) => env[name]).find(Boolean),
			workflow: github.workflow,
			branch: github.branch,
		});
}

// The head commit of the workflow's last successful run on a branch, read from
// the GitHub API with the workflow's token (`actions: read`). Anything that
// goes wrong reads as "no green run", which runs everything. The runs are
// listed and the newest success taken: the API's own `status=success` filter
// can answer with a run far older than the newest green one, and the range
// would then name every task since.
async function lastGreenRun({
	repository,
	token,
	workflow = "ci.yml",
	branch = "main",
}: {
	repository: string;
	token?: string;
	workflow?: string;
	branch?: string;
}): Promise<string | undefined> {
	if (!repository) return undefined;
	const url =
		`https://api.github.com/repos/${repository}/actions/workflows/${workflow}/runs` +
		`?branch=${encodeURIComponent(branch)}&per_page=50`;
	const response = await fetch(url, {
		headers: {
			accept: "application/vnd.github+json",
			...(token ? { authorization: `Bearer ${token}` } : {}),
		},
	});
	if (!response.ok) return undefined;
	const body = (await response.json()) as { workflow_runs?: WorkflowRun[] };
	return firstGreen(body.workflow_runs ?? []);
}

interface WorkflowRun {
	head_sha?: string;
	conclusion?: string | null;
	created_at?: string;
}

// The newest successful run's head commit, whatever order the list came in.
export const firstGreen = (runs: WorkflowRun[]): string | undefined =>
	[...runs]
		.filter((run) => run.conclusion === "success" && run.head_sha)
		.sort((a, b) => (b.created_at ?? "").localeCompare(a.created_at ?? ""))[0]?.head_sha;

export type Answer = { handle: string } | { problem: string };

// The login `gh` is signed in as; throws when gh is missing or signed out.
const ghLogin = () =>
	execFileSync("gh", ["api", "user", "--jq", ".login"], {
		encoding: "utf8",
		stdio: ["ignore", "pipe", "pipe"],
	}).trim();

// The GitHub account `gh` is signed in as, or why there is none.
export function githubIdentity(hint: string, login: () => string = ghLogin): () => Answer {
	return () => {
		try {
			const handle = login();
			if (handle) return { handle };
		} catch (error) {
			if ((error as NodeJS.ErrnoException).code === "ENOENT")
				return { problem: `gh is not installed, so this session is nobody; ${hint}` };
		}
		return { problem: `gh is not signed in (gh auth login), so this session is nobody; ${hint}` };
	};
}

// Who a session works for when `--as` does not say.
export function identityProvider(identity: IdentityConfig = config().work?.identity ?? {}) {
	const hint = identity.hint ?? "pass --as <handle>";
	const provider = identity.provider ?? "github";
	if (provider === "github") return githubIdentity(hint);
	if (provider === "none")
		return (): Answer => ({
			problem: `work.identity is none, so this session is nobody; ${hint}`,
		});
	const command = identity.command ?? "";
	return (): Answer => {
		const handle = firstLine(command);
		return handle
			? { handle }
			: { problem: `\`${command}\` gave no handle, so this session is nobody; ${hint}` };
	};
}

const escape = (s: string) => s.replace(/[.*+?^$()|[\]\\{}]/g, "\\$&");

// The logins of an All Contributors table: each person's cell links to their
// profile (`login_from`, `https://github.com/{login}` by default) between the
// list's markers.
export function allContributorsMd(
	markdown: string,
	loginFrom = "https://github.com/{login}",
): string[] {
	const list =
		/<!-- ALL-CONTRIBUTORS-LIST:START[^>]*-->([\s\S]*?)<!-- ALL-CONTRIBUTORS-LIST:END/.exec(
			markdown,
		);
	const [before = "", after = ""] = loginFrom.split("{login}");
	const link = new RegExp(`href="${escape(before)}([^"/?#\\s]+)${escape(after)}"`, "g");
	return [...new Set([...(list?.[1] ?? "").matchAll(link)].map((m) => m[1]!))];
}

// The logins of an `.all-contributorsrc`: its `contributors[].login`.
export function allContributorsRc(json: string): string[] {
	const rc = JSON.parse(json) as { contributors?: { login?: unknown }[] };
	if (!Array.isArray(rc.contributors)) throw new Error("has no contributors list");
	return [
		...new Set(
			rc.contributors.map((c, i) => {
				if (typeof c?.login !== "string" || !c.login)
					throw new Error(`contributors[${i}] has no login`);
				return c.login;
			}),
		),
	];
}

// The logins of a YAML file that is a list of them.
export function yamlLogins(text: string): string[] {
	const list: unknown = parse(text);
	if (!Array.isArray(list)) throw new Error("is not a list of logins");
	return [
		...new Set(
			list.map((login, i) => {
				if (typeof login !== "string" || !login) throw new Error(`entry ${i} is not a login`);
				return login;
			}),
		),
	];
}

export const DEFAULT_PEOPLE: PeopleConfig = {
	source: "all-contributors-md",
	file: "CONTRIBUTORS.md",
};

// Who may own work, read from the file the people source names.
export function people(source: PeopleConfig = config().work?.people ?? DEFAULT_PEOPLE): string[] {
	const text = readFileSync(source.file, "utf8");
	try {
		if (source.source === "all-contributorsrc") return allContributorsRc(text);
		if (source.source === "yaml") return yamlLogins(text);
		return allContributorsMd(text, source.login_from);
	} catch (error) {
		throw new Error(`${source.file} (work.people, ${source.source}) ${(error as Error).message}`);
	}
}
