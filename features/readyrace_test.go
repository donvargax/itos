// The steps of bug-160 (push-mode-draft.feature): the race between --ready's
// push and its marking, as a sequence with no sleep. The fake GitHub answers a
// commit's runs by a script (headScript), which reads the recording gh's calls
// at each answer, so a run gh pr ready starts shows at the first answer after
// that call, whichever way the two raced. The rest give it two runs of one
// second, and a watched run of a certain number of looks: bug-160's wait takes
// one before anything is marked.
package features

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

// The address of the draft's run a race's step does not name.
const draftsRunURL = "https://ci.example/runs/draft"

// headScript is what the fake GitHub answers a commit's runs by, in place of
// the watched run: each answer first counts itself and runs answer, which may
// change g.runs. marked is whether the recording gh's calls hold pr ready;
// before counts the answers while they did not; looks those so far; held keeps
// a draft's run going until a step finishes it. The fake's lock is held.
type headScript struct {
	sha    string
	marked func() (bool, error)
	answer func(g *fakeGitHub, marked bool)
	looks  int
	before int
	held   bool
}

// play is one answer's turn: the script sees what gh has been asked, then
// counts the look.
func (s *headScript) play(g *fakeGitHub) error {
	marked, err := s.marked()
	if err != nil {
		return err
	}
	if !marked {
		s.before++
	}
	s.answer(g, marked)
	s.looks++
	return nil
}

// Whether the recording gh has been called as pr ready.
func (w *world) ghMarkedReady() (bool, error) {
	calls, err := w.ghCallLines()
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(calls, func(line string) bool { return strings.HasPrefix(line, "pr ready") }), nil
}

func initializeReadyRaceSteps(sc *godog.ScenarioContext, w *world) {
	const starts = `, then skipping every job, and the recording gh's "pr ready" starts the run "([^"]*)" of it, cancelled if the draft's was still going, else its jobs "([^"]*)" and "([^"]*)" succeeding$`
	sc.Step(`^the fake GitHub's run of the clone's HEAD is a draft's, going for its first (\d+) looks`+starts, func(n int, url, a, b string) error {
		return w.draftsRunRaces(n, false, url, a, b)
	})
	sc.Step(`^the fake GitHub's run of the clone's HEAD is a draft's, going until a step finishes it`+starts, func(url, a, b string) error {
		return w.draftsRunRaces(0, true, url, a, b)
	})
	sc.Step(`^the fake GitHub's draft's run finishes$`, func() error {
		if w.github == nil || w.github.script == nil {
			return errors.New("no draft's run on the fake GitHub: say one first")
		}
		w.github.mu.Lock()
		w.github.script.held = false
		w.github.mu.Unlock()
		return nil
	})
	sc.Step(`^the fake GitHub has no run of the clone's HEAD until the recording gh's "pr ready", then the run "([^"]*)" of it, whose jobs "([^"]*)" and "([^"]*)" succeed$`, w.noRunUntilMarked)
	sc.Step(`^the fake GitHub was asked for the run of the clone's HEAD at least twice before the recording gh's "pr ready" call$`, func() error {
		if w.github == nil || w.github.script == nil {
			return errors.New("no script of the clone's HEAD on the fake GitHub: say one first")
		}
		w.github.mu.Lock()
		before := w.github.script.before
		w.github.mu.Unlock()
		if before < 2 {
			return fmt.Errorf("the fake GitHub was asked for the run %d times before pr ready\n%s", before, w.report())
		}
		return nil
	})
	sc.Step(`^the fake GitHub has, created in one second, the run "([^"]*)" of the clone's HEAD, whose jobs "([^"]*)" and "([^"]*)" succeed, and the draft's run "([^"]*)" listed before it, every job skipped$`,
		func(url, a, b, draft string) error {
			return w.runsOfOneSecond(url, "success", []watchedJob{job(a, "success"), job(b, "success")}, draft)
		})
	sc.Step(`^the fake GitHub has, created in one second, the cancelled run "([^"]*)" of the clone's HEAD, listing no job, and the draft's run "([^"]*)" listed before it, every job skipped$`,
		func(url, draft string) error { return w.runsOfOneSecond(url, "cancelled", nil, draft) })
	sc.Step(`^the watched run's first (\d+) looks have its jobs "([^"]*)" and "([^"]*)" skipped, as a draft's run has, and its later looks its job "([^"]*)" failing and its job "([^"]*)" succeeding$`,
		func(n int, a, b, failing, succeeding string) error {
			var looks []watchedRun
			for range n {
				looks = append(looks, w.finished(job(a, "skipped"), job(b, "skipped")))
			}
			return w.watchedRunPolls(append(looks, w.finished(job(failing, "failure"), job(succeeding, "success")))...)
		})
	sc.Step(`^the watched run has finished at its first look, and goes on at every later one$`, func() error {
		return w.watchedRunPolls(w.finished(job("ci", "success"), job("platform", "success")),
			watchedRun{URL: w.watchURL, Status: "in_progress", Jobs: []watchedJob{{Name: "ci", Status: "in_progress"}}})
	})
	sc.Step(`^the fake GitHub was last asked for the run of the head of the remote's branch "([^"]*)"$`, func(branch string) error {
		if w.github == nil {
			return errors.New("no fake GitHub: start one first")
		}
		head, err := w.remoteBranchHead(branch)
		if err != nil {
			return err
		}
		w.github.mu.Lock()
		given := slices.Clone(w.github.given)
		w.github.mu.Unlock()
		if len(given) == 0 || given[len(given)-1] != head {
			return fmt.Errorf("the fake GitHub was asked for the runs of %v, the last not the head %s of the remote's branch %q\n%s", given, head, branch, w.report())
		}
		return nil
	})
}

