package next

import (
	"strings"

	"github.com/donvargax/itos/v8/internal/ask"
	"github.com/donvargax/itos/v8/internal/value"
	"github.com/donvargax/itos/v8/internal/work"
)

// Next is an item's state and next step, as next prints it: the Item's id,
// its Kind ("" for none), its State, its Owner as written (nil for null or
// none), the Actor and Action of the step, the Argv of the itos command the
// step runs when it is a deterministic one, WaitsOn, what the state waits
// on, BlockedReason, "decision", "status" or "attempt" on a blocked item,
// Observed, what the forge and the journal said when they were asked, and
// the Revision of the facts it was judged from.
type Next struct {
	Item          string
	Kind          string
	State         string
	Owner         any
	Actor, Action string
	Argv          []string
	WaitsOn       []string
	BlockedReason string
	Observed      *Observed
	Revision      string
}

// inFlight is the state of an item with a spec, slice-203's stand-in for
// the states the forge knows, which push.mode pr asks it for.
const inFlight = "in-flight"

// Judge is the item's state and next step from its facts: the first state
// that holds wins, in the order done, parked, blocked (an open question
// before the registry's status), waiting, idea, needs-spec, then, with a
// spec, the forge's states when it was asked (forge), else in-flight.
func Judge(f Facts) Next {
	n := Next{Item: value.String(f.Item.At("id")), Kind: f.Kind, Observed: f.Observed, Revision: Revision(f)}
	if owner := f.Item.At("owner"); owner != value.Undefined {
		n.Owner = owner
	}
	status := f.Item.At("status")
	waits := undone(f.Dependencies)
	switch {
	case status == "done":
		n.step("done", "none", "none")
	case status == work.Dropped || f.Item.At("deferred") != value.Undefined:
		n.step("parked", "none", "none")
	case len(f.Questions) > 0:
		n.step("blocked", "person", "answer")
		n.BlockedReason = "decision"
		for _, q := range f.Questions {
			n.WaitsOn = append(n.WaitsOn, q.ID)
		}
	case status == "blocked":
		n.step("blocked", "person", "answer")
		n.BlockedReason = "status"
		n.WaitsOn = blockedBy(f.Item)
	case waits != nil:
		n.step("waiting", "none", "wait")
		n.WaitsOn = waits
	case f.Kind != "slice" && f.Kind != "bug" && f.Kind != "task":
		n.step("idea", "person", "promote")
	case len(f.Scenarios) == 0 && len(f.Checks) == 0:
		n.step("needs-spec", "drafter", "specify")
		n.Argv = []string{"itos", "brief", n.Item, "--role", "drafter"}
	case f.Observed != nil:
		n.forge(f)
	default:
		n.step(inFlight, "forge", "observe")
	}
	return n
}

// forge is the state of an item with a spec under push.mode pr, from what
// the forge and the journal said, the first that holds winning: merged,
// repair, review-unrequested where merging would be, merging, blocked on the
// attempt, spec-changes on a spec defect, review-unrequested where
// handed-back would be, handed-back, implementing, spec-accepted,
// spec-changes, spec-review, spec-proposed; an item neither todo nor doing
// past those is in-flight. review-unrequested (slice-215) is a configured
// reviewer not requested for the pull request's head.
func (n *Next) forge(f Facts) {
	o, pull := f.Observed, f.Observed.Pull
	status := f.Item.At("status")
	at := o.latest("implementer")
	var latest Record
	if at >= 0 {
		latest = o.Records[at]
	}
	implementer := []string{"itos", "brief", n.Item, "--role", "implementer"}
	drafter := []string{"itos", "brief", n.Item, "--role", "drafter"}
	request := []string{"itos", "review", "request", n.Item}
	unrequested := len(o.Unrequested) > 0
	switch {
	case pull != nil && pull.Merged:
		n.run("merged", "driver", "close", "itos", "work", "done", n.Item)
	case pull != nil && !pull.Draft && (o.Checks == checksFailure || o.ThreadsOpen > 0):
		n.run("repair", "implementer", "implement", implementer...)
	case pull != nil && !pull.Draft && unrequested:
		n.run("review-unrequested", "driver", "request", request...)
	case pull != nil && !pull.Draft:
		n.step("merging", "forge", "observe")
	case at >= 0 && latest.asksPerson() && !spent(f.Asked, o.nth(at)):
		n.step("blocked", "person", "answer")
		n.BlockedReason = "attempt"
	case at >= 0 && latest.outcome() == "spec_defect" && !o.drafterAfter(at):
		n.run("spec-changes", "drafter", "specify", drafter...)
	case status == "doing" && pull != nil && handedBack(latest, pull.Head) && unrequested:
		n.run("review-unrequested", "driver", "request", request...)
	case status == "doing" && pull != nil && handedBack(latest, pull.Head):
		n.run("handed-back", "driver", "land", "itos", "push", "--ready")
	case status == "doing":
		n.run("implementing", "implementer", "implement", implementer...)
	case status == "todo" && o.Verdict == "approve":
		n.run("spec-accepted", "driver", "take", "itos", "work", "take", n.Item)
	case status == "todo" && o.Verdict == "changes":
		n.run("spec-changes", "drafter", "specify", drafter...)
	case status == "todo" && pull != nil:
		n.run("spec-review", "reviewer", "review", "itos", "brief", n.Item, "--role", "reviewer")
	case status == "todo":
		n.run("spec-proposed", "drafter", "specify", drafter...)
	default:
		n.step(inFlight, "forge", "observe")
	}
}

// spent is whether the k-th question naming the item, in the questions
// file's order, is answered: the k-th implementer record asking the person
// is answered by it, so an answer from before a record cannot spend it.
func spent(asked []ask.Question, k int) bool {
	return k <= len(asked) && asked[k-1].Status() == ask.Answered
}

// handedBack is whether the implementer's latest record is accepted, done,
// at the pull request's head.
func handedBack(r Record, head string) bool {
	return r.Verdict == "accepted" && r.outcome() == "done" && r.Head != nil && strings.EqualFold(*r.Head, head)
}

// run is the step with the argv of the deterministic itos command it runs.
func (n *Next) run(state, actor, action string, argv ...string) {
	n.step(state, actor, action)
	n.Argv = argv
}

func (n *Next) step(state, actor, action string) {
	n.State, n.Actor, n.Action = state, actor, action
}

// blockedBy are the ids the item's blocked_by lists, none when it is prose
// (why, in words, as the registry's header describes it) or absent.
func blockedBy(item *value.Map) []string {
	listed, _ := item.At("blocked_by").([]any)
	var ids []string
	for _, id := range listed {
		ids = append(ids, value.String(id))
	}
	return ids
}

// undone are the ids of the dependencies not done, in the depends_on's
// order; nil for none.
func undone(deps []Dependency) []string {
	var ids []string
	for _, d := range deps {
		if !d.Done() {
			ids = append(ids, d.ID)
		}
	}
	return ids
}
