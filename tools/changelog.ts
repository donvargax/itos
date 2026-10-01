// vp run changelog                        write the changelog to docs/changelog/CHANGELOG.md
// vp run changelog -- --task T-010        print only the commits whose Task: footer names it
// vp run changelog -- --scenario @ID-APP-01   the same for a Scenarios: footer
//
// The history is the changelog: git-cliff reads the Conventional
// Commits with the root cliff.toml, grouped by type, newest first, each with
// its short SHA, its Task: / Scenarios: footers and its body. The file is
// written on demand into docs/changelog/, which git, the formatter, the linter
// and the audit ignore, so nothing committed can be stale. git-cliff has no filter
// by footer, so a filtered run takes its context (the parsed commits as JSON),
// keeps the commits that name the id and renders that. Always `--offline`: no
// remote is read or linked.
import { spawnSync } from "node:child_process";
import { mkdirSync } from "node:fs";

const OUT = "docs/changelog/CHANGELOG.md";
const CLIFF = "node_modules/.bin/git-cliff";
const FOOTER = { "--task": "Task", "--scenario": "Scenarios" } as const;

interface Footer {
	token: string;
	value: string;
}
interface Release {
	commits: { footers: Footer[] }[];
}

function cliff(args: string[], input?: string): string {
	const result = spawnSync(CLIFF, ["--offline", ...args], {
		input,
		encoding: "utf8",
		maxBuffer: 256 * 1024 * 1024,
		stdio: ["pipe", "pipe", "inherit"],
		env: { ...process.env, RUST_LOG: "warn" },
	});
	if (result.status !== 0) process.exit(result.status ?? 1);
	return result.stdout;
}

// Whole ids only: T-01 does not match T-010. A scenario id may come with or
// without its @.
const names = (footer: Footer, id: string) =>
	footer.value
		.split(/[\s,]+/)
		.map((token) => token.replace(/^@/, ""))
		.includes(id.replace(/^@/, ""));

// `vp run changelog -- --task <id>` hands the script its `--` as well.
const args = process.argv.slice(2);
const [flag, id] = args[0] === "--" ? args.slice(1) : args;
if (flag === undefined) {
	mkdirSync("docs/changelog", { recursive: true });
	cliff(["--output", OUT]);
	console.log(`wrote ${OUT}`);
} else if ((flag === "--task" || flag === "--scenario") && id) {
	const token = FOOTER[flag];
	const releases = JSON.parse(cliff(["--context"])) as Release[];
	let kept = 0;
	for (const release of releases) {
		release.commits = release.commits.filter((commit) =>
			commit.footers.some((footer) => footer.token === token && names(footer, id)),
		);
		kept += release.commits.length;
	}
	if (kept === 0) {
		console.error(`no commit names ${id} in a ${token}: footer`);
		process.exit(1);
	}
	process.stdout.write(cliff(["--from-context", "-"], JSON.stringify(releases)));
} else {
	console.error("usage: vp run changelog [-- --task <T-id> | -- --scenario <@ID-…>]");
	process.exit(2);
}
