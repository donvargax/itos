// The scenario-moves rule: outside feat and fix, live scenarios may only
// change file, unchanged; @wip ones may come, go and change; a file's header
// stays; and an allowed rename passes, any other fails.
import { describe, expect, it } from "vite-plus/test";
import { moveProblems, readFeatures } from "./scenario-moves.ts";

const header = (title: string, tags = "@phase-1") =>
	`${tags}\nFeature: ${title}\n\n  Background:\n    Given the page is open\n`;
const scenario = (id: string, name: string, steps = '    When I press "Play"', tags = "") =>
	`\n  @${id}${tags}\n  Scenario: ${name}\n${steps}\n`;

const one = header("One") + scenario("ID-A-01", "First") + scenario("ID-A-02", "Second");
const two = header("Two") + scenario("ID-B-01", "Third");
const before = readFeatures({ "one.feature": one, "two.feature": two });

describe("scenario moves", () => {
	it("passes an unchanged set", () => {
		expect(moveProblems(before, before)).toEqual([]);
	});

	it("passes a move to another file, a new file and a file left empty and deleted", () => {
		const after = readFeatures({
			"two.feature": two + scenario("ID-A-02", "Second"),
			"three.feature": header("Three", "@phase-3") + scenario("ID-A-01", "First"),
		});
		expect(moveProblems(before, after)).toEqual([]);
	});

	it("fails a moved scenario whose steps, name or tags changed", () => {
		const steps = readFeatures({
			"one.feature": header("One") + scenario("ID-A-01", "First"),
			"two.feature": two + scenario("ID-A-02", "Second", '    When I press "Pause"'),
		});
		expect(moveProblems(before, steps)).toEqual([
			"changes the live scenario ID-A-02 in two.feature: a moved scenario keeps its ID, name, tags and steps exactly",
		]);
		const name = readFeatures({
			"one.feature": one.replace("Scenario: First", "Scenario: First, renamed"),
			"two.feature": two,
		});
		expect(moveProblems(before, name)).toHaveLength(1);
		const tags = readFeatures({
			"one.feature": one.replace("@ID-A-01", "@ID-A-01 @clock"),
			"two.feature": two,
		});
		expect(moveProblems(before, tags)).toHaveLength(1);
	});

	it("fails a lost or an added live scenario, and passes a @wip one", () => {
		const lost = readFeatures({ "one.feature": one, "two.feature": header("Two") });
		expect(moveProblems(before, lost)).toEqual(["loses the scenario ID-B-01 of two.feature"]);
		const added = readFeatures({
			"one.feature": one,
			"two.feature": two + scenario("ID-B-02", "Fourth"),
		});
		expect(moveProblems(before, added)).toEqual(["adds the live scenario ID-B-02 to two.feature"]);
		const pending = two + scenario("ID-B-02", "Fourth", undefined, " @wip");
		const wip = readFeatures({ "one.feature": one, "two.feature": pending });
		expect(moveProblems(before, wip)).toEqual([]);
		const changed = readFeatures({
			"one.feature": one,
			"two.feature": two + scenario("ID-B-02", "Fourth, reworded", undefined, " @wip"),
		});
		expect(moveProblems(wip, changed)).toEqual([]);
		expect(moveProblems(wip, before)).toEqual([]);
		// Turning it live is a feat's.
		expect(moveProblems(wip, added)).toHaveLength(1);
		// A file tagged @wip above its Feature line is @wip throughout.
		const spec = readFeatures({
			"one.feature": one,
			"two.feature": two,
			"four.feature": header("Four", "@phase-4 @wip") + scenario("ID-C-01", "Fifth"),
		});
		expect(moveProblems(before, spec)).toEqual([]);
	});

	it("fails a changed header or Background of a file with a live scenario", () => {
		const reworded = readFeatures({
			"one.feature": one.replace("Feature: One", "Feature: Won"),
			"two.feature": two,
		});
		expect(moveProblems(before, reworded)).toEqual([
			"changes the header or Background of one.feature",
		]);
		const background = readFeatures({
			"one.feature": one.replace(
				"the page is open",
				"the page is open\n    And the clock is paused",
			),
			"two.feature": two,
		});
		expect(moveProblems(before, background)).toHaveLength(1);
		// A file with only @wip scenarios may change its header.
		const pending = readFeatures({
			"five.feature": header("Five") + scenario("ID-D-01", "Sixth", undefined, " @wip"),
		});
		const retitled = readFeatures({
			"five.feature": header("Five, retitled") + scenario("ID-D-01", "Sixth", undefined, " @wip"),
		});
		expect(moveProblems(pending, retitled)).toEqual([]);
	});

	it("ignores comment lines in a header and around or inside a scenario", () => {
		// A reason written beside a scenario, in the header, above the tag line
		// or between the steps, is no change to what the rule compares.
		const commented = readFeatures({
			"one.feature": one
				.replace("Feature: One", "Feature: One\n  # why this file exists")
				.replace("\n  @ID-A-02", "\n  # why the second checks what it does\n  @ID-A-02")
				.replace("Scenario: First\n", "Scenario: First\n    # a step-level note\n"),
			"two.feature": two,
		});
		expect(moveProblems(before, commented)).toEqual([]);
		expect(moveProblems(commented, before)).toEqual([]);
		// A comment does not hide a real change beside it.
		const both = readFeatures({
			"one.feature": one.replace("\n  @ID-A-02", "\n  # a note\n  @ID-A-02 @clock"),
			"two.feature": two,
		});
		expect(moveProblems(before, both)).toHaveLength(1);
	});

	it("passes an allowed rename and no other", () => {
		const renames: Record<string, string> = {
			"ID-ITEM-01": "Items are listed in order, pairs kept together",
		};
		const items = header("Items") + scenario("ID-ITEM-01", "Items are listed in order");
		const allowed = items.replace(
			"Items are listed in order",
			"Items are listed in order, pairs kept together",
		);
		const moves = (from: string, to: string, allow = renames) =>
			moveProblems(readFeatures({ "s.feature": from }), readFeatures({ "s.feature": to }), allow);
		expect(moves(items, allowed)).toEqual([]);
		// Without the allowance, the same rename fails.
		expect(moves(items, allowed, {})).toHaveLength(1);
		const other = items.replace("listed in order", "listed by name");
		expect(moves(items, other)).toHaveLength(1);
		// The name alone may change: not a step beside it.
		const both = allowed.replace('I press "Play"', 'I press "Pause"');
		expect(moves(items, both)).toHaveLength(1);
	});
});
