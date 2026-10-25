// The steps of tests list --tsv (tests-list.feature, slice 154): feature files under a kind
// root of any name, a command adapter whose listing a scenario writes, invalid rows included,
// the listings at HEAD, the index and the working tree, and the tripwires that show no runner,
// hook or CI provider ran: every scenario of the slice runs under the recording shell, with a
// hook stand-in and a gh first on the PATH that record their calls.
package features

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/cucumber/godog"
)

// The state of one slice-154 scenario beyond the world's.
type tsvScenario struct {
	live, wip []string          // the scenario kind's live and @wip IDs, as the steps wrote them
	fixture   *fixtureListing   // the command adapter's listing, when a step wrote one
	state     map[string]string // the repository's state a step recorded before running itos
	byTree    map[string]tsvRun // the TSV listing at each tree, as "I obtain TSV listings" ran them
	expected  map[string][]idAt // each tree's live tests, as the steps that made the trees wrote them
	text      tsvRun            // the ordinary text listing
	jsonRun   tsvRun            // the JSON listing
	bad       badRow            // the row ID-TESTLIST-05's example writes after the valid one
	calls     int               // the adapter's calls when the last run that exited 0 ended
}

// One run of itos: its exit code and output.
type tsvRun struct {
	exit           int
	stdout, stderr string
}

// A live test as a tree holds it: its ID and its project-relative file.
type idAt struct{ id, file string }

// The listing the fixture kind's command adapter prints: its tests, each the protocol's object
// with any extra field, and the protocol object's extra fields.
type fixtureListing struct {
	tests []map[string]any
	extra map[string]any
}

