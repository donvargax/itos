// The config's JSON Schema, for editors: yaml-language-server, JetBrains and
// SchemaStore check an itos.yaml against it as it is written.
//
//	go run ./tools/bin/config-schema [<out file>]
//
// writes it to <out file>, or to stdout with none. It is generated, never
// kept by hand: from internal/config's schema (config.Schema), the table
// config check holds a file to, and its one table of defaults
// (config.DefaultsFor), so it says what config check says. Each key has its
// type, the values an enum allows, what is required, its words as
// `description` and its default as `default` (tests.<kind>'s under each
// kind); an object's unknown key is refused (`additionalProperties: false`),
// as config check refuses it. What the schema cannot say it leaves to config
// check: version 1 (here a number), the cross-checks (a name that must refer
// to something, a pattern that must compile, commits.since a full SHA, a step
// with exactly one of run, tests and tasks) and required_for's word, all. So
// a config the schema refuses config check refuses too, and not the reverse.
//
// A release's GoReleaser (.goreleaser.yaml's before hook) writes it as
// itos.schema.json, published beside the archives and listed in checksums.txt,
// and tools/selftest/go-schema.ts proves it.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/donvargax/itos/v5/internal/config"
	"github.com/donvargax/itos/v5/internal/value"
)

// Draft is the JSON Schema dialect: 2020-12, the one yaml-language-server,
// JetBrains and SchemaStore read.
const Draft = "https://json-schema.org/draft/2020-12/schema"

func main() {
	if len(os.Args) > 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./tools/bin/config-schema [<out file>]")
		os.Exit(2)
	}
	text, err := generate()
	if err == nil && len(os.Args) == 2 {
		err = os.WriteFile(os.Args[1], text, 0o644)
	} else if err == nil {
		_, err = os.Stdout.Write(text)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "config-schema: "+err.Error())
		os.Exit(1)
	}
}

// generate is the schema as indented JSON, its keys in the order the table
// writes them.
func generate() ([]byte, error) {
	root := config.Schema()
	defaults := config.DefaultsFor(nil, false)
	described, err := convert(root, defaults, "")
	if err != nil {
		return nil, err
	}
	doc := value.NewMap("$schema", Draft, "title", "itos.yaml")
	for _, k := range described.Keys() {
		doc.Set(k, described.At(k))
	}
	var out bytes.Buffer
	if err := json.Indent(&out, []byte(value.JSON(doc)), "", "  "); err != nil {
		return nil, err
	}
	out.WriteByte('\n')
	return out.Bytes(), nil
}

// convert is one node as a JSON Schema, with the defaults the table gives at
// its path (value.Undefined for none). A default the schema has no key for is
// an error: the two tables have drifted apart.
func convert(n *config.Node, defaults any, path string) (*value.Map, error) {
	s := value.NewMap()
	if n.Description != "" {
		s.Set("description", n.Description)
	}
	at := func(k string) string { return strings.TrimPrefix(path+"."+k, ".") }
	switch n.Kind {
	case "string", "number", "boolean":
		s.Set("type", n.Kind)
	case "strings":
		s.Set("type", "array")
		s.Set("items", value.NewMap("type", "string"))
	case "enum":
		s.Set("type", "string")
		s.Set("enum", strings2any(n.Enum))
	case "either":
		var shapes []any
		for _, e := range n.Either {
			shape, err := convert(e, value.Undefined, path)
			if err != nil {
				return nil, err
			}
			shapes = append(shapes, shape)
		}
		s.Set("anyOf", shapes)
	case "list":
		item, err := convert(n.Of, value.Undefined, path+"[]")
		if err != nil {
			return nil, err
		}
		s.Set("type", "array")
		s.Set("items", item)
	case "map":
		// A map's defaults apply to each of its entries: the table holds them
		// under config.KindKey (tests.<kind>).
		var each any = value.Undefined
		if d, ok := defaults.(*value.Map); ok {
			for _, k := range d.Keys() {
				if k != config.KindKey {
					return nil, fmt.Errorf("the defaults have %s, which the schema has no key for", at(k))
				}
			}
			each = d.At(config.KindKey)
		}
		entry, err := convert(n.Of, each, at(config.KindKey))
		if err != nil {
			return nil, err
		}
		s.Set("type", "object")
		s.Set("additionalProperties", entry)
		return s, nil
	case "object":
		d, _ := defaults.(*value.Map)
		properties := value.NewMap()
		known := map[string]bool{}
		for _, f := range n.Fields {
			known[f.Key] = true
			var under any = value.Undefined
			if d != nil {
				under = d.At(f.Key)
			}
			property, err := convert(f.Node, under, at(f.Key))
			if err != nil {
				return nil, err
			}
			properties.Set(f.Key, property)
		}
		if d != nil {
			for _, k := range d.Keys() {
				if !known[k] {
					return nil, fmt.Errorf("the defaults have %s, which the schema has no key for", at(k))
				}
			}
		}
		s.Set("type", "object")
		s.Set("properties", properties)
		if len(n.Required) > 0 {
			s.Set("required", strings2any(n.Required))
		}
		s.Set("additionalProperties", false)
		return s, nil
	default:
		return nil, fmt.Errorf("%s is of a kind the generator does not know: %q", path, n.Kind)
	}
	// A value with no keys below it takes its default whole.
	if defaults != value.Undefined {
		if _, nested := defaults.(*value.Map); nested {
			return nil, fmt.Errorf("the defaults have a mapping at %s, which the schema gives no keys", path)
		}
		s.Set("default", defaults)
	}
	return s, nil
}

func strings2any(list []string) []any {
	out := make([]any, len(list))
	for i, s := range list {
		out[i] = s
	}
	return out
}
