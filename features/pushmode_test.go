// The steps of push.mode pr (push-mode.feature, slices 174 and 175): the
// config committed and pushed with a block of YAML added to it; a recording gh
// standing in for GitHub, first on the PATH, which records each call, keeps
// the pull requests it opens, answers a pull request's head and rebases the
// remote's branch when asked to update it (or leaves it as it is, or answers
// the head from before the rebase for a few calls, as a GitHub that has not
// caught up does); the clone on a new branch, with or without tracking the
// remote's main; the GitHub repository the environment names
// (GITHUB_REPOSITORY), as the fake GitHub gives it, or none; the remote's HEAD
// naming a branch it does not have; what a branch of the remote ends with; and
// the head of a remote's branch the fake GitHub was asked for the run of; and
// the commits itos makes beside the registry (slice 177): a task's close that
// takes out its checks, and an idea added with a file. Slice 179's give the
// recording gh draft pull requests, reviews and review threads, and say a call
// of it lacks a text. Slice 184's give its pull requests a title, check the
// length of a call's --title in Unicode code points, and record a line break
// inside an argument as a backslash and an n, so a call stays one line.
// Slice 191's give a review thread an id and its first comment's author, answer
// the two mutations that reply to a thread and resolve it, and check the order
// of two calls.
// Slice 190's give it a comment that asks a login to review a head again, a
// Codex review state that changes once a pr comment call has been made, and
// check the calls it was made as. Slice 199's give the recording gh an answer
// to the aliased pullRequests query of itos status.
// Slice-192's say a remote's branch has the commit of the remote's main with a
// subject, give a ledger task written as a block a single check, and compare
// the clone's HEAD with a remote branch's head or with a commit when no rebase
// is in progress.
package features

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/cucumber/godog"

	"github.com/donvargax/itos/v8/internal/git"
)

func initializePushModeSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the config, committed and pushed to the remote's main, adds the YAML:$`, func(text *godog.DocString) error {
		w.config.extraYAML = append(w.config.extraYAML, strings.TrimRight(text.Content, "\n")+"\n")
		return w.pushConfig("chore: add to the config")
	})
	sc.Step(`^a recording gh on the PATH, which opens pull requests as "([^"]*)"$`, w.recordingGh)
	sc.Step(`^the recording gh has the pull request "([^"]*)" open for the branch "([^"]*)"$`, w.ghHasPullRequest)
	sc.Step(`^the recording gh has the draft pull request "([^"]*)" open for the branch "([^"]*)"$`, w.ghHasDraftPullRequest)
	sc.Step(`^the recording gh has the pull request "([^"]*)" open for the branch "([^"]*)", titled "([^"]*)"$`, func(address, branch, title string) error {
		return w.ghHasPullRequestIn(address, branch, "OPEN", false, title)
	})
	sc.Step(`^the recording gh has the draft pull request "([^"]*)" open for the branch "([^"]*)", titled "([^"]*)"$`, func(address, branch, title string) error {
		return w.ghHasPullRequestIn(address, branch, "OPEN", true, title)
	})
	sc.Step(`^the recording gh's "([^"]*)" call has a --title of (\d+) characters ending with "([^"]*)"$`, w.ghCallTitle)
	sc.Step(`^the recording gh answers that the pull request has a review by "([^"]*)"$`, w.ghAnswersReview)
	sc.Step(`^the recording gh answers that the pull request has a review by "([^"]*)" whose body is "([^"]*)"$`, w.ghAnswersReviewBody)
	sc.Step(`^the recording gh answers that the pull request has a comment by "([^"]*)" holding Codex's review state, its code review "([^"]*)" and its security review "([^"]*)"$`,
		w.ghAnswersCodexState)
	sc.Step(`^the recording gh answers that the pull request has a comment by "([^"]*)" holding Codex's review state, its code review "([^"]*)" and its security review "([^"]*)" for its first (\d+) api graphql calls, then both "([^"]*)"$`,
		w.ghAnswersCodexStateThen)
	sc.Step(`^the recording gh answers that the pull request has a comment by "([^"]*)" holding Codex's review state, its code review "([^"]*)" and its security review "([^"]*)" until a pr comment call, then both "([^"]*)"$`,
		w.ghAnswersCodexStateUntilComment)
	sc.Step(`^the recording gh answers that the pull request has a comment by "([^"]*)" asking "([^"]*)" to review the clone's HEAD again$`,
		w.ghAnswersReviewRequestForHead)
	sc.Step(`^the recording gh answers that the pull request has a comment by "([^"]*)" asking "([^"]*)" to review the commit "([^"]*)" again$`,
		w.ghAnswersReviewRequest)
	sc.Step(`^the recording gh's "([^"]*)" call has the full SHA of the clone's HEAD$`, w.ghCallHasHead)
	sc.Step(`^the recording gh was called as "([^"]*)" once$`, w.ghCalledOnce)
	sc.Step(`^the recording gh answers that the pull request has an unresolved review thread on "([^"]*)" line (\d+), its first comment the lines "([^"]*)" and "([^"]*)"$`,
		func(path string, line int, first, second string) error {
			return w.ghAnswersThread(false, path, line, first, second)
		})
	sc.Step(`^the recording gh answers that the pull request has a resolved review thread on "([^"]*)" line (\d+), its first comment the lines "([^"]*)" and "([^"]*)"$`,
		func(path string, line int, first, second string) error {
			return w.ghAnswersThread(true, path, line, first, second)
		})
	sc.Step(`^the recording gh answers that the pull request has an unresolved review thread "([^"]*)" on "([^"]*)" line (\d+) by "([^"]*)", its first comment the lines "([^"]*)" and "([^"]*)"$`,
		func(id, path string, line int, login, first, second string) error {
			return w.ghAnswersThreadBy(false, id, path, line, login, first, second)
		})
	sc.Step(`^the recording gh answers that the pull request has a resolved review thread "([^"]*)" on "([^"]*)" line (\d+) by "([^"]*)", its first comment the lines "([^"]*)" and "([^"]*)"$`,
		func(id, path string, line int, login, first, second string) error {
			return w.ghAnswersThreadBy(true, id, path, line, login, first, second)
		})
	sc.Step(`^the recording gh's "([^"]*)" call does not have "([^"]*)"$`, w.ghCallLacks)
	sc.Step(`^the recording gh's call with "([^"]*)" comes before its call with "([^"]*)"$`, w.ghCallOrder)
	sc.Step(`^the recording gh's pr update-branch leaves the branch as it is$`, func() error {
		return os.WriteFile(w.ghKeeps(), nil, 0o644)
	})
	sc.Step(`^the recording gh's pr view answers the pull request's previous head for its first (\d+) calls after pr update-branch$`, func(n int) error {
		return os.WriteFile(w.ghStale(), fmt.Appendf(nil, "%d\n", n), 0o644)
	})
	sc.Step(`^the fake GitHub was asked for the run of the head of the remote's branch "([^"]*)"$`, w.fakeGitHubGivenRemoteBranchHead)
	sc.Step(`^the recording gh has the pull request "([^"]*)" merged for the branch "([^"]*)"$`, func(address, branch string) error {
		return w.ghHasPullRequestIn(address, branch, "MERGED", false, "")
	})
	sc.Step(`^the recording gh has the pull request "([^"]*)" merged for the branch "([^"]*)", its merge commit the head of the remote's branch "([^"]*)"$`, func(address, branch, merge string) error {
		if err := w.ghHasPullRequestIn(address, branch, "MERGED", false, ""); err != nil {
			return err
		}
		head, err := w.remoteBranchHead(merge)
		if err != nil {
			return err
		}
		return os.WriteFile(w.ghMergeCommit(branch), []byte(head+"\n"), 0o644)
	})
	sc.Step(`^the recording gh has the pull request "([^"]*)" closed without merging for the branch "([^"]*)"$`, func(address, branch string) error {
		return w.ghHasPullRequestIn(address, branch, "CLOSED", false, "")
	})
	sc.Step(`^the recording gh fails every call, saying "([^"]*)"$`, func(words string) error {
		return os.WriteFile(w.ghFailure(), []byte(words+"\n"), 0o644)
	})
	sc.Step(`^the recording gh's "([^"]*)" call has "([^"]*)"$`, w.ghCallHas)
	sc.Step(`^every call of the recording gh has "([^"]*)"$`, w.ghEveryCallHas)
	sc.Step(`^the recording gh was never called$`, func() error { return w.ghNeverCalled("") })
	sc.Step(`^the recording gh was never called as "([^"]*)"$`, w.ghNeverCalled)
	sc.Step(`^the recording gh was called once as "([^"]*)"$`, w.ghCalledOnce) // slice 190's check, worded as slice 199's scenarios do
	sc.Step(`^the clone has the commit "([^"]*)" closing the task "([^"]*)" and taking out its checks$`, w.cloneClosesTask)
	sc.Step(`^the clone has the commit "([^"]*)" adding the idea "([^"]*)" to the registry and touching "([^"]*)"$`, w.cloneAddsIdeaTouching)
	sc.Step(`^the clone is on the new branch "([^"]*)"$`, func(branch string) error {
		return w.git("checkout", "-q", "-b", branch)
	})
	sc.Step(`^the clone is on the new branch "([^"]*)", tracking the remote's main$`, func(branch string) error {
		return w.git("checkout", "-q", "-b", branch, "--track", "origin/main")
	})
	sc.Step(`^the GitHub repository in the environment is "([^"]*)"$`, func(repository string) error {
		w.dropVar("GITHUB_REPOSITORY=")
		w.vars = append(w.vars, "GITHUB_REPOSITORY="+repository)
		return nil
	})
	sc.Step(`^no GitHub repository in the environment$`, func() error {
		w.dropVar("GITHUB_REPOSITORY=")
		return nil
	})
	sc.Step(`^the remote's HEAD names the branch "([^"]*)", which it does not have$`, func(branch string) error {
		return w.gitIn(w.remote(), "symbolic-ref", "HEAD", "refs/heads/"+branch)
	})
	sc.Step(`^the remote's branch "([^"]*)" ends with "([^"]*)" then "([^"]*)", with no merge commit$`, w.remoteBranchEndsWith)
	sc.Step(`^the remote's branch "([^"]*)" has the remote main's commit "([^"]*)"$`, w.remoteBranchHasMainsCommit)
	sc.Step(`^the clone has the commit "([^"]*)" giving the ledger's task "([^"]*)" the check "([^"]*)"$`, w.cloneGivesTaskCheck)
	sc.Step(`^the clone is back on its branch "([^"]*)"$`, func(branch string) error {
		return w.git("checkout", "-q", branch)
	})
	sc.Step(`^the clone has merged its branch "([^"]*)" with the merge commit "([^"]*)"$`, func(branch, subject string) error {
		return w.git("merge", "-q", "--no-ff", "--no-verify", "-m", subject, branch)
	})
	sc.Step(`^the clone's HEAD is the head of the remote's branch "([^"]*)"$`, w.cloneHeadIsRemoteHead)
	sc.Step(`^the clone's HEAD is the commit "([^"]*)", with no rebase in progress$`, w.cloneHeadIsCommitNotRebasing)
}

