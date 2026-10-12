// The plugin version rule's self-test (T-074's check, kept as a step in T-125, a Go test since
// T-143), in a scratch repository laid out as this one is, the plugin in integrations/claude-code/:
//
//   - a range that changes the plugin and leaves plugin.json's version as it was is refused,
//     naming the files changed, the version at the start and at the end, and the versions to
//     raise it to;
//   - one that raises it (a patch, a minor, a major, a pre-release's release) passes;
//   - one that lowers it, or raises only a pre-release below it, is refused;
//   - one that leaves the plugin alone passes, saying so, even when the version was not raised;
//   - a change undone within the range is no change;
//   - a shallow clone, no range start, or a start that is not a commit of the clone stops it
//     with exit 2, never a pass;
//
// and itos.yaml's ci runs it over the range, in its steps and in a prose-only range's (the
// plugin's skill is Markdown).
package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/donvargax/itos/v8/tools/selftest/scratch"
)

const manifest = "integrations/claude-code/.claude-plugin/plugin.json"

func TestSelftest(t *testing.T) {
	scratch.Gate(t)
	rel := scratch.NewRelease(t)
	repo := filepath.Join(rel.Tmp, "repo")
	bin := scratch.Build(t, "plugin-version")
	write := func(file, text string) { scratch.WriteFiles(t, repo, map[string]string{file: text}) }
	setVersion := func(version string) {
		data, _ := json.MarshalIndent(map[string]string{"name": "itos", "version": version}, "", "\t")
		write(manifest, string(data)+"\n")
	}
	commitAll := func(message string) string {
		rel.Git(repo, "add", "-A")
		rel.Git(repo, "commit", "-q", "--allow-empty", "-m", message)
		return rel.Git(repo, "rev-parse", "HEAD")
	}
	check := func(dir string, args ...string) scratch.Result { return scratch.Run(dir, rel.Env, "", bin, args...) }

	// The start: a plugin at 2.3.0, and a file outside it.
	rel.Git(rel.Tmp, "init", "-q", "-b", "main", repo)
	setVersion("2.3.0")
	write("integrations/claude-code/hooks/guard.sh", "echo guard\n")
	write("integrations/claude-code/skills/itos/SKILL.md", "# itos\n")
	write("README.md", "# readme\n")
	start := commitAll("chore: the plugin at 2.3.0")
	branch := func(name string) { rel.Git(repo, "checkout", "-q", "-b", name, start) }

	// The plugin changed, its version not: refused, naming the files, both versions and the
	// remedy.
	branch("unbumped")
	write("integrations/claude-code/hooks/guard.sh", "echo guard, changed\n")
	commitAll("chore: change the guard")
	write("integrations/claude-code/skills/itos/SKILL.md", "# itos, changed\n")
	commitAll("docs: change the skill")
	t.Run("a plugin change without a raised version is refused", func(t *testing.T) {
		r := check(repo, "-range-from", start)
		out := r.Output()
		for _, want := range []string{
			"integrations/claude-code/hooks/guard.sh", "integrations/claude-code/skills/itos/SKILL.md",
			"is still 2.3.0", "it was 2.3.0", manifest,
			"2.3.1 for a fix, 2.4.0 for a new behaviour, 3.0.0 for a breaking change",
		} {
			if r.Code != 1 || !strings.Contains(out, want) {
				t.Errorf("a plugin change without a bump should be refused (exit 1), saying %q, exited %d:\n%s", want, r.Code, out)
			}
		}
	})

	// A version set at from in a commit of its own on start, then the plugin changed and the
	// version set to to: the check over that.
	moved := func(from, to, verb string) scratch.Result {
		rel.Git(repo, "checkout", "-q", "--detach", start)
		setVersion(from)
		at := commitAll("chore: the plugin at " + from)
		write("integrations/claude-code/hooks/guard.sh", "echo guard for "+to+"\n")
		setVersion(to)
		commitAll("chore: " + verb + " the plugin to " + to)
		return check(repo, "-range-from", at)
	}

	// Raised: each passes, saying from what to what.
	for _, v := range [][2]string{
		{"2.3.0", "2.3.1"}, {"2.3.0", "2.4.0"}, {"2.3.0", "3.0.0"}, {"2.4.0-rc.1", "2.4.0"},
		{"2.4.0-rc.1", "2.4.0-rc.2"}, {"2.4.0-alpha", "2.4.0-beta"}, {"2.3.9", "2.3.10"},
	} {
		from, to := v[0], v[1]
		t.Run(from+" raised to "+to+" passes", func(t *testing.T) {
			r := moved(from, to, "raise")
			if r.Code != 0 || !strings.Contains(r.Output(), "went from "+from+" to "+to) {
				t.Errorf("a plugin change raising %s to %s should pass, exited %d:\n%s", from, to, r.Code, r.Output())
			}
		})
	}

	// Lowered, or equal by precedence: refused.
	for _, v := range [][3]string{
		{"2.3.0", "2.2.9", "went down to 2.2.9"},
		{"2.3.0", "2.3.0-rc.1", "went down to 2.3.0-rc.1"},
		{"2.4.0", "2.4.0+build.7", "is still 2.4.0+build.7"},
		{"2.3.10", "2.3.9", "went down to 2.3.9"},
	} {
		from, to, how := v[0], v[1], v[2]
		t.Run(from+" set to "+to+" is refused", func(t *testing.T) {
			r := moved(from, to, "set")
			if r.Code != 1 || !strings.Contains(r.Output(), how) || !strings.Contains(r.Output(), "it was "+from) {
				t.Errorf("a plugin change from %s to %s should be refused (exit 1), saying %q, exited %d:\n%s", from, to, how, r.Code, r.Output())
			}
		})
	}

	t.Run("a version that is not semver is refused", func(t *testing.T) {
		rel.Git(repo, "checkout", "-q", "--detach", start)
		setVersion("2.4")
		commitAll("chore: a version that is not semver")
		r := check(repo, "-range-from", start)
		if r.Code != 1 || !strings.Contains(r.Output(), "not semver") {
			t.Errorf("a version that is not semver should be refused (exit 1), exited %d:\n%s", r.Code, r.Output())
		}
	})

	// The plugin left alone passes, saying so; a change undone in the range is none.
	branch("alone")
	write("README.md", "# readme, changed\n")
	commitAll("docs: change the readme")
	t.Run("a range that leaves the plugin alone passes, saying so", func(t *testing.T) {
		r := check(repo, "-range-from", start)
		if r.Code != 0 || !strings.Contains(r.Output(), "nothing under integrations/claude-code/ changed") {
			t.Errorf("a range that leaves the plugin alone should pass, saying so, exited %d:\n%s", r.Code, r.Output())
		}
	})
	t.Run("a plugin change undone within the range passes", func(t *testing.T) {
		write("integrations/claude-code/hooks/guard.sh", "echo guard, for a while\n")
		commitAll("chore: change the guard")
		write("integrations/claude-code/hooks/guard.sh", "echo guard\n")
		commitAll("revert: change the guard back")
		r := check(repo, "-range-from", start)
		if r.Code != 0 || !strings.Contains(r.Output(), "nothing under integrations/claude-code/ changed") {
			t.Errorf("a plugin change undone within the range should pass, exited %d:\n%s", r.Code, r.Output())
		}
	})

	// No range start, a start not in the clone, and a shallow clone stop it.
	rel.Git(repo, "checkout", "-q", "unbumped")
	t.Run("no range start stops it", func(t *testing.T) {
		r := check(repo, "-range-from", "")
		if r.Code != 2 || !strings.Contains(r.Output(), "no range start") {
			t.Errorf("no range start should stop the check (exit 2), exited %d:\n%s", r.Code, r.Output())
		}
	})
	t.Run("a range start the clone lacks stops it", func(t *testing.T) {
		r := check(repo, "-range-from", "0123456789abcdef0123456789abcdef01234567")
		if r.Code != 2 || !strings.Contains(r.Output(), "is not a commit of this clone") {
			t.Errorf("a range start the clone lacks should stop the check (exit 2), exited %d:\n%s", r.Code, r.Output())
		}
	})
	t.Run("a shallow clone stops it", func(t *testing.T) {
		r := check(rel.Shallow(repo), "-range-from", start)
		if r.Code != 2 || !strings.Contains(r.Output(), "shallow") {
			t.Errorf("a shallow clone should stop the check (exit 2), exited %d:\n%s", r.Code, r.Output())
		}
	})

	// itos.yaml's ci runs it over the range, in its steps and its prose steps.
	const command = `go run ./tools/bin/plugin-version -range-from "${FROM-}"`
	data, err := os.ReadFile(filepath.Join(scratch.Root(t), "itos.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var policy struct {
		CI struct {
			Steps []yaml.Node `yaml:"steps"`
			Prose struct {
				Steps []string `yaml:"steps"`
			} `yaml:"prose"`
		} `yaml:"ci"`
	}
	if err := yaml.Unmarshal(data, &policy); err != nil {
		t.Fatal(err)
	}
	t.Run("itos.yaml's ci steps run it", func(t *testing.T) {
		found := false
		for _, step := range policy.CI.Steps {
			var s struct {
				Run string `yaml:"run"`
			}
			if step.Kind == yaml.MappingNode && step.Decode(&s) == nil && s.Run == command {
				found = true
			}
		}
		if !found {
			t.Errorf("itos.yaml's ci.steps should run %s", command)
		}
	})
	t.Run("itos.yaml's prose steps run it", func(t *testing.T) {
		if !slices.Contains(policy.CI.Prose.Steps, command) {
			t.Errorf("itos.yaml's ci.prose.steps should run %s: the plugin's skill is Markdown", command)
		}
	})
}
