// The steps of evidence-local.feature (slice 169): the evidence probe, a
// program the scenarios put on the PATH that logs each of its runs outside
// the repository, and the readings of the evidence records in the git
// common dir and of the evidence an itos JSON names. The probe's log, not
// the speed or the wording of a run, is what shows a hit.
package features

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
)

// evidenceProbe is evidence-probe <exit> [<path>...] and evidence-probe mark
// <path>. Each run makes the folder run-<n> in the log and records there its
// arguments (args), the folder it ran in (cwd) and, for each path, its text
// less the final newline (read-<path>, with / as _) or that it is absent
// (absent-<path>). It prints "<path>: <text>" lines and exits <exit>. mark
// writes <path> holding "mark" in the folder it runs in.
const evidenceProbe = `#!/bin/sh
n=0
while test -d "$EVIDENCE_LOG/run-$n"; do n=$((n + 1)); done
out="$EVIDENCE_LOG/run-$n"
mkdir "$out" || exit 90
printf '%s' "$*" > "$out/args"
if test -f "$EVIDENCE_LOG/order"; then
 mv "$EVIDENCE_LOG/order" "$EVIDENCE_LOG/order-done" || exit 91
 . "$EVIDENCE_LOG/order-done" || exit 91
fi
{ pwd -W 2>/dev/null || pwd -P; } > "$out/cwd"
code="$1"
shift
if test "$code" = mark; then
 printf mark > "$1"
 exit 0
fi
nl='
'
for p in "$@"; do
 f=$p
 while case $f in */*) true;; *) false;; esac; do f=${f%%/*}_${f#*/}; done
 if test -f "$p"; then
  t=
  while IFS= read -r line; do t=$t$line$nl; done < "$p"
  t=$t$line
  while case $t in *"$nl") true;; *) false;; esac; do t=${t%"$nl"}; done
  printf '%s' "$t" > "$out/read-$f"
  printf '%s: %s\n' "$p" "$t"
 else
  : > "$out/absent-$f"
  printf '%s: absent\n' "$p"
 fi
done
exit "$code"
`

func initializeEvidenceSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the evidence probe is on PATH, logging its runs outside the repository$`, func() error {
		common, err := w.gitOutput("rev-parse", "--path-format=absolute", "--git-common-dir")
		if err != nil {
			return err
		}
		w.snapFixture = &snapshotFixture{common: strings.TrimSpace(common)}
		bin, err := w.binOnPath()
		if err != nil {
			return err
		}
		log := w.evidenceLog()
		if err := os.MkdirAll(log, 0o755); err != nil {
			return err
		}
		if err := w.writeProgram(filepath.Join(bin, "evidence-probe"), evidenceProbe); err != nil {
			return err
		}
		w.vars = append(w.vars, "EVIDENCE_LOG="+filepath.ToSlash(log))
		return nil
	})
	sc.Step(`^the evidence probe on PATH is rewritten with the same behaviour in different bytes$`, func() error {
		bin, err := w.binOnPath()
		if err != nil {
			return err
		}
		return w.writeProgram(filepath.Join(bin, "evidence-probe"), evidenceProbe+"# the same, rewritten\n")
	})
	sc.Step(`^the CI steps are the YAML:$`, func(steps *godog.DocString) error {
		w.config.ciStepsYAML = strings.Split(strings.TrimRight(steps.Content, "\n"), "\n")
		return w.writeConfig()
	})
	sc.Step(`^the empty untracked file "([^"]*)"$`, func(path string) error { return w.write(path, "") })
	sc.Step(`^the working-tree ledger's check of "([^"]*)" is changed to run "([^"]*)"$`, w.ledgerCheckChangedTo)
	sc.Step(`^itos runs CI over the commits after the first with --json$`, func() error {
		if len(w.commits) == 0 {
			return fmt.Errorf("the repository has no commit yet")
		}
		return w.itos("ci", "run", w.commits[0], "HEAD", "--json")
	})

	sc.Step(`^the evidence probe ran (\d+) times?$`, func(times int) error { return w.probeRan("", times) })
	sc.Step(`^the evidence probe ran "([^"]*)" (\d+) times?$`, w.probeRan)
	sc.Step(`^the evidence probe's last run was in the working folder$`, func() error {
		cwd, err := w.lastProbeRun("cwd")
		if err != nil {
			return err
		}
		if realPath(cwd) != realPath(w.dir) {
			return fmt.Errorf("the probe's last run was in %s, not the working folder %s", cwd, w.dir)
		}
		return nil
	})
	sc.Step(`^the evidence probe's last run was in a disposable checkout under the git common dir's "([^"]*)"$`, func(sub string) error {
		cwd, err := w.lastProbeRun("cwd")
		if err != nil {
			return err
		}
		common, err := w.commonDir()
		if err != nil {
			return err
		}
		base := realPath(filepath.Join(common, sub))
		rel, err := filepath.Rel(base, realPath(cwd))
		if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("the probe's last run was in %s, not in a checkout under %s", cwd, base)
		}
		return nil
	})
	sc.Step(`^the evidence probe's last run read "([^"]*)" from "([^"]*)"$`, func(text, path string) error {
		got, err := w.lastProbeRun("read-" + probeName(path))
		if err != nil {
			return fmt.Errorf("the probe's last run did not read %s: %w", path, err)
		}
		if got != text {
			return fmt.Errorf("the probe's last run read %q from %s, not %q", got, path, text)
		}
		return nil
	})
	sc.Step(`^the evidence probe's last run found no "([^"]*)"$`, func(path string) error {
		if _, err := w.lastProbeRun("absent-" + probeName(path)); err != nil {
			return fmt.Errorf("the probe's last run did not find %s absent: %w", path, err)
		}
		return nil
	})

	sc.Step(`^the evidence store holds (\d+) complete records?$`, func(want int) error {
		got, err := w.evidenceRecords(true)
		if err != nil {
			return err
		}
		if len(got) != want {
			return fmt.Errorf("the evidence store holds %d complete records, not %d: %v", len(got), want, got)
		}
		return nil
	})
	initializeEvidenceStoreSteps(sc, w)
	sc.Step(`^the evidence store holds no record$`, func() error {
		got, err := w.evidenceRecords(false)
		if err != nil {
			return err
		}
		if len(got) != 0 {
			return fmt.Errorf("the evidence store holds records: %v", got)
		}
		return nil
	})

	sc.Step(`^its JSON's evidence for the task "([^"]*)" has "([^"]*)" (true|false)$`, func(task, field, want string) error {
		return w.evidenceOfTaskIs(task, field, want == "true")
	})
	sc.Step(`^its JSON's evidence for the task "([^"]*)" has "([^"]*)" "([^"]*)"$`, func(task, field, want string) error {
		return w.evidenceOfTaskIs(task, field, want)
	})
	sc.Step(`^its JSON's evidence for the task "([^"]*)" has a key, which the fixture keeps$`, func(task string) error {
		evidence, err := w.evidenceOfTask(task)
		if err != nil {
			return err
		}
		key, _ := evidence["key"].(string)
		if key == "" {
			return fmt.Errorf("the task %s's evidence names no key\n%s", task, w.report())
		}
		w.evidenceKey = key
		return nil
	})
	sc.Step(`^its JSON's evidence for the task "([^"]*)" names (HEAD's tree|the index tree, which is not HEAD's)$`, func(task, which string) error {
		evidence, err := w.evidenceOfTask(task)
		if err != nil {
			return err
		}
		tree, _ := evidence["tree"].(string)
		head, err := w.gitOutput("rev-parse", "HEAD^{tree}")
		if err != nil {
			return err
		}
		head = strings.TrimSpace(head)
		if which == "HEAD's tree" {
			if tree != head {
				return fmt.Errorf("the evidence names the tree %q, not HEAD's %s\n%s", tree, head, w.report())
			}
			return nil
		}
		index, err := w.gitOutput("write-tree")
		if err != nil {
			return err
		}
		if index = strings.TrimSpace(index); tree != index || tree == head {
			return fmt.Errorf("the evidence names the tree %q, not the index's %s, which is not HEAD's %s\n%s", tree, index, head, w.report())
		}
		return nil
	})
	sc.Step(`^its JSON's evidence for the tasks "([^"]*)" and "([^"]*)" has (the same key|different keys)$`, func(a, b, which string) error {
		first, err := w.evidenceOfTask(a)
		if err != nil {
			return err
		}
		second, err := w.evidenceOfTask(b)
		if err != nil {
			return err
		}
		ka, _ := first["key"].(string)
		kb, _ := second["key"].(string)
		if ka == "" || kb == "" || (ka == kb) != (which == "the same key") {
			return fmt.Errorf("the keys of %s and %s are %q and %q, not %s\n%s", a, b, ka, kb, which, w.report())
		}
		return nil
	})
	sc.Step(`^its JSON's evidence for the CI command "([^"]*)" has "([^"]*)" (true|false)$`, func(command, field, want string) error {
		var report struct {
			Evidence []map[string]any `json:"evidence"`
		}
		if err := json.Unmarshal([]byte(w.stdout), &report); err != nil {
			return fmt.Errorf("the output is not JSON: %v\n%s", err, w.report())
		}
		for _, e := range report.Evidence {
			if e["command"] == command {
				if e[field] != (want == "true") {
					return fmt.Errorf("the CI command %q has %s %v, not %s\n%s", command, field, e[field], want, w.report())
				}
				return nil
			}
		}
		return fmt.Errorf("the JSON has no evidence for the CI command %q\n%s", command, w.report())
	})
	sc.Step(`^its JSON gives the task "([^"]*)"'s check "([^"]*)" no evidence$`, func(task, command string) error {
		checks, err := w.checksOfTask(task)
		if err != nil {
			return err
		}
		for _, c := range checks {
			if c["command"] == command {
				if _, has := c["evidence"]; has {
					return fmt.Errorf("the check %q has evidence\n%s", command, w.report())
				}
				return nil
			}
		}
		return fmt.Errorf("the task %s has no check %q in the JSON\n%s", task, command, w.report())
	})
	sc.Step(`^its JSON has no "([^"]*)"$`, func(key string) error {
		var object map[string]json.RawMessage
		if err := json.Unmarshal([]byte(w.stdout), &object); err != nil {
			return fmt.Errorf("the output is not JSON: %v\n%s", err, w.report())
		}
		if _, has := object[key]; has {
			return fmt.Errorf("the JSON has %q\n%s", key, w.report())
		}
		return nil
	})
}

