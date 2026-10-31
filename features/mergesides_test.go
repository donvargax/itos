// The steps of bugs 158 and 159 (commit-msg.feature): a merge whose two
// sides change one file, committed or in progress, a merge of a root commit
// with no history in common with HEAD, and a merge of a line that deletes a
// file and one that changes it. How each is built is the comment above each
// bug's scenarios.
package features

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cucumber/godog"
	"github.com/donvargax/itos/v8/internal/git"
)

func initializeMergeSidesSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^a merge commit "([^"]*)" on top of it, whose two sides change (neighbouring lines|the same line) of "([^"]*)", (.+)$`, w.mergeOfSides)
	sc.Step(`^a merge of the branch "([^"]*)" is in progress, whose two sides change (neighbouring lines|the same line) of "([^"]*)", (git's merge|git merge-tree's conflict tree) staged$`, w.mergeOfSidesInProgress)
	sc.Step(`^a merge commit "([^"]*)" on top of it, of a root commit with no history in common with it, with no change of its own$`, w.mergeOfUnrelatedRoot)
	sc.Step(`^a merge commit "([^"]*)" on top of it, whose main line deletes "([^"]*)" and whose topic line changes it, keeping the topic's change$`, w.mergeOfDeleteAndChange)
}

// The merge whose tree is the one git merge-tree writes for its two lines.
const asMergeTreeWritesIt = "the conflict left as git merge-tree writes it, markers and all"

// The notes both sides change, as the base commit writes them.
const notesBase = "one\ntwo\nthree\nfour\nfive\n"

// The texts of the notes on the main line and the topic line, for "change
// neighbouring lines" or "change the same line".
func sidesTexts(change string) (mainText, topicText string) {
	topicText = strings.Replace(notesBase, "two\n", "two, the topic's\n", 1)
	if change == "the same line" {
		return strings.Replace(notesBase, "two\n", "two, main's\n", 1), topicText
	}
	return strings.Replace(notesBase, "four\n", "four, main's\n", 1), topicText
}

// The notes as the merge's tree holds them: git's clean merge of the two
// sides' neighbouring lines, with or without a line of the merge's own, or
// the same line's conflict resolved by a line neither side has, or by taking
// the topic's.
func mergedNotes(change, merge string) (string, error) {
	mainText, topicText := sidesTexts(change)
	clean := strings.Replace(topicText, "four\n", "four, main's\n", 1)
	switch change + ", " + merge {
	case "neighbouring lines, with no change of its own":
		return clean, nil
	case "neighbouring lines, with a line of its own added to it":
		return clean + "six, the merge's\n", nil
	case "the same line, the conflict resolved by a line neither side has":
		return strings.Replace(mainText, "two, main's\n", "two, both's\n", 1), nil
	case "the same line, the conflict resolved by taking the topic side's line":
		return topicText, nil
	}
	return "", fmt.Errorf("no merge whose two sides change %s %s", change, merge)
}

// A merge commit on top of HEAD whose two sides change the path, built in a
// scratch index, HEAD's branch moved to it.
func (w *world) mergeOfSides(message, change, path, merge string) error {
	if len(w.ledger) == 0 {
		return errors.New("the repository's ledger has no task to name")
	}
	footer := "\n\nTask: " + w.ledger[0].id + "\n"
	head, err := w.head()
	if err != nil {
		return err
	}
	s := w.scratchIndex()
	notes, err := s.commitOn(head, "chore: write the notes"+footer, map[string]string{path: notesBase})
	if err != nil {
		return err
	}
	mainText, topicText := sidesTexts(change)
	mainLine, err := s.commitOn(notes, "chore: change the notes on the main line"+footer, map[string]string{path: mainText})
	if err != nil {
		return err
	}
	topic, err := s.commitOn(notes, "chore: change the notes on the topic line"+footer, map[string]string{path: topicText})
	if err != nil {
		return err
	}
	tree, err := s.mergedTree(change, merge, path, mainLine, topic)
	if err != nil {
		return err
	}
	merged, err := s.commit(tree, message+"\n", mainLine, topic)
	if err != nil {
		return err
	}
	if _, err := s.git("update-ref", "HEAD", merged); err != nil {
		return err
	}
	w.commits = append(w.commits, notes, mainLine, topic, merged)
	return nil
}

