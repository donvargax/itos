// Package prtrust is what CI and work done share of trusting a merged pull
// request's green head (slices 181 and 188): the paths that are bookkeeping,
// the comparison of a task's own checks at two commits, and the fetch of a
// head the clone lacks from refs/pull/<n>/head.
//
// Judge is slice 188's rule for itos ci plan and ci run, in the order
// ci-trust.feature gives it. Under push.mode pr, with ci.range's github
// provider (a repository to ask about) and ci.prose's steps, a push's range
// (a start that is an ancestor of the end) whose last commit is the one a
// merged pull request is listed to have merged as (GitHub's commits/<sha>/pulls)
// is trusted when the range holds the head's commits, rebased, after main's own
// commits that touch the work registry and the ledger alone and name no task
// and no test; each of the head's commits' counterparts differs from it only
// in those files; the tasks the range names have the same checks in the
// ledger at the head and at the end when a ledger file differs; and the head's
// CI run, of the workflow ci.range names, passed. The head is fetched once
// when the clone lacks it. The answer is a plan.Trust, or the line that says
// why not, nothing when no merged pull request counts or GitHub, git or the
// ledger cannot give a reading the rule needs.
package prtrust
