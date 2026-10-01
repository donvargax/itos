import { greeting } from "./greeting.ts";

// The page's one element, filled in from the pure function below it.
const app = document.querySelector<HTMLElement>("#app");
if (app) {
	const heading = document.createElement("h1");
	heading.textContent = greeting(new URLSearchParams(location.search).get("name"));
	app.append(heading);
}
