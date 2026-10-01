import { test as base, createBdd } from "playwright-bdd";
import { App } from "./app.ts";

// The page objects, one per area of the interface: a step destructures the
// ones it drives and never locates an element itself.
export interface PageObjects {
	app: App;
}

export const test = base.extend<{ pageErrors: string[] } & PageObjects>({
	// Every uncaught error the page throws, for the steps that say there is none.
	pageErrors: async ({ page }, use) => {
		const errors: string[] = [];
		page.on("pageerror", (error) => errors.push(error.message));
		await use(errors);
	},
	app: async ({ page }, use) => {
		await use(new App(page));
	},
});

export const { Given, When, Then } = createBdd(test);
