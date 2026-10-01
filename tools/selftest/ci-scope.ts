// The prose shortcut must cover prose and nothing else. A path that reaches
// any gate — a task, a scenario, a config, a source file — must send the push
// down the full pipeline. This holds the project's own prose paths
// (itos.yaml's `ci.prose.paths`) to that.
//
// What a prose-only range plans (the prose steps, the named tasks' static and
// `prose: true` checks, nothing late) and what waits for a task not started
// are the tool's behaviour, whatever the config: they are
// tools/itos/conformance/plans.yaml's and ci-run.yaml's.
import { strict as assert } from "node:assert";
import { docsOnly } from "../itos/ci-scope.ts";

assert.equal(docsOnly(["docs/HANDOFF.md", "AGENTS.md", "PLAN.md"]), true);
assert.equal(docsOnly(["docs/decisions/format.md"]), true);

assert.equal(docsOnly(["AGENTS.md", "src/greeting.ts"]), false);
assert.equal(docsOnly(["tasks/phase-0.yaml"]), false);
assert.equal(docsOnly(["e2e/features/app.feature"]), false);
assert.equal(docsOnly(["package.json"]), false);
assert.equal(docsOnly([".github/workflows/ci.yml"]), false);

// Nothing known changed (a first push, a shallow clone): run everything.
assert.equal(docsOnly([]), false);

console.log("ci scope: the project's prose paths hold prose and nothing else");
