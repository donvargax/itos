// Package ci is the ci command, a slice (ADR-0053) with no domain of its
// own: its CLI and Run are the command line, its Query and Handler the
// application, over internal/plan, which makes a range's or the nightly's
// plan, internal/ci (imported as cirun), which carries one out, and
// internal/engine/ciwatch, which waits for a commit's CI run.
//
// Command (command.go) declares its name, help and flag spec, by which
// internal/cli reads the line and hands Parse the arguments after ci; every
// error Parse gives is a usage error. ci plan and ci scope print the plan
// and whether a range is prose alone, a typed range's refs checked to be
// commits (the refs engine) before the config is read; ci plan with no
// range asks the Stealth port first, as verify does. A pushed range is
// planned as a prose-only one when internal/engine/prtrust trusts a merged
// pull request's green head for it (slice 188), GitHub asked through
// providers.MergedPullsProvider with the Env port. ci run makes the same
// plan, cut to its run of named tests under --tests, and carries it out
// through the RunPlan port, annotating its failures when the Env port says
// GITHUB_ACTIONS is "true" (slice 198). ci range reads the config first, so a broken
// ci.range is a config error even with --base, then asks
// providers.RangeStart, its provider reading the Env port. ci watch waits
// for a commit's run through ciwatch.Watch, with the provider the Watcher
// port gives, which internal/cli binds to the watcher the end of itos push
// and work done use too. Handle reaches the world through Ports, which the
// slice owns and internal/cli binds, and is handed where ci run and ci
// watch write as they work.
package ci
