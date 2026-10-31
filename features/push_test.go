// The steps of itos push (push.feature) and of the pre-push hook
// (pre-push.feature): a remote, a bare repository made from the scratch one,
// a clone of it where itos runs, commits the remote gains from another clone,
// the pre-push hook and hooks.pre_push's commands, a push of a new branch,
// and what the remote's branches hold afterwards; two takes of one item of
// the work registry, one the remote's and one the clone's, and the owner the
// remote's registry gives it (slice 66), or a new idea added on each side
// (bug 118); a tag on the remote's head, and a
// fetch of the remote into the clone (status.feature, slice 70); ci.range's
// command printing one of the remote's commits (slice 73), the full SHA of
// a commit of the remote's main by its header (bug 24's fake GitHub too), and
// an origin that cannot be reached or is no repository (slice 90); a take
// of an item in a commit no remote has that touches only the registry
// (slice 93); the real git run in the clone, past any shim, its hooks
// running (slice 128); and annotated tags and another branch of the clone,
// and the tags and branches the remote has afterwards (slice 131); and bug
// 136's: a linked worktree of the clone on a new branch, where itos runs, a
// branch of the clone with no upstream, and the remote's HEAD naming a new
// branch.
package features

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"

	"github.com/cucumber/godog"

	"github.com/donvargax/itos/v8/internal/git"
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
	sc.Step(`^the clone's remote origin is "([^"]*)"$`, func(url string) error {
		return w.git("remote", "set-url", "origin", url)
	})
	sc.Step(`^the clone's remote origin is a folder that is no repository$`, w.originNoRepository)
	sc.Step(`^the clone's itos hooks are not installed$`, func() error { return w.undeclareHooks(w.dir) })
	sc.Step(`^the clone has fetched the remote$`, func() error {
		return w.git("fetch", "-q", "origin")
	})
	sc.Step(`^the clone has the commit "([^"]*)" touching "([^"]*)"$`, w.cloneCommits)
	sc.Step(`^the clone has the commit "([^"]*)" naming the task "([^"]*)" and touching "([^"]*)"$`, w.cloneCommitsNaming)
	sc.Step(`^the remote has gained the commit "([^"]*)" naming the task "([^"]*)" and touching "([^"]*)"$`, w.remoteGainsCommitNaming)
	sc.Step(`^the clone has the annotated tag "([^"]*)" on its HEAD$`, func(tag string) error {
		return w.git("tag", "-a", "-m", "Release "+tag+".", tag, "HEAD")
	})
	sc.Step(`^the clone has the branch "([^"]*)" with the commit "([^"]*)" touching "([^"]*)"$`, w.cloneBranchCommits)
	sc.Step(`^the remote has the tag "([^"]*)"$`, w.remoteHasTag)
	sc.Step(`^the remote does not have the tag "([^"]*)"$`, w.remoteLacksTag)
	sc.Step(`^the remote has the branch "([^"]*)" with "([^"]*)"$`, w.remoteBranchHas)
	sc.Step(`^the clone's "([^"]*)" has an uncommitted change$`, w.uncommittedChange)
	sc.Step(`^the clone's "([^"]*)" still has its uncommitted change$`, w.stillUncommitted)
	sc.Step(`^the remote's branch ends with "([^"]*)" then "([^"]*)", with no merge commit$`, w.remoteEndsWith)
	sc.Step(`^the remote's branch ends with "([^"]*)"$`, w.remoteEndsWithOne)
	sc.Step(`^the remote's branch does not have "([^"]*)"$`, w.remoteLacks)
	sc.Step(`^the remote's branch has "([^"]*)"$`, w.remoteHas)
	sc.Step(`^the remote has gained the commit "([^"]*)" making "([^"]*)" the owner of "([^"]*)"$`, w.remoteGainsTake)
	sc.Step(`^the clone has the commit "([^"]*)" making "([^"]*)" the owner of "([^"]*)"$`, w.cloneTakes)
	sc.Step(`^the remote has gained the commit "([^"]*)" adding the idea "([^"]*)" to the registry$`, w.remoteGainsIdea)
	sc.Step(`^the remote has gained the commit "([^"]*)" adding the task "([^"]*)" to the ledger$`, w.remoteGainsTask)
	sc.Step(`^the remote has gained the commit "([^"]*)" adding the late check "([^"]*)" to the ledger's task "([^"]*)"$`, w.remoteGainsLateCheck)
	sc.Step(`^the remote has gained the commit "([^"]*)" giving the ledger's task "([^"]*)" the why "([^"]*)"$`, w.remoteGainsTaskWhy)
	sc.Step(`^the clone has the commit "([^"]*)" adding the idea "([^"]*)" to the registry$`, w.cloneAddsIdea)
	sc.Step(`^the remote's registry gives "([^"]*)" the owner "([^"]*)"$`, w.remoteRegistryOwner)
	sc.Step(`^the clone has a commit no remote has that takes "([^"]*)" for "([^"]*)"$`, w.cloneTakesUnpushed)

	sc.Step(`^the pre-push hook is installed$`, func() error {
		return w.hookInstalled("pre-push", `"$@"`)
	})
	sc.Step(`^hooks\.pre_push's commands record that they ran$`, w.recordingPrePush)
	sc.Step(`^hooks\.pre_push's command commits "([^"]*)" touching "([^"]*)" in the clone$`, w.committingPrePush)
	sc.Step(`^its output names the remote branch's head as the commit pushed$`, w.outputNamesRemoteHead)
	sc.Step(`^git pushes HEAD to the remote's new branch "([^"]*)"$`, func(branch string) error {
		return w.run(w.dir, git.Bin(runtime.GOOS), "push", "origin", "HEAD:refs/heads/"+branch)
	})
	// The real git, past any shim, in the clone, its hooks running.
	sc.Step(`^the real git runs "([^"]*)"$`, func(line string) error {
		args, err := shellWords(line)
		if err != nil {
			return err
		}
		return w.run(w.dir, git.Bin(runtime.GOOS), args...)
	})
	sc.Step(`^the real git exits with code (\d+)$`, w.gitExitsWith)
	sc.Step(`^the remote has no branch "([^"]*)"$`, w.remoteHasNoBranch)
	sc.Step(`^the clone has a linked worktree on the new branch "([^"]*)", where itos runs$`, w.cloneLinkedWorktree)
	sc.Step(`^the clone's branch "([^"]*)" has no upstream$`, w.cloneBranchHasNoUpstream)
	sc.Step(`^the remote's HEAD names its new branch "([^"]*)", made at its main's head$`, w.remoteHeadNamesNewBranch)
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
	cmd := exec.Command(git.Bin(runtime.GOOS), "rev-parse", "main")
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
	cmd := exec.Command(git.Bin(runtime.GOOS), "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	cmd.Dir = w.remote()
	cmd.Env = w.env()
	if err := cmd.Run(); err == nil {
		return fmt.Errorf("the remote has the branch %q\n%s", branch, w.report())
	}
	return nil
}

