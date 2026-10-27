package ciwatch

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/providers"
	"github.com/donvargax/itos/v8/internal/vcs/port"
	"github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// remoteTags is a repository whose remote answers git ls-remote with the refs
// it is given, and records what it was asked.
type remoteTags struct {
	porttest.VCS
	refs   map[string]string
	err    error
	remote string
	asked  []string
	talk   port.Talk
}

func (r *remoteTags) RemoteRefs(_ context.Context, remote string, patterns []string, talk port.Talk) (map[string]string, error) {
	r.remote, r.asked, r.talk = remote, patterns, talk
	return r.refs, r.err
}

func TestTagsAtAreTheRemotesTagsAtTheCommit(t *testing.T) {
	const head = "1111111111111111111111111111111111111111"
	for name, tc := range map[string]struct {
		refs map[string]string
		want []string
	}{
		"none":              {map[string]string{}, nil},
		"a lightweight tag": {map[string]string{"refs/tags/v1.0.0": head}, []string{"v1.0.0"}},
		"an annotated tag, by its commit": {map[string]string{
			"refs/tags/v1.3.0": "aaaa", "refs/tags/v1.3.0^{}": head,
		}, []string{"v1.3.0"}},
		"an annotated tag at another commit": {map[string]string{
			"refs/tags/v1.2.0": "bbbb", "refs/tags/v1.2.0^{}": "2222",
		}, nil},
		"a tag object that is the head": {map[string]string{
			"refs/tags/odd": head, "refs/tags/odd^{}": "2222",
		}, nil},
		"older tags": {map[string]string{"refs/tags/v1.2.0": "2222", "refs/tags/v1.3.0": head}, []string{"v1.3.0"}},
		"in byte order": {map[string]string{
			"refs/tags/v2": head, "refs/tags/v10": head, "refs/tags/a": head,
		}, []string{"a", "v10", "v2"}},
		"a branch at the head is no tag": {map[string]string{"refs/heads/main": head, "HEAD": head}, nil},
		"a peeled ref of a branch":       {map[string]string{"refs/heads/x^{}": head}, nil},
	} {
		for range 20 { // map order is random: the peeled commit wins whatever it is
			v := &remoteTags{refs: tc.refs}
			got := tagsAt(context.Background(), v, "origin", head)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("%s: %q, not %q", name, got, tc.want)
			}
			if v.remote != "origin" || !reflect.DeepEqual(v.asked, []string{"refs/tags/*"}) || !v.talk.NoPrompt {
				t.Fatalf("%s: asked %s for %q, %+v", name, v.remote, v.asked, v.talk)
			}
		}
	}
	if got := tagsAt(context.Background(), &remoteTags{err: errors.New("down")}, "origin", head); got != nil {
		t.Errorf("a remote that cannot be read: %q", got)
	}
}

func failedRun() providers.Run {
	return providers.Run{URL: "u", Status: "completed", Conclusion: "failure", HeadSHA: "abc", Jobs: []providers.Job{
		{Name: "ci", Status: "completed", Conclusion: "failure", ID: 11,
			Steps: []providers.Step{{Name: "Set up job", Conclusion: "success"}, {Name: "Run the unit tests", Conclusion: "failure"}}},
		{Name: "lint", Status: "completed", Conclusion: "success", ID: 12},
		{Name: "docs", Status: "completed", Conclusion: "skipped", ID: 13},
		{Name: "extra", Status: "completed", Conclusion: "neutral", ID: 14},
		{Name: "slow", Status: "in_progress", ID: 15},
		{Name: "platform", Status: "completed", Conclusion: "cancelled", ID: 16},
	}}
}

