// The features: every *.feature file in this folder, run by godog through go
// test, each scenario a subtest of TestFeatures.
//
//	go test ./features -count=1                        every live scenario
//	go test ./features -count=1 -scenarios=<regexp>    the live scenarios with a
//	                                                   tag the expression matches
//
// godog's own tag filter takes exact tags joined by commas; itos's run
// templates join the IDs they select with |, as a regular expression
// (itos.yaml's tests.scenario.run), so -scenarios takes that expression and
// is turned here into godog's filter. A scenario tagged @wip never runs.
package features

import (
	"bufio"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/cucumber/godog"
)

var scenarios = flag.String("scenarios", "", "run only the live scenarios with a tag this regular expression matches")

func TestFeatures(t *testing.T) {
	filter, err := tagFilter(*scenarios, ".")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(removeCallerPath)
	suite := godog.TestSuite{
		Name:                "itos",
		ScenarioInitializer: initializeScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"."},
			Tags:     filter,
			Strict:   true,
			TestingT: t,
		},
	}
	if suite.Run() != 0 {
		t.Fatal("a scenario failed")
	}
}

// godog's tag filter for a selection: every live scenario without one, else
// the live scenarios carrying a tag the expression matches. An expression
// that matches no tag is an error, so a selection never runs nothing.
func tagFilter(selection, dir string) (string, error) {
	if selection == "" {
		return "~@wip", nil
	}
	pattern, err := regexp.Compile(selection)
	if err != nil {
		return "", err
	}
	tags, err := featureTags(dir)
	if err != nil {
		return "", err
	}
	var each []string
	for _, tag := range tags {
		if pattern.MatchString(tag) {
			each = append(each, tag+"&&~@wip")
		}
	}
	if len(each) == 0 {
		return "", &noMatch{selection}
	}
	return strings.Join(each, ","), nil
}

type noMatch struct{ selection string }

func (e *noMatch) Error() string {
	return "no scenario has a tag that " + e.selection + " matches"
}

// Every tag written in the feature files under dir, once each, sorted.
func featureTags(dir string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.feature"))
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return nil, err
		}
		lines := bufio.NewScanner(f)
		for lines.Scan() {
			line := strings.TrimSpace(lines.Text())
			if !strings.HasPrefix(line, "@") {
				continue
			}
			for _, word := range strings.Fields(line) {
				if strings.HasPrefix(word, "#") {
					break
				}
				if strings.HasPrefix(word, "@") {
					seen[word] = true
				}
			}
		}
		f.Close()
		if err := lines.Err(); err != nil {
			return nil, err
		}
	}
	tags := make([]string, 0, len(seen))
	for tag := range seen {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags, nil
}
