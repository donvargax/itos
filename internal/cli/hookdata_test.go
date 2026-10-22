package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scratchFolder is a new folder, made the current one, that no repository
// above it holds, with git's own environment cleared, away from the user's
// git config, a fixed author, and no ITOS_CONFIG.
func scratchFolder(t *testing.T) string {
	t.Helper()
	disableScratchMaintenance(t)
	dir := t.TempDir()
	for _, name := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_COMMON_DIR", "ITOS_CONFIG", AmendEnv} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	for name, value := range map[string]string{
		"GIT_CEILING_DIRECTORIES": filepath.Dir(dir), "GIT_CONFIG_GLOBAL": os.DevNull, "GIT_CONFIG_NOSYSTEM": "1",
		"GIT_AUTHOR_NAME": "A", "GIT_AUTHOR_EMAIL": "a@example.com",
		"GIT_COMMITTER_NAME": "A", "GIT_COMMITTER_EMAIL": "a@example.com",
	} {
		t.Setenv(name, value)
	}
	t.Chdir(dir)
	return dir
}

// writeFiles writes each file, from the current folder.
func writeFiles(t *testing.T, files map[string]string) {
	t.Helper()
	for path, text := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

const smokedConfig = `version: 1
tests:
  scenario: { root: features, id: "ID-[A-Z]+-\\d+", tag_prefix: "@", smoke: { file: features/smoke.yaml, every_file: false } }
  unsmoked: { root: features, id: "ID-[A-Z]+-\\d+" }
`

// The hook's data rule as the index holds it: nothing outside a repository
// or where git cannot say where the index is but the error; no rejection
// for a commit in a repository without a config, whose missing config is
// the next rule's error; data that cannot be read one data-unreadable
// problem; and a staged feature file with no smoke scenario the smoke
// rule's problem, under the staged reject message (slice 117).
func TestStagedDataIssues(t *testing.T) {
	scratchFolder(t)
	// A commit made under a hook takes the author date git exports to it,
	// which would read as an amend of HEAD.
	t.Setenv("GIT_AUTHOR_DATE", "")
	os.Unsetenv("GIT_AUTHOR_DATE")
	writeFiles(t, map[string]string{"msg": "docs: x\n"})
	hook := func() (int, string, error) {
		t.Helper()
		var stderr strings.Builder
		code, err := runHook([]string{"commit-msg", "msg"}, Out{Stdout: &stderr, Stderr: &stderr})
		return code, stderr.String(), err
	}
	if _, _, err := hook(); err == nil {
		t.Error("outside a repository: no error")
	}

	gitIn(t, "init", "-q", "-b", "main")
	writeFiles(t, map[string]string{"README.md": "scratch\n"})
	gitIn(t, "add", "README.md")
	if code, stderr, err := hook(); code != 0 || stderr != "" || err == nil {
		t.Errorf("no config: exit %d, %v\n%s", code, err, stderr)
	}

	useWrappedGit(t, "rev-parse", "", "")
	if _, _, err := hook(); err == nil {
		t.Error("an index git cannot place: no error")
	}
	t.Setenv("ITOS_GIT", "")
	os.Unsetenv("ITOS_GIT")

	writeFiles(t, map[string]string{
		"itos.yaml":          "version: 1\nledger: { files: \"tasks/phase-{group}.yaml\", id: \"T-\\\\d+\" }\n",
		"tasks/phase-1.yaml": "- { id: T-001, type: build, title: One }\n",
	})
	gitIn(t, "add", ".")
	gitIn(t, "commit", "-q", "-m", "docs: start")
	writeFiles(t, map[string]string{"tasks/work-items.yaml": "phases: [\n"})
	gitIn(t, "add", "tasks/work-items.yaml")
	code, stderr, err := hook()
	if code != ExitPolicy || err != nil || !strings.HasPrefix(stderr, "Commit rejected:\n  - ") || strings.Count(stderr, "\n") != 2 {
		t.Errorf("a registry that cannot be read: exit %d, %v\n%s", code, err, stderr)
	}
	gitIn(t, "rm", "-q", "--cached", "tasks/work-items.yaml")

	writeFiles(t, map[string]string{
		"itos.yaml": strings.Replace(smokedConfig, "every_file: false", "every_file: true", 1) +
			"commits: { reject_message: \"Refused:\" }\n",
		"features/a.feature":  "Feature: A\n\n  @ID-A-01\n  Scenario: One\n    When it runs\n",
		"features/smoke.yaml": "- file: a.feature\n  scenarios: [{ id: \"@ID-A-01\", why: listed }]\n",
	})
	gitIn(t, "add", "itos.yaml", "features")
	gitIn(t, "commit", "-q", "-m", "test: the smoke set")
	writeFiles(t, map[string]string{"features/b.feature": "Feature: B\n\n  @ID-B-01\n  Scenario: One\n    When it runs\n"})
	gitIn(t, "add", "features/b.feature")
	code, stderr, err = hook()
	if code != ExitPolicy || err != nil || !strings.HasPrefix(stderr, "Refused:\n  - b.feature ") || strings.Count(stderr, "\n") != 2 {
		t.Errorf("a staged feature file with no smoke scenario: exit %d, %v\n%s", code, err, stderr)
	}
}
