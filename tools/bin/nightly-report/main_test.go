package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

// The testdata are excerpts of the "nightly" job's log of three runs, as the API gives it
// (timestamps and colours kept): the plan's step lines, the mutation steps' progress and their
// itos-cc reports, the failure lines itos ci run interleaved into those reports where they fell,
// and the job's end. Each report keeps its ok, problems, sampling and schema keys, and of its
// subjects the lines around the interleaved failure line; its selected and stages keys, thousands
// of lines, are left out.
func excerpt(t *testing.T, run string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + run + ".txt")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

const (
	random  = "`tools/bin/pinned itos-cc mutation run --count 20 --seed \"${GITHUB_RUN_ID:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}\" --json cmd internal`"
	changed = "`base=$(go run ./tools/bin/nightly-base) && files=$(go run ./tools/bin/nightly-base -linux-files cmd internal) && tools/bin/pinned itos-cc mutation run --since \"$base\" --count 60 --seed \"${GITHUB_RUN_ID:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}\" --fail-uncovered=lines --json $files`"
)

// Run 38896023548: both mutation steps failed on survivors and the plan ran to its end; each
// failure line fell inside the report its step printed.
func TestBothMutationStepsFailed(t *testing.T) {
	want := "Failed at:\n\n" +
		"- " + random + ": it exited 1.\n" +
		"  - survived internal/config/stealth.go:53:27 `||` → `&&` in `config#Locate`\n" +
		"  - survived internal/cli/initshim.go:66:7 `+` → `-` in `cli#parseShimFlag`\n" +
		"- " + changed + ": it exited 1.\n" +
		"  - survived internal/cli/watch.go:225:11 `>` → `>=` in `cli#watchCovered`\n" +
		"  - survived internal/cli/workwrite.go:510:14 `false` → `true` in `cli#workPerson`\n" +
		"  - survived internal/cli/status.go:592:11 `!=` → `==` in `cli.standing#readCI`\n" +
		"  - survived internal/cli/status.go:586:7 `!` → (deleted) in `cli.standing#readCI`\n" +
		"\nNo failing scenario in its log.\n"
	if got := read(excerpt(t, "38896023548")).render(); got != want {
		t.Errorf("the body says:\n%s\nwant:\n%s", got, want)
	}
}

// Run 39029889396: the random step failed on survivors, then the runner received a shutdown
// signal during the changed-sites step, whose report, interrupted, still names what it found, and
// the steps after it never ran. That report is itos-cc 0.12.0's, whose count.interrupted later
// releases replaced with interrupted (TestAnInterruptedReport), so it names no interruption.
func TestTheRunnerShutDownDuringAStep(t *testing.T) {
	want := "Failed at:\n\n" +
		"- " + random + ": it exited 1.\n" +
		"  - survived internal/cli/follow.go:222:44 `!=` → `==` in `cli#followAdd`\n" +
		"  - survived internal/cli/push.go:342:20 `||` → `&&` in `cli.pushRun#rebase`\n" +
		"  - survived internal/work/defer.go:60:16 `||` → `&&` in `work#Defer`\n" +
		"\nThe runner received a shutdown signal during the plan's step " + changed + ": the steps after it did not run.\n" +
		"\nWhat it found before it stopped:\n\n" +
		"- uncovered internal/message/header.go:138:19 `0` → `1` in `message#gluedAt`\n" +
		"- survived internal/cli/watch.go:212:14 `0` → `1` in `cli#watchCovered`\n" +
		"- survived internal/cli/hook.go:383:46 `!` → (deleted) in `cli#stagedRule`\n" +
		"\nNo failing scenario in its log.\n"
	if got := read(excerpt(t, "39029889396")).render(); got != want {
		t.Errorf("the body says:\n%s\nwant:\n%s", got, want)
	}
}

