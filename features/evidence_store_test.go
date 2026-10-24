// The steps of evidence-store.feature (slice 170): the fixture plays the other
// writer, the lock a killed itos left and the damage a record can suffer, and
// the readings of what the store holds besides records. The evidence probe is
// evidence_test.go's.
package features

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

// evidenceFixture is what the fixture holds of the store: where it moved the
// kept key's record aside and what that record held, and the itos it started
// in the background.
type evidenceFixture struct {
	aside string
	files map[string][]byte
	bg    *backgroundItos
}

// backgroundItos is an itos started and not yet waited for.
type backgroundItos struct {
	done           chan error
	stdout, stderr *bytes.Buffer
}

// quarantined names a quarantined entry: <key>.<reason>-<UTC timestamp>.
var quarantined = regexp.MustCompile(`^[0-9a-f]+\.(markerless|corrupt)-[0-9]{8}T[0-9]{6}`)

func initializeEvidenceStoreSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the fixture moves the kept key's record aside$`, w.moveRecordAside)
	sc.Step(`^the fixture makes the kept key's lock file$`, func() error {
		store, err := w.storeDir()
		if err != nil {
			return err
		}
		return os.WriteFile(w.keyPath(store, ".lock"), []byte("1\n"), 0o600)
	})
	sc.Step(`^the kept key's lock file is still there$`, func() error {
		store, err := w.storeDir()
		if err != nil {
			return err
		}
		if _, err := os.Stat(w.keyPath(store, ".lock")); err != nil {
			return fmt.Errorf("the kept key's lock file is gone: %w", err)
		}
		return nil
	})
	sc.Step(`^itos runs "([^"]*)" in the background$`, func(line string) error {
		args, err := shellWords(line)
		if err != nil {
			return err
		}
		return w.startBackground(args)
	})
	sc.Step(`^while that itos still waits, the fixture puts the kept key's record back and removes its lock file$`, w.releaseWaiter)
	sc.Step(`^the background itos finishes$`, w.finishBackground)

	sc.Step(`^the fixture removes the completion marker from the kept key's record$`, func() error {
		return w.damage(func(at string) error { return os.Remove(filepath.Join(at, "complete")) })
	})
	sc.Step(`^the fixture writes a NUL byte into the kept key's manifest$`, func() error {
		return w.damage(func(at string) error { return os.WriteFile(filepath.Join(at, "manifest.json"), []byte{0}, 0o600) })
	})
	sc.Step(`^the fixture replaces the kept key's record with a regular file$`, func() error {
		return w.damage(func(at string) error {
			if err := os.RemoveAll(at); err != nil {
				return err
			}
			return os.WriteFile(at, []byte("not a record"), 0o600)
		})
	})
	sc.Step(`^the fixture moves the kept key's record into an unfinished temporary folder of the store$`, func() error {
		store, err := w.storeDir()
		if err != nil {
			return err
		}
		return os.Rename(w.keyPath(store, ""), filepath.Join(store, ".tmp-"+w.evidenceKey+"-unfinished"))
	})

	sc.Step(`^the check, as it runs, will put (the record moved aside|a regular file|the record moved aside, less its completion marker) at the kept key$`, w.orderPlanting)
	sc.Step(`^the kept key's record is complete$`, func() error {
		store, err := w.storeDir()
		if err != nil {
			return err
		}
		for _, f := range []string{"manifest.json", "result.json", "complete"} {
			if _, err := os.Stat(filepath.Join(w.keyPath(store, ""), f)); err != nil {
				return fmt.Errorf("the kept key's record lacks %s: %w", f, err)
			}
		}
		return nil
	})
	sc.Step(`^the kept key's record is the one the fixture moved aside$`, func() error {
		store, err := w.storeDir()
		if err != nil {
			return err
		}
		for name, want := range w.evidenceFix().files {
			got, err := os.ReadFile(filepath.Join(w.keyPath(store, ""), name))
			if err != nil {
				return err
			}
			if !bytes.Equal(got, want) {
				return fmt.Errorf("the kept key's %s is not the one the fixture moved aside", name)
			}
		}
		return nil
	})
	sc.Step(`^its JSON's evidence for the task "([^"]*)" has "reason" "([^"]*)" and the kept record's time$`, w.evidenceKeptRecordsTime)
	sc.Step(`^its JSON's evidence for the task "([^"]*)" has "recorded" true and no "reason"$`, func(task string) error {
		if err := w.evidenceOfTaskIs(task, "recorded", true); err != nil {
			return err
		}
		evidence, err := w.evidenceOfTask(task)
		if err != nil {
			return err
		}
		if reason, has := evidence["reason"]; has {
			return fmt.Errorf("the task %s's evidence has the reason %v\n%s", task, reason, w.report())
		}
		return nil
	})
	sc.Step(`^the evidence store holds no quarantined entry$`, func() error {
		got, err := w.quarantinedEntries()
		if err != nil {
			return err
		}
		if len(got) != 0 {
			return fmt.Errorf("the evidence store holds quarantined entries: %v", got)
		}
		return nil
	})
	sc.Step(`^the evidence store holds the kept key's entry quarantined as "([^"]*)"$`, w.quarantinedAs)
	sc.Step(`^the evidence store holds no temporary folder$`, func() error {
		store, err := w.storeDir()
		if err != nil {
			return err
		}
		entries, err := os.ReadDir(store)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".tmp-") {
				return fmt.Errorf("the evidence store holds the temporary folder %s", e.Name())
			}
		}
		return nil
	})
	sc.Step(`^the git common dir's "([^"]*)" is still the fixture's regular file$`, func(p string) error {
		common, err := w.commonDir()
		if err != nil {
			return err
		}
		at := filepath.Join(common, filepath.FromSlash(p))
		info, err := os.Lstat(at)
		if err != nil {
			return err
		}
		text, _ := os.ReadFile(at)
		if !info.Mode().IsRegular() || string(text) != "blocking file" {
			return fmt.Errorf("%s is not the fixture's regular file", p)
		}
		return nil
	})
}

