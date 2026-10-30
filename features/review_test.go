// The steps of slice 206 (review.feature): itos review post records a
// reviewer's verdict as a pull request comment bound to the head it judged.
// They give the recording gh a comment recording a verdict (at the head a
// remote's branch has when gh is called, written as the placeholders
// @HEAD:<branch>@ and @SHORT:<branch>@, which nodes/28-comment-heads.sh and
// nodes/27-comment-short-heads.sh replace at each call, or at a commit
// named), on the first or the second page of comments, and a comment saying
// any text; and they check the body a call posted, line by line, and a JSON
// key holding a remote branch's head.
package features

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cucumber/godog"
)

func initializeReviewSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the recording gh answers that the pull request has a comment by "([^"]*)" recording the "([^"]*)" review of "([^"]*)" at the head of the remote's branch "([^"]*)" as round (\d+): "([^"]*)"$`,
		func(login, stage, item, branch string, round int, verdict string) error {
			if err := w.ghKeepsHead(branch); err != nil {
				return err
			}
			return w.ghAnswersVerdict(w.ghComments(), login, stage, item, headOf(branch), shortOf(branch), round, verdict)
		})
	sc.Step(`^the recording gh answers that the pull request has a comment by "([^"]*)" recording the "([^"]*)" review of "([^"]*)" at the commit "([^"]*)" as round (\d+): "([^"]*)"$`,
		func(login, stage, item, sha string, round int, verdict string) error {
			return w.ghAnswersVerdict(w.ghComments(), login, stage, item, sha, first7(sha), round, verdict)
		})
	sc.Step(`^the recording gh answers that the pull request has, on a second page of comments, a comment by "([^"]*)" recording the "([^"]*)" review of "([^"]*)" at the head of the remote's branch "([^"]*)" as round (\d+): "([^"]*)"$`,
		func(login, stage, item, branch string, round int, verdict string) error {
			if err := w.ghKeepsHead(branch); err != nil {
				return err
			}
			return w.ghAnswersVerdict(w.ghCommentsPage2(), login, stage, item, headOf(branch), shortOf(branch), round, verdict)
		})
	sc.Step(`^the recording gh answers that the pull request has a comment by "([^"]*)" saying "([^"]*)"$`, func(login, text string) error {
		return w.ghAddNode(w.ghComments(), map[string]any{"author": map[string]any{"login": login}, "body": text})
	})
	sc.Step(`^the recording gh's "([^"]*)" call has the field "body" holding these lines, @HEAD:([^@]*)@ the full SHA of the head of the remote's branch "([^"]*)" and @SHORT:([^@]*)@ its first 7 characters:$`,
		w.ghCallBodyIs)
	sc.Step(`^its JSON's "([^"]*)" holds the full SHA of the head of the remote's branch "([^"]*)"$`, func(key, branch string) error {
		sha, err := w.remoteBranchHead(branch)
		if err != nil {
			return err
		}
		return w.jsonValueIs(key, jsonOf(sha))
	})
}

// The first 7 characters of a SHA, as itos abbreviates one.
func first7(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// The text a comment holds in place of the first 7 characters of the head
// the remote's branch has when gh is called.
func shortOf(branch string) string { return "@SHORT:" + branch + "@" }

// The recording gh's api graphql answers that the pull request has, in the
// file of comments given, a comment by the login recording the verdict as
// itos review post writes one: the readable line with the head's first 7
// characters, an empty line, and the marker with the full SHA and the round.
func (w *world) ghAnswersVerdict(file, login, stage, item, sha, short string, round int, verdict string) error {
	body := fmt.Sprintf("itos: %s review of %s at %s: %s\n\n"+
		`<!-- itos:v1 {"record":"verdict","item":%q,"stage":%q,"verdict":%q,"head":%q,"round":%d} -->`,
		stage, item, short, verdict, item, stage, verdict, sha, round)
	return w.ghAddNode(file, map[string]any{"author": map[string]any{"login": login}, "body": body})
}

// The first call of the recording gh that starts with the words of the
// command has, after its " -f body=", to the end of the recorded line, the
// doc string's lines joined by the two characters \n (dispatch.sh records a
// line end inside an argument so), the placeholder of the first branch
// replaced by the full SHA of the head its remote branch has now and the
// second by its first 7 characters. So body must be the call's last field:
// what follows it on the line would be read as part of it.
func (w *world) ghCallBodyIs(command, headBranch, branch, shortBranch string, lines *godog.DocString) error {
	calls, err := w.ghCallLines()
	if err != nil {
		return err
	}
	sha, err := w.remoteBranchHead(branch)
	if err != nil {
		return err
	}
	want := strings.ReplaceAll(strings.ReplaceAll(lines.Content, "\r\n", "\n"), "\n", `\n`)
	want = strings.ReplaceAll(want, headOf(headBranch), sha)
	want = strings.ReplaceAll(want, shortOf(shortBranch), first7(sha))
	for _, call := range calls {
		if !strings.HasPrefix(call, command+" ") && call != command {
			continue
		}
		_, got, found := strings.Cut(call, " -f body=")
		if !found {
			return fmt.Errorf("the first %q call of the recording gh has no body field: %s\n%s", command, call, w.report())
		}
		if got != want {
			return fmt.Errorf("the first %q call of the recording gh has the body\n%s\nnot\n%s\n%s", command, got, want, w.report())
		}
		return nil
	}
	return fmt.Errorf("the recording gh was never called as %q; its calls:\n%s\n%s", command, strings.Join(calls, "\n"), w.report())
}

// A JSON text of the value, as a step writes one for jsonValueIs.
func jsonOf(v any) string {
	text, _ := json.Marshal(v)
	return string(text)
}
