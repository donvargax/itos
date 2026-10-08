// Package proof is the proof of done that itos work done requires beyond the
// landing (slice 100; decision 43, docs/plans/v7-proof-of-done.md): a
// task's proof is computed from its diff, never written by the model that
// did the work. Core itos owns the rule, a provider measures, and itos reads
// the provider by its exit code and its --json alone, never its files.
//
// Today there is one proof, the code's (config proof.code). Touches says
// whether an item's commits touch the paths that count as code, so need it;
// Command writes the provider's check with {base}, the parent of the item's
// first commit, which work done finds (internal/cli's workdone.go); Run runs
// it through the config's shell, its stderr passed through, its stdout kept
// for Read.
//
// Read judges the provider's answer by itos-cc's machine contract (its
// docs/CLI.md): exit 0 passes, whatever it prints; exit 1 with the --json
// object {"schema":1,"ok":false,"problems":[{"rule","message","fix",…}]}
// refuses, its problems kept (the keys besides rule, message and fix are the
// problem's subject, read by no one here); anything else cannot run: a
// command that did not start or was stopped, an exit 1 whose stdout is no
// such object, and every other code, 2 (usage or config), 3 (environment),
// 70 (internal) and 75 (temporary) among them. A check that cannot run is
// never a pass: an adapter's failure is an error, and nothing closes an item
// past a proof that did not pass (the user's call, 2026-10-07). There is no
// bypass, no flag and no variable.
package proof
