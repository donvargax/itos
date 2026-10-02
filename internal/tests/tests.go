// Package tests is named tests behind an adapter (tools/itos/tests.ts,
// gherkin.ts, smoke.ts and smoke-rule.ts): which tests a kind has at a tree
// (the working tree, the index or a commit) and which are live, as the
// adapter protocol's list, and the kind's smoke set with its rule. The
// built-in Gherkin adapter is the only reader of a feature file; a command
// adapter is `<command> list --at <tree>`, held to the protocol.
//
// The kind's run templates turn selections into one command (run.go), and
// its recognize templates read a task check back as a selection
// (recognize.go), for CI's plan's one merged run.
package tests

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/donvargax/itos/internal/config"
	"github.com/donvargax/itos/internal/shell"
	"github.com/donvargax/itos/internal/value"
)

// Test is one named test: its ID without the tag prefix, its file relative to
// the kind's root (the smoke rule's unit), and whether it is live.
type Test struct {
	ID   string `json:"id"`
	File string `json:"file"`
	Live bool   `json:"live"`
}

// List is the adapter protocol's answer: the tests, and every file, the ones
// without a test included. Raw is a command adapter's object as it printed
// it, keys the protocol does not have included, which `tests list --json`
// prints as the TypeScript does; nil for the built-in adapter.
type List struct {
	Protocol int      `json:"protocol"`
	Tests    []Test   `json:"tests"`
	Files    []string `json:"files"`
	Raw      any      `json:"-"`
}

// Warnings is where a listing's warning goes: a command adapter that cannot
// list a tree, read as the working tree. stderr, as console.warn's.
var Warnings io.Writer = os.Stderr

// warned are the kinds whose warning this run has printed: one per run.
var warned = map[string]bool{}

// KindOf is the kind the config names, its defaults under it.
func KindOf(cfg *config.Loaded, name string) (config.Kind, error) {
	k, ok := cfg.Tests.Get(name)
	if !ok {
		return k, config.Invalid(cfg.Path, "tests."+name+" is missing")
	}
	return k, nil
}

// BareID is an ID as the kind's lists hold it: without its tag prefix.
func BareID(k config.Kind, id string) string {
	if p := k.TagPrefix; p != "" && len(id) >= len(p) && id[:len(p)] == p {
		return id[len(p):]
	}
	return id
}

// ListTests is the kind's tests at a tree: "worktree", "index" or a commit.
func ListTests(cfg *config.Loaded, name, at string) (List, error) {
	return ListTestsUnder(cfg, name, at, nil)
}

// ListTestsUnder is ListTests with root, when not nil, standing in for a
// Gherkin kind's own (`tests smoke check --features <dir>`: a copy of its
// files). A command adapter lists what it lists.
func ListTestsUnder(cfg *config.Loaded, name, at string, root *string) (List, error) {
	k, err := KindOf(cfg, name)
	if err != nil {
		return List{}, err
	}
	switch {
	case k.Adapter.Command != "":
		tree := at
		if k.Adapter.SupportsAt != nil && !*k.Adapter.SupportsAt && tree != "worktree" {
			if !warned[name] {
				fmt.Fprintf(Warnings, "tests.%s cannot list a tree; %s is read as the working tree\n", name, tree)
			}
			warned[name] = true
			tree = "worktree"
		}
		return commandList(cfg, name, k.Adapter.Command, tree)
	case k.Adapter.Name != "gherkin":
		return List{}, config.Invalid(cfg.Path, "tests."+name+".adapter "+k.Adapter.Name+" is not built in")
	}
	if root != nil {
		k.Root = root
	}
	options, err := gherkinOptions(cfg, name, k)
	if err != nil {
		return List{}, err
	}
	return gherkinList(options, at)
}

// commandList is a command adapter's list, held to the protocol: a non-zero
// exit or output that is not the protocol fails whatever needed the list,
// with the adapter's stderr.
func commandList(cfg *config.Loaded, name, command, at string) (List, error) {
	var stdout, stderr bytes.Buffer
	run := shell.Run(cfg, command+" list --at "+ShellWord(at), shell.Options{Stdout: &stdout, Stderr: &stderr})
	fail := func(why string) (List, error) {
		text := fmt.Sprintf("tests.%s: `%s list --at %s` %s\n%s", name, command, at, why, stderr.String())
		return List{}, fmt.Errorf("%s", strings.TrimRightFunc(text, isTrimmed))
	}
	if !run.OK() {
		return fail("exited " + run.Status())
	}
	raw, err := value.ParseJSON(stdout.String())
	if err != nil {
		return fail("did not print JSON")
	}
	list, why := protocolList(raw)
	if why != "" {
		return fail(why)
	}
	return list, nil
}

// protocolList is the adapter's answer as a List, or what keeps it from
// being the protocol's.
func protocolList(raw any) (List, string) {
	if _, isList := raw.([]any); !isList && !value.IsMapping(raw) {
		return List{}, "printed no object"
	}
	if p := value.Prop(raw, "protocol"); p != 1.0 {
		said := "undefined"
		if p != value.Undefined {
			said = value.JSON(p)
		}
		return List{}, "speaks protocol " + said + ", not 1"
	}
	list := List{Protocol: 1, Tests: []Test{}, Files: []string{}}
	files, ok := value.Prop(raw, "files").([]any)
	if !ok {
		return List{}, "printed no list of files"
	}
	for _, f := range files {
		file, ok := f.(string)
		if !ok {
			return List{}, "printed no list of files"
		}
		list.Files = append(list.Files, file)
	}
	tests, ok := value.Prop(raw, "tests").([]any)
	if !ok {
		return List{}, "printed no list of tests"
	}
	for _, t := range tests {
		id, idOK := value.Prop(t, "id").(string)
		file, fileOK := value.Prop(t, "file").(string)
		live, liveOK := value.Prop(t, "live").(bool)
		if !value.IsMapping(t) || !idOK || !fileOK || !liveOK {
			return List{}, "printed a test that is not { id, file, live }: " + value.JSON(t)
		}
		list.Tests = append(list.Tests, Test{ID: id, File: file, Live: live})
	}
	list.Raw = raw
	return list, ""
}

// ShellWord is a string as one shell word.
func ShellWord(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// gherkinOptions are a Gherkin kind's options: its root and ID pattern, which
// have no default and are a config problem when left out, and its tag prefix
// and wip tag.
func gherkinOptions(cfg *config.Loaded, name string, k config.Kind) (Options, error) {
	if k.Root == nil {
		return Options{}, config.Invalid(cfg.Path, "tests."+name+".root is missing")
	}
	if k.ID == nil {
		return Options{}, config.Invalid(cfg.Path, "tests."+name+".id is missing")
	}
	return Options{Root: *k.Root, ID: *k.ID, TagPrefix: k.TagPrefix, WipTag: k.WipTag}, nil
}
