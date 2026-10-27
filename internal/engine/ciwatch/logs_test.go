package ciwatch

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/providers"
)

func log(lines ...string) string { return strings.Join(lines, "\n") + "\n" }

func TestCleanLogSplitsAtLFAndDropsCRTimestampsAndEscapes(t *testing.T) {
	for name, tc := range map[string]struct {
		text string
		want []string
	}{
		"nothing":                     {"", nil},
		"one LF ends the last":        {"a\nb\n", []string{"a", "b"}},
		"a second LF is a line":       {"a\n\n", []string{"a", ""}},
		"no final LF":                 {"a\nb", []string{"a", "b"}},
		"CRLF":                        {"a\r\nb\r\n", []string{"a", "b"}},
		"a CR inside is kept":         {"a\rb\n", []string{"a\rb"}},
		"timestamps":                  {"2026-10-23T10:00:00.1234567Z one\n2026-10-23T10:00:00.1234568Z \n", []string{"one", ""}},
		"no timestamp":                {"one 2026-10-23T10:00:00.1234567Z two\n", []string{"one 2026-10-23T10:00:00.1234567Z two"}},
		"a timestamp needs its space": {"2026-10-23T10:00:00Z\n", []string{"2026-10-23T10:00:00Z"}},
		"colours":                     {"2026-10-23T10:00:00Z \x1b[1;31mred\x1b[0m done\n", []string{"red done"}},
		"colours before a stamp":      {"\x1b[0m2026-10-23T10:00:00Z x\n", []string{"x"}},
		"a hyperlink (OSC)":           {"\x1b]8;;https://x.example\x07link\x1b]8;;\x07\n", []string{"link"}},
		"an OSC ended by ST":          {"\x1b]0;title\x1b\\text\n", []string{"text"}},
		"a lone ESC":                  {"a\x1b\n", []string{"a"}},
	} {
		if got := cleanLog(tc.text); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: %q, not %q", name, got, tc.want)
		}
	}
}

func fail(kind, name, pkg string, lines ...string) providers.Failure {
	if lines == nil {
		lines = []string{}
	}
	return providers.Failure{Kind: kind, Name: name, Package: pkg, Lines: lines}
}

func TestReadLogNamesAGoTestWithItsPackageAndLines(t *testing.T) {
	failures, more := readLog(log(
		"--- FAIL: TestA (0.00s)",
		"    a_test.go:1: one",
		"      indented more",
		"    a_test.go:2: two",
		"FAIL",
		"FAIL\tgithub.com/x/a\t0.012s",
		"ok  \tgithub.com/x/b\t0.020s",
		"--- FAIL: TestB (0.00s)",
		"\tb_test.go:9: tabbed",
		"FAIL\tgithub.com/x/c [build failed]",
	))
	want := []providers.Failure{
		fail("go-test", "TestA", "github.com/x/a", "a_test.go:1: one", "  indented more", "a_test.go:2: two"),
		fail("go-test", "TestB", "github.com/x/c", "b_test.go:9: tabbed"),
	}
	if more != 0 || !reflect.DeepEqual(failures, want) {
		t.Fatalf("%+v, more %d, not %+v", failures, more, want)
	}
}

func TestAGoTestsLinesEndAtTheFirstLineNoDeeper(t *testing.T) {
	for name, end := range map[string]string{
		"a sibling's FAIL":  "--- FAIL: TestB (0.00s)",
		"FAIL":              "FAIL",
		"a package line":    "FAIL\tgithub.com/x/a\t0.1s",
		"ok":                "ok  \tgithub.com/x/a\t0.1s",
		"PASS":              "PASS",
		"RUN":               "=== RUN   TestC",
		"a blank line":      "",
		"an empty log line": "",
	} {
		failures, _ := readLog(log("--- FAIL: TestA (0.00s)", "    one", "    two", end, "    not mine"))
		if len(failures) == 0 || !reflect.DeepEqual(failures[0].Lines, []string{"one", "two"}) {
			t.Errorf("%s: %+v", name, failures)
		}
	}
}

