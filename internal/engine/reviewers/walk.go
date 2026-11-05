package reviewers

import (
	"slices"
	"time"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/providers"
)

// Reviewing is what a wait keeps of the logins' reviews between its looks:
// stall, push.pr.review_stall; since, the first look that read each login's
// review running for the head; unstarted, the first that read each review not
// started; and asked, each request this wait posted for the head, since GitHub
// may not list one yet.
type Reviewing struct {
	stall     float64
	since     map[string]time.Time
	unstarted map[Request]time.Time
	asked     map[Request][]requestMade
}

// NewReviewing is a wait's first look at the reviews.
func NewReviewing(stall float64) *Reviewing {
	return &Reviewing{stall: stall, since: map[string]time.Time{}, unstarted: map[Request]time.Time{}, asked: map[Request][]requestMade{}}
}

// Record keeps a request this wait posted: the address gh printed and when.
func (rv *Reviewing) Record(req Request, address string, at time.Time) {
	rv.asked[req] = append(rv.asked[req], requestMade{address, at})
}

// Posted is whether this wait posted a request of the login.
func (rv *Reviewing) Posted(login string) bool {
	for req := range rv.asked {
		if req.Login == login {
			return true
		}
	}
	return false
}

// requestMade is one comment making a request: its address and when it was made,
// each empty where GitHub or gh does not say.
type requestMade struct {
	address string
	at      time.Time
}

// requests are the comments making the request for the head: those GitHub lists,
// then those this wait posted that it does not list yet. A request posted and
// listed counts once, known by its address.
func (rv *Reviewing) requests(req Request, comments []providers.Authored, head string) []requestMade {
	var made []requestMade
	for _, c := range req.Made(comments, head) {
		made = append(made, requestMade{c.URL, c.Created})
	}
	listed := len(made)
	for _, posted := range rv.asked[req] {
		if posted.address == "" || !slices.ContainsFunc(made[:listed], func(m requestMade) bool { return m.address == posted.address }) {
			made = append(made, posted)
		}
	}
	return made
}

// latest is the request made last by its time, the one given last of those with
// the same time or with none, GitHub's times being whole seconds.
func latest(made []requestMade) requestMade {
	var last requestMade
	for _, m := range made {
		if !m.at.Before(last.at) {
			last = m
		}
	}
	return last
}

// Why is what a Post answers.
type Why int

const (
	// ForHead is a review whose request for the head no comment makes.
	ForHead Why = iota
	// NotStarted is a review with one request, not started past review_stall.
	NotStarted
	// Stalled is a review under way past review_stall that does not count as asked.
	Stalled
)

// Post is a request the walk reached.
type Post struct {
	Request Request
	Why     Why
}

// Stop is the review that ends a wait, as --ready's not_started and next's
// review_stuck give it: the login as the config writes it, the review, code or
// security, the head's full SHA and the address of the latest request, an under
// way review's being its code request whichever review runs. Stalled is whether
// it is under way, else it has not started.
type Stop struct {
	Login   string `json:"login"`
	Review  string `json:"review"`
	Head    string `json:"head"`
	Request string `json:"request"`
	Stalled bool   `json:"-"`
}

// Walk is one look at the reviews of each of Logins, push.pr.review_by in
// the config's order, on Head, over State. Now is the clock, read when the judging
// starts. Post is the poster, which keeps the request (Record) and says whether the
// walk goes on; with none, the first post the walk reaches ends it. Between, when
// there is one, is called between the walk's two phases and says whether it goes on.
type Walk struct {
	Logins  []string
	Head    string
	State   providers.PullState
	Now     func() time.Time
	Post    func(Post) bool
	Between func() bool
}

// post is whether the walk goes on past the request.
func (w Walk) post(p Post) bool { return w.Post != nil && w.Post(p) }

// Walk walks the look, asking for each review the head lacks (askForHead), then,
// with a stall, judging it (judgeReviews): the Stop it reaches, nil when it reaches
// none or a poster or Between ends it first. --ready and next both call it (slice-224),
// so what next says of a pull request is what --ready would do.
func (rv *Reviewing) Walk(w Walk) *Stop {
	if !rv.askForHead(w) {
		return nil
	}
	if w.Between != nil && !w.Between() {
		return nil
	}
	if rv.stall <= 0 {
		return nil
	}
	return rv.judgeReviews(w)
}

// Stuck is the Stop a first look at the pull request would reach with nothing
// posted, at the time: nil when it would post first, or no clock has run. A time
// GitHub does not give counts from this look, so it has run for nothing.
func Stuck(logins []string, stall float64, state providers.PullState, head string, at time.Time) *Stop {
	return NewReviewing(stall).Walk(Walk{Logins: logins, Head: head, State: state, Now: func() time.Time { return at }})
}

