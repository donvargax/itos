// The greeting the page shows: the visitor's name when the address gives one,
// else the world.
export function greeting(name: string | null | undefined): string {
	const who = name?.trim();
	return `Hello, ${who ? who : "world"}!`;
}
