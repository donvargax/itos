package attempt

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/message"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/tests"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	"github.com/donvargax/itos/v8/internal/work"
)

// Ports are what the attempt command reaches the world through, which
// internal/cli binds: Config, the config every command finds, whose VCS
// git is asked through; Request, the request itos brief computes for the
// item and role at the commit base, or the problems it refuses that brief
// for; ReadFile, the result file's bytes; Journal, the item's journal's
// bytes, none when it has none; and Record, which appends a line to it.
type Ports interface {
	Config(ctx context.Context) (*config.Loaded, error)
	Request(ctx context.Context, id, role, base string) (string, []out.Problem, error)
	ReadFile(ctx context.Context, path string) ([]byte, error)
	Journal(ctx context.Context, id string) ([]byte, error)
	Record(ctx context.Context, id string, line []byte) error
}

// Handler is the attempt command's application, over its ports.
type Handler struct {
	ports Ports
}

// NewHandler is the handler over p.
func NewHandler(p Ports) Handler {
	return Handler{ports: p}
}

// Taken is a result taken in: Already when the journal held it, and what
// the output echoes of it.
type Taken struct {
	Already bool
	Result
}

// Answer is what Handle found: the problems a result is Refused for, or
// the result Taken.
type Answer struct {
	Refused []out.Problem
	Taken   *Taken
}

func refused(rule, msg, fix string) Answer {
	return Answer{Refused: []out.Problem{{Rule: rule, Message: msg, Fix: fix}}}
}

// Handle checks the result q names for the item, in order, and records it
// when it is accepted or rotated. An error is a config, a registry, a
// journal or a repository that cannot be read, or a remote that cannot be
// fetched from, each with its kind.
func (h Handler) Handle(ctx context.Context, q Query) (Answer, error) {
	cfg, err := h.ports.Config(ctx)
	if err != nil {
		return Answer{}, err
	}
	file := cfg.Work.Registry
	if !cfg.Source.Has(ctx, file) {
		return Answer{Refused: []out.Problem{work.Missing(file, false)}}, nil
	}
	registry, err := work.Load(cfg, file)
	if err != nil {
		return Answer{}, err
	}
	if _, problem := work.Show(registry, q.ID); problem != nil {
		return Answer{Refused: []out.Problem{*problem}}, nil
	}
	data, err := h.ports.ReadFile(ctx, q.Path)
	if err != nil {
		return refused("attempt-malformed", "the result file cannot be read: "+err.Error(),
			"write the agent's result to "+q.File+" and submit it again"), nil
	}
	saved, err := h.ports.Journal(ctx, q.ID)
	if err != nil {
		return Answer{}, err
	}
	j, err := readJournal(q.ID, saved)
	if err != nil {
		return Answer{}, err
	}
	sum := digest(data)
	p := parse(data, q.File)
	if len(p.faults) > 0 {
		return h.rotate(ctx, q.ID, p, j, sum)
	}
	c := claim{cfg: cfg, v: cfg.VCS, id: q.ID, Result: p.Result}
	if a, err := h.current(ctx, &c); a.Refused != nil || err != nil {
		return a, err
	}
	if j.holds(c.Result) {
		return Answer{Taken: &Taken{Already: true, Result: c.Result}}, nil
	}
	if next := j.taken(c.Role) + 1; c.Attempt != next {
		return refused("attempt-out-of-order", fmt.Sprintf("%s's %s attempt %d is out of order: expected attempt %d", c.id, c.Role, c.Attempt, next),
			fmt.Sprintf("submit the result as attempt %d, the next of %s's %s", next, c.id, c.Role)), nil
	}
	if c.Outcome == done {
		if a, err := c.claims(ctx); a.Refused != nil || err != nil {
			return a, err
		}
	}
	r := c.Result
	rec := record{Schema: 1, Role: r.Role, Attempt: r.Attempt, Request: &r.Request, Base: &r.Base, Outcome: &r.Outcome,
		Head: r.Head, Notes: r.Notes, Verdict: accepted, SHA256: sum}
	if err := h.ports.Record(ctx, q.ID, rec.line()); err != nil {
		return Answer{}, err
	}
	return Answer{Taken: &Taken{Result: r}}, nil
}