// The clone moves into the support folder, as linkedWorktree moves the scratch
// repository, and gets a linked worktree there on a new branch made at its
// HEAD with git worktree add -b, as Claude Code's isolation makes one: the
// branch has no upstream. itos and every later step run in the worktree
// (bug 136).
func (w *world) cloneLinkedWorktree(branch string) error {
	moved := filepath.Join(w.support, "clone")
	if err := os.Rename(w.dir, moved); err != nil {
		return err
	}
	w.dir = moved
	worktree := filepath.Join(w.support, "worktree")
	if err := w.git("worktree", "add", "-q", "-b", branch, worktree); err != nil {
		return err
	}
	w.dir = worktree
	return nil
}

// The clone's branch has neither branch.<name>.remote nor branch.<name>.merge.
func (w *world) cloneBranchHasNoUpstream(branch string) error {
	for _, key := range []string{"remote", "merge"} {
		cmd := exec.Command(git.Bin(runtime.GOOS), "config", "--get", "branch."+branch+"."+key)
		cmd.Dir = w.dir
		cmd.Env = w.env()
		if out, err := cmd.Output(); err == nil {
			return fmt.Errorf("the clone's branch %q has branch.%s.%s %q\n%s", branch, branch, key, strings.TrimSpace(string(out)), w.report())
		}
	}
	return nil
}

