// Package follow is itos follow's threads (slice 61, features/follow.feature):
// following up with a person, kept as a thread rather than a work item. A
// thread has an id, the person it is with (free text: no people file is
// read), a title, a status (open or closed) and its notes, each a dated
// entry appended and never edited. They are the person's own: one file,
// follow-ups.yaml, in itos's folder of the git common dir, where the stealth
// mode keeps its data, so git never commits it and every linked worktree of
// the clone reads the same threads. No config is read: any git repository
// will do.
//
// The package reads and writes the file and gives a thread as Markdown; the
// command line (internal/cli/follow.go) reads the clock, once a run, and
// hands each change its time, and holds the file's lock (internal/lock)
// across a change's load and save, so two itos at once keep both changes
// (bug 16).
package follow

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// FileName is the threads' file in itos's folder of the git common dir.
const FileName = "follow-ups.yaml"

// The statuses a thread has.
const (
	Open   = "open"
	Closed = "closed"
)

// Stamp is how a note's time is written in the file: local time, to the
// second, with its offset.
const Stamp = time.RFC3339

// Shown is how a time is printed: to the minute, in the reader's time zone
// (bug 16: each in the offset it was written with, notes written from two
// zones read out of order).
const Shown = "2006-01-02 15:04"

// Note is one dated entry of a thread.
type Note struct {
	At   string `yaml:"at" json:"at"`
	Text string `yaml:"text" json:"text"`
}

// Thread is a follow-up with one person. Closed is when it was closed, ""
// while open; Docs are the files follow doc wrote it to, absolute.
type Thread struct {
	ID     string   `yaml:"id" json:"id"`
	With   string   `yaml:"with" json:"with"`
	Title  string   `yaml:"title" json:"title"`
	Status string   `yaml:"status" json:"status"`
	Closed string   `yaml:"closed,omitempty" json:"closed,omitempty"`
	Notes  []Note   `yaml:"notes" json:"notes"`
	Docs   []string `yaml:"docs,omitempty" json:"docs,omitempty"`
}

// File is the threads' file: every thread, in the order they were opened.
type File struct {
	Threads []Thread `yaml:"threads"`
}

// head is the comment the file starts with.
const head = "# itos follow's threads: this clone's own, never committed (itos help follow).\n"

// idPattern is what a thread's id may be: a word typed on a command line.
var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// ValidID is whether an id may name a thread.
func ValidID(id string) bool { return idPattern.MatchString(id) }

// Load reads the threads' file at path; one not there is no threads yet.
// A file that is not as itos writes it is an error naming it, since saving
// what was read would drop the rest (bug 16): an unknown key, a thread with
// no id or two with one id, a status neither open nor closed, a second YAML
// document, or no document at all, which itos never writes (it writes
// "threads: []" for none), so an empty file is a write cut short, not a
// clone with no threads.
func Load(path string) (File, error) {
	text, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return File{}, nil
	}
	if err != nil {
		return File{}, err
	}
	var f File
	dec := yaml.NewDecoder(bytes.NewReader(text))
	dec.KnownFields(true)
	if err := dec.Decode(&f); errors.Is(err, io.EOF) {
		return File{}, fmt.Errorf("%s holds no threads, not even an empty list, so itos did not write it whole; it is left as it is: remove it if no thread was in it", path)
	} else if err != nil {
		return File{}, fmt.Errorf("%s: %w", path, err)
	}
	var more yaml.Node
	if err := dec.Decode(&more); !errors.Is(err, io.EOF) {
		return File{}, fmt.Errorf("%s holds more than one YAML document, and itos writes one, so it is left as it is: move the threads into the first", path)
	}
	seen := map[string]bool{}
	for i, t := range f.Threads {
		switch {
		case !ValidID(t.ID):
			return File{}, fmt.Errorf("%s: thread %d has no id itos can take: %q", path, i+1, t.ID)
		case seen[t.ID]:
			return File{}, fmt.Errorf("%s: two threads have the id %s", path, t.ID)
		case t.Status != Open && t.Status != Closed:
			return File{}, fmt.Errorf("%s: %s's status is %q, not open or closed", path, t.ID, t.Status)
		}
		seen[t.ID] = true
	}
	return f, nil
}

// Save writes the threads' file at path, readable by its owner alone: the
// text written to a file beside it and synced to the disk, then moved over
// it, so a reader never sees half of it and a crash leaves the old file or
// the new one, never an empty one. Its folder is made when it is not there.
func Save(path string, f File) error {
	var body bytes.Buffer
	enc := yaml.NewEncoder(&body)
	enc.SetIndent(2)
	if err := enc.Encode(f); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+FileName+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(head + body.String()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Find is the thread with the id, or nil.
func (f *File) Find(id string) *Thread {
	for i := range f.Threads {
		if f.Threads[i].ID == id {
			return &f.Threads[i]
		}
	}
	return nil
}

// Add opens a thread with its first note, at the end of the threads.
func (f *File) Add(id, with, title, note string, at time.Time) *Thread {
	f.Threads = append(f.Threads, Thread{
		ID: id, With: with, Title: title, Status: Open,
		Notes: []Note{{At: at.Format(Stamp), Text: note}},
	})
	return &f.Threads[len(f.Threads)-1]
}

// Note appends a dated entry to the thread.
func (t *Thread) Note(text string, at time.Time) {
	t.Notes = append(t.Notes, Note{At: at.Format(Stamp), Text: text})
}

// Close closes the thread, with a last note when text is not "".
func (t *Thread) Close(text string, at time.Time) {
	if text != "" {
		t.Note(text, at)
	}
	t.Status = Closed
	t.Closed = at.Format(Stamp)
}

// Wrote records a file the thread was written to, once.
func (t *Thread) Wrote(path string) {
	if !slices.Contains(t.Docs, path) {
		t.Docs = append(t.Docs, path)
	}
}

// Opened is when the thread's first note was written, "" with none.
func (t Thread) Opened() string {
	if len(t.Notes) == 0 {
		return ""
	}
	return t.Notes[0].At
}

// Last is when the thread's last note was written, "" with none.
func (t Thread) Last() string {
	if len(t.Notes) == 0 {
		return ""
	}
	return t.Notes[len(t.Notes)-1].At
}

// Show is a time as itos prints it, to the minute in the reader's time
// zone (time.Local, TZ where the system reads it); one it cannot read as it
// is written.
func Show(stamp string) string {
	at, err := time.Parse(Stamp, stamp)
	if err != nil {
		return stamp
	}
	return at.Local().Format(Shown)
}

// Markdown is the whole thread as a Markdown document: its title, who it is
// with and its status, then each note under its date, in order.
func Markdown(t Thread) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", t.Title)
	state := "open"
	if t.Status == Closed {
		state = "closed " + Show(t.Closed)
	}
	fmt.Fprintf(&b, "A thread with %s, %s (itos follow show %s).\n", t.With, state, t.ID)
	for _, n := range t.Notes {
		fmt.Fprintf(&b, "\n## %s\n\n%s\n", Show(n.At), strings.TrimRight(n.Text, "\n"))
	}
	return b.String()
}
