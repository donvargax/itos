// The plugin's version rule (T-074, tools/bin/plugin-version) in a scratch
// repository laid out as this one is, the plugin in integrations/claude-code/:
//
//   - a range that changes the plugin and leaves plugin.json's version as it
//     was is refused, naming the files changed, the version at the start and
//     at the end, and the versions to raise it to;
//   - one that raises it (a patch, a minor, a major, a pre-release's release)
//     passes;
//   - one that lowers it, or raises only a pre-release below it, is refused;
//   - one that leaves the plugin alone passes, saying so, even when the
//     version was not raised;
//   - a change undone within the range is no change;
//   - a shallow clone, no range start, or a start that is not a commit of
//     the clone stops it with exit 2, never a pass;
//
// and itos.yaml's ci runs it over the range, in its steps and in a prose-only
// range's (the plugin's skill is Markdown).
import { spawnSync } from "node:child_process";
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { parse } from "yaml";
import { finish, outsideEnv, releaseRepos } from "./scratch.ts";

const tmp = mkdtempSync(join(tmpdir(), "plugin-version-selftest-"));
const repo = join(tmp, "repo");
const bin = join(tmp, "plugin-version");
const problems: string[] = [];
const expect = (ok: boolean, problem: string) => ok || problems.push(problem);

const { env, git, shallow } = releaseRepos(tmp);
const manifest = "integrations/claude-code/.claude-plugin/plugin.json";
const write = (file: string, text: string) => {
	const at = join(repo, file);
	mkdirSync(dirname(at), { recursive: true });
	writeFileSync(at, text);
};
const setVersion = (version: string) =>
	write(manifest, `${JSON.stringify({ name: "itos", version }, null, "\t")}\n`);
const commitAll = (message: string) => {
	git(repo, "add", "-A");
	git(repo, "commit", "-q", "--allow-empty", "-m", message);
	return git(repo, "rev-parse", "HEAD");
};
const check = (cwd: string, args: string[]) => {
	const r = spawnSync(bin, args, { cwd, env, encoding: "utf8" });
	return { status: r.status, output: `${r.stdout}${r.stderr}` };
};

