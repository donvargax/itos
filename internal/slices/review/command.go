package review

import "github.com/donvargax/itos/v8/internal/declare"

var Command = declare.Command{
	Name: "review",
	Rank: 105,
	Lines: `  review post <item> --verdict approve|changes --at <commit> [--stage spec|diff] [--body-file <path>]
                                   record a reviewer's verdict on the item's pull request, bound to its head
  review request <item> [--expect <revision>]
                                   ask each reviewer push.pr.reviewers names for the pull request's head, once
  review status <item>             post the itos/reviews commit status on the item's pull request's head
`,
	Help: map[string]string{
		"": `Usage: itos review post <item> --verdict approve|changes --at <commit> [--stage spec|diff] [--body-file <path>]
       itos review request <item> [--expect <revision>]
       itos review status <item>

Records a reviewer's verdict on an item's pull request, under push.mode pr,
from any branch or a detached HEAD: the item names the branch, and its open
pull request is the one the verdict goes on. The verdict is one plain issue
comment, append-only, never a GitHub review (the person cannot approve their
own pull request): the line "itos: <stage> review of <item> at <the head's
first 7 characters>: <verdict>", then, with --body-file, an empty line and the
file's text (\r\n read as \n, its trailing line ends dropped), then an empty
line and the hidden marker programs read,

  <!-- itos:v1 {"record":"verdict","item":"<item>","stage":"<stage>","verdict":"<verdict>","head":"<the full SHA>","round":<n>} -->

--stage is spec by default; diff is the review of the implementation. It says
"Posted the <stage> review of <item> at <7 characters>: <verdict>, round <n>,
on <the pull request's address>.", exit 0.

--at <commit> names the commit judged: a full SHA (40 hex digits) is taken as
it is; anything else (an abbreviated SHA, HEAD, HEAD~1, a branch) is what git
resolves it to in the clone, so a reviewer who fetched the branch may say
HEAD. It must be the pull request's current head: one that is not, because
the branch was pushed again after the review, is refused with nothing posted
(review-head-moved), naming the new head and the command to run once it is
reviewed. So a verdict never names a head nobody can read the marker against.

The round counts the verdict markers of the same item and stage whose
comment's author is a login among the people the config names (work.people,
compared regardless of case; with no people, a stealth config or a people file
missing or unreadable, every login counts): the new marker's round is one more.
A marker by any other login, of another item or stage, a comment whose marker
is not JSON or not a verdict, the review-request marker ("<!-- itos-review-
request:v1 ... -->") and any prose count for nothing. The latest counted marker
is the verdict that stands: when it records this verdict for this head, nothing
is posted and it says "The <stage> review of <item> at <7 characters>:
<verdict> already, round <its round>, on <address>; nothing was posted.", exit
0; any other latest marker, another verdict at this head or any verdict at
another head, makes the next round, so approve, then changes, then approve
again at one head are rounds 1, 2 and 3, and a reader takes the last.

The gh calls, each through the gh on the PATH naming the repository push
names (ci.range.github.repository_env, else the remote's URL): gh pr list for
the open pull request, the gh api graphql question itos push --ready puts for
the head and the comments (every page of them), then gh api
repos/<owner>/<name>/issues/<number>/comments -f body=<the body>, whose answer
gives the comment's address. It fetches, rebases and pushes nothing and posts
no GitHub review and no pr comment; the comment's author is whoever gh is
signed in as.

Refused, exit 1, nothing posted, each a problem naming its rule, message and
fix: review-without-pr-mode, outside push.mode pr (trunk, no config, or one
that cannot be read); work-unknown-item, an item the registry does not have,
as itos work show refuses it; review-bad-verdict, a --verdict that is neither
approve nor changes, exactly those words (a problem, not a usage error, since a
driver computes the verdict from a reviewer's answer and reads the problem
JSON); review-body-file, a --body-file that cannot be read; review-unknown-
commit, an --at git cannot resolve; review-no-pull-request, no open pull
request for the item (itos push opens it); and review-head-moved, above. The
first five are refused before gh is asked. Exit 2 is a usage error, asking gh
nothing: review alone or with any word but post, request or status, post with no item or two,
without --verdict or without --at, --stage with a word that is neither spec nor
diff, --body-file with no path, or any other flag. Exit 3, rule
review-pull-request-failed, is a gh that is missing or fails, naming the call
and gh's words ("gh pr list failed: HTTP 401: Bad credentials"), nothing posted;
no-github-repository, exit 3, is a remote that names no GitHub repository.

--json: {"schema":1,"ok":true,"outcome","item","stage","verdict","head","round","comment"?},
        outcome posted or already, head the full SHA, comment the posted
        comment's address as GitHub gives it (html_url), left out for already;
        a refusal {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}

itos review request <item> [--expect <revision>] asks each reviewer
push.pr.reviewers names ({login, kind}, in the config's order) for the head of
the item's open pull request, each request once, under push.mode pr, from any
branch or a detached HEAD. A request is one gh pr comment <number> --repo
<repository> --body=<the request>, posted with the user's gh (Bugbot ignores a
command from a bot or an app): the kind's request line, an empty line and the
marker

  <!-- itos-review-request:v1 <login> <the head's full SHA> -->

the login as the config writes it. The kinds, each an adapter in itos:

  codex   posts "@codex review" (the code review), then "@codex security
          review" (the security review). Its own review of the head is its
          review-state comment ("<!-- codex-review-state:v1 {...} -->", the
          last the login wrote): the code review counts for the first request
          when its commit_sha is the head, the security review for the second
          when its commit_sha is, each alone, whatever its status.
  bugbot  posts "@cursor review". Its own review of the head is a GitHub review
          by the login whose body starts with "<!-- BUGBOT_REVIEW -->" and
          whose commit is the head.

A request is on the head when a comment by anyone has the request line as its
first line and the marker naming the login (a "[bot]" suffix ignored) and the
head; a mention without the marker, or a marker for another login or head,
counts for nothing. A request on the head, or answered by the reviewer's own
review of it, is not posted again, so run again it posts only what is still
missing. It says "Asked <login> at <7 characters> on <address>: <the request
line>." for each request posted (outcome requested), or "Every reviewer of
<item> has a request or a review for <7 characters> on <address>; nothing was
posted." (already), exit 0.

--expect <revision>, the revision itos next <item> --json gave, is judged once
GitHub is read and before anything is posted: already holds whatever it says;
else one that is not the item's revision now is refused, review-stale, exit 1,
nothing posted, naming the current one. A revision that is not "sha256:" and 64
lowercase hex digits is a usage error.

Refused, exit 1, nothing posted: review-without-pr-mode, work-unknown-item,
review-no-reviewers (push.pr.reviewers names none; push.pr.review_by names
whom itos push --ready waits for, never whom to ask), each before gh is asked;
review-no-pull-request; review-stale. Exit 3, review-pull-request-failed: a gh
that is missing or fails, naming the call and gh's words; the first gh pr
comment that fails stops the command, the requests before it posted and none
after it tried. Exit 2: request with no item or two, a malformed --expect, or
any other flag. It fetches, rebases and pushes nothing, and posts no GitHub
review and no comment through gh api.

--json: {"schema":1,"ok":true,"outcome","item","head","posted"}, outcome
        requested or already, head the full SHA, posted the requests posted in
        order, each {"login","kind","request"}, [] for already

itos review status <item> judges the reviews of the item's open pull
request's head as itos push --ready judges them and posts the result as one
GitHub commit status on that head, its context itos/reviews, so branch
protection can require it and a workflow can run it on every pull request
event. Under push.mode pr, from any branch or a detached HEAD. The head is the
one GitHub gives in the same answer as the reviews, never the clone's.

A login of push.pr.review_by (a trailing "[bot]" dropped from both sides) that
wrote a Codex review-state comment has reviewed the head when that comment
says both its reviews completed, both of the head; any other login has when it
wrote a review that is not Copilot's refusal ("Copilot ... was unable to review
this pull request"); a login whose only reviews are refusals has refused. The
status's three states:

  failure  a login refused, whatever the others' state: "Refused to review:
           <the logins that refused>."
  pending  a login has no verdict for the head: "Waiting on <those logins>."
  success  every login reviewed the head: "Reviewed by <every login>."

the logins as the config writes them, in its order. A description over 140
characters, which GitHub refuses, is cut at a whole login, the rest counted
(" and <n> more."). It says "Posted itos/reviews <state> on <7 characters> of
<the pull request's address>: " and who reviewed, who it waits on, or each
refusal's first line, exit 0 whatever the state. It asks nobody for a review,
waits for nothing and merges nothing: one gh pr list, the gh api graphql
question --ready puts, and gh api repos/<owner>/<name>/statuses/<the head's
full SHA> -f state=<state> -f context=itos/reviews -f description=<description>,
so running it again posts the same state again.

Refused, exit 1, nothing posted: review-without-pr-mode, outside push.mode pr;
review-without-review-by, push.pr.review_by empty or absent; work-unknown-item;
review-no-pull-request. Exit 3: review-pull-request-failed, a gh that is
missing or fails, naming the call and gh's words, and no-github-repository.
Exit 2, asking gh nothing: status with no item or two, or with any flag.

--json: {"schema":1,"ok":true,"outcome","item","state","head","pull_request":{"url","number"},"description","missing","refused"},
        outcome reviewed, review-pending or review-refused, head the full SHA,
        missing every login with no verdict for the head, refused those that
        refused, each [] for none, description the one posted, logins uncut`,
	},
	Spec: declare.Spec{Subs: map[string]declare.Spec{
		"post":    {Flags: []declare.Flag{declare.Value("--verdict"), declare.Value("--at"), declare.Value("--stage"), declare.Value("--body-file")}},
		"request": {Flags: []declare.Flag{declare.Value("--expect")}},
		"status":  {},
	}},
}