// rotate refuses a malformed result, and records it rotated when it names
// a role, so that it counts as that role's next attempt: null for each key
// at fault, the journal's own number for the attempt. Bytes the journal
// holds already are refused with nothing recorded.
func (h Handler) rotate(ctx context.Context, id string, p parsed, j journal, sum string) (Answer, error) {
	msg := strings.Join(p.faults, "; ")
	fix := "write the result with exactly the keys itos attempt --help names, and submit it again"
	switch {
	case !p.good["role"]:
		return refused("attempt-malformed", msg+"; it names no role, so it counts for no attempt", fix), nil
	case j.read(sum):
		return refused("attempt-malformed", msg+"; the journal holds these bytes already, so they count for no attempt again", fix), nil
	}
	n := j.taken(p.Role) + 1
	rec := record{Schema: 1, Role: p.Role, Attempt: n, Request: p.text("request", p.Request), Base: p.text("base", p.Base),
		Outcome: p.text("outcome", p.Outcome), Head: p.Head, Notes: p.Notes, Verdict: rotated, SHA256: sum}
	if err := h.ports.Record(ctx, id, rec.line()); err != nil {
		return Answer{}, err
	}
	return refused("attempt-malformed", fmt.Sprintf("%s; recorded as %s's %s attempt %d, rotated", msg, id, p.Role, n),
		fmt.Sprintf("write the result with exactly the keys itos attempt --help names, and submit it as attempt %d", n+1)), nil
}

// claim is a well-formed result of the item id, checked against git
// through v under cfg, the clone's config; tip is the head of the item's
// branch on the remote once fetched, "" when the remote has none.
type claim struct {
	Result
	cfg    *config.Loaded
	v      vcsport.VCS
	id     string
	remote string
	tip    string
}

// current refuses a stale result: one of another item, a base the clone
// has no commit for once the item's branch is fetched, or a request not
// the one itos brief computes at that base. The branch is fetched for a
// done result, whose head is judged against its tip, and for a base the
// clone lacks, never otherwise.
func (h Handler) current(ctx context.Context, c *claim) (Answer, error) {
	fix := fmt.Sprintf("brief again with itos brief %s --role %s --json, and submit a result of that brief", c.id, c.Role)
	if c.Item != c.id {
		return refused("attempt-stale", fmt.Sprintf("the result is %s's, and was submitted for %s", c.Item, c.id), fix), nil
	}
	_, lacks := c.v.Resolve(ctx, c.Base)
	if c.Outcome == done || lacks != nil {
		if err := c.fetch(ctx); err != nil {
			return Answer{}, err
		}
	}
	if lacks != nil {
		if _, err := c.v.Resolve(ctx, c.Base); err != nil {
			return refused("attempt-stale", fmt.Sprintf("the result's base %s is no commit of the clone, %s's branch fetched", c.Base, c.id), fix), nil
		}
	}
	request, problems, err := h.ports.Request(ctx, c.id, c.Role, c.Base)
	switch {
	case err != nil:
		return Answer{}, err
	case len(problems) > 0:
		return refused("attempt-stale", fmt.Sprintf("itos brief refuses %s's %s at the base %s: %s", c.id, c.Role, c.Base, problems[0].Message), fix), nil
	case request != c.Request:
		return refused("attempt-stale", fmt.Sprintf("the result's request %s is not the one itos brief %s --role %s computes at its base %s, %s",
			c.Request, c.id, c.Role, c.Base, request), fix), nil
	}
	return Answer{}, nil
}

// fetch fetches the item's branch from the remote: the remote of the
// branch HEAD is on, origin when it is on none; tip is its head, "" when
// the remote has no such branch, which is no error. A remote out of reach
// is a temporary error, any other failure a missing environment.
func (c *claim) fetch(ctx context.Context) error {
	c.remote = "origin"
	if branch := c.v.Branch(ctx); branch != "" {
		c.remote, _, _ = c.v.Upstream(ctx, branch)
	}
	talk := vcsport.Talk{NoPrompt: true}
	ref := "refs/heads/" + c.id
	refs, err := c.v.RemoteRefs(ctx, c.remote, []string{ref}, talk)
	if err == nil && refs[ref] != "" {
		c.tip, err = c.v.Fetch(ctx, c.remote, "+"+ref+":refs/remotes/"+c.remote+"/"+c.id, talk)
	}
	if err == nil {
		return nil
	}
	err = fmt.Errorf("fetching %s's branch from %s: %w", c.id, c.remote, err)
	var failed *vcsport.Failed
	if errors.As(err, &failed) && failed.Failure == vcsport.Unreachable {
		return kind.Wrap(kind.Temporary, err)
	}
	return kind.Else(kind.Missing, err)
}