func TestAWhitespaceOnlyLineIsDeeperOnlyWhenLongerThanTheTestsIndent(t *testing.T) {
	failures, _ := readLog(log("--- FAIL: TestA (0.00s)", "    one", "        ", "    two", "FAIL"))
	if want := []string{"one", "    ", "two"}; !reflect.DeepEqual(failures[0].Lines, want) {
		t.Errorf("a long blank line: %q, not %q", failures[0].Lines, want)
	}
	failures, _ = readLog(log("--- FAIL: TestA (0.00s)", "    one", "    ", "    two", "FAIL"))
	if want := []string{"one", "", "two"}; !reflect.DeepEqual(failures[0].Lines, want) {
		t.Errorf("a blank line: %q, not %q", failures[0].Lines, want)
	}
	failures, _ = readLog(log("    --- FAIL: TestA/b (0.00s)", "        one", "    ", "        two"))
	if want := []string{"one"}; !reflect.DeepEqual(failures[0].Lines, want) {
		t.Errorf("a blank line as long as the indent ends the lines: %q, not %q", failures[0].Lines, want)
	}
}

func TestAParentWithAFailedSubtestIsNeverAFailureOfItsOwn(t *testing.T) {
	failures, _ := readLog(log(
		"--- FAIL: TestPlanOrders (0.00s)",
		"    --- FAIL: TestPlanOrders/a_task (0.00s)",
		"        plan_test.go:52: the check ran before its task",
		"    --- FAIL: TestPlanOrders/b_task (0.00s)",
		"        plan_test.go:53: b",
		"        plan_test.go:54: bb",
		"    plan_test.go:60: 1 of 3 orders failed",
		"FAIL",
		"FAIL\tgithub.com/x/plan\t0.011s",
		"--- FAIL: TestOther (0.00s)",
		"    other_test.go:1: x",
	))
	want := []providers.Failure{
		fail("go-test", "TestPlanOrders/a_task", "github.com/x/plan", "plan_test.go:52: the check ran before its task"),
		fail("go-test", "TestPlanOrders/b_task", "github.com/x/plan", "plan_test.go:53: b", "plan_test.go:54: bb"),
		fail("go-test", "TestOther", "", "other_test.go:1: x"),
	}
	if !reflect.DeepEqual(failures, want) {
		t.Fatalf("%+v, not %+v", failures, want)
	}
}

func TestAGoTestWithNoLinesOrPackageIsStillNamed(t *testing.T) {
	failures, _ := readLog(log("--- FAIL: TestA (0.00s)"))
	want := []providers.Failure{fail("go-test", "TestA", ""), {Kind: "tail", Lines: []string{"--- FAIL: TestA (0.00s)"}}}
	if !reflect.DeepEqual(failures, want) {
		t.Fatalf("%+v, not %+v", failures, want)
	}
}

// godogLog is this repository's own red feature run as go test prints it
// without -v: godog's pretty print, its summary of failed steps, then go
// test's lines, each \e an escape character.
func godogLog() string {
	return strings.ReplaceAll(log(
		`\e[1;37mFeature:\e[0m itos push waits for the CI run it started`,
		``,
		`  \e[1;37mScenario:\e[0m A push waits for its run                     \e[1;30m# watch.feature:27\e[0m`,
		`    \e[31mAnd\e[0m \e[31mits output says "ci: success"\e[0m              \e[1;30m# steps_test.go:544 -> *world\e[0m`,
		`    \e[1;31mafter scenario hook failed: the output does not say "ci: success"`,
		`exit 1`,
		`\e[0m`,
		``,
		`--- \e[31mFailed steps:\e[0m`,
		``,
		`  \e[31mScenario: A push waits for its run\e[0m \e[1;30m# watch.feature:27\e[0m`,
		`    \e[31mAnd its output says "ci: success"\e[0m \e[1;30m# watch.feature:31\e[0m`,
		`      \e[31mError: \e[0m\e[1;31mafter scenario hook failed: the output does not say "ci: success"`,
		`exit 1`,
		`--- stdout`,
		`   an indented line of the error`,
		`\e[0m`,
		``,
		`  \e[31mScenario: Another # one\e[0m \e[1;30m# watch.feature:40\e[0m`,
		`    \e[31mThen it works\e[0m \e[1;30m# watch.feature:41\e[0m`,
		`      \e[31mError: \e[0m\e[1;31mit does not`,
		`\e[0m`,
		``,
		``,
		`2 scenarios (\e[31m2 failed\e[0m)`,
		`--- FAIL: TestFeatures (0.50s)`,
		`    --- FAIL: TestFeatures/A_push_waits_for_its_run (0.46s)`,
		`        suite.go:640: after scenario hook failed`,
		`    --- FAIL: TestFeatures/Another_#_one (0.46s)`,
		`        suite.go:640: it does not`,
		`    features_test.go:232: a scenario failed`,
		`FAIL`,
		`FAIL	github.com/x/features	0.519s`,
	), `\e`, "\x1b")
}

