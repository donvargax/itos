// Package ask is the questions of itos question (slice 62, features/ask.feature): the
// questions waiting on the person a repository's work is for, each with an
// id, q-1, q-2 and so on, never reused, the text asked, the registry item it
// holds up when it names one, and its answer once given. They are public and
// committed, unlike itos followup's threads: one file, asks.yaml, beside the
// work registry (work.asks), written whole by itos, which commits it alone
// as the registry's commands commit theirs (internal/cli/ask.go).
//
// An answered question is a decision, and itos question record (slice 69) writes
// it as an architecture decision record (internal/adr), noting the record's
// number on the question as its decision, or none for an answer that
// concerned its item alone; one answered with no decision is recorded
// nowhere yet, and itos question names it.
//
// The file is itos's own, so it is read with typed structs and written whole,
// each text as a double-quoted scalar, JSON's escapes being YAML's, so what
// it holds reads back exactly and no formatter has a reason to change it.
package ask

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/donvargax/itos/v5/internal/kind"
)

// FileName is the questions' file, beside the work registry by default.
const FileName = "asks.yaml"

// The statuses a question has: open until answered.
const (
	Open     = "open"
	Answered = "answered"
)

// Question is one question: its id, the item it holds up ("" for none), its
// text, its answer ("" while open) and its decision: the number of the
// decision record its answer was written as, None when it was marked as
// recorded nowhere, "" while it is neither.
type Question struct {
	ID       string `yaml:"id"`
	Item     string `yaml:"item,omitempty"`
	Question string `yaml:"question"`
	Answer   string `yaml:"answer,omitempty"`
	Decision string `yaml:"decision,omitempty"`
}

// None is the decision of an answer that concerned its item alone, so it is
// recorded nowhere and itos question stops naming it.
const None = "none"

// Unrecorded is whether the question is answered and its answer recorded
// nowhere: no decision record, and not marked None.
func (q Question) Unrecorded() bool { return q.Answer != "" && q.Decision == "" }

// Status is the question's status: open, or answered.
func (q Question) Status() string {
	if q.Answer == "" {
		return Open
	}
	return Answered
}

// File is the questions' file: every question, in the order asked.
type File struct {
	Questions []Question `yaml:"questions"`
}

// head is the comment the file starts with.
const head = "# The questions to the person the work is for, written by itos question (itos help question).\n"

// idPattern is what a question's id is: q- and a number, 1 or more.
var idPattern = regexp.MustCompile(`^q-[1-9][0-9]*$`)

// decisionPattern is what a decision is: a record's number, 1 or more, or
// None.
var decisionPattern = regexp.MustCompile(`^(?:[1-9][0-9]*|none)$`)

// ValidID is whether an id may name a question.
func ValidID(id string) bool { return idPattern.MatchString(id) }

// Parse reads the questions' file's text, at path for what it reports. A
// file that is not as itos writes it is an error naming it, since writing
// what was read would drop the rest: an unknown key, a question with no id
// itos gives, two with one id or one with no text, a decision that is
// neither a record's number nor none, or one on a question not answered, a
// second YAML document, or no document at all (itos writes "questions: []"
// for none). Each is a data file itos refuses, kind.Usage (exit 2).
func Parse(path, text string) (File, error) {
	f, err := parse(path, text)
	return f, kind.Wrap(kind.Usage, err)
}

func parse(path, text string) (File, error) {
	var f File
	dec := yaml.NewDecoder(strings.NewReader(text))
	dec.KnownFields(true)
	if err := dec.Decode(&f); errors.Is(err, io.EOF) {
		return File{}, fmt.Errorf("%s holds no questions, not even an empty list, so itos did not write it whole; it is left as it is", path)
	} else if err != nil {
		return File{}, fmt.Errorf("%s: %w", path, err)
	}
	var more yaml.Node
	if err := dec.Decode(&more); !errors.Is(err, io.EOF) {
		return File{}, fmt.Errorf("%s holds more than one YAML document, and itos writes one, so it is left as it is: move the questions into the first", path)
	}
	seen := map[string]bool{}
	for i, q := range f.Questions {
		switch {
		case !ValidID(q.ID):
			return File{}, fmt.Errorf("%s: question %d's id is %q, not q-<n>", path, i+1, q.ID)
		case seen[q.ID]:
			return File{}, fmt.Errorf("%s: two questions have the id %s", path, q.ID)
		case strings.TrimSpace(q.Question) == "":
			return File{}, fmt.Errorf("%s: %s asks nothing", path, q.ID)
		case q.Decision != "" && !decisionPattern.MatchString(q.Decision):
			return File{}, fmt.Errorf("%s: %s's decision is %q, neither a decision record's number nor none", path, q.ID, q.Decision)
		case q.Decision != "" && q.Answer == "":
			return File{}, fmt.Errorf("%s: %s has a decision and no answer", path, q.ID)
		}
		seen[q.ID] = true
	}
	return f, nil
}

// Text is the file as itos writes it: its head comment, then each question
// a block mapping, its texts double-quoted, its decision, a number or none,
// plain.
func Text(f File) string {
	var b strings.Builder
	b.WriteString(head)
	if len(f.Questions) == 0 {
		b.WriteString("questions: []\n")
		return b.String()
	}
	b.WriteString("questions:\n")
	for _, q := range f.Questions {
		fmt.Fprintf(&b, "  - id: %s\n", q.ID)
		if q.Item != "" {
			fmt.Fprintf(&b, "    item: %s\n", quoted(q.Item))
		}
		fmt.Fprintf(&b, "    question: %s\n", quoted(q.Question))
		if q.Answer != "" {
			fmt.Fprintf(&b, "    answer: %s\n", quoted(q.Answer))
		}
		if q.Decision != "" {
			fmt.Fprintf(&b, "    decision: %s\n", q.Decision)
		}
	}
	return b.String()
}

// quoted is s as a double-quoted scalar: JSON's string, whose escapes YAML
// reads the same, with no HTML escaping.
func quoted(s string) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	// A string always encodes.
	_ = enc.Encode(s)
	return strings.TrimSuffix(b.String(), "\n")
}

// Find is the question with the id, or nil.
func (f *File) Find(id string) *Question {
	for i := range f.Questions {
		if f.Questions[i].ID == id {
			return &f.Questions[i]
		}
	}
	return nil
}

// NextID is the id the next question gets: one past the highest the file
// holds, answered or not, so an id is never given twice.
func (f *File) NextID() string {
	highest := 0
	for _, q := range f.Questions {
		if n, err := strconv.Atoi(strings.TrimPrefix(q.ID, "q-")); err == nil {
			highest = max(highest, n)
		}
	}
	return "q-" + strconv.Itoa(highest+1)
}

// Add asks a question, open, at the end of the file, with the next id.
func (f *File) Add(text, item string) *Question {
	f.Questions = append(f.Questions, Question{ID: f.NextID(), Item: item, Question: text})
	return &f.Questions[len(f.Questions)-1]
}

// About are the questions that name the item, in the order asked.
func (f *File) About(item string) []Question {
	var found []Question
	for _, q := range f.Questions {
		if q.Item == item {
			found = append(found, q)
		}
	}
	return found
}