// The merge's tree: the one git merge-tree writes for the two lines, or the
// main line's with the path's text as mergedNotes gives it.
func (s scratchIndex) mergedTree(change, merge, path, mainLine, topic string) (string, error) {
	if merge == asMergeTreeWritesIt {
		return s.mergeTree(mainLine, topic)
	}
	text, err := mergedNotes(change, merge)
	if err != nil {
		return "", err
	}
	return s.tree(mainLine, map[string]string{path: text})
}

// A merge commit on top of HEAD of a main line that deletes the path and a
// topic line that changes it, its tree the main line's with the topic's
// path, built in a scratch index, HEAD's branch moved to it.
func (w *world) mergeOfDeleteAndChange(message, path string) error {
	if len(w.ledger) == 0 {
		return errors.New("the repository's ledger has no task to name")
	}
	footer := "\n\nTask: " + w.ledger[0].id + "\n"
	head, err := w.head()
	if err != nil {
		return err
	}
	s := w.scratchIndex()
	notes, err := s.commitOn(head, "chore: write the notes"+footer, map[string]string{path: notesBase})
	if err != nil {
		return err
	}
	if _, err := s.git("read-tree", notes); err != nil {
		return err
	}
	if _, err := s.git("update-index", "--force-remove", path); err != nil {
		return err
	}
	deleted, err := s.git("write-tree")
	if err != nil {
		return err
	}
	mainLine, err := s.commit(deleted, "chore: remove the notes on the main line"+footer, notes)
	if err != nil {
		return err
	}
	_, topicText := sidesTexts("the same line")
	topic, err := s.commitOn(notes, "chore: change the notes on the topic line"+footer, map[string]string{path: topicText})
	if err != nil {
		return err
	}
	tree, err := s.tree(mainLine, map[string]string{path: topicText})
	if err != nil {
		return err
	}
	merged, err := s.commit(tree, message+"\n", mainLine, topic)
	if err != nil {
		return err
	}
	if _, err := s.git("update-ref", "HEAD", merged); err != nil {
		return err
	}
	w.commits = append(w.commits, notes, mainLine, topic, merged)
	return nil
}

// The same three commits made with git's porcelain, the topic line's on the
// new branch, and git merge --no-ff --no-commit of it on HEAD's branch, what
// the scenario staged unstaged first and left in the work tree. Where the
// sides change the same line, git merge stops at the conflict, MERGE_HEAD
// written, and the tree git merge-tree writes for HEAD and MERGE_HEAD is
// staged in place of git merge's index.
func (w *world) mergeOfSidesInProgress(branch, change, path, staged string) error {
	if len(w.ledger) == 0 {
		return errors.New("the repository's ledger has no task to name")
	}
	footer := "Task: " + w.ledger[0].id
	out, err := w.gitOutput("symbolic-ref", "--short", "HEAD")
	if err != nil {
		return err
	}
	home := strings.TrimSpace(out)
	mainText, topicText := sidesTexts(change)
	commit := func(subject, text string) error {
		if err := w.write(path, text); err != nil {
			return err
		}
		if err := w.git("add", "--", path); err != nil {
			return err
		}
		return w.git("commit", "-q", "--no-verify", "-m", subject, "-m", footer, "--", path)
	}
	if err := w.git("reset", "-q"); err != nil {
		return err
	}
	if err := commit("chore: write the notes", notesBase); err != nil {
		return err
	}
	if err := w.git("checkout", "-q", "-b", branch); err != nil {
		return err
	}
	if err := commit("chore: change the notes on the topic line", topicText); err != nil {
		return err
	}
	if err := w.git("checkout", "-q", home); err != nil {
		return err
	}
	if err := commit("chore: change the notes on the main line", mainText); err != nil {
		return err
	}
	merging := w.git("merge", "-q", "--no-ff", "--no-commit", branch)
	if staged == "git's merge" {
		return merging
	}
	if merging == nil {
		return errors.New("git merge of the two sides stopped at no conflict")
	}
	head, err := w.head()
	if err != nil {
		return err
	}
	theirs, err := w.gitOutput("rev-parse", "MERGE_HEAD")
	if err != nil {
		return err
	}
	tree, err := w.scratchIndex().mergeTree(head, strings.TrimSpace(theirs))
	if err != nil {
		return err
	}
	return w.git("read-tree", tree)
}

