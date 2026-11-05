package ciwatch

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/donvargax/itos/v8/internal/providers"
	"github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

var failedJobRun = providers.Run{URL: "going", Status: "in_progress", HeadSHA: "abc", Jobs: []providers.Job{
	{Name: "a", Status: "completed", Conclusion: "failure", ID: 1, Steps: []providers.Step{{Name: "the step", Conclusion: "failure"}}},
	{Name: "b", Status: "in_progress"},
}}

// The stop comes at the look that finds the job failed, before either limit: an
// earlier look found none, ci.watch.timeout passed at the run's start and
// push.pr.ready_wait has too. The job is described from its log, and the run stays
// as GitHub gave it.
func TestWatchReadyStopsAtTheLookThatFindsAJobFailed(t *testing.T) {
	noSleep(t)
	nothing := providers.Run{URL: "going", Status: "in_progress", Jobs: []providers.Job{{Name: "a", Status: "in_progress"}}}
	looks := 0
	look := func(string) (providers.Run, bool, error) {
		if looks++; looks == 1 {
			return nothing, true, nil
		}
		late := failedJobRun
		late.Started = time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
		return late, true, nil
	}
	wr := providers.Watcher{Look: look, Log: func(int64) (string, error) { return "it broke\n", nil }}
	var stdout, stderr strings.Builder
	w := WatchReady(context.Background(), watchConfig(0, 60), wr, &porttest.VCS{}, "abc", "", Out{Stdout: &stdout, Stderr: &stderr}, Ready{})
	if w.Code != exitPolicy || w.Outcome != "failure" || looks != 2 {
		t.Fatalf("code %d, outcome %s, looks %d\n%s%s", w.Code, w.Outcome, looks, stdout.String(), stderr.String())
	}
	if w.Run.Status != "in_progress" || w.Run.Conclusion != "" || w.Run.Jobs[0].Step != "the step" || w.Run.Jobs[0].Log != providers.LogRead {
		t.Errorf("the run: %+v", *w.Run)
	}
	for _, said := range []string{"CI failure: going; failed jobs: a; the run has not finished\n", "the step", "it broke"} {
		if !strings.Contains(stderr.String(), said) {
			t.Errorf("stderr does not say %q:\n%s", said, stderr.String())
		}
	}

	looks = 1
	w = WatchReady(context.Background(), watchConfig(0, 60), wr, &porttest.VCS{}, "abc", "", Out{Stdout: &stdout, Stderr: &stderr},
		Ready{Left: func() time.Duration { return -time.Hour }})
	if w.Outcome != "failure" {
		t.Errorf("past the bound: outcome %s", w.Outcome)
	}
}

// A stop on the newer run followed after a cancelled one keeps the cancelled
// run's address.
func TestWatchReadyStoppedOnANewerRunKeepsSuperseded(t *testing.T) {
	noSleep(t)
	cancelled := providers.Run{URL: "u", Status: "completed", Conclusion: "cancelled", ID: 1, HeadSHA: "abc", Branch: "main", Created: "1"}
	newer := failedJobRun
	newer.URL, newer.ID, newer.HeadSHA, newer.Branch, newer.Created = "v", 2, "new", "main", "2"
	repo := &porttest.VCS{History: []porttest.Commit{{Name: "abc"}, {Name: "new", Parents: []string{"abc"}}}}
	wr := providers.Watcher{
		Look: func(sha string) (providers.Run, bool, error) {
			if sha == "new" {
				return newer, true, nil
			}
			return cancelled, true, nil
		},
		Runs: func(string) ([]providers.Run, error) { return []providers.Run{newer, cancelled}, nil },
	}
	var stdout, stderr strings.Builder
	w := WatchReady(context.Background(), watchConfig(0, 60), wr, repo, "abc", "", Out{Stdout: &stdout, Stderr: &stderr}, Ready{})
	if w.Code != exitPolicy || w.Outcome != "failure" || w.Superseded != "u" || w.Run.URL != "v" {
		t.Fatalf("code %d, outcome %s, superseded %q\n%s%s", w.Code, w.Outcome, w.Superseded, stdout.String(), stderr.String())
	}
}

// Only a job concluded failure stops it, and only --ready's watch: the rest are
// left to the run's end, here a success.
func TestAWatchGoesOnPastWhatDoesNotStopIt(t *testing.T) {
	noSleep(t)
	failedStep := []providers.Step{{Name: "the step", Conclusion: "failure"}}
	for name, c := range map[string]struct {
		job   providers.Job
		watch func(wr providers.Watcher, o Out) Watched
	}{
		"timed_out":                        {job: providers.Job{Name: "a", Status: "completed", Conclusion: "timed_out"}},
		"cancelled":                        {job: providers.Job{Name: "a", Status: "completed", Conclusion: "cancelled"}},
		"a running job with a failed step": {job: providers.Job{Name: "a", Status: "in_progress", Steps: failedStep}},
		"a failure not yet completed":      {job: providers.Job{Name: "a", Status: "in_progress", Conclusion: "failure"}},
		"a failed job, under Watch": {
			job: providers.Job{Name: "a", Status: "completed", Conclusion: "failure"},
			watch: func(wr providers.Watcher, o Out) Watched {
				return watchRun(watchConfig(0, 60), wr, "abc", "", o)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			looks := 0
			wr := providers.Watcher{Look: func(string) (providers.Run, bool, error) {
				if looks++; looks == 1 {
					return providers.Run{URL: "going", Status: "in_progress", Jobs: []providers.Job{c.job}}, true, nil
				}
				return providers.Run{URL: "going", Status: "completed", Conclusion: "success", Jobs: []providers.Job{{Name: "a", Status: "completed", Conclusion: "success"}}}, true, nil
			}}
			var stdout, stderr strings.Builder
			o := Out{Stdout: &stdout, Stderr: &stderr}
			var w Watched
			if c.watch != nil {
				w = c.watch(wr, o)
			} else {
				w = WatchReady(context.Background(), watchConfig(0, 60), wr, &porttest.VCS{}, "abc", "", o, Ready{})
			}
			if w.Code != 0 || w.Outcome != "success" || looks != 2 {
				t.Fatalf("code %d, outcome %s, looks %d\n%s%s", w.Code, w.Outcome, looks, stdout.String(), stderr.String())
			}
		})
	}
}
