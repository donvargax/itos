import { test } from "node:test";
import { resolveNativeGit } from "./run.ts";
import { runnerRegressions } from "./run-regressions.ts";

for (const regression of runnerRegressions) {
	test(regression.name, () => regression.run(resolveNativeGit()));
}
