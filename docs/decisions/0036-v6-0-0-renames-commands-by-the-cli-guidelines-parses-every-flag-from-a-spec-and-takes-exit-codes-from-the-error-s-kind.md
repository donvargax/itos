---
status: accepted
date: 2026-10-06
---

# v6.0.0 renames commands by the CLI guidelines, parses every flag from a spec and takes exit codes from the error's kind

## Context and Problem Statement

Which CLI changes does v6.0.0 carry, besides exit 75 and ci run's own exit code (slice-86) and itos version saying what answers it (slice-87)?

Asked as q-16.

## Considered Options

- Ship each breaking change in its own major release
- Keep old names as hidden aliases for one major
- Bundle every breaking change ready now into v6.0.0, old names refused with a pointer

## Decision Outcome

The user's calls, 2026-10-06, from the CLI review (docs/CLI.md). The Claude Code guard becomes itos guard claude-code; itos ask becomes itos question and itos follow becomes itos followup (asks.yaml, follow-ups.yaml and the q-<n> ids stay); hooks install becomes hook install; work promote --as becomes --id and work queue --drop becomes --remove; itos help with an unknown topic exits 2. An old name exits 2 naming the new one; nothing keeps an old interface working, no alias and no feature flag. Every command parses its flags from a declared spec: an unknown flag and a flag with no value are refused with exit 2, --flag=value is read as --flag value, a flag's value is never read as a global flag, and a ref argument is checked. Exit codes come from the kind of the error, never from a default: 2 usage or config, 3 missing environment, 75 a failure that may pass when run again, 70 (sysexits' EX_SOFTWARE) an error itos cannot classify. All of it ships in one push, with slice-86, slice-87 and T-101: each part is built by one agent at a time, committed and not pushed, and the coordinator pushes the stack once. Code that keeps an old interface working is removed after v6 (p1-drop-compat-code).

### Consequences

slice-88 (flag spec, features/cli.feature) and slice-89 (renames) join slice-86, slice-87 and T-101 in one push; the exit codes are 0, 1, 2, 3, 70 and 75; the plugin's guard.sh calls itos guard claude-code and its version rises; repositories pinned below v6 get no guard until they upgrade; docs/CLI.md's open questions are answered.