// Run 39817143873, under itos-cc 0.26.0: two steps failed before the mutation steps, the second a
// command of several lines whose failure the next step's blank line split, so the random step's
// "$ " line follows that failure's last line and the changed-sites step's follows its failure
// line, neither after a blank one. The changed-sites report names unexecuted blocks
// (mutation.uncovered-statement) by their line alone. Each report keeps two of its problems, the
// changed-sites one two of each rule.
func TestItosCC026Reports(t *testing.T) {
	const latestCI = "`id=$(gh run list --workflow ci.yml --branch main --status completed --limit 1 --json databaseId --jq '.[0].databaseId') && gh run view \"$id\" --json jobs --jq '.jobs as $jobs | [\"ci (ubuntu-latest)\", \"platform (macos-latest)\", \"platform (windows-latest)\"] as $names | def passed($name): any(.steps[]?; .name == $name and .conclusion == \"success\"); ($names | all(. as $name | [$jobs[] | select(.name == $name)] | length == 1 and all(.conclusion == \"success\"))) and ([$jobs[] | select(.name as $name | $names | index($name))] |` …"
	const changed026 = "`base=$(go run ./tools/bin/nightly-base) && tools/bin/pinned itos-cc mutation run --since \"$base\" --count 60 --seed \"${GITHUB_RUN_ID:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}\" --fail-uncovered=lines --json cmd internal`"
	want := "Failed at:\n\n" +
		"- `ITOS_SELFTEST=1 go test -count=1 -run '^TestSelftest$' ./tools/selftest/gates`: it exited 1.\n" +
		"- " + latestCI + ": it exited 1.\n" +
		"- `tools/bin/pinned itos-cc mutation run --count 20 --seed \"${GITHUB_RUN_ID:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}\" --json cmd internal`: it exited 1.\n" +
		"  - survived internal/value/doc.go:132:14 `||` → `&&` in `value.Doc#Set`\n" +
		"  - survived internal/message/trailers.go:69:77 `0` → `1` in `message#trailerLine`\n" +
		"- " + changed026 + ": it exited 1.\n" +
		"  - survived internal/message/trailers.go:69:77 `0` → `1` in `message#trailerLine`\n" +
		"  - survived internal/value/rename.go:264:50 `0` → `1` in `value.Doc#insert`\n" +
		"  - uncovered-statement internal/cli/workwrite.go:59 in `cli#soundRegistry`\n" +
		"  - uncovered-statement internal/store/tree/adapter/tree.go:69 in `adapter.Trees#At`\n" +
		"\nNo failing scenario in its log.\n"
	if got := read(excerpt(t, "39817143873")).render(); got != want {
		t.Errorf("the body says:\n%s\nwant:\n%s", got, want)
	}
}

