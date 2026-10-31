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

// The review-state comments Codex wrote on PR 242 and #243 (testdata/codex),
// by the login GraphQL gives, chatgpt-codex-connector: each names its head
// for both reviews, so at that head neither request is missing, whether the
// config writes the login with "[bot]" or not; at another head both are.
func TestCodexReadsItsRecordedReviewStates(t *testing.T) {
	for name, head := range map[string]string{"242-review-state": head242, "243-review-state": head243} {
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
