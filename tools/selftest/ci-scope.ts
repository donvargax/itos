// The prose shortcut must cover prose and nothing else. A path that reaches
// any gate — a task, a scenario, a config, a source file — must send the push
// down the full pipeline. This holds the project's own prose paths
// (itos.yaml's `ci.prose.paths`) to that, through `itos ci scope` over ranges
// made with `git commit-tree` on top of HEAD, each changing the paths named
// (objects only: no branch, index or file of the checkout moves).
//
// What a prose-only range plans (the prose steps, the named tasks' static and
// `prose: true` checks, nothing late) and what waits for a task not started
// are the tool's behaviour, whatever the config: they are
// tools/itos/conformance/plans.yaml's and ci-run.yaml's.
import { strict as assert } from "node:assert";
import { spawnSync } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { itos, outsideEnv, realGit } from "./scratch.ts";

const scratch = mkdtempSync(join(tmpdir(), "ci-scope-selftest-"));
// A private index, so the checkout's own is never read or written.
const env = { ...outsideEnv(), GIT_INDEX_FILE: join(scratch, "index") };
const git = (args: string[], input?: string) => {
	const run = spawnSync(realGit(), args, { env, input, encoding: "utf8" });
	assert.equal(run.status, 0, `git ${args.join(" ")} failed:\n${run.stdout}${run.stderr}`);
	return run.stdout.trim();
};

// A commit on top of HEAD whose tree changes each path, added if it is new.
function changing(paths: string[]): string {
	git(["read-tree", "HEAD"]);
	for (const path of paths) {
		const blob = git(["hash-object", "-w", "--stdin"], `ci scope self-test: ${path}\n`);
		git(["update-index", "--add", "--cacheinfo", `100644,${blob},${path}`]);
	}
	const tree = git(["write-tree"]);
	return git([
		"-c",
		"user.name=ci-scope",
		"-c",
		"user.email=selftest@localhost",
		"commit-tree",
		tree,
		"-p",
		"HEAD",
		"-m",
		"docs: a ci scope self-test range",
	]);
}
const docsOnly = (paths: string[]) => {
	const said = itos(["ci", "scope", "HEAD", changing(paths)]).trim();
	assert.match(said, /^docs_only=(true|false)$/, `itos ci scope said ${JSON.stringify(said)}`);
	return said === "docs_only=true";
};

try {
	assert.equal(docsOnly(["docs/HANDOFF.md", "AGENTS.md", "PLAN.md"]), true);
	assert.equal(docsOnly(["docs/decisions/format.md"]), true);

	assert.equal(docsOnly(["AGENTS.md", "internal/cli/cli.go"]), false);
	assert.equal(docsOnly(["tasks/phase-0.yaml"]), false);
	assert.equal(docsOnly(["features/since.feature"]), false);
	assert.equal(docsOnly(["package.json"]), false);
	assert.equal(docsOnly(["go.mod"]), false);
	assert.equal(docsOnly([".github/workflows/ci.yml"]), false);

	// Nothing known changed (a first push, a shallow clone): run everything.
	assert.equal(itos(["ci", "scope", "", "HEAD"]).trim(), "docs_only=false");
} finally {
	rmSync(scratch, { recursive: true, force: true });
}

console.log("ci scope: the project's prose paths hold prose and nothing else");
