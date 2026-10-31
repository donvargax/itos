// Package reviewers is how itos asks the reviewers push.pr.reviewers names
// for a pull request's head, and reads whether each was asked (slice-215,
// features/review-request.feature): what itos review request posts and itos
// next's review-unrequested judges, an engine (ADR-0053) so the two cannot
// disagree.
//
// A reviewer is a login and a kind, and a kind is an adapter here, never a
// format the config describes, since vendors change their formats without
// notice: its request lines, in the order they are posted, and how the
// reviewer's own review of a head reads. Each adapter is tested against the
// bodies its vendor really wrote, recorded in testdata/<kind>/, so a new
// format is a case to record.
//
//   - codex asks "@codex review" (the code review), then "@codex security
//     review" (the security review). Its own review of the head is in the
//     login's review-state comment as itos push --ready reads it, the last
//     "<!-- codex-review-state:v1 {...} -->" line of its comments: the code
//     review counts for the first request when its commit_sha is the head,
//     whatever its status, the security review for the second when its
//     commit_sha is, each alone. Codex's other comments and its GitHub
//     reviews (receipts, the "To use Codex here" reply) count for nothing.
//   - bugbot asks "@cursor review". Its own review of the head is a GitHub
//     review by the login whose body starts with "<!-- BUGBOT_REVIEW -->" and
//     whose commit is the head; any other review, a person's empty one
//     included, counts for nothing.
//
// A request (Request.Body) is a comment of the request line, an empty line
// and itos's request marker, "<!-- itos-review-request:v1 <login> <the
// head's full SHA> -->", the login as the config writes it. A request is on
// the head when a comment by anyone has the line as its first line and a
// marker line naming the login and the head; a line without the marker, or a
// marker of another login or head, counts for nothing. Logins are compared
// with a trailing "[bot]" dropped from each, as GraphQL names a bot without
// it. Missing is each request of each reviewer, in the config's order and the
// kind's, neither on the head nor answered by the reviewer's own review of
// it; Unrequested the logins, as written, with a request missing.
package reviewers
