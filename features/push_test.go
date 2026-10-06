// The steps of itos push (push.feature) and of the pre-push hook
// (pre-push.feature): a remote, a bare repository made from the scratch one,
// a clone of it where itos runs, commits the remote gains from another clone,
// the pre-push hook and hooks.pre_push's commands, a push of a new branch,
// and what the remote's branches hold afterwards; two takes of one item of
// the work registry, one the remote's and one the clone's, and the owner the
// remote's registry gives it (slice 66); a tag on the remote's head, and a
// fetch of the remote into the clone (status.feature, slice 70); ci.range's
// command printing one of the remote's commits (slice 73), and the full SHA of
// a commit of the remote's main by its header (bug 24's fake GitHub too).
package features

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/cucumber/godog"
	"go.yaml.in/yaml/v3"
)

// The line an uncommitted change adds to a file of the clone.
const uncommittedLine = "An uncommitted change.\n"

func initializePushSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^a clone of it, where itos runs$`, w.cloneOfRemote)
	sc.Step(`^the clone's git config says "([^"]*)" is "([^"]*)"$`, func(key, value string) error {
		return w.git("config", "--local", key, value)
	})
	sc.Step(`^the remote has gained the commit "([^"]*)" touching "([^"]*)"$`, w.remoteGainsCommit)
	sc.Step(`^the remote's head is tagged "([^"]*)"$`, func(tag string) error {
		return w.gitIn(w.remote(), "tag", tag, "HEAD")
	})
	sc.Step(`^the clone has fetched the remote$`, func() error {
		return w.git("fetch", "-q", "origin")
	})
	sc.Step(`^the clone has the commit "([^"]*)" touching "([^"]*)"$`, w.cloneCommits)
	sc.Step(`^the clone's "([^"]*)" has an uncommitted change$`, w.uncommittedChange)
	sc.Step(`^the clone's "([^"]*)" still has its uncommitted change$`, w.stillUncommitted)
	sc.Step(`^the remote's branch ends with "([^"]*)" then "([^"]*)", with no merge commit$`, w.remoteEndsWith)
	sc.Step(`^the remote's branch ends with "([^"]*)"$`, w.remoteEndsWithOne)
	sc.Step(`^the remote's branch does not have "([^"]*)"$`, w.remoteLacks)
	sc.Step(`^the remote's branch has "([^"]*)"$`, w.remoteHas)
	sc.Step(`^the remote has gained the commit "([^"]*)" making "([^"]*)" the owner of "([^"]*)"$`, w.remoteGainsTake)
	sc.Step(`^the clone has the commit "([^"]*)" making "([^"]*)" the owner of "([^"]*)"$`, w.cloneTakes)
	sc.Step(`^the remote's registry gives "([^"]*)" the owner "([^"]*)"$`, w.remoteRegistryOwner)

	sc.Step(`^the pre-push hook is installed$`, func() error {
		return w.hookInstalled("pre-push", `"$@"`)
	})
	sc.Step(`^hooks\.pre_push's commands record that they ran$`, w.recordingPrePush)
	sc.Step(`^hooks\.pre_push's command commits "([^"]*)" touching "([^"]*)" in the clone$`, w.committingPrePush)
	sc.Step(`^its output names the remote branch's head as the commit pushed$`, w.outputNamesRemoteHead)
	sc.Step(`^git pushes HEAD to the remote's new branch "([^"]*)"$`, func(branch string) error {
		return w.run(w.dir, "git", "push", "origin", "HEAD:refs/heads/"+branch)
	})
	sc.Step(`^the remote has no branch "([^"]*)"$`, w.remoteHasNoBranch)
	sc.Step(`^none of hooks\.pre_push's commands ran$`, func() error {
		if _, err := os.Stat(w.prePushRecord()); err == nil {
			text, _ := os.ReadFile(w.prePushRecord())
			return fmt.Errorf("hooks.pre_push's commands ran:\n%s\n%s", text, w.report())
		}
		return nil
	})
}