// evidenceLog is where the probe logs, outside the repository.
func (w *world) evidenceLog() string { return filepath.Join(w.support, "evidence-probe-log") }

// commonDir is the scratch repository's git common dir, absolute.
func (w *world) commonDir() (string, error) {
	common, err := w.gitOutput("rev-parse", "--path-format=absolute", "--git-common-dir")
	return strings.TrimSpace(common), err
}

// realPath is a path with its links and short names resolved as far as it
// exists: a disposable checkout is gone once the run that made it ends, so
// its nearest folder that is still there is resolved and the rest kept.
func realPath(path string) string {
	path = filepath.Clean(filepath.FromSlash(strings.TrimSpace(path)))
	rest := ""
	for p := path; ; p = filepath.Dir(p) {
		if resolved, err := filepath.EvalSymlinks(p); err == nil {
			return filepath.Join(resolved, rest)
		}
		if filepath.Dir(p) == p {
			return path
		}
		rest = filepath.Join(filepath.Base(p), rest)
	}
}

// probeName is the file name the probe records a path under.
func probeName(path string) string { return strings.ReplaceAll(path, "/", "_") }

// probeRuns are the probe's run folders, in the order the runs happened.
func (w *world) probeRuns() ([]string, error) {
	entries, err := os.ReadDir(w.evidenceLog())
	if err != nil {
		return nil, err
	}
	var runs []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "run-") {
			runs = append(runs, e.Name())
		}
	}
	number := func(run string) int { n, _ := strconv.Atoi(strings.TrimPrefix(run, "run-")); return n }
	sort.Slice(runs, func(i, j int) bool { return number(runs[i]) < number(runs[j]) })
	return runs, nil
}

