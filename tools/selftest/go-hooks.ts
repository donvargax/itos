// The hooks run the Go unit tests a change reaches, as they run Vitest's
// (T-059). Run against the real hooks in a scratch worktree of the current
// tree (uncommitted edits included), as gates.ts runs the other gates:
//
//   - pre-commit runs the unit tests of the Go package a staged change touches
//     and of the packages that import it, and no other package's;
//   - a staged Go change whose unit test fails is refused at commit, and one
//     whose tests pass is committed;
//   - pre-push does the same against the remote commit the push builds on;
//   - a change to go.mod, which every package builds with, runs every package;
//   - a Markdown-only commit, and a prose-only push, run no Go test.
//
// tools/bin/go-unit-tests picks the packages; both hooks call it.
import { hookGates, scratchRepo } from "./scratch.ts";

const repo = scratchRepo("go-hooks-selftest");
const { env, sh, git, edit, commit } = repo;

const MODULE = "github.com/donvargax/itos/v2/";
// The packages a `go test` run reported, by the line it prints for each:
// `ok`, `FAIL` or `?` (no test files), a tab, the import path.
const tested = (output: string, verdict = "ok|FAIL|\\?") =>
	new Set(
		[...output.matchAll(new RegExp(`^(?:${verdict})\\s+${MODULE}(\\S+)`, "gm"))].map((m) => m[1]!),
	);
let every: string[] = [];
const show = (packages: Set<string>) => [...packages].sort().join(", ") || "none";

const { problems, timings, expect, preCommit, prePush } = hookGates(repo);

// internal/version is imported by internal/cli and internal/launch (and
// cmd/itos, which imports both and has no tests) and by nothing else, so its
// change reaches those and not, say, internal/glob.
const pkg = "internal/version";
const file = `${pkg}/version.go`;
const reached = ["internal/version", "internal/cli", "internal/launch", "cmd/itos"];
const unreached = "internal/glob";
const testsReached = (hook: string, output: string) => {
	const ran = tested(output);
	expect(
		reached.every((p) => ran.has(p)) && !ran.has(unreached) && ran.size === reached.length,
		`${hook} should test ${reached.join(", ")} for a change to ${pkg}, tested ${show(ran)}`,
	);
};

let base = "";
try {
	base = repo.open();
	env.FALLOW_AUDIT_BASE = base;
	every = sh("go list ./cmd/... ./internal/...").output.split("\n").filter(Boolean);

	// 1. A harmless change to one package runs its tests and its importers',
	// passes, and is committed; the push of it does the same.
	edit(file, "// stamp is", "// go-hooks self-test: a harmless change\n// stamp is");
	let run = preCommit(`a harmless change to ${pkg}`);
	expect(run.status === 0, `pre-commit refused a harmless change to ${pkg}:\n${run.output}`);
	testsReached("pre-commit", run.output);
	let sha = commit("refactor: touch the version package");
	run = prePush("that change, committed", base, sha);
	expect(run.status === 0, `pre-push refused a harmless change to ${pkg}:\n${run.output}`);
	testsReached("pre-push", run.output);

	// 2. The same package, broken so its unit test fails: refused at commit,
	// in that package's test, and at push.
	git(`reset -q --hard ${base}`);
	edit(file, 'return strings.TrimPrefix(v, "v")', "return v");
	run = preCommit(`a change that breaks ${pkg}'s test`);
	expect(run.status !== 0, `pre-commit let through a change that breaks ${pkg}'s unit test`);
	expect(
		tested(run.output, "FAIL").has(pkg),
		`pre-commit should fail in ${pkg}, failed in ${show(tested(run.output, "FAIL"))}`,
	);
	sha = commit("refactor: break the version package");
	run = prePush("that change, committed", base, sha);
	expect(run.status !== 0, `pre-push let through a change that breaks ${pkg}'s unit test`);
	expect(tested(run.output, "FAIL").has(pkg), `pre-push did not fail in ${pkg}`);

	// 3. go.mod is read by every package's build: a change to it tests all.
	git(`reset -q --hard ${base}`);
	edit("go.mod", "module ", "// go-hooks self-test: a harmless change\nmodule ");
	run = preCommit("a change to go.mod");
	expect(run.status === 0, `pre-commit refused a comment in go.mod:\n${run.output}`);
	expect(
		tested(run.output).size === every.length,
		`a change to go.mod should test all ${every.length} packages, tested ${tested(run.output).size}`,
	);

	// 4. Prose runs no Go test, at commit or at push.
	git(`reset -q --hard ${base}`);
	edit("README.md", "# ", "A prose edit.\n\n# ");
	run = preCommit("a Markdown-only change");
	expect(run.status === 0, `pre-commit refused a Markdown-only change:\n${run.output}`);
	expect(!run.output.includes("go test"), `a Markdown-only commit ran Go tests:\n${run.output}`);
	sha = commit("docs: edit the readme");
	run = prePush("a prose-only push", base, sha);
	expect(run.status === 0, `pre-push refused a prose-only push:\n${run.output}`);
	expect(
		tested(run.output).size === 0,
		`a prose-only push ran Go tests: ${show(tested(run.output))}`,
	);
} finally {
	repo.remove();
}

console.log(timings.join("\n"));
for (const problem of problems) console.error(`FAIL ${problem}`);
console.log(
	problems.length
		? `\n${problems.length} Go hook check(s) failed`
		: `\nThe hooks run the Go unit tests a change reaches (${every.length} packages in all)`,
);
process.exit(problems.length ? 1 : 0);