func initializeTestsTSVSteps(sc *godog.ScenarioContext, w *world) {
	var s tsvScenario
	sc.Before(func(ctx context.Context, sn *godog.Scenario) (context.Context, error) {
		s = tsvScenario{}
		if w.outsideShard {
			return ctx, nil
		}
		for _, tag := range sn.Tags {
			if tag.Name == "@slice-154" {
				return ctx, w.armTripwires()
			}
		}
		return ctx, nil
	})

	sc.Step(`^another scenario is tagged @wip and a third feature is tagged @wip$`, func() error {
		s.live = []string{"ID-A-01", "ID-B-01"}
		if err := w.appendScenario("ID-A-01", "  @ID-A-02 @wip\n  Scenario: ID-A-02 waits\n    When it waits\n"); err != nil {
			return err
		}
		w.scenarioFiles["ID-A-02"] = w.scenarioFiles["ID-A-01"]
		w.scenarioFiles["ID-C-01"] = "id-c-01.feature"
		s.wip = []string{"ID-A-02", "ID-C-01"}
		return w.write("features/id-c-01.feature", "@phase-1 @wip\nFeature: id-c-01.feature\n\n  @ID-C-01\n  Scenario: ID-C-01 runs\n    When it runs\n")
	})
	sc.Step(`^stdout consists only of the two live IDs, their project-relative files and their fingerprints, the columns separated by one literal tab, one LF-terminated row each$`, func() error {
		prints, err := w.sideFingerprints()
		if err != nil {
			return err
		}
		want := map[string]string{}
		for _, id := range s.live {
			want[id] = id + "\tfeatures/" + w.scenarioFiles[id] + "\t" + prints[id]
		}
		rows, err := w.tsvRows()
		if err != nil {
			return err
		}
		if len(rows) != len(want) {
			return fmt.Errorf("stdout has %d rows, not the %d live IDs %v\n%s", len(rows), len(want), s.live, w.report())
		}
		for _, row := range rows {
			id, _, _ := strings.Cut(row, "\t")
			if want[id] != row || !fingerprintShape.MatchString(prints[id]) {
				return fmt.Errorf("the row %q is not one of %q\n%s", row, want, w.report())
			}
			delete(want, id)
		}
		return nil
	})
	sc.Step(`^the rows retain the normal adapter listing order$`, func() error {
		listed, err := w.sideJSONTests()
		if err != nil {
			return err
		}
		var order []string
		for _, t := range listed {
			if t.Live {
				order = append(order, t.ID)
			}
		}
		rows, err := w.tsvRows()
		if err != nil {
			return err
		}
		var got []string
		for _, row := range rows {
			id, _, _ := strings.Cut(row, "\t")
			got = append(got, id)
		}
		if !slices.Equal(got, order) {
			return fmt.Errorf("the rows list %v, and the JSON listing's live tests are %v\n%s", got, order, w.report())
		}
		return nil
	})
	sc.Step(`^stdout has no tag prefix, wip rows, heading, status words or other column$`, func() error {
		rows, err := w.tsvRows()
		if err != nil {
			return err
		}
		for _, row := range rows {
			fields := strings.Split(row, "\t")
			switch {
			case len(fields) != 3 || !fingerprintShape.MatchString(fields[2]):
				return fmt.Errorf("the row %q is not an ID, a file and a fingerprint\n%s", row, w.report())
			case strings.HasPrefix(fields[0], "@"):
				return fmt.Errorf("the row %q has the tag prefix\n%s", row, w.report())
			case slices.Contains(s.wip, fields[0]):
				return fmt.Errorf("the row %q is a @wip scenario's\n%s", row, w.report())
			case !scenarioID.MatchString(fields[0]):
				return fmt.Errorf("the row %q is no scenario's: a heading\n%s", row, w.report())
			case regexp.MustCompile(`\b(live|wip)\b`).MatchString(row):
				return fmt.Errorf("the row %q has a status word\n%s", row, w.report())
			}
		}
		return nil
	})
	sc.Step(`^I obtain the ordinary text and JSON listings of the same kind$`, func() error {
		if err := w.itos("tests", "list", "scenario"); err != nil {
			return err
		}
		s.text = tsvRun{w.exit, w.stdout, w.stderr}
		if err := w.itos("tests", "list", "scenario", "--json"); err != nil {
			return err
		}
		s.jsonRun = tsvRun{w.exit, w.stdout, w.stderr}
		return nil
	})
	sc.Step(`^those listings retain their existing file paths, live flags and output shape$`, func() error {
		return w.ordinaryListings(s)
	})
	sc.Step(`^no test runner, hook or CI provider has run$`, func() error { return w.noneRan(true) })
	sc.Step(`^no test runner has run$`, func() error { return w.noneRan(false) })

	sc.Step(`^the scenario kind root is "([^"]*)"$`, func(root string) error {
		w.config.scenarioRoot = root
		w.config.scenarios = true
		return w.writeConfig()
	})
	sc.Step(`^it contains the live scenario "([^"]*)" in "([^"]*)"$`, func(id, file string) error {
		if w.scenarioFiles == nil {
			w.scenarioFiles = map[string]string{}
		}
		w.scenarioFiles[id] = file
		return w.write(filepath.Join(w.config.scenarioRoot, file), fingerprintedFeature(file, id))
	})
	sc.Step(`^the repository uses (project|stealth) configuration$`, func(layout string) error {
		if layout == "stealth" {
			return w.configToGitFolder()
		}
		return nil
	})
	sc.Step(`^I run "itos ([^"]*)" from a project subfolder$`, func(line string) error {
		args, err := shellWords(line)
		if err != nil {
			return err
		}
		sub := filepath.Join(w.dir, "docs", "notes")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			return err
		}
		if s.state, err = w.configHeadIndex(); err != nil {
			return err
		}
		return w.itosIn(sub, args...)
	})
	sc.Step(`^stdout is exactly ID "([^"]*)", one tab, path "([^"]*)", one tab, its fingerprint and one LF$`, func(id, path string) error {
		prints, err := w.sideFingerprints()
		if err != nil {
			return err
		}
		if !fingerprintShape.MatchString(prints[id]) {
			return fmt.Errorf("the JSON listing gives %s the fingerprint %q", id, prints[id])
		}
		if want := id + "\t" + path + "\t" + prints[id] + "\n"; w.stdout != want {
			return fmt.Errorf("stdout is %q, not %q\n%s", w.stdout, want, w.report())
		}
		return nil
	})
	sc.Step(`^the path has slash separators on every supported platform$`, func() error {
		fields := strings.Split(strings.TrimSuffix(w.stdout, "\n"), "\t")
		if len(fields) < 2 || strings.Contains(fields[1], `\`) || !strings.Contains(fields[1], "/") {
			return fmt.Errorf("the path of %q is not slash separated\n%s", w.stdout, w.report())
		}
		return nil
	})
	sc.Step(`^neither configuration data, HEAD nor the index changed$`, func() error {
		now, err := w.configHeadIndex()
		if err != nil {
			return err
		}
		return sameState(s.state, now, w.report())
	})

	sc.Step(`^HEAD has a committed live scenario in "([^"]*)"$`, func(file string) error {
		w.config.scenarios = true
		if err := w.writeConfig(); err != nil {
			return err
		}
		if err := w.write(file, fingerprintedFeature(filepath.Base(file), "ID-OLD-01")); err != nil {
			return err
		}
		if err := w.commit("test: the old scenario"); err != nil {
			return err
		}
		s.expected = map[string][]idAt{"HEAD": {{"ID-OLD-01", file}}}
		return nil
	})
	sc.Step(`^the index has moved it to "([^"]*)" and added a live scenario$`, func(to string) error {
		from := s.expected["HEAD"][0].file
		if err := w.git("mv", from, to); err != nil {
			return err
		}
		added := "features/added.feature"
		if err := w.write(added, fingerprintedFeature("added.feature", "ID-NEW-01")); err != nil {
			return err
		}
		if err := w.git("add", "--", added); err != nil {
			return err
		}
		s.expected["index"] = []idAt{{"ID-NEW-01", added}, {"ID-OLD-01", to}}
		return nil
	})
	sc.Step(`^the working tree adds a third live scenario and a @wip scenario$`, func() error {
		s.wip = []string{"ID-WIP-01"}
		third := "features/third.feature"
		text := fingerprintedFeature("third.feature", "ID-THIRD-01") + "\n  @ID-WIP-01 @wip\n  Scenario: ID-WIP-01 waits\n    When it waits\n"
		if err := w.write(third, text); err != nil {
			return err
		}
		s.expected["worktree"] = append(slices.Clone(s.expected["index"]), idAt{"ID-THIRD-01", third})
		return nil
	})
	sc.Step(`^I obtain TSV listings at HEAD, index and worktree$`, func() error {
		var err error
		if s.state, err = w.treesState(); err != nil {
			return err
		}
		s.byTree = map[string]tsvRun{}
		for _, tree := range []string{"HEAD", "index", "worktree"} {
			if err := w.itos("tests", "list", "scenario", "--tsv", "--at", tree); err != nil {
				return err
			}
			s.byTree[tree] = tsvRun{w.exit, w.stdout, w.stderr}
		}
		return nil
	})
	sc.Step(`^each listing exits with code 0$`, func() error {
		for tree, run := range s.byTree {
			if run.exit != 0 {
				return fmt.Errorf("the listing at %s exited %d\n%s", tree, run.exit, run.report())
			}
		}
		return nil
	})
	sc.Step(`^each lists precisely its tree's live IDs and project-relative files$`, func() error {
		for tree, run := range s.byTree {
			var got []idAt
			for _, row := range strings.Split(strings.TrimSuffix(run.stdout, "\n"), "\n") {
				fields := strings.Split(row, "\t")
				if len(fields) < 2 {
					return fmt.Errorf("the listing at %s has the row %q\n%s", tree, row, run.report())
				}
				got = append(got, idAt{fields[0], fields[1]})
			}
			byID := func(a, b idAt) int { return strings.Compare(a.id, b.id) }
			want := slices.Clone(s.expected[tree])
			slices.SortFunc(got, byID)
			slices.SortFunc(want, byID)
			if !slices.Equal(got, want) {
				return fmt.Errorf("the listing at %s lists %v, not %v\n%s", tree, got, want, run.report())
			}
		}
		return nil
	})
	sc.Step(`^none lists the @wip scenario$`, func() error {
		for tree, run := range s.byTree {
			for _, id := range s.wip {
				if strings.Contains(run.stdout, id) {
					return fmt.Errorf("the listing at %s lists %s\n%s", tree, id, run.report())
				}
			}
		}
		return nil
	})
	sc.Step(`^the staged changes, unstaged changes and HEAD remain unchanged$`, func() error {
		now, err := w.treesState()
		if err != nil {
			return err
		}
		return sameState(s.state, now, w.report())
	})

	sc.Step(`^a command adapter lists live IDs "([^"]*)" and "([^"]*)" in that order, with an intervening non-live test$`, func(a, b string) error {
		s.fixture = &fixtureListing{tests: []map[string]any{
			{"id": a, "file": strings.ToLower(a) + ".txt", "live": true},
			{"id": "F-002", "file": "f-002.txt", "live": false},
			{"id": b, "file": strings.ToLower(b) + ".txt", "live": true},
		}}
		return w.writeFixture(s.fixture)
	})
	sc.Step(`^that kind has no configured root and lists project-relative files$`, func() error {
		for _, t := range s.fixture.tests {
			t["file"] = "fixtures/" + t["file"].(string)
		}
		return w.writeFixture(s.fixture)
	})
	sc.Step(`^its adapter protocol includes extra object and test fields$`, func() error {
		s.fixture.extra = map[string]any{"generator": "fixture-adapter"}
		for _, t := range s.fixture.tests {
			t["suite"] = "unit"
		}
		return w.writeFixture(s.fixture)
	})
	sc.Step(`^stdout contains exactly the two live ID/file rows in adapter order with two tab-separated columns$`, func() error {
		var want strings.Builder
		for _, t := range s.fixture.tests {
			if t["live"] == true {
				fmt.Fprintf(&want, "%s\t%s\n", t["id"], t["file"])
			}
		}
		if w.stdout != want.String() {
			return fmt.Errorf("stdout is %q, not %q\n%s", w.stdout, want.String(), w.report())
		}
		return nil
	})
	sc.Step(`^the adapter was called once for "([^"]*)"$`, func(tree string) error {
		calls, err := w.adapterCalls()
		if err != nil {
			return err
		}
		if want := []string{"list --at " + tree}; !slices.Equal(calls, want) {
			return fmt.Errorf("the adapter was called as %q, not %q\n%s", calls, want, w.report())
		}
		return nil
	})
	sc.Step(`^no runner command was called$`, func() error { return w.noneRan(false) })
	sc.Step(`^its JSON still preserves the extra adapter fields$`, func() error {
		if w.exit != 0 {
			return fmt.Errorf("itos exited %d\n%s", w.exit, w.report())
		}
		var report map[string]any
		if err := json.Unmarshal([]byte(w.stdout), &report); err != nil {
			return fmt.Errorf("the output is not JSON: %v\n%s", err, w.report())
		}
		if report["generator"] != "fixture-adapter" {
			return fmt.Errorf("the JSON lost the object's extra field generator\n%s", w.report())
		}
		tests, _ := report["tests"].([]any)
		if len(tests) != len(s.fixture.tests) {
			return fmt.Errorf("the JSON lists %d tests, not %d\n%s", len(tests), len(s.fixture.tests), w.report())
		}
		for _, t := range tests {
			if test, _ := t.(map[string]any); test["suite"] != "unit" {
				return fmt.Errorf("the JSON's test %v lost its extra field suite\n%s", t, w.report())
			}
		}
		return nil
	})
	sc.Step(`^the adapter with "supports_at: false" is listed as TSV at HEAD$`, func() error {
		off := false
		w.config.commandKind.supportsAt = &off
		if err := w.writeConfig(); err != nil {
			return err
		}
		return w.itos("tests", "list", "fixture", "--tsv", "--at", "HEAD")
	})
	sc.Step(`^its existing working-tree fallback warning goes to stderr$`, func() error {
		for _, part := range []string{"tests.fixture", "HEAD", "working tree"} {
			if !strings.Contains(w.stderr, part) {
				return fmt.Errorf("stderr does not say %q\n%s", part, w.report())
			}
		}
		return nil
	})
	sc.Step(`^stdout still contains only TSV rows$`, func() error {
		if w.exit != 0 {
			return fmt.Errorf("itos exited %d\n%s", w.exit, w.report())
		}
		rows, err := w.tsvRows()
		if err != nil {
			return err
		}
		for _, row := range rows {
			fields := strings.Split(row, "\t")
			if len(fields) != 2 || !fixtureID.MatchString(fields[0]) || !strings.HasPrefix(fields[1], "fixtures/") {
				return fmt.Errorf("the row %q is not an ID and a file\n%s", row, w.report())
			}
		}
		return nil
	})

	sc.Step(`^a command adapter lists one valid live row followed by (.+)$`, func(problem string) error {
		bad, ok := badRows[problem]
		if !ok {
			return fmt.Errorf("no step writes %q", problem)
		}
		row := bad.row(w)
		s.fixture = &fixtureListing{tests: []map[string]any{
			{"id": "F-001", "file": "fixtures/f-001.txt", "live": true},
			{"id": row.id, "file": row.file, "live": true},
		}}
		s.bad = bad
		return w.writeFixture(s.fixture)
	})
	sc.Step(`^stdout is empty$`, func() error {
		if w.stdout != "" {
			return fmt.Errorf("stdout is not empty\n%s", w.report())
		}
		return nil
	})
	sc.Step(`^stderr identifies the row and why it cannot be represented as project-relative TSV$`, func() error {
		for _, part := range []string{s.bad.names, s.bad.why} {
			if !strings.Contains(w.stderr, part) {
				return fmt.Errorf("stderr does not say %q\n%s", part, w.report())
			}
		}
		return nil
	})

	sc.Step(`^a kind whose valid listing contains only non-live tests$`, func() error {
		s.fixture = &fixtureListing{tests: []map[string]any{
			{"id": "F-001", "file": "fixtures/f-001.txt", "live": false},
			{"id": "F-002", "file": "fixtures/f-002.txt", "live": false},
		}}
		return w.writeFixture(s.fixture)
	})
	sc.Step(`^itos exits with code 0 and empty stdout$`, func() error {
		if w.exit != 0 || w.stdout != "" {
			return fmt.Errorf("itos exited %d, its stdout %q, not 0 and empty\n%s", w.exit, w.stdout, w.report())
		}
		calls, err := w.adapterCalls()
		s.calls = len(calls)
		return err
	})
	sc.Step(`^itos exits with code 2 before invoking the adapter$`, func() error {
		if err := w.exitsWith(2); err != nil {
			return err
		}
		calls, err := w.adapterCalls()
		if err != nil {
			return err
		}
		if len(calls) != s.calls {
			return fmt.Errorf("the adapter was called %d times by the run: %q\n%s", len(calls)-s.calls, calls[s.calls:], w.report())
		}
		return nil
	})
	sc.Step(`^its diagnostic names the mutually exclusive output formats$`, func() error {
		for _, flag := range []string{"--tsv", "--json"} {
			if !strings.Contains(w.stderr, flag) {
				return fmt.Errorf("stderr does not name %s\n%s", flag, w.report())
			}
		}
		return nil
	})
	sc.Step(`^its help explains the live-only TSV format, the fingerprint column a Gherkin kind adds, and project-relative paths$`, func() error {
		return w.helpSays("--tsv", "live", "tab", "fingerprint", "Gherkin", "project")
	})
	sc.Step(`^it explains that warnings and diagnostics stay on stderr$`, func() error {
		return w.helpSays("stderr")
	})
}