// claims checks what a done result claims against git, never its notes:
// its head the tip of the item's branch on the remote, fetched, then the
// role's work at that head.
func (c *claim) claims(ctx context.Context) (Answer, error) {
	head := *c.Head
	switch {
	case c.tip == "":
		return refused("attempt-head-not-pushed", fmt.Sprintf("the head %s is not pushed: %s has no branch %s", head, c.remote, c.id),
			fmt.Sprintf("push the work to %s's branch %s, then submit its tip as the head", c.remote, c.id)), nil
	case c.tip != head:
		return refused("attempt-head-not-pushed", fmt.Sprintf("the head %s is not the tip of %s's branch %s: the tip is %s", head, c.remote, c.id, c.tip),
			fmt.Sprintf("push the head to %s's branch %s, or submit the tip as the head when it is the work", c.remote, c.id)), nil
	}
	tagged, err := tests.Tagged(c.cfg, "@"+c.id, head)
	if err != nil {
		return Answer{}, err
	}
	if c.Role == drafter {
		return c.spec(tagged), nil
	}
	return c.implemented(ctx, tagged)
}

// spec refuses a drafter's head holding no scenario tagged for the item,
// or one that is not @wip.
func (c *claim) spec(tagged []tests.Test) Answer {
	fix := fmt.Sprintf("tag the item's scenarios @wip @%s, push them, and submit the new tip as the head", c.id)
	if len(tagged) == 0 {
		return refused("attempt-spec-missing", fmt.Sprintf("no scenario is tagged @%s at the head %s", c.id, *c.Head), fix)
	}
	for _, t := range tagged {
		if t.Live {
			return refused("attempt-spec-missing", fmt.Sprintf("%s (%s) is tagged @%s and not @wip at the head %s: a drafter's spec is @wip", t.ID, t.File, c.id, *c.Head), fix)
		}
	}
	return Answer{}
}

// implemented refuses an implementer's head whose first-parent commits
// past the base hold no test commit before the first feat or fix, or that
// leaves a scenario tagged for the item @wip.
func (c *claim) implemented(ctx context.Context, tagged []tests.Test) (Answer, error) {
	first, err := c.untested(ctx)
	if err != nil {
		return Answer{}, err
	}
	if first != nil {
		return refused("attempt-not-red-first", fmt.Sprintf("no test commit comes before %s (%s), the first feat or fix past the base %s", first.Subject, first.SHA, c.Base),
			"commit the steps alone in a test commit, the scenarios still @wip, before the implementation, and push"), nil
	}
	for _, t := range tagged {
		if !t.Live {
			return refused("attempt-still-wip", fmt.Sprintf("%s (%s) is tagged @%s and still @wip at the head %s", t.ID, t.File, c.id, *c.Head),
				"turn the item's scenarios live in the commit that makes them pass, push, and submit the new tip as the head"), nil
		}
	}
	return Answer{}, nil
}

// untested is the first feat or fix commit on the head's first-parent
// history past the base when no test commit comes before it, nil when one
// does or there is none: a merge's commits from another branch are not the
// item's. The commits past the base are git's list of their names, and
// each judged one must have its record in the log: a message holding the
// byte the log's records are framed by loses its record, and a commit
// whose type cannot be read is an error, never passed over.
func (c *claim) untested(ctx context.Context) (*vcsport.Entry, error) {
	r := vcsport.Range{From: c.Base, To: *c.Head}
	names, err := c.v.Commits(ctx, r)
	if err != nil || len(names) == 0 {
		return nil, err
	}
	past, err := c.v.Log(ctx, r)
	if err != nil {
		return nil, err
	}
	byName := make(map[string]vcsport.Entry, len(past))
	for _, e := range past {
		byName[e.SHA] = e
	}
	in := make(map[string]bool, len(names))
	for _, name := range names {
		in[name] = true
	}
	chain, err := c.v.FirstParents(ctx, *c.Head, 0, len(names))
	if err != nil {
		return nil, err
	}
	tested := false
	for i := len(chain) - 1; i >= 0; i-- {
		if !in[chain[i]] {
			continue
		}
		e, ok := byName[chain[i]]
		if !ok {
			return nil, kind.Wrap(kind.Usage, fmt.Errorf("the commit %s past the base has no record in git log, so its type cannot be read: its message may hold the byte 0x1e", chain[i]))
		}
		switch message.Type(e.Message) {
		case "test":
			tested = true
		case "feat", "fix":
			if tested {
				return nil, nil
			}
			return &e, nil
		}
	}
	return nil, nil
}
