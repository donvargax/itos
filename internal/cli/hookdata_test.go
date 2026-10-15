package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
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

// The kinds judged picks are the ones judged, among those with a smoke set
// (unsmoked has none): a sound set prints its line
// (without every_file, that every smoke test is live), and a set that
// cannot be read is one smoke-unreadable problem.
func TestJudgedSmokeFindings(t *testing.T) {
	scratchFolder(t)
	writeFiles(t, map[string]string{
		"itos.yaml":           smokedConfig,
		"features/a.feature":  "Feature: A\n\n  @ID-A-01\n  Scenario: One\n    When it runs\n",
		"features/smoke.yaml": "- file: a.feature\n  scenarios: [{ id: \"@ID-A-01\", why: listed }]\n",
	})
	cfg, err := config.Load("itos.yaml")
	if err != nil {
		t.Fatal(err)
	}
	found, lines, err := judgedSmokeFindings(cfg, nil)
	if err != nil || len(found) != 0 || !slices.Equal(lines, []string{"features/smoke.yaml: every smoke test is live (1 in all)"}) {
		t.Errorf("a sound set: %v %q %v", found, lines, err)
	}
	var asked []string
	found, lines, _ = judgedSmokeFindings(cfg, func(name, file string) bool {
		asked = append(asked, name+" "+file)
		return false
	})
	if len(found) != 0 || len(lines) != 0 || !slices.Equal(asked, []string{"scenario features/smoke.yaml"}) {
		t.Errorf("a kind left out: %v %q, asked %q", found, lines, asked)
	}
	writeFiles(t, map[string]string{"features/smoke.yaml": "- [\n"})
	found, _, _ = judgedSmokeFindings(cfg, nil)
	if len(found) != 1 || found[0].Rule != "smoke-unreadable" || found[0].Fix != "correct features/smoke.yaml" || found[0].Area != "smoke" {
		t.Errorf("a set that cannot be read: %v", found)
	}
}

// The hook's data rule as the index holds it: nothing outside a repository
// or where git cannot say where the index is but the error; no problem for a
// commit in a repository without a config; data that cannot be read one
// data-unreadable problem; and a staged feature file with no smoke scenario
// the smoke rule's problem, under the staged reject message (slice 117).
func TestStagedDataIssues(t *testing.T) {
	scratchFolder(t)
	if _, _, err := stagedDataIssues(""); err == nil {
		t.Error("outside a repository: no error")
	}

	gitIn(t, "init", "-q", "-b", "main")
	writeFiles(t, map[string]string{"README.md": "scratch\n"})
	gitIn(t, "add", "README.md")
	if found, _, err := stagedDataIssues(""); err != nil || len(found) != 0 {
		t.Errorf("no config: %v %v", found, err)
	}

	useWrappedGit(t, "rev-parse", "", "")
	if _, _, err := stagedDataIssues(""); err == nil {
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
	found, header, err := stagedDataIssues("")
	if err != nil || len(found) != 1 || found[0].Rule != "data-unreadable" || header != "Commit rejected:" {
		t.Errorf("a registry that cannot be read: %v %q %v", found, header, err)
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
	found, header, err = stagedDataIssues("")
	if err != nil || len(found) != 1 || found[0].Rule != "smoke-missing" || !strings.HasPrefix(found[0].Message, "b.feature ") || header != "Refused:" {
		t.Errorf("a staged feature file with no smoke scenario: %v %q %v", found, header, err)
	}
}
