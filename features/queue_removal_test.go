package features

import (
	"fmt"
	"slices"
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
	sc.Step(`^the last commit SHA is saved as "([^"]*)"$`, func(name string) error { return saveWorkCommit(w, name) })
	sc.Step(`^the historical removal commit SHA is saved as "([^"]*)"$`, func(name string) error { return saveWorkCommit(w, name) })
	sc.Step(`^that historical removal commit SHA is saved as "([^"]*)"$`, func(name string) error { return saveWorkCommit(w, name) })
	sc.Step(`^the queue does not contain "([^"]*)"$`, w.registryQueueLacks)
	sc.Step(`^the queue is "([^"]*)"$`, w.registryQueueIs)
	sc.Step(`^the last commit changes only the configured registry$`, func() error { return w.lastTouchesOnly("tasks/work-items.yaml") })
	sc.Step(`^exactly one commit was added by the command$`, func() error {
		count, err := w.gitOutput("rev-list", "--count", "HEAD")
		if err != nil {
			return err
		}
		got, err := strconv.Atoi(strings.TrimSpace(count))
		if err != nil {
			return err
		}
		if got != w.queueCommitCount+1 {
			return fmt.Errorf("command added %d commits, not one", got-w.queueCommitCount)
		}
		return nil
	})
	sc.Step(`^the repository state is saved for comparison$`, func() error {
		var err error
		w.queueSavedHead, err = w.gitOutput("rev-parse", "HEAD")
		if err != nil {
			return err
		}
		w.queueSavedFiles, err = w.snapshot()
		return err
	})
	sc.Step(`^the repository files index and HEAD match the saved state$`, func() error {
		head, err := w.gitOutput("rev-parse", "HEAD")
		if err != nil {
			return err
		}
		files, err := w.snapshot()
		if err != nil {
			return err
		}
		if strings.TrimSpace(head) != strings.TrimSpace(w.queueSavedHead) || !mapsEqual(files, w.queueSavedFiles) {
			return fmt.Errorf("repository state differs from saved state\n%s", w.report())
		}
		return nil
	})
	sc.Step(`^the registry's item "([^"]*)" has the status "([^"]*)"$`, func(id, status string) error { return w.registryItemIs(id, status, "") })
	sc.Step(`^all those items are removed by one work-queue command$`, func() error {
		args := append([]string{"work", "queue"}, w.queueIDs...)
		return w.itos(append(args, "--remove")...)
	})
	sc.Step(`^the commit body names every removed item$`, func() error {
		body, err := w.gitOutput("log", "-1", "--format=%b")
		if err != nil {
			return err
		}
		for _, id := range w.queueIDs {
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
		for _, id := range w.queueIDs {
			if slices.Contains(queue, id) {
				return fmt.Errorf("%s remains queued: %v\n%s", id, queue, w.report())
			}
		}
		return nil
	})
	sc.Step(`^no ledger data was changed$`, func() error {
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

func saveWorkCommit(w *world, name string) error {
	sha, err := w.gitOutput("rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if w.workCommits == nil {
		w.workCommits = map[string]string{}
	}
	w.workCommits[name] = strings.TrimSpace(sha)
	return nil
}

func initializeQueueHistoryAssertion(sc *godog.ScenarioContext, w *world) {
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
}

func (w *world) queueFixture(ids []string, status string) error {
	w.queueIDs = slices.Clone(ids)
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
	if err := w.commit("docs: queue fixture"); err != nil {
		return err
	}
	count, err := w.gitOutput("rev-list", "--count", "HEAD")
	if err != nil {
		return err
	}
	w.queueCommitCount, err = strconv.Atoi(strings.TrimSpace(count))
	return err
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

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if b[key] != value {
			return false
		}
	}
	return true
}
