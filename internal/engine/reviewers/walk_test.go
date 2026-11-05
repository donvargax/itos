package reviewers

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/donvargax/itos/v8/internal/providers"
)

// walkAt is the clock of the walk's tests.
var walkAt = time.Unix(1_000_000, 0)

// stateComment is the login's review-state comment, each review a status and the
// commit it is of, last edited those many seconds before walkAt.
func stateComment(login, code, codeSHA, security, securitySHA string, edited int) providers.Authored {
	review := func(status, sha string) string {
		text, _ := json.Marshal(map[string]any{"status": orNull(status), "commit_sha": orNull(sha)})
		return string(text)
	}
	return providers.Authored{Login: login, Updated: walkAt.Add(-time.Duration(edited) * time.Second),
		Body: `<!-- codex-review-state:v1 {"code_review":` + review(code, codeSHA) + `,"security_review":` + review(security, securitySHA) + `} -->`}
}

// orNull is the text, or null for none.
func orNull(text string) any {
	if text == "" {
		return nil
	}
	return text
}

// madeAgo is the comment making the request for head243, those many seconds before
// walkAt, at the address.
func madeAgo(req Request, seconds int, address string) providers.Authored {
	bodies := req.Bodies(head243)
	return providers.Authored{Login: "someone", Body: bodies[len(bodies)-1], URL: address, Created: walkAt.Add(-time.Duration(seconds) * time.Second)}
}

// walked is a walk over the comments, under the stall, recording its posts and
// keeping each of them, and each line between its two phases, in calls; end is the
// first call that ends the walk, "" for none.
func walked(rv *Reviewing, comments []providers.Authored, end string, logins ...string) (*Stop, []string) {
	var calls []string
	stop := rv.Walk(Walk{
		Logins: logins,
		Head:   head243,
		State:  providers.PullState{Comments: comments},
		Now:    func() time.Time { return walkAt },
		Post: func(p Post) bool {
			call := map[Why]string{ForHead: "for head", NotStarted: "not started", Stalled: "stalled"}[p.Why] + " " + p.Request.Login + " " + p.Request.Line
			calls = append(calls, call)
			rv.Record(p.Request, "#posted", walkAt)
			return call != end
		},
		Between: func() bool {
			calls = append(calls, "between")
			return end != "between"
		},
	})
	return stop, calls
}

// With a poster the walk posts what each phase reaches, in order: the first phase
// each request the head lacks, the limits between, then each second request, going
// on past each, and what it posted it does not post again at the next look.
func TestWalkPostsInOrder(t *testing.T) {
	const a, b = "a", "b"
	rv := NewReviewing(60)
	comments := []providers.Authored{
		stateComment(a, "completed", older, "completed", older, 400),
		stateComment(b, "completed", head243, "", "", 400),
		madeAgo(CodexSecurity(b), 120, "#1"),
	}
	stop, calls := walked(rv, comments, "", a, b)
	want := []string{"for head a @codex review", "for head a @codex security review", "between", "not started b @codex security review"}
	if stop != nil || !reflect.DeepEqual(calls, want) {
		t.Errorf("stop %+v, calls %q, want %q", stop, calls, want)
	}
	// The posted request counts: the first phase has nothing more for a, and b's
	// second request, the latest of two by when it was posted, has just been made.
	stop, calls = walked(rv, comments, "", a, b)
	if stop != nil || !reflect.DeepEqual(calls, []string{"between"}) {
		t.Errorf("the next look: stop %+v, calls %q", stop, calls)
	}
}

// A poster or Between saying no ends the walk with no Stop, nothing after it reached,
// though a stop waits there. With no stall the second phase is never reached.
func TestWalkEnds(t *testing.T) {
	stuck := []providers.Authored{stateComment("a", "completed", head243, "", "", 400), madeAgo(CodexSecurity("a"), 300, "#1"), madeAgo(CodexSecurity("a"), 120, "#2")}
	missing := append([]providers.Authored{stateComment("b", "completed", head243, "", "", 400)}, stuck...)
	for name, c := range map[string]struct {
		rv       *Reviewing
		comments []providers.Authored
		end      string
		stopped  bool
		calls    []string
	}{
		"a poster's no in the first phase": {NewReviewing(60), missing, "for head b @codex security review", false, []string{"for head b @codex security review"}},
		"between's no":                     {NewReviewing(60), stuck, "between", false, []string{"between"}},
		"no stall":                         {NewReviewing(0), stuck, "", false, []string{"between"}},
		"a stall":                          {NewReviewing(60), stuck, "", true, []string{"between"}},
	} {
		t.Run(name, func(t *testing.T) {
			stop, calls := walked(c.rv, c.comments, c.end, "a", "b")
			if (stop != nil) != c.stopped || !reflect.DeepEqual(calls, c.calls) {
				t.Errorf("stop %+v, calls %q, want %v, %q", stop, calls, c.stopped, c.calls)
			}
		})
	}
}