// The file hooks.pre_push's commands write when they run.
func (w *world) prePushRecord() string { return filepath.Join(w.support, "pre-push-ran") }

// hooks.pre_push's per_base and whole each record that they ran, in the
// clone's config, committed and pushed to the remote's main with the hook
// left out, as a project's config would be there already: the clone holds
// no uncommitted change, which itos push would refuse, and no commit of it
// but the scenario's is pushed afterwards.
func (w *world) recordingPrePush() error {
	w.config.prePushRecord = true
	return w.pushPrePushConfig("chore: record the pre-push commands")
}

// hooks.pre_push's per_base and whole each make a commit in the clone of
// the file, written as the commit's own line, while the push runs (bug 21):
// a commit landing during the hook's minutes of unit tests. git's hooks are
// off for that commit, as the scenario's setup, and the config is committed
// and pushed as recordingPrePush's is. One of the two runs on a push.
func (w *world) committingPrePush(subject, path string) error {
	full := filepath.Join(w.dir, path)
	w.config.prePushCommit = fmt.Sprintf("printf '%%s\\n' %s > %s && git -C %s add -- %s && "+
		"git -C %s -c core.hooksPath=/dev/null commit -q --no-verify -m %s -- %s",
		quote("A line for "+subject+"."), quote(full), quote(w.dir), quote(path),
		quote(w.dir), quote(subject), quote(path))
	return w.pushPrePushConfig("chore: commit while the pre-push hook runs")
}

// The clone's config, as the scenario set it, committed with the subject and
// pushed to the remote's main past the hooks.
func (w *world) pushPrePushConfig(subject string) error {
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

// The output's "Pushed <sha>" names the commit the remote's main is at: the
// short SHA it prints is the start of that commit's full one.
func (w *world) outputNamesRemoteHead() error {
	cmd := exec.Command("git", "rev-parse", "main")
	cmd.Dir = w.remote()
	cmd.Env = w.env()
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("git rev-parse main in the remote: %w", err)
	}
	head := strings.TrimSpace(string(out))
	m := pushedLine.FindStringSubmatch(w.output())
	if m == nil {
		return fmt.Errorf("the output names no commit pushed; the remote's main is at %s\n%s", head, w.report())
	}
	if len(m[1]) < 7 || !strings.HasPrefix(head, m[1]) {
		return fmt.Errorf("the output says Pushed %s, but the remote's main is at %s\n%s", m[1], head, w.report())
	}
	return nil
}

// The line itos push prints for the commit it pushed.
var pushedLine = regexp.MustCompile(`Pushed ([0-9a-f]+) to `)

func (w *world) remoteHas(subject string) error {
	subjects, _, err := w.remoteHistory()
	if err != nil {
		return err
	}
	if !slices.Contains(subjects, subject) {
		return fmt.Errorf("the remote's main does not have %q:\n%s\n%s", subject, strings.Join(subjects, "\n"), w.report())
	}
	return nil
}

func (w *world) remoteHasNoBranch(branch string) error {
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	cmd.Dir = w.remote()
	cmd.Env = w.env()
	if err := cmd.Run(); err == nil {
		return fmt.Errorf("the remote has the branch %q\n%s", branch, w.report())
	}
	return nil
}

// The remote: a bare repository in the support folder, as a host keeps one.
func (w *world) remote() string { return filepath.Join(w.support, "origin.git") }

// The scratch repository's history in a bare remote, and a clone of that,
// its main tracking the remote's, with the files the scratch repository has
// not committed laid into it. itos and every later step run in the clone.
func (w *world) cloneOfRemote() error {
	if err := w.git("clone", "-q", "--bare", w.dir, w.remote()); err != nil {
		return err
	}
	clone, err := os.MkdirTemp("", "itos-features-clone-")
	if err != nil {
		return err
	}
	if err := w.gitIn(w.support, "clone", "-q", w.remote(), clone); err != nil {
		os.RemoveAll(clone)
		return err
	}
	if err := w.layUncommitted(clone); err != nil {
		os.RemoveAll(clone)
		return err
	}
	w.origin, w.dir = w.dir, clone
	return nil
}

