// Where a run of the features spends its time (T-115): with -timings=<file>,
// the run writes a report to the file once it ends, the run's whole time, the
// slowest scenarios, and every kind of step by the time it took in all,
// a kind being the step's text with each quoted string in it replaced by "…",
// beside the scenarios' set-up and clean-up (the scratch repository made, its
// folders removed), which are not steps. It judges nothing: CI prints it, so
// a slow platform's run says what it pays for.
package features

import (
	"cmp"
	"context"
	"flag"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"
)

var timingsFile = flag.String("timings", "", "write where the run spent its time to this file")

// The run's timings, kept only when -timings names a file.
var timings = struct {
	sync.Mutex
	start     time.Time
	scenarios []scenarioTime
	kinds     map[string]*kindTime
}{kinds: map[string]*kindTime{}}

type scenarioTime struct {
	name, uri string
	took      time.Duration
}

type kindTime struct {
	count int
	took  time.Duration
}

// How many of the slowest scenarios and the costliest kinds the report lists.
const timingsListed = 40

var quoted = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)

func timingsOn() bool { return *timingsFile != "" }

func startTimings() {
	if timingsOn() {
		timings.start = time.Now()
	}
}

// timeKind adds took to the kind's time.
func timeKind(kind string, took time.Duration) {
	if !timingsOn() {
		return
	}
	timings.Lock()
	defer timings.Unlock()
	k := timings.kinds[kind]
	if k == nil {
		k = &kindTime{}
		timings.kinds[kind] = k
	}
	k.count++
	k.took += took
}

// A scenario's clock: when it and its current step began.
type scenarioClock struct{ began, stepBegan time.Time }

// timeScenario registers the hooks that start a scenario's clock and time its
// steps, first of the scenario's hooks, so its time holds its set-up; end
// registers the one that stops it, last, so it holds its clean-up. Nil when
// the run keeps no timings.
func timeScenario(sc *godog.ScenarioContext) *scenarioClock {
	if !timingsOn() {
		return nil
	}
	c := &scenarioClock{}
	sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		c.began = time.Now()
		return ctx, nil
	})
	sc.StepContext().Before(func(ctx context.Context, _ *godog.Step) (context.Context, error) {
		c.stepBegan = time.Now()
		return ctx, nil
	})
	sc.StepContext().After(func(ctx context.Context, st *godog.Step, _ godog.StepResultStatus, err error) (context.Context, error) {
		timeKind(quoted.ReplaceAllString(st.Text, `"…"`), time.Since(c.stepBegan))
		return ctx, err
	})
	return c
}

func (c *scenarioClock) end(sc *godog.ScenarioContext) {
	if c == nil {
		return
	}
	sc.After(func(ctx context.Context, s *godog.Scenario, err error) (context.Context, error) {
		timings.Lock()
		timings.scenarios = append(timings.scenarios, scenarioTime{s.Name, s.Uri, time.Since(c.began)})
		timings.Unlock()
		return ctx, err
	})
}

// timed runs f and adds its time to the kind's.
func timed(kind string, f func() error) error {
	began := time.Now()
	err := f()
	timeKind(kind, time.Since(began))
	return err
}

// writeTimings writes the report to the -timings file.
func writeTimings() error {
	if !timingsOn() {
		return nil
	}
	timings.Lock()
	defer timings.Unlock()
	var b strings.Builder
	var inScenarios time.Duration
	for _, s := range timings.scenarios {
		inScenarios += s.took
	}
	fmt.Fprintf(&b, "%d scenarios in %.1fs, %.1fs of it in the scenarios\n",
		len(timings.scenarios), time.Since(timings.start).Seconds(), inScenarios.Seconds())

	slowest := slices.Clone(timings.scenarios)
	slices.SortFunc(slowest, func(a, b scenarioTime) int { return cmp.Compare(b.took, a.took) })
	fmt.Fprintf(&b, "\nThe slowest scenarios:\n")
	for _, s := range slowest[:min(timingsListed, len(slowest))] {
		fmt.Fprintf(&b, "%7.2fs  %s: %s\n", s.took.Seconds(), s.uri, s.name)
	}

	type kind struct {
		text string
		kindTime
	}
	var kinds []kind
	for text, k := range timings.kinds {
		kinds = append(kinds, kind{text, *k})
	}
	slices.SortFunc(kinds, func(a, b kind) int { return cmp.Compare(b.took, a.took) })
	fmt.Fprintf(&b, "\nThe kinds of step by their time in all (seconds, runs, mean):\n")
	for _, k := range kinds[:min(timingsListed, len(kinds))] {
		fmt.Fprintf(&b, "%7.1fs %5d %6.3fs  %s\n", k.took.Seconds(), k.count, k.took.Seconds()/float64(k.count), k.text)
	}
	return os.WriteFile(*timingsFile, []byte(b.String()), 0o644)
}
