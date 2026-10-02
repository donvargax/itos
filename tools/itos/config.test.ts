// What RE2 cannot compile (config.ts's lacksInRE2), the TypeScript's stand-in
// for compiling the config's patterns as RE2 until the switch. Each verdict
// here is Go's regexp.Compile's, the Go binary's, checked against it when it
// was written; lacksInRE2 agreed with Go on every pattern of a differential
// run of over 600,000, listed and fuzzed, but the two that reach RE2's caps
// on size and height (p2-pattern-re2-escapes). What config check prints for
// one is conformance/config.yaml's.
import { describe, expect, it } from "vite-plus/test";
import { lacksInRE2 } from "./config.ts";

const refused = (...patterns: string[]) => {
	for (const p of patterns) expect(lacksInRE2(p), p).toBe(true);
};
const compiled = (...patterns: string[]) => {
	for (const p of patterns) expect(lacksInRE2(p), p).toBe(false);
};

describe("what RE2 cannot compile", () => {
	it("is a lookaround outside a character class", () => {
		refused("(?=\\d)\\d+", "a(?!b)", "(?<=a)b", "(?<!a)b", "[]a(?=b)");
		compiled("[(?=]a", "\\(?=a\\)", "(?<name>a)", "(?:a)", "(?P<name>a)");
	});
	it("is a backreference's escape, in or out of a class", () => {
		refused("(\\d)\\1*", "(a)\\9", "(?<n>a)\\k<n>", "[\\1]", "a\\k", "\\8");
		compiled("\\\\1", "a\\12", "\\0", "\\123", "[\\12]", "\\d\\w\\s", "T-\\d+");
	});
	// RE2's letter escapes, from Go's regexp/syntax over every letter: outside
	// a class \A \b \B \d \D \Q \s \S \w \W \z and \a \f \n \r \t \v, inside
	// one only the last six and \d \D \s \S \w \W; \p, \P and \x take more.
	it("is a letter escape RE2 has no reading for", () => {
		const outside = "ABDQSWabdfnrstvwz";
		const inside = "DSWadfnrstvw";
		for (const c of "ABCDEFGHIJKLMNOQRSTUVWXYZabcdefghijklmnoqrstuvwyz") {
			expect(lacksInRE2(`\\${c}`), `\\${c}`).toBe(!outside.includes(c));
			expect(lacksInRE2(`[\\${c}]`), `[\\${c}]`).toBe(!inside.includes(c));
		}
		refused("\\cA", "\\c1", "[\\cA]", "\\e", "\\u0041", "\\E", "\\é");
		refused("\\x", "\\x4", "\\x4g", "\\x{}", "\\x{110000}", "\\x{g}");
		compiled("\\x41", "\\x{41}", "\\x{10FFFF}", "\\x{1001}", "[\\x{41}]", "\\-", "\\_", "\\ ");
		compiled("\\Q\\e\\E", "\\Qa(\\E", "\\Q\\c");
	});
	it("is a class RE2 cannot close, or a name it does not have", () => {
		refused("[]", "[^]", "[]a(?:b)", "[^](?:b)", "[\\b]", "[\\Q]", "[\\z]");
		refused("[a-\\d]", "[a-\\pL]", "[z-a]", "[a-\\a]", "[[:foo:]]", "[[:Alpha:]]", "[[:a]b:]]");
		refused("\\p{Foo}", "\\pq", "\\p", "\\p{}", "\\p{^}", "\\p{Script=Greek}", "\\p{L&}");
		compiled("[]a]", "[^]a]", "[\\d-z]", "[\\w-]", "[a-]", "[-a]", "[[:alpha:]]", "[[:^digit:]x]");
		compiled("\\pL", "\\PL", "\\pl", "\\p{Greek}", "\\p{^Greek}", "\\P{Lu}", "[\\p{Han}]");
		compiled("\\p{Old_Italic}", "\\p{old italic}", "\\p{OLDITALIC}", "\\p{Letter}", "\\p{Any}");
	});
	it("is a group or flags RE2 has no syntax for", () => {
		refused("(?i-:a)", "(?-:a)", "(?x:a)", "(?<$n>a)", "(?<é>a)", "(?<>a)", "(?#c)", "(?P=n)");
		compiled("(?i:a)", "(?-i:a)", "(?i-s:a)", "(?U:a)", "(?<_1>a)", "(?<1n>a)", "(?i)abc");
		refused("(a", "a)", "(?:a");
	});
	it("is a repeat count above 1000, nested counted repeats multiplying", () => {
		refused("a{1001}", "\\d{1,1001}", "a{1001,}", "a{0,1001}", "a{100000000}", "a{2,1}");
		refused("(a{100}){11}", "(?:a{2}b){501}", "((a{10}){10}){11}", "((a{2})*){501}");
		compiled("a{1000}", "a{0,1000}", "a{01001}", "a{,1001}", "(a{100}){10}", "(a{1000}b{1000}){0}");
		compiled("(a{1000}){1}", "(a{1000}){0,1}", "(a{500})*", "a{2}?", "x{2}y{1000}z{1000}");
	});
	it("is a repeat of nothing, or of a repeat", () => {
		refused("*a", "(*a)", "a|*", "a**", "a*??", "a{2}{3}", "a{2}*", "\\Q\\E*");
		compiled("^*", "\\b+", "a*?", "a{2}?", "()*", "a\\Q\\E*");
	});
});
