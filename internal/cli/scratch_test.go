package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/git"
)

// Git may detach maintenance after a commit, fetch or merge and keep writing
// .git during TempDir cleanup. Append command config so every child Git stays
// synchronous, including when inherited config would enable automatic work.
func disableScratchMaintenance(t *testing.T) {
	t.Helper()
	n := 0
	if count := os.Getenv("GIT_CONFIG_COUNT"); count != "" {
		var err error
		n, err = strconv.Atoi(count)
		if err != nil || n < 0 {
			t.Fatalf("invalid inherited GIT_CONFIG_COUNT %q", count)
		}
	}
	for i, setting := range []struct{ key, value string }{
		{"maintenance.auto", "false"}, {"gc.auto", "0"},
	} {
		suffix := strconv.Itoa(n + i)
		t.Setenv("GIT_CONFIG_KEY_"+suffix, setting.key)
		t.Setenv("GIT_CONFIG_VALUE_"+suffix, setting.value)
	}
	t.Setenv("GIT_CONFIG_COUNT", strconv.Itoa(n+2))
}

func TestScratchReposDisableAutoMaintenance(t *testing.T) {
	fixtures := []struct {
		name  string
		setup func(*testing.T)
	}{
		{"gitConfigRepo", func(t *testing.T) { gitConfigRepo(t, "version: 1\n") }},
		{"notesRepo", func(t *testing.T) { notesRepo(t) }},
		{"initScratch", func(t *testing.T) { initScratch(t, "commit", nil) }},
		{"scratchFolder", func(t *testing.T) { scratchFolder(t); gitIn(t, "init", "-q") }},
	}
	for _, fixture := range fixtures {
		for _, inherited := range []bool{false, true} {
			mode := "defaults"
			if inherited {
				mode = "inherited config"
			}
			t.Run(fixture.name+"/"+mode, func(t *testing.T) {
				t.Setenv("GIT_CONFIG_COUNT", "0")
				t.Setenv("GIT_CONFIG_PARAMETERS", "")
				if inherited {
					t.Setenv("GIT_CONFIG_COUNT", "3")
					for i, setting := range []struct{ key, value string }{
						{"maintenance.auto", "true"}, {"gc.auto", "1"}, {"user.name", "inherited author"},
					} {
						suffix := strconv.Itoa(i)
						t.Setenv("GIT_CONFIG_KEY_"+suffix, setting.key)
						t.Setenv("GIT_CONFIG_VALUE_"+suffix, setting.value)
					}
				}
				fixture.setup(t)
				if inherited {
					// Even a user config restored after setup cannot re-enable background work.
					global := filepath.Join(t.TempDir(), "gitconfig")
					if err := os.WriteFile(global, []byte("[maintenance]\n auto = true\n[gc]\n auto = 1\n"), 0o644); err != nil {
						t.Fatal(err)
					}
					t.Setenv("GIT_CONFIG_GLOBAL", global)
				}
				for _, setting := range []struct{ kind, key, want string }{
					{"--bool", "maintenance.auto", "false"}, {"--int", "gc.auto", "0"},
				} {
					out, err := exec.Command(git.Bin(), "config", setting.kind, "--get", setting.key).CombinedOutput()
					if err != nil || strings.TrimSpace(string(out)) != setting.want {
						t.Errorf("effective %s = %q (%v), want %s", setting.key, out, err, setting.want)
					}
				}
				if inherited {
					if got := gitIn(t, "config", "--get", "user.name"); got != "inherited author\n" {
						t.Errorf("unrelated inherited config changed: %q", got)
					}
				}
			})
		}
	}
}
