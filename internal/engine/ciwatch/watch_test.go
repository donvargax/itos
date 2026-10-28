package ciwatch

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/providers"
	"github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// watchConfig is a loaded config whose watch has the interval and timeout.
func watchConfig(interval, timeout float64) *config.Loaded {
	cfg := &config.Loaded{}
	cfg.CI.Watch.Interval, cfg.CI.Watch.Timeout = &interval, &timeout
	return cfg
}

// watchRun is a watch of the commit's own run, in an empty repository.
func watchRun(cfg *config.Loaded, wr providers.Watcher, sha, remote string, o Out) Watched {
	return Watch(context.Background(), cfg, wr, &porttest.VCS{}, sha, remote, "", o)
}

// noSleep makes a watch's wait between two looks a millisecond, whatever
// its interval.
func noSleep(t *testing.T) {
	was := sleep
	sleep = func(time.Duration) { time.Sleep(time.Millisecond) }
	t.Cleanup(func() { sleep = was })
}

func TestAWatchWaitsForTheRunToAppear(t *testing.T) {
	noSleep(t)
	looks := 0
	look := func(string) (providers.Run, bool, error) {
		looks++
		if looks < 3 {
			return providers.Run{}, false, nil
		}
		return providers.Run{URL: "u", Status: "completed", Conclusion: "success"}, true, nil
	}
	var stdout, stderr strings.Builder
	w := watchRun(watchConfig(0, 60), providers.Watcher{Look: look}, "abc", "", Out{Stdout: &stdout, Stderr: &stderr})
	if w.Code != 0 || w.Outcome != "success" || looks != 3 {
		t.Fatalf("code %d, outcome %s, looks %d\n%s%s", w.Code, w.Outcome, looks, stdout.String(), stderr.String())
	}
}

func TestSuccessfulLookResetsTheTemporaryFailureBudgetToZero(t *testing.T) {
	noSleep(t)
	looks := 0
	look := func(string) (providers.Run, bool, error) {
		looks++
		if looks == 1 {
			return providers.Run{URL: "u", Status: "in_progress"}, true, nil
		}
		if looks <= giveUp {
			return providers.Run{}, false, kind.Wrap(kind.Temporary, errors.New("502"))
		}
		return providers.Run{URL: "u", Status: "completed", Conclusion: "success"}, true, nil
	}
	var stdout, stderr strings.Builder
	w := watchRun(watchConfig(0, 60), providers.Watcher{Look: look}, "abc", "", Out{Stdout: &stdout, Stderr: &stderr})
	if w.Code != 0 || w.Outcome != "success" || looks != giveUp+1 {
		t.Fatalf("code %d, outcome %s, looks %d\n%s%s", w.Code, w.Outcome, looks, stdout.String(), stderr.String())
	}
}

func TestATransientFailureIsLookedPast(t *testing.T) {
	noSleep(t)
	looks := 0
	look := func(string) (providers.Run, bool, error) {
		looks++
		if looks == 1 {
			return providers.Run{}, false, kind.Wrap(kind.Temporary, errors.New("502"))
		}
		return providers.Run{URL: "u", Status: "completed", Conclusion: "failure"}, true, nil
	}
	var stdout, stderr strings.Builder
	w := watchRun(watchConfig(0, 60), providers.Watcher{Look: look}, "abc", "", Out{Stdout: &stdout, Stderr: &stderr})
	if w.Code != exitPolicy || w.Outcome != "failure" || !strings.Contains(stderr.String(), "CI failure: u") {
		t.Fatalf("code %d, outcome %s\n%s%s", w.Code, w.Outcome, stdout.String(), stderr.String())
	}
}

