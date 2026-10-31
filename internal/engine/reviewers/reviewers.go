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
			if requested(state.Comments, rv.Login, line, head) || a.reviewed(state, rv.Login, line, head) {
				continue
			}
			missing = append(missing, Request{Login: rv.Login, Kind: rv.Kind, Line: line})
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

// requested is whether a comment, by anyone, has the line as its first line
// and a marker line naming the login and the head.
func requested(comments []providers.Authored, login, line, head string) bool {
	for _, c := range comments {
		lines := strings.Split(strings.ReplaceAll(c.Body, "\r\n", "\n"), "\n")
		if strings.TrimSpace(lines[0]) != line {
			continue
		}
		for _, l := range lines[1:] {
			text, ok := strings.CutPrefix(strings.TrimSpace(l), marker)
			text, closed := strings.CutSuffix(text, " -->")
			who, sha, both := strings.Cut(text, " ")
			if ok && closed && both && sameLogin(who, login) && strings.EqualFold(sha, head) {
				return true
			}
		}
	}
	return false
}

// sameLogin is whether two logins are equal once a trailing "[bot]" is
// dropped from each, GraphQL naming a bot without it and the REST API with
// it, as itos push --ready compares push.pr.review_by's.
func sameLogin(a, b string) bool {
	return strings.TrimSuffix(a, "[bot]") == strings.TrimSuffix(b, "[bot]")
}
