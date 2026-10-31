package attempt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"slices"
	"sort"
	"strconv"
)

// The roles a result may name, and its outcomes.
const (
	drafter     = "drafter"
	implementer = "implementer"
	done        = "done"
)

var (
	roles    = []string{drafter, implementer}
	outcomes = []string{done, "blocked", "needs_decision", "spec_defect"}
	// keys are a result's keys, in the contract's order.
	keys = []string{"schema", "item", "role", "request", "base", "attempt", "outcome", "head", "notes"}
	// requestForm is itos brief's request, fullSHA a commit's full name,
	// SHA-1's or SHA-256's, as git prints it.
	requestForm = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	fullSHA     = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
)

// Result is a well-formed result: the Item and Role it is of, the Request
// and Base of its brief, its Attempt, its Outcome, the Head it claims (nil
// with an outcome that claims none) and its Notes (nil for none).
type Result struct {
	Item, Role, Request, Base, Outcome string
	Attempt                            int
	Head, Notes                        *string
}

// parsed is a result file read: the Result's fields that are well formed,
// each key that is in good, and the faults, each naming its key, none for
// a well-formed result.
type parsed struct {
	Result
	good   map[string]bool
	faults []string
}

// text is the field's value when its key is good, nil otherwise, as a
// rotated record writes it.
func (p parsed) text(key, v string) *string {
	if !p.good[key] {
		return nil
	}
	return &v
}

// parse reads a result file's bytes, file the name it is read as: one JSON
// object whose keys are each given once, all of them the contract's, each
// of its type and form.
func parse(data []byte, file string) parsed {
	p := parsed{good: map[string]bool{}}
	values, twice, err := object(data)
	if err != nil {
		p.faults = []string{fmt.Sprintf("the result file %s is not one JSON object: %v", file, err)}
		return p
	}
	var others []string
	for key := range values {
		if !slices.Contains(keys, key) {
			others = append(others, key)
		}
	}
	sort.Strings(others)
	for _, key := range others {
		p.faults = append(p.faults, "the result has the key "+key+", which a result does not have")
	}
	for _, key := range twice {
		p.faults = append(p.faults, "the result gives the key "+key+" twice")
	}
	field := func(key string, read func(raw json.RawMessage) string) {
		raw, given := values[key]
		switch {
		case slices.Contains(twice, key):
		case !given:
			p.faults = append(p.faults, "the result has no "+key)
		default:
			if fault := read(raw); fault != "" {
				p.faults = append(p.faults, fmt.Sprintf("the result's %s is %s, %s", key, raw, fault))
				return
			}
			p.good[key] = true
		}
	}
	field("schema", func(raw json.RawMessage) string {
		if n, ok := integer(raw); !ok || n != 1 {
			return "not 1"
		}
		return ""
	})
	field("item", func(raw json.RawMessage) string {
		s, ok := text(raw)
		if !ok || s == "" {
			return "not an item's id"
		}
		p.Item = s
		return ""
	})
	field("role", func(raw json.RawMessage) string {
		s, ok := text(raw)
		if !ok || !slices.Contains(roles, s) {
			return "not drafter or implementer"
		}
		p.Role = s
		return ""
	})
	field("request", func(raw json.RawMessage) string {
		s, ok := text(raw)
		if !ok || !requestForm.MatchString(s) {
			return `not "sha256:" and 64 lowercase hex digits, the request itos brief gives`
		}
		p.Request = s
		return ""
	})
	field("base", func(raw json.RawMessage) string {
		s, ok := text(raw)
		if !ok || !fullSHA.MatchString(s) {
			return "not a commit's full lowercase SHA"
		}
		p.Base = s
		return ""
	})
	field("attempt", func(raw json.RawMessage) string {
		n, ok := integer(raw)
		if !ok || n < 1 {
			return "not a positive integer"
		}
		p.Attempt = n
		return ""
	})
	field("outcome", func(raw json.RawMessage) string {
		s, ok := text(raw)
		if !ok || !slices.Contains(outcomes, s) {
			return "not done, blocked, needs_decision or spec_defect"
		}
		p.Outcome = s
		return ""
	})
	p.head(values, twice)
	p.notes(values, twice)
	return p
}

// head reads the head: required with done and refused with any other
// outcome, null being none; a well-formed one is kept whatever the
// outcome, as a rotated record writes it.
func (p *parsed) head(values map[string]json.RawMessage, twice []string) {
	if slices.Contains(twice, "head") {
		return
	}
	raw, given := values["head"]
	given = given && !null(raw)
	switch {
	case !given && p.Outcome == done:
		p.faults = append(p.faults, "the result has no head, which the outcome done needs")
		return
	case !given:
		p.good["head"] = true
		return
	}
	s, ok := text(raw)
	if !ok || !fullSHA.MatchString(s) {
		p.faults = append(p.faults, fmt.Sprintf("the result's head is %s, not a commit's full lowercase SHA", raw))
		return
	}
	if p.good["outcome"] && p.Outcome != done {
		p.faults = append(p.faults, fmt.Sprintf("the result gives a head with the outcome %s, which claims none", p.Outcome))
		return
	}
	p.Head = &s
	p.good["head"] = true
}

// notes reads the notes: a text, or none when absent or null.
func (p *parsed) notes(values map[string]json.RawMessage, twice []string) {
	raw, given := values["notes"]
	switch {
	case slices.Contains(twice, "notes"):
		return
	case !given || null(raw):
		p.good["notes"] = true
		return
	}
	s, ok := text(raw)
	if !ok {
		p.faults = append(p.faults, fmt.Sprintf("the result's notes is %s, not a text", raw))
		return
	}
	p.Notes = &s
	p.good["notes"] = true
}

// object is the one JSON object data holds, each key's raw value, and the
// keys it gives more than once, in the order met; an error when data is
// not one object, or has anything after it.
func object(data []byte) (map[string]json.RawMessage, []string, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, nil, fmt.Errorf("it does not start with {")
	}
	values := map[string]json.RawMessage{}
	var twice []string
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, nil, err
		}
		key, _ := t.(string)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, nil, err
		}
		if _, seen := values[key]; seen && !slices.Contains(twice, key) {
			twice = append(twice, key)
		}
		values[key] = raw
	}
	if _, err := dec.Token(); err != nil {
		return nil, nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, nil, fmt.Errorf("something follows the object")
	}
	return values, twice, nil
}

// text is a JSON string's value; not ok for any other value, null among
// them.
func text(raw json.RawMessage) (string, bool) {
	var s string
	if len(raw) == 0 || raw[0] != '"' || json.Unmarshal(raw, &s) != nil {
		return "", false
	}
	return s, true
}

// integer is a JSON number written as an integer, no fraction and no
// exponent; not ok for any other value.
func integer(raw json.RawMessage) (int, bool) {
	n, err := strconv.Atoi(string(raw))
	return n, err == nil
}

// null is whether the value is JSON's null.
func null(raw json.RawMessage) bool { return string(raw) == "null" }
