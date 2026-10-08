// The steps of work done's code proof (work.feature, slice 100): the
// config's proof.code, its paths and a check run through a fake provider, a
// script in the scenario's support folder that records the words it was run
// with and answers as itos-cc's mutation check answers by its machine
// contract, exit 0 with {"schema":1,"ok":true}, or exit 1 with
// {"schema":1,"ok":false,"problems":[…]}, one mutation.survived problem whose
// subject is the function; or a check whose command does not exist. The
// item's commit is the clone's, linked to the item by an Item: footer (the
// config's registry footer, turned on for it) and pushed to the remote past
// the hooks, so it has landed. What the provider was run with is read back
// from its record: its --since the parent of the item's first commit, or no
// run at all.
package features

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"
)

// What a scenario sets of the scratch config's proof.code.
type proofConfig struct {
	paths []string
	check string
}

func initializeProofSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the config's proof\.code covers "([^"]*)" and checks with a provider that passes$`, func(paths string) error {
		return w.proofProvider(paths, 0, map[string]any{"schema": 1, "ok": true})
	})
	sc.Step(`^the config's proof\.code covers "([^"]*)" and checks with a provider that finds a survivor in "([^"]*)"$`, func(paths, function string) error {
		return w.proofProvider(paths, 1, map[string]any{"schema": 1, "ok": false, "problems": []map[string]any{{
			"rule":        "mutation.survived",
			"message":     "src/app.go:3:9: a mutant of " + function + " survived: > became >=",
			"fix":         "add a test that tells > from >= in " + function + ", or except the mutant with its reason",
			"file":        "src/app.go",
			"line":        3,
			"column":      9,
			"function":    function,
			"original":    ">",
			"replacement": ">=",
		}}})
	})
	sc.Step(`^the config's proof\.code covers "([^"]*)" and checks with a command that does not exist$`, func(paths string) error {
		w.config.proof = &proofConfig{paths: []string{paths}, check: "itos-no-such-provider mutation check --since {base} --fail-uncovered --json"}
		return w.pushConfig("chore: prove the code")
	})
	sc.Step(`^the remote has the item "([^"]*)"'s commit touching "([^"]*)"$`, w.itemCommitPushed)
	sc.Step(`^the provider was run with the base before the item "([^"]*)"'s first commit$`, w.providerRanWithBase)
	sc.Step(`^the provider was not run$`, w.providerNotRun)
}

// The file the fake provider records each run's words in, a line a run.
func (w *world) providerLog() string { return filepath.Join(w.support, "provider-log") }

// proof.code covers the paths, its check the fake provider, run as itos-cc's
// mutation check is run (mutation check --since {base} --fail-uncovered
// --json), which records its words and prints the object on stdout and
// exits with the code; the config committed and pushed.
func (w *world) proofProvider(paths string, code int, answer map[string]any) error {
	text, err := json.Marshal(answer)
	if err != nil {
		return err
	}
	script := filepath.Join(w.support, "provider.sh")
	body := fmt.Sprintf("printf '%%s\\n' \"$*\" >> %s\nprintf '%%s\\n' %s\nexit %d\n",
		quote(filepath.ToSlash(w.providerLog())), quote(string(text)), code)
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		return err
	}
	w.config.proof = &proofConfig{
		paths: []string{paths},
		check: "sh " + quote(filepath.ToSlash(script)) + " mutation check --since {base} --fail-uncovered --json",
	}
	return w.pushConfig("chore: prove the code")
}

// The scratch config's proof section, when the scenario sets one.
func (w *world) proofSection() string {
	p := w.config.proof
	if p == nil {
		return ""
	}
	quoted := make([]string, len(p.paths))
	for i, path := range p.paths {
		quoted[i] = fmt.Sprintf("%q", path)
	}
	return fmt.Sprintf("proof:\n  code:\n    paths: [%s]\n    check: %q\n", strings.Join(quoted, ", "), p.check)
}

// The clone commits a line of the file with an Item: footer naming the item,
// the config's registry footer turned on (and pushed) for it, then pushes the
// commit to the remote's main past the hooks: the item's one commit, landed.
func (w *world) itemCommitPushed(id, path string) error {
	if !w.config.itemFooter {
		w.config.itemFooter = true
		if err := w.pushConfig("chore: link commits to items"); err != nil {
			return err
		}
	}
	subject := "chore: build " + id
	if err := writeLine(w.dir, path, subject); err != nil {
		return err
	}
	if err := w.git("add", "--", path); err != nil {
		return err
	}
	if err := w.git("commit", "-q", "--no-verify", "-m", subject+"\n\nItem: "+id+"\n"); err != nil {
		return err
	}
	return w.git("push", "-q", "--no-verify", "origin", "HEAD:refs/heads/main")
}

// The runs the fake provider recorded, a line of words each.
func (w *world) providerRuns() ([]string, error) {
	text, err := os.ReadFile(w.providerLog())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimRight(string(text), "\n"), "\n"), nil
}

// The fake provider ran once, its --since the full SHA of the parent of the
// item's first commit: the oldest commit of the clone's HEAD whose Item:
// footer names it.
func (w *world) providerRanWithBase(id string) error {
	first, err := w.gitOutput("log", "--reverse", "--format=%H", "--grep", "^Item: "+id+"$", "HEAD")
	if err != nil {
		return err
	}
	shas := strings.Fields(first)
	if len(shas) == 0 {
		return fmt.Errorf("HEAD has no commit whose Item: footer names %s", id)
	}
	base, err := w.gitOutput("rev-parse", shas[0]+"^")
	if err != nil {
		return err
	}
	base = strings.TrimSpace(base)
	runs, err := w.providerRuns()
	if err != nil {
		return err
	}
	if len(runs) != 1 {
		return fmt.Errorf("the provider ran %d times, not once: %q\n%s", len(runs), runs, w.report())
	}
	want := "mutation check --since " + base + " --fail-uncovered --json"
	if runs[0] != want {
		return fmt.Errorf("the provider was run with %q, not %q\n%s", runs[0], want, w.report())
	}
	return nil
}

func (w *world) providerNotRun() error {
	runs, err := w.providerRuns()
	if err != nil {
		return err
	}
	if len(runs) > 0 {
		return fmt.Errorf("the provider ran: %q\n%s", runs, w.report())
	}
	return nil
}
