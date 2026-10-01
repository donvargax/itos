import { defineConfig, type UserConfig } from "vite-plus";

// What the pre-commit hook's `vp staged` formats and lints, by path.
const staged = {
	"tools/**/*.{ts,js,json}": "vp check --fix",
	"*.{md,json,yaml,ts,toml}": "vp check --fix",
	"{docs,tasks,features,.github}/**/*.{md,yml,yaml}": "vp check --fix",
	// Go: formatted in place, then vetted as a whole, since go vet reads
	// packages, not files (the files `vp staged` appends are ignored).
	"**/*.go": ["gofmt -w", "sh -c 'go vet ./...' go-vet"],
};

const lint: NonNullable<UserConfig["lint"]> = {
	ignorePatterns: ["dist/**", "coverage/**", "docs/changelog/**", ".claude/**"],
	jsPlugins: [{ name: "vite-plus", specifier: "vite-plus/oxlint-plugin" }],
	rules: { "vite-plus/prefer-vite-plus-imports": "error" },
	options: { typeAware: true, typeCheck: true },
};

const test = {
	passWithNoTests: true,
	// The conformance suite runs the task tool's command line in scratch
	// repositories; on a busy machine that outgrows Vitest's 5 s default.
	testTimeout: 30_000,
	// `--changed` (the git hooks) picks tests by what they import; a change to one
	// of these reruns everything instead. Written as the files themselves: the
	// defaults' `**/package.json/**` form never matches a changed file. The
	// config, the dependencies and the task tooling's config, which the tests
	// read from disk rather than import.
	forceRerunTriggers: [
		"**/package.json",
		"**/pnpm-lock.yaml",
		"**/{vitest,vite}.config.*",
		"**/itos.yaml",
	],
	// Agents' git worktrees live under .claude/worktrees/, inside this folder:
	// their tests are theirs, often half-written, and never this checkout's.
	exclude: ["**/node_modules/**", "**/dist/**", ".claude/**"],
	coverage: {
		provider: "v8" as const,
		reporter: ["text", "html", "json"],
		// itos's own modules. Collected for the audit, which scores a changed
		// function by it (.fallowrc.json), and held to no threshold: itos is
		// proven through its command line, by the features (features/) and the
		// conformance corpus, and a unit test's coverage cannot see a command
		// it spawns.
		include: ["tools/itos/*.ts"],
		exclude: ["**/*.test.ts"],
	},
};

const fmt = {
	useTabs: true,
	ignorePatterns: [
		// The changelog, written on demand by `vp run changelog`.
		"docs/changelog/**",
		// The conformance fixtures hold exact command output, byte for byte; the
		// formatter would reflow it.
		"tools/itos/conformance/**",
		"coverage/**",
		".claude/**",
	],
};

// The package a consumer installs (`vp pack`, then `npm pack`): tools/itos/main.ts
// and everything it imports, `yaml` included, in one file, so the install has no
// runtime dependencies and Node never meets TypeScript inside node_modules, where
// it strips no types. package.json's `bin` names it. In this repository
// tools/bin/itos runs the TypeScript instead: the hooks and CI judge the working
// tree, and tools/selftest/release.ts proves this file.
const pack: NonNullable<UserConfig["pack"]> = {
	entry: { itos: "tools/itos/main.ts" },
	format: "esm",
	platform: "node",
	target: "node24",
	dts: false,
	// Every import inlined, the commands' lazy ones too: one file, nothing else.
	deps: { alwaysBundle: [/.*/] },
	outputOptions: { codeSplitting: false },
	banner: { js: "#!/usr/bin/env node" },
};

export default defineConfig({
	staged,
	lint,
	test,
	fmt,
	pack,
});
