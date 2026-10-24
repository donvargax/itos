package proof

import (
	"bytes"
	"encoding/json"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// maxCount is the largest count the envelope carries: the largest integer a
// JSON number holds exactly in a double.
const maxCount = 9007199254740991

var (
	countToken = regexp.MustCompile(`^(?:0|[1-9][0-9]*)$`)
	unitToken  = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)
)

// Summary is what a passing provider reported of the work it judged, as the
// envelope names it: the range-wide counts and the unit they count, each
// null when the provider did not report it (never zero), and nothing else of
// what the provider printed.
type Summary struct {
	Version   int     `json:"version"`
	Judged    *uint64 `json:"judged"`
	Unit      *string `json:"unit"`
	Files     *uint64 `json:"files"`
	Killed    *uint64 `json:"killed"`
	Survived  *uint64 `json:"survived"`
	Excepted  *uint64 `json:"excepted"`
	Uncovered *uint64 `json:"uncovered"`
}

// FieldError is the field of an opted-in envelope that breaks its contract,
// named as the provider wrote it (itos_proof.judged), and why.
type FieldError struct {
	Field, Reason string
}

// Message is the sentence that names the field first, then what it must be.
func (e *FieldError) Message() string {
	return e.Field + " " + e.Reason
}

// ReadSummary reads the optional itos_proof envelope of an exit-zero
// provider's stdout. Stdout opts in when it is one complete JSON object with
// a top-level itos_proof member; anything else, nested or cased otherwise
// included, is no summary, nil with no error. An opted-in envelope that
// breaks the contract (docs/rule-ids.md, ci-code-proof-summary) gives the
// field. Names match exactly; a duplicate itos_proof or recognized key is an
// error, a duplicate unknown key is ignored with its value.
func ReadSummary(stdout []byte) (*Summary, *FieldError) {
	if !json.Valid(stdout) {
		return nil, nil
	}
	top, dup := members(stdout, "itos_proof")
	switch {
	case top == nil:
		return nil, nil
	case dup != "":
		return nil, &FieldError{dup, "must appear once"}
	}
	raw, ok := top["itos_proof"]
	if !ok {
		return nil, nil
	}
	return readEnvelope(raw)
}

// members are the members of a JSON object by name, only the names asked
// for, each raw as written; nil for a JSON value that is not an object. dup
// is the first of the names asked for that appears twice, "" for none.
func members(text []byte, names ...string) (found map[string]json.RawMessage, dup string) {
	dec := json.NewDecoder(bytes.NewReader(text))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, ""
	}
	found = map[string]json.RawMessage{}
	for dec.More() {
		key, _ := dec.Token()
		var value json.RawMessage
		_ = dec.Decode(&value)
		name, _ := key.(string)
		if !slices.Contains(names, name) {
			continue
		}
		if _, seen := found[name]; seen && dup == "" {
			dup = name
		}
		found[name] = value
	}
	return found, dup
}

var fields = []string{"version", "judged", "unit", "files", "killed", "survived", "excepted", "uncovered"}

func readEnvelope(raw json.RawMessage) (*Summary, *FieldError) {
	m, dup := members(raw, fields...)
	if m == nil {
		return nil, &FieldError{"itos_proof", "must be a JSON object"}
	}
	if dup != "" {
		return nil, &FieldError{"itos_proof." + dup, "must appear once"}
	}
	if string(m["version"]) != "1" {
		return nil, &FieldError{"itos_proof.version", "must be the integer 1"}
	}
	s := &Summary{Version: 1}
	counts := map[string]**uint64{"judged": &s.Judged, "files": &s.Files, "killed": &s.Killed,
		"survived": &s.Survived, "excepted": &s.Excepted, "uncovered": &s.Uncovered}
	for _, name := range fields[1:] {
		if name == "unit" {
			if err := readUnit(m["unit"], s); err != nil {
				return nil, err
			}
			continue
		}
		n, ok := readCount(m[name])
		if !ok {
			return nil, &FieldError{"itos_proof." + name, "must be an unsigned integer up to 9007199254740991, or null"}
		}
		*counts[name] = n
	}
	if s.Judged != nil && s.Unit == nil {
		return nil, &FieldError{"itos_proof.unit", "must name what judged counts when judged is given"}
	}
	return s, nil
}

// readCount is the count a member holds: nil for an absent or null one, ok
// false for anything but an unsigned integer token within maxCount.
func readCount(raw json.RawMessage) (n *uint64, ok bool) {
	text := string(raw)
	if text == "" || text == "null" {
		return nil, true
	}
	if !countToken.MatchString(text) {
		return nil, false
	}
	v, err := strconv.ParseUint(text, 10, 64)
	if err != nil || v > maxCount {
		return nil, false
	}
	return &v, true
}

// readUnit sets the summary's unit from a member that is absent, null or a
// string of the unit's shape.
func readUnit(raw json.RawMessage, s *Summary) *FieldError {
	if text := string(raw); text == "" || text == "null" {
		return nil
	}
	var unit string
	if json.Unmarshal(raw, &unit) != nil || !unitToken.MatchString(unit) {
		return &FieldError{"itos_proof.unit", "must be a name of ASCII letters, digits, _ and -, starting with a letter, of at most 64 characters, or null"}
	}
	s.Unit = &unit
	return nil
}

// Line is the one line the run's log gives for the provider's summary: its
// reported fields as label=value, in the order judged, unit, files, killed,
// survived, excepted, uncovered, or, with none (a nil summary included), that
// the provider reported none.
func (s *Summary) Line() string {
	var parts []string
	if s != nil {
		count := func(label string, n *uint64) {
			if n != nil {
				parts = append(parts, label+"="+strconv.FormatUint(*n, 10))
			}
		}
		count("judged", s.Judged)
		if s.Unit != nil {
			parts = append(parts, "unit="+*s.Unit)
		}
		count("files", s.Files)
		count("killed", s.Killed)
		count("survived", s.Survived)
		count("excepted", s.Excepted)
		count("uncovered", s.Uncovered)
	}
	if len(parts) == 0 {
		return "Code proof summary unavailable: provider did not report a summary."
	}
	return "Code proof summary: " + strings.Join(parts, "; ")
}