// A root commit adding other.md and a README.md of its own, and a merge of
// HEAD and it whose tree is HEAD's with other.md added, built in a scratch
// index, HEAD's branch moved to it.
func (w *world) mergeOfUnrelatedRoot(message string) error {
	if len(w.ledger) == 0 {
		return errors.New("the repository's ledger has no task to name")
	}
	head, err := w.head()
	if err != nil {
		return err
	}
	other := map[string]string{"other.md": "The other line.\n"}
	s := w.scratchIndex()
	rootTree, err := s.tree(git.EmptyTree, map[string]string{"other.md": other["other.md"], "README.md": "# The other line\n"})
	if err != nil {
		return err
	}
	root, err := s.commit(rootTree, "chore: start the other line\n\nTask: "+w.ledger[0].id+"\n")
	if err != nil {
		return err
	}
	tree, err := s.tree(head, other)
	if err != nil {
		return err
	}
	merged, err := s.commit(tree, message+"\n", head, root)
	if err != nil {
		return err
	}
	if _, err := s.git("update-ref", "HEAD", merged); err != nil {
		return err
	}
	w.commits = append(w.commits, root, merged)
	return nil
}

// scratchIndex is git's plumbing run in the scenario's repository on an
// index of its own, so that what the scenario staged stays staged and out of
// every commit it makes.
type scratchIndex struct {
	w     *world
	index string
}

func (w *world) scratchIndex() scratchIndex {
	return scratchIndex{w: w, index: filepath.Join(w.support, "merge-index")}
}

// git's stdout, trimmed.
func (s scratchIndex) git(args ...string) (string, error) {
	cmd := exec.Command(git.Bin(runtime.GOOS), args...)
	cmd.Dir = s.w.dir
	cmd.Env = append(s.w.env(), "GIT_INDEX_FILE="+s.index)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// The tree git merge-tree --write-tree writes for the two commits, named as
// given (full SHAs, as itos names them, label its conflict markers): the
// first line it prints, where it exits 0 on a clean merge or 1 on a
// conflict.
func (s scratchIndex) mergeTree(first, second string) (string, error) {
	args := []string{"merge-tree", "--write-tree", "--no-messages", first, second}
	cmd := exec.Command(git.Bin(runtime.GOOS), args...)
	cmd.Dir = s.w.dir
	cmd.Env = s.w.env()
	out, err := cmd.Output()
	if exit := (*exec.ExitError)(nil); errors.As(err, &exit) && exit.ExitCode() == 1 {
		err = nil
	}
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	tree, _, _ := strings.Cut(string(out), "\n")
	return tree, nil
}

// A tree: the commit's (or tree's), with each path set to its text.
func (s scratchIndex) tree(from string, texts map[string]string) (string, error) {
	if _, err := s.git("read-tree", from); err != nil {
		return "", err
	}
	for path, text := range texts {
		file := filepath.Join(s.w.support, "merge-blob")
		if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
			return "", err
		}
		blob, err := s.git("hash-object", "-w", file)
		if err != nil {
			return "", err
		}
		if _, err := s.git("update-index", "--add", "--cacheinfo", "100644,"+blob+","+path); err != nil {
			return "", err
		}
	}
	return s.git("write-tree")
}

// A commit of the tree on the parents.
func (s scratchIndex) commit(tree, message string, parents ...string) (string, error) {
	args := []string{"commit-tree", tree, "-m", message}
	for _, p := range parents {
		args = append(args, "-p", p)
	}
	return s.git(args...)
}

// A commit on the parent of its tree with each path set to its text.
func (s scratchIndex) commitOn(parent, message string, texts map[string]string) (string, error) {
	tree, err := s.tree(parent, texts)
	if err != nil {
		return "", err
	}
	return s.commit(tree, message, parent)
}
