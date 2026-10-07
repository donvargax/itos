// The steps of itos push waiting for CI and itos ci watch (watch.feature):
// ci.watch's provider is github, asking the fake GitHub of range_test.go, which
// reports the watched run, one look a request, and records the commit it was
// asked about, so nothing reaches a network; the config's ci.watch is
// committed and pushed to the remote, as a project's config would be there
// already. The command provider these steps once scripted went in v5.0.0
// (slice 85). Bug 41's steps give the fake GitHub runs of their own: a run a
// newer push cancelled, and the newer run, of a commit the clone has or of
// the head another clone pushed while itos waited.
package features

import (
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"

	"github.com/cucumber/godog"

	"github.com/donvargax/itos/v6/internal/git"
)

// What a scenario sets of the scratch config's ci.watch.
type watchConfig struct {
	provider string // ci.watch.provider
	timeout  int    // ci.watch.timeout, when above 0
	// ci.watch.github.nightly_workflow, when set
	nightlyWorkflow string
}

// The workflow ci.watch asks the fake GitHub about.
const watchedWorkflow = "ci.yml"

// One look at the watched run, as the fake GitHub reports it.
type watchedRun struct {
	URL        string       `json:"url"`
	Status     string       `json:"status"`
	Conclusion string       `json:"conclusion,omitempty"`
	Jobs       []watchedJob `json:"jobs"`
}

type watchedJob struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion,omitempty"`
}

func initializeWatchSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the watched run's jobs "([^"]*)" and "([^"]*)" succeed$`, func(a, b string) error {
		return w.watchedRunPolls(w.finished(job(a, "success"), job(b, "success")))
	})
	sc.Step(`^the watched run's job "([^"]*)" fails and its job "([^"]*)" succeeds$`, func(a, b string) error {
		return w.watchedRunPolls(w.finished(job(a, "failure"), job(b, "success")))
	})
	sc.Step(`^the watched run finishes its job "([^"]*)" first and its job "([^"]*)" one poll later, both succeeding$`, func(a, b string) error {
		first := watchedRun{URL: w.watchURL, Status: "in_progress", Jobs: []watchedJob{job(a, "success"), {Name: b, Status: "in_progress"}}}
		return w.watchedRunPolls(first, w.finished(job(a, "success"), job(b, "success")))
	})
	sc.Step(`^the watched run never finishes$`, func() error {
		return w.watchedRunPolls(watchedRun{URL: w.watchURL, Status: "in_progress", Jobs: []watchedJob{{Name: "ci", Status: "in_progress"}}})
	})
	sc.Step(`^ci\.watch\.timeout is (\d+)$`, func(seconds int) error {
		w.config.watch.timeout = seconds
		return w.pushConfig("chore: wait less for CI")
	})
	sc.Step(`^ci\.watch's provider is "([^"]*)"$`, func(provider string) error {
		w.config.watch.provider = provider
		return w.pushConfig("chore: watch CI another way")
	})
	sc.Step(`^no gh on the PATH$`, func() error {
		w.noGh = true
		return nil
	})
	sc.Step(`^its output says "([^"]*)" once, before "([^"]*)"$`, w.outputSaysOnceBefore)

	sc.Step(`^ci\.watch asks a fake GitHub, which reports the run "([^"]*)"$`, w.watchAsksFakeGitHub)
	sc.Step(`^ci\.watch's nightly workflow on the fake GitHub has the run "([^"]*)", its job "([^"]*)" failed$`, w.nightlyOnFakeGitHub)
	sc.Step(`^the fake GitHub refuses the token$`, func() error {
		if w.github == nil {
			return errors.New("no fake GitHub: start one first")
		}
		w.github.refuses = true
		return nil
	})
	sc.Step(`^the fake GitHub answers every look with a server error$`, func() error {
		if w.github == nil {
			return errors.New("no fake GitHub: start one first")
		}
		w.github.failing = true
		return nil
	})
	sc.Step(`^no GitHub token in the environment$`, func() error {
		w.vars = slices.DeleteFunc(w.vars, func(v string) bool { return strings.HasPrefix(v, "GITHUB_TOKEN=") })
		return nil
	})
	sc.Step(`^the fake GitHub reports the run of the clone's (HEAD(?:~\d+)?) cancelled$`, w.cancelledRunOf)
	sc.Step(`^the fake GitHub reports the run "([^"]*)" of the clone's HEAD, whose jobs "([^"]*)" and "([^"]*)" succeed$`, func(url, a, b string) error {
		return w.runOfHead(url, "success", job(a, "success"), job(b, "success"))
	})
	sc.Step(`^the fake GitHub reports the run "([^"]*)" of the clone's HEAD, whose job "([^"]*)" fails$`, func(url, name string) error {
		return w.runOfHead(url, "failure", job(name, "failure"))
	})
	sc.Step(`^once itos has pushed, another clone pushes the commit "([^"]*)"$`, w.pushedOverWhileWaiting)
	sc.Step(`^the fake GitHub reports the pushed commit's run cancelled, and the run "([^"]*)" of the remote's head, whose jobs "([^"]*)" and "([^"]*)" succeed$`, w.pushedRunCancelled)
	sc.Step(`^the fake GitHub was asked for the run of the clone's HEAD$`, w.fakeGitHubGivenHead)
	sc.Step(`^the fake GitHub was never asked about a run$`, func() error {
		if w.github == nil {
			return errors.New("no fake GitHub: start one first")
		}
		if asked := w.github.requests(); len(asked) > 0 {
			return fmt.Errorf("the fake GitHub was asked %v\n%s", asked, w.report())
		}
		return nil
	})
}