func TestReadLogNamesAGodogScenarioAndMergesItsGoTestSubtest(t *testing.T) {
	failures, more := readLog(godogLog())
	want := []providers.Failure{
		fail("scenario", "A push waits for its run", "",
			`And its output says "ci: success" # watch.feature:31`,
			`Error: after scenario hook failed: the output does not say "ci: success"`,
			"exit 1", "--- stdout", "   an indented line of the error"),
		fail("scenario", "Another # one", "", "Then it works # watch.feature:41", "Error: it does not"),
	}
	if more != 0 || !reflect.DeepEqual(failures, want) {
		t.Fatalf("%+v, more %d, not %+v", failures, more, want)
	}
}

func TestOnlyTheSummarysScenarioLinesAreFailures(t *testing.T) {
	failures, _ := readLog(log(
		"  Scenario: Not failed # f.feature:1",
		"--- Failed steps:",
		"",
		"  Scenario: Failed # f.feature:2",
		"    Step # f.feature:3",
		"      Error: x",
		"",
		"1 scenarios (1 failed)",
		"  Scenario: After the summary's end # f.feature:4",
	))
	want := []providers.Failure{fail("scenario", "Failed", "", "Step # f.feature:3", "Error: x")}
	if !reflect.DeepEqual(failures, want) {
		t.Fatalf("%+v, not %+v", failures, want)
	}
}

func TestAScenariosFirstTwoLinesLoseTheirIndentationAndTheRestKeepIt(t *testing.T) {
	failures, _ := readLog(log("--- Failed steps:", "", "  Scenario: S # f:1", "    step", "\t  Error: e", "   third", "  \tfourth"))
	if want := []string{"step", "Error: e", "   third", "  \tfourth"}; !reflect.DeepEqual(failures[0].Lines, want) {
		t.Fatalf("%q, not %q", failures[0].Lines, want)
	}
}

func TestAGoTestFailureOfAScenarioIsTheSameFailureWhateverItsNameIsRewrittenTo(t *testing.T) {
	summary := func(name string) []string {
		return []string{"--- Failed steps:", "", "  Scenario: " + name + " # f:1", "    step", "      Error: e", ""}
	}
	for name, tc := range map[string]struct {
		scenario, test string
		merged         bool
	}{
		"spaces":        {"a b c", "TestFeatures/a_b_c", true},
		"tabs":          {"a\tb", "TestFeatures/a_b", true},
		"a duplicate":   {"a b", "TestFeatures/a_b#01", true},
		"unicode space": {"a" + string(rune(0xa0)) + "b", "TestFeatures/a_b", true},
		"not printable": {"a" + string(rune(0x200b)) + "b", "TestFeatures/a" + "\\" + "u200b" + "b", true},
		"unicode":       {"é ü", "TestFeatures/é_ü", true},
		"another":       {"a b", "TestFeatures/a_c", false},
		"unrewritten":   {"a b", "TestFeatures/a b", false},
		"not the suite": {"a b", "TestOther/a_b", false},
		"the suite":     {"a b", "TestFeatures", false},
	} {
		lines := append(summary(tc.scenario), "--- FAIL: "+tc.test+" (0.00s)", "    suite.go:1: x", "FAIL")
		failures, _ := readLog(log(lines...))
		kinds := ""
		for _, f := range failures {
			kinds += f.Kind + " "
		}
		if tc.merged != (kinds == "scenario ") {
			t.Errorf("%s: %s", name, kinds)
		}
	}
}

