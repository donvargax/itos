import type { SyncRule, UserConfig } from "@commitlint/types";
import { checkFooter, footers } from "./tools/itos/footers.ts";

// Which footer each commit type needs, and what its IDs must be, is
// `commits.footers` in itos.yaml (see tasks/README.md): one rule,
// `<key>-footer`, per footer it names, each the same generic check
// (tools/itos/footers.ts), whose named tests come from their kind's adapter
// (tools/itos/tests.ts).
export const footerRule =
	(key: string): SyncRule =>
	(commit) =>
		checkFooter(key, commit.type, commit.raw ?? "");

const names = footers().map((f) => [`${f.key.toLowerCase()}-footer`, f.key] as const);

const config: UserConfig = {
	extends: ["@commitlint/config-conventional"],
	plugins: [{ rules: Object.fromEntries(names.map(([name, key]) => [name, footerRule(key)])) }],
	rules: Object.fromEntries(names.map(([name]) => [name, [2, "always"]])),
};

export default config;