// The remote's branch of the name has the commit of the remote's main with the
// subject (slice-192): that commit is an ancestor of the branch's head, so the
// same SHA is on both.
func (w *world) remoteBranchHasMainsCommit(branch, subject string) error {
	cmd := exec.Command(git.Bin(runtime.GOOS), "log", "--format=%H %s", "refs/heads/main")
	cmd.Dir = w.remote()
	cmd.Env = w.env()
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("git log in the remote: %w", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		sha, got, _ := strings.Cut(line, " ")
		if got != subject {
			continue
		}
		if err := w.gitIn(w.remote(), "merge-base", "--is-ancestor", sha, "refs/heads/"+branch); err != nil {
			return fmt.Errorf("the remote's branch %q does not have the remote main's commit %q (%s): %w\n%s", branch, subject, sha, err, w.report())
		}
		return nil
	}
	return fmt.Errorf("the remote's main has no commit %q\n%s", subject, w.report())
}

// The ledger's task of the id as it stands in the clone, written as a block by
// task add, with its done_when made the one check, and the ledger alone
// committed, past the hooks.
func (w *world) cloneGivesTaskCheck(subject, id, command string) error {
	file := filepath.Join(w.dir, w.data(w.ledgerPath()))
	text, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	var kept []string
	in, found, checks := false, false, false
	closeTask := func() {
		if in {
			kept = append(kept, "  done_when:", fmt.Sprintf("    - run: %q", command))
		}
		in = false
	}
	for _, line := range strings.Split(strings.TrimSuffix(string(text), "\n"), "\n") {
		if !strings.HasPrefix(line, " ") {
			closeTask()
			in = line == "- id: "+id
			found = found || in
			checks = false
		}
		switch {
		case in && strings.HasPrefix(line, "  done_when:"):
			checks = true
		case in && checks && strings.HasPrefix(line, "   "):
		default:
			checks = false
			kept = append(kept, line)
		}
	}
	closeTask()
	if !found {
		return fmt.Errorf("the ledger has no task %s written as a block:\n%s", id, text)
	}
	if err := os.WriteFile(file, []byte(strings.Join(kept, "\n")+"\n"), 0o644); err != nil {
		return err
	}
	return w.commitPaths(subject, w.data(w.ledgerPath()))
}

