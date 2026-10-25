// The steps of the YAML defaults' commented examples (ci-help.feature,
// slice 151): the YAML --print-defaults prints, read with its comments
// dropped, against the defaults --json prints.
package features

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/cucumber/godog"
	"go.yaml.in/yaml/v3"
)

func initializeCIHelpSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the YAML on stdout reads as the "([^"]*)" that "([^"]*)" prints in the same folder$`, w.yamlReadsAsJSON)
	sc.Step(`^the YAML on stdout has no key "([^"]*)"$`, w.yamlHasNoKey)
}

// The stdout YAML as a JSON value, so that 600 and 600.0 are equal.
func (w *world) stdoutYAMLAsJSON() (any, error) {
	var doc any
	if err := yaml.Unmarshal([]byte(w.stdout), &doc); err != nil {
		return nil, fmt.Errorf("stdout is not YAML: %w\n%s", err, w.report())
	}
	text, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	var v any
	return v, json.Unmarshal(text, &v)
}

// The YAML on stdout equals the member of the JSON the command prints in the
// scenario's folder; the command's own output is not kept.
func (w *world) yamlReadsAsJSON(member, command string) error {
	got, err := w.stdoutYAMLAsJSON()
	if err != nil {
		return err
	}
	stdout, stderr, exit, last := w.stdout, w.stderr, w.exit, w.lastCommand
	defer func() { w.stdout, w.stderr, w.exit, w.lastCommand = stdout, stderr, exit, last }()
	if err := w.itos(strings.Fields(command)...); err != nil {
		return err
	}
	var printed map[string]any
	if err := json.Unmarshal([]byte(w.stdout), &printed); err != nil {
		return fmt.Errorf("%q printed no JSON object: %w\n%s", command, err, w.report())
	}
	want, ok := printed[member]
	if !ok {
		return fmt.Errorf("%q printed no %q\n%s", command, member, w.report())
	}
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("the YAML on stdout is not the %q %q prints\nYAML: %v\nJSON: %v", member, command, got, want)
	}
	return nil
}

// The YAML on stdout has no value at the dotted key.
func (w *world) yamlHasNoKey(key string) error {
	v, err := w.stdoutYAMLAsJSON()
	if err != nil {
		return err
	}
	for _, part := range strings.Split(key, ".") {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		if v, ok = m[part]; !ok {
			return nil
		}
	}
	return fmt.Errorf("the YAML on stdout has the key %q", key)
}