// The clone's HEAD has the draft's run, going for its first n looks or, held,
// until a step finishes it, then completed with every job skipped; the first
// answer stamps it, for a short ci.watch.timeout must not pass before itos
// looks. The first answer that finds pr ready among the recording gh's calls
// adds the ready run, of the draft's second and listed behind it: its place in
// the fake's runs, oldest first, is kept before the draft's, under no workflow
// until then. It is cancelled when the looks so far are fewer than n, or the
// draft's is held, else its jobs succeed.
func (w *world) draftsRunRaces(n int, held bool, url, a, b string) error {
	if w.github == nil {
		return errors.New("no fake GitHub: start one first")
	}
	sha, err := w.head()
	if err != nil {
		return err
	}
	g := w.github
	g.mu.Lock()
	defer g.mu.Unlock()
	ready, draft := len(g.runs), len(g.runs)+1
	g.runs = append(g.runs,
		gitHubRun{ID: ready + 1, HeadSHA: sha, HeadBranch: g.branch, Status: "completed", HTMLURL: url},
		gitHubRun{ID: draft + 1, HeadSHA: sha, HeadBranch: g.branch, Status: "in_progress", HTMLURL: draftsRunURL, workflow: watchedWorkflow})
	g.script = &headScript{sha: sha, marked: w.ghMarkedReady, held: held}
	g.script.answer = func(g *fakeGitHub, marked bool) {
		if g.runs[draft].CreatedAt == "" {
			g.runs[draft].CreatedAt = time.Now().UTC().Format(time.RFC3339)
		}
		going := g.script.held || g.script.looks < n
		if marked && g.runs[ready].workflow == "" {
			conclusion, jobs := "success", []watchedJob{job(a, "success"), job(b, "success")}
			if going {
				conclusion, jobs = "cancelled", []watchedJob{job(a, "cancelled"), job(b, "cancelled")}
			}
			g.runs[ready].workflow, g.runs[ready].CreatedAt = watchedWorkflow, g.runs[draft].CreatedAt
			g.runs[ready].Conclusion, g.runs[ready].jobs = &conclusion, jobs
		}
		if !going {
			success := "success"
			g.runs[draft].Status, g.runs[draft].Conclusion = "completed", &success
			g.runs[draft].jobs = []watchedJob{job(a, "skipped"), job(b, "skipped")}
		}
	}
	return nil
}

// The clone's HEAD has no run, nor the watched run in its place, until the
// recording gh has been called as pr ready, and then the run at the address.
func (w *world) noRunUntilMarked(url, a, b string) error {
	if w.github == nil {
		return errors.New("no fake GitHub: start one first")
	}
	sha, err := w.head()
	if err != nil {
		return err
	}
	w.github.mu.Lock()
	defer w.github.mu.Unlock()
	added := false
	w.github.script = &headScript{sha: sha, marked: w.ghMarkedReady, answer: func(g *fakeGitHub, marked bool) {
		if marked && !added {
			added = true
			g.addRun(sha, url, "success", []watchedJob{job(a, "success"), job(b, "success")})
		}
	}}
	return nil
}

// The clone's HEAD has two completed runs created in one second: the run at the
// address, the ID 1, and the draft's, the ID 2, every job skipped, which GitHub
// lists first as the fake lists the later of its runs first.
func (w *world) runsOfOneSecond(url, conclusion string, jobs []watchedJob, draft string) error {
	if w.github == nil {
		return errors.New("no fake GitHub: start one first")
	}
	sha, err := w.head()
	if err != nil {
		return err
	}
	g := w.github
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.runs) > 0 {
		return fmt.Errorf("the fake GitHub holds %d runs, so the first named would not have the ID 1", len(g.runs))
	}
	g.addRun(sha, url, conclusion, jobs)
	g.addRun(sha, draft, "success", []watchedJob{job("ci", "skipped"), job("platform", "skipped")})
	g.runs[1].CreatedAt = g.runs[0].CreatedAt
	return nil
}
