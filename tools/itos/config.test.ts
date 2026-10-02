// What RE2 lacks of a pattern JavaScript compiles (config.ts's lacksInRE2),
// the TypeScript's stand-in for compiling the config's patterns as RE2 until
// the switch. What config check prints for one is conformance/config.yaml's.
import { describe, expect, it } from "vite-plus/test";
import { lacksInRE2 } from "./config.ts";

describe("what RE2 lacks", () => {
	it("is a lookaround outside a character class", () => {
		for (const p of ["(?=\\d)\\d+", "a(?!b)", "(?<=a)b", "(?<!a)b"])
			expect(lacksInRE2(p)).toBe(true);
		for (const p of ["[(?=]a", "\\(?=a\\)", "(?<name>a)", "(?:a)", "[]a(?:b)", "[^](?:b)"])
			expect(lacksInRE2(p)).toBe(false);
		expect(lacksInRE2("[]a(?=b)")).toBe(true);
	});
	it("is a backreference's escape, in or out of a class", () => {
		for (const p of ["(\\d)\\1*", "(a)\\9", "(?<n>a)\\k<n>", "[\\1]", "a\\k"])
			expect(lacksInRE2(p)).toBe(true);
		for (const p of ["\\\\1", "a\\12", "\\0", "\\d\\w\\s", "T-\\d+"])
			expect(lacksInRE2(p)).toBe(false);
	});
});