// The remote gets a branch at its main's head, and its HEAD names that
// branch rather than main.
func (w *world) remoteHeadNamesNewBranch(branch string) error {
	if err := w.gitIn(w.remote(), "branch", branch, "main"); err != nil {
		return err
	}
	return w.gitIn(w.remote(), "symbolic-ref", "HEAD", "refs/heads/"+branch)
}

// The remote: a bare repository in the support folder, as a host keeps one.
func (w *world) remote() string { return filepath.Join(w.support, "origin.git") }

// The clone's origin is an empty folder beside the remote (slice 90): a path
// that exists, so git finds it, and holds no repository, so git says it does
// not appear to be one. The branch keeps its upstream, origin's main.
func (w *world) originNoRepository() error {
	folder := filepath.Join(w.support, "not-a-repository")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return err
	}
	return w.git("remote", "set-url", "origin", folder)
}

// The scratch repository's history in a bare remote, and a clone of that,
// its main tracking the remote's, itos's hooks declared in its git config as
// in every scenario's repository (declareHooks), with the files the scratch
// repository has not committed laid into it. itos and every later step run
// in the clone.
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
	if err := w.declareHooks(clone); err != nil {
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
	return w.remoteGains(path, subject)
}

// The same commit, with the footer Task: of the id, past the hooks.
func (w *world) remoteGainsCommitNaming(subject, id, path string) error {
	return w.remoteGains(path, subject, "Task: "+id)
}

// The commit's message is the subject, then each paragraph after it.
func (w *world) remoteGains(path, subject string, paragraphs ...string) error {
	return w.remoteGainsWriting(path, subject, lineFor(subject), paragraphs...)
}

// The same, the file holding the text.
func (w *world) remoteGainsWriting(path, subject, text string, paragraphs ...string) error {
	other := filepath.Join(w.support, "other")
	if _, err := os.Stat(other); err != nil {
		if err := w.gitIn(w.support, "clone", "-q", w.remote(), other); err != nil {
			return err
		}
	}
	if err := writeText(other, path, text); err != nil {
		return err
	}
	if err := w.gitIn(other, "add", "--", path); err != nil {
		return err
	}
	if err := w.gitIn(other, append([]string{"commit", "-q", "--no-verify", "-m", subject}, messageArgs(paragraphs)...)...); err != nil {
		return err
	}
	return w.gitIn(other, "push", "-q", "--no-verify", "origin", "HEAD:refs/heads/main")
}

// Each paragraph as a -m of its own.
func messageArgs(paragraphs []string) []string {
	var args []string
	for _, p := range paragraphs {
		args = append(args, "-m", p)
	}
	return args
}

// The full SHA of the commit with the header on the remote's main.
func (w *world) remoteCommit(header string) (string, error) {
	cmd := exec.Command(git.Bin(runtime.GOOS), "log", "--format=%H %s", "main")
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
	return w.cloneCommitsWith(subject, path)
}

// The same commit, with the footer Task: of the id, past the hooks.
func (w *world) cloneCommitsNaming(subject, id, path string) error {
	return w.cloneCommitsWith(subject, path, "Task: "+id)
}

// The commit's message is the subject, then each paragraph after it.
func (w *world) cloneCommitsWith(subject, path string, paragraphs ...string) error {
	return w.cloneCommitsWriting(subject, path, lineFor(subject), paragraphs...)
}

