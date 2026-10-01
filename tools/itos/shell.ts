// The one way itos starts a command the config or the ledger gives it: a task
// check, a CI step, a header-lint delegate, a range check, a provider's or a
// test adapter's command, the pre-push commands and the smoke run. Each goes
// through the shell the config names, `shell`: the argv prefix the command is
// appended to as one argument, `[sh, -c]` by default.
import {
	spawnSync,
	type SpawnSyncOptions,
	type SpawnSyncOptionsWithStringEncoding,
	type SpawnSyncReturns,
} from "node:child_process";
import { config } from "./config.ts";

export function inShell(
	command: string,
	options: SpawnSyncOptionsWithStringEncoding,
): SpawnSyncReturns<string>;
export function inShell(command: string, options?: SpawnSyncOptions): SpawnSyncReturns<unknown>;
export function inShell(command: string, options: SpawnSyncOptions = {}) {
	const [shell = "sh", ...flags] = config().shell ?? ["sh", "-c"];
	return spawnSync(shell, [...flags, command], options);
}
