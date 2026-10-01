// A problem a check reports: the sentence the text output prints, and for
// `--json` a stable `rule` id and, where one exists, a `fix` an agent can act
// on (the command or edit that resolves it).
export interface Problem {
	rule: string;
	message: string;
	fix?: string;
}

export const problem = (rule: string, message: string, fix?: string): Problem =>
	fix === undefined ? { rule, message } : { rule, message, fix };

export const messages = (found: Problem[]): string[] => found.map((p) => p.message);

// How one command reports (itos's global flags): `--json` prints one object on
// stdout, `schema: 1` first, and sends the logs to stderr; `-q` leaves out a
// check's success line.
export interface Output {
	json: boolean;
	quiet: boolean;
}

export const TEXT: Output = { json: false, quiet: false };

export const emit = (value: object) =>
	console.log(JSON.stringify({ schema: 1, ...value }, null, 2));

// Where a command's logs go: stdout in text, stderr under `--json`.
export const logger =
	(out: Output) =>
	(...args: unknown[]) =>
		out.json ? console.error(...args) : console.log(...args);
