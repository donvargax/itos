---
status: accepted
date: 2026-10-06
---

# Only machine output is itos's contract: exit codes, --json less message and fix, and the files it writes

## Context and Problem Statement

slice-82 changes the words of one v5.0.8 corpus case: config check still refuses builtin moves on a command kind without supports_at, same rule (config-range-check-builtin), exit 2 and problem count, but the message now names supports_at instead of 'reads feature files'. previous-release judges a config error's words (T-095), and a feat may not change an old case, so it is a breaking change as the gate stands. Recommended: previous-release judges an old config error by its exit code, rule ids and problem count, not its message and fix text (a ci task before slice-82 lands), since a refusal reworded when its reason changes breaks no consumer and every such refinement would otherwise cost a major. Or: land slice-82 as breaking, v6.0.0. Or: keep the old, now untrue, message when supports_at is absent (not recommended). The finished patch waits in the scratchpad (slice-82-feat.patch).

Asked as q-15, about slice-82.

## Considered Options

- Judge an old config error by its exit code, rule ids and problem count only (the narrow fix for slice 82)
- Judge only machine output everywhere, announced in a major release
- Keep judging every word, and land slice 82 as a breaking change

## Decision Outcome

Only machine output is itos's contract: its exit codes, its --json output less every key named message or fix, and the files it writes. Every plain-text output, a refusal's or a success's, is for people and may change in any release; message and fix in --json are the same sentences and may change with them, so a script reads the rule id and the other keys, never a message. The exit codes keep grep's and diff's model: 0 success, 1 a check said no, 2 usage or config error, 3 missing environment; v6 adds 75 (sysexits' EX_TEMPFAIL) for a failure that may pass if run again unchanged (a network failure, a server error or rate limit itos gave up retrying, a run still going at ci.watch.timeout), so a retry wrapper can act on it without --json. ci run stops passing a failing step's exit code through, which could read as 75: it exits 1 and names the step's code. previous-release judges an old case by that contract alone (T-100), replacing the help and usage-error exemptions; this tree's own corpus still pins every word. v6.0.0 announces the contract in a BREAKING-CHANGE footer and carries 75 and ci run's code (slice-86, with the /v6 module path, T-101). Per-rule structured fields in --json problems, so no script needs a message, come after v6 as additive keys. slice-82 lands after T-100 as a plain feat.

### Consequences

previous-release no longer compares an old case's plain stdout or stderr, nor message and fix in its json (T-100); this replaces the part of 0021 that kept a config error's words judged, and its help and usage-error exemptions become the general rule. Exit 75 is added and ci run exits 1 for a failing step (slice-86), released as v6.0.0 with the contract in its BREAKING-CHANGE footer. Renaming a rule id or a json key, or changing an exit code, is a breaking change. Structured per-rule fields in --json problems follow as additive keys.
