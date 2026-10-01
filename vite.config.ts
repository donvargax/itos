import { defineConfig, type UserConfig } from "vite-plus";

// What the pre-commit hook's `vp staged` formats and lints, by path.
const staged = {
	"{src,e2e,tools}/**/*.{ts,js,json,html,css}": "vp check --fix",
	"*.{md,json,yaml,ts,toml}": "vp check --fix",
	"{docs,tasks,.github}/**/*.{md,yml,yaml}": "vp check --fix",
};

const lint: NonNullable<UserConfig["lint"]> = {
	ignorePatterns: [
		"dist/**",
		"coverage/**",
		"e2e/.features-gen/**",
		"docs/changelog/**",
		".claude/**",
	],
	jsPlugins: [{ name: "vite-plus", specifier: "vite-plus/oxlint-plugin" }],
	rules: { "vite-plus/prefer-vite-plus-imports": "error" },
	overrides: [
		{
			// E2E tests drive the browser, never the production modules.
			files: ["e2e/**/*.ts"],
			rules: {
				"no-restricted-imports": [
					"error",
					{
						patterns: [
							{
								regex: "(^|/)src(/|$)",
								message: "E2E tests use the browser, not production modules.",
							},
						],
					},
				],
			},
		},
	],
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
	exclude: ["**/node_modules/**", "**/dist/**", "e2e/**", ".claude/**"],
	coverage: {
		provider: "v8" as const,
		reporter: ["text", "html", "json"],
		// The application's own modules; src/main.ts only wires them to the page,
		// and the scenarios cover it.
		include: ["src/**/*.ts"],
		exclude: ["**/*.test.ts", "src/main.ts"],
		thresholds: { statements: 90, branches: 75, functions: 90, lines: 90 },
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
		"e2e/.features-gen/**",
		"coverage/**",
		".claude/**",
	],
};

export default defineConfig({
	staged,
	lint,
	test,
	fmt,
});
