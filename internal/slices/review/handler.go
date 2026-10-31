package review

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
	"github.com/donvargax/itos/v8/internal/work"
)

// The exit codes review gives itself (itos --help): a verdict refused, and
// a missing repository, gh or config.
const (
	policy  = 1
	missing = 3
)

// The verdicts, exactly these words.
const (
	approve = "approve"
	changes = "changes"
)

// The outcomes --json names: the comment posted, and the latest marker
// recording this verdict at this head already, nothing posted.
const (
	posted  = "posted"
	already = "already"
)

// fullSHA is a commit named in full, taken as given.
var fullSHA = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

// Query is what review post asks: the Item whose pull request the Verdict
// goes on, At, the commit judged, the Stage (spec or diff) and the BodyFile
// to put between the line and the marker, "" for none.
type Query struct {
	Item, Verdict, At, Stage, BodyFile string
}

// Ports are what review reaches the world through, which internal/cli
// binds: ConfigFound, whether there is a config file; LoadConfig, the
// config, loaded from the working tree; Resolve, the full SHA of the commit
// a revision names in the clone, an error for none; Remote, the branch's
// upstream's remote, else origin; GitHubRepository, the repository every gh
// call names, "" for none; Gh, gh run with the arguments, its stdout,
// providers.ErrNoGh when it is not installed; and ReadFile, a file's bytes
// from the folder itos runs in.
type Ports interface {
	ConfigFound(ctx context.Context) bool
	LoadConfig(ctx context.Context) (*config.Loaded, error)
	Resolve(ctx context.Context, rev string) (string, error)
	Remote(ctx context.Context) string
	GitHubRepository(ctx context.Context, cfg *config.Loaded, remote string) string
	Gh(ctx context.Context, args ...string) (string, error)
	ReadFile(ctx context.Context, path string) ([]byte, error)
}

// Handler is review's application, over its ports.
type Handler struct {
	ports Ports
}

// NewHandler is the handler over p.
func NewHandler(p Ports) Handler {
	return Handler{ports: p}
}

// Answer is what Handle did: Code, its exit; Problems, what refused it,
// nothing posted; else Outcome, posted or already, the Item, Stage and
// Verdict, the Head judged (the full SHA), the Round the marker records,
// the pull request's Address and, when posted, the Comment's address.
type Answer struct {
	Code     int
	Problems []out.Problem
	Outcome  string
	Item     string
	Stage    string
	Verdict  string
	Head     string
	Round    int
	Address  string
	Comment  string
}

