// The steps of slice 200 (push-mode-draft.feature): itos push --ready merges a
// clean pull request, pins its merges to the head it judged, counts a review
// state only for that head and returns within push.pr.ready_wait. They give
// the recording gh calls that fail on a text, a pull request that GitHub
// merges at once, Codex's review state of a commit (the clone's HEAD, a commit
// named, or the head a remote's branch has when gh is asked), the times GitHub
// keeps (a comment's, the ready event's, the head's push in the repository's
// activity), and the fake GitHub's run a start time; and they check a call
// for the full SHA of a head.
package features

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

func initializeReadyWaitSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the recording gh's "([^"]*)" calls with "([^"]*)" fail, saying "([^"]*)"$`, w.ghCallsFail)
	sc.Step(`^the recording gh's pr merge with --auto merges the pull request at once, as gh does when it reads it clean$`, func() error {
		return w.ghWrite(w.ghMergesAtOnce(), nil)
	})
	sc.Step(`^the recording gh's pull request for the branch "([^"]*)" is merged already$`, func(branch string) error {
		return w.ghWrite(filepath.Join(w.support, "gh-merged-"+branch), []byte(secondsAgo(1)+"\n"))
	})
	sc.Step(`^the recording gh's "([^"]*)" call has the full SHA of the head of the remote's branch "([^"]*)"$`, func(command, branch string) error {
		return w.ghCallWithHasHead(command, "", branch)
	})
	sc.Step(`^the recording gh's "([^"]*)" call with "([^"]*)" has the full SHA of the clone's HEAD$`, func(command, with string) error {
		sha, err := w.head()
		if err != nil {
			return err
		}
		return w.ghCallWithHas(command, with, sha)
	})
	sc.Step(`^the recording gh's "([^"]*)" call with "([^"]*)" has the full SHA of the head of the remote's branch "([^"]*)"$`, w.ghCallWithHasHead)
	sc.Step(`^its output says "([^"]*)" with the full SHA of the head of the remote's branch "([^"]*)"$`, func(text, branch string) error {
		sha, err := w.remoteBranchHead(branch)
		if err != nil {
			return err
		}
		return w.outputSays(text + sha)
	})
	sc.Step(`^the recording gh answers that the pull request has a comment by "([^"]*)" holding Codex's review state, its code review "([^"]*)" of (the clone's HEAD|the commit "[^"]*") and its security review "([^"]*)" of (the clone's HEAD|the commit "[^"]*")$`,
		w.ghAnswersCodexStateOf)
	sc.Step(`^the recording gh answers that the pull request has a comment by "([^"]*)" holding Codex's review state of the head of the remote's branch "([^"]*)" at each call, its code review "([^"]*)" and its security review "([^"]*)"$`,
		func(login, branch, code, security string) error {
			if err := w.ghKeepsHead(branch); err != nil {
				return err
			}
			return w.ghAddNode(w.ghComments(), codexComment(login, code, headOf(branch), security, headOf(branch)))
		})
	sc.Step(`^the recording gh answers that the pull request has a comment by "([^"]*)" holding Codex's review state of the head of the remote's branch "([^"]*)" at each call, its code review "([^"]*)" and its security review "([^"]*)" for its first (\d+) api graphql calls, then both "([^"]*)"$`,
		func(login, branch, code, security string, n int, then string) error {
			if err := w.ghKeepsHead(branch); err != nil {
				return err
			}
			if err := w.ghAddNode(w.ghCommentsEarly(), codexComment(login, code, headOf(branch), security, headOf(branch))); err != nil {
				return err
			}
			if err := w.ghAddNode(w.ghComments(), codexComment(login, then, headOf(branch), then, headOf(branch))); err != nil {
				return err
			}
			return os.WriteFile(w.ghCommentsLeft(), fmt.Appendf(nil, "%d\n", n), 0o644)
		})
	sc.Step(`^the recording gh answers that the pull request has a comment by "([^"]*)" holding Codex's review state, its code review "([^"]*)" and its security review "([^"]*)", last edited (\d+) seconds ago$`,
		func(login, code, security string, seconds int) error {
			sha, err := w.head()
			if err != nil {
				return err
			}
			then := secondsAgo(seconds)
			return w.ghAddNode(w.ghComments(), codexComment(login, code, sha, security, sha, then, then))
		})
	sc.Step(`^the recording gh answers that the pull request has a comment by "([^"]*)" asking "([^"]*)" to review the clone's HEAD again, posted (\d+) seconds ago$`,
		func(author, login string, seconds int) error {
			sha, err := w.head()
			if err != nil {
				return err
			}
			body := "@codex review\n\n<!-- itos-review-request:v1 " + login + " " + sha + " -->"
			then := secondsAgo(seconds)
			return w.ghAddNode(w.ghComments(), map[string]any{"author": map[string]any{"login": author}, "body": body, "createdAt": then, "updatedAt": then})
		})
	sc.Step(`^the recording gh answers that the pull request was marked ready (\d+) seconds ago and its head pushed (\d+) seconds ago$`,
		func(ready, pushed int) error {
			if err := w.ghWrite(w.ghReadyAt(), []byte(secondsAgo(ready)+"\n")); err != nil {
				return err
			}
			return w.ghWrite(w.ghActivity(), []byte(secondsAgo(pushed)+"\n"))
		})
	sc.Step(`^ci\.watch\.interval is (\d+)$`, func(seconds int) error {
		w.watchConfigured().interval = seconds
		return w.pushConfig("chore: look at CI less often")
	})
	sc.Step(`^the watched run was started (\d+) seconds ago$`, func(seconds int) error {
		if w.github == nil {
			return errors.New("no fake GitHub: start one first")
		}
		w.github.mu.Lock()
		w.github.startedAt = secondsAgo(seconds)
		w.github.mu.Unlock()
		return nil
	})
}