// The same, the file holding the text.
func (w *world) cloneCommitsWriting(subject, path, text string, paragraphs ...string) error {
	if err := writeText(w.dir, path, text); err != nil {
		return err
	}
	if err := w.git("add", "--", path); err != nil {
		return err
	}
	args := append([]string{"commit", "-q", "--no-verify", "-m", subject}, messageArgs(paragraphs)...)
	return w.git(append(args, "--", path)...)
}

// A branch of the clone made at its HEAD with one commit of that one file,
// as cloneCommits makes it, the clone left on the branch it was on (slice
// 131): a local branch other than the one checked out.
func (w *world) cloneBranchCommits(branch, subject, path string) error {
	if err := w.git("checkout", "-q", "-b", branch); err != nil {
		return err
	}
	if err := w.cloneCommits(subject, path); err != nil {
		return err
	}
	return w.git("checkout", "-q", "-")
}

// What the ref names in the repository in dir, "" when it names nothing.
func (w *world) refIn(dir, ref string) string {
	cmd := exec.Command(git.Bin(runtime.GOOS), "rev-parse", "--verify", "--quiet", ref)
	cmd.Dir = dir
	cmd.Env = w.env()
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
}

// The remote has the tag, the clone's own tag object (slice 131): the one
// pushed, not another of the name.
func (w *world) remoteHasTag(tag string) error {
	ref := "refs/tags/" + tag
	theirs, ours := w.refIn(w.remote(), ref), w.refIn(w.dir, ref)
	if theirs == "" {
		return fmt.Errorf("the remote has no tag %q\n%s", tag, w.report())
	}
	if theirs != ours {
		return fmt.Errorf("the remote's tag %q is %s, not the clone's %s\n%s", tag, theirs, ours, w.report())
	}
	return nil
}

func (w *world) remoteLacksTag(tag string) error {
	if sha := w.refIn(w.remote(), "refs/tags/"+tag); sha != "" {
		return fmt.Errorf("the remote has the tag %q, at %s\n%s", tag, sha, w.report())
	}
	return nil
}

// The remote's branch of the name has the commit with the subject.
func (w *world) remoteBranchHas(branch, subject string) error {
	cmd := exec.Command(git.Bin(runtime.GOOS), "log", "--format=%s", "refs/heads/"+branch)
	cmd.Dir = w.remote()
	cmd.Env = w.env()
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("the remote has no branch %q: %w\n%s", branch, err, w.report())
	}
	if !slices.Contains(strings.Split(strings.TrimSpace(string(out)), "\n"), subject) {
		return fmt.Errorf("the remote's branch %q does not have %q:\n%s\n%s", branch, subject, out, w.report())
	}
	return nil
}

// The file at path in the folder dir, holding one line naming the commit, so
// two commits touching the same file conflict.
func writeLine(dir, path, subject string) error {
	return writeText(dir, path, lineFor(subject))
}

// The one line naming the commit.
func lineFor(subject string) string { return "A line for " + subject + ".\n" }

// The file at path in the folder dir, holding the text.
func writeText(dir, path, text string) error {
	full := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, []byte(text), 0o644)
}

// A take of the item pushed to the remote's main from another clone, as
// remoteGainsRegistry commits it.
func (w *world) remoteGainsTake(subject, owner, id string) error {
	return w.remoteGainsRegistry(subject, func(dir string) error { return w.takeIn(dir, id, owner) })
}

// A new idea pushed to the remote's main from another clone, as
// remoteGainsRegistry commits it.
func (w *world) remoteGainsIdea(subject, id string) error {
	return w.remoteGainsRegistry(subject, func(dir string) error { return w.addIdeaIn(dir, id) })
}