// A scenario ID of the scratch config's pattern, and one of the fixture's listings.
var (
	scenarioID = regexp.MustCompile(`^ID-[A-Z]+-\d+$`)
	fixtureID  = regexp.MustCompile(`^F-\d+$`)
)

// A row TSV cannot represent: the test's id and file, given the world, and what the diagnostic
// names it by (the field of it that is representable) and why it is refused.
type badRow struct {
	row        func(w *world) idAt
	names, why string
}

// The rows of ID-TESTLIST-05's examples, each after the valid live row F-001 in
// fixtures/f-001.txt.
var badRows = map[string]badRow{
	"a live ID containing a tab":                {fixed("F-0\t02", "fixtures/f-002.txt"), "fixtures/f-002.txt", "tab"},
	"a live file containing a carriage return":  {fixed("F-002", "fixtures/f\r002.txt"), "F-002", "carriage return"},
	"a live file containing a newline":          {fixed("F-002", "fixtures/f\n002.txt"), "F-002", "newline"},
	"a live ID containing NUL":                  {fixed("F-0\x0002", "fixtures/f-002.txt"), "fixtures/f-002.txt", "NUL"},
	"a live test with an empty ID":              {fixed("", "fixtures/f-002.txt"), "fixtures/f-002.txt", "empty"},
	"a live test with an empty file":            {fixed("F-002", ""), "F-002", "empty"},
	"a second live row with the same ID":        {fixed("F-001", "fixtures/again.txt"), "fixtures/again.txt", "same id"},
	"a live file resolving outside the project": {fixed("F-002", "../outside.txt"), "F-002", "outside the project"},
	// A file named from a root of the file system, under the scenario's support folder, which
	// is outside the repository.
	"a live file under a root outside the project": {func(w *world) idAt {
		return idAt{"F-002", filepath.Join(w.support, "elsewhere", "f-002.txt")}
	}, "F-002", "outside the project"},
}

