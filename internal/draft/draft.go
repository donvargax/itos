package draft

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

	"go.yaml.in/yaml/v3"

	"github.com/donvargax/itos/v7/internal/kind"
)

// Folder is the drafts' folder in itos's folder of the git common dir.
const Folder = "drafts"

// FileName is the list of drafts in Folder.
const FileName = "drafts.yaml"

// PatchSuffix is what a change's patch file adds to its id.
const PatchSuffix = ".patch"

// Edits is the folder, beside the list, of the edit drafts' copies: one
// folder per draft, named by its id, holding a copy of each of its paths.
// No id or patch file can be named edits, so it collides with neither.
const Edits = "edits"

// Draft is one pending change. A change has Message and Paths, the paths
// it was drafted from as the repository's top names them, and either its
// patch in <id>.patch beside the list or, for an edit draft, Base, the
// commit its copies were taken from, the copies in edits/<id>/; a command
// has Command, the itos arguments to run, never through a shell.
type Draft struct {
	ID      string   `yaml:"id" json:"id"`
	Message string   `yaml:"message,omitempty" json:"message,omitempty"`
	Paths   []string `yaml:"paths,omitempty" json:"paths,omitempty"`
	Base    string   `yaml:"base,omitempty" json:"base,omitempty"`
	Command []string `yaml:"command,omitempty" json:"command,omitempty"`
}

// IsChange is whether the draft is a change to files, not a command line.
func (d Draft) IsChange() bool { return len(d.Command) == 0 }

// IsEdit is whether the draft is a change kept as copies of its files to
// edit (itos draft edit), not as a patch.
func (d Draft) IsEdit() bool { return d.IsChange() && d.Base != "" }

// Header is a change's message's first line.
func (d Draft) Header() string {
	header, _, _ := strings.Cut(strings.TrimSpace(d.Message), "\n")
	return strings.TrimSpace(header)
}

// Line is a command draft as a person would type it: itos and its
// arguments, each quoted for a POSIX shell when it needs to be.
func (d Draft) Line() string {
	words := []string{"itos"}
	for _, a := range d.Command {
		words = append(words, Quote(a))
	}
	return strings.Join(words, " ")
}

// plain is a word a shell reads as itself.
var plain = regexp.MustCompile(`^[A-Za-z0-9@%+=:,./_-]+$`)

// Quote is the word as a POSIX shell reads it back: itself when it holds
// nothing the shell would read, else in single quotes.
func Quote(word string) string {
	if plain.MatchString(word) {
		return word
	}
	return "'" + strings.ReplaceAll(word, "'", `'\''`) + "'"
}

// File is the list of drafts, in the order they were added.
type File struct {
	Drafts []Draft `yaml:"drafts"`
}

// head is the comment the file starts with.
const head = "# itos draft's pending changes: this clone's own, never committed (itos help draft).\n"

// idPattern is what a draft's id may be: a word typed on a command line,
// and a file's name.
var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// ValidID is whether an id may name a draft.
func ValidID(id string) bool { return idPattern.MatchString(id) }

// Patch is the file of a change's patch, in the folder of the list at path.
func Patch(path, id string) string {
	return filepath.Join(filepath.Dir(path), id+PatchSuffix)
}

// Copies is the folder of an edit draft's copies, in the folder of the list
// at path: edits/<id>, a copy of each path at the path within it.
func Copies(path, id string) string {
	return filepath.Join(filepath.Dir(path), Edits, id)
}

// Copy is an edit draft's copy of the path, as the repository's top names
// it, in the folder of the list at path.
func Copy(path, id, name string) string {
	return filepath.Join(Copies(path, id), filepath.FromSlash(name))
}

// Load reads the list at path; one not there is no drafts yet. A list that
// is not as itos writes it is an error naming it, kind.Usage (exit 2),
// since saving what was read would drop the rest: an unknown key, a draft
// with no id or two with one id, one that is neither a change with its
// message and paths nor a command, a second YAML document, or none at all.
func Load(path string) (File, error) {
	text, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return File{}, nil
	}
	if err != nil {
		return File{}, err
	}
	f, err := parse(path, text)
	return f, kind.Wrap(kind.Usage, err)
}

func parse(path string, text []byte) (File, error) {
	var f File
	dec := yaml.NewDecoder(bytes.NewReader(text))
	dec.KnownFields(true)
	if err := dec.Decode(&f); errors.Is(err, io.EOF) {
		return File{}, fmt.Errorf("%s holds no drafts, not even an empty list, so itos did not write it whole; it is left as it is: remove it if no draft was in it", path)
	} else if err != nil {
		return File{}, fmt.Errorf("%s: %w", path, err)
	}
	var more yaml.Node
	if err := dec.Decode(&more); !errors.Is(err, io.EOF) {
		return File{}, fmt.Errorf("%s holds more than one YAML document, and itos writes one, so it is left as it is: move the drafts into the first", path)
	}
	seen := map[string]bool{}
	for i, d := range f.Drafts {
		switch {
		case !ValidID(d.ID):
			return File{}, fmt.Errorf("%s: draft %d has no id itos can take: %q", path, i+1, d.ID)
		case seen[d.ID]:
			return File{}, fmt.Errorf("%s: two drafts have the id %s", path, d.ID)
		case len(d.Command) > 0 && (d.Message != "" || len(d.Paths) > 0 || d.Base != ""):
			return File{}, fmt.Errorf("%s: %s has a command and a change; a draft is one or the other", path, d.ID)
		case len(d.Command) == 0 && (strings.TrimSpace(d.Message) == "" || len(d.Paths) == 0):
			return File{}, fmt.Errorf("%s: %s is neither a command nor a change with its message and paths", path, d.ID)
		}
		seen[d.ID] = true
	}
	return f, nil
}

// Save writes the list at path, readable by its owner alone, as
// follow.Save writes the threads: to a file beside it, synced, then moved
// over it, so a reader never sees half of it. Its folder is made when it is
// not there.
func Save(path string, f File) error {
	if f.Drafts == nil {
		f.Drafts = []Draft{}
	}
	var body bytes.Buffer
	enc := yaml.NewEncoder(&body)
	enc.SetIndent(2)
	if err := enc.Encode(f); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return WriteFile(path, append([]byte(head), body.Bytes()...))
}

// WriteFile writes data at path, its owner's alone (0600, the folder made
// 0700), through a file beside it moved over it.
func WriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
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

// Find is the draft with the id, or nil.
func (f *File) Find(id string) *Draft {
	for i := range f.Drafts {
		if f.Drafts[i].ID == id {
			return &f.Drafts[i]
		}
	}
	return nil
}

// Remove takes the draft with the id out of the list.
func (f *File) Remove(id string) {
	f.Drafts = slices.DeleteFunc(f.Drafts, func(d Draft) bool { return d.ID == id })
}