// The clone's HEAD is the head of the remote's branch of the name, by their
// full SHAs.
func (w *world) cloneHeadIsRemoteHead(branch string) error {
	head, err := w.head()
	if err != nil {
		return err
	}
	theirs, err := w.remoteBranchHead(branch)
	if err != nil {
		return err
	}
	if head != theirs {
		return fmt.Errorf("the clone's HEAD is %s, not the head of the remote's branch %q, %s\n%s", head, branch, theirs, w.report())
	}
	return nil
}

// The clone's HEAD commit has the subject, and no rebase is in progress: the
// git folder has neither rebase-merge nor rebase-apply.
func (w *world) cloneHeadIsCommitNotRebasing(subject string) error {
	got, err := w.gitOutput("log", "-1", "--format=%s", "HEAD")
	if err != nil {
		return err
	}
	if strings.TrimSpace(got) != subject {
		return fmt.Errorf("the clone's HEAD is %q, not %q\n%s", strings.TrimSpace(got), subject, w.report())
	}
	for _, name := range []string{"rebase-merge", "rebase-apply"} {
		out, err := w.gitOutput("rev-parse", "--git-path", name)
		if err != nil {
			return err
		}
		dir := strings.TrimSpace(out)
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(w.dir, dir)
		}
		if _, err := os.Stat(dir); err == nil {
			return fmt.Errorf("the clone has a rebase in progress (%s)\n%s", dir, w.report())
		}
	}
	return nil
}

// dropVar takes out of the variables the scenario sets those that start with
// the prefix.
func (w *world) dropVar(prefix string) {
	w.vars = slices.DeleteFunc(w.vars, func(v string) bool { return strings.HasPrefix(v, prefix) })
}

// The remote's branch of the name ends with the two commits, the older one
// before the newer, and has no merge commit.
func (w *world) remoteBranchEndsWith(branch, older, newer string) error {
	subjects, merges, err := w.remoteBranchHistory(branch)
	if err != nil {
		return err
	}
	if len(merges) > 0 {
		return fmt.Errorf("the remote's branch %q has merge commits %v:\n%s", branch, merges, strings.Join(subjects, "\n"))
	}
	if len(subjects) < 2 || subjects[0] != newer || subjects[1] != older {
		return fmt.Errorf("the remote's branch %q does not end with %q then %q:\n%s", branch, older, newer, strings.Join(subjects, "\n"))
	}
	return nil
}

// The recording gh's files: a line for each call, as its arguments were
// given, and a line for each pull request, its branch, its address and its
// state.
func (w *world) ghCalls() string { return filepath.Join(w.support, "gh-calls") }
func (w *world) ghPulls() string { return filepath.Join(w.support, "gh-pull-requests") }

// The folder the handlers of testdata/gh are written to.
func (w *world) ghHandlers() string { return filepath.Join(w.support, "gh-handlers") }

// What the recording gh says on stderr when it fails every call, if it does.
func (w *world) ghFailure() string { return filepath.Join(w.support, "gh-failure") }

// The files that steer pr view and pr update-branch: the first exists when the
// branch is to be left as it is; the second holds the number of calls pr view
// answers the previous head for after pr update-branch; the head a branch had
// when the recording gh was given its pull request is in the third, by the
// branch's name; the fourth is where pr update-branch rebases.
func (w *world) ghKeeps() string { return filepath.Join(w.support, "gh-update-keeps") }
func (w *world) ghStale() string { return filepath.Join(w.support, "gh-stale-calls") }
func (w *world) ghPrevious(branch string) string {
	return filepath.Join(w.support, "gh-previous-"+branch)
}
func (w *world) ghRebaseDir() string { return filepath.Join(w.support, "gh-rebase") }

// The commit a merged pull request was merged as, by its branch's name
// (slice 195): the file holds its full SHA.
func (w *world) ghMergeCommit(branch string) string {
	return filepath.Join(w.support, "gh-merge-commit-"+branch)
}

// The nodes api graphql answers for the pull request's reviews and review
// threads, each a JSON object, comma-separated (slice 179).
func (w *world) ghReviews() string { return filepath.Join(w.support, "gh-reviews") }
func (w *world) ghThreads() string { return filepath.Join(w.support, "gh-threads") }

// The nodes api graphql answers for the pull request's comments, those it
// answers instead for its first calls, and how many calls are left of them
// (slice 187).
func (w *world) ghComments() string      { return filepath.Join(w.support, "gh-comments") }
func (w *world) ghCommentsEarly() string { return filepath.Join(w.support, "gh-comments-early") }
func (w *world) ghCommentsLeft() string  { return filepath.Join(w.support, "gh-comments-left") }