func TestDetailReadsTheLogOfEachFailedJobOnce(t *testing.T) {
	var asked []int64
	wr := providers.Watcher{Log: func(id int64) (string, error) {
		asked = append(asked, id)
		if id == 16 {
			return "", errors.New("410")
		}
		return log("--- FAIL: TestA (0.00s)", "    a_test.go:1: m", "FAIL\tgithub.com/x/a\t0.1s"), nil
	}}
	run := failedRun()
	v := &remoteTags{refs: map[string]string{"refs/tags/v1.0.0": "abc"}}
	got := Detail(context.Background(), wr, v, "origin", run)

	if !reflect.DeepEqual(asked, []int64{11, 16}) {
		t.Fatalf("asked for the logs of %v", asked)
	}
	ci, platform := got.Jobs[0], got.Jobs[5]
	wantFailures := []providers.Failure{fail("go-test", "TestA", "github.com/x/a", "a_test.go:1: m")}
	if ci.Step != "Run the unit tests" || ci.Log != "read" || !reflect.DeepEqual(ci.Failures, wantFailures) || ci.MoreFailures != 0 {
		t.Errorf("the failed job: %+v", ci)
	}
	if platform.Step != "" || platform.Log != "unreadable" || platform.Failures != nil {
		t.Errorf("the job whose log cannot be read: %+v", platform)
	}
	for _, i := range []int{1, 2, 3, 4} {
		if want := run.Jobs[i]; !reflect.DeepEqual(got.Jobs[i], want) || got.Jobs[i].Log != "" || got.Jobs[i].Step != "" {
			t.Errorf("job %s: %+v", want.Name, got.Jobs[i])
		}
	}
	if !reflect.DeepEqual(got.Tags, []string{"v1.0.0"}) {
		t.Errorf("tags %q", got.Tags)
	}
	if run.Jobs[0].Log != "" || run.Tags != nil {
		t.Errorf("the run it was given changed: %+v", run)
	}
}

func TestDetailAsksNoLogOfARunThatSucceeded(t *testing.T) {
	wr := providers.Watcher{Log: func(int64) (string, error) { t.Error("a log was asked for"); return "", nil }}
	run := failedRun()
	run.Conclusion = "success"
	v := &remoteTags{refs: map[string]string{"refs/tags/v2": "abc"}}
	got := Detail(context.Background(), wr, v, "origin", run)
	if !reflect.DeepEqual(got.Jobs, run.Jobs) || !reflect.DeepEqual(got.Tags, []string{"v2"}) {
		t.Fatalf("%+v", got)
	}
}

func TestDetailNamesTheStepAloneForAProviderThatKeepsNoLogs(t *testing.T) {
	got := Detail(context.Background(), providers.Watcher{}, &remoteTags{}, "origin", failedRun())
	if ci := got.Jobs[0]; ci.Step != "Run the unit tests" || ci.Log != "" || ci.Failures != nil {
		t.Fatalf("%+v", ci)
	}
}

func TestEndedSaysWhatEachFailedJobNames(t *testing.T) {
	run := failedRun()
	run.Jobs[0].Step, run.Jobs[0].Log = "Run the unit tests", "read"
	run.Jobs[0].Failures = []providers.Failure{
		{Kind: "go-test", Name: "TestA", Package: "github.com/x/a", Lines: []string{"one", "two"}, MoreLines: 3},
		{Kind: "ci-run", Name: "go test (it exited 1)", Lines: []string{}},
		{Kind: "scenario", Name: "S", Lines: []string{"step"}},
	}
	run.Jobs[0].MoreFailures = 4
	run.Jobs[5].Log = "unreadable"
	run.Jobs[1].Step = "never printed: it did not fail"
	run.Jobs[1].Log = ""
	run.Tags = []string{"v1.3.0", "latest"}
	var stdout, stderr strings.Builder
	w := Ended(run, Out{Stdout: &stdout, Stderr: &stderr, Quiet: true}, &stdout)
	want := `CI failure: u; failed jobs: ci, platform; tags: v1.3.0, latest
ci: failed at the step "Run the unit tests"
  go-test TestA (github.com/x/a):
    one
    two
    ... 3 more lines
  ci-run go test (it exited 1):
  scenario S:
    step
  ... 4 more failures
lint: failed at the step "never printed: it did not fail"
platform: failed; its log could not be read
`
	if w.Code != 1 || w.Outcome != "failure" || stdout.Len() != 0 || stderr.String() != want {
		t.Fatalf("code %d, outcome %s\n%q\nnot\n%q", w.Code, w.Outcome, stderr.String(), want)
	}
}

