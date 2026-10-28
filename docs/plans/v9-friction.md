# v9 design input: the v8 close-out's friction

The person, 2026-10-27: fix every friction met before v9 design starts; 2026-10-28: finish the
specced items, then design v9 from what the friction showed. This file is that evidence, grouped
by cause. Each item's commits and pull request hold its detail (`itos work show <id>`).

## What landed (2026-10-27 to 28)

| Item      | What it does                                                                                                          | PR   |
| --------- | --------------------------------------------------------------------------------------------------------------------- | ---- |
| slice-190 | --ready says what it waits on; push.pr.review_stall re-requests a stalled review once                                 | #210 |
| slice-191 | `itos push threads` / `itos push thread <id> --reply --resolve`                                                       | #208 |
| slice-192 | unpushed registry commits anywhere in an item branch's push go to main                                                | #209 |
| slice-193 | `itos push --onto-main`; pre-push accepts a non-main rewrite that drops no commit (same author, author date, message) | #211 |
| slice-195 | work done counts a merged pull request's commits (its merge commit) as the item's                                     | #214 |
| slice-196 | status and itos go fetch the branch they read and say when the clone is behind                                        | #213 |
| slice-197 | ci watch / push read a failed job's failure-level annotations when the log host is refused                            | #212 |
| slice-198 | under GitHub Actions, ci run prints its failures as ::error annotations                                               | #217 |
| slice-199 | status marks an in-progress item whose pull request merged, with its work done                                        | #218 |
| slice-202 | ci.jobs.<name>.paths; ci plan says which jobs a range affects; platform jobs split and skipped when untouched         | #221 |
| T-334     | build, ci and refactor commits may name their item in Item:                                                           | #216 |
| T-335     | scenario command lines quote an apostrophe                                                                            | #220 |
| T-336     | go problem matcher in the CI toolchain action                                                                         | #219 |
| T-337     | the feature tests' recording gh is one handler file per case                                                          | #224 |

In flight at the hand-off: slice-200 (#223: --ready merges a clean pull request itself, pinned to
the judged head; reviews count only for the head; push.pr.ready_wait; clocks from GitHub's own
times) and slice-201 (#222: thread show, threads from any branch).

## The four causes

Almost every friction met was a symptom of one of these. Each is a design choice v8 made, not a bug,
and each is v9's to keep or replace.

1. **The registry lives in git and moves through pull requests.** A take, a close, a queue move is
   a commit; under push.mode pr they must reach main apart from the item's pull request, conflict
   across branches, and `work done` must infer from commits what landed. Built around it:
   slice-192, slice-195, slice-199, T-334, bug-146's stuck close.
2. **The --ready ceremony.** Wait for CI, wait for Codex (which reviews only on ready_for_review and
   stalled on several PRs), turn on auto-merge, meet GitHub's "clean status" refusal (gh merges a
   CLEAN pull request directly and unpinned; the refusal is a race), fall back to a `gh pr merge`
   Claude Code's classifier refuses agents ("Merge Without Review"), outlive a harness's 600 s
   foreground cap, re-run CI after every GitHub rebase. Built around it: slice-190, slice-200,
   slice-201, the Codex re-request rules. Nearly every PR this batch needed the person to merge by
   hand or a second --ready.
3. **Item branches plus "never force".** A branch cut before a sibling merges in the same files
   falls behind and conflicts; bringing it up to main is a rewrite; the pre-push hook needed a
   counterpart rule (slice-193); rebuilding a commit during a conflict changes its author date, so
   the rule counts it dropped (p2-onto-main-rebuilt-commit); and Claude Code's classifier refuses
   `itos push --onto-main` to agents as "Git Destructive" whatever the allow rules say, so the
   person ran every one (#210, #218, #209). Workaround in use: bring a spec-only branch up to main
   before briefing (`gh pr update-branch --rebase`).
4. **Shared test fixtures.** One generated shell script answered every gh call; every slice edited
   the same lines (#210 and #218 conflicted there). T-337 removed it.

## What the gates cost and caught

- Spec review (an Opus reviewer with the local reviewer's prompt, the API credits being out) found
  real defects in every spec but one, often in two rounds: wrong-build-passes scenarios, races,
  GitHub semantics (job-level if before matrix expansion; gh's direct merge of CLEAN; GraphQL
  NOT_FOUND; per-page caps). Worth keeping.
- Codex at ready found about one real defect per pull request (a test that never hit the cache, a
  set-vs-one-to-one match, a stale fetched tip, merges dropped by a cherry-pick) and one false
  positive (a @guard scenario; AGENTS.md now exempts guards). Worth keeping; its stalls and its
  ready-only trigger are the cost.
- Push proof (one mutant, changed-line coverage) caught real gaps on most implementations.
- The per-commit hooks (amend refused, footers, scopes) and the never-force rule are where agents
  lost the most time for the least evidence of value this batch: the questions for v9.

## Open ideas tagged v9 from this batch

p2-onto-main-rebuilt-commit, p3-prepush-precommit-remote, p2-ci-trusted-skips-platform,
p3-review-threads-paging (`itos work list --tag v9`).

## Questions to start v9 from

- Does the registry stay in git, and if so, how do takes and closes avoid being commits that race
  the work?
- Is one pull request per item, with a blocking --ready, the unit itos should drive, or should itos
  hand off to the forge (auto-merge, merge queue) and stop waiting?
- Can item branches avoid rewrites altogether (merge main in instead of rebasing; short-lived
  branches), since agents can never be given a rewrite?
- Which local gates earn their cost, given the evidence above?
