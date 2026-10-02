// Package tests is named tests behind an adapter (tools/itos/tests.ts,
// gherkin.ts, smoke.ts and smoke-rule.ts): which tests a kind has and which
// are live, as the adapter protocol's list, and the kind's smoke set with its
// rule. The built-in Gherkin adapter is the only reader of a feature file.
//
// The port has what config check needs so far: the Gherkin adapter over the
// working tree, the smoke set's loader and its rule's problems. A command
// adapter, another tree, the run templates and recognition are later groups'
// (PLAN.md, phase 2, step 4).
package tests

import (
	"errors"

	"github.com/donvargax/itos/internal/config"
)

// Test is one named test: its ID without the tag prefix, its file relative to
// the kind's root (the smoke rule's unit), and whether it is live.
type Test struct {
	ID   string `json:"id"`
	File string `json:"file"`
	Live bool   `json:"live"`
}

// List is the adapter protocol's answer: the tests, and every file, the ones
// without a test included.
type List struct {
	Protocol int      `json:"protocol"`
	Tests    []Test   `json:"tests"`
	Files    []string `json:"files"`
}

// ErrCommandAdapter is a kind whose adapter is a command: the named tests'
// group of the port has not reached it.
var ErrCommandAdapter = errors.New("a command adapter")

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

// ListTests is the kind's tests in the working tree.
func ListTests(cfg *config.Loaded, name string) (List, error) {
	k, err := KindOf(cfg, name)
	if err != nil {
		return List{}, err
	}
	switch {
	case k.Adapter.Command != "" || k.Adapter.Name == "":
		return List{}, ErrCommandAdapter
	case k.Adapter.Name != "gherkin":
		return List{}, config.Invalid(cfg.Path, "tests."+name+".adapter "+k.Adapter.Name+" is not built in")
	}
	options, err := gherkinOptions(cfg, name, k)
	if err != nil {
		return List{}, err
	}
	return gherkinList(options)
}

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
