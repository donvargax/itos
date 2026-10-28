// The steps of CI running only the jobs a range can affect (ci.feature, slice
// 202): a config kept in a folder ITOS_CONFIG names, a commit that touches
// it, and a range whose first commit's tree is gone.
package features

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"
)

func initializeCIJobsSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^itos plans CI over every commit up to HEAD with --json$`, func() error {
		return w.itos("ci", "plan", "", "HEAD", "--json")
	})
	sc.Step(`^the config is at "([^"]*)", which ITOS_CONFIG names$`, w.configIsAt)
	sc.Step(`^the commit "([^"]*)" touching only the config on top of it$`, w.commitTouchingConfig)
	sc.Step(`^the tree of the first commit is missing from the repository's objects$`, func() error {
		if len(w.commits) == 0 {
			return errors.New("the repository has no commit yet")
		}
		out, err := w.gitOutput("rev-parse", w.commits[0]+"^{tree}")
		if err != nil {
			return err
		}
		tree := strings.TrimSpace(out)
		object := filepath.Join(w.dir, ".git", "objects", tree[:2], tree[2:])
		if err := os.Remove(object); err != nil {
			return fmt.Errorf("the tree %s of the first commit is not a loose object, so it cannot be deleted: %w", tree, err)
		}
		return nil
	})
}

// The scratch config written at path, none left at the root, and ITOS_CONFIG
// naming it by its absolute path in itos's environment. Its data paths stay
// relative to the folder itos runs in, as they were at the root.
func (w *world) configIsAt(path string) error {
	w.configAt = path
	if err := w.writeConfig(); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(w.dir, "itos.yaml")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	w.vars = append(w.vars, "ITOS_CONFIG="+filepath.Join(w.dir, path))
	return nil
}

// A YAML comment line appended to the config file itos reads, committed
// alone: the config still loads, which a plain line appended to it would not.
func (w *world) commitTouchingConfig(message string) error {
	path := w.configFile()
	text, err := os.ReadFile(filepath.Join(w.dir, path))
	if err != nil {
		return err
	}
	if err := w.write(path, string(text)+"# A line for "+message+".\n"); err != nil {
		return err
	}
	if err := w.git("add", "--", path); err != nil {
		return err
	}
	if err := w.git("commit", "-q", "--no-verify", "-m", message, "--", path); err != nil {
		return err
	}
	sha, err := w.head()
	if err != nil {
		return err
	}
	w.commits = append(w.commits, sha)
	return nil
}
