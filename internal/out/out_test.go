package out

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEmitKeepsTheOrderAndSchemaFirst(t *testing.T) {
	var b strings.Builder
	err := Emit(&b,
		Field{"version", "1.0.0"},
		Field{"requires", ">=0.1.0 <2 & more"},
		Field{"problems", []Problem{{Rule: "r", Message: "m"}}},
		Field{"none", []string{}})
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "schema": 1,
  "version": "1.0.0",
  "requires": ">=0.1.0 <2 & more",
  "problems": [
    {
      "rule": "r",
      "message": "m"
    }
  ],
  "none": []
}
`
	if b.String() != want {
		t.Errorf("Emit wrote\n%s\nwant\n%s", b.String(), want)
	}
}

// A problem of a failed task check carries its run's keys, its exit null when
// the check did not exit by itself; any other problem has none of them.
func TestProblemWithACheckRun(t *testing.T) {
	exit := 2
	for _, c := range []struct {
		problem Problem
		want    string
	}{
		{Problem{Rule: "r", Message: "m"}, `{"rule":"r","message":"m"}`},
		{Problem{Rule: "r", Message: "m", CheckRun: &CheckRun{Task: "T-1", Command: "c", Exit: &exit, Output: "o"}},
			`{"rule":"r","message":"m","task":"T-1","command":"c","exit":2,"timed_out":false,"output":"o"}`},
		{Problem{Rule: "r", Message: "m", CheckRun: &CheckRun{Task: "T-1", Command: "c", TimedOut: true}},
			`{"rule":"r","message":"m","task":"T-1","command":"c","exit":null,"timed_out":true,"output":""}`},
	} {
		got, err := json.Marshal(c.problem)
		if err != nil || string(got) != c.want {
			t.Errorf("got %s, %v, want %s", got, err, c.want)
		}
	}
}
