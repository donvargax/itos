import { expect } from "@playwright/test";
import { Given, Then } from "../support/fixtures.ts";

Given("the page is open", async ({ app }) => {
	await app.open();
});

Given("the page is open for {string}", async ({ app }, name: string) => {
	await app.open(name);
});

Then("the heading reads {string}", async ({ app }, text: string) => {
	await expect(app.heading).toHaveText(text);
});

Then("no page errors were reported", async ({ pageErrors }) => {
	expect(pageErrors).toEqual([]);
});
