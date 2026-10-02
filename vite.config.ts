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

// The package a consumer installs (`vp pack`, then `npm pack`): tools/itos/main.ts
// and everything it imports, `yaml` included, in one file, so the install has no
// runtime dependencies and Node never meets TypeScript inside node_modules, where
// it strips no types. package.json's `bin` names it. In this repository
// tools/bin/itos runs the Go binary and tools/bin/itos-ts the working tree's
// TypeScript unbundled, and tools/selftest/release.ts proves this file.
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
	fmt,
	pack,
});
