package reviewers

import (
	"reflect"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/providers"
)

// codexMissing is what Missing says of Codex alone, configured as login,
// over the comments, at the head.
func codexMissing(login string, comments []providers.Authored, head string) []string {
	return lines(Missing([]config.Reviewer{{Login: login, Kind: "codex"}}, providers.PullState{Comments: comments}, head))
}

// The review-state comments Codex wrote on PR 242, #243 and #263 (testdata/codex),
// by the login GraphQL gives, chatgpt-codex-connector: each names its head
// for both reviews, so at that head neither request is missing, whether the
// config writes the login with "[bot]" or not; at another head both are.
func TestCodexReadsItsRecordedReviewStates(t *testing.T) {
	for name, head := range map[string]string{"242-review-state": head242, "243-review-state": head243, "263-review-state": head263} {
		state := []providers.Authored{{Login: codexLogin, Body: recorded(t, "codex", name)}}
		for _, login := range []string{codexLogin, codexLogin + "[bot]"} {
			if got := codexMissing(login, state, head); len(got) != 0 {
				t.Errorf("%s as %s at its head: missing %q", name, login, got)
			}
		}
		want := []string{codexLogin + " @codex review", codexLogin + " @codex security review"}
		if got := codexMissing(codexLogin, state, older); !reflect.DeepEqual(got, want) {
			t.Errorf("%s at another head: missing %q, want %q", name, got, want)
		}
	}
}

// Each review counts through its own commit_sha, whatever its status: in
// manual mode the code review is at the head and the security review at an
// older commit (242's state with its security review's commit moved), so only
// the security request is missing; and the other way round. A review still
// running counts as asked.
func TestCodexJudgesEachReviewAlone(t *testing.T) {
	body := recorded(t, "codex", "242-review-state")
	security := `"security_review":{"commit_sha":"` + head242 + `"`
	code := `"code_review":{"commit_sha":"` + head242 + `"`
	if !strings.Contains(body, security) || !strings.Contains(body, code) {
		t.Fatalf("242's state no longer reads as recorded:\n%s", body)
	}
	manual := strings.Replace(body, security, `"security_review":{"commit_sha":"`+older+`"`, 1)
	if got := codexMissing(codexLogin, []providers.Authored{{Login: codexLogin, Body: manual}}, head242); !reflect.DeepEqual(got, []string{codexLogin + " @codex security review"}) {
		t.Errorf("the code review at the head, the security review older: missing %q", got)
	}
	flipped := strings.Replace(body, code, `"code_review":{"commit_sha":"`+older+`"`, 1)
	if got := codexMissing(codexLogin, []providers.Authored{{Login: codexLogin, Body: flipped}}, head242); !reflect.DeepEqual(got, []string{codexLogin + " @codex review"}) {
		t.Errorf("the security review at the head, the code review older: missing %q", got)
	}
	running := strings.ReplaceAll(body, `"status":"completed"`, `"status":"running"`)
	if got := codexMissing(codexLogin, []providers.Authored{{Login: codexLogin, Body: running}}, head242); len(got) != 0 {
		t.Errorf("both reviews running at the head: missing %q", got)
	}
}

// Only the login's last review state counts: a later state of an older
// commit outweighs an earlier one of the head, and one that cannot be read
// names no commit. A state written by another login counts for nothing.
func TestCodexReadsTheLastStateOfTheLogin(t *testing.T) {
	atHead := recorded(t, "codex", "243-review-state")
	elsewhere := strings.ReplaceAll(atHead, head243, older)
	both := []string{codexLogin + " @codex review", codexLogin + " @codex security review"}
	for name, comments := range map[string][]providers.Authored{
		"a later state of an older commit": {{Login: codexLogin, Body: atHead}, {Login: codexLogin, Body: elsewhere}},
		"a later state that is not JSON":   {{Login: codexLogin, Body: atHead}, {Login: codexLogin, Body: "<!-- codex-review-state:v1 {nope -->"}},
		"the state quoted by a person":     {{Login: "donvargax", Body: atHead}},
	} {
		if got := codexMissing(codexLogin, comments, head243); !reflect.DeepEqual(got, both) {
			t.Errorf("%s: missing %q, want %q", name, got, both)
		}
	}
	if got := codexMissing(codexLogin, []providers.Authored{{Login: codexLogin, Body: elsewhere}, {Login: codexLogin, Body: atHead}}, head243); len(got) != 0 {
		t.Errorf("a later state of the head: missing %q", got)
	}
}

