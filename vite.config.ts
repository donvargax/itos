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
	ignorePatterns: ["dist/**", "docs/changelog/**", ".claude/**"],
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
	],
};

export default defineConfig({
	staged,
	lint,
	fmt,
});
