package ask

import (
	"strings"
	"testing"
)

// What Text writes, Parse reads back as it was, texts with quotes, newlines,
// tabs and a colon included.
func TestTextReadsBack(t *testing.T) {
	var f File
	f.Add("Labels or Projects?", "slice-9")
	q := f.Add("Two lines:\nthe second \"quoted\" & <b> 'single' ünïcode \t tab", "")
	q.Answer = "Labels: yes."
	back, err := Parse("asks.yaml", Text(f))
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Questions) != 2 || back.Questions[0] != f.Questions[0] || back.Questions[1] != f.Questions[1] {
		t.Errorf("read back %+v, wrote %+v", back.Questions, f.Questions)
	}
	if back.Questions[0].Status() != Open || back.Questions[1].Status() != Answered {
		t.Errorf("statuses %s, %s", back.Questions[0].Status(), back.Questions[1].Status())
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
	if got := f.Add("e", "").ID; got != "q-10" {
		t.Errorf("next id %s, want q-10", got)
	}
	if about := f.About(""); len(about) != 4 {
		t.Errorf("about no item: %d", len(about))
	}
}

// A file itos did not write whole is refused, naming it.
func TestParseRefuses(t *testing.T) {
	for name, text := range map[string]string{
		"empty":       "",
		"comment":     "# nothing\n",
		"unknown key": "questions: []\nother: 1\n",
		"bad id":      "questions:\n  - { id: x-1, question: a }\n",
		"zero id":     "questions:\n  - { id: q-0, question: a }\n",
		"twice":       "questions:\n  - { id: q-1, question: a }\n  - { id: q-1, question: b }\n",
		"no text":     "questions:\n  - { id: q-1, question: \" \" }\n",
		"two docs":    "questions: []\n---\nquestions: []\n",
		"field":       "questions:\n  - { id: q-1, question: a, status: open }\n",
	} {
		if _, err := Parse("tasks/asks.yaml", text); err == nil || !strings.Contains(err.Error(), "tasks/asks.yaml") {
			t.Errorf("%s: %v", name, err)
		}
	}
}