// A cancelled run is no verdict (bug 41): one whose branch has no newer run
// of the commit exits 75, outcome cancelled; one that has, a rerun of the
// commit itself here, is followed, and its result is the watch's. The first
// watch ends at its first look: a second is an error that ends it, so a
// watch that follows no run does not spin.
func TestACancelledRunIsFollowedOr75(t *testing.T) {
	noSleep(t)
	cancelled := providers.Run{URL: "u", Status: "completed", Conclusion: "cancelled", ID: 1, HeadSHA: "abc", Branch: "main", Created: "1"}
	once := 0
	look := func(string) (providers.Run, bool, error) {
		if once++; once > 1 {
			return providers.Run{}, false, errors.New("looked again")
		}
		return cancelled, true, nil
	}
	alone := func(string) ([]providers.Run, error) { return []providers.Run{cancelled}, nil }
	var stdout, stderr strings.Builder
	w := watchRun(watchConfig(0, 60), providers.Watcher{Look: look, Runs: alone}, "abc", "", Out{Stdout: &stdout, Stderr: &stderr})
	if w.Code != exitTemporary || w.Outcome != "cancelled" || w.Superseded != "" {
		t.Fatalf("code %d, outcome %s, superseded %q\n%s", w.Code, w.Outcome, w.Superseded, stderr.String())
	}

	rerun := providers.Run{URL: "v", Status: "completed", Conclusion: "success", ID: 2, HeadSHA: "abc", Branch: "main", Created: "2"}
	looks := 0
	look = func(string) (providers.Run, bool, error) {
		if looks++; looks == 1 {
			return cancelled, true, nil
		}
		return rerun, true, nil
	}
	newer := func(string) ([]providers.Run, error) { return []providers.Run{rerun, cancelled}, nil }
	stdout.Reset()
	stderr.Reset()
	w = watchRun(watchConfig(0, 60), providers.Watcher{Look: look, Runs: newer}, "abc", "", Out{Stdout: &stdout, Stderr: &stderr})
	if w.Code != 0 || w.Outcome != "success" || w.Superseded != "u" || !strings.Contains(stdout.String(), "following v") {
		t.Fatalf("code %d, outcome %s, superseded %q\n%s%s", w.Code, w.Outcome, w.Superseded, stdout.String(), stderr.String())
	}
}

