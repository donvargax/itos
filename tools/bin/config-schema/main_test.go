package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/donvargax/itos/v7/internal/config"
)

// A key renamed declares the key it was on its property, and the keys
// removed are the top's list (slice 113); a schema declaring neither has
// neither.
func TestGenerateDeclarations(t *testing.T) {
	root := config.Schema()
	plain, err := generateFrom(root)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plain), "x-itos-") {
		t.Errorf("a schema declaring nothing declares something:\n%s", plain)
	}

	declared := *root
	declared.Removed = []string{"ci.watch.timeout"}
	declared.Fields = append([]config.Field{}, root.Fields...)
	for i, f := range declared.Fields {
		if f.Key == "requires" {
			renamed := *f.Node
			renamed.RenamedFrom = "needs"
			declared.Fields[i].Node = &renamed
		}
	}
	text, err := generateFrom(&declared)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Removed    []string `json:"x-itos-removed"`
		Properties map[string]struct {
			RenamedFrom string `json:"x-itos-renamed-from"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(text, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Removed) != 1 || doc.Removed[0] != "ci.watch.timeout" {
		t.Errorf("x-itos-removed is %v", doc.Removed)
	}
	if got := doc.Properties["requires"].RenamedFrom; got != "needs" {
		t.Errorf("requires is renamed from %q", got)
	}
	if got := doc.Properties["version"].RenamedFrom; got != "" {
		t.Errorf("version is renamed from %q", got)
	}
}
