package review

import "github.com/donvargax/itos/v8/internal/declare"

var Command = declare.Command{
	Name: "review",
	Rank: 105,
	Lines: `  review post <item> --verdict approve|changes --at <commit> [--stage spec|diff] [--body-file <path>]
                                   record a reviewer's verdict on the item's pull request, bound to its head
`,
	Help: map[string]string{
		"": `Usage: itos review post <item> --verdict approve|changes --at <commit> [--stage spec|diff] [--body-file <path>]

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
nothing: review alone or with any word but post, post with no item or two,
without --verdict or without --at, --stage with a word that is neither spec nor
diff, --body-file with no path, or any other flag. Exit 3, rule
review-pull-request-failed, is a gh that is missing or fails, naming the call
and gh's words ("gh pr list failed: HTTP 401: Bad credentials"), nothing posted;
no-github-repository, exit 3, is a remote that names no GitHub repository.

--json: {"schema":1,"ok":true,"outcome","item","stage","verdict","head","round","comment"?},
        outcome posted or already, head the full SHA, comment the posted
        comment's address as GitHub gives it (html_url), left out for already;
        a refusal {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}`,
	},
	Spec: declare.Spec{Subs: map[string]declare.Spec{
		"post": {Flags: []declare.Flag{declare.Value("--verdict"), declare.Value("--at"), declare.Value("--stage"), declare.Value("--body-file")}},
	}},
}