// A commit with no run of its own (bug 49), pushed with a commit after it
// that CI ran for alone, is judged by the newest run on the branch whose head
// has it, which the watch names; a run of a head that does not have it is no
// verdict, and without a branch the watch waits for the commit's own run.
func TestACommitWithNoRunIsJudgedByTheRunThatCoversIt(t *testing.T) {
	noSleep(t)
	aside, judged, head := "aside", "judged", "head"
	repo := &porttest.VCS{History: []porttest.Commit{
		{Name: "base"},
		{Name: aside, Parents: []string{"base"}},
		{Name: judged, Parents: []string{"base"}},
		{Name: head, Parents: []string{judged}},
	}}

	headRun := providers.Run{URL: "h", Status: "completed", Conclusion: "success", ID: 2, HeadSHA: head, Branch: "main", Created: "2"}
	asideRun := providers.Run{URL: "a", Status: "completed", Conclusion: "failure", ID: 3, HeadSHA: aside, Branch: "main", Created: "3"}
	var asked []string
	look := func(sha string) (providers.Run, bool, error) {
		asked = append(asked, sha)
		if sha == head {
			return headRun, true, nil
		}
		return providers.Run{}, false, nil
	}
	runs := func(string) ([]providers.Run, error) { return []providers.Run{asideRun, headRun}, nil }
	var stdout, stderr strings.Builder
	w := Watch(context.Background(), watchConfig(0, 60), providers.Watcher{Look: look, Runs: runs}, repo, judged, "", "main", Out{Stdout: &stdout, Stderr: &stderr})
	if w.Code != 0 || w.Run == nil || w.Run.HeadSHA != head || !strings.Contains(stdout.String(), "following h") {
		t.Fatalf("code %d, asked %v\n%s%s", w.Code, asked, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	w = Watch(context.Background(), watchConfig(0, 0.05), providers.Watcher{Look: look, Runs: runs}, repo, judged, "", "", Out{Stdout: &stdout, Stderr: &stderr})
	if w.Code != exitTemporary || w.Outcome != "timeout" {
		t.Fatalf("code %d, outcome %s\n%s%s", w.Code, w.Outcome, stdout.String(), stderr.String())
	}
}

func TestAnErrorEndsTheWatch(t *testing.T) {
	noSleep(t)
	look := func(string) (providers.Run, bool, error) { return providers.Run{}, false, errors.New("401") }
	var stdout, stderr strings.Builder
	w := watchRun(watchConfig(0, 60), providers.Watcher{Look: look}, "abc", "", Out{Stdout: &stdout, Stderr: &stderr})
	if w.Code != exitMissing || !strings.Contains(stderr.String(), "itos ci watch abc") {
		t.Fatalf("code %d\n%s", w.Code, stderr.String())
	}
}

func TestARunThatNeverAppearsTimesOut(t *testing.T) {
	look := func(string) (providers.Run, bool, error) { return providers.Run{}, false, nil }
	var stdout, stderr strings.Builder
	w := watchRun(watchConfig(0, 0.05), providers.Watcher{Look: look}, "abc", "", Out{Stdout: &stdout, Stderr: &stderr})
	if w.Code != exitTemporary || w.Outcome != "timeout" || !strings.Contains(stderr.String(), "has not appeared") ||
		strings.Contains(stderr.String(), "the last look failed") {
		t.Fatalf("code %d, outcome %s\n%s", w.Code, w.Outcome, stderr.String())
	}
}

// A timeout names the last look when it failed for a temporary reason and the
// watch looked past it, and says nothing of a look when none failed.
func TestATimeoutNamesTheLastLookThatFailed(t *testing.T) {
	noSleep(t)
	looks := 0
	look := func(string) (providers.Run, bool, error) {
		if looks++; looks == 1 {
			return providers.Run{}, false, kind.Wrap(kind.Temporary, errors.New("502"))
		}
		return providers.Run{}, false, nil
	}
	var stdout, stderr strings.Builder
	w := watchRun(watchConfig(0, 0.05), providers.Watcher{Look: look}, "abc", "", Out{Stdout: &stdout, Stderr: &stderr})
	if w.Code != exitTemporary || w.Outcome != "timeout" || !strings.Contains(stderr.String(), "itos: the last look failed: 502\n") {
		t.Fatalf("code %d, outcome %s\n%s", w.Code, w.Outcome, stderr.String())
	}
}

// A look that fails for a temporary reason giveUp times in a row ends the
// watch with 75 before its timeout; one that answers between them starts the
// count again (slice 86).
func TestTemporaryFailuresInARowGiveUpWith75(t *testing.T) {
	noSleep(t)
	looks := 0
	look := func(string) (providers.Run, bool, error) {
		looks++
		if looks == giveUp {
			return providers.Run{}, false, nil
		}
		return providers.Run{}, false, kind.Wrap(kind.Temporary, errors.New("500"))
	}
	var stdout, stderr strings.Builder
	w := watchRun(watchConfig(0, 600), providers.Watcher{Look: look}, "abc", "", Out{Stdout: &stdout, Stderr: &stderr})
	if w.Code != exitTemporary || w.Outcome != "error" || looks != 2*giveUp || !strings.Contains(stderr.String(), "itos ci watch abc") {
		t.Fatalf("code %d, outcome %s, looks %d\n%s", w.Code, w.Outcome, looks, stderr.String())
	}
}

// A rate limit's wait (bug 40) replaces the interval when longer, but never
// holds the watch past its timeout: a reset an hour away ends it with 75.
func TestARateLimitsWaitIsBoundedByTheTimeout(t *testing.T) {
	var slept []time.Duration
	was := sleep
	sleep = func(d time.Duration) { slept = append(slept, d); time.Sleep(d) }
	t.Cleanup(func() { sleep = was })
	limited := kind.Wrap(kind.Temporary, &providers.RateLimited{Wait: time.Hour, Err: errors.New("403")})
	look := func(string) (providers.Run, bool, error) { return providers.Run{}, false, limited }
	var stdout, stderr strings.Builder
	start := time.Now()
	w := watchRun(watchConfig(0, 0.05), providers.Watcher{Look: look}, "abc", "", Out{Stdout: &stdout, Stderr: &stderr})
	if w.Code != exitTemporary || w.Outcome != "timeout" || time.Since(start) > 10*time.Second ||
		len(slept) == 0 || slept[0] > 50*time.Millisecond || !strings.Contains(stdout.String(), "rate limit") {
		t.Fatalf("code %d, outcome %s, slept %v\n%s%s", w.Code, w.Outcome, slept, stdout.String(), stderr.String())
	}
}

func TestARateLimitsWaitReplacesAShorterInterval(t *testing.T) {
	var slept []time.Duration
	was := sleep
	sleep = func(d time.Duration) { slept = append(slept, d) }
	t.Cleanup(func() { sleep = was })
	looks := 0
	look := func(string) (providers.Run, bool, error) {
		if looks++; looks == 1 {
			return providers.Run{}, false, kind.Wrap(kind.Temporary, &providers.RateLimited{Wait: 30 * time.Second, Err: errors.New("403")})
		}
		if looks == 2 {
			return providers.Run{}, false, nil
		}
		return providers.Run{URL: "u", Status: "completed", Conclusion: "success"}, true, nil
	}
	var stdout, stderr strings.Builder
	w := watchRun(watchConfig(10, 600), providers.Watcher{Look: look}, "abc", "", Out{Stdout: &stdout, Stderr: &stderr})
	if w.Code != 0 || len(slept) != 2 || slept[0] != 30*time.Second || slept[1] != 10*time.Second ||
		strings.Count(stdout.String(), "rate limit") != 1 {
		t.Fatalf("code %d, slept %v\n%s%s", w.Code, slept, stdout.String(), stderr.String())
	}
}

// The watch times out only once no time is left: with a nanosecond left it
// looks once more, and a run found then is its result. The bubble's clock
// moves only while the watch sleeps, so the nanosecond is exact.
func TestAWatchLooksAgainWhileAnyTimeIsLeft(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		looks := 0
		look := func(string) (providers.Run, bool, error) {
			if looks++; looks == 1 {
				return providers.Run{}, false, nil
			}
			return providers.Run{URL: "u", Status: "completed", Conclusion: "success"}, true, nil
		}
		var stdout, stderr strings.Builder
		w := watchRun(watchConfig(0, 1e-9), providers.Watcher{Look: look}, "abc", "", Out{Stdout: &stdout, Stderr: &stderr})
		if w.Code != 0 || w.Outcome != "success" || looks != 2 {
			t.Fatalf("code %d, outcome %s, looks %d\n%s%s", w.Code, w.Outcome, looks, stdout.String(), stderr.String())
		}
	})
}