// probeRan is whether the probe ran this many times, those with these
// arguments when they are given.
func (w *world) probeRan(args string, times int) error {
	runs, err := w.probeRuns()
	if err != nil {
		return err
	}
	n := 0
	for _, run := range runs {
		got, err := os.ReadFile(filepath.Join(w.evidenceLog(), run, "args"))
		if err != nil {
			return err
		}
		if args == "" || string(got) == args {
			n++
		}
	}
	if n != times {
		what := "the probe"
		if args != "" {
			what = fmt.Sprintf("the probe with %q", args)
		}
		return fmt.Errorf("%s ran %d times, not %d (%d runs in all)", what, n, times, len(runs))
	}
	return nil
}

// lastProbeRun is what the probe's last run recorded in the file.
func (w *world) lastProbeRun(file string) (string, error) {
	runs, err := w.probeRuns()
	if err != nil {
		return "", err
	}
	if len(runs) == 0 {
		return "", fmt.Errorf("the probe has not run")
	}
	got, err := os.ReadFile(filepath.Join(w.evidenceLog(), runs[len(runs)-1], file))
	return string(got), err
}

// evidenceRecords are the folders of the store that are records: not a
// temporary folder, which begins with a dot, nor a quarantined one, whose
// name holds one, and, when complete is asked, holding the completion marker.
func (w *world) evidenceRecords(complete bool) ([]string, error) {
	common, err := w.commonDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(common, "itos", "evidence")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var records []string
	for _, e := range entries {
		if !e.IsDir() || strings.Contains(e.Name(), ".") {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, e.Name(), "complete")); complete && err != nil {
			continue
		}
		records = append(records, e.Name())
	}
	return records, nil
}

// checksOfTask are the checks the JSON gives the task.
func (w *world) checksOfTask(task string) ([]map[string]any, error) {
	var report struct {
		Tasks []struct {
			ID     string           `json:"id"`
			Checks []map[string]any `json:"checks"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal([]byte(w.stdout), &report); err != nil {
		return nil, fmt.Errorf("the output is not JSON: %v\n%s", err, w.report())
	}
	for _, t := range report.Tasks {
		if t.ID == task {
			return t.Checks, nil
		}
	}
	return nil, fmt.Errorf("the JSON has no task %s\n%s", task, w.report())
}

// evidenceOfTask is the evidence of the task's first check that has any.
func (w *world) evidenceOfTask(task string) (map[string]any, error) {
	checks, err := w.checksOfTask(task)
	if err != nil {
		return nil, err
	}
	for _, c := range checks {
		if evidence, ok := c["evidence"].(map[string]any); ok {
			return evidence, nil
		}
	}
	return nil, fmt.Errorf("the JSON gives the task %s's checks no evidence\n%s", task, w.report())
}

func (w *world) evidenceOfTaskIs(task, field string, want any) error {
	evidence, err := w.evidenceOfTask(task)
	if err != nil {
		return err
	}
	if evidence[field] != want {
		return fmt.Errorf("the task %s's evidence has %s %v, not %v\n%s", task, field, evidence[field], want, w.report())
	}
	return nil
}

// ledgerCheckChangedTo rewrites the command of the task's check in the
// working tree's ledger alone, leaving the index as it was.
func (w *world) ledgerCheckChangedTo(task, command string) error {
	path := filepath.Join(w.dir, w.ledgerPath())
	text, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !strings.Contains(string(text), task) {
		return fmt.Errorf("the ledger has no task %s", task)
	}
	changed := regexp.MustCompile(`(?m)^(\s+- run: ).*$`).ReplaceAllString(string(text), "${1}"+command)
	if changed == string(text) {
		return fmt.Errorf("the ledger's check was not changed")
	}
	return os.WriteFile(path, []byte(changed), 0o644)
}