// A new task, with no checks, pushed to the remote's main from another clone
// as the ledger's one file gets it (slice 181), committed as remoteGainsRegistry
// commits.
func (w *world) remoteGainsTask(subject, id string) error {
	return w.remoteGainsRegistry(subject, func(dir string) error {
		file := filepath.Join(dir, w.data(w.ledgerPath()))
		text, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		return os.WriteFile(file, append(text, fmt.Sprintf("- { id: %s, type: chore, title: Tidy }\n", id)...), 0o644)
	})
}

// The ledger's task of the id, with the check { run: command, cost: late }
// appended to its done_when (made when it has none), pushed to the remote's
// main from another clone as remoteGainsRegistry commits it (bug 137).
func (w *world) remoteGainsLateCheck(subject, command, id string) error {
	check := fmt.Sprintf("{ run: %q, cost: late }", command)
	return w.remoteGainsRegistry(subject, func(dir string) error {
		return w.editLedgerTask(dir, id, func(line string) string {
			if before, after, ok := strings.Cut(line, ", done_when: ["); ok {
				checks, rest, _ := strings.Cut(after, "]")
				return before + ", done_when: [" + checks + ", " + check + "]" + rest
			}
			return strings.TrimSuffix(line, " }") + ", done_when: [" + check + "] }"
		})
	})
}

// The ledger's task of the id, with the why given, pushed to the remote's
// main from another clone as remoteGainsRegistry commits it (bug 137).
func (w *world) remoteGainsTaskWhy(subject, id, why string) error {
	return w.remoteGainsRegistry(subject, func(dir string) error {
		return w.editLedgerTask(dir, id, func(line string) string {
			line, _, _ = strings.Cut(line, ", why: ")
			return strings.TrimSuffix(line, " }") + fmt.Sprintf(", why: %q }", why)
		})
	})
}

// The line of the task of the id in the ledger's one file in the clone at
// dir, as ledgerText writes it, replaced by edit's; an error when the file
// has no such task.
func (w *world) editLedgerTask(dir, id string, edit func(line string) string) error {
	file := filepath.Join(dir, w.data(w.ledgerPath()))
	text, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	lines := strings.Split(string(text), "\n")
	i := slices.IndexFunc(lines, func(l string) bool { return strings.HasPrefix(l, "- { id: "+id+",") })
	if i < 0 {
		return fmt.Errorf("the ledger has no task %s:\n%s", id, text)
	}
	lines[i] = edit(lines[i])
	return os.WriteFile(file, []byte(strings.Join(lines, "\n")), 0o644)
}