// WatchReady (slice 179) takes a run all of whose jobs were skipped, or that
// lists none, for no run yet, as the draft's run is: it looks again until the
// run that marking the pull request ready starts shows, whose result it gives.
// A run with one job that was not skipped, finished or not, is a real run, and
// plain Watch takes the draft's run for what it says.
func TestWatchReadyPassesOverADraftsRun(t *testing.T) {
	noSleep(t)
	jobs := func(conclusions ...string) []providers.Job {
		var listed []providers.Job
		for i, c := range conclusions {
			j := providers.Job{Name: string(rune('a' + i)), Status: "completed", Conclusion: c}
			if c == "" {
				j.Status = "in_progress"
			}
			listed = append(listed, j)
		}
		return listed
	}
	draft := func(listed []providers.Job) providers.Run {
		return providers.Run{URL: "draft", Status: "completed", Conclusion: "success", Jobs: listed}
	}
	real := providers.Run{URL: "real", Status: "completed", Conclusion: "failure", Jobs: jobs("failure", "success")}
	for name, first := range map[string]providers.Run{"skipped jobs": draft(jobs("skipped", "skipped")), "no jobs": draft(nil)} {
		t.Run(name, func(t *testing.T) {
			looks := 0
			look := func(string) (providers.Run, bool, error) {
				if looks++; looks == 1 {
					return first, true, nil
				}
				return real, true, nil
			}
			var stdout, stderr strings.Builder
			w := WatchReady(context.Background(), watchConfig(0, 60), providers.Watcher{Look: look}, &porttest.VCS{}, "abc", "", Out{Stdout: &stdout, Stderr: &stderr}, Ready{})
			if w.Code != exitPolicy || w.Outcome != "failure" || w.Run == nil || w.Run.URL != "real" || looks != 2 ||
				strings.Contains(stdout.String()+stderr.String(), "draft") {
				t.Errorf("code %d, outcome %s, looks %d\n%s%s", w.Code, w.Outcome, looks, stdout.String(), stderr.String())
			}
		})
	}
	for name, first := range map[string]providers.Run{
		"a job that succeeded": draft(jobs("skipped", "success")),
		"a job that failed":    {URL: "draft", Status: "completed", Conclusion: "failure", Jobs: jobs("skipped", "failure")},
	} {
		t.Run(name, func(t *testing.T) {
			looks := 0
			look := func(string) (providers.Run, bool, error) {
				looks++
				return first, true, nil
			}
			var stdout, stderr strings.Builder
			w := WatchReady(context.Background(), watchConfig(0, 60), providers.Watcher{Look: look}, &porttest.VCS{}, "abc", "", Out{Stdout: &stdout, Stderr: &stderr}, Ready{})
			if w.Run == nil || w.Run.URL != "draft" || looks != 1 {
				t.Errorf("code %d, outcome %s, looks %d\n%s%s", w.Code, w.Outcome, looks, stdout.String(), stderr.String())
			}
		})
	}
	t.Run("a job still going", func(t *testing.T) {
		looks := 0
		look := func(string) (providers.Run, bool, error) {
			if looks++; looks == 1 {
				return providers.Run{URL: "going", Status: "in_progress", Jobs: jobs("skipped", "")}, true, nil
			}
			return providers.Run{URL: "going", Status: "completed", Conclusion: "success", Jobs: jobs("skipped", "success")}, true, nil
		}
		var stdout, stderr strings.Builder
		w := WatchReady(context.Background(), watchConfig(0, 60), providers.Watcher{Look: look}, &porttest.VCS{}, "abc", "", Out{Stdout: &stdout, Stderr: &stderr}, Ready{})
		if w.Code != 0 || looks != 2 || !strings.Contains(stdout.String(), "CI run: going") {
			t.Errorf("code %d, looks %d\n%s%s", w.Code, looks, stdout.String(), stderr.String())
		}
	})
	t.Run("no other run comes", func(t *testing.T) {
		look := func(string) (providers.Run, bool, error) { return draft(jobs("skipped")), true, nil }
		var stdout, stderr strings.Builder
		w := WatchReady(context.Background(), watchConfig(0, 0.05), providers.Watcher{Look: look}, &porttest.VCS{}, "abc", "", Out{Stdout: &stdout, Stderr: &stderr}, Ready{})
		if w.Code != exitTemporary || w.Outcome != "timeout" || w.Run != nil || !strings.Contains(stderr.String(), "has not appeared") ||
			strings.Contains(stdout.String(), "draft") {
			t.Errorf("code %d, outcome %s\n%s%s", w.Code, w.Outcome, stdout.String(), stderr.String())
		}
	})
	t.Run("Watch takes the draft's run as it is", func(t *testing.T) {
		look := func(string) (providers.Run, bool, error) { return draft(jobs("skipped", "skipped")), true, nil }
		var stdout, stderr strings.Builder
		w := watchRun(watchConfig(0, 60), providers.Watcher{Look: look}, "abc", "", Out{Stdout: &stdout, Stderr: &stderr})
		if w.Code != 0 || w.Outcome != "success" {
			t.Errorf("code %d, outcome %s\n%s%s", w.Code, w.Outcome, stdout.String(), stderr.String())
		}
	})
}

