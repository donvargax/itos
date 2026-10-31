package work

import (
	"encoding/json"
	"strings"
)

// The marker a verdict comment ends with: the hidden JSON between these,
// its record verdict (ReadVerdict, Verdict.Marker).
const (
	verdictStart = "<!-- itos:v1 "
	verdictEnd   = " -->"
)

// Verdict is a verdict marker's JSON, its keys in the order written: a
// reviewer's verdict on an item's pull request at the head it judged, as
// itos review post records it (slice-206) and itos next reads it
// (slice-209).
type Verdict struct {
	Record  string `json:"record"`
	Item    string `json:"item"`
	Stage   string `json:"stage"`
	Verdict string `json:"verdict"`
	Head    string `json:"head"`
	Round   int    `json:"round"`
}

// ReadVerdict reads a comment's verdict marker: the JSON between the last
// marker start and the end after it, whose record is verdict; the last,
// since itos appends the marker after the review's text, which may quote
// one. A body without one, with one that is not JSON or with another record
// is prose, not ok.
func ReadVerdict(body string) (Verdict, bool) {
	at := strings.LastIndex(body, verdictStart)
	if at < 0 {
		return Verdict{}, false
	}
	text, _, found := strings.Cut(body[at+len(verdictStart):], verdictEnd)
	if !found {
		return Verdict{}, false
	}
	var v Verdict
	if err := json.Unmarshal([]byte(text), &v); err != nil || v.Record != "verdict" {
		return Verdict{}, false
	}
	return v, true
}

// Marker is the marker a program reads: the start, the JSON with its keys
// in Verdict's order and nothing else in it, the end.
func (v Verdict) Marker() string {
	text, _ := json.Marshal(v)
	return verdictStart + string(text) + verdictEnd
}

// Counts is whether a verdict comment by the login counts: among the
// people, compared regardless of case as GitHub's logins are, or anyone's
// when the registry has no people to hold owners to (Nobody).
func (r Registry) Counts(login string) bool {
	if r.Nobody() {
		return true
	}
	for _, person := range r.Logins {
		if strings.EqualFold(person, login) {
			return true
		}
	}
	return false
}
