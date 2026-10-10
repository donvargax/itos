import { defineConfig, type UserConfig } from "vite-plus";

// What the pre-commit hook's `vp staged` formats and lints, by path.
const staged = {
	"tools/**/*.{ts,js,json}": "vp check --fix",
	"*.{md,json,yml,yaml,ts,toml}": "vp check --fix",
	"{docs,tasks,features,.github}/**/*.{md,yml,yaml}": "vp check --fix",
	// The Claude Code plugin and the marketplace that names it (T-066).
	".claude-plugin/*.json": "vp check --fix",
	"integrations/**/*.{ts,json,md}": "vp check --fix",
	// Go: formatted in place, then vetted as a whole, since go vet reads
	// packages, not files (the files `vp staged` appends are ignored).
	"**/*.go": ["gofmt -w", "sh -c 'go vet ./...' go-vet"],
};

const lint: NonNullable<UserConfig["lint"]> = {
	ignorePatterns: [
		"dist/**",
		"docs/changelog/**",
		".claude/**",
		// The Claude Code plugin's hooks module and tests (T-066) are typed by the declarations
		// Claude Code lays beside a plugin it loads (.claude-plugin/types/, git-ignored), which
		// no install of this repository has, so the type-aware lint cannot judge them. Claude
		// Code judges them instead: the nightly's steps run claude plugin validate --strict
		// (T-125 moved them there from T-066's checks), which reads the module as the engine
		// will, and claude plugin test. The formatter and the audit still read them.
		"integrations/claude-code/**",
	],
	jsPlugins: [{ name: "vite-plus", specifier: "vite-plus/oxlint-plugin" }],
	rules: { "vite-plus/prefer-vite-plus-imports": "error" },
	options: { typeAware: true, typeCheck: true },
};

const fmt = {
	useTabs: true,
	ignorePatterns: [
		// The changelog, written on demand by `vp run changelog`.
		"docs/changelog/**",
		// The conformance fixtures hold exact command output, byte for byte; the
		// formatter would reflow it.
		"tools/itos/conformance/**",
		".claude/**",
		// The questions, the work registry and the ledgers: itos writes and commits them
		// itself, past the staged formatter, so their layout is itos's (T-137).
		"tasks/*.yaml",
	],
};

export default defineConfig({
	staged,
	lint,
	fmt,
});
