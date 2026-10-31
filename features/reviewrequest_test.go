// The steps of slice 215 (review-request.feature): itos review request asks
// each configured reviewer for a pull request's head once. They give the
// recording gh a comment requesting a review with itos's request marker, at
// the clone's HEAD or a commit named; a review by a login of the clone's HEAD
// or of a commit named, with its commit; and they check every call of a
// command, in order, against a doc string.
package features

import (
	"fmt"
	"strings"

	"github.com/cucumber/godog"
)

func initializeReviewRequestSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the recording gh answers that the pull request has a comment by "([^"]*)" requesting "([^"]*)" of "([^"]*)" at the clone's HEAD$`,
		func(author, line, login string) error {
			sha, err := w.head()
			if err != nil {
				return err
			}
			return w.ghAnswersRequest(author, line, login, sha)
		})
	sc.Step(`^the recording gh answers that the pull request has a comment by "([^"]*)" requesting "([^"]*)" of "([^"]*)" at the commit "([^"]*)"$`,
		w.ghAnswersRequest)
	sc.Step(`^the recording gh answers that the pull request has a review by "([^"]*)" of the clone's HEAD whose body is "([^"]*)"$`,
		func(login, body string) error {
			sha, err := w.head()
			if err != nil {
				return err
			}
			return w.ghAnswersReviewOf(login, sha, body)
		})
	sc.Step(`^the recording gh answers that the pull request has a review by "([^"]*)" of the commit "([^"]*)" whose body is "([^"]*)"$`,
		w.ghAnswersReviewOf)
	sc.Step(`^the recording gh's "([^"]*)" calls are these lines, in order, @HEAD:([^@]*)@ the full SHA of the head of the remote's branch "([^"]*)":$`,
		w.ghCallsAre)
}

// The recording gh's api graphql answers that the pull request has a comment
// by the author requesting the line of the login at the commit: the line, an
// empty line, then itos's request marker for the login and the commit.
func (w *world) ghAnswersRequest(author, line, login, sha string) error {
	body := line + "\n\n<!-- itos-review-request:v1 " + login + " " + sha + " -->"
	return w.ghAddNode(w.ghComments(), map[string]any{"author": map[string]any{"login": author}, "body": body})
}

// The recording gh's api graphql answers that the pull request has a review
// by the login of the commit, whose body is the one given.
func (w *world) ghAnswersReviewOf(login, sha, body string) error {
	return w.ghAddNode(w.ghReviews(), map[string]any{"author": map[string]any{"login": login}, "body": body,
		"commit": map[string]any{"oid": sha}})
}

// Every call of the recording gh that starts with the words of the command
// is, in order, a line of the doc string, and there are as many: the
// placeholder of the branch replaced by the full SHA of the head its remote
// branch has now, a line end inside a call written as the two characters \n,
// as dispatch.sh records it.
func (w *world) ghCallsAre(command, headBranch, branch string, lines *godog.DocString) error {
	calls, err := w.ghCallLines()
	if err != nil {
		return err
	}
	sha, err := w.remoteBranchHead(branch)
	if err != nil {
		return err
	}
	want := strings.Split(strings.ReplaceAll(strings.ReplaceAll(lines.Content, "\r\n", "\n"), headOf(headBranch), sha), "\n")
	var got []string
	for _, call := range calls {
		if strings.HasPrefix(call, command+" ") || call == command {
			got = append(got, call)
		}
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		return fmt.Errorf("the recording gh's %q calls are\n%s\nnot\n%s\n%s", command, strings.Join(got, "\n"), strings.Join(want, "\n"), w.report())
	}
	return nil
}