// WatchReady counts ci.watch.timeout from the start of the run's latest attempt
// when GitHub gives it (slice 200): a run started past the timeout ago ends the
// watch at its first look, one that started a moment ago waits, and a start
// that cannot be read, or none, counts from the watch's first look. Plain Watch
// counts from its first look whatever the run says.
func TestWatchReadyCountsTheTimeoutFromTheRunsStart(t *testing.T) {
	noSleep(t)
	going := func(started string) func(string) (providers.Run, bool, error) {
		return func(string) (providers.Run, bool, error) {
			return providers.Run{URL: "going", Status: "in_progress", Started: started,
				Jobs: []providers.Job{{Name: "a", Status: "in_progress"}}}, true, nil
		}
	}
	ago := func(d time.Duration) string { return time.Now().UTC().Add(-d).Format(time.RFC3339) }
	for name, c := range map[string]struct {
		started string
		timeout float64
		quick   bool
	}{
		"started past the timeout ago": {ago(time.Hour), 60, true},
		"started within the timeout":   {ago(2 * time.Second), 3, true},
		"no start":                     {"", 0.3, false},
		"a start that cannot be read":  {"yesterday", 0.3, false},
	} {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr strings.Builder
			began := time.Now()
			w := WatchReady(context.Background(), watchConfig(0, c.timeout), providers.Watcher{Look: going(c.started)}, &porttest.VCS{}, "abc", "", Out{Stdout: &stdout, Stderr: &stderr}, Ready{})
			took := time.Since(began)
			if w.Code != exitTemporary || w.Outcome != "timeout" || !strings.Contains(stderr.String(), "(ci.watch.timeout)") {
				t.Fatalf("code %d, outcome %s\n%s%s", w.Code, w.Outcome, stdout.String(), stderr.String())
			}
			if c.quick && took > 2*time.Second || !c.quick && took < 200*time.Millisecond {
				t.Errorf("the watch took %s", took)
			}
		})
	}
	t.Run("Watch counts from its first look", func(t *testing.T) {
		var stdout, stderr strings.Builder
		began := time.Now()
		w := watchRun(watchConfig(0, 0.3), providers.Watcher{Look: going(ago(time.Hour))}, "abc", "", Out{Stdout: &stdout, Stderr: &stderr})
		if w.Outcome != "timeout" || time.Since(began) < 200*time.Millisecond {
			t.Errorf("outcome %s after %s\n%s%s", w.Outcome, time.Since(began), stdout.String(), stderr.String())
		}
	})
}