func TestReadLogNamesAnITOSCIRunFailureWithItsLines(t *testing.T) {
	for name, tc := range map[string]struct {
		log   []string
		names []string
		lines [][]string
	}{
		"the code proof": {
			[]string{"CI failed at the code proof: itos-cc mutation run", "mutation.survived: a.go:1 in f", "  fix: kill it", "", "after"},
			[]string{"the code proof: itos-cc mutation run"}, [][]string{{"mutation.survived: a.go:1 in f", "  fix: kill it"}},
		},
		"also failed at": {
			[]string{"CI failed at: one (it exited 1)", "detail one", "also failed at: two (it exited 2)", "detail two", "##[group]x", "not mine"},
			[]string{"one (it exited 1)", "two (it exited 2)"}, [][]string{{"detail one"}, {"detail two"}},
		},
		"a go test line ends it": {
			[]string{"CI failed at: one", "detail one", "--- FAIL: TestX (0.00s)", "    x_test.go:1: m"},
			[]string{"one", "TestX"}, [][]string{{"detail one"}, {"x_test.go:1: m"}},
		},
		"the shared indentation goes": {
			[]string{"CI failed at: one", "    a", "      b", "    c", ""},
			[]string{"one"}, [][]string{{"a", "  b", "c"}},
		},
		"the first line is the margin": {
			[]string{"CI failed at: one", "a", "  b"},
			[]string{"one"}, [][]string{{"a", "  b"}},
		},
	} {
		failures, _ := readLog(log(tc.log...))
		var names []string
		var lines [][]string
		for _, f := range failures {
			names, lines = append(names, f.Name), append(lines, f.Lines)
		}
		if !reflect.DeepEqual(names, tc.names) || !reflect.DeepEqual(lines, tc.lines) {
			t.Errorf("%s: %q %q, not %q %q", name, names, lines, tc.names, tc.lines)
		}
	}
}

func TestACIRunFailureWithNoLinesBringsTheTailToo(t *testing.T) {
	failures, _ := readLog(log("$ go vet ./...", "", "CI failed at: go vet ./... (it exited 1)", "##[error]Process completed with exit code 1."))
	want := []providers.Failure{
		fail("ci-run", "go vet ./... (it exited 1)", ""),
		{Kind: "tail", Lines: []string{"$ go vet ./...", "", "CI failed at: go vet ./... (it exited 1)", "##[error]Process completed with exit code 1."}},
	}
	if !reflect.DeepEqual(failures, want) {
		t.Fatalf("%+v, not %+v", failures, want)
	}
}

func TestATailIsGivenWhenNoFailureHasLinesOfItsOwn(t *testing.T) {
	var lines []string
	for n := 1; n <= 30; n++ {
		lines = append(lines, fmt.Sprintf("line %d", n))
	}
	failures, _ := readLog(log(append(lines, "##[error]Process completed with exit code 2.", "Post job cleanup.")...))
	want := append(append([]string{}, lines[11:]...), "##[error]Process completed with exit code 2.")
	if len(failures) != 1 || failures[0].Kind != "tail" || failures[0].Name != "" || failures[0].MoreLines != 11 ||
		!reflect.DeepEqual(failures[0].Lines, want) || len(want) != 20 {
		t.Fatalf("%+v", failures)
	}
}

func TestATailIsTheWholeLogUpToTheErrorLineWhenItIsShort(t *testing.T) {
	failures, _ := readLog(log("a", "##[error]x", "##[error]y", "b"))
	if want := []providers.Failure{{Kind: "tail", Lines: []string{"a", "##[error]x"}}}; !reflect.DeepEqual(failures, want) {
		t.Fatalf("%+v", failures)
	}
	failures, _ = readLog(log("##[error]x", "b"))
	if want := []providers.Failure{{Kind: "tail", Lines: []string{"##[error]x"}}}; !reflect.DeepEqual(failures, want) {
		t.Fatalf("an error on the first line: %+v", failures)
	}
}

func TestATailWithNoErrorLineIsTheLogsLast20(t *testing.T) {
	var lines []string
	for n := 1; n <= 21; n++ {
		lines = append(lines, fmt.Sprintf("line %d", n))
	}
	failures, _ := readLog(log(lines...))
	if len(failures) != 1 || len(failures[0].Lines) != 20 || failures[0].Lines[0] != "line 2" || failures[0].MoreLines != 1 {
		t.Fatalf("%+v", failures)
	}
	failures, _ = readLog(log(lines[:20]...))
	if len(failures) != 1 || len(failures[0].Lines) != 20 || failures[0].Lines[0] != "line 1" || failures[0].MoreLines != 0 {
		t.Fatalf("a log of 20 lines: %+v", failures)
	}
}

func TestAnEmptyLogNamesNothing(t *testing.T) {
	if failures, more := readLog(""); len(failures) != 0 || more != 0 {
		t.Fatalf("%+v, %d", failures, more)
	}
}

