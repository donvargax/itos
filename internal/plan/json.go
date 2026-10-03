package plan

import (
	"fmt"
	"io"
	"strings"

	"github.com/donvargax/itos/v2/internal/check"
	"github.com/donvargax/itos/v2/internal/git"
	"github.com/donvargax/itos/v2/internal/out"
)

// The plan as JSON, which `ci plan --json` prints (ci-plan-json.ts): the
// contract two implementations of itos are compared on, so its keys, their
// order and their values are the TypeScript's. Keys are only ever added.
//
// Each item of `order` has a `cost` (static or late) and where it came from
// (`cost_from`: explicit, pattern, default or order); a check also has its
// `action`, with `kind` when merged and `covered_by` when covered.

// Range is the range a plan is of, as the JSON gives it.
type Range struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Readable bool   `json:"readable"`
	Nightly  bool   `json:"nightly"`
}

// fullSHA is a commit's full SHA, or what was given when git cannot resolve
// it.
func fullSHA(ref string) string {
	if ref == "" {
		return ref
	}
	sha, err := git.Output("rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if sha = strings.TrimSpace(sha); err != nil || sha == "" {
		return ref
	}
	return sha
}

// RangeOf is a range as the JSON gives it: its ends as full SHAs where git
// resolves them, whether it can be read (never for the nightly), and
// whether it is the nightly.
func RangeOf(from, to string, nightly bool) Range {
	return Range{fullSHA(from), fullSHA(to), !nightly && Readable(from, to), nightly}
}

type checkJSON struct {
	Task    string `json:"task"`
	Index   int    `json:"index"`
	Command string `json:"command"`
	Fails   bool   `json:"fails,omitempty"`
}

type itemJSON struct {
	Step      *string        `json:"step,omitempty"`
	Check     *checkJSON     `json:"check,omitempty"`
	Cost      check.Cost     `json:"cost"`
	CostFrom  check.CostFrom `json:"cost_from"`
	Action    Action         `json:"action,omitempty"`
	Kind      string         `json:"kind,omitempty"`
	CoveredBy string         `json:"covered_by,omitempty"`
}

func (c *Check) json() *checkJSON {
	return &checkJSON{c.Task.ID, c.Index, c.Check.Command(), c.Check.Fails != nil}
}

func (i Item) json() itemJSON {
	if i.Step != nil {
		return itemJSON{Step: &i.Step.Command, Cost: i.Step.Cost, CostFrom: i.Step.From}
	}
	c := i.Check
	item := itemJSON{Check: c.json(), Cost: c.Cost, CostFrom: c.From, Action: c.Action}
	switch c.Action {
	case Merged:
		item.Kind = c.Kind
	case Covered:
		item.CoveredBy = c.CoveredBy
	}
	return item
}

// Fields are the plan's JSON object, key by key after its schema.
func (p *Plan) Fields(r Range) []out.Field {
	order := make([]itemJSON, len(p.Order))
	for i, item := range p.Order {
		order[i] = item.json()
	}
	leftOut := make([]*checkJSON, len(p.LeftOut))
	for i, c := range p.LeftOut {
		leftOut[i] = c.json()
	}
	list := func(l []string) []string {
		if l == nil {
			return []string{}
		}
		return l
	}
	return []out.Field{
		{Key: "range", Value: r},
		{Key: "prose", Value: p.Prose},
		{Key: "steps", Value: list(p.Steps)},
		{Key: "order", Value: order},
		{Key: "tasks", Value: list(p.Tasks)},
		{Key: "left_out", Value: leftOut},
		{Key: "unknown", Value: list(p.Unknown)},
		{Key: "not_started", Value: list(p.NotStarted)},
	}
}

// Print prints the plan as text, one item a line: a step's cost and
// command, a check's cost, task, command and action.
func (p *Plan) Print(w io.Writer) {
	for _, item := range p.Order {
		if item.Step != nil {
			fmt.Fprintf(w, "%s  %s\n", item.Step.Cost, item.Step.Command)
			continue
		}
		c := item.Check
		fmt.Fprintf(w, "%s  %s: %s   (%s)\n", c.Cost, c.Task.ID, c.Check.Command(), c.Action)
	}
}
