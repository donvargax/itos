# CLI design guidelines

These are the rules for the itos command line. Each rule names its source. When itos does not follow a rule yet, the rule says so and the gap is listed in [Where itos does not follow these rules yet](#where-itos-does-not-follow-these-rules-yet).

Use these rules when you add or change a command, a flag, an exit code or an output. Decision records in `docs/decisions/` can change a rule. When a decision changes a rule, change this document in the same commit.

## Sources

| Key      | Source                                                                                                                                                                       |
| -------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| CLIG     | Command Line Interface Guidelines, <https://clig.dev>. The anchor after the key names the section, for example CLIG `#help`.                                                 |
| GNU-CLI  | GNU Coding Standards, "Standards for Command Line Interfaces", <https://www.gnu.org/prep/standards/html_node/Command_002dLine-Interfaces.html>                               |
| GNU-VER  | GNU Coding Standards, "--version", <https://www.gnu.org/prep/standards/html_node/_002d_002dversion.html>                                                                     |
| GNU-HELP | GNU Coding Standards, "--help", <https://www.gnu.org/prep/standards/html_node/_002d_002dhelp.html>                                                                           |
| GNU-ERR  | GNU Coding Standards, "Formatting Error Messages", <https://www.gnu.org/prep/standards/html_node/Errors.html>                                                                |
| POSIX    | POSIX.1-2024 Base Definitions, section 12.2, "Utility Syntax Guidelines", <https://pubs.opengroup.org/onlinepubs/9799919799/basedefs/V1_chap12.html#tag_12_02>               |
| COBRA    | Cobra README, "Concepts", <https://github.com/spf13/cobra/blob/main/README.md#concepts>                                                                                      |
| D35      | Decision 35, [Only machine output is itos's contract](decisions/0035-only-machine-output-is-itos-s-contract-exit-codes-json-less-message-and-fix-and-the-files-it-writes.md) |

## The contract with scripts

Scripts can rely on three things only (D35):

- The exit code.
- The `--json` output, less every key named `message` or `fix`. These keys hold the same sentences as the plain output.
- The data of the files that itos writes. Their comment lines are for people, like the plain output, and can change in any release (decision 40).

All plain output is for people. It can change in any release. A script reads `--json` and the exit code, never the plain output and never a `message`.

The `previous-release` check holds each release to this contract (T-100). It runs the scenarios and the corpus cases of the last release against the new binary. A change to the contract is a breaking change, and a breaking change makes a major release.

### Exit codes

| Code | Meaning                                                                                                                   |
| ---- | ------------------------------------------------------------------------------------------------------------------------- |
| 0    | Success.                                                                                                                  |
| 1    | A check said no: a rule refused a commit, a check failed, an item is not ready.                                           |
| 2    | A usage error or a config error.                                                                                          |
| 3    | The environment is missing something: a person, a tool, a release, a git repository, itos's hooks in the git config.      |
| 70   | An internal error: itos met an error that it cannot classify. Report it.                                                  |
| 75   | A temporary failure. The same command can pass when you run it again with no change, for example after a network failure. |

These codes follow grep and diff (0 yes, 1 no, 2 trouble) and the BSD `sysexits.h` values `EX_SOFTWARE` for 70 and `EX_TEMPFAIL` for 75 (CLIG `#the-basics`: map the non-zero codes to the most important failure modes). Decision 36 sets 70 and 75.

## Rules

### Command names and grammar

1. Keep the program name short and lowercase. (POSIX guidelines 1 and 2; CLIG `#naming`.) itos follows this rule.
2. Write a subcommand name in lowercase, with dashes between words: `check-paths`, `next-id`. itos follows this rule.
3. Name a group of commands with a noun. Name an action in a group with a verb in the imperative: `work take`, `config get`. (CLIG `#subcommands`; COBRA `#concepts`, `APPNAME VERB NOUN --ADJECTIVE` or `APPNAME COMMAND ARG --FLAG`.) Some groups do not follow this rule yet: `commit`, `verify`. `ask` and `follow` became the groups `decision` and `followup` in v6.0.0 (slice-89, slice-92).
4. Use the singular for a group name. Some groups do not follow this rule yet: `tests`. `hooks install` became `hook install` in v6.0.0 (slice-89).
5. Do not give two commands similar names or overlapping meanings. (CLIG `#subcommands`.) itos follows this rule: since v6.0.0 (slice-89) `hooks install` is `hook install`, and no `hooks` group stands beside `hook`.
6. Do not name a command with an everyday verb when the same verb in a request can point to a different command. An agent picks a command by its name before it reads the help. Example: "I need to ask someone this" meant `itos follow`, but the word "ask" pointed to `itos ask`. (This rule comes from use of itos, and agrees with CLIG `#subcommands`.) itos follows this rule: since v6.0.0 (slice-89, slice-92) they are `itos followup` and `itos decision`. A name says who answers, not what is asked (decision 38): `itos question` was still read as "ask someone a question", so the group became `itos decision`.
7. Do not add a new implicit default subcommand, a command that runs an action when you give it no subcommand. (CLIG `#future-proofing`, "Don't have a catch-all subcommand".) The existing ones stay: `task <id>`, `work`, `decision`, `followup`, and `draft` (slice-96), which lists its drafts as `followup` lists its threads.
8. Do not let a command accept an abbreviation of a subcommand. Make an alias only when you name it explicitly. (CLIG `#future-proofing`.) itos follows this rule.

### Help and version

9. Show help for `itos`, `itos --help`, `itos help <command>` and `<command> --help`, and for `-h` in any position. Write help to stdout and exit 0. (CLIG `#help`; GNU-HELP.) itos follows this rule.
10. In the help of each command, give the shape of its `--json` output and its exit codes. itos follows this rule.
11. Support `--version` and `version`. The first line of the output is `itos <version>`. (GNU-CLI; GNU-VER; CLIG `#arguments-and-flags`.) itos follows this rule: a first argument `--version` is `itos version` (slice-87).
12. For an unknown command, exit 2. If you can guess the command that the person meant, name it. (CLIG `#help`.) `itos help nosuch` exits 2 (slice-89), and itos gives no suggestions yet.
13. For a group with no subcommand, name the subcommands that the group takes. `itos hook` does not do this yet.
14. End the help with an example or two and the address for issue reports. (CLIG `#help`; GNU-HELP.) itos does not do this yet.

### Flags and arguments

15. Give each flag a long form. Give a one-letter form only to the most common flags. (CLIG `#arguments-and-flags`; GNU-CLI.) itos follows this rule: `-h` and `-q` are its only short flags.
16. Use the standard name when a standard name exists: `--json`, `-q`/`--quiet`, `-h`/`--help`, `--force`, `--version`. (CLIG `#arguments-and-flags`.)
17. Let a flag mean the same thing in every command. (CLIG `#subcommands`.) itos follows this rule: `--as` names a person in `work`, `status` and `work take`, and `work promote` takes the new ID as `--id` (slice-89).
18. Use a flag to change an action, never to select a different action. (COBRA `#concepts`.) `config check --print-defaults` does not follow this rule yet; `work queue --drop` became `--remove` in v6.0.0 (slice-89).
19. Accept `--flag=value` and `--flag value`. (CLIG `#arguments-and-flags`; GNU-CLI, as `getopt_long` reads them.) itos follows this rule (slice-88): every built-in command reads its flags by one spec, `internal/cli/spec.go`.
20. Refuse an unknown flag, and a flag with no value, with exit 2. (CLIG `#robustness-guidelines`.) itos follows this rule (slice-88), and also refuses a switch given a value and a once-only flag given twice.
21. Give an option-argument to its option only. Never read it as a global flag. (POSIX guidelines 6 and 14.) itos follows this rule (slice-88): `task list --group --json` is a `--group` with no value, exit 2.
22. Do not make an option-argument optional. (POSIX guideline 7.) `init --plugin [<scope>]` does not follow this rule yet.
23. Let `--` end the options, and let `-` mean stdin or stdout. (POSIX guidelines 10 and 13; CLIG `#arguments-and-flags`.) itos follows this rule.
24. Accept flags in any position. (CLIG `#arguments-and-flags`.) itos follows this rule.
25. Check each argument before you use it, and refuse a bad one with exit 2. (CLIG `#robustness-guidelines`.) itos follows this rule for refs (slice-88): `ci plan nosuchref HEAD` exits 2, naming the ref. A CI range's start may be a full commit name the repository lacks, which the plan reads as a range it cannot read.

### Output

26. Write the main output to stdout. Write logs, progress and errors to stderr. (CLIG `#the-basics`.) itos follows this rule.
27. Print JSON only with `--json`. `--json` prints one object with `"schema": 1`, and a later release only adds keys to it. (CLIG `#output`; D35.) `config get` does not follow this rule yet: it prints a mapping or a list as JSON with no flag.
28. When the plain output of a command looks like data, such as the YAML from `config get`, write a line on stderr that tells the reader to use `--json` in scripts.
29. With `--json`, print the object for every failure too: `"ok": false` and the rule ID of each problem. (CLIG `#output`; D35.) Now a usage error or an unexpected error prints nothing on stdout.
30. Do not use colour, and pass `NO_COLOR` on to the programs that itos runs. (CLIG `#output`, `#environment-variables`.) itos follows this rule.

### Errors and exit codes

31. Get the exit code from the kind of the error, never from a default. (CLIG `#the-basics`.) itos follows this rule (slice-86): an error takes its kind where it is made (`internal/kind`), and an error that no code classified exits 70. `itos push` reads the kind of a failed fetch or push from git's message (slice-90): a remote that cannot be reached exits 75, a remote that is no repository 3, a push that the remote or a hook refuses 1, never git's own 128. In `ci run` a failing step exits 1 and names the step's own code, so a step's code is never read as itos's.
32. Start each error line with `itos:`. Write it for people: say what happened and what to do next. Do not show a raw command line as the message. (GNU-ERR; CLIG `#errors`.) Some error lines do not follow this rule yet.
33. Let the help and the code agree on each exit code. Now the top-level help says that `itos task` exits 1 for an unknown task, but it exits 2.

### Environment variables

34. Start each environment variable that itos reads with `ITOS_`. Write it in uppercase with underscores. (CLIG `#environment-variables`.) itos follows this rule.
35. Read settings in this order: flag, then environment variable, then the project config. (CLIG `#configuration`.) itos follows this rule: `--config`, then `ITOS_CONFIG`, then `itos.yaml`, then the stealth config.
36. Do not let a run in CI depend on the network for an update check. Give an opt-out for the check. (CLIG `#future-proofing`, "Don't create a time bomb".) itos follows this rule: `CI` and `ITOS_NO_UPDATE` stop the check.

### Prompts

37. Ask a question only when stdin and stdout are terminals. Give a flag for each question, so that a script never needs a terminal. (CLIG `#interactivity`.) itos follows this rule: only `init` asks, and each question has a flag.

### Configuration

38. Keep the settings of a project in a file in the repository, under version control. (CLIG `#configuration`.) itos follows this rule with `itos.yaml`. The stealth config in the git folder is an intentional exception for a clone that cannot change the repository.

### Entry points for other programs

Some commands are not for people. Another program calls them: git calls the hook commands, Claude Code calls the guard, and the shim runs when a program calls `git`. The commands the itos plugin for Claude Code calls are part of the contract with the plugin and are never renamed or removed, even in a major release (decision 41): `guard claude-code`, and `work list --all` and `task list` with `--json` for its titles. CI checks that the built itos answers every one of them.

39. An entry point uses the protocol of the program that calls it. It does not use the itos contract when the two do not agree. For example, Claude Code reads exit 2 as "block", so the guard never exits 2.
40. An entry point for one program does not share a group with entry points for a different program. Name its group for its function and the program that it serves. (CLIG `#subcommands`.) itos follows this rule (slice-89): Claude Code's guard is `itos guard claude-code`, and `itos hook` holds git's two hooks and `hook install`, which declares them in the git config (slice-91).
41. The output of the guard follows the Claude Code schema exactly, with no extra keys. Claude Code can refuse keys it does not know. The guard output is not part of the itos contract, so `previous-release` does not judge it. This tree's corpus and `features/guard.feature` still test it.

### Changing the interface

42. A rename of a command or a flag, or a change to an exit code, is a breaking change (D35). Put the breaking changes that are ready into one major release together. Do not make one major release for each change.
43. Do not keep code to stay compatible with an old interface. Only `previous-release` judges compatibility, against the contract above. The commands the plugin calls are the exception (decision 41): they are never renamed, so no compatibility code is needed for them either.

## Where itos does not follow these rules yet

Each row is a gap that the 2026-10-06 review found and reproduced. The rule number links each gap to its rule.

| Rule | What itos does now                                                | Example                                   |
| ---- | ----------------------------------------------------------------- | ----------------------------------------- |
| 13   | A group with no subcommand says that the group is unknown.        | `itos hook`                               |
| 18   | A flag selects a different action.                                | `config check --print-defaults`           |
| 19   | `--flag=value` is dropped with exit 0.                            | `task list --group=foo` lists every task  |
| 20   | Some commands ignore an unknown flag and exit 0.                  | `version --bogus`, `ci plan A B --bogus`  |
| 21   | A flag's value is read as a global flag.                          | `task list --group --json`                |
| 25   | A bad ref passes.                                                 | `ci plan nosuchref HEAD` exits 0          |
| 27   | JSON with no `--json`.                                            | `config get` of a mapping                 |
| 29   | `--json` prints nothing for a usage error or an unexpected error. | `itos --json verify nosuchref HEAD`       |
| 32   | Error lines with no `itos:` prefix, or a raw command line.        | `itos: Command failed: git rev-list …`    |
| 33   | The help and the code do not agree.                               | `itos task nope` exits 2; the help says 1 |

## Decided for v6.0.0

Decision 36 settles the questions that this review left open. v6.0.0 carries these changes, so the gaps above for rules 3 to 6, 17 to 21, 25, 31 and 40 close with it:

- The Claude Code guard is `itos guard claude-code`. A guard for another harness is `itos guard <harness>`.
- `itos ask` is `itos decision` (decision 38 renamed decision 36's `itos question` before any release had it; `itos ask` and `itos question` exit 2 naming `itos decision` and `itos followup`), and `itos follow` is `itos followup`. `asks.yaml`, `follow-ups.yaml` and the `q-<n>` IDs do not change.
- `hooks install` is `hook install`. `itos hook` holds git's hooks only.
- `work promote --as` is `--id`, and `work queue --drop` is `--remove`.
- `hook install` declares itos's two hooks in the clone's git config (`hook.itos-commit-msg` and `hook.itos-pre-push`) and knows no hook manager: `--manager` and `hooks.manager` are gone (decision 37, slice-91). Every itos command that commits or pushes exits 3 when git would not run its hook: the git config does not declare it (run `itos hook install` once in the clone), or the git is older than 2.54.0, the first release that runs the hooks its config declares (git's `Documentation/RelNotes/2.54.0.adoc`).
- `itos help` with an unknown topic exits 2.
- An old name exits 2 and names the new one. No alias and no feature flag keeps an old interface working.
- Every command parses its flags from a declared spec (rules 19 to 21 and 25).
- Each exit code comes from the kind of the error (rule 31), and an error that itos cannot classify exits 70.
