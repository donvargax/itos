# JSON problem rule IDs

When `--json` reports a problem, `rule` identifies the rule that found it. Scripts may branch on `rule`, but not on `message` or `fix`: those are prose and may change. Renaming a rule ID is a breaking change.

A problem contains `rule`, `message` and, when it suggests an action, `fix`. It does not promise structured subject fields.

## Built-in IDs

The headings name the command or check that emits each rule. One invocation can reach more than one group.

### Config loading and `config check`

- `config-invalid`: the config is not valid YAML.
- `config-missing-section`: a required top-level config section is absent.
- `config-unreadable`: the config file cannot be read.
- `config-version`: the config declares an unsupported version.
- `config-path-set`: a commit scope refers to a path set that does not exist.
- `config-removed`: the config still uses a key removed from the schema.
- `config-type`: a config value has the wrong data type.
- `config-enum`: a config value is not one of the choices allowed for that key.
- `config-unknown-key`: a config object contains an unrecognized key.
- `config-missing-key`: a required config key is absent.
- `config-ledger-files`: `ledger.files` does not contain the `{group}` placeholder.
- `config-scope-except`: a commit-scope exception is invalid.
- `config-scope-type`: a commit scope names an invalid commit type.
- `config-since`: the configured verification start is invalid.
- `config-since-commit`: the configured start commit cannot be found or resolved.
- `config-pin`: release pin information is incomplete or invalid.
- `config-footer-source`: a footer source is missing or conflicts with another source.
- `config-footer-text`: a free-text footer rule is not valid for its source.
- `config-footer-in-place-of`: footer source replacement rules conflict.
- `config-footer-types`: footer requirements name invalid or conflicting commit types.
- `config-step-tasks`: a CI step names a task that cannot be selected there.
- `config-step`: a CI step has an invalid shape or kind.
- `config-step-cost`: a CI step has an invalid cost class or order.
- `config-step-tests`: a CI step has an invalid named-test selection.
- `config-regexp`: a configured regular expression is invalid or unsupported.
- `config-login-from`: `work.people.login_from` is used with a people source that cannot read it, or its template does not contain exactly one `{login}`.
- `config-check-timeout`: `hooks.commit_msg.check_timeout` is not greater than zero.
- `config-work-tag-empty`: a configured work tag is empty.
- `config-work-tag-twice`: `work.tags` declares the same tag more than once.
- `config-watch-interval`: `ci.watch.interval` is negative.
- `config-watch-timeout`: `ci.watch.timeout` is not greater than zero.
- `config-range-check-builtin`: a built-in range check has incompatible commands, adapter, or rename settings.
- `config-proof-paths`: `proof.code.paths` is empty, so no commit can select the code proof.
- `config-proof-glob`: a `proof.code.paths` entry is not a glob itos can parse.
- `config-proof-base`: `proof.code.check` does not use `{base}` to limit the check to the commit range.

### Ledger validation by `task`, `work`, and related commands

- `ledger-folder-missing`: the configured ledger directory is absent.
- `ledger-add-type`: the requested task type is not an allowed commit type.
- `ledger-duplicate-id`: two ledger entries use the same task ID.
- `ledger-unknown-key`: a task entry contains an unrecognized key.
- `ledger-id-pattern`: a task ID does not match the configured pattern.
- `ledger-type`: a task entry has an invalid commit type.
- `ledger-no-id`: a task entry has no ID.
- `ledger-no-title`: a task entry has no title.
- `ledger-why`: a task's `why` is not valid prose.
- `ledger-done-when`: the task's checks field is not a valid list.
- `ledger-check-run-or-fails`: a check must specify exactly one of `run` or `fails`.
- `ledger-check-command`: a check's command is empty or invalid.
- `ledger-check-unknown-key`: a check contains an unrecognized key.
- `ledger-check-value`: a check option has an invalid value.
- `ledger-check-shape`: a check entry is not an object of the expected shape.
- `ledger-pattern-static-after-late`: a static check appears after a late check.

### Commit message checks and hooks

The built-in header linter emits these individual rules. Their names come from the header-lint rule table.

- `subject-empty`: the commit subject is empty.
- `type-empty`: the commit type is empty.
- `body-max-line-length`: a body line exceeds the configured maximum.
- `body-leading-blank`: the body starts with a blank line.
- `footer-leading-blank`: the footer is not separated from the preceding text by a blank line.
- `footer-max-line-length`: a footer line exceeds the configured maximum.
- `header-max-length`: the commit header exceeds the configured maximum.
- `header-trim`: the header has leading or trailing whitespace.
- `subject-case`: the subject begins with a disallowed case.
- `subject-full-stop`: the subject ends with a full stop.
- `type-case`: the commit type uses disallowed capitalization.
- `type-enum`: the commit type is not in the configured list.
- `named-type`: a `!`-marked header does not begin with a configured commit type.
- `header-lint`: a delegated linter reports a problem without a usable rule ID.
- `footer-source-missing`: a footer names a configured source that cannot be read.

### Commit path and named-test range checks

- `scope-only`: a commit changes a path outside the paths allowed for its type.
- `scope-never`: a commit changes a path forbidden for its type.
- `scope-must-touch`: a commit does not change a path required for its type.
- `moves`: a commit changes a named-test set where the built-in moves rule requires unchanged tests.
- `moves-not-gherkin`: the built-in moves rule is used with an adapter that is not Gherkin-compatible.