// Codex's other comments and its GitHub reviews are no review state, even
// those naming the head: its receipts on #243, its "To use Codex here"
// reply, and its review with suggestions on PR 242 count for nothing. A state
// naming no commit counts for no head, not even an empty one.
func TestCodexCountsNothingButItsState(t *testing.T) {
	both := []string{codexLogin + " @codex review", codexLogin + " @codex security review"}
	var comments []providers.Authored
	for _, name := range []string{"243-code-review-receipt", "243-security-review-receipt", "243-environment-reply"} {
		comments = append(comments, providers.Authored{Login: codexLogin, Body: recorded(t, "codex", name)})
	}
	state := providers.PullState{Comments: comments, Reviews: []providers.Authored{
		{Login: codexLogin, Body: recorded(t, "codex", "242-f0be6ec-review-with-suggestions"), Commit: head243},
	}}
	if got := lines(Missing([]config.Reviewer{codexReviewer}, state, head243)); !reflect.DeepEqual(got, both) {
		t.Errorf("Codex's receipts, reply and review: missing %q, want %q", got, both)
	}
	empty := []providers.Authored{{Login: codexLogin, Body: `<!-- codex-review-state:v1 {"code_review":{},"security_review":{}} -->`}}
	if got := codexMissing(codexLogin, empty, ""); !reflect.DeepEqual(got, both) {
		t.Errorf("a state naming no commit, at no head: missing %q, want %q", got, both)
	}
}

// Codex's security request (bug-163): a comment by anyone that is the marker alone,
// naming the login and the head, whatever else it has around it being space. So the
// forms Codex ran as a code review, the line's comment carrying the marker, are no
// request, nor is the line with no marker, and nor is a marker under any other line.
// The comments of #263 and #260 are as recorded; the marker is no other request's.
func TestCodexSecurityRequestIsTheMarkerAlone(t *testing.T) {
	mark := func(login, sha string) string { return marker + login + " " + sha + " -->" }
	here := mark(codexLogin, head263)
	alone := recorded(t, "codex", "263-request-marker")
	if strings.TrimSpace(alone) != here {
		t.Fatalf("#263's marker is %q, want %q", alone, here)
	}
	cases := map[string]struct {
		body string
		want bool
	}{
		"#263's marker, with its line end": {alone, true},
		"with no line end":                 {here, true},
		"CRLF line ends":                   {here + "\r\n", true},
		"empty lines around it, CRLF":      {"\r\n\r\n" + here + "\r\n\r\n", true},
		"indented, trailing spaces":        {"  " + here + "  ", true},
		"spaces on the empty lines":        {"  \n" + here + "\n \t\n", true},
		"the login with [bot]":             {mark(codexLogin+"[bot]", head263), true},
		"the head in capitals":             {mark(codexLogin, strings.ToUpper(head263)), true},
		"#263's line alone":                {recorded(t, "codex", "263-security-request"), false},
		"#260's old form, line then blank": {strings.ReplaceAll(recorded(t, "codex", "260-marked-security-request"), head260, head263), false},
		"the marker, blank, the line":      {here + "\n\n@codex security review", false},
		"the line, the marker next":        {"@codex security review\n" + here, false},
		"the code review's request":        {"@codex review\n\n" + here, false},
		"under a line that is no request":  {"see below\n\n" + here, false},
		"above a line that is no request":  {here + "\n\nsee above", false},
		"text before it on its line":       {"see " + here, false},
		"text after it on its line":        {here + " see", false},
		"two markers":                      {here + "\n" + here, false},
		"another login":                    {mark(bugbotLogin, head263), false},
		"another head":                     {mark(codexLogin, older), false},
		"a short head":                     {mark(codexLogin, head263[:7]), false},
		"left open":                        {strings.TrimSuffix(here, " -->"), false},
		"another version":                  {strings.Replace(here, "v1", "v2", 1), false},
		"an empty comment":                 {"", false},
	}
	security := CodexSecurity(codexLogin)
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			comments := []providers.Authored{{Login: "donvargax", Body: c.body}}
			if got := security.OnHead(comments, head263); got != c.want {
				t.Errorf("OnHead = %t, want %t", got, c.want)
			}
			if got := len(security.Made(comments, head263)) == 1; got != c.want {
				t.Errorf("Made gives it: %t, want %t", got, c.want)
			}
		})
	}
	// A marker alone is no request of any other: the code review's, or Bugbot's.
	for _, other := range []Request{CodexReview(codexLogin), {Login: codexLogin, Kind: "bugbot", Line: "@cursor review"}} {
		if other.OnHead([]providers.Authored{{Body: alone}}, head263) {
			t.Errorf("a marker alone reads as %q", other.Line)
		}
	}
	// What the request posts reads as made, once, by its marker's comment.
	var posted []providers.Authored
	for i, body := range security.Bodies(head263) {
		posted = append(posted, providers.Authored{Body: body, URL: []string{"line", "marker"}[i]})
	}
	if made := security.Made(posted, head263); len(made) != 1 || made[0].URL != "marker" {
		t.Errorf("Made of what it posts = %+v", made)
	}
	// So Missing reads only it: with the old form Codex's security request is asked.
	old := []providers.Authored{{Body: "@codex review\n\n" + here}, {Body: recorded(t, "codex", "260-marked-security-request")}}
	if got, want := codexMissing(codexLogin, old, head260), []string{codexLogin + " @codex review", codexLogin + " @codex security review"}; !reflect.DeepEqual(got, want) {
		t.Errorf("#260's old form at its head: missing %q, want %q", got, want)
	}
	if got := codexMissing(codexLogin, append(posted, providers.Authored{Body: "@codex review\n\n" + here}), head263); len(got) != 0 {
		t.Errorf("both requests at the head: missing %q", got)
	}
}
