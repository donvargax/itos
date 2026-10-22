package ask

import "testing"

// A question's entry carries its status, its decision a record's number,
// none as written, and nothing while it has none.
func TestEntryOf(t *testing.T) {
	for _, c := range []struct {
		q    Question
		want Entry
	}{
		{Question{ID: "q-1", Item: "T-1", Question: "Which?"}, Entry{ID: "q-1", Item: "T-1", Question: "Which?", Status: Open}},
		{Question{ID: "q-2", Question: "Which?", Answer: "This.", Decision: "53"},
			Entry{ID: "q-2", Question: "Which?", Status: Answered, Answer: "This.", Decision: 53}},
		{Question{ID: "q-3", Question: "Which?", Answer: "This.", Decision: None},
			Entry{ID: "q-3", Question: "Which?", Status: Answered, Answer: "This.", Decision: None}},
	} {
		if got := EntryOf(c.q); got != c.want {
			t.Errorf("%+v: %+v, not %+v", c.q, got, c.want)
		}
	}
}