// A commit made in another clone of the remote and pushed to its main, the
// file written as the commit's own line.
func (w *world) remoteGainsCommit(subject, path string) error {
	other := filepath.Join(w.support, "other")
	if _, err := os.Stat(other); err != nil {
		if err := w.gitIn(w.support, "clone", "-q", w.remote(), other); err != nil {
			return err
		}
	}
	if err := writeLine(other, path, subject); err != nil {
		return err
	}
	if err := w.gitIn(other, "add", "--", path); err != nil {
		return err
	}
	if err := w.gitIn(other, "commit", "-q", "--no-verify", "-m", subject); err != nil {
		return err
	}
	return w.gitIn(other, "push", "-q", "--no-verify", "origin", "HEAD:refs/heads/main")
}

// The full SHA of the commit with the header on the remote's main.
func (w *world) remoteCommit(header string) (string, error) {
	cmd := exec.Command("git", "log", "--format=%H %s", "main")
	cmd.Dir = w.remote()
	cmd.Env = w.env()
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git log in the remote: %w", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if sha, subject, _ := strings.Cut(line, " "); subject == header {
			return sha, nil
		}
	}
	return "", fmt.Errorf("the remote's main has no commit %q:\n%s", header, out)
}

// A commit in the clone of that one file, written as the commit's own line.
func (w *world) cloneCommits(subject, path string) error {
	if err := writeLine(w.dir, path, subject); err != nil {
		return err
	}
	if err := w.git("add", "--", path); err != nil {
		return err
	}
	return w.git("commit", "-q", "--no-verify", "-m", subject, "--", path)
}

// The file at path in the folder dir, holding one line naming the commit, so
// two commits touching the same file conflict.
func writeLine(dir, path, subject string) error {
	full := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, []byte("A line for "+subject+".\n"), 0o644)
}

// A take of the item pushed to the remote's main from another clone: the
// clone's registry, as the scenario wrote it, pushed first, so both start
// from it, then the item made the owner's and doing there, as itos work take
// edits it, and committed and pushed past the hooks.
func (w *world) remoteGainsTake(subject, owner, id string) error {
	if err := w.git("push", "-q", "--no-verify", "origin", "HEAD:refs/heads/main"); err != nil {
		return err
	}
	other := filepath.Join(w.support, "other")
	if _, err := os.Stat(other); err != nil {
		if err := w.gitIn(w.support, "clone", "-q", w.remote(), other); err != nil {
			return err
		}
	} else {
		if err := w.gitIn(other, "fetch", "-q", "origin"); err != nil {
			return err
		}
		if err := w.gitIn(other, "reset", "-q", "--hard", "origin/main"); err != nil {
			return err
		}
	}
	if err := w.takeIn(other, id, owner); err != nil {
		return err
	}
	if err := w.gitIn(other, "commit", "-q", "--no-verify", "-am", subject); err != nil {
		return err
	}
	return w.gitIn(other, "push", "-q", "--no-verify", "origin", "HEAD:refs/heads/main")
}

// A take of the item committed in the clone, past the hooks.
func (w *world) cloneTakes(subject, owner, id string) error {
	if err := w.takeIn(w.dir, id, owner); err != nil {
		return err
	}
	path := w.data(startingRegistry)
	if err := w.git("add", "--", path); err != nil {
		return err
	}
	return w.git("commit", "-q", "--no-verify", "-m", subject, "--", path)
}

// The registry of the repository in dir with the item's line, as the work
// steps write it, giving it the owner and the status doing.
func (w *world) takeIn(dir, id, owner string) error {
	file := filepath.Join(dir, w.data(startingRegistry))
	text, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	lines := strings.Split(string(text), "\n")
	found := false
	for i, line := range lines {
		if strings.Contains(line, "{ id: "+id+",") {
			line = strings.Replace(line, "owner: null", "owner: "+owner, 1)
			lines[i] = strings.Replace(line, "status: todo", "status: doing", 1)
			found = true
		}
	}
	if !found {
		return fmt.Errorf("the registry in %s has no item %s:\n%s", dir, id, text)
	}
	return os.WriteFile(file, []byte(strings.Join(lines, "\n")), 0o644)
}

