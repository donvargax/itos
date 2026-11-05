// The steps of slice-225 (push-mode-draft.feature): the watched run's looks for
// --ready's stop at a job that already failed, the run still going.
package features

import (
	"errors"
	"fmt"
	"slices"

	"github.com/cucumber/godog"
)

func initializeReadyFailedJobSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the watched run goes on at every look, its job "([^"]*)" still running$`, func(name string) error {
		g := w.github
		g.mu.Lock()
		defer g.mu.Unlock()
		for i := range g.watched {
			r := g.watched[i]
			jobs := slices.Clone(r.Jobs)
			running := watchedJob{Name: name, Status: "in_progress"}
			if at := slices.IndexFunc(jobs, func(j watchedJob) bool { return j.Name == name }); at >= 0 {
				jobs[at] = running
			} else {
				jobs = append(jobs, running)
			}
			g.watched[i] = watchedRun{URL: r.URL, Status: "in_progress", Jobs: jobs}
		}
		return nil
	})
	sc.Step(`^the watched run has one look before those, finished with its jobs "([^"]*)" and "([^"]*)" skipped, as a draft's run has$`, func(a, b string) error {
		first := w.finished(job(a, "skipped"), job(b, "skipped"))
		g := w.github
		g.mu.Lock()
		defer g.mu.Unlock()
		g.watched = append([]watchedRun{first}, g.watched...)
		return nil
	})
	sc.Step(`^the watched run's first look goes on with its job "([^"]*)" cancelled and its job "([^"]*)" running, and its later looks end it cancelled, both jobs cancelled$`, func(a, b string) error {
		return w.watchedRunPolls(
			watchedRun{URL: w.watchURL, Status: "in_progress", Jobs: []watchedJob{job(a, "cancelled"), {Name: b, Status: "in_progress"}}},
			watchedRun{URL: w.watchURL, Status: "completed", Conclusion: "cancelled", Jobs: []watchedJob{job(a, "cancelled"), job(b, "cancelled")}})
	})
	sc.Step(`^the watched run's first look goes on with its job "([^"]*)" failed and its job "([^"]*)" running, and its later looks end it a success, "([^"]*)" still failed and "([^"]*)" succeeded, as continue-on-error lets a run$`, func(a, b, c, d string) error {
		return w.watchedRunPolls(
			watchedRun{URL: w.watchURL, Status: "in_progress", Jobs: []watchedJob{job(a, "failure"), {Name: b, Status: "in_progress"}}},
			watchedRun{URL: w.watchURL, Status: "completed", Conclusion: "success", Jobs: []watchedJob{job(c, "failure"), job(d, "success")}})
	})
	sc.Step(`^the watched run's first look goes on with its job "([^"]*)" running, and its later looks go on with its job "([^"]*)" failed$`, func(a, b string) error {
		return w.watchedRunPolls(
			watchedRun{URL: w.watchURL, Status: "in_progress", Jobs: []watchedJob{{Name: a, Status: "in_progress"}, {Name: b, Status: "in_progress"}}},
			watchedRun{URL: w.watchURL, Status: "in_progress", Jobs: []watchedJob{{Name: a, Status: "in_progress"}, job(b, "failure")}})
	})
	sc.Step(`^the watched run's every look has finished it cancelled, its job "([^"]*)" failed and its job "([^"]*)" cancelled$`, func(a, b string) error {
		return w.watchedRunPolls(watchedRun{URL: w.watchURL, Status: "completed", Conclusion: "cancelled", Jobs: []watchedJob{job(a, "failure"), job(b, "cancelled")}})
	})
	sc.Step(`^the fake GitHub was asked for the run of the clone's HEAD once$`, func() error {
		if w.github == nil {
			return errors.New("no fake GitHub: start one first")
		}
		head, err := w.head()
		if err != nil {
			return err
		}
		w.github.mu.Lock()
		given := slices.Clone(w.github.given)
		w.github.mu.Unlock()
		if len(given) != 1 || given[0] != head {
			return fmt.Errorf("the fake gave looks %v, not one of %s\n%s", given, head, w.report())
		}
		return nil
	})
}
