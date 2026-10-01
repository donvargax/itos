import type { Locator, Page } from "@playwright/test";

// The page as a visitor sees it: its address and its heading.
export class App {
	readonly heading: Locator;
	private readonly page: Page;

	constructor(page: Page) {
		this.page = page;
		this.heading = page.getByRole("heading", { level: 1 });
	}

	// Opens the page, with a name in its address when one is given.
	async open(name?: string): Promise<void> {
		await this.page.goto(name === undefined ? "/" : `/?name=${encodeURIComponent(name)}`);
	}
}