func job(name, conclusion string) watchedJob {
	return watchedJob{Name: name, Status: "completed", Conclusion: conclusion}
}

// The run completed with these jobs: a success when every one succeeded.
func (w *world) finished(jobs ...watchedJob) watchedRun {
	conclusion := "success"
	for _, j := range jobs {
		if j.Conclusion != "success" {
			conclusion = "failure"
		}
	}
	return watchedRun{URL: w.watchURL, Status: "completed", Conclusion: conclusion, Jobs: jobs}
}

// The jobs of a run a newer push cancelled, as GitHub reports them.
func cancelledJobs() []watchedJob {
	return []watchedJob{job("ci", "cancelled"), job("platform", "cancelled")}
}

// The fake GitHub holds a run of the clone's commit, by its revision, that
// was cancelled, as a newer push's run cancels it by CI's concurrency (bug
// 41), at an address of its own.
func (w *world) cancelledRunOf(rev string) error {
	if w.github == nil {
		return errors.New("no fake GitHub: start one first")
	}
	sha, err := w.gitOutput("rev-parse", "--verify", rev+"^{commit}")
	if err != nil {
		return err
	}
	g := w.github
	g.mu.Lock()
	defer g.mu.Unlock()
	g.addRun(strings.TrimSpace(sha), "https://ci.example/runs/cancelled", "cancelled", cancelledJobs())
	return nil
}

// The fake GitHub holds a completed run of the clone's HEAD at the address,
// with the conclusion and the jobs, newer than every run before it.
func (w *world) runOfHead(url, conclusion string, jobs ...watchedJob) error {
	if w.github == nil {
		return errors.New("no fake GitHub: start one first")
	}
	head, err := w.head()
	if err != nil {
		return err
	}
	g := w.github
	g.mu.Lock()
	defer g.mu.Unlock()
	g.addRun(head, url, conclusion, jobs)
	return nil
}

// When the fake GitHub is first asked for the watched run, after itos pushed,
// another clone of the remote pushes the commit to its main past the hooks,
// touching NOTES.md, as a second push landing close behind the first does.
func (w *world) pushedOverWhileWaiting(subject string) error {
	if w.github == nil {
		return errors.New("no fake GitHub: start one first")
	}
	before := w.github.firstLook
	w.github.firstLook = func() error {
		if before != nil {
			if err := before(); err != nil {
				return err
			}
		}
		return w.remoteGainsCommit(subject, "NOTES.md")
	}
	return nil
}

// Once the other clone has pushed (pushedOverWhileWaiting, which comes
// first), the fake GitHub holds the pushed commit's run, the clone's HEAD's,
// cancelled, and a newer run of the remote's head at the address, its jobs
// succeeding: the newer push's run, which cancelled the first.
func (w *world) pushedRunCancelled(url, a, b string) error {
	if w.github == nil || w.github.firstLook == nil {
		return errors.New("no push by another clone to cancel the pushed commit's run: say one first")
	}
	before := w.github.firstLook
	g := w.github
	g.firstLook = func() error {
		if err := before(); err != nil {
			return err
		}
		pushed, err := w.head()
		if err != nil {
			return err
		}
		cmd := exec.Command(git.Bin(), "rev-parse", "main")
		cmd.Dir = w.remote()
		cmd.Env = w.env()
		out, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("git rev-parse main in the remote: %w", err)
		}
		newer := strings.TrimSpace(string(out))
		if newer == pushed {
			return fmt.Errorf("the remote's head is still the pushed commit %s: no other clone pushed", pushed)
		}
		g.addRun(pushed, "https://ci.example/runs/cancelled", "cancelled", cancelledJobs())
		g.addRun(newer, url, "success", []watchedJob{job(a, "success"), job(b, "success")})
		return nil
	}
	return nil
}

