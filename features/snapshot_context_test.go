// The steps of task-snapshot-context.feature (bug 123): the git context a
// task snapshot holds, read back from what the snapshot probe's git-context
// and write-git modes record in the observation log, and compared with the
// same git commands run in the source checkout or a linked worktree of it; the
// source's HEAD in its three states; an ignored program in the working folder;
// the manifest of the evidence record a JSON named.
package features

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/cucumber/godog"

	"github.com/donvargax/itos/v8/internal/git"
)

func initializeSnapshotContextSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the itos under test is first on the PATH as itos$`, w.itosOnPath)
	sc.Step(`^the source's HEAD is on its branch$`, func() error { return nil })
	sc.Step(`^the source's HEAD is detached at its commit$`, func() error { return w.git("checkout", "-q", "--detach") })
	sc.Step(`^the source's HEAD is unborn, on the new branch "([^"]*)"$`, func(branch string) error {
		return w.git("checkout", "-q", "--orphan", branch)
	})
	sc.Step(`^the working folder has the ignored program "([^"]*)", which exits (\d+)$`, func(program string, code int) error {
		path := filepath.Join(w.dir, filepath.FromSlash(program))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return w.writeProgram(path, fmt.Sprintf("#!/bin/sh\nexit %d\n", code))
	})
	sc.Step(`^the commit-msg hook checks the message in the linked worktree:$`, func(message *godog.DocString) error {
		if w.linked == "" {
			return fmt.Errorf("the scenario adds no linked worktree")
		}
		file := filepath.Join(w.support, "COMMIT_EDITMSG")
		if err := os.WriteFile(file, []byte(message.Content+"\n"), 0o644); err != nil {
			return err
		}
		return w.itosIn(w.linked, "hook", "commit-msg", file)
	})

	sc.Step(`^the snapshot probe ran (\d+) times?$`, func(times int) error {
		runs, err := w.probeRunFolders()
		if err != nil {
			return err
		}
		if len(runs) != times {
			return fmt.Errorf("the snapshot probe ran %d times, not %d", len(runs), times)
		}
		return nil
	})
	sc.Step(`^the probe recorded the source checkout as its execution directory$`, func() error {
		cwd, err := w.snapshotRecord(0, "cwd")
		if err != nil {
			return err
		}
		if realPath(cwd) != realPath(w.dir) {
			return fmt.Errorf("the probe ran in %s, not the source checkout %s", strings.TrimSpace(cwd), w.dir)
		}
		return nil
	})
	sc.Step(`^the probe's last run recorded HEAD, its branch and its parent as (the linked worktree|the saved source state|the source checkout) has them$`, func(of string) error {
		return w.lastRunHas(of, "head", "branch", "parent")
	})
	sc.Step(`^the probe's last run recorded the source checkout's refs, remote config and upstream$`, func() error {
		return w.lastRunHas("the source checkout", "refs", "remotes", "upstream")
	})
	sc.Step(`^the probe's last run recorded the refs, remote config and upstream of the saved source state$`, func() error {
		return w.lastRunHas("the saved source state", "refs", "remotes", "upstream")
	})
	sc.Step(`^the probe's last run recorded the staged tree as its index$`, func() error {
		want, err := w.gitOutput("write-tree")
		if err != nil {
			return err
		}
		got, err := w.lastSnapshotRecord("index")
		if err != nil {
			return err
		}
		if strings.TrimSpace(got) != strings.TrimSpace(want) {
			return fmt.Errorf("the probe's index tree was %q, not the staged %q", strings.TrimSpace(got), strings.TrimSpace(want))
		}
		return nil
	})
	sc.Step(`^the source checkout's local git config matches the saved state$`, func() error {
		got, err := w.gitOutput("config", "--local", "--list")
		if err != nil {
			return err
		}
		if got != w.snapFixture.config {
			return fmt.Errorf("the source's local git config changed: before\n%s\nafter\n%s", w.snapFixture.config, got)
		}
		return nil
	})
	sc.Step(`^the source checkout's objects hold none of the objects the probe wrote$`, func() error {
		written, err := w.lastSnapshotRecord("objects")
		if err != nil {
			return err
		}
		ids := strings.Fields(written)
		if len(ids) != 2 {
			return fmt.Errorf("the probe recorded %d object ids, not 2: %q", len(ids), written)
		}
		for _, id := range ids {
			if _, err := w.gitOutput("cat-file", "-e", id); err == nil {
				return fmt.Errorf("the source checkout holds the object %s the probe wrote", id)
			}
		}
		return nil
	})
	sc.Step(`^the manifest of the record the last JSON named has "([^"]*)" equal to the source checkout's HEAD commit$`, func(field string) error {
		manifest, err := w.namedManifest()
		if err != nil {
			return err
		}
		head, err := w.head()
		if err != nil {
			return err
		}
		if manifest[field] != head {
			return fmt.Errorf("the manifest has %s %v, not HEAD's %s", field, manifest[field], head)
		}
		return nil
	})
	sc.Step(`^the manifest of the record the last JSON named has no "([^"]*)"$`, func(field string) error {
		manifest, err := w.namedManifest()
		if err != nil {
			return err
		}
		if value, has := manifest[field]; has {
			return fmt.Errorf("the manifest has %s %v", field, value)
		}
		return nil
	})
}

