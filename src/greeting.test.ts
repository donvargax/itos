import { describe, expect, it } from "vite-plus/test";
import { greeting } from "./greeting.ts";

describe("greeting", () => {
	it("greets the world when no name is given", () => {
		expect(greeting(null)).toBe("Hello, world!");
		expect(greeting("   ")).toBe("Hello, world!");
	});

	it("greets a visitor by name", () => {
		expect(greeting(" Ada ")).toBe("Hello, Ada!");
	});
});