// A poster's no at a second request ends the walk there, under way or not started.
func TestWalkEndsAtASecondRequest(t *testing.T) {
	for name, c := range map[string]struct {
		comments []providers.Authored
		end      string
	}{
		"not started": {[]providers.Authored{stateComment("a", "completed", head243, "", "", 400), madeAgo(CodexSecurity("a"), 120, "#1")}, "not started a @codex security review"},
		"under way":   {[]providers.Authored{stateComment("a", "running", head243, "completed", head243, 120), madeAgo(CodexReview("a"), 300, "#1")}, "stalled a @codex review"},
	} {
		t.Run(name, func(t *testing.T) {
			stop, calls := walked(NewReviewing(60), c.comments, c.end, "a", "b")
			if stop != nil || !reflect.DeepEqual(calls, []string{"between", c.end}) {
				t.Errorf("stop %+v, calls %q", stop, calls)
			}
		})
	}
}

// Stuck is the walk with no poster: the first post it reaches ends it, so it holds
// only where a look would stop with nothing posted, read at the time given. Exactly
// review_stall past a request has run, as the clock's boundary is for --ready.
func TestStuck(t *testing.T) {
	state := func(seconds int) providers.PullState {
		return providers.PullState{Comments: []providers.Authored{stateComment("a", "completed", head243, "", "", 400),
			madeAgo(CodexSecurity("a"), 300, "#1"), madeAgo(CodexSecurity("a"), seconds, "#2")}}
	}
	want := &Stop{Login: "a", Review: "security", Head: head243, Request: "#2"}
	if got := Stuck([]string{"a"}, 60, state(60), head243, walkAt); !reflect.DeepEqual(got, want) {
		t.Errorf("exactly review_stall past: %+v, want %+v", got, want)
	}
	if got := Stuck([]string{"a"}, 60, state(59), head243, walkAt); got != nil {
		t.Errorf("inside review_stall: %+v", got)
	}
	if got := Stuck([]string{"a"}, 0, state(60), head243, walkAt); got != nil {
		t.Errorf("no review_stall: %+v", got)
	}
	if got := Stuck([]string{"a", "b"}, 60, providers.PullState{Comments: append(state(60).Comments, stateComment("b", "completed", older, "completed", older, 400))}, head243, walkAt); got != nil {
		t.Errorf("a request to post for b: %+v", got)
	}
	// A Stop's JSON is what next's observed gives: no word of how it stopped.
	text, _ := json.Marshal(Stop{Login: "a", Review: "code", Head: head243, Request: "#1", Stalled: true})
	if string(text) != `{"login":"a","review":"code","head":"`+head243+`","request":"#1"}` {
		t.Errorf("a Stop's JSON is %s", text)
	}
}

// A review under way stops once it counts as asked, past review_stall, each row as
// it reads: its code request names it whichever review runs, and the code review
// comes first where both are under way. A login with no state is passed over.
func TestWalkStopsAReviewUnderWay(t *testing.T) {
	for name, c := range map[string]struct {
		code, security string
		want           *Stop
	}{
		"the code review":     {"running", "completed", &Stop{Login: "b", Review: "code", Head: head243, Request: "#1", Stalled: true}},
		"the security review": {"completed", "running", &Stop{Login: "b", Review: "security", Head: head243, Request: "#1", Stalled: true}},
		"both":                {"running", "running", &Stop{Login: "b", Review: "code", Head: head243, Request: "#1", Stalled: true}},
	} {
		t.Run(name, func(t *testing.T) {
			comments := []providers.Authored{stateComment("b", c.code, head243, c.security, head243, 300),
				madeAgo(CodexReview("b"), 120, "#1"), madeAgo(CodexSecurity("b"), 120, "#2")}
			if stop, calls := walked(NewReviewing(60), comments, "", "nobody", "b"); !reflect.DeepEqual(stop, c.want) || !reflect.DeepEqual(calls, []string{"between"}) {
				t.Errorf("stop %+v, calls %q, want %+v", stop, calls, c.want)
			}
		})
	}
}

