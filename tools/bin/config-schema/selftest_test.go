// The config schema's self-test (T-054, a Go test since T-143): the schema a Go release
// publishes beside its archives, itos.schema.json, which an editor checks an itos.yaml against as
// it is written, says what config check says.
//
// It generates the schema into a scratch folder the way the release build does (go run
// ./tools/bin/config-schema, from the Go config's table), compiles it as JSON Schema draft
// 2020-12 with every keyword one that draft knows (as ajv's strict mode held it until T-143), and
// holds it to config check:
//
//   - this repository's itos.yaml passes;
//   - every config the corpus's config.yaml calls sound (config check exits 0, or 1 for the
//     ledger's, the registry's or the smoke set's problems) passes, and so every config the
//     schema refuses there is one config check refuses (exit 2): the schema may say less than
//     config check, never something different;
//   - every config of every other corpus file the schema refuses is one its case expects itos to
//     refuse (exit 2);
//   - every corpus config config check refuses for what the schema itself says (an unknown key, a
//     wrong type, a value not allowed, a missing key) the schema refuses too;
//   - this repository's itos.yaml with a misspelt key, a wrong type or a value not allowed is
//     refused, each for that reason, and so is it by config check;
//
// and it validated something.
package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v3"

	"github.com/donvargax/itos/v8/tools/selftest/scratch"
)

const draft = "https://json-schema.org/draft/2020-12/schema"

// keywords is every keyword of JSON Schema 2020-12, its annotations included.
var keywords = strings.Fields(`$schema $id $ref $defs $anchor $dynamicRef $dynamicAnchor $vocabulary
$comment title description default examples deprecated readOnly writeOnly type enum const
multipleOf maximum exclusiveMaximum minimum exclusiveMinimum maxLength minLength pattern maxItems
minItems uniqueItems maxContains minContains maxProperties minProperties required
dependentRequired format contentEncoding contentMediaType contentSchema properties
patternProperties additionalProperties propertyNames items prefixItems contains allOf anyOf
oneOf not if then else dependentSchemas unevaluatedItems unevaluatedProperties`)

// unknownKeywords is every key of schema s, and of the schemas it holds, that is no keyword,
// by its path.
func unknownKeywords(s any, at string) []string {
	obj, ok := s.(map[string]any)
	if !ok {
		return nil
	}
	var found []string
	for k, v := range obj {
		path := at + "/" + k
		switch {
		case !slices.Contains(keywords, k):
			found = append(found, path)
		case k == "properties" || k == "patternProperties" || k == "$defs" || k == "dependentSchemas":
			if m, ok := v.(map[string]any); ok {
				for name, sub := range m {
					found = append(found, unknownKeywords(sub, path+"/"+name)...)
				}
			}
		case k == "allOf" || k == "anyOf" || k == "oneOf" || k == "prefixItems":
			if list, ok := v.([]any); ok {
				for i, sub := range list {
					found = append(found, unknownKeywords(sub, path+"/"+strconv.Itoa(i))...)
				}
			}
		case k == "additionalProperties" || k == "propertyNames" || k == "items" || k == "contains" || k == "not" ||
			k == "if" || k == "then" || k == "else" || k == "unevaluatedItems" || k == "unevaluatedProperties" || k == "contentSchema":
			found = append(found, unknownKeywords(v, path)...)
		}
	}
	return found
}

// value is a YAML node as JSON reads it: YAML 1.2's core schema, so a timestamp stays a string.
func value(n *yaml.Node) (any, error) {
	switch n.Kind {
	case yaml.DocumentNode:
		if len(n.Content) == 0 {
			return nil, nil
		}
		return value(n.Content[0])
	case yaml.AliasNode:
		return value(n.Alias)
	case yaml.MappingNode:
		m := map[string]any{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			v, err := value(n.Content[i+1])
			if err != nil {
				return nil, err
			}
			m[n.Content[i].Value] = v
		}
		return m, nil
	case yaml.SequenceNode:
		list := []any{}
		for _, c := range n.Content {
			v, err := value(c)
			if err != nil {
				return nil, err
			}
			list = append(list, v)
		}
		return list, nil
	}
	switch n.ShortTag() {
	case "!!null":
		return nil, nil
	case "!!bool", "!!int", "!!float":
		var v any
		if err := n.Decode(&v); err != nil {
			return nil, err
		}
		return v, nil
	}
	return n.Value, nil
}

// instance is YAML text as the validator reads a document: maps, lists, strings, booleans, nil
// and Go numbers, NaN and the infinities among them, as YAML's .nan and .inf are numbers.
func instance(text string) (any, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil {
		return nil, err
	}
	return value(&doc)
}

