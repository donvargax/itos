package features

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
)

func initializeQueueRemovalSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^a committed queue fixture with the items "([^"]*)" queued in that order$`, func(ids string) error {
		return w.queueFixture(strings.Split(ids, ","), "")
	})
	sc.Step(`^a committed queue fixture with the completed task "([^"]*)" still queued$`, func(id string) error {
		return w.queueFixture([]string{id}, "done")
	})
	sc.Step(`^a committed queue fixture for "([^"]*)"$`, func(ids string) error {
		parts, err := queueIDs(ids)
		if err != nil {
			return err
		}
		return w.queueFixture(parts, "")
	})
	sc.Step(`^a committed queue fixture with "([^"]*)" already removed by a historical "([^"]*)" commit$`, func(id, header string) error {
		if err := w.queueFixture([]string{id}, ""); err != nil {
			return err
		}
		w.registryQueue = nil
		if err := w.writeRegistryLines(); err != nil {
			return err
		}
		return w.commit(header)
	})
	sc.Step(`^the last commit SHA is saved as "([^"]*)"$`, func(name string) error {
		sha, err := w.gitOutput("rev-parse", "HEAD")
		if err != nil {
			return err
		}
		if w.workCommits == nil {
			w.workCommits = map[string]string{}
		}
		w.workCommits[name] = strings.TrimSpace(sha)
		return nil
	})
	sc.Step(`^the historical removal commit SHA is saved as "([^"]*)"$`, func(name string) error {
		sha, err := w.gitOutput("rev-parse", "HEAD")
		if err != nil {
			return err
		}
		if w.workCommits == nil {
			w.workCommits = map[string]string{}
		}
		w.workCommits[name] = strings.TrimSpace(sha)
		return nil
	})
	sc.Step(`^the shown history includes the saved "([^"]*)" commit$`, func(name string) error {
		sha := w.workCommits[name]
		if sha == "" {
			return fmt.Errorf("no commit saved as %q", name)
		}
		if !strings.Contains(w.stdout, sha) {
			return fmt.Errorf("work show history omits %s\n%s", sha, w.report())
		}
		return nil
	})
	sc.Step(`^the commit body names every removed item$`, func() error {
		body, err := w.gitOutput("log", "-1", "--format=%b")
		if err != nil {
			return err
		}
		for _, id := range w.registryQueue {
			if !strings.Contains(body, id) {
				return fmt.Errorf("commit body omits %s\n%s", id, w.report())
			}
		}
		return nil
	})
	sc.Step(`^none of those items remains queued$`, func() error {
		queue, err := w.queueRead()
		if err != nil {
			return err
		}
		for _, id := range w.registryQueue {
			if slicesContains(queue, id) {
				return fmt.Errorf("%s remains queued: %v\n%s", id, queue, w.report())
			}
		}
		return nil
	})
	sc.Step(`^no ledger data was changed$`, func() error {
		// Queue commands must not write the task ledger; check its tracked content.
		out, err := w.gitOutput("diff", "HEAD^", "HEAD", "--", "tasks/phase-1.yaml")
		if err != nil {
			return err
		}
		if out != "" {
			return fmt.Errorf("queue removal changed ledger data\n%s", out)
		}
		return nil
	})
}

func (w *world) queueFixture(ids []string, status string) error {
	for _, id := range ids {
		st := status
		if st == "" {
			st = "todo"
		}
		if err := w.registryItem(id, "null", st, "", ""); err != nil {
			return err
		}
	}
	w.registryQueue = append([]string(nil), ids...)
	if err := w.writeRegistryLines(); err != nil {
		return err
	}
	return w.commit("docs: queue fixture")
}

func queueIDs(spec string) ([]string, error) {
	fields := strings.Fields(spec)
	if len(fields) == 3 && fields[1] == "and" {
		return []string{fields[0], fields[2]}, nil
	}
	if len(fields) == 3 && fields[1] == "to" {
		first, err := strconv.Atoi(strings.TrimPrefix(fields[0], "slice-"))
		if err != nil {
			return nil, err
		}
		last, err := strconv.Atoi(strings.TrimPrefix(fields[2], "slice-"))
		if err != nil {
			return nil, err
		}
		ids := make([]string, 0, last-first+1)
		for n := first; n <= last; n++ {
			ids = append(ids, fmt.Sprintf("slice-%d", n))
		}
		return ids, nil
	}
	return nil, fmt.Errorf("cannot read queue item list %q", spec)
}

func slicesContains(items []string, item string) bool {
	for _, candidate := range items {
		if candidate == item {
			return true
		}
	}
	return false
}
