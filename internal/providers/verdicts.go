package providers

import (
	"encoding/json"
	"strings"
	"time"
)

// Refusal is a login with no verdict whose reviews are all refusals: the
// first line of the last.
type Refusal struct {
	Login, Line string
}

// ReviewState is a login's review-state comment: whether it wrote one, each
// of its two reviews and when the comment was last edited (zero when GitHub
// does not say).
type ReviewState struct {
	Stated         bool
	Code, Security StateReview
	Updated        time.Time
}

// StateReview is one review of a review-state comment: its status and the
// commit it is of.
type StateReview struct {
	Status, SHA string
}

// Completed is whether both reviews are.
func (s ReviewState) Completed() bool {
	return s.Code.Status == "completed" && s.Security.Status == "completed"
}

// ForHead is whether both reviews are of the head.
func (s ReviewState) ForHead(head string) bool {
	return s.Code.SHA == head && s.Security.SHA == head
}

// SameLogin is whether two logins are equal once a trailing "[bot]" is
// dropped from each, GraphQL naming a bot without it and the REST API with
// it.
func SameLogin(a, b string) bool {
	return strings.TrimSuffix(a, "[bot]") == strings.TrimSuffix(b, "[bot]")
}

// Verdicts splits the logins, as the config wrote them, that have no verdict
// for the head (missing) from those among them whose reviews are all
// refusals (refused, in the config's order). A login that wrote a
// review-state comment has the verdict that comment gives and no other: both
// its reviews completed, both of the head (slice 200); else any review of its
// that is not a refusal is one, of whatever commit, since PullStateQuery asks
// for a review's commit nowhere.
func (s PullState) Verdicts(logins []string, head string) (missing []string, refused []Refusal) {
	for _, login := range logins {
		if st := s.StateOf(login); st.Stated {
			if !st.Completed() || !st.ForHead(head) {
				missing = append(missing, login)
			}
			continue
		}
		gave, last := false, ""
		for _, rv := range s.Reviews {
			if !SameLogin(rv.Login, login) {
				continue
			}
			if line, refuses := RefusalLine(rv.Body); refuses {
				last = line
			} else {
				gave = true
			}
		}
		if gave {
			continue
		}
		missing = append(missing, login)
		if last != "" {
			refused = append(refused, Refusal{login, last})
		}
	}
	return missing, refused
}

// RefusalLine is the first line of a review body and whether it is Copilot's
// refusal to review: it begins "Copilot" and says it was unable to review the
// pull request.
func RefusalLine(body string) (string, bool) {
	first, _, _ := strings.Cut(body, "\n")
	first = strings.TrimSpace(first)
	return first, strings.HasPrefix(first, "Copilot") && strings.Contains(first, "was unable to review this pull request")
}

// StateOf reads the login's review-state comment, the last it wrote with a
// "<!-- codex-review-state:v1 {json} -->" line; one that cannot be read has
// reviews that are none and no commit.
func (s PullState) StateOf(login string) (state ReviewState) {
	for _, c := range s.Comments {
		if !SameLogin(c.Login, login) {
			continue
		}
		for _, line := range strings.Split(c.Body, "\n") {
			text, ok := strings.CutPrefix(strings.TrimSpace(line), "<!-- codex-review-state:v1 ")
			text, ok2 := strings.CutSuffix(text, " -->")
			if !ok || !ok2 {
				continue
			}
			var parsed struct {
				CodeReview struct {
					Status string `json:"status"`
					SHA    string `json:"commit_sha"`
				} `json:"code_review"`
				SecurityReview struct {
					Status string `json:"status"`
					SHA    string `json:"commit_sha"`
				} `json:"security_review"`
			}
			state = ReviewState{Stated: true, Updated: c.Updated}
			if json.Unmarshal([]byte(text), &parsed) == nil {
				state.Code = StateReview{parsed.CodeReview.Status, parsed.CodeReview.SHA}
				state.Security = StateReview{parsed.SecurityReview.Status, parsed.SecurityReview.SHA}
			}
		}
	}
	return state
}