// The registry on the remote's main gives the item the owner.
func (w *world) remoteRegistryOwner(id, owner string) error {
	cmd := exec.Command("git", "show", "main:"+filepath.ToSlash(w.data(startingRegistry)))
	cmd.Dir = w.remote()
	cmd.Env = w.env()
	text, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("the remote's main has no registry: %w\n%s", err, w.report())
	}
	var registry struct {
		Items []map[string]any `yaml:"items"`
	}
	if err := yaml.Unmarshal(text, &registry); err != nil {
		return fmt.Errorf("the remote's registry does not read: %w\n%s", err, text)
	}
	for _, item := range registry.Items {
		if fmt.Sprint(item["id"]) != id {
			continue
		}
		if fmt.Sprint(item["owner"]) != owner {
			return fmt.Errorf("the remote's registry gives %s the owner %v, not %s\n%s\n%s", id, item["owner"], owner, text, w.report())
		}
		return nil
	}
	return fmt.Errorf("the remote's registry has no item %s\n%s\n%s", id, text, w.report())
}

func (w *world) uncommittedChange(path string) error {
	full := filepath.Join(w.dir, path)
	text, err := os.ReadFile(full)
	if err != nil {
		return err
	}
	return os.WriteFile(full, append(text, uncommittedLine...), 0o644)
}

// The file still ends with the uncommitted line, and git still sees it
// changed against HEAD: nothing stashed it, nothing committed it.
func (w *world) stillUncommitted(path string) error {
	text, err := os.ReadFile(filepath.Join(w.dir, path))
	if err != nil {
		return err
	}
	if !strings.HasSuffix(string(text), uncommittedLine) {
		return fmt.Errorf("%s lost its uncommitted change:\n%s", path, text)
	}
	if err := w.git("diff", "--quiet", "HEAD", "--", path); err == nil {
		return fmt.Errorf("git sees no change to %s against HEAD", path)
	}
	return nil
}

// The subjects of the remote's main, newest first, and its merge commits.
func (w *world) remoteHistory() (subjects, merges []string, err error) {
	read := func(args ...string) ([]string, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = w.remote()
		cmd.Env = w.env()
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("git %s in the remote: %w", strings.Join(args, " "), err)
		}
		if text := strings.TrimSpace(string(out)); text != "" {
			return strings.Split(text, "\n"), nil
		}
		return nil, nil
	}
	if subjects, err = read("log", "--format=%s", "main"); err != nil {
		return nil, nil, err
	}
	merges, err = read("rev-list", "--merges", "main")
	return subjects, merges, err
}

func (w *world) remoteEndsWith(older, newer string) error {
	subjects, merges, err := w.remoteHistory()
	if err != nil {
		return err
	}
	if len(merges) > 0 {
		return fmt.Errorf("the remote's main has merge commits %v:\n%s", merges, strings.Join(subjects, "\n"))
	}
	if len(subjects) < 2 || subjects[0] != newer || subjects[1] != older {
		return fmt.Errorf("the remote's main does not end with %q then %q:\n%s", older, newer, strings.Join(subjects, "\n"))
	}
	return nil
}

// The remote's main's newest commit has the header subject.
func (w *world) remoteEndsWithOne(subject string) error {
	subjects, _, err := w.remoteHistory()
	if err != nil {
		return err
	}
	if len(subjects) == 0 || subjects[0] != subject {
		return fmt.Errorf("the remote's main does not end with %q:\n%s\n%s", subject, strings.Join(subjects, "\n"), w.report())
	}
	return nil
}

func (w *world) remoteLacks(subject string) error {
	subjects, _, err := w.remoteHistory()
	if err != nil {
		return err
	}
	if slices.Contains(subjects, subject) {
		return fmt.Errorf("the remote's main has %q:\n%s", subject, strings.Join(subjects, "\n"))
	}
	return nil
}