// leaves is every error of a validation error's tree that has no causes.
func leaves(err *jsonschema.ValidationError) []*jsonschema.ValidationError {
	if len(err.Causes) == 0 {
		return []*jsonschema.ValidationError{err}
	}
	var out []*jsonschema.ValidationError
	for _, c := range err.Causes {
		out = append(out, leaves(c)...)
	}
	return out
}

// problems is a validation's errors on one line.
func problems(err error) string { return strings.ReplaceAll(fmt.Sprint(err), "\n", "; ") }

// corpusCase is a conformance case, as much of it as this test reads.
type corpusCase struct {
	Name  string         `yaml:"name"`
	Argv  []string       `yaml:"argv"`
	Cwd   string         `yaml:"cwd"`
	Files map[string]any `yaml:"files"`
	Exit  int            `yaml:"exit"`
	JSON  *struct {
		Problems []struct {
			Rule string `yaml:"rule"`
		} `yaml:"problems"`
	} `yaml:"json"`
	Stderr any `yaml:"stderr"`
}

// config check's rules for what the schema itself says, and the words its text output gives them.
var (
	schemaRules = []string{"config-type", "config-enum", "config-unknown-key", "config-missing-key"}
	schemaWords = []*regexp.Regexp{regexp.MustCompile(`: unknown key `), regexp.MustCompile(`: \S+ should be `)}
	fillSHA     = regexp.MustCompile(`\{\{sha\.[\w-]+\}\}`)
	fillText    = regexp.MustCompile(`\{\{[\w.-]+\}\}`)
)

