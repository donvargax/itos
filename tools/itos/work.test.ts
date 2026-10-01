// The work registry as the project ships it. What the registry's rules, the
// proposal, the people sources and the identity providers do is
// conformance/work.yaml's.
import { describe, expect, it } from "vite-plus/test";
import { load, problems } from "./work.ts";

describe("the work registry", () => {
	it("is sound as it ships", () => {
		expect(problems(load())).toEqual([]);
	});

	it("knows the owners by the logins the people source lists", () => {
		expect(load().logins.length).toBeGreaterThan(0);
	});
});