try {
	const built = spawnSync("go", ["build", "-o", bin, "./tools/bin/plugin-version"], {
		cwd: resolve("."),
		env: outsideEnv(),
		encoding: "utf8",
	});
	if (built.status !== 0) throw new Error(`go build ./tools/bin/plugin-version:\n${built.stderr}`);

	// The start: a plugin at 2.3.0, and a file outside it.
	git(tmp, "init", "-q", "-b", "main", repo);
	setVersion("2.3.0");
	write("integrations/claude-code/hooks/guard.sh", "echo guard\n");
	write("integrations/claude-code/skills/itos/SKILL.md", "# itos\n");
	write("README.md", "# readme\n");
	const start = commitAll("chore: the plugin at 2.3.0");
	const branch = (name: string) => git(repo, "checkout", "-q", "-b", name, start);

	// 1. The plugin changed, its version not: refused, naming the files, both
	// versions and the remedy.
	branch("unbumped");
	write("integrations/claude-code/hooks/guard.sh", "echo guard, changed\n");
	commitAll("chore: change the guard");
	write("integrations/claude-code/skills/itos/SKILL.md", "# itos, changed\n");
	commitAll("docs: change the skill");
	let r = check(repo, ["-range-from", start]);
	expect(
		r.status === 1 &&
			r.output.includes("integrations/claude-code/hooks/guard.sh") &&
			r.output.includes("integrations/claude-code/skills/itos/SKILL.md") &&
			r.output.includes("is still 2.3.0") &&
			r.output.includes("it was 2.3.0") &&
			r.output.includes(manifest) &&
			r.output.includes("2.3.1 for a fix, 2.4.0 for a new behaviour, 3.0.0 for a breaking change"),
		`a plugin change without a bump should be refused (exit 1), naming the files, the versions and the remedy, exited ${r.status}:\n${r.output}`,
	);

	// 2. Raised: each passes, saying from what to what.
	for (const [from, to] of [
		["2.3.0", "2.3.1"],
		["2.3.0", "2.4.0"],
		["2.3.0", "3.0.0"],
		["2.4.0-rc.1", "2.4.0"],
		["2.4.0-rc.1", "2.4.0-rc.2"],
		["2.4.0-alpha", "2.4.0-beta"],
		["2.3.9", "2.3.10"],
	]) {
		git(repo, "checkout", "-q", "--detach", start);
		setVersion(from);
		const at = commitAll(`chore: the plugin at ${from}`);
		write("integrations/claude-code/hooks/guard.sh", `echo guard for ${to}\n`);
		setVersion(to);
		commitAll(`chore: raise the plugin to ${to}`);
		r = check(repo, ["-range-from", at]);
		expect(
			r.status === 0 && r.output.includes(`went from ${from} to ${to}`),
			`a plugin change raising ${from} to ${to} should pass, exited ${r.status}:\n${r.output}`,
		);
	}

	// 3. Lowered, or equal by precedence: refused.
	for (const [from, to, how] of [
		["2.3.0", "2.2.9", "went down to 2.2.9"],
		["2.3.0", "2.3.0-rc.1", "went down to 2.3.0-rc.1"],
		["2.4.0", "2.4.0+build.7", "is still 2.4.0+build.7"],
		["2.3.10", "2.3.9", "went down to 2.3.9"],
	]) {
		git(repo, "checkout", "-q", "--detach", start);
		setVersion(from);
		const at = commitAll(`chore: the plugin at ${from}`);
		write("integrations/claude-code/hooks/guard.sh", `echo guard for ${to}\n`);
		setVersion(to);
		commitAll(`chore: set the plugin to ${to}`);
		r = check(repo, ["-range-from", at]);
		expect(
			r.status === 1 && r.output.includes(how) && r.output.includes(`it was ${from}`),
			`a plugin change from ${from} to ${to} should be refused (exit 1), saying "${how}", exited ${r.status}:\n${r.output}`,
		);
	}

	// 4. A version that is not semver at the end: refused.
	git(repo, "checkout", "-q", "--detach", start);
	setVersion("2.4");
	commitAll("chore: a version that is not semver");
	r = check(repo, ["-range-from", start]);
	expect(
		r.status === 1 && r.output.includes("not semver"),
		`a version that is not semver should be refused (exit 1), exited ${r.status}:\n${r.output}`,
	);

	// 5. The plugin left alone passes, saying so; a change undone in the range
	// is none.
	branch("alone");
	write("README.md", "# readme, changed\n");
	commitAll("docs: change the readme");
	r = check(repo, ["-range-from", start]);
	expect(
		r.status === 0 && r.output.includes("nothing under integrations/claude-code/ changed"),
		`a range that leaves the plugin alone should pass, saying so, exited ${r.status}:\n${r.output}`,
	);
	write("integrations/claude-code/hooks/guard.sh", "echo guard, for a while\n");
	commitAll("chore: change the guard");
	write("integrations/claude-code/hooks/guard.sh", "echo guard\n");
	commitAll("revert: change the guard back");
	r = check(repo, ["-range-from", start]);
	expect(
		r.status === 0 && r.output.includes("nothing under integrations/claude-code/ changed"),
		`a plugin change undone within the range should pass, exited ${r.status}:\n${r.output}`,
	);

	// 6. No range start, a start not in the clone, and a shallow clone stop it.
	git(repo, "checkout", "-q", "unbumped");
	r = check(repo, ["-range-from", ""]);
	expect(
		r.status === 2 && r.output.includes("no range start"),
		`no range start should stop the check (exit 2), exited ${r.status}:\n${r.output}`,
	);
	r = check(repo, ["-range-from", "0123456789abcdef0123456789abcdef01234567"]);
	expect(
		r.status === 2 && r.output.includes("is not a commit of this clone"),
		`a range start the clone lacks should stop the check (exit 2), exited ${r.status}:\n${r.output}`,
	);
	r = check(shallow(repo), ["-range-from", start]);
	expect(
		r.status === 2 && r.output.includes("shallow"),
		`a shallow clone should stop the check (exit 2), exited ${r.status}:\n${r.output}`,
	);

	// 7. itos.yaml's ci runs it over the range, in its steps and its prose steps.
	const command = 'go run ./tools/bin/plugin-version -range-from "${FROM-}"';
	const ci = parse(readFileSync("itos.yaml", "utf8")).ci;
	expect(
		ci.steps.some((s: { run?: string }) => s.run === command),
		`itos.yaml's ci.steps should run ${command}`,
	);
	expect(
		ci.prose.steps.includes(command),
		`itos.yaml's ci.prose.steps should run ${command}: the plugin's skill is Markdown`,
	);
} catch (error) {
	problems.push((error as Error).message);
} finally {
	rmSync(tmp, { recursive: true, force: true });
}

finish(
	problems,
	"plugin version",
	"The plugin's version rule refuses a plugin change that does not raise the version, passes one that does and a range that leaves the plugin alone, and never passes a range it cannot read",
);
