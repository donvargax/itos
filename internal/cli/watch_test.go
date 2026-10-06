package cli

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/donvargax/itos/v5/internal/config"
	"github.com/donvargax/itos/v5/internal/providers"
)

// watchConfig is a loaded config whose watch has the interval and timeout.
func watchConfig(interval, timeout float64) *config.Loaded {
	cfg := &config.Loaded{}
	cfg.CI.Watch.Interval, cfg.CI.Watch.Timeout = &interval, &timeout
	return cfg
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
	w := watchRun(watchConfig(0, 60), look, "abc", Out{Stdout: &stdout, Stderr: &stderr})
	if w.code != 0 || w.outcome != "success" || looks != 3 {
		t.Fatalf("code %d, outcome %s, looks %d\n%s%s", w.code, w.outcome, looks, stdout.String(), stderr.String())
	}
}

func TestATransientFailureIsLookedPast(t *testing.T) {
	noSleep(t)
	looks := 0
	look := func(string) (providers.Run, bool, error) {
		looks++
		if looks == 1 {
			return providers.Run{}, false, providers.Transient{Err: errors.New("502")}
		}
		return providers.Run{URL: "u", Status: "completed", Conclusion: "cancelled"}, true, nil
	}
	var stdout, stderr strings.Builder
	w := watchRun(watchConfig(0, 60), look, "abc", Out{Stdout: &stdout, Stderr: &stderr})
	if w.code != ExitPolicy || w.outcome != "failure" || !strings.Contains(stderr.String(), "CI cancelled: u") {
		t.Fatalf("code %d, outcome %s\n%s%s", w.code, w.outcome, stdout.String(), stderr.String())
	}
}

func TestAnErrorEndsTheWatch(t *testing.T) {
	noSleep(t)
	look := func(string) (providers.Run, bool, error) { return providers.Run{}, false, errors.New("401") }
	var stdout, stderr strings.Builder
	w := watchRun(watchConfig(0, 60), look, "abc", Out{Stdout: &stdout, Stderr: &stderr})
	if w.code != ExitMissing || !strings.Contains(stderr.String(), "itos ci watch abc") {
		t.Fatalf("code %d\n%s", w.code, stderr.String())
	}
}

func TestARunThatNeverAppearsTimesOut(t *testing.T) {
	look := func(string) (providers.Run, bool, error) { return providers.Run{}, false, nil }
	var stdout, stderr strings.Builder
	w := watchRun(watchConfig(0, 0.05), look, "abc", Out{Stdout: &stdout, Stderr: &stderr})
	if w.code != ExitMissing || w.outcome != "timeout" || !strings.Contains(stderr.String(), "has not appeared") {
		t.Fatalf("code %d, outcome %s\n%s", w.code, w.outcome, stderr.String())
	}
}
