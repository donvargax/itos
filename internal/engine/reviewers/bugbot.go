package reviewers

import (
	"strings"

	"github.com/donvargax/itos/v8/internal/providers"
)

// bugbotMarker opens the body of every review Cursor's Bugbot writes.
const bugbotMarker = "<!-- BUGBOT_REVIEW -->"

var bugbot = adapter{lines: []string{"@cursor review"}, reviewed: bugbotReviewed}

// bugbotReviewed is whether a review by the login, its body starting with
// Bugbot's marker, is of the head.
func bugbotReviewed(state providers.PullState, login, _, head string) bool {
	for _, r := range state.Reviews {
		if sameLogin(r.Login, login) && strings.HasPrefix(r.Body, bugbotMarker) && r.Commit != "" && strings.EqualFold(r.Commit, head) {
			return true
		}
	}
	return false
}