// ci.watch's provider is github, asking the fake GitHub about ci.yml with no
// interval between looks, and the fake GitHub reports the run at the address
// for the commit it is asked about, its jobs ci and platform succeeding until a
// step says otherwise.
func (w *world) watchAsksFakeGitHub(url string) error {
	w.fakeGitHub()
	w.watchURL = url
	w.config.watch = &watchConfig{provider: "github"}
	if err := w.watchedRunPolls(w.finished(job("ci", "success"), job("platform", "success"))); err != nil {
		return err
	}
	return w.pushConfig("chore: watch CI")
}

// ci.watch.github.nightly_workflow is nightly.yml, whose one run on main the
// fake GitHub holds: at the address, completed, its one job failed, added to
// the ci.watch the Background set up and pushed with it.
func (w *world) nightlyOnFakeGitHub(url, name string) error {
	if w.config.watch == nil || w.github == nil {
		return errors.New("no ci.watch asking a fake GitHub for the nightly to join: set one up first")
	}
	g := w.github
	g.mu.Lock()
	failure := "failure"
	g.runs = append(g.runs, gitHubRun{ID: len(g.runs) + 1, HeadSHA: strings.Repeat("0", 40), HeadBranch: "main",
		Status: "completed", Conclusion: &failure, CreatedAt: "2026-10-05T11:00:00Z", HTMLURL: url,
		workflow: "nightly.yml", jobs: []watchedJob{job(name, "failure")}})
	g.mu.Unlock()
	w.config.watch.nightlyWorkflow = "nightly.yml"
	return w.pushConfig("chore: read the nightly")
}

// Every look at the watched run the fake GitHub gave was of the clone's HEAD,
// by its full SHA, and it gave one.
func (w *world) fakeGitHubGivenHead() error {
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
	if len(given) == 0 {
		return fmt.Errorf("the fake GitHub was never asked for the watched run; it was asked %v\n%s", w.github.requests(), w.report())
	}
	for _, sha := range given {
		if sha != head {
			return fmt.Errorf("the fake GitHub was asked for the run of %q, not HEAD's %s\n%s", sha, head, w.report())
		}
	}
	return nil
}

// The watched run's looks the fake GitHub gives, one a poll, the nth poll
// the nth look, the last look for every poll after it.
func (w *world) watchedRunPolls(runs ...watchedRun) error {
	if w.github == nil {
		return errors.New("no fake GitHub: start one first")
	}
	w.github.mu.Lock()
	w.github.watched = runs
	w.github.mu.Unlock()
	return nil
}

// The config with the scenario's ci.watch, committed in the clone and pushed
// to the remote's main with the hooks left out, so the clone holds no
// uncommitted change, which itos push would refuse.
func (w *world) pushConfig(subject string) error {
	if len(w.ledger) == 0 {
		return errors.New("the ledger has no task for the config's commit to name")
	}
	if err := w.writeConfig(); err != nil {
		return err
	}
	if err := w.git("add", "--", w.data("itos.yaml")); err != nil {
		return err
	}
	if err := w.git("commit", "-q", "--no-verify", "-m", subject+"\n\nTask: "+w.ledger[0].id+"\n"); err != nil {
		return err
	}
	return w.git("push", "-q", "--no-verify", "origin", "HEAD:refs/heads/main")
}

// The scratch config's ci.watch lines, under ci.
func (w *world) watchSection() string {
	c := w.config.watch
	if c == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "  watch:\n    provider: %s\n    interval: 0\n    github:\n      workflow: %s\n", c.provider, watchedWorkflow)
	if c.nightlyWorkflow != "" {
		fmt.Fprintf(&b, "      nightly_workflow: %s\n", c.nightlyWorkflow)
	}
	if c.timeout > 0 {
		fmt.Fprintf(&b, "    timeout: %d\n", c.timeout)
	}
	return b.String()
}

func (w *world) outputSaysOnceBefore(text, later string) error {
	output := w.output()
	if n := strings.Count(output, text); n != 1 {
		return fmt.Errorf("the output says %q %d times, not once\n%s", text, n, w.report())
	}
	at, laterAt := strings.Index(output, text), strings.Index(output, later)
	if laterAt < 0 || at > laterAt {
		return fmt.Errorf("the output does not say %q before %q\n%s", text, later, w.report())
	}
	return nil
}
