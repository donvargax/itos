// Package hook is the hook command, a slice (ADR-0053): `hook commit-msg
// <file>` and `hook pre-push <remote> <url>`, the two entry points git runs
// (hooks.ts, commit-data.ts, commit-scope.ts, commit.ts, commit-tasks.ts and
// pre-push.ts), and `hook install`, which declares them in the clone's git
// config, or on an older git writes them as hook files
// (internal/engine/hooks, whose doc says how). Its CLI and Run are
// the command line, its Query and Handler the application. Claude Code's
// entry point is `guard claude-code`, in a group of its own since v6.0.0.
//
// Command (command.go) declares its name, help and flag spec. Parse reads
// commit-msg's file from where the person stood.
//
// hook commit-msg (commitmsg.go) is four rules, the first to fail deciding,
// each a call to the judgement its own command makes, never a second copy
// of it, the last three on the message git will store as the header lint
// reads it (message.Cleaned, bug 36), so a blank or comment first line, as
// an editor leaves one, does not hide the type from the paths and the
// footers:
//
//  1. itos's data at commit, read from the index (the Trees port's
//     At("index")), since the other rules read the config and the working
//     tree may hold what the commit does not: configcheck.Findings when a
//     staged path is the config, a ledger file, the registry, the
//     questions' file (slice 130), a smoke set or a decision record or
//     index, as the staged config names them (stagesData); data that cannot
//     be read is one data-unreadable problem, the rejection's first line
//     the staged commits.reject_message. Else, for each kind one of whose
//     test files is staged (tests.StagesTestFile) and whose smoke set the
//     index holds, the smoke rule, through configcheck.JudgedSmokeFindings
//     (stagedSmokeIssues, slice 117): it reads the test files as well as the
//     set, so a feat that makes a file's tests live stages no data.
//  2. The staged paths through scope.Of(cfg).Issues and each kind's staged
//     range command, run through the Shell port, each `  - ` line it prints
//     a problem (both only for a type Ruled), then the moves rule
//     (tests.Moves.Between). judgedBase is HEAD's parent when amending takes
//     the commit for an amend, else HEAD, and stagedFiles(base) reads the
//     paths against it: an amend is judged as the commit it makes, so one
//     that only rewords a feat still has the feat's paths. itos commit says
//     whether it amends in ITOS_AMEND; without it, amending guesses from
//     the author git exports, which an amend keeps from HEAD to the second.
//     A merge being made is judged by its own paths (the VCS's
//     StagedOwnPaths), and one with none returns before the rules.
//  3. The header lint, message.LintFile (the built-in lint, or the
//     commits.header_lint.hook delegate with {file} as one tests.ShellWord,
//     inheriting the hook's streams, its exit code the hook's), beside the
//     footer rules at the staged tree (or ITOS_AT). Under a stealth config
//     the footers judged are handedFooters: ITOS_FOOTERS, else an amend's
//     HEAD note.
//  4. The named tasks' checks up to each task's first late one, the ledger
//     and the registry read from one captured index tree by the staged config, the
//     checks' costs and timeouts by the working tree's, each run by
//     judge.FirstFailure in a disposable checkout of that same tree. The
//     checkout has private Git metadata and captured output beside it; all
//     Git-local environment names and ITOS_FOOTERS/ITOS_AMEND are scrubbed.
//     Only runnable checks create a snapshot, removed on either verdict.
//     A check that opts in to evidence records (slice 171, ADR-0060) is
//     looked up first, by the captured tree and the key itos task binds
//     (internal/engine/evidence, Hook set): a hit runs nothing and prints one
//     stderr line saying "cached evidence", and a miss runs in a checkout of
//     its own, under the cap, and is recorded unless the cap stopped it.
//     checkouts makes a snapshot only when some check must run, before any
//     does, so a snapshot that cannot be made refuses the commit (exit 3, never
//     a working-folder run) and an all-hit commit makes none; with no plain
//     check the snapshot is the engine's for its first miss.
//     A failure rejects the commit when
//     work.ItemStatuses says the task is done, and is only printed
//     otherwise. hooks.commit_msg.task_checks: false turns them off.
//
// hook pre-push (prepush.go) reads git's ref lines, or under pre-commit or
// prek the line their environment gives, as pushedRefs: each ref's local
// commit and the remote's commit it replaces when this clone has it, a
// deleted ref left out. First it refuses a push that rewrites the remote
// (slice 128, rewrites): a ref whose remote commit this clone lacks, or
// that is not an ancestor of the commit pushed, as only a forced push
// replaces; refuseRewrite names the ref and itos push, exit 1, before the
// config is read and with no command run. It verifies next (verifyPush):
// judge.Over on each distinct pushedRange, <remote sha>..<local sha>, or
// the unpushed commits up to the local one when there is no remote commit
// to compare with, each run's report written to a buffer, so a push whose
// commits pass prints nothing. A failing range's report is printed with
// fixAdvice (git commit --amend, or git rebase -i from the parent of the
// first failing commit, --root for a root commit) and the hook exits 1 with
// no command run. Then it runs hooks.pre_push's per_base once per remote
// base this clone has (pushBases), else whole, through the Shell port with
// the hook's streams, nothing for a deleted branch, and nothing at all
// without hooks.pre_push.
//
// Handle reaches the world through Ports, which the slice owns and
// internal/cli binds: the repository (the VCS port, git behind it), its
// trees, its git config and hooks folder (hooks.Ports), the config loaded
// from the working tree, the working tree's files, config check's task
// flags and the config's shell. A rejection is in Handle's Answer, which Run prints;
// what the rules run writes as it runs.
package hook
