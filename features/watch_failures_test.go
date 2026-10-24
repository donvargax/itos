// The steps of watch-failures.feature (slice 172): a failed run's jobs named
// by the step they failed at and what their logs say failed. They give the
// fake GitHub of range_test.go a failed job with the steps the jobs API lists
// and the log its job keeps, and read the jobs of itos's --json back by name.
package features

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
)

func initializeWatchFailureSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the watched run's job "([^"]*)" fails at its step "([^"]*)", its log:$`, func(name, step string, log *godog.DocString) error {
		return w.watchedJobFails(name, failedAt(step), log.Content)
	})
	sc.Step(`^the watched run's job "([^"]*)" fails at its step "([^"]*)", its log, each \\e in it an escape character:$`, func(name, step string, log *godog.DocString) error {
		return w.watchedJobFails(name, failedAt(step), strings.ReplaceAll(log.Content, `\e`, "\x1b"))
	})
	sc.Step(`^the watched run's job "([^"]*)" fails at its step "([^"]*)", its log the lines "([^"]*)" to "([^"]*)", then GitHub's line for exit code (\d+) and its cleanup$`,
		func(name, step, from, to string, code int) error {
			log, err := numberedLines(from, to)
			if err != nil {
				return err
			}
			log = append(log, fmt.Sprintf("##[error]Process completed with exit code %d.", code),
				"Post job cleanup.", "Cleaning up orphan processes")
			return w.watchedJobFails(name, failedAt(step), strings.Join(log, "\n"))
		})
	sc.Step(`^the watched run's job "([^"]*)" fails at its step "([^"]*)", its log (\d+) failed Go tests of (\d+) error lines each$`,
		func(name, step string, tests, lines int) error {
			var log []string
			for t := 1; t <= tests; t++ {
				log = append(log, fmt.Sprintf("--- FAIL: TestFailure%02d (0.00s)", t))
				for l := 1; l <= lines; l++ {
					log = append(log, fmt.Sprintf("    failure_test.go:%d: error line %d of test %d", l, l, t))
				}
			}
			log = append(log, "FAIL", "FAIL\tgithub.com/donvargax/itos/v8/internal/many\t0.010s", "##[error]Process completed with exit code 1.")
			return w.watchedJobFails(name, failedAt(step), strings.Join(log, "\n"))
		})
	sc.Step(`^the watched run's job "([^"]*)" fails with no steps listed, its log:$`, func(name string, log *godog.DocString) error {
		return w.watchedJobFails(name, nil, log.Content)
	})
	sc.Step(`^the fake GitHub answers 410 for the log of the job "([^"]*)"$`, func(name string) error {
		return w.logSetting(name, func(g *fakeGitHub) *map[string]bool { return &g.logGone })
	})
	sc.Step(`^the fake GitHub serves the log of the job "([^"]*)" with CRLF line ends$`, func(name string) error {
		return w.logSetting(name, func(g *fakeGitHub) *map[string]bool { return &g.logCRLF })
	})
	sc.Step(`^the watched run's jobs "([^"]*)" and "([^"]*)" succeed, and once itos has pushed, the remote's head is given the annotated tag "([^"]*)"$`,
		w.watchedRunTagged)
	sc.Step(`^the fake GitHub was never asked for a job's log$`, func() error {
		if w.github == nil {
			return errors.New("no fake GitHub: start one first")
		}
		for _, asked := range w.github.requests() {
			if strings.Contains(asked, "/actions/jobs/") {
				return fmt.Errorf("the fake GitHub was asked for a job's log: %s\n%s", asked, w.report())
			}
		}
		return nil
	})

	sc.Step(`^its stderr says "([^"]*)"$`, func(text string) error {
		if !strings.Contains(w.stderr, text) {
			return fmt.Errorf("the stderr does not say %q\n%s", text, w.report())
		}
		return nil
	})
	sc.Step(`^its stderr does not say "([^"]*)"$`, func(text string) error {
		if strings.Contains(w.stderr, text) {
			return fmt.Errorf("the stderr says %q\n%s", text, w.report())
		}
		return nil
	})
	sc.Step(`^its JSON's job "([^"]*)" has "([^"]*)" "([^"]*)"$`, func(name, key, want string) error {
		return w.jobHas(name, key, want)
	})
	sc.Step(`^its JSON's job "([^"]*)" has "([^"]*)" (\d+)$`, func(name, key string, want int) error {
		return w.jobHas(name, key, float64(want))
	})
	sc.Step(`^its JSON's job "([^"]*)" has no "([^"]*)"$`, func(name, key string) error {
		j, err := w.jobOf(name)
		if err != nil {
			return err
		}
		if got, ok := j[key]; ok {
			return fmt.Errorf("the JSON's job %q has %q, %v\n%s", name, key, got, w.report())
		}
		return nil
	})
	sc.Step(`^its JSON's run has no "([^"]*)"$`, func(key string) error {
		run, err := w.runJSON()
		if err != nil {
			return err
		}
		if got, ok := run[key]; ok {
			return fmt.Errorf("the JSON's run has %q, %v\n%s", key, got, w.report())
		}
		return nil
	})
	sc.Step(`^its JSON's job "([^"]*)" lists these failures:$`, w.jobListsFailures)
	sc.Step(`^its JSON's job "([^"]*)"'s failure "([^"]*)" has "([^"]*)" "([^"]*)"$`, func(name, failure, key, want string) error {
		f, err := w.failureOf(name, failure)
		if err != nil {
			return err
		}
		if f[key] != want {
			return fmt.Errorf("the failure %q of job %q has %q %v, not %q\n%s", failure, name, key, f[key], want, w.report())
		}
		return nil
	})
	sc.Step(`^its JSON's job "([^"]*)"'s failure "([^"]*)" has the lines:$`, func(name, failure string, want *godog.DocString) error {
		f, err := w.failureOf(name, failure)
		if err != nil {
			return err
		}
		return w.linesAre(f, strings.Split(want.Content, "\n"))
	})
	sc.Step(`^its JSON's job "([^"]*)"'s tail has (\d+) lines, from "([^"]*)" to "([^"]*)"$`, func(name string, n int, from, to string) error {
		f, err := w.failureOf(name, "")
		if err != nil {
			return err
		}
		lines, _ := f["lines"].([]any)
		if len(lines) != n || n == 0 || lines[0] != from || lines[n-1] != to {
			return fmt.Errorf("the tail of job %q is %d lines %v, not %d from %q to %q\n%s", name, len(lines), lines, n, from, to, w.report())
		}
		return nil
	})
	sc.Step(`^its JSON's job "([^"]*)"'s tail has "more_lines" (\d+)$`, func(name string, want int) error {
		f, err := w.failureOf(name, "")
		if err != nil {
			return err
		}
		if f["more_lines"] != float64(want) {
			return fmt.Errorf("the tail of job %q has \"more_lines\" %v, not %d\n%s", name, f["more_lines"], want, w.report())
		}
		return nil
	})
	sc.Step(`^its JSON's job "([^"]*)" has (\d+) failures, each a "([^"]*)" with (\d+) lines and "more_lines" (\d+)$`,
		func(name string, n int, kind string, lines, more int) error {
			failures, err := w.failuresOf(name)
			if err != nil {
				return err
			}
			if len(failures) != n {
				return fmt.Errorf("the job %q has %d failures, not %d\n%s", name, len(failures), n, w.report())
			}
			for _, f := range failures {
				if got, _ := f["lines"].([]any); f["kind"] != kind || len(got) != lines || f["more_lines"] != float64(more) {
					return fmt.Errorf("the failure %v is not a %q with %d lines and \"more_lines\" %d\n%s", f, kind, lines, more, w.report())
				}
			}
			return nil
		})
}

