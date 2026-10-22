package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

// itos decision --all lists each answered question with its answer, and
// under it the decision it was recorded as, when it was: a record's number,
// or none; an answer recorded nowhere has no such line.
func TestDecisionListsTheDecisionsRecorded(t *testing.T) {
	gitConfigRepo(t, "version: 1\n")
	putFile(t, filepath.Join("tasks", "asks.yaml"), "questions:\n"+
		"  - id: q-1\n    question: \"Open?\"\n"+
		"  - id: q-2\n    question: \"Recorded?\"\n    answer: \"Yes.\"\n    decision: 7\n"+
		"  - id: q-3\n    question: \"Item only?\"\n    answer: \"Yes.\"\n    decision: none\n"+
		"  - id: q-4\n    question: \"Not yet?\"\n    answer: \"No.\"\n")
	code, stdout, stderr := run("decision", "--all")
	want := "Open questions:\n  q-1  Open?\n\nAnswered questions:\n" +
		"  q-2  Recorded?\n       → Yes.\n         decision 7\n" +
		"  q-3  Item only?\n       → Yes.\n         no decision (--none)\n" +
		"  q-4  Not yet?\n       → No.\n" +
		"\nAnswered and recorded nowhere: itos decision record q-4 (--title <title>, or --none)\n"
	if code != 0 || stdout != want {
		t.Errorf("exit %d, stderr %s\n got %q\nwant %q", code, stderr, stdout, want)
	}
	code, stdout, stderr = run("decision", "--all", "--json")
	if code != 0 || !strings.Contains(stdout, `"decision": 7`) || !strings.Contains(stdout, `"decision": "none"`) ||
		strings.Count(stdout, `"decision"`) != 2 {
		t.Errorf("--json: exit %d, stderr %s\n%s", code, stderr, stdout)
	}
}