// itos-cc 0.26.0's report of a counted run a signal stopped: the rule interrupted, exit 130, and
// where it stopped in its sampling's stop, which the body gives rather than the problem's message.
// A report with no stop, as a complete run's, says only that it was interrupted.
func TestAnInterruptedReport(t *testing.T) {
	survived := `{"rule": "mutation.survived", "file": "internal/cli/watch.go", "line": 212, "column": 14, "original": "0", "replacement": "1", "function": "github.com/donvargax/itos/v8/internal/cli#watchCovered", "message": "survived", "exit": 1}`
	interrupted := `{"rule": "interrupted", "message": "interrupted before it finished", "fix": "Run it again and let it finish.", "exit": 130}`
	for name, c := range map[string]struct{ sampling, want string }{
		"counted":  {`, "sampling": {"completion": "interrupted", "stop": "interrupted while selected sites were judged"}`, "  - interrupted: interrupted while selected sites were judged\n"},
		"complete": {"", "  - interrupted\n"},
	} {
		t.Run(name, func(t *testing.T) {
			got := read(log(
				"", "$ tools/bin/pinned itos-cc mutation run --count 60 --json cmd internal",
				"{", `  "ok": false,`, `  "problems": [`+survived+", "+interrupted+"]"+c.sampling+",", `  "schema": 1`, "}",
				"", "CI failed at: tools/bin/pinned itos-cc mutation run --count 60 --json cmd internal (it exited 130)",
			)).render()
			want := "Failed at:\n\n- `tools/bin/pinned itos-cc mutation run --count 60 --json cmd internal`: it exited 130.\n" +
				"  - survived internal/cli/watch.go:212:14 `0` → `1` in `cli#watchCovered`\n" + c.want +
				"\nNo failing scenario in its log.\n"
			if got != want {
				t.Errorf("the body says:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

// log is lines as the API gives them: each after a timestamp, with carriage returns.
func log(lines ...string) string {
	var b strings.Builder
	for i, l := range lines {
		fmt.Fprintf(&b, "2026-10-17T11:%02d:00.0000000Z %s\r\n", i%60, l)
	}
	return b.String()
}

func TestFailingScenarios(t *testing.T) {
	a := read(log(
		"##[group]Run go test ./features -count=1",
		"\x1b[36;1mgo test ./features -count=1\x1b[0m",
		"    --- FAIL: TestFeatures/ID-CI-29 (0.20s)",
		"echo '--- FAIL: TestFeatures/not-a-scenario'",
		"--- FAIL: TestFeatures (3.10s)",
		"    --- FAIL: TestFeatures/ID-CI-30 (0.10s)",
		"##[error]Process completed with exit code 1.",
	))
	want := "Failing scenarios:\n\n```\n    --- FAIL: TestFeatures/ID-CI-29 (0.20s)\n    --- FAIL: TestFeatures/ID-CI-30 (0.10s)\n```\n"
	if got := a.render(); got != want {
		t.Errorf("the body says:\n%s\nwant:\n%s", got, want)
	}
}

// The fuzz step's failing inputs (tools/bin/nightly-fuzz): each path named in the order the step
// printed it, the bytes after a path left in the log; the step's failure beside them.
func TestFailingFuzzInputs(t *testing.T) {
	a := read(log(
		"",
		"$ tools/bin/nightly-fuzz",
		"fuzz: example.com/m/internal/message FuzzTrailersMatchGit for 60s",
		"    Failing input written to testdata/fuzz/FuzzTrailersMatchGit/0a1757c4b4fc9a9c",
		"fuzz input: internal/message/testdata/fuzz/FuzzTrailersMatchGit/0a1757c4b4fc9a9c",
		"go test fuzz v1",
		`string("0000")`,
		"fuzz: example.com/m/internal/other FuzzOther for 60s",
		"fuzz input: internal/other/testdata/fuzz/FuzzOther/77",
		"go test fuzz v1",
		"",
		"CI failed at: tools/bin/nightly-fuzz (it exited 1)",
	))
	want := "Failed at:\n\n- `tools/bin/nightly-fuzz`: it exited 1.\n\n" +
		"Failing fuzz inputs, their bytes in the log; keep each as a seed at its path:\n\n" +
		"- `internal/message/testdata/fuzz/FuzzTrailersMatchGit/0a1757c4b4fc9a9c`\n" +
		"- `internal/other/testdata/fuzz/FuzzOther/77`\n\n" +
		"No failing scenario in its log.\n"
	if got := a.render(); got != want {
		t.Errorf("the body says:\n%s\nwant:\n%s", got, want)
	}
}

// A step whose command spans lines: itos ci run names its first line after the lead and its exit
// code at the end of its last.
func TestAStepOfSeveralLinesFailed(t *testing.T) {
	a := read(log(
		"",
		"$ id=$(gh run list) && gh run view \"$id\" --jq '.jobs",
		"  | length' | grep -qx true",
		"",
		"also failed at: id=$(gh run list) && gh run view \"$id\" --jq '.jobs",
		"  | length' | grep -qx true (it exited 1)",
		"",
		"$ go run ./tools/bin/verify-release",
		"",
		"also failed at: go run ./tools/bin/verify-release (it gave no exit code)",
		"",
		"also failed at T-002 (a task), its check: exit 5",
		"",
		"also failed at: gh run view \"$id\" --jq '.jobs",
		"  | length' | grep -qx true",
	))
	want := "Failed at:\n\n" +
		"- `id=$(gh run list) && gh run view \"$id\" --jq '.jobs` …: it exited 1.\n" +
		"- `go run ./tools/bin/verify-release`: it gave no exit code.\n" +
		"- T-002 (a task), its check: exit 5\n" +
		"- `gh run view \"$id\" --jq '.jobs` …: its exit code is not in the log.\n" +
		"\nNo failing scenario in its log.\n"
	if got := a.render(); got != want {
		t.Errorf("the body says:\n%s\nwant:\n%s", got, want)
	}
}

func TestAMutationStepsReportIsUnreadable(t *testing.T) {
	for name, output := range map[string][]string{
		"none":     {"itos-cc: coverage failed"},
		"not JSON": {"{", "  \"ok\": false,", "}"},
	} {
		t.Run(name, func(t *testing.T) {
			lines := append([]string{"", "$ tools/bin/pinned itos-cc mutation run --json cmd"}, output...)
			lines = append(lines, "", "CI failed at: tools/bin/pinned itos-cc mutation run --json cmd (it exited 2)")
			want := "Failed at:\n\n- `tools/bin/pinned itos-cc mutation run --json cmd`: it exited 2.\n" +
				"  - Its itos-cc report could not be read from the log.\n\nNo failing scenario in its log.\n"
			if got := read(log(lines...)).render(); got != want {
				t.Errorf("the body says:\n%s\nwant:\n%s", got, want)
			}
		})
	}
	// A failure line naming a step whose "$ " line is not in the log.
	got := read(log("CI failed at: tools/bin/pinned itos-cc mutation run --json cmd (it exited 1)")).render()
	if !strings.Contains(got, "could not be read") {
		t.Errorf("the body says:\n%s", got)
	}
}

func TestAMutationReportWithNoSite(t *testing.T) {
	got := read(log(
		"", "$ tools/bin/pinned itos-cc mutation run --json cmd",
		"{", `  "ok": false,`, `  "problems": [{"rule": "coverage.failed", "message": "no coverage"}]`, "}",
		"", "CI failed at: tools/bin/pinned itos-cc mutation run --json cmd (it exited 1)",
	)).render()
	if !strings.Contains(got, "\n  - Its itos-cc report names no survived or uncovered site.\n") {
		t.Errorf("the body says:\n%s", got)
	}
}

// Cancelled outside the plan: the last of the job's steps that started is named.
func TestCancelledBeforeThePlan(t *testing.T) {
	got := read(log(
		"##[group]Run actions/checkout@v4",
		"##[endgroup]",
		"##[group]Run tools/bin/itos version",
		"##[endgroup]",
		"##[error]The operation was canceled.",
	)).render()
	want := "The operation was cancelled during the job's step `tools/bin/itos version`: the steps after it did not run.\n\nNo failing scenario in its log.\n"
	if got != want {
		t.Errorf("the body says:\n%s\nwant:\n%s", got, want)
	}
	if got := read(log("##[error]The operation was canceled.")).render(); !strings.HasPrefix(got, "The operation was cancelled before any step started.\n") {
		t.Errorf("the body says:\n%s", got)
	}
}

// A log's escape sequences, CSI (colours) and OSC (hyperlinks, ended by ST or BEL), reach neither
// what is matched nor the body.
func TestEscapeSequences(t *testing.T) {
	got := read(log(
		"",
		"$ \x1b[1;32mgo run ./tools/bin/verify-release\x1b[0m",
		"\x1b[31m    --- FAIL: TestFeatures/\x1b]8;;https://example.com/ID-CI-29\x1b\\ID-CI-29\x1b]8;;\x1b\\ (0.20s)\x1b[0m",
		"",
		"CI failed at: \x1b]8;;https://example.com\x07go run ./tools/bin/verify-release\x1b]8;;\x07 \x1b[2K(it exited 1)",
	)).render()
	want := "Failed at:\n\n- `go run ./tools/bin/verify-release`: it exited 1.\n\n" +
		"Failing scenarios:\n\n```\n    --- FAIL: TestFeatures/ID-CI-29 (0.20s)\n```\n"
	if got != want {
		t.Errorf("the body says:\n%q\nwant:\n%q", got, want)
	}
}

func TestNothingFound(t *testing.T) {
	got := read(log("##[group]Run ITOS_SELFTEST=1 go test ./tools/selftest/gates", "--- FAIL: TestSelftest", "##[error]Process completed with exit code 1.")).render()
	if got != "No failing scenario in its log: it failed before the features ran, or outside them. See its log.\n" {
		t.Errorf("the body says:\n%s", got)
	}
}

func TestCode(t *testing.T) {
	for s, want := range map[string]string{"a": "`a`", "a`b": "``a`b``", "`a": "`` `a ``"} {
		if got := code(s, false); got != want {
			t.Errorf("code(%q) = %q, want %q", s, got, want)
		}
	}
}

// fake answers gh api for the attempt's jobs and their logs.
type fake struct {
	jobs    string
	jobsErr error
	logs    map[string]string // a job id's log; one not here fails
	calls   []string
}

func (f *fake) run(name string, args ...string) (string, error) {
	call := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, call)
	if call == "gh api --paginate repos/donvargax/itos/actions/runs/7/attempts/2/jobs" {
		return f.jobs, f.jobsErr
	}
	for id, text := range f.logs {
		if call == "gh api --allow-escape-sequences repos/donvargax/itos/actions/jobs/"+id+"/logs" {
			return text, nil
		}
	}
	return "", errors.New("gh: exit status 1: gh: Not Found (HTTP 404)\n{\"message\":\"Not Found\"}")
}

var args = []string{"-repo", "donvargax/itos", "-run", "7", "-attempt", "2", "-url", "https://github.com/donvargax/itos/actions/runs/7"}

func TestRun(t *testing.T) {
	f := &fake{
		// Two pages, as gh --paginate prints them: the report job, still running, has no conclusion.
		jobs: `{"total_count":4,"jobs":[{"id":1,"name":"nightly","conclusion":"failure"},{"id":2,"name":"platform (macos-latest)","conclusion":"success"}]}` +
			`{"total_count":4,"jobs":[{"id":3,"name":"platform (windows-latest)","conclusion":"cancelled"},{"id":4,"name":"report","conclusion":null},{"id":5,"name":"skipped","conclusion":"skipped"}]}`,
		logs: map[string]string{"1": excerpt(t, "38896023548")},
	}
	var stdout, stderr strings.Builder
	if code := run(args, f.run, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	want := "Nightly run https://github.com/donvargax/itos/actions/runs/7 is red.\n\n### nightly: failure\n\n" +
		read(excerpt(t, "38896023548")).render() +
		"\n### platform (windows-latest): cancelled\n\nIts log could not be read: see the run.\n\n" +
		"`gh: exit status 1: gh: Not Found (HTTP 404)`\n"
	if stdout.String() != want {
		t.Errorf("the body says:\n%s\nwant:\n%s", stdout.String(), want)
	}
	// Only the log fetch passes --allow-escape-sequences: the jobs list is JSON.
	for _, call := range f.calls {
		if strings.HasSuffix(call, "/logs") != strings.Contains(call, " --allow-escape-sequences ") {
			t.Errorf("gh was called as %q", call)
		}
	}
	// The whole error, gh's every line, goes to stderr.
	if !strings.Contains(stderr.String(), "job platform (windows-latest) (3)") || !strings.Contains(stderr.String(), `{"message":"Not Found"}`) {
		t.Errorf("stderr says:\n%s", stderr.String())
	}
}

func TestTheJobsCannotBeRead(t *testing.T) {
	for name, c := range map[string]struct {
		f   *fake
		why string // what the body gives under the sentence; nothing when no call failed
	}{
		"gh fails": {&fake{jobsErr: errors.New("gh: exit status 1: gh: HTTP 403\nmore")}, "\n`gh: exit status 1: gh: HTTP 403`\n"},
		"not JSON": {&fake{jobs: "<html>"}, "\n`invalid character '<' looking for beginning of value`\n"},
		"none red": {&fake{jobs: `{"jobs":[{"id":1,"name":"nightly","conclusion":"success"}]}`}, ""},
	} {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr strings.Builder
			if code := run(args, c.f.run, &stdout, &stderr); code != 0 {
				t.Fatalf("exit %d: %s", code, stderr.String())
			}
			want := "Nightly run https://github.com/donvargax/itos/actions/runs/7 is red.\n\nIts jobs could not be read: see the run's log.\n" + c.why
			if stdout.String() != want {
				t.Errorf("the body says:\n%s", stdout.String())
			}
			if (c.why == "") != (stderr.Len() == 0) {
				t.Errorf("stderr says: %q", stderr.String())
			}
		})
	}
}

func TestUsage(t *testing.T) {
	for _, bad := range [][]string{nil, args[:6], append(append([]string{}, args...), "extra"), {"-nope"}} {
		var stdout, stderr strings.Builder
		if code := run(bad, (&fake{}).run, &stdout, &stderr); code != 2 || stdout.Len() > 0 {
			t.Errorf("%v: exit %d, printed %q", bad, code, stdout.String())
		}
	}
}