// The clone's registry, as the scenario wrote it, pushed to the remote's
// main first, so both start from it unless the remote's main has it already,
// so that one gain may follow another, then edited in another clone by edit,
// and committed and pushed past the hooks.
func (w *world) remoteGainsRegistry(subject string, edit func(dir string) error) error {
	head, err := w.head()
	if err != nil {
		return err
	}
	if w.gitIn(w.remote(), "merge-base", "--is-ancestor", head, "refs/heads/main") != nil {
		if err := w.git("push", "-q", "--no-verify", "origin", "HEAD:refs/heads/main"); err != nil {
			return err
		}
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
	if err := edit(other); err != nil {
		return err
	}
	if err := w.gitIn(other, "commit", "-q", "--no-verify", "-am", subject); err != nil {
		return err
	}
	return w.gitIn(other, "push", "-q", "--no-verify", "origin", "HEAD:refs/heads/main")
}

// A take of the item committed in the clone, past the hooks.
func (w *world) cloneTakes(subject, owner, id string) error {
	return w.cloneCommitsRegistry(subject, func() error { return w.takeIn(w.dir, id, owner) })
}

// A new idea committed in the clone, past the hooks.
func (w *world) cloneAddsIdea(subject, id string) error {
	return w.cloneCommitsRegistry(subject, func() error { return w.addIdeaIn(w.dir, id) })
}

// The clone's registry edited by edit and committed alone, past the hooks.
func (w *world) cloneCommitsRegistry(subject string, edit func() error) error {
	if err := edit(); err != nil {
		return err
	}
	path := w.data(startingRegistry)
	if err := w.git("add", "--", path); err != nil {
		return err
	}
	return w.git("commit", "-q", "--no-verify", "-m", subject, "--", path)
}

// A take of the item for the person, as itos work take commits it: the
// registry alone, in a commit the clone has and no remote has. A person the
// people file does not list is added first, in a commit of the people file
// pushed to the remote's main past the hooks, as a project's people would be
// there already, so the registry stays sound and the take's commit touches
// nothing but the registry (slice 93).
func (w *world) cloneTakesUnpushed(id, owner string) error {
	people := w.data("people.yaml")
	text, err := os.ReadFile(filepath.Join(w.dir, people))
	if err != nil {
		return err
	}
	if !slices.Contains(strings.Split(string(text), "\n"), "- "+owner) {
		if err := os.WriteFile(filepath.Join(w.dir, people), append(text, []byte("- "+owner+"\n")...), 0o644); err != nil {
			return err
		}
		if err := w.git("add", "--", people); err != nil {
			return err
		}
		if err := w.git("commit", "-q", "--no-verify", "-m", "docs: list "+owner, "--", people); err != nil {
			return err
		}
		if err := w.git("push", "-q", "--no-verify", "origin", "HEAD:refs/heads/main"); err != nil {
			return err
		}
	}
	return w.cloneTakes("docs: take "+id, owner, id)
}

// An item's owner and status as the registry's line writes them.
var (
	ownerOf  = regexp.MustCompile(`owner: [^,}]*`)
	statusOf = regexp.MustCompile(`status: [^,}]*`)
)

// The registry of the repository in dir with the item's line, as the work
// steps write it, giving it the owner and the status doing, whoever had it.
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
			line = ownerOf.ReplaceAllLiteralString(line, "owner: "+owner)
			lines[i] = statusOf.ReplaceAllLiteralString(line, "status: doing")
			found = true
		}
	}
	if !found {
		return fmt.Errorf("the registry in %s has no item %s:\n%s", dir, id, text)
	}
	return os.WriteFile(file, []byte(strings.Join(lines, "\n")), 0o644)
}

// The registry of the repository in dir with one more idea of phase 1, todo
// and owned by nobody, its line after the last item's, where itos work add
// puts one: two such ideas, one on each side, meet at the same place.
func (w *world) addIdeaIn(dir, id string) error {
	file := filepath.Join(dir, w.data(startingRegistry))
	text, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	line := fmt.Sprintf("  - { id: %s, title: %s, phase: 1, owner: null, status: todo, kind: idea, depends_on: [] }\n", id, id)
	at := strings.Index(string(text), "\nqueue:")
	if at < 0 {
		return os.WriteFile(file, append(text, line...), 0o644)
	}
	return os.WriteFile(file, []byte(string(text[:at+1])+line+string(text[at+1:])), 0o644)
}

// The registry on the remote's main gives the item the owner.
func (w *world) remoteRegistryOwner(id, owner string) error {
	cmd := exec.Command(git.Bin(runtime.GOOS), "show", "main:"+filepath.ToSlash(w.data(startingRegistry)))
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
	return w.remoteBranchHistory("main")
}

// The same for the remote's branch of the name.
func (w *world) remoteBranchHistory(branch string) (subjects, merges []string, err error) {
	read := func(args ...string) ([]string, error) {
		cmd := exec.Command(git.Bin(runtime.GOOS), args...)
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
	if subjects, err = read("log", "--format=%s", "refs/heads/"+branch); err != nil {
		return nil, nil, err
	}
	merges, err = read("rev-list", "--merges", "refs/heads/"+branch)
	return subjects, merges, err
}

func (w *world) remoteEndsWith(older, newer string) error {
	return w.remoteBranchEndsWith("main", older, newer)
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
