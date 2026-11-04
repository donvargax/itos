package reviewers

import (
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/providers"
)

// marker opens the line of a request that names whom it asks and for which
// head: "<!-- itos-review-request:v1 <login> <sha> -->".
const marker = "<!-- itos-review-request:v1 "

// Request is one request of a reviewer: its Login as the config writes it,
// its Kind, and the Line its kind posts.
type Request struct {
	Login string `json:"login"`
	Kind  string `json:"kind"`
	Line  string `json:"request"`
}

// Body is the comment that asks for the request at the head: the line, an
// empty line, then the marker naming the login and the head.
func (r Request) Body(head string) string {
	return r.Line + "\n\n" + marker + r.Login + " " + head + " -->"
}

// adapter is a kind: the lines it posts, in order, and whether the login's
// own review of the head answers one of them.
type adapter struct {
	lines    []string
	reviewed func(state providers.PullState, login, line, head string) bool
}

// kinds are the adapters by kind; config check refuses any other.
var kinds = map[string]adapter{"codex": codex, "bugbot": bugbot}

// Lines are the request lines of the kind, in the order they are posted;
// none for a kind itos has no adapter for.
func Lines(kind string) []string { return slices.Clone(kinds[kind].lines) }

// Missing is each request of each reviewer, in the config's order and its
// kind's, that is neither on the head nor answered by the reviewer's own
// review of it.
func Missing(reviewers []config.Reviewer, state providers.PullState, head string) []Request {
	var missing []Request
	for _, rv := range reviewers {
		a := kinds[rv.Kind]
		for _, line := range a.lines {
			r := Request{Login: rv.Login, Kind: rv.Kind, Line: line}
			if r.OnHead(state.Comments, head) || a.reviewed(state, rv.Login, line, head) {
				continue
			}
			missing = append(missing, r)
		}
	}
	return missing
}

// Unrequested are the logins, as the config writes them and in its order,
// of the reviewers with a request missing for the head; empty, never nil,
// for none.
func Unrequested(reviewers []config.Reviewer, state providers.PullState, head string) []string {
	logins := []string{}
	for _, r := range Missing(reviewers, state, head) {
		if !slices.Contains(logins, r.Login) {
			logins = append(logins, r.Login)
		}
	}
	return logins
}

// OnHead is whether a comment on the pull request makes the request for the head.
func (r Request) OnHead(comments []providers.Authored, head string) bool {
	return slices.ContainsFunc(comments, func(c providers.Authored) bool { return r.madeBy(c, head) })
}

// Made is each comment that makes the request for the head, in the order given.
func (r Request) Made(comments []providers.Authored, head string) []providers.Authored {
	var made []providers.Authored
	for _, c := range comments {
		if r.madeBy(c, head) {
			made = append(made, c)
		}
	}
	return made
}

// madeBy is whether the comment, by anyone, has the request's line as its first
// line and a marker line naming its login and the head.
func (r Request) madeBy(c providers.Authored, head string) bool {
	lines := strings.Split(strings.ReplaceAll(c.Body, "\r\n", "\n"), "\n")
	if strings.TrimSpace(lines[0]) != r.Line {
		return false
	}
	return slices.ContainsFunc(lines[1:], func(l string) bool {
		text, ok := strings.CutPrefix(strings.TrimSpace(l), marker)
		text, closed := strings.CutSuffix(text, " -->")
		who, sha, both := strings.Cut(text, " ")
		return ok && closed && both && sameLogin(who, r.Login) && strings.EqualFold(sha, head)
	})
}

// sameLogin is whether two logins are equal once a trailing "[bot]" is
// dropped from each, GraphQL naming a bot without it and the REST API with
// it, as itos push --ready compares push.pr.review_by's.
func sameLogin(a, b string) bool {
	return strings.TrimSuffix(a, "[bot]") == strings.TrimSuffix(b, "[bot]")
}