// Ready.Left ends the watch, exit 75, outcome bound and no word of the watch's
// own timeout, at the first look after it has passed, having looked once at
// least, while ci.watch.timeout passing first, or at the same time, is the
// timeout's. A bound still ahead shortens the wait between two looks.
func TestWatchReadyEndsAtItsBound(t *testing.T) {
	noSleep(t)
	looks := 0
	look := func(string) (providers.Run, bool, error) {
		looks++
		return providers.Run{URL: "going", Status: "in_progress", Jobs: []providers.Job{{Name: "a", Status: "in_progress"}}}, true, nil
	}
	left := func(d time.Duration) Ready { return Ready{Left: func() time.Duration { return d }} }
	for name, c := range map[string]struct {
		timeout float64
		started string
		ready   Ready
		outcome string
	}{
		"the bound passed":                     {60, "", left(-time.Second), "bound"},
		"the bound passed before the timeout":  {0.05, "", left(-time.Hour), "bound"},
		"the timeout passed before the bound":  {60, time.Now().UTC().Add(-time.Hour).Format(time.RFC3339), left(-time.Millisecond), "timeout"},
		"both passed at once":                  {0, "", left(0), "timeout"},
		"a bound ahead is left to the timeout": {0.05, "", left(time.Hour), "timeout"},
		"a bound nearer than the interval":     {60, "", Ready{Left: boundIn(100 * time.Millisecond)}, "bound"},
	} {
		t.Run(name, func(t *testing.T) {
			looks = 0
			var stdout, stderr strings.Builder
			stamped := func(sha string) (providers.Run, bool, error) {
				run, found, err := look(sha)
				run.Started = c.started
				return run, found, err
			}
			w := WatchReady(context.Background(), watchConfig(30, c.timeout), providers.Watcher{Look: stamped}, &porttest.VCS{}, "abc", "", Out{Stdout: &stdout, Stderr: &stderr}, c.ready)
			if w.Outcome != c.outcome || w.Code != exitTemporary || looks == 0 || w.Run == nil {
				t.Fatalf("code %d, outcome %s, looks %d\n%s%s", w.Code, w.Outcome, looks, stdout.String(), stderr.String())
			}
			if bound := c.outcome == "bound"; bound == strings.Contains(stderr.String(), "ci.watch.timeout") {
				t.Errorf("stderr: %s", stderr.String())
			}
		})
	}
}

// boundIn is a Left that counts down from the duration by the clock.
func boundIn(d time.Duration) func() time.Duration {
	until := time.Now().Add(d)
	return func() time.Duration { return time.Until(until) }
}
