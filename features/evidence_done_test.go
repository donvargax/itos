// The steps of evidence-done.feature (slice 173): a gitlink committed to
// HEAD's tree, and the readings of the work-done-task-check problem and of
// the evidence an itos work done JSON names for a command.
package features

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/cucumber/godog"
)

func initializeEvidenceDoneSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^a gitlink at "([^"]*)" pointing to HEAD is committed$`, func(path string) error {
		head, err := w.head()
		if err != nil {
			return err
		}
		if err := w.git("update-index", "--add", "--cacheinfo", "160000,"+head+","+path); err != nil {
			return err
		}
		if err := w.git("commit", "-q", "--no-verify", "-m", "test: add a gitlink"); err != nil {
			return err
		}
		sha, err := w.head()
		if err != nil {
			return err
		}
		w.commits = append(w.commits, sha)
		return nil
	})
	sc.Step(`^its JSON's problem "([^"]*)" has "([^"]*)" (.+)$`, func(rule, field, want string) error {
		problem, err := w.problemOfRule(rule)
		if err != nil {
			return err
		}
		var expected any
		if err := json.Unmarshal([]byte(want), &expected); err != nil {
			return fmt.Errorf("the scenario's %s is not JSON: %v", want, err)
		}
		got, has := problem[field]
		if !has || !reflect.DeepEqual(got, expected) {
			return fmt.Errorf("the problem %s has %s %s, not %s\n%s", rule, field, jsonText(got), want, w.report())
		}
		return nil
	})
	sc.Step(`^its JSON's problem "([^"]*)"'s "([^"]*)" says "([^"]*)"$`, func(rule, field, text string) error {
		problem, err := w.problemOfRule(rule)
		if err != nil {
			return err
		}
		if got, _ := problem[field].(string); !strings.Contains(got, text) {
			return fmt.Errorf("the problem %s's %s is %q, which does not say %q\n%s", rule, field, got, text, w.report())
		}
		return nil
	})
	sc.Step(`^its JSON's evidence for the command "([^"]*)" has "([^"]*)" "([^"]*)"$`, func(command, field, want string) error {
		var report struct {
			Evidence []map[string]any `json:"evidence"`
		}
		if err := json.Unmarshal([]byte(w.stdout), &report); err != nil {
			return fmt.Errorf("the output is not JSON: %v\n%s", err, w.report())
		}
		for _, e := range report.Evidence {
			if e["command"] == command {
				if e[field] != want {
					return fmt.Errorf("the command %q has %s %v, not %q\n%s", command, field, e[field], want, w.report())
				}
				return nil
			}
		}
		return fmt.Errorf("the JSON has no evidence for the command %q\n%s", command, w.report())
	})
}

// problemOfRule is the one entry of the JSON's problems whose rule is given.
func (w *world) problemOfRule(rule string) (map[string]any, error) {
	var report struct {
		Problems []map[string]any `json:"problems"`
	}
	if err := json.Unmarshal([]byte(w.stdout), &report); err != nil {
		return nil, fmt.Errorf("the output is not JSON: %v\n%s", err, w.report())
	}
	for _, p := range report.Problems {
		if p["rule"] == rule {
			return p, nil
		}
	}
	return nil, fmt.Errorf("the JSON has no problem whose rule is %q\n%s", rule, w.report())
}