// askForHead reaches each review, in a login's review-state comment, whose commit
// is not the head, one with no commit included (slices 200, 217): Codex here reviews
// on ready_for_review alone, so after a fix is pushed nothing else would, and each
// review is judged alone, whatever the other's commit or status. It does so whether
// or not a stall is set, unless a comment with that review's request for the head is
// on the pull request (Missing), or this wait posted it. A login with no review-state
// comment is asked nothing. It is whether the walk goes on.
func (rv *Reviewing) askForHead(w Walk) bool {
	for _, login := range w.Logins {
		if !w.State.StateOf(login).Stated {
			continue
		}
		for _, req := range Missing([]config.Reviewer{{Login: login, Kind: "codex"}}, w.State, w.Head) {
			if _, posted := rv.asked[req]; posted {
				continue
			}
			if !w.post(Post{Request: req, Why: ForHead}) {
				return false
			}
		}
	}
	return true
}

// judgeReviews judges a look under a stall, each login in the config's order that
// wrote a review-state comment, each of its reviews alone: its code review not
// started, then a review under way (stalled), then its security review not started
// (notStarted).
func (rv *Reviewing) judgeReviews(w Walk) *Stop {
	at := w.Now()
	for _, login := range w.Logins {
		st := w.State.StateOf(login)
		if !st.Stated {
			delete(rv.since, login)
			continue
		}
		if stop, ended := rv.notStarted(w, at, CodexReview(login), st.Code); ended {
			return stop
		}
		if stop, ended := rv.stalled(w, at, login, st); ended {
			return stop
		}
		if stop, ended := rv.notStarted(w, at, CodexSecurity(login), st.Security); ended {
			return stop
		}
	}
	return nil
}

// notStarted judges one review of a login's review-state comment (slice 218). One
// with a request for it on the head (requests) that reads not started review_stall seconds
// after the latest such request is posted once more, when there is one request, and
// stops when there are two. Two requests for a review and a head, ever: they are
// counted from the pull request, across waits, since a driver rerunning a --ready
// stopped here would otherwise post at each review_stall. The clock is GitHub's, as
// stalled's: a request's createdAt, or when this wait posted it until GitHub lists
// it; a time it does not give counts from the first look of this wait that read the
// review not started. ended is whether the walk ends here.
func (rv *Reviewing) notStarted(w Walk, at time.Time, req Request, review providers.StateReview) (stop *Stop, ended bool) {
	made := rv.requests(req, w.State.Comments, w.Head)
	if !review.NotStarted(w.Head) || len(made) == 0 {
		delete(rv.unstarted, req)
		return nil, false
	}
	if _, ok := rv.unstarted[req]; !ok {
		rv.unstarted[req] = at
	}
	last := latest(made)
	from := last.at
	if from.IsZero() {
		from = rv.unstarted[req]
	}
	if at.Sub(from).Seconds() < rv.stall {
		return nil, false
	}
	if len(made) > 1 {
		return &Stop{Login: req.Login, Review: req.Review(), Head: w.Head, Request: last.address}, true
	}
	return nil, !w.post(Post{Request: req, Why: NotStarted})
}

// stalled judges a login's reviews under way (slice 190): one that has read running
// for review_stall seconds by stallClock and does not count as asked is posted again, and
// one that counts as asked stops. A security request counts for nothing here: it
// neither takes the place of "@codex review" nor starts a clock. ended is whether
// the walk ends here.
func (rv *Reviewing) stalled(w Walk, at time.Time, login string, st providers.ReviewState) (stop *Stop, ended bool) {
	codeUnderWay := st.Code.UnderWay(w.Head)
	if !codeUnderWay && !st.Security.UnderWay(w.Head) {
		delete(rv.since, login)
		return nil, false
	}
	if _, ok := rv.since[login]; !ok {
		rv.since[login] = at
	}
	code := CodexReview(login)
	made := rv.requests(code, w.State.Comments, w.Head)
	from, asked := stallClock(rv.since[login], st.Updated, made, codeUnderWay)
	if at.Sub(from).Seconds() < rv.stall {
		return nil, false
	}
	if asked {
		review := code.Review()
		if !codeUnderWay {
			review = CodexSecurity(login).Review()
		}
		return &Stop{Login: login, Review: review, Head: w.Head, Request: latest(made).address, Stalled: true}, true
	}
	return nil, !w.post(Post{Request: code, Why: Stalled})
}

// stallClock is what a review under way counts from and whether its code review
// counts as asked. The clocks are GitHub's where it gives them (slice 200, bug-164):
// the review-state comment's last edit, when the review started or later; a request
// made at or after it instead, GitHub's times being whole seconds, and one this
// wait posted counting from when it posted it until GitHub lists it. A request
// older than that edit asked for what is running, not for it again, so one of them
// alone leaves the review unasked while its code review is under way; with the
// code review not under way "@codex review" would ask for nothing that runs, so it
// counts as asked, as two do. A time GitHub does not give counts from first, the
// first look of this wait that read the review running.
func stallClock(first, edited time.Time, made []requestMade, codeUnderWay bool) (time.Time, bool) {
	if len(made) == 0 {
		if edited.IsZero() {
			return first, false
		}
		return edited, false
	}
	last := latest(made).at
	switch {
	case last.IsZero():
		return first, true
	case edited.IsZero() || !last.Before(edited):
		return last, true
	}
	return edited, len(made) > 1 || !codeUnderWay
}