// The code review not started stops before a review under way, and names itself.
func TestWalkStopsTheCodeReviewFirst(t *testing.T) {
	comments := []providers.Authored{stateComment("a", "", "", "running", head243, 300),
		madeAgo(CodexReview("a"), 300, "#1"), madeAgo(CodexReview("a"), 120, "#2"), madeAgo(CodexSecurity("a"), 300, "#3")}
	want := &Stop{Login: "a", Review: "code", Head: head243, Request: "#2"}
	if stop, _ := walked(NewReviewing(60), comments, "", "a"); !reflect.DeepEqual(stop, want) {
		t.Errorf("stop %+v, want %+v", stop, want)
	}
}

// What a review under way counts from and whether it counts as asked. first is the
// wait's first look that read it running, edited its comment's last edit; a request
// at or after the edit counts from itself; one older counts as asked only as the
// second, or with the code review not under way.
func TestStallClock(t *testing.T) {
	first, edit, before, after := time.Unix(5_000, 0), time.Unix(2_000, 0), time.Unix(1_000, 0), time.Unix(3_000, 0)
	none := time.Time{}
	for name, c := range map[string]struct {
		edited       time.Time
		made         []requestMade
		codeUnderWay bool
		from         time.Time
		asked        bool
	}{
		"no request, no edit":                      {none, nil, true, first, false},
		"no request, an edit":                      {edit, nil, true, edit, false},
		"a request with no time":                   {edit, []requestMade{{"a", none}}, true, first, true},
		"a request, no edit":                       {none, []requestMade{{"a", before}}, true, before, true},
		"a request after the edit":                 {edit, []requestMade{{"a", after}}, true, after, true},
		"a request at the edit":                    {edit, []requestMade{{"a", edit}}, true, edit, true},
		"an older request, the code under way":     {edit, []requestMade{{"a", before}}, true, edit, false},
		"an older request, the code not under way": {edit, []requestMade{{"a", before}}, false, edit, true},
		"two older requests, the code under way":   {edit, []requestMade{{"a", before}, {"b", before}}, true, edit, true},
	} {
		if from, asked := stallClock(first, c.edited, c.made, c.codeUnderWay); !from.Equal(c.from) || asked != c.asked {
			t.Errorf("%s: %v, %v, want %v, %v", name, from, asked, c.from, c.asked)
		}
	}
}

// Each kept clock forgets a review that no longer reads so: a review running, then
// done, then running again counts from its look, as does one not started again.
func TestWalkForgetsItsClocks(t *testing.T) {
	const a = "a"
	noTime := func(req Request) providers.Authored {
		bodies := req.Bodies(head243)
		return providers.Authored{Login: "someone", Body: bodies[len(bodies)-1]}
	}
	running := stateComment(a, "running", head243, "completed", head243, 0)
	running.Updated = time.Time{}
	notStarted := stateComment(a, "completed", head243, "", "", 0)
	notStarted.Updated = time.Time{}
	done := stateComment(a, "completed", head243, "completed", head243, 0)
	look := func(rv *Reviewing, at time.Time, comments ...providers.Authored) *Stop {
		return rv.Walk(Walk{Logins: []string{a}, Head: head243, State: providers.PullState{Comments: comments}, Now: func() time.Time { return at }})
	}
	later := func(seconds int) time.Time { return walkAt.Add(time.Duration(seconds) * time.Second) }
	for name, c := range map[string]struct {
		state    providers.Authored
		requests []providers.Authored
		want     string
	}{
		"under way":   {running, []providers.Authored{noTime(CodexReview(a))}, "code"},
		"not started": {notStarted, []providers.Authored{noTime(CodexSecurity(a)), noTime(CodexSecurity(a))}, "security"},
	} {
		t.Run(name, func(t *testing.T) {
			rv := NewReviewing(60)
			comments := append([]providers.Authored{c.state}, c.requests...)
			if stop := look(rv, walkAt, comments...); stop != nil {
				t.Fatalf("the first look: %+v", stop)
			}
			if stop := look(rv, later(30), done); stop != nil {
				t.Fatalf("done: %+v", stop)
			}
			if stop := look(rv, later(60), comments...); stop != nil {
				t.Errorf("past review_stall from the first look, read so again: %+v", stop)
			}
			if stop := look(rv, later(120), comments...); stop == nil || stop.Review != c.want {
				t.Errorf("past review_stall from the look that read it so again: %+v", stop)
			}
		})
	}
	// A login whose comment is gone is read afresh when it comes back.
	rv := NewReviewing(60)
	comments := []providers.Authored{running, noTime(CodexReview(a))}
	if stop := look(rv, walkAt, comments...); stop != nil {
		t.Fatalf("the first look: %+v", stop)
	}
	if stop := look(rv, later(30)); stop != nil {
		t.Fatalf("no comment: %+v", stop)
	}
	if stop := look(rv, later(60), comments...); stop != nil {
		t.Errorf("past review_stall from before the comment went: %+v", stop)
	}
}