// fixed is a bad row that is the same in every world.
func fixed(id, file string) func(*world) idAt {
	return func(*world) idAt { return idAt{id, file} }
}

// The tripwires of slice 154's scenarios: the config's shell is the recording shell, so every
// command itos starts through it (a runner, a CI step, an adapter) is logged; the hook stand-in
// records each hook it is asked to run; and the gh first on the PATH records each call.
func (w *world) armTripwires() error {
	if err := w.recordingShell(); err != nil {
		return err
	}
	standIn := filepath.Join(w.support, "fixture-hooks", "itos")
	record := "#!/bin/sh\nprintf '%%s\\n' \"$*\" >> %s\nexit 0\n"
	if err := w.writeProgram(standIn, fmt.Sprintf(record, quote(filepath.Join(w.support, "hook-log")))); err != nil {
		return err
	}
	bin, err := w.binOnPath()
	if err != nil {
		return err
	}
	return w.writeProgram(filepath.Join(bin, "gh"), fmt.Sprintf(record, quote(filepath.Join(w.support, "gh-log"))))
}

// The tripwires untouched but for an adapter listing a tree: the recording shell ran no
// command but `<adapter> list --at <tree>`, and with hooks, no hook ran and gh was not called.
func (w *world) noneRan(hooks bool) error {
	log, err := w.shellLog()
	if err != nil {
		return err
	}
	var adapters []string
	if k := w.config.commandKind; k != nil {
		adapters = append(adapters, k.command)
	}
	if w.config.adapter != nil {
		adapters = append(adapters, w.adapterCommand())
	}
	listing := func(line string) bool {
		return slices.ContainsFunc(adapters, func(a string) bool { return strings.HasPrefix(line, a+" list --at ") })
	}
	for _, line := range log {
		if !listing(line) {
			return fmt.Errorf("the shell ran %q, which lists no tests\n%s", line, w.report())
		}
	}
	calls, err := w.adapterCalls()
	if err != nil {
		return err
	}
	for _, call := range calls {
		if !strings.HasPrefix(call, "list --at ") {
			return fmt.Errorf("the adapter was called as %q\n%s", call, w.report())
		}
	}
	if !hooks {
		return nil
	}
	for _, record := range []string{"hook-log", "gh-log"} {
		text, err := os.ReadFile(filepath.Join(w.support, record))
		if err == nil {
			return fmt.Errorf("%s holds %q\n%s", record, text, w.report())
		}
		if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// A scenario appended to the feature file that holds id.
func (w *world) appendScenario(id, block string) error {
	path := filepath.Join(w.dir, "features", w.scenarioFiles[id])
	text, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(string(text)+"\n"+block), 0o644)
}

// The fixture kind, its command adapter a script that records each call's arguments in
// adapter-log and prints the listing to list, beside the scenario kind.
func (w *world) writeFixture(f *fixtureListing) error {
	listing := map[string]any{"protocol": 1, "tests": f.tests}
	files := []string{}
	for _, t := range f.tests {
		if file, _ := t["file"].(string); file != "" && !slices.Contains(files, file) {
			files = append(files, file)
		}
	}
	listing["files"] = files
	for k, v := range f.extra {
		listing[k] = v
	}
	text, err := json.Marshal(listing)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(w.support, "fixture-listing.json"), text, 0o644); err != nil {
		return err
	}
	script := fmt.Sprintf("printf '%%s\\n' \"$*\" >> %s\n[ \"$1\" = list ] && cat %s\n",
		quote(filepath.ToSlash(filepath.Join(w.support, "adapter-log"))),
		quote(filepath.ToSlash(filepath.Join(w.support, "fixture-listing.json"))))
	path := filepath.Join(w.support, "fixture-adapter.sh")
	if err := os.WriteFile(path, []byte(script), 0o644); err != nil {
		return err
	}
	if w.config.commandKind == nil {
		w.config.commandKind = &commandKind{name: "fixture", wipTag: "@wip"}
	}
	w.config.commandKind.command = "sh " + quote(filepath.ToSlash(path))
	return w.writeConfig()
}