### Smoke-set validation and smoke checks

- `smoke-unreadable`: the smoke-set file cannot be read.
- `smoke-not-a-file`: a smoke entry names a file that does not exist.
- `smoke-not-live`: a smoke entry names a test that is not live.
- `smoke-no-why`: a smoke entry has no reason.
- `smoke-more-no-why`: a `more` smoke entry has no reason.
- `smoke-missing`: a file containing a live test has no smoke entry.

### `task` and `work` operations

- `work-registry-missing`: the configured work registry is absent.
- `work-people-missing`: the configured people source is absent.
- `work-people-unreadable`: the configured people source cannot be read.
- `work-unknown-item`: the requested work item is not in the registry.
- `work-no-person`: itos cannot determine a usable session identity.
- `work-done-wip`: work cannot close while one of its slice scenarios is still marked work in progress.
- `work-done-unpushed`: work cannot close while a commit naming it is unpushed.
- `work-done-task-check`: work cannot close because a required task check failed.
- `work-done-ci`: work cannot close because CI has not passed.
- `work-commit-failed`: itos could not commit a work-registry change.
- `work-note-spec`: a requested work-item edit conflicts with that item's specification.
- `work-resume-not-deferred`: the requested item is not deferred.
- `work-cycle`: work-item dependencies contain a cycle.
- `work-duplicate-id`: the registry repeats a work-item ID.
- `work-unknown-phase`: an item names a phase that is not in the ledger.
- `work-unknown-phase-owner`: a phase owner is not a known contributor.
- `work-unknown-owner`: an item owner is not a known contributor.
- `work-unknown-dependency`: an item depends on an ID absent from the registry.
- `work-unknown-tag`: an item uses a tag not declared in config.
- `work-done-before-dependency`: an item is marked done before one of its dependencies.
- `work-dropped-dependency`: an active item depends on an item that was dropped.
- `work-unknown-kind`: an item has a kind outside the configured choices.
- `work-unknown-status`: an item has a status outside the configured choices.
- `work-idea-started`: an idea was started before it was specified as a task or slice.
- `work-deferred-no-reason`: an item's `deferred` field has no reason.
- `work-deferred-started`: a deferred item has already started.
- `work-why-not-text`: an item's `why` field is not non-empty text.
- `work-no-title`: an item has no title.
- `work-take-idea`: an idea cannot be taken as implementation work.
- `work-queue-done`: a queue operation names an item that is already complete.
- `work-queue-unknown-item`: a queue operation names an item not in the registry.
- `work-queue-twice`: a queue operation names the same item more than once.

### `decision` commands

- `ask-no-question`: no question was supplied.
- `ask-unknown-item`: the decision refers to a work item that does not exist.
- `ask-answered`: the decision already has an answer.
- `ask-unanswered`: the requested answer has not been supplied.
- `ask-recorded`: the decision has already been recorded.
- `ask-no-decision`: the referenced decision does not exist.
- `decisions-number-twice`: two decision records use the same number.
- `decisions-superseded-by-missing`: a decision names a superseding decision that does not exist.

### `followup` commands

- `follow-no-thread`: the requested thread does not exist.
- `follow-id-taken`: the requested thread ID is already in use.
- `follow-closed`: the thread is already closed.
- `follow-doc-exists`: the output document already exists.

### `draft` commands

- `draft-id-taken`: the requested draft ID is already in use.
- `draft-no-head`: there is no commit at `HEAD` to draft against.
- `draft-paths`: the selected paths cannot be drafted.
- `draft-no-change`: the selected paths have no changes against `HEAD` to save.
- `draft-none`: there is no draft to apply.
- `draft-tree-changed`: the worktree no longer matches the draft's saved tree.
- `draft-checkout-busy`: Git cannot switch to the draft's checkout.
- `draft-not-applied`: applying the draft failed.

### `hook`, `init`, `pin`, and `verify`

- `data-unreadable`: hook data cannot be read.
- `hook-missing`: a required Git hook is not installed or cannot run.
- `pin-behind`: the repository's pinned itos release is older than the required release.
- `config-unreadable`: a command cannot read its config file.
- `merge-type`: a merge commit contains an invalid commit type.

### `ci run`

- `ci-code-proof`: the configured code-proof provider reports failure or cannot provide a usable result. Provider-supplied problem IDs are passed through unchanged and are not built-in itos IDs.

## Configured and delegated IDs

- **Footer checks:** itos lowercases each configured footer key and appends `-footer`. A missing `Task:` footer is `task-footer`; projects can configure other keys, so the set is open.
- **Named-test range checks:** a configured range-check name is used as the rule ID when that check reports a failure. `config-range-check-builtin` is the separate config-validation rule for an unsupported built-in check setup; `moves` is the built-in moves check.
- **Lint delegates:** itos preserves rule IDs reported by the configured header-lint delegate. That tool controls the IDs and may change them independently. If the report has no usable ID, itos emits `header-lint`.
- **Code-proof providers:** `ci-code-proof` is the itos-side rule. Provider problems, such as `mutation.survived`, retain the provider's IDs; they are not a fixed list of itos rules.

The conformance corpus contains examples, not the full catalogue. It includes fixture-only delegate and provider IDs, while some built-in IDs are not represented. Review the implementation's problem constructors and the corpus together when changing this contract.
