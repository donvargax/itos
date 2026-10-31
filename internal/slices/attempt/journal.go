package attempt

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/donvargax/itos/v8/internal/kind"
)

// The verdicts a journal line records.
const (
	accepted = "accepted"
	rotated  = "rotated"
)

// record is one line of an item's journal: a result taken in, its Verdict
// and the SHA256 of the file's bytes; a key a rotated result had at fault
// is null.
type record struct {
	Schema  int     `json:"schema"`
	Role    string  `json:"role"`
	Attempt int     `json:"attempt"`
	Request *string `json:"request"`
	Base    *string `json:"base"`
	Outcome *string `json:"outcome"`
	Head    *string `json:"head"`
	Notes   *string `json:"notes"`
	Verdict string  `json:"verdict"`
	SHA256  string  `json:"sha256"`
}

// journal is an item's records, oldest first.
type journal []record

// readJournal is the records of a journal's bytes, a line each, blank
// lines passed over; a line that is no record is an error, a usage error
// as a data file itos refuses is.
func readJournal(id string, data []byte) (journal, error) {
	var j journal
	for i, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var r record
		if err := json.Unmarshal([]byte(line), &r); err != nil || r.Role == "" {
			return nil, kind.Wrap(kind.Usage, fmt.Errorf("line %d of %s's journal of attempts is not a record: %s", i+1, id, line))
		}
		j = append(j, r)
	}
	return j, nil
}

// taken is the number of results the journal holds for the role.
func (j journal) taken(role string) int {
	n := 0
	for _, r := range j {
		if r.Role == role {
			n++
		}
	}
	return n
}

// holds is whether the journal accepted the result: the same role,
// request, attempt, head and outcome, whatever its notes.
func (j journal) holds(r Result) bool {
	for _, rec := range j {
		if rec.Verdict == accepted && rec.Role == r.Role && rec.Attempt == r.Attempt &&
			same(rec.Request, &r.Request) && same(rec.Outcome, &r.Outcome) && same(rec.Head, r.Head) {
			return true
		}
	}
	return false
}

// read is whether the journal holds a line of the file's bytes.
func (j journal) read(sum string) bool {
	for _, rec := range j {
		if rec.SHA256 == sum {
			return true
		}
	}
	return false
}

// same is whether two optional texts are both none, or both the same text.
func same(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// line is the record as its journal line, LF-terminated, <, > and & as
// they are.
func (r record) line() []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(r)
	return buf.Bytes()
}

// digest is "sha256:" and the hex digest of the bytes.
func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