// namedManifest is the manifest.json of the record the last JSON's first
// check with evidence names by its key.
func (w *world) namedManifest() (map[string]any, error) {
	var report struct {
		Tasks []struct {
			Checks []struct {
				Evidence *struct {
					Key string `json:"key"`
				} `json:"evidence"`
			} `json:"checks"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal([]byte(w.stdout), &report); err != nil {
		return nil, fmt.Errorf("the output is not JSON: %v\n%s", err, w.report())
	}
	for _, task := range report.Tasks {
		for _, c := range task.Checks {
			if c.Evidence == nil || c.Evidence.Key == "" {
				continue
			}
			common, err := w.commonDir()
			if err != nil {
				return nil, err
			}
			text, err := os.ReadFile(filepath.Join(common, "itos", "evidence", c.Evidence.Key, "manifest.json"))
			if err != nil {
				return nil, err
			}
			var manifest map[string]any
			return manifest, json.Unmarshal(text, &manifest)
		}
	}
	return nil, fmt.Errorf("the JSON names no record\n%s", w.report())
}

// probeRunFolders are the probe's run folders, oldest first.
func (w *world) probeRunFolders() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(w.support, "snapshot-observations"))
	if err != nil {
		return nil, err
	}
	var runs []string
	for i := range entries {
		runs = append(runs, fmt.Sprintf("run-%d", i))
	}
	return runs, nil
}

// snapshotRecord is what the probe's nth run recorded in the file.
func (w *world) snapshotRecord(n int, file string) (string, error) {
	text, err := os.ReadFile(w.probePath(n, file))
	return string(text), err
}

// lastSnapshotRecord is what the probe's last run recorded in the file.
func (w *world) lastSnapshotRecord(file string) (string, error) {
	runs, err := w.probeRunFolders()
	if err != nil {
		return "", err
	}
	if len(runs) == 0 {
		return "", fmt.Errorf("the snapshot probe has not run")
	}
	return w.snapshotRecord(len(runs)-1, file)
}

// lastRunHas is whether what the probe's last run recorded under each name is
// what the git commands of the probe's git-context mode give in the folder of
// the source: the source checkout, the linked worktree or the state saved
// before the run.
func (w *world) lastRunHas(of string, names ...string) error {
	var want map[string]string
	switch of {
	case "the saved source state":
		want = w.snapFixture.context
	case "the linked worktree":
		if w.linked == "" {
			return fmt.Errorf("the scenario adds no linked worktree")
		}
		want = w.gitContextIn(w.linked)
	default:
		want = w.gitContextIn(w.dir)
	}
	for _, name := range names {
		got, err := w.lastSnapshotRecord(name)
		if err != nil {
			return fmt.Errorf("the probe's last run recorded no %s: %w", name, err)
		}
		if normalContext(name, got) != want[name] {
			return fmt.Errorf("the probe's last run recorded %s\n%s\nbut %s has\n%s", name, normalContext(name, got), of, want[name])
		}
	}
	return nil
}

// gitContextIn is what the probe's git-context mode records, except the
// index, by the same git commands run in dir.
func (w *world) gitContextIn(dir string) map[string]string {
	try := func(fallback string, args ...string) string {
		cmd := exec.Command(git.Bin(runtime.GOOS), args...)
		cmd.Dir, cmd.Env = dir, w.env()
		out, err := cmd.Output()
		if err != nil && fallback != "" {
			return fallback
		}
		return string(out)
	}
	context := map[string]string{
		"head":     try("unborn", "rev-parse", "--verify", "-q", "HEAD"),
		"branch":   try("detached", "symbolic-ref", "-q", "HEAD"),
		"parent":   try("none", "rev-parse", "--verify", "-q", "HEAD~1"),
		"refs":     try("", "for-each-ref", "--format=%(refname) %(objectname)"),
		"remotes":  try("", "config", "--get-regexp", `^remote\.`),
		"upstream": try("none", "rev-parse", "--symbolic-full-name", "@{u}"),
	}
	for name, text := range context {
		context[name] = normalContext(name, text)
	}
	return context
}

// normalContext is a recorded value as it is compared: line endings as LF,
// the lines of the refs and the remote config in order, the ends trimmed.
func normalContext(name, text string) string {
	lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n")), "\n")
	if name == "refs" || name == "remotes" {
		sort.Strings(lines)
	}
	return strings.Join(lines, "\n")
}
