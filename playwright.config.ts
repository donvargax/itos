import { createHash } from "node:crypto";
import { defineConfig, devices } from "@playwright/test";
import { defineBddConfig } from "playwright-bdd";

const testDir = defineBddConfig({
	features: "e2e/features/**/*.feature",
	steps: ["e2e/steps/**/*.ts", "e2e/support/fixtures.ts"],
	outputDir: "e2e/.features-gen",
	// Specified but not yet implemented; see e2e/features/README.md.
	tags: "not @wip",
});

// Each checkout (the main one and every agent's worktree) serves on a port of
// its own, from a hash of its path, so two runs on one machine never share a
// server. E2E_PORT overrides it.
const checkout = Number.parseInt(
	createHash("sha1").update(process.cwd()).digest("hex").slice(0, 8),
	16,
);
const port = Number(process.env.E2E_PORT) || 5200 + (checkout % 400);
const baseURL = `http://127.0.0.1:${port}`;

export default defineConfig({
	testDir,
	// Scenarios are independent, so they spread by scenario, not by file.
	fullyParallel: true,
	// A local run stops at the failure worth reading; CI reports every one.
	maxFailures: process.env.CI ? 0 : 1,
	// CI runs a scenario once unrecorded and, only when it fails, once more with
	// the trace and the video. A scenario that fails and then passes is flaky,
	// and failOnFlakyTests keeps the run red, so the retry records a failure but
	// never hides one. Locally, no retry.
	retries: process.env.CI ? 1 : 0,
	failOnFlakyTests: !!process.env.CI,
	forbidOnly: !!process.env.CI,
	reporter: [["list"], ["html", { open: "never" }]],
	use: {
		...devices["Desktop Chrome"],
		baseURL,
		// Recording is what a run spends, so nothing is recorded on a first
		// attempt; CI records the retry of a scenario that failed.
		trace: process.env.CI ? "on-first-retry" : "off",
		screenshot: "only-on-failure",
		video: process.env.CI ? "on-first-retry" : "off",
	},
	// The scenarios exercise what `vp build` produces, served as it would be.
	webServer: {
		command: `vp build && vp preview --host 127.0.0.1 --port ${port} --strictPort`,
		url: baseURL,
		reuseExistingServer: false,
		timeout: 120_000,
	},
});
