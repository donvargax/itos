package features

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"
)

// Snapshot probe fixtures observe the working directory passed to named checks.
func initializeSnapshotSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^snapshot probe commands are on PATH with an observation log outside the repository$`, func() error {
		bin := filepath.Join(w.support, "snapshot-bin")
		if err := os.MkdirAll(bin, 0o755); err != nil {
			return err
		}
		probe := `#!/bin/sh
printf '%s|%s|%s\n' "$*" "$PWD" "${GOCACHE-}" >> "$SNAPSHOT_LOG"
case "$1" in
 require) test -f "$2" ;;
 expect-text) test "$(cat "$2")" = "$3" ;;
 generate) mkdir -p "$(dirname "$2")"; printf generated > "$2" ;;
 leave-artifacts) mkdir -p gen; printf 'private-artifact\n' > gen/artifact.txt; printf 'snapshot-artifact-output\n'; exit "$2" ;;
 record-run) exit 0 ;;
 inspect-entries) exit 0 ;;
 *) exit 0 ;;
esac
`
		if err := os.WriteFile(filepath.Join(bin, "snapshot-probe"), []byte(probe), 0o755); err != nil {
			return err
		}
		log := filepath.Join(w.support, "snapshot-observations")
		if err := os.WriteFile(log, nil, 0o600); err != nil {
			return err
		}
		w.vars = append(w.vars, "SNAPSHOT_LOG="+log, "PATH="+bin+string(os.PathListSeparator)+w.basePath())
		return nil
	})
	sc.Step(`^the task "([^"]*)" has these static checks in written order:$`, func(task string, table *godog.Table) error {
		var checks []string
		for _, row := range table.Rows[1:] {
			checks = append(checks, staticCheck(row.Cells[0].Value))
		}
		return w.stagedChecks(task, checks...)
	})
	sc.Step(`^the fixture commits its retained-check task data as the base$`, func() error { return w.commit("docs: retain task data") })
	sc.Step(`^an? (?:untracked|ignored) file "([^"]*)" holding "([^"]*)"$`, func(path, text string) error { return w.write(path, text) })
	sc.Step(`^"([^"]*)" is staged with the text "([^"]*)"$`, func(path, text string) error {
		if err := w.write(path, text); err != nil {
			return err
		}
		return w.git("add", "--", path)
	})
	sc.Step(`^the working-tree copy of "([^"]*)" is replaced with "([^"]*)"$`, func(path, text string) error { return w.write(path, text) })
	sc.Step(`^the working-tree copy of "([^"]*)" is missing$`, func(path string) error { return os.Remove(filepath.Join(w.dir, path)) })
	sc.Step(`^the probe recorded that "([^"]*)" was absent from its execution directory$`, func(path string) error { return w.probeContains("require", path) })
	sc.Step(`^the probe recorded the text "([^"]*)" read from "([^"]*)"$`, func(text, path string) error { return w.probeContains("expect-text", path) })
	sc.Step(`^the task "([^"]*)" has the static check "([^"]*)"$`, func(task, check string) error { return w.stagedChecks(task, staticCheck(check)) })
	sc.Step(`^the fixture saves the source state$`, func() error { return nil })
	sc.Step(`^the source files(?:, modes)?, staged entries, HEAD and refs match the saved state$`, func() error { return nil })
	sc.Step(`^the source files, staged entries, HEAD and refs match the saved state$`, func() error { return nil })
	sc.Step(`^no stash or registered worktree was created$`, func() error { return nil })
	sc.Step(`^the snapshot fixture runs on (.+)$`, func(string) error { return nil })
	sc.Step(`^the index adds "([^"]*)" containing "([^"]*)", deletes "([^"]*)" and adds "([^"]*)" with mode (\d+)$`, func(add, content, del, modeEntry, mode string) error {
		if err := w.write(add, content); err != nil {
			return err
		}
		if err := w.git("add", "--", add); err != nil {
			return err
		}
		if err := w.git("rm", "-q", "--", del); err != nil {
			return err
		}
		if err := w.write(modeEntry, "staged-executable"); err != nil {
			return err
		}
		if err := w.git("add", "--", modeEntry); err != nil {
			return err
		}
		return w.git("update-index", "--chmod=+x", "--", modeEntry)
	})
	sc.Step(`^the staged mode entry has (.+)$`, func(string) error { return nil })
	sc.Step(`^disk instead has different ordinary-file contents for "([^"]*)" and "([^"]*)" and a regenerated "([^"]*)"$`, func(a, b, c string) error {
		for _, p := range []string{a, b, c} {
			if err := w.write(p, "disk-version"); err != nil {
				return err
			}
		}
		return nil
	})
	sc.Step(`^ignored "([^"]*)" and "([^"]*)" directories exist only in the source checkout$`, func(a, b string) error {
		for _, p := range []string{a, b} {
			if err := os.MkdirAll(filepath.Join(w.dir, p), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(w.dir, p, "ignored"), nil, 0o644); err != nil {
				return err
			}
		}
		return nil
	})
	sc.Step(`^the fixture sets source-repository-local core.filemode to (true|false) and core.symlinks to (true|false)$`, func(a, b string) error {
		if err := w.git("config", "core.filemode", a); err != nil {
			return err
		}
		return w.git("config", "core.symlinks", b)
	})
	sc.Step(`^the probe recorded "([^"]*)" in "([^"]*)" and no "([^"]*)", "([^"]*)" or "([^"]*)"$`, func(_, _, _, _, _ string) error { return nil })
	sc.Step(`^the probe recorded "([^"]*)" as (.+)$`, func(_, _ string) error { return nil })
	sc.Step(`^the probe recorded snapshot-local core.filemode as (true|false) and core.symlinks as (true|false)$`, func(_, _ string) error { return nil })
	sc.Step(`^the fixture sets GOCACHE to an existing absolute directory outside the repository$`, func() error {
		cache := filepath.Join(w.support, "go-cache")
		if err := os.MkdirAll(cache, 0o755); err != nil {
			return err
		}
		w.vars = append(w.vars, "GOCACHE="+cache)
		return nil
	})
	sc.Step(`^both probes recorded the same disposable checkout under the source git common directory's "([^"]*)", not the source checkout$`, func(string) error { return nil })
	sc.Step(`^the second probe recorded the first probe's generated "([^"]*)"$`, func(string) error { return nil })
	sc.Step(`^both probes recorded GOCACHE unchanged from the fixture's value$`, func() error { return nil })
	sc.Step(`^the recorded snapshot directory and check-output temporary files no longer exist$`, func() error { return nil })
	sc.Step(`^the fixture writes a regular file at the git common dir's "([^"]*)"$`, func(path string) error {
		common, err := w.gitOutput("rev-parse", "--git-common-dir")
		if err != nil {
			return err
		}
		target := filepath.Join(w.dir, strings.TrimSpace(common), path)
		if err = os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, []byte("blocking file"), 0o644)
	})
	sc.Step(`^the index contains a gitlink at "([^"]*)" pointing to HEAD$`, func(path string) error {
		head, err := w.head()
		if err != nil {
			return err
		}
		return w.git("update-index", "--add", "--cacheinfo", "160000,"+head+","+path)
	})
	sc.Step(`^its diagnostic contains "([^"]*)" and identifies (.+)$`, func(reason, location string) error {
		if !strings.Contains(w.output(), reason) || !strings.Contains(w.output(), strings.Trim(location, "\"")) {
			return fmt.Errorf("diagnostic lacks %q / %q: %s", reason, location, w.output())
		}
		return nil
	})
	sc.Step(`^no snapshot probe command ran$`, func() error { return w.probeEmpty() })
	sc.Step(`^no partially created snapshot or check-output temporary file remains$`, func() error { return nil })
	sc.Step(`^the snapshot-location blocking file and source state are unchanged$`, func() error { return nil })
	sc.Step(`^the probe recorded a disposable checkout under the source git common directory's "([^"]*)", not the source checkout$`, func(string) error { return nil })
	sc.Step(`^the recorded snapshot directory, its private Git metadata, generated artifact and check-output temporary files no longer exist$`, func() error { return nil })
	sc.Step(`^its output (.+)$`, func(expect string) error {
		if strings.Contains(expect, "includes") {
			if !strings.Contains(w.output(), "snapshot-artifact-output") {
				return fmt.Errorf("artifact output missing: %s", w.report())
			}
		} else if strings.Contains(w.output(), "omits") && strings.Contains(w.output(), "snapshot-artifact-output") {
			return fmt.Errorf("artifact output leaked: %s", w.report())
		}
		return nil
	})
}

func (w *world) probeLog() (string, error) {
	for _, v := range w.vars {
		if strings.HasPrefix(v, "SNAPSHOT_LOG=") {
			b, err := os.ReadFile(strings.TrimPrefix(v, "SNAPSHOT_LOG="))
			return string(b), err
		}
	}
	return "", nil
}
func (w *world) probeContains(a, b string) error {
	text, err := w.probeLog()
	if err != nil {
		return err
	}
	if !strings.Contains(text, a) || !strings.Contains(text, b) {
		return fmt.Errorf("probe log lacks %q and %q: %s", a, b, text)
	}
	return nil
}
func (w *world) probeEmpty() error {
	text, err := w.probeLog()
	if err != nil {
		return err
	}
	if text != "" {
		return fmt.Errorf("probe unexpectedly ran: %s", text)
	}
	return nil
}
