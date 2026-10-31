package reviewers

import (
	"encoding/json"
	"strings"

	"github.com/donvargax/itos/v8/internal/providers"
)

// Codex's two requests: its code review, then its security review, which
// "@codex review" no longer starts once automatic reviews are off.
const (
	codexReview   = "@codex review"
	codexSecurity = "@codex security review"
)

var codex = adapter{lines: []string{codexReview, codexSecurity}, reviewed: codexReviewed}

// codexReviewed is whether the login's review state names the head for the
// review the line asks for: the code review's commit_sha for "@codex
// review", the security review's for "@codex security review".
func codexReviewed(state providers.PullState, login, line, head string) bool {
	sha, security := codexState(state.Comments, login)
	if line == codexSecurity {
		sha = security
	}
	return sha != "" && strings.EqualFold(sha, head)
}

// codexState is the commit of each review in the login's review state: the
// last "<!-- codex-review-state:v1 {json} -->" line of its comments; "" for a
// review it does not name, or with no state or one that cannot be read.
func codexState(comments []providers.Authored, login string) (code, security string) {
	for _, c := range comments {
		if !sameLogin(c.Login, login) {
			continue
		}
		for _, line := range strings.Split(c.Body, "\n") {
			text, ok := strings.CutPrefix(strings.TrimSpace(line), "<!-- codex-review-state:v1 ")
			text, closed := strings.CutSuffix(text, " -->")
			if !ok || !closed {
				continue
			}
			var parsed struct {
				CodeReview struct {
					SHA string `json:"commit_sha"`
				} `json:"code_review"`
				SecurityReview struct {
					SHA string `json:"commit_sha"`
				} `json:"security_review"`
			}
			code, security = "", ""
			if json.Unmarshal([]byte(text), &parsed) == nil {
				code, security = parsed.CodeReview.SHA, parsed.SecurityReview.SHA
			}
		}
	}
	return code, security
}