func TestAFailureWithLinesOfItsOwnBringsNoTail(t *testing.T) {
	failures, _ := readLog(log("CI failed at: a", "CI failed at: b", "detail", "other"))
	if len(failures) != 2 || failures[1].Kind != "ci-run" {
		t.Fatalf("%+v", failures)
	}
}

func TestAFailureKeepsTenLinesAndCountsTheRest(t *testing.T) {
	for n, more := range map[int]int{9: 0, 10: 0, 11: 1, 14: 4} {
		lines := []string{"--- FAIL: TestA (0.00s)"}
		for i := 1; i <= n; i++ {
			lines = append(lines, fmt.Sprintf("    line %d", i))
		}
		failures, _ := readLog(log(lines...))
		if len(failures) != 1 || len(failures[0].Lines) != min(n, 10) || failures[0].MoreLines != more {
			t.Errorf("%d lines: %+v", n, failures)
		}
	}
}

func TestAJobKeepsTwentyFailuresAndCountsTheRest(t *testing.T) {
	for n, more := range map[int]int{19: 0, 20: 0, 21: 1, 25: 5} {
		var lines []string
		for i := 1; i <= n; i++ {
			lines = append(lines, fmt.Sprintf("--- FAIL: Test%d (0.00s)", i), "    x")
		}
		failures, got := readLog(log(lines...))
		if len(failures) != min(n, 20) || got != more || failures[0].Name != "Test1" {
			t.Errorf("%d failures: %d kept, %d more", n, len(failures), got)
		}
	}
}

func TestAFailureRightAfterTheSummaryOfFailedStepsIsStillNamed(t *testing.T) {
	failures, _ := readLog(log("--- Failed steps:", "", "  Scenario: S # f:1", "    step", "      Error: e", "", "CI failed at: x", "detail"))
	want := []providers.Failure{fail("scenario", "S", "", "step", "Error: e"), fail("ci-run", "x", "", "detail")}
	if !reflect.DeepEqual(failures, want) {
		t.Fatalf("%+v, not %+v", failures, want)
	}
	failures, _ = readLog(log("--- Failed steps:", "", "  Scenario: S # f:1", "    step"))
	if want := []providers.Failure{fail("scenario", "S", "", "step")}; !reflect.DeepEqual(failures, want) {
		t.Fatalf("a log that ends in the entry: %+v, not %+v", failures, want)
	}
}

func TestACIRunFailuresLinesLoseOnlyTheIndentationTheyShare(t *testing.T) {
	failures, _ := readLog(log("CI failed at: x", "    a", "  ", "    b"))
	if want := []string{"a", "", "b"}; !reflect.DeepEqual(failures[0].Lines, want) {
		t.Fatalf("%q, not %q", failures[0].Lines, want)
	}
}

func TestReadAnnotationsKeepsFailuresInTheOrderListed(t *testing.T) {
	failures, more := readAnnotations([]providers.Annotation{
		{Path: "a_test.go", Line: 7, Level: "failure", Message: "one\r\ntwo\n"},
		{Path: ".github", Line: 1, Level: "warning", Message: "deprecated"},
		{Path: "x", Line: 2, Level: "notice", Message: "fyi"},
		{Path: ".github", Line: 1, Level: "failure", Message: ""},
	})
	want := []providers.Failure{
		{Kind: "annotation", Name: "a_test.go:7", Level: "failure", Lines: []string{"one", "two"}},
		{Kind: "annotation", Name: ".github:1", Level: "failure", Lines: []string{}},
	}
	if more != 0 || !reflect.DeepEqual(failures, want) {
		t.Fatalf("%+v, more %d", failures, more)
	}
	if failures, more := readAnnotations(nil); failures != nil || more != 0 {
		t.Fatalf("no annotations: %+v, %d", failures, more)
	}
}

func TestReadAnnotationsBoundsThemAsALogsFailuresAre(t *testing.T) {
	var annotations []providers.Annotation
	for n := 1; n <= 23; n++ {
		annotations = append(annotations,
			providers.Annotation{Path: "p", Line: n, Level: "failure", Message: strings.Repeat("l\n", 12)},
			providers.Annotation{Path: "p", Line: n, Level: "warning", Message: "w"})
	}
	failures, more := readAnnotations(annotations)
	if len(failures) != 20 || more != 3 {
		t.Fatalf("%d failures, more %d", len(failures), more)
	}
	if f := failures[19]; f.Name != "p:20" || len(f.Lines) != 10 || f.MoreLines != 2 {
		t.Fatalf("%+v", f)
	}
}