// The recording gh's files of slice 200: the calls that fail on a text, one
// line each (the command, the text and the words, tab separated); whether
// pr merge --auto merges at once; the time the pull request was marked ready
// and the time the head was pushed, which the activity API gives; and the
// branches whose head the comments name when gh is called.
func (w *world) ghFailCalls() string    { return filepath.Join(w.support, "gh-fail-calls") }
func (w *world) ghMergesAtOnce() string { return filepath.Join(w.support, "gh-merges-at-once") }
func (w *world) ghReadyAt() string      { return filepath.Join(w.support, "gh-ready-at") }
func (w *world) ghActivity() string     { return filepath.Join(w.support, "gh-activity") }
func (w *world) ghCommentHeads() string { return filepath.Join(w.support, "gh-comment-heads") }

// Writes the file of the recording gh's, which must be on the PATH already.
func (w *world) ghWrite(file string, text []byte) error {
	if _, err := os.Stat(programPath(filepath.Join(w.support, "bin", "gh"))); err != nil {
		return errors.New("no recording gh: put one on the PATH first")
	}
	return os.WriteFile(file, text, 0o644)
}

// The recording gh fails the calls of the command whose recorded line has the
// text, writing the words to stderr, as every other call does as before.
func (w *world) ghCallsFail(command, text, words string) error {
	if _, err := os.Stat(programPath(filepath.Join(w.support, "bin", "gh"))); err != nil {
		return errors.New("no recording gh: put one on the PATH first")
	}
	f, err := os.OpenFile(w.ghFailCalls(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(f, "%s\t%s\t%s\n", command, text, words)
	return errors.Join(err, f.Close())
}

// The text a comment holds in place of the full SHA of the head the remote's
// branch has when gh is called.
func headOf(branch string) string { return "@HEAD:" + branch + "@" }

// The recording gh names, in the comments it answers, the head the remote's
// branch has at each call.
func (w *world) ghKeepsHead(branch string) error {
	if _, err := os.Stat(programPath(filepath.Join(w.support, "bin", "gh"))); err != nil {
		return errors.New("no recording gh: put one on the PATH first")
	}
	f, err := os.OpenFile(w.ghCommentHeads(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(f, branch)
	return errors.Join(err, f.Close())
}

var commitNamed = regexp.MustCompile(`^the commit "([^"]*)"$`)

// Codex's review state with each review a status of a commit, each commit the
// clone's HEAD or one named.
func (w *world) ghAnswersCodexStateOf(login, code, codeCommit, security, securityCommit string) error {
	commit := func(text string) (string, error) {
		if m := commitNamed.FindStringSubmatch(text); m != nil {
			return m[1], nil
		}
		return w.head()
	}
	codeSHA, err := commit(codeCommit)
	if err != nil {
		return err
	}
	securitySHA, err := commit(securityCommit)
	if err != nil {
		return err
	}
	return w.ghAddNode(w.ghComments(), codexComment(login, code, codeSHA, security, securitySHA))
}

// The time that many seconds before now, as GitHub writes one.
func secondsAgo(seconds int) string {
	return time.Now().UTC().Add(-time.Duration(seconds) * time.Second).Format(time.RFC3339)
}

// Some call of the recording gh starts with the words of the command, has the
// text with and, when it is not empty, the text after it.
func (w *world) ghCallWithHas(command, with, text string) error {
	calls, err := w.ghCallLines()
	if err != nil {
		return err
	}
	var named []string
	for _, call := range calls {
		if (strings.HasPrefix(call, command+" ") || call == command) && strings.Contains(call, with) {
			named = append(named, call)
		}
	}
	if len(named) == 0 {
		return fmt.Errorf("the recording gh was never called as %q with %q; its calls:\n%s\n%s", command, with, strings.Join(calls, "\n"), w.report())
	}
	for _, call := range named {
		if strings.Contains(call, text) {
			return nil
		}
	}
	return fmt.Errorf("no %q call of the recording gh with %q has %q:\n%s\n%s", command, with, text, strings.Join(named, "\n"), w.report())
}

// The same, the text the full SHA of the head the remote's branch has now.
func (w *world) ghCallWithHasHead(command, with, branch string) error {
	sha, err := w.remoteBranchHead(branch)
	if err != nil {
		return err
	}
	return w.ghCallWithHas(command, with, sha)
}
