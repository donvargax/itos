package upgrade

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/donvargax/itos/v8/internal/value"
)

const (
	workflowsDir = ".github/workflows"
	actionPrefix = "donvargax/itos@"
)

// actionUse is a step's uses: of the itos action, as it is written: the
// workflow File, slash-separated, the one-based Line of its uses: key, its
// Ref after the @ and the step's with.version, when it has one.
type actionUse struct {
	File         string  `json:"file"`
	Line         int     `json:"line"`
	Ref          string  `json:"ref"`
	VersionInput *string `json:"version_input,omitempty"`
}

// scanProblem is a workflow upgrade could not read, and why: its uses: are
// not named.
type scanProblem struct {
	File    string `json:"file"`
	Message string `json:"message"`
}

// scanWorkflows names every step of the workflows directly in
// .github/workflows of fsys, the files named *.yml or *.yaml, each read as
// YAML, whose uses: is donvargax/itos@<ref> (slice 150): sorted by file,
// then line. It reads only; a file it cannot read or parse is a problem and
// the other files are still scanned.
func scanWorkflows(fsys fs.FS) ([]actionUse, []scanProblem) {
	uses, problems := []actionUse{}, []scanProblem{}
	entries, err := fs.ReadDir(fsys, workflowsDir)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			problems = append(problems, scanProblem{workflowsDir, "cannot be listed: " + why(err)})
		}
		return uses, problems
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !(strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")) {
			continue
		}
		file := workflowsDir + "/" + name
		text, err := fs.ReadFile(fsys, file)
		if err != nil {
			problems = append(problems, scanProblem{file, "cannot be read: " + why(err)})
			continue
		}
		found, err := stepUses(file, text)
		if err != nil {
			problems = append(problems, scanProblem{file, "cannot be read as YAML: " + err.Error()})
			continue
		}
		uses = append(uses, found...)
	}
	return uses, problems
}

// why is err without the path it names, which the problem's file holds.
func why(err error) string {
	var failed *fs.PathError
	if errors.As(err, &failed) {
		return failed.Err.Error()
	}
	return err.Error()
}

// stepUses is the uses of the itos action in the steps of text's jobs, by
// line. Text that value.Parse refuses (a syntax error, a key written twice,
// several documents, aliases that expand without bound) is an error, so no
// report is made of half a workflow.
func stepUses(file string, text []byte) ([]actionUse, error) {
	if _, err := value.Parse(string(text)); err != nil {
		return nil, err
	}
	var doc yaml.Node
	_ = yaml.Unmarshal(text, &doc) // value.Parse read it without error
	var uses []actionUse
	for _, job := range values(entry(&doc, "jobs")) {
		for _, step := range items(entry(job, "steps")) {
			key, value := keyed(step, "uses")
			if value == nil || value.Kind != yaml.ScalarNode {
				continue
			}
			ref, ok := strings.CutPrefix(value.Value, actionPrefix)
			if !ok || ref == "" {
				continue
			}
			u := actionUse{File: file, Line: key.Line, Ref: ref}
			if v := entry(entry(step, "with"), "version"); v != nil && v.Kind == yaml.ScalarNode {
				u.VersionInput = &v.Value
			}
			uses = append(uses, u)
		}
	}
	slices.SortStableFunc(uses, func(a, b actionUse) int { return a.Line - b.Line })
	return uses, nil
}

// resolved is n, the node an alias stands for when it is one.
func resolved(n *yaml.Node) *yaml.Node {
	for n != nil && n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	if n != nil && n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		return resolved(n.Content[0])
	}
	return n
}

// keyed is the key node and the value node of key in the mapping n, nil for
// both when n is no mapping or has no such key.
func keyed(n *yaml.Node, key string) (*yaml.Node, *yaml.Node) {
	n = resolved(n)
	if n == nil || n.Kind != yaml.MappingNode {
		return nil, nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i], resolved(n.Content[i+1])
		}
	}
	return nil, nil
}

func entry(n *yaml.Node, key string) *yaml.Node {
	_, v := keyed(n, key)
	return v
}

// values are the values of the mapping n, none for anything else.
func values(n *yaml.Node) []*yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	var all []*yaml.Node
	for i := 1; i < len(n.Content); i += 2 {
		all = append(all, n.Content[i])
	}
	return all
}

// items are the items of the sequence n, none for anything else.
func items(n *yaml.Node) []*yaml.Node {
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	return n.Content
}

// line is the use told to a person: where it is, then the action as written,
// then its version input.
func (u actionUse) line() string {
	s := fmt.Sprintf("%s:%d: %s%s", u.File, u.Line, actionPrefix, u.Ref)
	switch {
	case u.VersionInput == nil:
	case *u.VersionInput == "":
		s += ` (version: "")`
	default:
		s += " (version: " + *u.VersionInput + ")"
	}
	return s
}
