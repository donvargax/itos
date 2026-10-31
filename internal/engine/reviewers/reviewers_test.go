package reviewers

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/providers"
)

// The heads of the recorded cases, as GitHub gave them.
const (
	head242      = "08d8cad5329599664cd89a3cdd238892f4cc3c53"
	head243      = "64da3fbcb1112982c1ca27e3406b6a0f103ac596"
	older        = "0123456789abcdef0123456789abcdef01234567"
	codexLogin   = "chatgpt-codex-connector"
	bugbotLogin  = "cursor[bot]"
	bugbotAuthor = "cursor"
)

var (
	codexReviewer  = config.Reviewer{Login: codexLogin, Kind: "codex"}
	bugbotReviewer = config.Reviewer{Login: bugbotLogin, Kind: "bugbot"}
)

// recorded is a body a vendor really wrote, testdata/<kind>/<name>.txt.
func recorded(t *testing.T, kind, name string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", kind, name+".txt"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(string(body), "\n")
}

// lines are the request lines of the requests.
func lines(requests []Request) []string {
	got := []string{}
	for _, r := range requests {
		got = append(got, r.Login+" "+r.Line)
	}
	return got
}

// A request is the line, an empty line and the marker naming the login as
// written and the head; each kind posts its lines in its order.
func TestRequestBodyAndLines(t *testing.T) {
	r := Request{Login: bugbotLogin, Kind: "bugbot", Line: "@cursor review"}
	if got, want := r.Body(head243), "@cursor review\n\n<!-- itos-review-request:v1 cursor[bot] "+head243+" -->"; got != want {
		t.Errorf("Body = %q, want %q", got, want)
	}
	for kind, want := range map[string][]string{
		"codex":   {"@codex review", "@codex security review"},
		"bugbot":  {"@cursor review"},
		"copilot": nil,
	} {
		if got := Lines(kind); !reflect.DeepEqual(got, want) {
			t.Errorf("Lines(%s) = %q, want %q", kind, got, want)
		}
	}
}

// Nobody asked: every request of every reviewer is missing, in the
// config's order and each kind's; Unrequested names each login once. With
// no reviewers, none is missing and Unrequested is empty, not nil.
func TestMissingInOrder(t *testing.T) {
	reviewers := []config.Reviewer{codexReviewer, bugbotReviewer}
	got := Missing(reviewers, providers.PullState{}, head243)
	want := []Request{
		{Login: codexLogin, Kind: "codex", Line: "@codex review"},
		{Login: codexLogin, Kind: "codex", Line: "@codex security review"},
		{Login: bugbotLogin, Kind: "bugbot", Line: "@cursor review"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Missing = %+v, want %+v", got, want)
	}
	if got := Unrequested(reviewers, providers.PullState{}, head243); !reflect.DeepEqual(got, []string{codexLogin, bugbotLogin}) {
		t.Errorf("Unrequested = %q", got)
	}
	if got := Unrequested(nil, providers.PullState{}, head243); got == nil || len(got) != 0 {
		t.Errorf("Unrequested with no reviewers = %#v, want []", got)
	}
}

// A request is on the head only through a comment, by anyone, whose first
// line is the request line and with a marker line naming the login (its
// "[bot]" aside) and the head (in any case); a mention without the marker,
// as the person posted on #243, a marker for another login or head, and a
// marker under another line count for nothing.
func TestRequested(t *testing.T) {
	mark := func(login, sha string) string { return "<!-- itos-review-request:v1 " + login + " " + sha + " -->" }
	cases := map[string]struct {
		body string
		want []string
	}{
		"the marked request":         {"@cursor review\n\n" + mark(bugbotLogin, head243), nil},
		"the login without [bot]":    {"@cursor review\n\n" + mark("cursor", head243), nil},
		"the head in capitals":       {"@cursor review\n\n" + mark(bugbotLogin, strings.ToUpper(head243)), nil},
		"CRLF line ends":             {"@cursor review\r\n\r\n" + mark(bugbotLogin, head243), nil},
		"#243's mention, no marker":  {"@cursor review", []string{bugbotLogin + " @cursor review"}},
		"a marker of another head":   {"@cursor review\n\n" + mark(bugbotLogin, older), []string{bugbotLogin + " @cursor review"}},
		"a marker of another login":  {"@cursor review\n\n" + mark(codexLogin, head243), []string{bugbotLogin + " @cursor review"}},
		"another line with a marker": {"@codex review\n\n" + mark(bugbotLogin, head243), []string{bugbotLogin + " @cursor review"}},
		"a marker without its login": {"@cursor review\n\n<!-- itos-review-request:v1 " + head243 + " -->", []string{bugbotLogin + " @cursor review"}},
		"a marker left open":         {"@cursor review\n\n<!-- itos-review-request:v1 cursor " + head243, []string{bugbotLogin + " @cursor review"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			state := providers.PullState{Comments: []providers.Authored{{Login: "donvargax", Body: c.body}}}
			got := lines(Missing([]config.Reviewer{bugbotReviewer}, state, head243))
			if want := c.want; !reflect.DeepEqual(got, append([]string{}, want...)) {
				t.Errorf("Missing = %q, want %q", got, want)
			}
		})
	}
}

// The real mentions the person posted on #243, with no marker, ask nobody as
// itos reads a request: every request is missing.
func TestUnmarkedMentionsOf243(t *testing.T) {
	state := providers.PullState{Comments: []providers.Authored{
		{Login: "donvargax", Body: "@codex review"},
		{Login: "donvargax", Body: "@codex security review"},
		{Login: "donvargax", Body: "@cursor review"},
	}}
	got := lines(Missing([]config.Reviewer{codexReviewer, bugbotReviewer}, state, head243))
	want := []string{codexLogin + " @codex review", codexLogin + " @codex security review", bugbotLogin + " @cursor review"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Missing = %q, want %q", got, want)
	}
}
