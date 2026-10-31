// Package review is the review command, a slice (ADR-0053): itos review post
// <item> --verdict approve|changes --at <commit> [--stage spec|diff]
// [--body-file <path>] records a reviewer's verdict on an item's pull request
// as an append-only comment bound to the head it judged (slice-206,
// features/review.feature; the v9 coordinator's pilot, docs/plans/
// v9-coordinator.md question 3), so that itos next can derive spec review
// from a fact rather than from prose. Its CLI and Run are the command line,
// its Query and Handler the application.
//
// Command (command.go) declares its name, help and flag spec: post's four
// valued flags, which internal/cli's parser holds every argument to, so an
// unknown flag or one with no value is a usage error before Parse runs.
// Parse (parse.go) refuses the rest, exit 2: review alone or with any word
// but post, no item or two, no --verdict or no --at, a --stage that is
// neither spec nor diff. A --verdict that is neither approve nor changes is
// not a usage error but a problem (review-bad-verdict, exit 1): a driver
// computes the verdict from a reviewer's answer and reads the problem JSON
// to say what came back, while --stage is written by hand in the driver's
// own command line. A body file path is read from the folder the person
// stood in (typed), as git reads one.
//
// Handle (handler.go) refuses, before any gh call and with nothing posted,
// each a problem of the house JSON (out.Problem; Run prints its message
// and fix on stderr, or under --json one object with ok false and problems):
// outside push.mode pr, a missing or unreadable config included
// (review-without-pr-mode); an item the registry does not have, as work
// show refuses it (work-unknown-item, or work-registry-missing with no
// registry); the bad verdict; a body file that cannot be read
// (review-body-file); an --at git cannot resolve (review-unknown-commit). A
// full 40-hex --at is taken as given, anything else is what git resolves it
// to in the clone (Ports.Resolve), so a reviewer who fetched the branch may
// say HEAD. The body file's text has \r\n read as \n and its trailing line
// ends dropped.
//
// Then gh, each call through Ports.Gh naming the repository push names
// (Ports.GitHubRepository, the environment's else the remote's URL, the
// remote the branch's upstream's else origin, Ports.Remote; none is
// no-github-repository, exit 3): gh pr list --repo <repository> --head <item>
// --state open --json number,url finds the item's open pull request (none:
// review-no-pull-request); providers.AskPullState puts --ready's question,
// which gives the head (headRefOid) and the comments, every page of them. A
// head that is not --at's is review-head-moved, naming the new head and the
// command to run once it is reviewed. A gh that is missing or fails, or an
// answer without the shape asked for, is review-pull-request-failed, exit 3,
// naming the call and gh's words.
//
// The round (count): the verdict markers of the same item and stage whose
// comment's author is a login among the people (work.Registry.Counts:
// compared regardless of case as GitHub's logins are; with no people,
// Nobody, every login counts) are counted, in GitHub's order, and the new
// marker's round is one more. A marker (work.ReadVerdict: "<!-- itos:v1 " to "
// -->", JSON with record verdict) by any other login, of another item or
// stage, one that is not JSON or not a verdict, the review-request marker
// and any prose count for nothing. The latest counted marker is the verdict
// that stands: when it records this verdict for this head, nothing is posted
// (already, exit 0, its round); any other latest marker makes the next round.
//
// The post: one plain issue comment, gh api repos/<owner>/<name>/issues/
// <number>/comments -f body=<body>, body the call's last field, holding the
// line "itos: <stage> review of <item> at <7 chars>: <verdict>", the body
// file's text after an empty line when there is one, and after an empty
// line the marker, its JSON's keys record, item, stage, verdict, head (the
// full SHA) and round, in that order (work.Verdict.Marker). gh's answer gives the
// comment's html_url, --json's comment; an answer without it is
// review-pull-request-failed, saying the comment may be posted. No GitHub
// review (the person cannot approve their own pull request), no pr comment,
// nothing fetched, rebased or pushed.
//
// # review status
//
// itos review status <item> (slice-212, features/review-status.feature)
// posts the commit status itos/reviews on the item's pull request's head, so
// that branch protection, not a session's wait, holds the merge on the
// reviews. Parse takes one item and no flag (the declared spec gives status
// none). Status refuses, before gh, outside push.mode pr
// (review-without-pr-mode), with push.pr.review_by empty
// (review-without-review-by, judged before the item, since a success for
// nobody would pass any head), and an item the registry lacks; then it finds
// the open pull request and puts the question as post does (openPull,
// providers.AskPullState), and judges with providers.PullState.Verdicts,
// --ready's own code, against the head that answer gives: never the clone's.
// The state is failure when a login refused, whatever the others' (no review
// comes unless someone asks again), else pending when a login has no verdict,
// else success; the description names the logins of its state, as the config
// writes them, cut at a whole login to GitHub's 140 characters (described).
// One gh api repos/<owner>/<name>/statuses/<head> -f state -f context -f
// description, in that order and no target_url, its answer unread. It asks
// nobody for a review and waits for nothing, so a workflow may run it on
// every event. A gh that fails is review-pull-request-failed, exit 3, as
// post's, each fix naming status to run again.
//
// Handle reaches the world through Ports, which internal/cli binds: the
// config's presence and load, git's resolve, the remote, the repository, gh
// and the body file's read.
package review