// The steps of a job that failed at the step: GitHub's own before and after
// it, which succeeded.
func failedAt(step string) []watchedStep {
	return []watchedStep{{"Set up job", "success"}, {step, "failure"}, {"Complete job", "success"}}
}

// The watched run, finished, has the failed job: the one of its name replaced,
// else added after the jobs it has.
func (w *world) watchedJobFails(name string, steps []watchedStep, log string) error {
	if w.github == nil {
		return errors.New("no fake GitHub: start one first")
	}
	w.github.mu.Lock()
	var jobs []watchedJob
	if len(w.github.watched) > 0 {
		jobs = slices.Clone(w.github.watched[len(w.github.watched)-1].Jobs)
	}
	w.github.mu.Unlock()
	failed := watchedJob{Name: name, Status: "completed", Conclusion: "failure", steps: steps, log: log}
	if at := slices.IndexFunc(jobs, func(j watchedJob) bool { return j.Name == name }); at >= 0 {
		jobs[at] = failed
	} else {
		jobs = append(jobs, failed)
	}
	return w.watchedRunPolls(w.finished(jobs...))
}

// The fake GitHub treats the job's log in the way the selected map says.
func (w *world) logSetting(name string, which func(*fakeGitHub) *map[string]bool) error {
	if w.github == nil {
		return errors.New("no fake GitHub: start one first")
	}
	w.github.mu.Lock()
	defer w.github.mu.Unlock()
	set := which(w.github)
	if *set == nil {
		*set = map[string]bool{}
	}
	(*set)[name] = true
	return nil
}