// The comments api graphql answers beside those above while no pr comment call
// has been made and after the first one (slice 190).
func (w *world) ghCommentsBefore() string { return filepath.Join(w.support, "gh-comments-before") }
func (w *world) ghCommentsAfter() string  { return filepath.Join(w.support, "gh-comments-after") }

// ghScript is the recording gh's answers: dispatch.sh, which records the call
// and runs the handler testdata/gh/<command>-<subcommand>.sh for it; an api
// graphql call goes to graphql/*.sh (one per mutation, the first whose name the
// call has answers) and then to graphql-nodes.sh, which joins nodes/*.sh.
//
//go:embed testdata/gh
var ghScript embed.FS

//go:embed testdata/gh/dispatch.sh
var ghDispatch string

// recordingGh is a gh first on the PATH that stands in for GitHub: it records
// each call on a line, a line break inside an argument written as a backslash
// and an n (slice 184), fails every call when a step says so, and answers the
// call as its handler in testdata/gh does (features/README.md says how to add
// one). Any other call succeeds silently. A pull request's number is the end
// of its address. The environment names owner/scratch as the GitHub
// repository, as the fake GitHub does.
func (w *world) recordingGh(address string) error {
	bin, err := w.binOnPath()
	if err != nil {
		return err
	}
	if err := w.writeGhHandlers(); err != nil {
		return err
	}
	script := fmt.Sprintf("#!/bin/sh\nS=%s\nH=\"$S/gh-handlers\"\nGIT=%s\nREMOTE=%s\nADDRESS=%s\n%s",
		quote(filepath.ToSlash(w.support)), quote(filepath.ToSlash(git.Bin(runtime.GOOS))),
		quote(filepath.ToSlash(w.remote())), quote(address), lineFeeds([]byte(ghDispatch)))
	if err := w.writeProgram(filepath.Join(bin, "gh"), script); err != nil {
		return err
	}
	if !slices.Contains(w.vars, "GITHUB_REPOSITORY="+fakeRepository) {
		w.vars = append(w.vars, "GITHUB_REPOSITORY="+fakeRepository)
	}
	return nil
}

// writeGhHandlers writes the handlers other than the dispatcher under the
// support folder, as the dispatcher finds them, with their line ends as sh
// reads them.
func (w *world) writeGhHandlers() error {
	return fs.WalkDir(ghScript, "testdata/gh", func(path string, d fs.DirEntry, err error) error {
		rel := strings.TrimPrefix(strings.TrimPrefix(path, "testdata/gh"), "/")
		if err != nil || rel == "dispatch.sh" {
			return err
		}
		to := filepath.Join(w.ghHandlers(), filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(to, 0o755)
		}
		text, err := ghScript.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(to, []byte(lineFeeds(text)), 0o644)
	})
}

// lineFeeds is the text with its line ends as sh reads them: a checkout that
// turns them into CRLF (Windows) would leave a carriage return in every word.
func lineFeeds(text []byte) string { return strings.ReplaceAll(string(text), "\r\n", "\n") }

// The recording gh holds the pull request open for the branch, untitled.
func (w *world) ghHasPullRequest(address, branch string) error {
	return w.ghHasPullRequestIn(address, branch, "OPEN", false, "")
}

// The recording gh holds the pull request open for the branch, as a draft,
// untitled.
func (w *world) ghHasDraftPullRequest(address, branch string) error {
	return w.ghHasPullRequestIn(address, branch, "OPEN", true, "")
}

