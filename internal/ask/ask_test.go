package ask

import (
	"slices"
	"strings"
	"testing"
)

// What Text writes, Parse reads back as it was, texts with quotes, newlines,
// tabs and a colon included, and a class, a provisional answer with who
// gave it, and an overturned one.
func TestTextReadsBack(t *testing.T) {
	var f File
	f.Add("Labels or Projects?", "slice-9", "")
	q := f.Add("Two lines:\nthe second \"quoted\" & <b> 'single' ünïcode \t tab", "", "")
	q.Answer = "Labels: yes."
	f.Add("Recorded?", "", "").Answer = "Yes."
	f.Questions[2].Decision = "12"
	f.Add("Recorded nowhere?", "", "").Answer = "No."
	f.Questions[3].Decision = None
	agent := f.Add("Delegated?", "", "spec: \"review\"")
	agent.Answer, agent.Provisional, agent.By = "Yes.", true, "agent: 1"
	person := f.Add("Overturned?", "", "spec")
	person.Answer, person.By, person.Overturned = "No.", "agent-1", "Yes."
	back, err := Parse("asks.yaml", Text(f))
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Questions) != 6 || !slices.Equal(back.Questions, f.Questions) {
		t.Errorf("read back %+v, wrote %+v", back.Questions, f.Questions)
	}
	if back.Questions[0].Status() != Open || back.Questions[1].Status() != Answered || back.Questions[4].Status() != Answered {
		t.Errorf("statuses %s, %s, %s", back.Questions[0].Status(), back.Questions[1].Status(), back.Questions[4].Status())
	}
	if !strings.Contains(Text(f), "\n    class: \"spec\"\n    answer: \"No.\"\n    by: \"agent-1\"\n    overturned: \"Yes.\"\n") ||
		!strings.Contains(Text(f), "\n    answer: \"Yes.\"\n    provisional: true\n    by: \"agent: 1\"\n") {
		t.Errorf("written as %q", Text(f))
	}
	for i, want := range []bool{false, true, false, false, false, true} {
		if got := f.Questions[i].Unrecorded(); got != want {
			t.Errorf("%s unrecorded: %t", f.Questions[i].ID, got)
		}
	}
	if empty, err := Parse("asks.yaml", Text(File{})); err != nil || len(empty.Questions) != 0 {
		t.Errorf("no questions read back as %+v, %v", empty, err)
	}
}

// The next id is one past the highest, answered ones and gaps counted, so an
// id is never given twice.
func TestNextID(t *testing.T) {
	var f File
	if f.NextID() != "q-1" {
		t.Errorf("first id %s", f.NextID())
	}
	f.Questions = []Question{{ID: "q-2", Question: "a", Answer: "b"}, {ID: "q-9", Question: "c"}, {ID: "q-4", Question: "d"}}
	if got := f.Add("e", "", "").ID; got != "q-10" {
		t.Errorf("next id %s, want q-10", got)
	}
	if about := f.About(""); len(about) != 4 {
		t.Errorf("about no item: %d", len(about))
	}
}

// A file itos did not write whole is refused, naming it.
func TestParseRefuses(t *testing.T) {
	for name, text := range map[string]string{
		"empty":        "",
		"comment":      "# nothing\n",
		"unknown key":  "questions: []\nother: 1\n",
		"bad id":       "questions:\n  - { id: x-1, question: a }\n",
		"zero id":      "questions:\n  - { id: q-0, question: a }\n",
		"twice":        "questions:\n  - { id: q-1, question: a }\n  - { id: q-1, question: b }\n",
		"no text":      "questions:\n  - { id: q-1, question: \" \" }\n",
		"two docs":     "questions: []\n---\nquestions: []\n",
		"field":        "questions:\n  - { id: q-1, question: a, status: open }\n",
		"bad decision": "questions:\n  - { id: q-1, question: a, answer: b, decision: 0001 }\n",
		"open decided": "questions:\n  - { id: q-1, question: a, decision: 3 }\n",
		// A provisional answer is an answer, by someone, and no decision yet;
		// who answered and an overturned answer go with an answer.
		"open provisional":    "questions:\n  - { id: q-1, question: a, provisional: true, by: x }\n",
		"provisional nobody":  "questions:\n  - { id: q-1, question: a, answer: b, provisional: true }\n",
		"provisional decided": "questions:\n  - { id: q-1, question: a, answer: b, provisional: true, by: x, decision: 3 }\n",
		"open by":             "questions:\n  - { id: q-1, question: a, by: x }\n",
		"open overturned":     "questions:\n  - { id: q-1, question: a, overturned: b }\n",
	} {
		if _, err := Parse("tasks/asks.yaml", text); err == nil || !strings.Contains(err.Error(), "tasks/asks.yaml") {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// Rename changes every question naming the item, answered or not, and no
// other, and names the ones it changed in the order asked.
func TestRename(t *testing.T) {
	var f File
	f.Add("One?", "p1-idea", "")
	f.Add("Two?", "slice-2", "")
	f.Add("Three?", "p1-idea", "").Answer = "Yes."
	f.Add("Four?", "", "")
	renamed := f.Rename("p1-idea", "slice-5")
	if strings.Join(renamed, ",") != "q-1,q-3" {
		t.Errorf("renamed %v", renamed)
	}
	if f.Questions[0].Item != "slice-5" || f.Questions[1].Item != "slice-2" ||
		f.Questions[2].Item != "slice-5" || f.Questions[3].Item != "" {
		t.Errorf("items after %+v", f.Questions)
	}
	if again := f.Rename("p1-idea", "slice-6"); len(again) != 0 {
		t.Errorf("renamed again %v", again)
	}
}