func (w *world) evidenceFix() *evidenceFixture {
	if w.evidenceStore == nil {
		w.evidenceStore = &evidenceFixture{}
	}
	return w.evidenceStore
}

// storeDir is the evidence store, in the git common dir.
func (w *world) storeDir() (string, error) {
	common, err := w.commonDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(common, "itos", "evidence"), nil
}

// keyPath is the kept key's entry in the store with a suffix (".lock" for its
// lock), the record itself with none.
func (w *world) keyPath(store, suffix string) string {
	return filepath.Join(store, w.evidenceKey+suffix)
}

// moveRecordAside moves the kept key's record out of the store, keeping what
// its manifest and result held.
func (w *world) moveRecordAside() error {
	store, err := w.storeDir()
	if err != nil {
		return err
	}
	fix := w.evidenceFix()
	fix.aside = filepath.Join(w.support, "evidence-aside", w.evidenceKey)
	if err := os.MkdirAll(filepath.Dir(fix.aside), 0o755); err != nil {
		return err
	}
	fix.files = map[string][]byte{}
	for _, name := range []string{"manifest.json", "result.json"} {
		if fix.files[name], err = os.ReadFile(filepath.Join(w.keyPath(store, ""), name)); err != nil {
			return err
		}
	}
	return os.Rename(w.keyPath(store, ""), fix.aside)
}

// orderPlanting leaves the probe the order to put something at the kept key
// on its next run, once: the record moved aside, with or without its marker,
// or a regular file.
func (w *world) orderPlanting(what string) error {
	store, err := w.storeDir()
	if err != nil {
		return err
	}
	at := filepath.ToSlash(w.keyPath(store, ""))
	aside := filepath.ToSlash(w.evidenceFix().aside)
	var order string
	switch what {
	case "a regular file":
		order = fmt.Sprintf("printf 'planted' > '%s'\n", at)
	case "the record moved aside":
		order = fmt.Sprintf("mv '%s' '%s'\n", aside, at)
	default:
		order = fmt.Sprintf("mv '%s' '%s' && rm '%s/complete'\n", aside, at, at)
	}
	return os.WriteFile(filepath.Join(w.evidenceLog(), "order"), []byte(order), 0o644)
}

// damage changes the kept key's record in place.
func (w *world) damage(change func(at string) error) error {
	store, err := w.storeDir()
	if err != nil {
		return err
	}
	return change(w.keyPath(store, ""))
}