// The recording gh holds the pull request for the branch, OPEN, MERGED or
// CLOSED (closed without merging), a draft or not, with the title ("" for
// none), and keeps the head the remote's branch has now as the pull
// request's previous head.
func (w *world) ghHasPullRequestIn(address, branch, state string, draft bool, title string) error {
	if _, err := os.Stat(programPath(filepath.Join(w.support, "bin", "gh"))); err != nil {
		return errors.New("no recording gh: put one on the PATH first")
	}
	// A branch the remote does not have (a pull request merged and its branch
	// deleted) has no previous head to answer.
	if head, err := w.remoteBranchHead(branch); err == nil {
		if err := os.WriteFile(w.ghPrevious(branch), []byte(head+"\n"), 0o644); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(w.ghPulls(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(f, "%s %s %s %t %s\n", branch, address, state, draft, title)
	return errors.Join(err, f.Close())
}

// The recording gh's api graphql answers that the pull request has a review
// whose author's login is the one given.
func (w *world) ghAnswersReview(login string) error {
	return w.ghAnswersReviewBody(login, "")
}

// The same, whose body is the one given.
func (w *world) ghAnswersReviewBody(login, body string) error {
	return w.ghAddNode(w.ghReviews(), map[string]any{"author": map[string]any{"login": login}, "body": body})
}

// The comment of Codex's as #159 has it: its summary marker, its state line
// with the two reviews, each a status of a commit, and its heading. times, when
// it has two, are the comment's createdAt and updatedAt (slice 200).
func codexComment(login, codeStatus, codeSHA, securityStatus, securitySHA string, times ...string) map[string]any {
	review := func(status, sha string) string {
		return fmt.Sprintf(`{"commit_sha":%q,"has_any_findings":false,"has_unresolved_findings":false,"status":%q}`, sha, status)
	}
	body := "<!-- codex-pull-request-review-summary -->\n" +
		`<!-- codex-review-state:v1 {"code_review":` + review(codeStatus, codeSHA) + `,"security_review":` + review(securityStatus, securitySHA) + "} -->\n" +
		"## Codex Review Summary"
	comment := map[string]any{"author": map[string]any{"login": login}, "body": body}
	if len(times) == 2 {
		comment["createdAt"], comment["updatedAt"] = times[0], times[1]
	}
	return comment
}

// The recording gh's api graphql answers that the pull request has a comment
// by the login holding Codex's review state with the two statuses, both of
// the clone's HEAD as it is now (slice 200).
func (w *world) ghAnswersCodexState(login, code, security string) error {
	sha, err := w.head()
	if err != nil {
		return err
	}
	return w.ghAddNode(w.ghComments(), codexComment(login, code, sha, security, sha))
}

// The same, with those statuses in the answers to the first n calls, and
// both the status given in every later one.
func (w *world) ghAnswersCodexStateThen(login, code, security string, n int, then string) error {
	sha, err := w.head()
	if err != nil {
		return err
	}
	if err := w.ghAddNode(w.ghCommentsEarly(), codexComment(login, code, sha, security, sha)); err != nil {
		return err
	}
	if err := w.ghAddNode(w.ghComments(), codexComment(login, then, sha, then, sha)); err != nil {
		return err
	}
	return os.WriteFile(w.ghCommentsLeft(), fmt.Appendf(nil, "%d\n", n), 0o644)
}

// The same, with those statuses in every answer until the recording gh has
// been called as pr comment, and both the status given in every one after it
// (slice 190), the comments the other steps gave it in both.
func (w *world) ghAnswersCodexStateUntilComment(login, code, security, then string) error {
	sha, err := w.head()
	if err != nil {
		return err
	}
	if err := w.ghAddNode(w.ghCommentsBefore(), codexComment(login, code, sha, security, sha)); err != nil {
		return err
	}
	return w.ghAddNode(w.ghCommentsAfter(), codexComment(login, then, sha, then, sha))
}

// The recording gh's api graphql answers that the pull request has a comment
// by the author asking the login to review the clone's HEAD, as it is now,
// again.
func (w *world) ghAnswersReviewRequestForHead(author, login string) error {
	sha, err := w.head()
	if err != nil {
		return err
	}
	return w.ghAnswersReviewRequest(author, login, sha)
}

// The same, asking for the commit given: "@codex review", an empty line, then
// the marker line itos writes.
func (w *world) ghAnswersReviewRequest(author, login, sha string) error {
	body := "@codex review\n\n<!-- itos-review-request:v1 " + login + " " + sha + " -->"
	return w.ghAddNode(w.ghComments(), map[string]any{"author": map[string]any{"login": author}, "body": body})
}

// Some call of the recording gh starts with the words of the command, and has
// the full SHA of the clone's HEAD.
func (w *world) ghCallHasHead(command string) error {
	sha, err := w.head()
	if err != nil {
		return err
	}
	return w.ghCallHas(command, sha)
}

// The recording gh was called as the command once: never, or more than once,
// fails.
func (w *world) ghCalledOnce(command string) error { return w.ghCalledTimes(command, 1) }

// The recording gh was called as the command exactly n times (slice 209).
func (w *world) ghCalledTimes(command string, n int) error {
	calls, err := w.ghCallLines()
	if err != nil {
		return err
	}
	var named []string
	for _, call := range calls {
		if strings.HasPrefix(call, command+" ") || call == command {
			named = append(named, call)
		}
	}
	if len(named) != n {
		return fmt.Errorf("the recording gh was called as %q %d times, not %d:\n%s\n%s", command, len(named), n, strings.Join(named, "\n"), w.report())
	}
	return nil
}

// The recording gh's api graphql answers that the pull request has a review
// thread, resolved or not, on the path and line, with one comment whose body
// is the two lines.
func (w *world) ghAnswersThread(resolved bool, path string, line int, first, second string) error {
	return w.ghAddNode(w.ghThreads(), map[string]any{"isResolved": resolved, "path": path, "line": line,
		"comments": map[string]any{"nodes": []any{map[string]any{"body": first + "\n" + second}}}})
}

// The same, a thread with an id whose first comment's author has the login;
// its node is answered to a call that asks for the id (pushthread_test.go).
func (w *world) ghAnswersThreadBy(resolved bool, id, path string, line int, login, first, second string) error {
	if err := w.ghAddNode(w.ghThreads(), map[string]any{"id": id, "isResolved": resolved, "path": path, "line": line,
		"comments": map[string]any{"nodes": []any{map[string]any{"author": map[string]any{"login": login}, "body": first + "\n" + second}}}}); err != nil {
		return err
	}
	return w.ghNodeOfThread(id, resolved, path, line, login, first, second)
}

// The node, as JSON, joins those the file holds.
func (w *world) ghAddNode(file string, node any) error {
	if _, err := os.Stat(programPath(filepath.Join(w.support, "bin", "gh"))); err != nil {
		return errors.New("no recording gh: put one on the PATH first")
	}
	text, err := json.Marshal(node)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if info, err := f.Stat(); err == nil && info.Size() > 0 {
		text = append([]byte(","), text...)
	}
	_, err = f.Write(text)
	return errors.Join(err, f.Close())
}

// The full SHA of the remote's branch.
func (w *world) remoteBranchHead(branch string) (string, error) {
	cmd := exec.Command(git.Bin(runtime.GOOS), "rev-parse", "--verify", "refs/heads/"+branch)
	cmd.Dir = w.remote()
	cmd.Env = w.env()
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse refs/heads/%s in the remote: %w", branch, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// The fake GitHub gave a look at the watched run, and every look it gave was
// of the head the remote's branch has now, by its full SHA.
func (w *world) fakeGitHubGivenRemoteBranchHead(branch string) error {
	if w.github == nil {
		return errors.New("no fake GitHub: start one first")
	}
	head, err := w.remoteBranchHead(branch)
	if err != nil {
		return err
	}
	w.github.mu.Lock()
	given := slices.Clone(w.github.given)
	w.github.mu.Unlock()
	if len(given) == 0 {
		return fmt.Errorf("the fake GitHub was never asked for the watched run; it was asked %v\n%s", w.github.requests(), w.report())
	}
	for _, sha := range given {
		if sha != head {
			return fmt.Errorf("the fake GitHub was asked for the run of %q, not the head %s of the remote's branch %q\n%s", sha, head, branch, w.report())
		}
	}
	return nil
}

// Each call of the recording gh, as its arguments were given, in order; none
// when it was never called.
func (w *world) ghCallLines() ([]string, error) {
	text, err := os.ReadFile(w.ghCalls())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimRight(string(text), "\n"), "\n"), nil
}

// Some call of the recording gh starts with the words of the command, and
// has the text.
func (w *world) ghCallHas(command, text string) error {
	calls, err := w.ghCallLines()
	if err != nil {
		return err
	}
	var named []string
	for _, call := range calls {
		if strings.HasPrefix(call, command+" ") || call == command {
			named = append(named, call)
		}
	}
	if len(named) == 0 {
		return fmt.Errorf("the recording gh was never called as %q; its calls:\n%s\n%s", command, strings.Join(calls, "\n"), w.report())
	}
	if !slices.ContainsFunc(named, func(call string) bool { return strings.Contains(call, text) }) {
		return fmt.Errorf("no %q call of the recording gh has %q:\n%s\n%s", command, text, strings.Join(named, "\n"), w.report())
	}
	return nil
}

// Some call of the recording gh starts with the words of the command, and its
// --title, the text between "--title=" and the next " --body=", is that many
// Unicode code points long and ends with the text.
func (w *world) ghCallTitle(command string, length int, ending string) error {
	calls, err := w.ghCallLines()
	if err != nil {
		return err
	}
	var titles []string
	for _, call := range calls {
		if !strings.HasPrefix(call, command+" ") {
			continue
		}
		_, after, found := strings.Cut(call, "--title=")
		if !found {
			continue
		}
		title, _, _ := strings.Cut(after, " --body=")
		if utf8.RuneCountInString(title) == length && strings.HasSuffix(title, ending) {
			return nil
		}
		titles = append(titles, fmt.Sprintf("%q (%d characters)", title, utf8.RuneCountInString(title)))
	}
	return fmt.Errorf("no %q call of the recording gh has a --title of %d characters ending with %q; its titles: %s\n%s",
		command, length, ending, strings.Join(titles, ", "), w.report())
}

// Some call of the recording gh starts with the words of the command, and none
// of them has the text.
func (w *world) ghCallLacks(command, text string) error {
	calls, err := w.ghCallLines()
	if err != nil {
		return err
	}
	var named []string
	for _, call := range calls {
		if strings.HasPrefix(call, command+" ") || call == command {
			named = append(named, call)
		}
	}
	if len(named) == 0 {
		return fmt.Errorf("the recording gh was never called as %q; its calls:\n%s\n%s", command, strings.Join(calls, "\n"), w.report())
	}
	for _, call := range named {
		if strings.Contains(call, text) {
			return fmt.Errorf("a %q call of the recording gh has %q: %s\n%s", command, text, call, w.report())
		}
	}
	return nil
}

// Some call of the recording gh has each text, and the first with the first
// text comes before the first with the second.
func (w *world) ghCallOrder(first, second string) error {
	calls, err := w.ghCallLines()
	if err != nil {
		return err
	}
	at := func(text string) int {
		return slices.IndexFunc(calls, func(call string) bool { return strings.Contains(call, text) })
	}
	a, b := at(first), at(second)
	if a < 0 || b < 0 || b < a {
		return fmt.Errorf("the recording gh's first call with %q (%d) does not come before its first call with %q (%d); its calls:\n%s\n%s",
			first, a, second, b, strings.Join(calls, "\n"), w.report())
	}
	return nil
}

// Every call of the recording gh has the text, and there was one.
func (w *world) ghEveryCallHas(text string) error {
	calls, err := w.ghCallLines()
	if err != nil {
		return err
	}
	if len(calls) == 0 {
		return fmt.Errorf("the recording gh was never called\n%s", w.report())
	}
	for _, call := range calls {
		if !strings.Contains(call, text) {
			return fmt.Errorf("the recording gh was called without %q: %s\n%s", text, call, w.report())
		}
	}
	return nil
}

// The recording gh was never called, or never as the command ("" for either).
func (w *world) ghNeverCalled(command string) error {
	calls, err := w.ghCallLines()
	if err != nil {
		return err
	}
	for _, call := range calls {
		if command == "" || strings.HasPrefix(call, command+" ") || call == command {
			return fmt.Errorf("the recording gh was called: %s\n%s", call, w.report())
		}
	}
	return nil
}

// The commit work done makes for a task with checks (slice 177), without
// running it: the registry's item given the status done and the owner
// someone, the ledger's entry without its done_when, both files and nothing
// else, committed past the hooks.
func (w *world) cloneClosesTask(subject, id string) error {
	state := regexp.MustCompile(`owner: [^,]*, status: [^,]*`)
	closed := false
	for i, line := range w.registryLines {
		if strings.HasPrefix(line, "  - { id: "+id+",") {
			w.registryLines[i] = state.ReplaceAllString(line, "owner: someone, status: done")
			closed = true
		}
	}
	if !closed {
		return fmt.Errorf("the registry has no item %q to close", id)
	}
	if err := w.writeRegistryLines(); err != nil {
		return err
	}
	at := slices.IndexFunc(w.ledger, func(t ledgerTask) bool { return t.id == id })
	if at < 0 {
		return fmt.Errorf("the ledger has no task %q", id)
	}
	w.ledger[at].checks = nil
	if err := w.write(w.data(w.ledgerPath()), w.ledgerText()); err != nil {
		return err
	}
	return w.commitPaths(subject, w.data(startingRegistry), w.data(w.ledgerPath()))
}

// A new idea in the registry and a line in the file, in one commit past the
// hooks (slice 177).
func (w *world) cloneAddsIdeaTouching(subject, id, path string) error {
	if err := w.addIdeaIn(w.dir, id); err != nil {
		return err
	}
	if err := writeLine(w.dir, path, subject); err != nil {
		return err
	}
	return w.commitPaths(subject, w.data(startingRegistry), path)
}

// The paths, and nothing else, committed past the hooks.
func (w *world) commitPaths(subject string, paths ...string) error {
	if err := w.git(append([]string{"add", "--"}, paths...)...); err != nil {
		return err
	}
	return w.git(append([]string{"commit", "-q", "--no-verify", "-m", subject, "--"}, paths...)...)
}
