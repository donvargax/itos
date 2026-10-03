import { expect, test } from "claude-code/testing";
import { Annotator, annotateLine, parseRegistry } from "./titles";

const REGISTRY = `items:
  - id: T-061
    title: v2.0.0, Go only
    phase: 3
  - id: slice-25
    title: "The built-in header lint replaces commitlint"
  - id: p2-stealth-mode
    title: A local mode
`;
const titles = parseRegistry(REGISTRY);

test("reads each item and its title, quotes stripped", async () => {
	expect(titles.get("T-061")).toBe("v2.0.0, Go only");
	expect(titles.get("slice-25")).toBe("The built-in header lint replaces commitlint");
	expect(titles.size).toBe(3);
});

test("a bare ID and an ID alone in backticks gain their title", async () => {
	expect(annotateLine("next is T-061 then `slice-25`.", titles)).toBe(
		"next is `T-061: v2.0.0, Go only` then `slice-25: The built-in header lint replaces commitlint`.",
	);
});

test("an ID already titled, inside a longer code span, or inside a word is left alone", async () => {
	const line =
		"`T-061: v2.0.0, Go only`, run `tools/bin/itos task T-061`, see T-0611 and tasks/T-061.md";
	expect(annotateLine(line, titles)).toBe(line);
	expect(annotateLine("T-061 (v2.0.0, Go only) lands", titles)).toBe(
		"T-061 (v2.0.0, Go only) lands",
	);
});

test("pieces split mid-ID come out whole, and fenced code is untouched", async () => {
	const a = new Annotator(titles);
	const out = [
		a.push("start T-0"),
		a.push("61 now\n```\nT-061\n```\nend p2-stealth"),
		a.push("-mode"),
		a.flush(),
	].join("");
	expect(out).toBe(
		"start `T-061: v2.0.0, Go only` now\n```\nT-061\n```\nend `p2-stealth-mode: A local mode`",
	);
});

test("a slice written with a space gains its title, as the registry names it", async () => {
	expect(annotateLine("Slice 25 lands, then slice 25 again; slice 99 is unknown.", titles)).toBe(
		"`slice-25: The built-in header lint replaces commitlint` lands, then `slice-25: The built-in header lint replaces commitlint` again; slice 99 is unknown.",
	);
	expect(
		annotateLine(
			"`slice-25: The built-in header lint replaces commitlint` and slices 18 to 23",
			titles,
		),
	).toBe("`slice-25: The built-in header lint replaces commitlint` and slices 18 to 23");
});
