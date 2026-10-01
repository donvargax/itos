// itos's version, and whether it satisfies a config's `requires`: a list of comparators, each of `>=`, `>`, `<=`,
// `<`, `=` (or none) and a version, all of which must hold.
//
// The version is package.json's, written nowhere else, so a release is one build commit to it and a
// tag. Here Node reads the file beside the source; the bundle (`vp pack`) inlines it, so the
// installed itos reads nothing beside itself.
import manifest from "../../package.json" with { type: "json" };

export const VERSION: string = manifest.version;

const parts = (v: string) => v.split(".").map((n) => Number.parseInt(n, 10) || 0);

// Negative, zero or positive, as a is older than, the same as or newer than b.
function compare(a: string, b: string): number {
	const [x, y] = [parts(a), parts(b)];
	for (let i = 0; i < 3; i++) if ((x[i] ?? 0) !== (y[i] ?? 0)) return (x[i] ?? 0) - (y[i] ?? 0);
	return 0;
}

const HOLDS: Record<string, (c: number) => boolean> = {
	">=": (c) => c >= 0,
	">": (c) => c > 0,
	"<=": (c) => c <= 0,
	"<": (c) => c < 0,
	"=": (c) => c === 0,
	"": (c) => c === 0,
};

export function satisfies(version: string, range: string): boolean {
	return range
		.trim()
		.split(/\s+/)
		.filter(Boolean)
		.every((comparator) => {
			const m = /^(>=|>|<=|<|=)?v?(\d+(?:\.\d+){0,2})$/.exec(comparator);
			return m ? HOLDS[m[1] ?? ""]!(compare(version, m[2]!)) : false;
		});
}
