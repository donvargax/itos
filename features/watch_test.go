// The steps of itos push waiting for CI and itos ci watch (watch.feature):
// ci.watch's command is a script of the scenario's that reports a run it was
// given, one JSON object a poll, and records the commit it was asked about,
// so nothing reaches a network; the config's ci.watch is committed and
// pushed to the remote, as a project's config would be there already.
package features

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cucumber/godog"
)

// What a scenario sets of the scratch config's ci.watch.
type watchConfig struct {
	provider string // ci.watch.provider
	timeout  int    // ci.watch.timeout, when above 0
	nightly  bool   // whether ci.watch.nightly_command runs the nightly's script
	github   bool   // whether it asks the fake GitHub
	// ci.watch.github.nightly_workflow, when set
	nightlyWorkflow string
}

// The workflow ci.watch asks the fake GitHub about.
const watchedWorkflow = "ci.yml"

// One look at the watched run, as the watch command prints it.
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
	sc.Step(`^ci\.watch runs a command that reports the run "([^"]*)"$`, w.watchCommandReports)
	sc.Step(`^ci\.watch\.nightly_command reports the run "([^"]*)", its job "([^"]*)" failed$`, w.nightlyCommandReports)
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
	sc.Step(`^the watch command prints "([^"]*)"$`, func(text string) error {
		return w.watchScript("printf '%s\\n' " + quote(text) + "\n")
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
	sc.Step(`^the watch command was given the full SHA of the clone's HEAD$`, w.watchGivenHead)
	sc.Step(`^the watch command was never run$`, func() error {
		if text, err := os.ReadFile(w.watchRecord()); err == nil {
			return fmt.Errorf("the watch command ran, given:\n%s\n%s", text, w.report())
		}
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
	sc.Step(`^no GitHub token in the environment$`, func() error {
		w.vars = slices.DeleteFunc(w.vars, func(v string) bool { return strings.HasPrefix(v, "GITHUB_TOKEN=") })
		return nil
	})
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

// The file the watch command records each commit it is given in, one a line.
func (w *world) watchRecord() string { return filepath.Join(w.support, "watch-given") }

// The watch command's script.
func (w *world) watchScriptPath() string { return filepath.Join(w.support, "watch.sh") }

// ci.watch's provider is command, run with no interval between polls, and
// the script reports the run at the address, its jobs ci and platform
// succeeding until a step says otherwise.
func (w *world) watchCommandReports(url string) error {
	w.watchURL = url
	w.config.watch = &watchConfig{provider: "command"}
	if err := w.watchedRunPolls(w.finished(job("ci", "success"), job("platform", "success"))); err != nil {
		return err
	}
	return w.pushConfig("chore: watch CI")
}

// The nightly's script, which ci.watch.nightly_command runs.
func (w *world) nightlyScriptPath() string { return filepath.Join(w.support, "nightly.sh") }

// ci.watch.nightly_command runs a script that reports the nightly's run at
// the address, completed, its one job failed, added to the ci.watch the
// Background set up and pushed with it.
func (w *world) nightlyCommandReports(url, name string) error {
	if w.config.watch == nil {
		return errors.New("no ci.watch for the nightly command to join: set one up first")
	}
	text, err := json.Marshal(watchedRun{URL: url, Status: "completed", Conclusion: "failure",
		Jobs: []watchedJob{job(name, "failure")}})
	if err != nil {
		return err
	}
	script := "#!/bin/sh\nprintf '%s\\n' " + quote(string(text)) + "\n"
	if err := os.WriteFile(w.nightlyScriptPath(), []byte(script), 0o755); err != nil {
		return err
	}
	w.config.watch.nightly = true
	return w.pushConfig("chore: read the nightly")
}

// ci.watch's provider is github, asking the fake GitHub about ci.yml with no
// interval between looks, and the fake GitHub reports the run at the address
// for the commit it is asked about, its jobs ci and platform succeeding until a
// step says otherwise.
func (w *world) watchAsksFakeGitHub(url string) error {
	w.fakeGitHub()
	w.watchURL = url
	w.config.watch = &watchConfig{provider: "github", github: true}
	if err := w.watchedRunPolls(w.finished(job("ci", "success"), job("platform", "success"))); err != nil {
		return err
	}
	return w.pushConfig("chore: watch CI")
}

// ci.watch.github.nightly_workflow is nightly.yml, whose one run on main the
// fake GitHub holds: at the address, completed, its one job failed, added to
// the ci.watch the Background set up and pushed with it.
func (w *world) nightlyOnFakeGitHub(url, name string) error {
	if w.config.watch == nil || !w.config.watch.github {
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

// The watched run's looks, one a poll, the nth poll the nth look, the last
// look for every poll after it: the fake GitHub's, when ci.watch asks it, else
// the watch command's script prints them.
func (w *world) watchedRunPolls(runs ...watchedRun) error {
	if w.config.watch != nil && w.config.watch.github {
		w.github.mu.Lock()
		w.github.watched = runs
		w.github.mu.Unlock()
		return nil
	}
	var b strings.Builder
	b.WriteString("n=$(wc -l < " + quote(w.watchRecord()) + " | tr -d ' ')\ncase \"$n\" in\n")
	for i, run := range runs {
		text, err := json.Marshal(run)
		if err != nil {
			return err
		}
		pattern := fmt.Sprint(i + 1)
		if i == len(runs)-1 {
			pattern = "*"
		}
		fmt.Fprintf(&b, "  %s) printf '%%s\\n' %s ;;\n", pattern, quote(string(text)))
	}
	b.WriteString("esac\n")
	return w.watchScript(b.String())
}

// The watch command's script: it records the commit it was given, then runs
// the body.
func (w *world) watchScript(body string) error {
	script := "#!/bin/sh\nprintf '%s\\n' \"$1\" >> " + quote(w.watchRecord()) + "\n" + body
	return os.WriteFile(w.watchScriptPath(), []byte(script), 0o755)
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
	if c.github {
		fmt.Fprintf(&b, "  watch:\n    provider: %s\n    interval: 0\n    github:\n      workflow: %s\n", c.provider, watchedWorkflow)
		if c.nightlyWorkflow != "" {
			fmt.Fprintf(&b, "      nightly_workflow: %s\n", c.nightlyWorkflow)
		}
		if c.timeout > 0 {
			fmt.Fprintf(&b, "    timeout: %d\n", c.timeout)
		}
		return b.String()
	}
	fmt.Fprintf(&b, "  watch:\n    provider: %s\n    command: %q\n    interval: 0\n",
		c.provider, "sh "+quote(w.watchScriptPath())+" {sha}")
	if c.timeout > 0 {
		fmt.Fprintf(&b, "    timeout: %d\n", c.timeout)
	}
	if c.nightly {
		fmt.Fprintf(&b, "    nightly_command: %q\n", "sh "+quote(w.nightlyScriptPath()))
	}
	return b.String()
}

// Every commit the watch command was given is the clone's HEAD, in full,
// and it was given one.
func (w *world) watchGivenHead() error {
	head, err := w.head()
	if err != nil {
		return err
	}
	text, err := os.ReadFile(w.watchRecord())
	if err != nil {
		return fmt.Errorf("the watch command never ran\n%s", w.report())
	}
	for _, line := range strings.Split(strings.TrimSpace(string(text)), "\n") {
		if line != head {
			return fmt.Errorf("the watch command was given %q, not HEAD's %s\n%s", line, head, w.report())
		}
	}
	return nil
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