func TestSelftest(t *testing.T) {
	scratch.Gate(t)
	root := scratch.Root(t)
	env := scratch.OutsideEnv()
	file := filepath.Join(t.TempDir(), "itos.schema.json")
	if r := scratch.Run(root, env, "", "go", "run", "./tools/bin/config-schema", file); r.Code != 0 {
		t.Fatalf("go run ./tools/bin/config-schema:\n%s", r.Output())
	}
	text, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(text))
	if err != nil {
		t.Fatalf("the schema is not JSON: %v", err)
	}
	var schema *jsonschema.Schema
	t.Run("the schema is JSON Schema 2020-12, every keyword one it knows", func(t *testing.T) {
		if got := doc.(map[string]any)["$schema"]; got != draft {
			t.Errorf("the schema's $schema is %v", got)
		}
		if unknown := unknownKeywords(doc, ""); len(unknown) > 0 {
			t.Errorf("the schema holds keys no 2020-12 keyword: %v", unknown)
		}
		c := jsonschema.NewCompiler()
		c.DefaultDraft(jsonschema.Draft2020)
		if err := c.AddResource("itos.schema.json", doc); err != nil {
			t.Fatal(err)
		}
		if schema, err = c.Compile("itos.schema.json"); err != nil {
			t.Fatalf("the schema does not compile: %v", err)
		}
	})
	if schema == nil {
		t.Fatal("no schema to validate with")
	}
	validate := func(v any) *jsonschema.ValidationError {
		err := schema.Validate(v)
		if err == nil {
			return nil
		}
		ve, ok := err.(*jsonschema.ValidationError)
		if !ok {
			t.Fatalf("validation failed: %v", err)
		}
		return ve
	}
	sound, refused, corpusConfigs := 0, 0, 0

	ownText, err := os.ReadFile(filepath.Join(root, "itos.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Run("this repository's itos.yaml passes", func(t *testing.T) {
		own, err := instance(string(ownText))
		if err != nil {
			t.Fatal(err)
		}
		if err := validate(own); err != nil {
			t.Errorf("this repository's itos.yaml is refused: %s", problems(err))
		} else {
			sound++
		}
	})

	// The corpus's configs: config.yaml's by config check's verdict on each, every other file's
	// by its case's exit, 2 for a config itos refuses.
	var unsound, unrefused, bySchema []string
	corpus := filepath.Join(root, "tools", "itos", "conformance")
	names, err := filepath.Glob(filepath.Join(corpus, "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(names)
	for _, path := range names {
		name := filepath.Base(path)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var c struct {
			Files map[string]any `yaml:"files"`
			Cases []corpusCase   `yaml:"cases"`
		}
		if err := yaml.Unmarshal(data, &c); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, k := range c.Cases {
			config := "itos.yaml"
			if at := slices.Index(k.Argv, "--config"); at >= 0 && at+1 < len(k.Argv) {
				config = k.Argv[at+1]
			}
			if k.Cwd != "" {
				config = k.Cwd + "/" + config
			}
			given, ok := k.Files[config]
			if !ok {
				given = c.Files[config]
			}
			written, ok := given.(string)
			if !ok {
				continue // no config file, or one the case removes (null)
			}
			// What the runner fills in: a commit's full SHA, or text.
			filled := fillText.ReplaceAllString(fillSHA.ReplaceAllString(written, strings.Repeat("0", 40)), "x")
			v, err := instance(filled)
			if err != nil {
				t.Fatalf("%s: %q: its config is not YAML: %v", name, k.Name, err)
			}
			corpusConfigs++
			where := fmt.Sprintf("%s: %q", name, k.Name)
			configCheck := name == "config.yaml" && len(k.Argv) >= 2 && k.Argv[0] == "config" && k.Argv[1] == "check" &&
				!slices.Contains(k.Argv, "--print-defaults")
			verr := validate(v)
			if verr == nil {
				if configCheck && k.Exit != 2 {
					sound++
				}
				// Refused by config check for what the schema itself says.
				said := false
				if k.JSON != nil && k.JSON.Problems != nil {
					for _, p := range k.JSON.Problems {
						said = said || slices.Contains(schemaRules, p.Rule)
					}
				} else {
					stderr, _ := k.Stderr.(string)
					for _, words := range schemaWords {
						said = said || words.MatchString(stderr)
					}
				}
				if configCheck && k.Exit == 2 && said {
					bySchema = append(bySchema, where)
				}
				continue
			}
			refused++
			if configCheck && k.Exit != 2 {
				unsound = append(unsound, where+": "+problems(verr))
			} else if k.Exit != 2 {
				unrefused = append(unrefused, fmt.Sprintf("%s: exits %d, not 2, and the schema refuses it: %s", where, k.Exit, problems(verr)))
			}
		}
	}
	t.Logf("%d corpus configs, %d sound to config check, %d refused by the schema", corpusConfigs, sound, refused)
	t.Run("every config config check calls sound passes", func(t *testing.T) {
		for _, u := range unsound {
			t.Errorf("sound to config check, refused: %s", u)
		}
	})
	t.Run("every corpus config the schema refuses is one itos refuses", func(t *testing.T) {
		for _, u := range unrefused {
			t.Error(u)
		}
	})
	t.Run("every config config check refuses for what the schema says the schema refuses", func(t *testing.T) {
		for _, b := range bySchema {
			t.Errorf("%s: refused by config check for what the schema says, passes", b)
		}
	})

	// This repository's config, each time with one thing wrong.
	for i, w := range []struct {
		name, keyword, path string
		edit                func(c map[string]any)
	}{
		{"a misspelt key is refused, by the schema and by config check", "additionalProperties", "/ci/cost", func(c map[string]any) {
			cost := c["ci"].(map[string]any)["cost"].(map[string]any)
			cost["keep_writen_order"] = cost["keep_written_order"]
			delete(cost, "keep_written_order")
		}},
		{"a wrong type is refused, by the schema and by config check", "type", "/version", func(c map[string]any) {
			c["version"] = "1"
		}},
		{"a value not allowed is refused, by the schema and by config check", "enum", "/commits/header_lint/use", func(c map[string]any) {
			c["commits"].(map[string]any)["header_lint"].(map[string]any)["use"] = "commitlint"
		}},
	} {
		t.Run(w.name, func(t *testing.T) {
			v, err := instance(string(ownText))
			if err != nil {
				t.Fatal(err)
			}
			config := v.(map[string]any)
			w.edit(config)
			data, err := json.Marshal(config)
			if err != nil {
				t.Fatal(err)
			}
			wrong, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			verr := validate(wrong)
			if verr == nil {
				t.Fatalf("it passes")
			}
			refused++
			found := false
			for _, leaf := range leaves(verr) {
				kp := leaf.ErrorKind.KeywordPath()
				at := "/" + strings.Join(leaf.InstanceLocation, "/")
				found = found || (len(kp) > 0 && kp[len(kp)-1] == w.keyword && at == w.path)
			}
			if !found {
				t.Errorf("it is refused, not by %s at %s: %s", w.keyword, w.path, problems(verr))
			}
			file := filepath.Join(t.TempDir(), "wrong-"+strconv.Itoa(i)+".yaml")
			if err := os.WriteFile(file, data, 0o644); err != nil {
				t.Fatal(err)
			}
			name, argv := scratch.Program(filepath.Join(root, "tools", "bin", "itos"), "config", "check", "-q", "--config", file)
			if r := scratch.Run(root, env, "", name, argv...); r.Code != 2 {
				t.Errorf("config check exits %d, not 2: %s", r.Code, r.Stderr)
			}
		})
	}

	// The repository's own config is one sound config; the corpus must give more.
	t.Run("the schema validated something", func(t *testing.T) {
		if corpusConfigs == 0 || sound < 2 || refused == 0 {
			t.Errorf("the schema validated nothing: %d corpus configs, %d sound, %d refused", corpusConfigs, sound, refused)
		}
	})
}
