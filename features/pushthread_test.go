// The steps of itos push thread that ask GitHub for a thread's node (slice 201,
// push-threads.feature): the recording gh keeps, for a review thread a step
// gave it an id, the node an api graphql call with an id argument is answered
// with, and answers an id it has no node for as GitHub and gh do.
package features

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
)

func initializePushThreadSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the recording gh answers that the review thread "([^"]*)" has a reply by "([^"]*)" saying "([^"]*)"$`, w.ghNodeReply)
	sc.Step(`^the recording gh answers, asked for the node "([^"]*)", an unresolved review thread of the pull request "([^"]*)" in the repository "([^"]*)" on "([^"]*)" line (\d+) by "([^"]*)", its first comment the lines "([^"]*)" and "([^"]*)"$`,
		func(id, address, repository, path string, line int, login, first, second string) error {
			return w.ghNodeThread(id, false, address, repository, path, line, login, first, second)
		})
}

// The node the recording gh answers a call asking for the node with the id.
func (w *world) ghNode(id string) string { return filepath.Join(w.support, "gh-node-"+id) }

// ghNodeOfThread is the node for a thread a step gave the recording gh with an
// id: the thread as the pull request's list has it, with the pull request the
// recording gh was last given and the fake's repository.
func (w *world) ghNodeOfThread(id string, resolved bool, path string, line int, login, first, second string) error {
	text, err := os.ReadFile(w.ghPulls())
	if err != nil || len(strings.TrimSpace(string(text))) == 0 {
		return errors.New("no pull request: give the recording gh one before its review thread")
	}
	lines := strings.Split(strings.TrimSpace(string(text)), "\n")
	fields := strings.Fields(lines[len(lines)-1])
	return w.ghNodeThread(id, resolved, fields[1], fakeRepository, path, line, login, first, second)
}

// ghNodeThread writes the node for the id: a review thread of the pull request
// at the address, in the repository.
func (w *world) ghNodeThread(id string, resolved bool, address, repository, path string, line int, login, first, second string) error {
	number, err := strconv.Atoi(address[strings.LastIndex(address, "/")+1:])
	if err != nil {
		return fmt.Errorf("no number at the end of the address %q", address)
	}
	text, err := json.Marshal(map[string]any{
		"id": id, "isResolved": resolved, "path": path, "line": line,
		"repository":  map[string]any{"nameWithOwner": repository},
		"pullRequest": map[string]any{"number": number, "url": address},
		"comments": map[string]any{"nodes": []any{
			map[string]any{"author": map[string]any{"login": login}, "body": first + "\n" + second}}},
	})
	if err != nil {
		return err
	}
	return os.WriteFile(w.ghNode(id), text, 0o644)
}

// ghNodeReply adds a comment by the login with the body after the others of
// the thread's node, and of its node alone: the pull request's list asks for
// the first comment only.
func (w *world) ghNodeReply(id, login, body string) error {
	text, err := os.ReadFile(w.ghNode(id))
	if err != nil {
		return fmt.Errorf("the recording gh has no node for the review thread %q: give it the thread first", id)
	}
	var whole map[string]any
	if err := json.Unmarshal(text, &whole); err != nil {
		return err
	}
	comments, _ := whole["comments"].(map[string]any)
	nodes, _ := comments["nodes"].([]any)
	whole["comments"] = map[string]any{"nodes": append(nodes,
		map[string]any{"author": map[string]any{"login": login}, "body": body})}
	text, err = json.Marshal(whole)
	if err != nil {
		return err
	}
	return os.WriteFile(w.ghNode(id), text, 0o644)
}
