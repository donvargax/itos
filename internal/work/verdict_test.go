package work

import "testing"

// A marker written is read back, the last of a body that quotes another;
// anything else is prose.
func TestVerdictMarkers(t *testing.T) {
	head := "0123456789abcdef0123456789abcdef01234567"
	v := Verdict{Record: "verdict", Item: "slice-9", Stage: "spec", Verdict: "approve", Head: head, Round: 2}
	written := v.Marker()
	if written != `<!-- itos:v1 {"record":"verdict","item":"slice-9","stage":"spec","verdict":"approve","head":"`+head+`","round":2} -->` {
		t.Errorf("written %s", written)
	}
	quoted := Verdict{Record: "verdict", Item: "slice-9", Stage: "spec", Verdict: "changes", Head: head, Round: 1}.Marker()
	if got, ok := ReadVerdict("Round 1 was " + quoted + ".\n\n" + written + "\n"); !ok || got != v {
		t.Errorf("read %+v, %t", got, ok)
	}
	for _, body := range []string{"", "prose", "<!-- itos:v1 -->", "<!-- itos:v1 {} -->", `<!-- itos:v1 {"record":"verdict"`} {
		if _, ok := ReadVerdict(body); ok {
			t.Errorf("%q read as a verdict", body)
		}
	}
}

// The people's logins count regardless of case, and with nobody listed
// every login does.
func TestCounts(t *testing.T) {
	people := Registry{People: true, Logins: []string{"Ana"}}
	if !people.Counts("ana") || people.Counts("someone") {
		t.Error("the people's logins are not the ones counted")
	}
	if !(Registry{}).Counts("anyone") {
		t.Error("with nobody listed, a login does not count")
	}
}
