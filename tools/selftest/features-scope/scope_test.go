// Package scope holds the self-test of what a CI run's features step selects (T-005's and
// T-016's checks, kept as a step in T-125, a Go test since T-143), proved by running it: go
// test's own record of the scenarios it ran (-json), each subtest's name read back to its ID from
// the feature files, for ranges made with git commit-tree on top of HEAD (objects only: no branch
// moves):
//
//   - tools/bin/itos tests smoke run scenario runs exactly the smoke set;
//   - a range naming one scenario runs the smoke set plus that one;
//   - a range CI can't read runs every live scenario, and so does the nightly: its plan (under
//     the hostile PATHs) and nightly.yml's platform jobs on Linux, macOS and Windows (T-163);
//   - a range naming the ledger's tasks, none of which holds a check that is a run of the
//     features or a step any more (T-125 emptied them), still runs one go test run, over the
//     smoke set alone;
//   - the smoke rule holds today.
//
// The plan, the scenarios, the smoke set and the rule are tools/bin/itos's answers (ci plan
// --json, tests list, tests smoke ids and check); the feature files are read here only to name go
// test's subtests.
//
// What the plan does whatever the repository (the cost order, the written order, the prose
// shortcut, merging a task check into that run, covering one by a step, the smoke rule's
// failures) is tools/itos/conformance/plans.yaml's and smoke.yaml's, over configurations of their
// own, since no task of this repository holds a check to try it on.
package scope

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/donvargax/itos/v8/tools/selftest/scratch"
)

// kind is the scenario kind, as itos.yaml states it.
type kind struct {
	Root      string `yaml:"root"`
	ID        string `yaml:"id"`
	TagPrefix string `yaml:"tag_prefix"`
	Run       struct {
		Whole string `yaml:"whole"`
	} `yaml:"run"`
}

// ran is how many times go test ran each subtest name.
type ran map[string]int