// Posted is of the login this wait posted a request of, and no other.
func TestReviewingPosted(t *testing.T) {
	rv := NewReviewing(1)
	rv.Record(CodexReview("a"), "#1", walkAt)
	if !rv.Posted("a") || rv.Posted("b") {
		t.Errorf("posted a %v, b %v", rv.Posted("a"), rv.Posted("b"))
	}
}

// A request's review is security for Codex's security request alone.
func TestRequestReview(t *testing.T) {
	if got := CodexSecurity("a").Review(); got != "security" {
		t.Errorf("the security request: %s", got)
	}
	for _, req := range []Request{CodexReview("a"), {Login: "a", Kind: "bugbot", Line: "@cursor review"}} {
		if got := req.Review(); got != "code" {
			t.Errorf("%s: %s", strings.TrimSpace(req.Line), got)
		}
	}
}

// A request posted counts until GitHub lists it, and then once, known by its
// address; one with no address cannot be known, and counts beside those listed.
func TestRequestsCountAPostedRequestOnce(t *testing.T) {
	const head = "0123456789abcdef0123456789abcdef01234567"
	req := CodexSecurity("a")
	listedAt, postedAt := time.Unix(1_000, 0), time.Unix(1_001, 0)
	mark := req.Bodies(head)[1]
	listed := []providers.Authored{{Body: mark, URL: "u1", Created: listedAt}}
	for name, c := range map[string]struct {
		comments []providers.Authored
		posted   []requestMade
		want     []requestMade
	}{
		"not listed yet":    {nil, []requestMade{{"u1", postedAt}}, []requestMade{{"u1", postedAt}}},
		"listed":            {listed, []requestMade{{"u1", postedAt}}, []requestMade{{"u1", listedAt}}},
		"another's listed":  {listed, []requestMade{{"u2", postedAt}}, []requestMade{{"u1", listedAt}, {"u2", postedAt}}},
		"no address":        {[]providers.Authored{{Body: mark}}, []requestMade{{"", postedAt}}, []requestMade{{"", time.Time{}}, {"", postedAt}}},
		"two with one name": {nil, []requestMade{{"u1", listedAt}, {"u1", postedAt}}, []requestMade{{"u1", listedAt}, {"u1", postedAt}}},
	} {
		rv := NewReviewing(1)
		rv.asked[req] = c.posted
		if got := rv.requests(req, c.comments, head); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %+v, want %+v", name, got, c.want)
		}
	}
}

// The latest request is by its time, whatever the order given, and the one given
// last of those with the same time or with none.
func TestLatestRequest(t *testing.T) {
	first, later := time.Unix(1_000, 0), time.Unix(2_000, 0)
	for _, c := range []struct {
		made []requestMade
		want string
	}{
		{nil, ""},
		{[]requestMade{{"a", first}, {"b", later}}, "b"},
		{[]requestMade{{"a", later}, {"b", first}}, "a"},
		{[]requestMade{{"a", later}, {"b", later}}, "b"},
		{[]requestMade{{"a", time.Time{}}, {"b", time.Time{}}}, "b"},
		{[]requestMade{{"a", time.Time{}}, {"b", first}}, "b"},
	} {
		if got := latest(c.made).address; got != c.want {
			t.Errorf("%+v: %q, want %q", c.made, got, c.want)
		}
	}
}