// The fixture adapter's calls, each its arguments, oldest first.
func (w *world) adapterCalls() ([]string, error) {
	text, err := os.ReadFile(filepath.Join(w.support, "adapter-log"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	return strings.Split(strings.TrimSuffix(string(text), "\n"), "\n"), err
}

// The rows of the last run's stdout: an error when it exited otherwise than 0, or its stdout is
// not LF-terminated rows with no CR.
func (w *world) tsvRows() ([]string, error) {
	if w.exit != 0 {
		return nil, fmt.Errorf("itos exited %d\n%s", w.exit, w.report())
	}
	if w.stdout == "" {
		return nil, nil
	}
	if !strings.HasSuffix(w.stdout, "\n") || strings.Contains(w.stdout, "\r") {
		return nil, fmt.Errorf("stdout is not rows, each ending in one LF\n%s", w.report())
	}
	return strings.Split(strings.TrimSuffix(w.stdout, "\n"), "\n"), nil
}

// The JSON listing's tests of the scenario kind, run at the top beside the scenario's own runs,
// which it leaves as they were.
func (w *world) sideJSONTests() ([]struct {
	ID          string `json:"id"`
	Live        bool   `json:"live"`
	Fingerprint string `json:"fingerprint"`
}, error) {
	exit, stdout, stderr := w.exit, w.stdout, w.stderr
	defer func() { w.exit, w.stdout, w.stderr = exit, stdout, stderr }()
	if err := w.itos("tests", "list", "scenario", "--json"); err != nil {
		return nil, err
	}
	var report struct {
		Tests []struct {
			ID          string `json:"id"`
			Live        bool   `json:"live"`
			Fingerprint string `json:"fingerprint"`
		} `json:"tests"`
	}
	if err := json.Unmarshal([]byte(w.stdout), &report); err != nil {
		return nil, fmt.Errorf("the JSON listing is not JSON: %v\n%s", err, w.report())
	}
	return report.Tests, nil
}

// The JSON listing's fingerprints by ID (sideJSONTests).
func (w *world) sideFingerprints() (map[string]string, error) {
	listed, err := w.sideJSONTests()
	prints := map[string]string{}
	for _, t := range listed {
		prints[t.ID] = t.Fingerprint
	}
	return prints, err
}

// The ordinary listings of ID-TESTLIST-01's kind as they were before --tsv: the text one a
// line per test, its ID, two spaces, its file from the kind's root and "  (not live)" for a
// wip one; the JSON's keys, each test's file from the kind's root and its live flag.
func (w *world) ordinaryListings(s tsvScenario) error {
	all := append(slices.Clone(s.live), s.wip...)
	slices.Sort(all)
	report := func(r tsvRun) string { return r.report() }
	if s.text.exit != 0 {
		return fmt.Errorf("the text listing exited %d\n%s", s.text.exit, report(s.text))
	}
	lines := strings.Split(strings.TrimSuffix(s.text.stdout, "\n"), "\n")
	if len(lines) != len(all) {
		return fmt.Errorf("the text listing has %d lines, not %d\n%s", len(lines), len(all), report(s.text))
	}
	for i, line := range lines {
		id, rest, _ := strings.Cut(line, "  ")
		file, status, _ := strings.Cut(rest, "  ")
		wip := slices.Contains(s.wip, id)
		if id != all[i] || file != w.scenarioFiles[id] || (status == "(not live)") != wip || (!wip && status != "") {
			return fmt.Errorf("the text listing's line %q is not %s, its file from the kind's root and its live flag\n%s", line, all[i], report(s.text))
		}
	}
	if s.jsonRun.exit != 0 {
		return fmt.Errorf("the JSON listing exited %d\n%s", s.jsonRun.exit, report(s.jsonRun))
	}
	var listing map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s.jsonRun.stdout), &listing); err != nil {
		return fmt.Errorf("the JSON listing is not JSON: %v\n%s", err, report(s.jsonRun))
	}
	keys := []string{}
	for k := range listing {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	if want := []string{"files", "protocol", "schema", "tests"}; !slices.Equal(keys, want) {
		return fmt.Errorf("the JSON listing has the keys %v, not %v\n%s", keys, want, report(s.jsonRun))
	}
	var tests []map[string]any
	if err := json.Unmarshal(listing["tests"], &tests); err != nil {
		return err
	}
	if len(tests) != len(all) {
		return fmt.Errorf("the JSON listing has %d tests, not %d\n%s", len(tests), len(all), report(s.jsonRun))
	}
	for i, t := range tests {
		id := all[i]
		if t["id"] != id || t["file"] != w.scenarioFiles[id] || t["live"] != !slices.Contains(s.wip, id) || len(t) != 4 {
			return fmt.Errorf("the JSON listing's test %v is not %s with its file from the kind's root, its live flag and its fingerprint\n%s", t, id, report(s.jsonRun))
		}
	}
	return nil
}

func (r tsvRun) report() string {
	return fmt.Sprintf("exit %d\n--- stdout\n%s--- stderr\n%s", r.exit, r.stdout, r.stderr)
}

// The help printed says each part.
func (w *world) helpSays(parts ...string) error {
	if err := w.exitsWith(0); err != nil {
		return err
	}
	for _, part := range parts {
		if !strings.Contains(w.stdout, part) {
			return fmt.Errorf("the help does not say %q\n%s", part, w.report())
		}
	}
	return nil
}

// The configuration data, HEAD and the index: each file of itos's data where the scenario
// keeps it (the root's itos.yaml, ledger, registry and people, or the git folder's itos
// folder), HEAD's commit and the index's entries.
func (w *world) configHeadIndex() (map[string]string, error) {
	state := map[string]string{}
	if w.dataDir != "" {
		err := filepath.WalkDir(filepath.Join(w.dir, w.dataDir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			text, err := os.ReadFile(path)
			state[path] = string(text)
			return err
		})
		if err != nil {
			return nil, err
		}
	} else {
		for _, path := range []string{"itos.yaml", "people.yaml", w.ledgerPath(), startingRegistry} {
			text, err := os.ReadFile(filepath.Join(w.dir, path))
			if err != nil {
				return nil, err
			}
			state[path] = string(text)
		}
	}
	return w.gitState(state, map[string][]string{"HEAD": {"rev-parse", "HEAD"}, "the index": {"ls-files", "--stage"}})
}

// HEAD, the staged changes and the unstaged ones, untracked files included.
func (w *world) treesState() (map[string]string, error) {
	return w.gitState(map[string]string{}, map[string][]string{
		"HEAD":                 {"rev-parse", "HEAD"},
		"the staged changes":   {"diff", "--cached", "--binary"},
		"the unstaged changes": {"diff", "--binary"},
		"the untracked files":  {"ls-files", "--others", "--exclude-standard"},
	})
}

// state with each git command's output under its name.
func (w *world) gitState(state map[string]string, commands map[string][]string) (map[string]string, error) {
	for name, args := range commands {
		out, err := w.gitOutput(args...)
		if err != nil {
			return nil, err
		}
		state[name] = out
	}
	return state, nil
}

// An error naming what changed between two states.
func sameState(before, after map[string]string, report string) error {
	var changed []string
	for name, was := range before {
		if after[name] != was {
			changed = append(changed, name)
		}
	}
	for name := range after {
		if _, ok := before[name]; !ok {
			changed = append(changed, name)
		}
	}
	if len(changed) > 0 {
		slices.Sort(changed)
		return fmt.Errorf("changed: %s\n%s", strings.Join(changed, ", "), report)
	}
	return nil
}
