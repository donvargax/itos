package providers

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	quota = "Copilot was unable to review this pull request because the user who requested the review has insufficient remaining quota."
	oops  = "Copilot encountered an error and was unable to review this pull request. You can try again by re-requesting a review."
)

// codexState is the comment Codex edits in place, with a state line holding
// the two statuses, both reviews of the head (@head).
func codexState(code, security string) string {
	return "<!-- codex-pull-request-review-summary -->\n<!-- codex-review-state:v1 " +
		fmt.Sprintf(`{"code_review":{"status":%q,"commit_sha":"@head"},"security_review":{"status":%q,"commit_sha":"@head"}}`, code, security) +
		" -->\n## Codex Review Summary"
}

// A verdict is a review that is no refusal, unless the login wrote a
// review-state comment, whose two statuses both completed are then the only
// verdict; a login whose reviews are all refusals, and that wrote no state, is
// refused with the first line of its last refusal.
func TestVerdicts(t *testing.T) {
	review := func(login, body string) Authored { return Authored{Login: login, Body: body} }
	comment := func(login, body string) []Authored { return []Authored{{Login: login, Body: body}} }
	for name, c := range map[string]struct {
		state   PullState
		missing []string
		refused []Refusal
	}{
		"nothing":                            {PullState{}, []string{"a"}, nil},
		"a review":                           {PullState{Reviews: []Authored{review("a", "")}}, nil, nil},
		"a review by a bot, a login without": {PullState{Reviews: []Authored{review("a[bot]", "x")}}, nil, nil},
		"another's review":                   {PullState{Reviews: []Authored{review("b", "")}}, []string{"a"}, nil},
		"a refusal":                          {PullState{Reviews: []Authored{review("a", quota+"\nmore")}}, []string{"a"}, []Refusal{{"a", quota}}},
		"an error":                           {PullState{Reviews: []Authored{review("a", oops)}}, []string{"a"}, []Refusal{{"a", oops}}},
		"the last refusal":                   {PullState{Reviews: []Authored{review("a", quota), review("a", oops)}}, []string{"a"}, []Refusal{{"a", oops}}},
		"a refusal then a review":            {PullState{Reviews: []Authored{review("a", quota), review("a", "<!-- ccr-overview-v2 -->")}}, nil, nil},
		"a review then a refusal":            {PullState{Reviews: []Authored{review("a", "ok"), review("a", quota)}}, nil, nil},
		"not Copilot's words":                {PullState{Reviews: []Authored{review("a", "It was unable to review this pull request")}}, nil, nil},
		"not on the first line":              {PullState{Reviews: []Authored{review("a", "hi\n"+quota)}}, nil, nil},
		"indented first line":                {PullState{Reviews: []Authored{review("a", "  "+quota)}}, []string{"a"}, []Refusal{{"a", quota}}},
		"a refusal and a bystander":          {PullState{Reviews: []Authored{review("a", quota), review("b", "")}}, []string{"a"}, []Refusal{{"a", quota}}},
		"completed":                          {PullState{Comments: comment("a", codexState("completed", "completed"))}, nil, nil},
		"completed by a bot":                 {PullState{Comments: comment("a[bot]", codexState("completed", "completed"))}, nil, nil},
		"code running":                       {PullState{Comments: comment("a", codexState("running", "completed"))}, []string{"a"}, nil},
		"security running":                   {PullState{Comments: comment("a", codexState("completed", "running"))}, []string{"a"}, nil},
		"by another":                         {PullState{Comments: comment("b", codexState("completed", "completed"))}, []string{"a"}, nil},
		"state beats a review":               {PullState{Reviews: []Authored{review("a", "### Codex Review")}, Comments: comment("a", codexState("completed", "running"))}, []string{"a"}, nil},
		"state beats a refusal":              {PullState{Reviews: []Authored{review("a", quota)}, Comments: comment("a", codexState("completed", "running"))}, []string{"a"}, nil},
		"state, then a refusal":              {PullState{Reviews: []Authored{review("a", quota)}, Comments: comment("a", codexState("completed", "completed"))}, nil, nil},
		"the last state":                     {PullState{Comments: []Authored{{Login: "a", Body: codexState("running", "running")}, {Login: "a", Body: codexState("completed", "completed")}}}, nil, nil},
		"carriage return":                    {PullState{Comments: comment("a", "x\r\n  "+strings.ReplaceAll(codexState("completed", "completed"), "\n", "\r\n")+"  ")}, nil, nil},
		"no security review":                 {PullState{Comments: comment("a", "<!-- codex-review-state:v1 {\"code_review\":{\"status\":\"completed\"}} -->")}, []string{"a"}, nil},
		"not json":                           {PullState{Comments: comment("a", "<!-- codex-review-state:v1 {nope -->")}, []string{"a"}, nil},
		"another comment":                    {PullState{Comments: comment("a", "all completed")}, []string{"a"}, nil},
		"no closing":                         {PullState{Comments: comment("a", "<!-- codex-review-state:v1 {\"code_review\":{\"status\":\"completed\"},\"security_review\":{\"status\":\"completed\"}}")}, []string{"a"}, nil},
		"no opening":                         {PullState{Comments: comment("a", "{\"code_review\":{\"status\":\"completed\"},\"security_review\":{\"status\":\"completed\"}} -->")}, []string{"a"}, nil},
	} {
		t.Run(name, func(t *testing.T) {
			missing, refused := c.state.Verdicts([]string{"a"}, "@head")
			if !reflect.DeepEqual(missing, c.missing) || !reflect.DeepEqual(refused, c.refused) {
				t.Errorf("missing %v, refused %v; want %v, %v", missing, refused, c.missing, c.refused)
			}
		})
	}
	missing, refused := PullState{Reviews: []Authored{review("c", "")}}.Verdicts([]string{"a", "b[bot]", "c"}, "@head")
	if !reflect.DeepEqual(missing, []string{"a", "b[bot]"}) || refused != nil {
		t.Errorf("missing %v, refused %v", missing, refused)
	}
}

// A state's verdict needs both reviews completed and both of the head; the
// state keeps the comment's last edit.
func TestReviewStateOf(t *testing.T) {
	const head, other = "0123456789abcdef0123456789abcdef01234567", "ffffffffffffffffffffffffffffffffffffffff"
	edited := time.Unix(1_234, 0)
	body := strings.ReplaceAll(codexState("completed", "completed"), "@head", head)
	s := PullState{Comments: []Authored{{Login: "a", Body: body, Updated: edited}}}
	st := s.StateOf("a")
	if !st.Stated || !st.Completed() || !st.ForHead(head) || st.ForHead(other) || !st.Updated.Equal(edited) {
		t.Errorf("state %+v", st)
	}
	if st := s.StateOf("b"); st.Stated {
		t.Errorf("a state for another login: %+v", st)
	}
}