// startBackground starts itos with the arguments and does not wait for it.
func (w *world) startBackground(args []string) error {
	w.lastCommand = strings.Join(args, " ") + " (background)"
	w.markRun()
	cmd := exec.Command(w.bin, args...)
	cmd.Dir = w.dir
	cmd.Env = w.env()
	bg := &backgroundItos{done: make(chan error, 1), stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}
	cmd.Stdout, cmd.Stderr = bg.stdout, bg.stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting itos: %w", err)
	}
	go func() { bg.done <- cmd.Wait() }()
	w.evidenceFix().bg = bg
	return nil
}

// releaseWaiter waits a second, fails if the background itos has exited by
// then (it did not wait), puts the kept key's record back and then removes
// its lock file, in that order, so that the itos that takes the lock finds
// the record.
func (w *world) releaseWaiter() error {
	fix := w.evidenceFix()
	if fix.bg == nil {
		return fmt.Errorf("no itos is running in the background")
	}
	select {
	case err := <-fix.bg.done:
		fix.bg.done <- err
		w.stdout, w.stderr = fix.bg.stdout.String(), fix.bg.stderr.String()
		return fmt.Errorf("the background itos exited before the lock was released: it did not wait\n%s", w.report())
	case <-time.After(time.Second):
	}
	store, err := w.storeDir()
	if err != nil {
		return err
	}
	if err := os.Rename(fix.aside, w.keyPath(store, "")); err != nil {
		return err
	}
	return os.Remove(w.keyPath(store, ".lock"))
}

// finishBackground waits for the background itos and keeps its exit code and
// output as the last run's.
func (w *world) finishBackground() error {
	fix := w.evidenceFix()
	if fix.bg == nil {
		return fmt.Errorf("no itos is running in the background")
	}
	var err error
	select {
	case err = <-fix.bg.done:
	case <-time.After(time.Minute):
		return fmt.Errorf("the background itos did not finish in a minute")
	}
	w.stdout, w.stderr = fix.bg.stdout.String(), fix.bg.stderr.String()
	w.exit = 0
	if exit, ok := err.(*exec.ExitError); ok {
		w.exit = exit.ExitCode()
	} else if err != nil {
		return err
	}
	fix.bg = nil
	return nil
}

// quarantinedEntries are the entries of the store that were renamed aside.
func (w *world) quarantinedEntries() ([]string, error) {
	store, err := w.storeDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(store)
	if err != nil {
		return nil, err
	}
	var got []string
	for _, e := range entries {
		if quarantined.MatchString(e.Name()) {
			got = append(got, e.Name())
		}
	}
	return got, nil
}

// quarantinedAs is whether the kept key has exactly one quarantined entry,
// under the reason, a markerless one a folder without its marker.
func (w *world) quarantinedAs(reason string) error {
	got, err := w.quarantinedEntries()
	if err != nil {
		return err
	}
	var mine []string
	for _, name := range got {
		if strings.HasPrefix(name, w.evidenceKey+".") {
			mine = append(mine, name)
		}
	}
	if len(mine) != 1 || !strings.HasPrefix(mine[0], w.evidenceKey+"."+reason+"-") {
		return fmt.Errorf("the kept key's quarantined entries are %v, not one under %q", mine, reason)
	}
	if reason == "markerless" {
		store, err := w.storeDir()
		if err != nil {
			return err
		}
		if _, err := os.Stat(filepath.Join(store, mine[0], "complete")); err == nil {
			return fmt.Errorf("the markerless entry %s holds a completion marker", mine[0])
		}
	}
	return nil
}

// evidenceKeptRecordsTime is whether the task's evidence has the reason and
// the time the kept record, the one the fixture moved aside, ended at, with
// hit and recorded false.
func (w *world) evidenceKeptRecordsTime(task, reason string) error {
	for field, want := range map[string]any{"reason": reason, "hit": false, "recorded": false} {
		if err := w.evidenceOfTaskIs(task, field, want); err != nil {
			return err
		}
	}
	var kept struct {
		Ended string `json:"ended"`
	}
	if err := json.Unmarshal(w.evidenceFix().files["result.json"], &kept); err != nil || kept.Ended == "" {
		return fmt.Errorf("the record the fixture moved aside has no end time: %v", err)
	}
	return w.evidenceOfTaskIs(task, "recorded_at", kept.Ended)
}