// The watched run's jobs succeed, and when the fake GitHub is first asked for
// the run, after itos pushed, the remote's main is given the annotated tag, as
// GitHub creates a release's tag while the run is going.
func (w *world) watchedRunTagged(a, b, tag string) error {
	if err := w.watchedRunPolls(w.finished(job(a, "success"), job(b, "success"))); err != nil {
		return err
	}
	before := w.github.firstLook
	w.github.firstLook = func() error {
		if before != nil {
			if err := before(); err != nil {
				return err
			}
		}
		return w.gitIn(w.remote(), "tag", "-a", "-m", "Release "+tag+".", tag, "main")
	}
	return nil
}

var endsInNumber = regexp.MustCompile(`^(.*?)(\d+)$`)

// The lines "line 1" to "line 30": the words and the number the first ends in,
// up to the number the second ends in.
func numberedLines(from, to string) ([]string, error) {
	first, last := endsInNumber.FindStringSubmatch(from), endsInNumber.FindStringSubmatch(to)
	if first == nil || last == nil {
		return nil, fmt.Errorf("%q and %q do not end in a number", from, to)
	}
	start, _ := strconv.Atoi(first[2])
	end, _ := strconv.Atoi(last[2])
	var lines []string
	for n := start; n <= end; n++ {
		lines = append(lines, first[1]+strconv.Itoa(n))
	}
	return lines, nil
}

// The run of the output's JSON.
func (w *world) runJSON() (map[string]any, error) {
	var got struct {
		Run map[string]any `json:"run"`
	}
	if err := json.Unmarshal([]byte(w.stdout), &got); err != nil {
		return nil, fmt.Errorf("the output is not JSON: %v\n%s", err, w.report())
	}
	if got.Run == nil {
		return nil, fmt.Errorf("the JSON has no run\n%s", w.report())
	}
	return got.Run, nil
}

// The job of the run in the output's JSON, by its name.
func (w *world) jobOf(name string) (map[string]any, error) {
	run, err := w.runJSON()
	if err != nil {
		return nil, err
	}
	jobs, _ := run["jobs"].([]any)
	for _, j := range jobs {
		if one, _ := j.(map[string]any); one["name"] == name {
			return one, nil
		}
	}
	return nil, fmt.Errorf("the JSON's run has no job %q\n%s", name, w.report())
}

func (w *world) jobHas(name, key string, want any) error {
	j, err := w.jobOf(name)
	if err != nil {
		return err
	}
	if j[key] != want {
		return fmt.Errorf("the JSON's job %q has %q %v, not %v\n%s", name, key, j[key], want, w.report())
	}
	return nil
}

// The failures of the job, each as an object.
func (w *world) failuresOf(name string) ([]map[string]any, error) {
	j, err := w.jobOf(name)
	if err != nil {
		return nil, err
	}
	listed, _ := j["failures"].([]any)
	var failures []map[string]any
	for _, f := range listed {
		one, _ := f.(map[string]any)
		failures = append(failures, one)
	}
	return failures, nil
}

// The job's failure of the name; the tail has none.
func (w *world) failureOf(name, failure string) (map[string]any, error) {
	failures, err := w.failuresOf(name)
	if err != nil {
		return nil, err
	}
	for _, f := range failures {
		if got, _ := f["name"].(string); got == failure && (failure != "" || f["kind"] == "tail") {
			return f, nil
		}
	}
	return nil, fmt.Errorf("the job %q has no failure named %q: %v\n%s", name, failure, failures, w.report())
}

// The job's failures are these, in this order, each a kind and a name (none
// for the tail's).
func (w *world) jobListsFailures(name string, table *godog.Table) error {
	failures, err := w.failuresOf(name)
	if err != nil {
		return err
	}
	var want, got []string
	for _, row := range table.Rows[1:] {
		want = append(want, row.Cells[0].Value+" "+row.Cells[1].Value)
	}
	for _, f := range failures {
		kind, _ := f["kind"].(string)
		failure, _ := f["name"].(string)
		got = append(got, kind+" "+failure)
	}
	if !slices.Equal(got, want) {
		return fmt.Errorf("the job %q lists the failures %q, not %q\n%s", name, got, want, w.report())
	}
	return nil
}

func (w *world) linesAre(failure map[string]any, want []string) error {
	var got []string
	listed, _ := failure["lines"].([]any)
	for _, l := range listed {
		got = append(got, fmt.Sprint(l))
	}
	if !slices.Equal(got, want) {
		return fmt.Errorf("the failure %v has the lines %q, not %q\n%s", failure["name"], got, want, w.report())
	}
	return nil
}
