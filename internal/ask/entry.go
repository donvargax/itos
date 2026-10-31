package ask

import "strconv"

// Entry is a question as --json gives it.
type Entry struct {
	ID       string `json:"id"`
	Item     string `json:"item,omitempty"`
	Question string `json:"question"`
	Status   string `json:"status"`
	Answer   string `json:"answer,omitempty"`
	// Class is the question's decision class, when it has one (slice 205);
	// Provisional, true while an answering agent's answer awaits the person,
	// By who gave it, and Overturned the agent's answer the person's replaced.
	Class       string `json:"class,omitempty"`
	Provisional bool   `json:"provisional,omitempty"`
	By          string `json:"by,omitempty"`
	Overturned  string `json:"overturned,omitempty"`
	// Decision is the record's number, or "none".
	Decision any `json:"decision,omitempty"`
}

// EntryOf is the question as --json gives it, its decision a number when it
// names a record.
func EntryOf(q Question) Entry {
	e := Entry{ID: q.ID, Item: q.Item, Question: q.Question, Status: q.Status(), Answer: q.Answer,
		Class: q.Class, Provisional: q.Provisional, By: q.By, Overturned: q.Overturned}
	if n, err := strconv.Atoi(q.Decision); err == nil {
		e.Decision = n
	} else if q.Decision != "" {
		e.Decision = q.Decision
	}
	return e
}