func TestEndedSaysATailsLinesAndWhatCameBeforeThem(t *testing.T) {
	run := failedRun()
	run.Jobs = run.Jobs[:1]
	run.Jobs[0].Log = "read"
	run.Jobs[0].Step = ""
	run.Jobs[0].Failures = []providers.Failure{
		{Kind: "tail", Lines: []string{"x", "y"}, MoreLines: 11},
		{Kind: "tail", Lines: []string{"z"}},
	}
	var stderr strings.Builder
	Ended(run, Out{Stderr: &stderr}, &stderr)
	want := `CI failure: u; failed jobs: ci
ci: failed
  the last lines of the log:
    ... 11 more lines before these
    x
    y
  the last lines of the log:
    z
`
	if stderr.String() != want {
		t.Fatalf("%q\nnot\n%q", stderr.String(), want)
	}
}

func TestEndedNamesTheTagsOfARunThatSucceeded(t *testing.T) {
	run := providers.Run{URL: "u", Status: "completed", Conclusion: "success", Tags: []string{"v1.3.0"}}
	var stdout strings.Builder
	Ended(run, Out{Stdout: &stdout}, &stdout)
	if stdout.String() != "CI passed: u; tags: v1.3.0\n" {
		t.Fatalf("%q", stdout.String())
	}
	stdout.Reset()
	run.Tags = nil
	Ended(run, Out{Stdout: &stdout}, &stdout)
	if stdout.String() != "CI passed: u\n" {
		t.Fatalf("%q", stdout.String())
	}
}

func TestAWatchOfAFailedRunDetailsItsJobsAndTheTagsAtItsHead(t *testing.T) {
	noSleep(t)
	run := failedRun()
	look := func(sha string) (providers.Run, bool, error) { return run, true, nil }
	wr := providers.Watcher{Look: look, Log: func(int64) (string, error) { return log("something went wrong", "##[error]exit 1"), nil }}
	v := &remoteTags{refs: map[string]string{"refs/tags/v9": "abc"}}
	var stdout, stderr strings.Builder
	w := Watch(context.Background(), watchConfig(0, 60), wr, v, "abc", "upstream", "", Out{Stdout: &stdout, Stderr: &stderr})
	if w.Code != 1 || v.remote != "upstream" || !reflect.DeepEqual(w.Run.Tags, []string{"v9"}) {
		t.Fatalf("code %d, remote %s, run %+v", w.Code, v.remote, w.Run)
	}
	if got := w.Run.Jobs[0]; got.Step != "Run the unit tests" || got.Failures[0].Kind != "tail" || got.Failures[0].MoreLines != 0 {
		t.Fatalf("%+v", got)
	}
	for _, say := range []string{"CI failure: u; failed jobs: ci, platform; tags: v9\n", "ci: failed at the step \"Run the unit tests\"\n", "    something went wrong\n"} {
		if !strings.Contains(stderr.String(), say) {
			t.Errorf("the end does not say %q:\n%s", say, stderr.String())
		}
	}
}

// A run that failed names no failed jobs when none of its jobs finished and
// did not succeed: its line ends at its address.
func TestEndedNamesNoFailedJobsForARunWithNone(t *testing.T) {
	cancelled := failedRun()
	cancelled.Conclusion = "cancelled"
	cancelled.Jobs = cancelled.Jobs[1:5]
	noJobs := failedRun()
	noJobs.Jobs = nil
	for name, run := range map[string]providers.Run{"jobs that did not fail": cancelled, "no jobs": noJobs} {
		var stderr strings.Builder
		w := Ended(run, Out{Stderr: &stderr}, &stderr)
		want := "CI " + run.Conclusion + ": u\n"
		if w.Code != 1 || stderr.String() != want {
			t.Errorf("%s: code %d, %q; want %q", name, w.Code, stderr.String(), want)
		}
	}
}