func TestSelftest(t *testing.T) {
	scratch.Gate(t)
	root := scratch.Root(t)
	env := scratch.OutsideEnv()
	data, err := os.ReadFile(filepath.Join(root, "itos.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var policy struct {
		Tests struct {
			Scenario kind `yaml:"scenario"`
		} `yaml:"tests"`
	}
	if err := yaml.Unmarshal(data, &policy); err != nil {
		t.Fatal(err)
	}
	k := policy.Tests.Scenario
	whole := k.Run.Whole

	sh := func(t *testing.T, command string) scratch.Result {
		t.Helper()
		return scratch.Run(root, env, "", "sh", "-c", command)
	}
	git := func(t *testing.T, args ...string) string {
		t.Helper()
		r := scratch.Run(root, env, "", scratch.Git(), args...)
		if r.Code != 0 {
			t.Fatalf("git %s failed:\n%s", strings.Join(args, " "), r.Output())
		}
		return strings.TrimSpace(r.Stdout)
	}
	head := git(t, "rev-parse", "HEAD")
	// A commit on top of HEAD with HEAD's tree and this message.
	rangeOf := func(t *testing.T, message string) string {
		return git(t, "-c", "user.name=features-scope", "-c", "user.email=selftest@localhost",
			"commit-tree", "HEAD^{tree}", "-p", "HEAD", "-m", message)
	}

	// go test's subtest names for a scenario, and the scenarios' names by them: go test names a
	// subtest after the scenario, its spaces turned to underscores, and two scenarios may share a
	// name (ID-ASK-01 and ID-ASK-20), which it tells apart by a "#01" suffix on the later one. The
	// suffix is dropped here and the scenarios sharing a name are counted, since which of them ran
	// is not in what go test reports. A Scenario Outline runs once per row of its Examples, each
	// run named after the outline with the row's values in its <column>s, so as many names as rows.
	idTag := regexp.MustCompile("^" + regexp.QuoteMeta(k.TagPrefix) + "(" + k.ID + ")$")
	scenarioLine := regexp.MustCompile(`^Scenario( Outline)?:\s*(.*?)\s*$`)
	subtestsOf := map[string][]string{} // an ID, its subtest name once per run
	names := map[string]bool{}          // every subtest name
	subtest := func(name string) string {
		s := "TestFeatures/" + strings.ReplaceAll(name, " ", "_")
		names[s] = true
		return s
	}
	cells := func(row string) []string {
		parts := strings.Split(strings.TrimSpace(row), "|")
		out := make([]string, 0, len(parts))
		for _, cell := range parts[1 : len(parts)-1] {
			out = append(out, strings.TrimSpace(cell))
		}
		return out
	}
	features, err := filepath.Glob(filepath.Join(root, k.Root, "*.feature"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range features {
		text, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var tags []string
		// The outline being read, and its current Examples' header once read.
		var outline, outlineID string
		var header []string
		inExamples := false
		for _, line := range strings.Split(string(text), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, k.TagPrefix) {
				tags = append(tags, strings.Fields(line)...)
			}
			if outlineID != "" && strings.HasPrefix(line, "Examples:") {
				inExamples, header, tags = true, nil, nil
				continue
			}
			if inExamples && strings.HasPrefix(line, "|") {
				if header == nil {
					header = cells(line)
					continue
				}
				name := outline
				for i, value := range cells(line) {
					if i < len(header) {
						name = strings.ReplaceAll(name, "<"+header[i]+">", value)
					}
				}
				subtestsOf[outlineID] = append(subtestsOf[outlineID], subtest(name))
				continue
			}
			m := scenarioLine.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			outline, outlineID, header, inExamples = "", "", nil, false
			for _, tag := range tags {
				if id := idTag.FindStringSubmatch(tag); id != nil {
					if m[1] != "" {
						outline, outlineID = m[2], id[1]
						subtestsOf[id[1]] = nil
					} else {
						subtestsOf[id[1]] = []string{subtest(m[2])}
					}
					break
				}
			}
			tags = nil
		}
	}
	suffix := regexp.MustCompile(`#\d+$`)
	// What a command ran, as go test reports it: each subtest's name and how many times, the
	// "#01" of a name two scenarios share dropped.
	runOf := func(t *testing.T, command string) ran {
		t.Helper()
		r := sh(t, command+" -json")
		if r.Code != 0 {
			t.Fatalf("%s failed:\n%s", command, r.Output())
		}
		counts := ran{}
		lines := bufio.NewScanner(strings.NewReader(r.Stdout))
		lines.Buffer(make([]byte, 1<<20), 64<<20)
		for lines.Scan() {
			var event struct{ Action, Test string }
			if !strings.HasPrefix(lines.Text(), "{") || json.Unmarshal(lines.Bytes(), &event) != nil {
				continue
			}
			if event.Action != "run" || !strings.HasPrefix(event.Test, "TestFeatures/") {
				continue
			}
			name := suffix.ReplaceAllString(event.Test, "")
			if !names[name] {
				t.Fatalf("go test ran %s, which no feature file names", event.Test)
			}
			counts[name]++
		}
		return counts
	}
	// The same, as a set of scenario IDs would read it: each name the IDs share, an outline's once
	// per Examples row.
	wanted := func(t *testing.T, ids ...[]string) ran {
		t.Helper()
		out := ran{}
		for _, each := range ids {
			for _, id := range each {
				subtests, ok := subtestsOf[id]
				if !ok {
					t.Fatalf("no feature file names %s", id)
				}
				if len(subtests) == 0 {
					t.Fatalf("the outline %s has no Examples row", id)
				}
				for _, name := range subtests {
					out[name]++
				}
			}
		}
		return out
	}
	same := func(t *testing.T, got, want ran, what string) {
		t.Helper()
		var missing, extra []string
		for name, n := range want {
			if got[name] != n {
				missing = append(missing, fmt.Sprintf("%s x%d", name, n))
			}
		}
		for name, n := range got {
			if want[name] != n {
				extra = append(extra, fmt.Sprintf("%s x%d", name, n))
			}
		}
		sort.Strings(missing)
		sort.Strings(extra)
		if len(missing) > 0 || len(extra) > 0 {
			t.Errorf("%s: missing %v; extra %v", what, missing, extra)
		}
	}

	var listed struct {
		Tests []struct {
			ID   string `json:"id"`
			Live bool   `json:"live"`
		} `json:"tests"`
	}
	if err := json.Unmarshal([]byte(scratch.Itos(t, root, "tests", "list", "scenario", "--json")), &listed); err != nil {
		t.Fatal(err)
	}
	var live []string
	for _, test := range listed.Tests {
		if test.Live {
			live = append(live, test.ID)
		}
	}
	smoke := strings.Fields(scratch.Itos(t, root, "tests", "smoke", "ids", "scenario"))

	t.Run("the whole run is every live scenario", func(t *testing.T) {
		same(t, runOf(t, whole), wanted(t, live), "the whole run is not every live scenario")
	})
	t.Run("the smoke run is the smoke set", func(t *testing.T) {
		same(t, runOf(t, "tools/bin/itos tests smoke run scenario --"), wanted(t, smoke), "the smoke run is not the smoke set")
	})

	// A range naming one scenario: the smoke set and it, one outside the smoke set.
	t.Run("a range naming a scenario runs the smoke set and it", func(t *testing.T) {
		named := live[len(live)-1]
		for _, id := range live {
			if !slices.Contains(smoke, id) {
				named = id
				break
			}
		}
		step := scratch.FeaturesStep(scratch.CIPlan(t, root, head, rangeOf(t, "feat: name a scenario\n\nScenarios: @"+named+"\n")), whole)
		if step == "" {
			t.Fatal("a push naming a scenario ran no features step")
		}
		same(t, runOf(t, step), wanted(t, smoke, []string{named}), "a range naming @"+named)
	})

	// The nightly, and a range that can't be read, run every scenario. Since T-163 the nightly's
	// plain run of every feature is .github/workflows/nightly.yml's platform jobs, one on each of
	// Linux, macOS and Windows; the plan runs every feature too, with something hostile first on
	// the PATH (a step wrapping the whole run in sh -c). A whole run here is the whole command
	// with nothing after it that could narrow it: the end of the command, a closing quote or a
	// separator, and in the platform jobs only the -timings flag (T-115).
	t.Run("the nightly runs every scenario on three platforms and in its plan, and the gates self-test", func(t *testing.T) {
		wholeRun := regexp.MustCompile(regexp.QuoteMeta(whole) + `(?:$|['"]|\s*(?:&&|\|\||;))`)
		nightly := scratch.CIPlan(t, root, "--nightly").Steps
		plain, gates := false, false
		for _, step := range nightly {
			plain = plain || wholeRun.MatchString(step)
			gates = gates || strings.Contains(step, "./tools/selftest/gates")
		}
		if !plain {
			t.Errorf("no step of the nightly's plan runs every scenario (%s): %v", whole, nightly)
		}
		if !gates {
			t.Error("the nightly runs no gates self-test")
		}

		data, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "nightly.yml"))
		if err != nil {
			t.Fatal(err)
		}
		var workflow struct {
			Jobs map[string]struct {
				If       string `yaml:"if"`
				RunsOn   string `yaml:"runs-on"`
				Strategy struct {
					Matrix struct {
						OS      []string `yaml:"os"`
						Include []struct {
							OS    string `yaml:"os"`
							Shard string `yaml:"shard"`
						} `yaml:"include"`
					} `yaml:"matrix"`
				} `yaml:"strategy"`
				Steps []struct {
					Run string `yaml:"run"`
				} `yaml:"steps"`
			} `yaml:"jobs"`
		}
		if err := yaml.Unmarshal(data, &workflow); err != nil {
			t.Fatal(err)
		}
		// A platform's shards (T-308) run every scenario together, when they are all i/n of one n.
		platformRun := regexp.MustCompile(`^` + regexp.QuoteMeta(whole) + `(?:\s+\$\{SHARD:\+-shard=\$SHARD\})?(?:\s+-timings=\S+)?\s*$`)
		covered := map[string]bool{}
		shards := map[string][]string{}
		for _, job := range workflow.Jobs {
			if job.If != "" || job.RunsOn != "${{ matrix.os }}" {
				continue
			}
			runs := false
			for _, step := range job.Steps {
				runs = runs || platformRun.MatchString(strings.TrimSpace(step.Run))
			}
			if !runs {
				continue
			}
			for _, runner := range job.Strategy.Matrix.OS {
				for _, system := range []string{"ubuntu", "macos", "windows"} {
					covered[system] = covered[system] || strings.HasPrefix(runner, system+"-")
				}
			}
			for _, entry := range job.Strategy.Matrix.Include {
				for _, system := range []string{"ubuntu", "macos", "windows"} {
					if strings.HasPrefix(entry.OS, system+"-") {
						shards[system] = append(shards[system], entry.Shard)
					}
				}
			}
		}
		for system, each := range shards {
			covered[system] = covered[system] || allShards(each)
		}
		for _, system := range []string{"ubuntu", "macos", "windows"} {
			if !covered[system] {
				t.Errorf("no job of nightly.yml runs every scenario (%s) on %s", whole, system)
			}
		}
	})
	t.Run("a range CI cannot read runs everything", func(t *testing.T) {
		if step := scratch.FeaturesStep(scratch.CIPlan(t, root, "", head), whole); step != whole {
			t.Errorf("an unread range should run everything, ran %q", step)
		}
	})

	// A range naming tasks: one go test run, over the smoke set. What a task check reads as a run
	// of this kind, or as a step that has done it, is the corpus's (plans.yaml), over a
	// configuration of its own: T-125 emptied this repository's ledger of done tasks' checks, so
	// there is no check here to try each rule on.
	t.Run("a range naming tasks runs one go test run, over the smoke set", func(t *testing.T) {
		plan := scratch.CIPlan(t, root, head, rangeOf(t, "test: name two tasks\n\nTask: T-016, T-009\n"))
		var runs []string
		for _, step := range plan.Steps {
			if strings.HasPrefix(step, whole) {
				runs = append(runs, step)
			}
		}
		if len(runs) != 1 {
			t.Fatalf("expected one go test run, got:\n%s", strings.Join(runs, "\n"))
		}
		same(t, runOf(t, runs[0]), wanted(t, smoke), "the run of a range naming tasks")
	})

	t.Run("the smoke rule holds today", func(t *testing.T) {
		scratch.Itos(t, root, "tests", "smoke", "check", "scenario")
	})
}

// allShards is whether the shards, "" for a job that runs every scenario, are one such job or every
// "i/n" of one n, once each.
func allShards(shards []string) bool {
	if len(shards) == 1 && shards[0] == "" {
		return true
	}
	n, seen := 0, map[int]bool{}
	for _, shard := range shards {
		var i, total int
		if _, err := fmt.Sscanf(shard, "%d/%d", &i, &total); err != nil || i < 1 || i > total || (n != 0 && total != n) || seen[i] {
			return false
		}
		n, seen[i] = total, true
	}
	return n != 0 && len(seen) == n
}