// Handle records q's verdict, or refuses it (doc.go says the order).
func (h Handler) Handle(ctx context.Context, q Query) (Answer, error) {
	refuse := func(code int, rule, message, fix string) Answer {
		return Answer{Code: code, Problems: []out.Problem{{Rule: rule, Message: message, Fix: fix}}}
	}
	cfg, pr := h.prMode(ctx)
	if !pr {
		return refuse(policy, "review-without-pr-mode",
			"itos review: a verdict goes on an item's pull request, and this config does not set push.mode: pr; nothing was posted.",
			"set push.mode: pr in the config, or record the verdict elsewhere"), nil
	}
	registry := cfg.Work.Registry
	if !cfg.Source.Has(ctx, registry) {
		return Answer{Code: policy, Problems: []out.Problem{work.Missing(registry, false)}}, nil
	}
	r, err := work.Load(cfg, registry)
	if err != nil {
		return Answer{}, err
	}
	if _, problem := work.Show(r, q.Item); problem != nil {
		return Answer{Code: policy, Problems: []out.Problem{*problem}}, nil
	}
	if q.Verdict != approve && q.Verdict != changes {
		return refuse(policy, "review-bad-verdict",
			fmt.Sprintf("itos review: %s is no verdict: --verdict takes approve or changes, exactly; nothing was posted.", q.Verdict),
			fmt.Sprintf("itos review post %s --verdict approve|changes --at %s", q.Item, q.At)), nil
	}
	text := ""
	if q.BodyFile != "" {
		body, err := h.ports.ReadFile(ctx, q.BodyFile)
		if err != nil {
			return refuse(policy, "review-body-file",
				fmt.Sprintf("itos review: the body file %s cannot be read: %s; nothing was posted.", q.BodyFile, err),
				"write the review's text to a file and name it with --body-file, or leave --body-file out"), nil
		}
		text = strings.TrimRight(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n")
	}
	// A full SHA is taken as given, in lower case as GitHub writes a head.
	head := strings.ToLower(q.At)
	if !fullSHA.MatchString(head) {
		sha, err := h.ports.Resolve(ctx, q.At)
		if err != nil {
			return refuse(policy, "review-unknown-commit",
				fmt.Sprintf("itos review: git cannot resolve %s in this clone; nothing was posted.", q.At),
				fmt.Sprintf("fetch the branch (git fetch origin %s), or name the commit judged by its full SHA", q.Item)), nil
		}
		head = sha
	}
	remote := h.ports.Remote(ctx)
	repository := h.ports.GitHubRepository(ctx, cfg, remote)
	if repository == "" {
		return refuse(missing, "no-github-repository",
			fmt.Sprintf("itos review: %s names no GitHub repository to find %s's pull request in; nothing was posted.", remote, q.Item),
			"set ci.range.github.repository_env to a variable naming it, or give the remote a GitHub address"), nil
	}
	number, address, a, failed := h.openPull(ctx, subPost, repository, q.Item)
	if failed {
		return a, nil
	}
	state, err := providers.AskPullState(ctx, h.ports.Gh, repository, number)
	switch {
	case errors.Is(err, providers.ErrNoPullState):
		return ghFailed(subPost, "gh api graphql", err.Error()), nil
	case err != nil:
		return ghFailed(subPost, "gh api graphql", providers.GhReason(err)), nil
	case state.Head == "":
		return ghFailed(subPost, "gh api graphql", "its answer names no head for the pull request"), nil
	}
	if state.Head != head {
		return refuse(policy, "review-head-moved",
			fmt.Sprintf("itos: %s's pull request %s is at %s, not %s; nothing was posted.", q.Item, address, state.Head, head),
			fmt.Sprintf("review %s and run itos review post %s --verdict %s --at %s", short(state.Head), q.Item, q.Verdict, state.Head)), nil
	}
	count, latest := countMarkers(state.Comments, q.Item, q.Stage, r)
	a = Answer{Item: q.Item, Stage: q.Stage, Verdict: q.Verdict, Head: head, Address: address}
	if latest != nil && latest.Verdict == q.Verdict && latest.Head == head {
		a.Outcome, a.Round = already, latest.Round
		return a, nil
	}
	a.Round = count + 1
	comment, failedAnswer, failed := h.post(ctx, repository, number, verdictBody(q, head, text, a.Round))
	if failed {
		return failedAnswer, nil
	}
	a.Outcome, a.Comment = posted, comment
	return a, nil
}

// prMode is the config, when it is there, reads and says push.mode pr.
func (h Handler) prMode(ctx context.Context) (*config.Loaded, bool) {
	if !h.ports.ConfigFound(ctx) {
		return nil, false
	}
	cfg, err := h.ports.LoadConfig(ctx)
	if err != nil || cfg.Push.Mode != "pr" {
		return nil, false
	}
	return cfg, true
}

// openPull is the item's open pull request, its number and address, asked
// with gh pr list as push threads asks: failed, with the problem, when gh
// fails (review-pull-request-failed, exit 3) or lists none
// (review-no-pull-request, exit 1), each fix naming review's word to run again.
func (h Handler) openPull(ctx context.Context, word, repository, item string) (number int, address string, a Answer, failed bool) {
	listed, err := h.ports.Gh(ctx, "pr", "list", "--repo", repository, "--head", item, "--state", "open", "--json", "number,url")
	if err != nil {
		return 0, "", ghFailed(word, "gh pr list", providers.GhReason(err)), true
	}
	var open []struct {
		Number int    `json:"number"`
		URL    string `json:"url"`
	}
	if err := json.Unmarshal([]byte(listed), &open); err != nil {
		return 0, "", ghFailed(word, "gh pr list", "its answer is not a list of pull requests: "+err.Error()), true
	}
	if len(open) == 0 {
		return 0, "", Answer{Code: policy, Problems: []out.Problem{{Rule: "review-no-pull-request",
			Message: fmt.Sprintf("itos review: %s has no open pull request; itos push opens it. Nothing was posted.", item),
			Fix:     "itos push from the item's branch opens its pull request; then itos review " + word + " again"}}}, true
	}
	return open[0].Number, open[0].URL, Answer{}, false
}

// ghFailed is the review-pull-request-failed problem (exit 3): the command
// that failed and why, nothing posted, its fix naming review's word to run again.
func ghFailed(word, command, reason string) Answer {
	return Answer{Code: missing, Problems: []out.Problem{{Rule: "review-pull-request-failed",
		Message: fmt.Sprintf("itos review: %s failed: %s; nothing was posted.", command, reason),
		Fix:     "fix that, then itos review " + word + " again"}}}
}

// countMarkers counts the verdict markers of the item and stage in the
// comments whose author is one of the registry's people (every login when
// it has none), in GitHub's order, and gives the latest, nil for none.
func countMarkers(comments []providers.Authored, item, stage string, r work.Registry) (int, *work.Verdict) {
	count := 0
	var latest *work.Verdict
	for _, c := range comments {
		m, ok := work.ReadVerdict(c.Body)
		if !ok || m.Item != item || m.Stage != stage || !r.Counts(c.Login) {
			continue
		}
		count++
		latest = &m
	}
	return count, latest
}

// verdictBody is the comment posted: the readable line, the body file's
// text after an empty line when there is one, and after an empty line the
// marker (verdictMarker).
func verdictBody(q Query, head, text string, round int) string {
	body := fmt.Sprintf("itos: %s review of %s at %s: %s", q.Stage, q.Item, short(head), q.Verdict)
	if text != "" {
		body += "\n\n" + text
	}
	return body + "\n\n" + verdictMarker(q, head, round)
}

// verdictMarker is the marker of q's verdict at the head, its round given
// (work.Verdict.Marker).
func verdictMarker(q Query, head string, round int) string {
	return work.Verdict{Record: "verdict", Item: q.Item, Stage: q.Stage, Verdict: q.Verdict, Head: head, Round: round}.Marker()
}

// post puts the body on the pull request as one plain issue comment (gh api
// repos/<owner>/<name>/issues/<number>/comments -f body=<body>, body the
// last field) and gives the comment's address from gh's answer: failed,
// with review-pull-request-failed, when gh fails or answers no address, in
// which case the comment may be posted.
func (h Handler) post(ctx context.Context, repository string, number int, body string) (string, Answer, bool) {
	path := "repos/" + repository + "/issues/" + strconv.Itoa(number) + "/comments"
	answered, err := h.ports.Gh(ctx, "api", path, "-f", "body="+body)
	if err != nil {
		return "", ghFailed(subPost, "gh api "+path, providers.GhReason(err)), true
	}
	var comment struct {
		URL string `json:"html_url"`
	}
	if err := json.Unmarshal([]byte(answered), &comment); err != nil || comment.URL == "" {
		return "", ghFailed(subPost, "gh api "+path, "its answer names no comment (html_url); the comment may be posted: read the pull request"), true
	}
	return comment.URL, Answer{}, false
}

// short is a commit's first 7 characters, as itos abbreviates one.
func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
