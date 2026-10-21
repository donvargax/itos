//go:build !windows

package corpus

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	itosgit "github.com/donvargax/itos/v8/internal/git"
)

// The runner's regressions, run.test.ts's until T-144: a case's processes, its native Git and its deadline, at RunCase's seam.
// The command under test is sh, so not on windows, where this file is not built.

// fixtureCase is the one case of a fixture's YAML text.
func fixtureCase(t *testing.T, text string) *Case {
	t.Helper()
	path := filepath.Join(t.TempDir(), "case.yaml")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := ReadFixture(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Cases) != 1 {
		t.Fatalf("%d cases, want 1", len(f.Cases))
	}
	return f.Cases[0]
}

// nativeGit is the git the tests run, internal/git's Real.
func nativeGit(t *testing.T) string {
	t.Helper()
	git, err := itosgit.Real()
	if err != nil {
		t.Skip(err)
	}
	return git
}

// newRunner is a runner of sh, its cases' folders made in root.
func newRunner(t *testing.T, git, root string, deadline time.Duration) *Runner {
	t.Helper()
	r, err := New(Options{Bin: "sh", Git: git, CaseDeadline: deadline, TempRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	return r
}

// entries are the names in a folder.
func entries(t *testing.T, dir string) []string {
	t.Helper()
	list, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, e := range list {
		names = append(names, e.Name())
	}
	return names
}

// running is whether a process is alive, a zombie not counted.
func running(pid int) bool {
	if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
		return false
	}
	out, err := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	state := strings.TrimSpace(string(out))
	return err == nil && state != "" && !strings.HasPrefix(state, "Z")
}

func TestRegressions(t *testing.T) {
	t.Run("a PATH-first Git shim remains fake for the command while config inspection uses native Git", func(t *testing.T) {
		git := nativeGit(t)
		root := t.TempDir()
		c := fixtureCase(t, `cases:
  - name: native inspection does not inherit fake Git
    argv: [-c, git --version]
    git: []
    files:
      old-git: { text: "#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then echo \"git version 2.30.0\"; exit 0; fi\nexec \"{{git}}\" \"$@\"\n", executable: true }
      shim-bin/git: { text: "#!/bin/sh\nexec \"$ITOS_GIT\" \"$@\"\n", executable: true }
    env:
      ITOS_GIT: "{{dir}}/old-git"
      PATH: "{{dir}}/shim-bin{{PATH_SEP}}{{PATH}}"
    git_config_after:
      hook.itos-commit-msg.event: commit-msg
      hook.itos-pre-push.event: pre-push
    exit: 0
`)
		out, _, err := newRunner(t, git, root, 10*time.Second).RunCase(c)
		if err != nil {
			t.Fatal(err)
		}
		if out.Exit == nil || *out.Exit != 0 || out.Stdout != "git version 2.30.0\n" {
			t.Errorf("the command under test must still reach the fixture's Git: exit %v, stdout %q, stderr %q", out.Exit, out.Stdout, out.Stderr)
		}
		for key, want := range map[string]string{"hook.itos-commit-msg.event": "commit-msg", "hook.itos-pre-push.event": "pre-push"} {
			if got := out.gitConfig[key]; got == nil || *got != want {
				t.Errorf("native inspection read %s as %v, want %q", key, got, want)
			}
		}
		if left := entries(t, root); len(left) != 0 {
			t.Errorf("RunCase left its scratch folder behind: %v", left)
		}
	})

	t.Run("an old fixture's bare Git fallback reaches native Git without replacing its fake version", func(t *testing.T) {
		git := nativeGit(t)
		root := t.TempDir()
		c := fixtureCase(t, `cases:
  - name: legacy old-Git fallback
    argv: [-c, '"$ITOS_GIT" --version; "$ITOS_GIT" config --get-all hook.itos-commit-msg.event']
    git: []
    files:
      old-git: { text: "#!/bin/sh\nif [ \"$1\" = --version ]; then echo \"git version 2.30.0\"; exit 0; fi\nexec git \"$@\"\n", executable: true }
    env: { ITOS_GIT: "{{dir}}/old-git" }
    git_config_after: { hook.itos-commit-msg.event: commit-msg }
    exit: 0
`)
		out, _, err := newRunner(t, git, root, 10*time.Second).RunCase(c)
		if err != nil {
			t.Fatal(err)
		}
		if out.Exit == nil || *out.Exit != 0 || out.Stdout != "git version 2.30.0\ncommit-msg\n" {
			t.Errorf("the fixture's bare git should reach native Git: exit %v, stdout %q, stderr %q", out.Exit, out.Stdout, out.Stderr)
		}
		if got := out.gitConfig["hook.itos-commit-msg.event"]; got == nil || *got != "commit-msg" {
			t.Errorf("native inspection read %v", got)
		}
		if left := entries(t, root); len(left) != 0 {
			t.Errorf("RunCase left its scratch folder behind: %v", left)
		}
	})

	t.Run("a hanging git_config_after inspection is bounded by the case deadline", func(t *testing.T) {
		git := nativeGit(t)
		root := t.TempDir()
		started := filepath.Join(root, "inspection-started")
		hanging := filepath.Join(t.TempDir(), "git")
		script := "#!/bin/sh\nif [ \"$1\" = config ] && [ \"$2\" = --get-all ]; then echo started > '" + started + "'; exec sleep 600; fi\nexec '" + git + "' \"$@\"\n"
		if err := os.WriteFile(hanging, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		c := fixtureCase(t, `cases:
  - name: hung post-check
    argv: [-c, exit 0]
    files:
      .git/HEAD: "ref: refs/heads/main\n"
      .git/config: "[core]\n\trepositoryformatversion = 0\n\tbare = false\n"
    git_config_after: { hook.itos-commit-msg.event: commit-msg }
    exit: 0
`)
		const deadline = 2 * time.Second
		began := time.Now()
		_, _, err := newRunner(t, hanging, root, deadline).RunCase(c)
		var expired *DeadlineError
		if !errors.As(err, &expired) {
			t.Fatalf("RunCase said %v, want the case's deadline", err)
		}
		if took := time.Since(began); took > deadline+20*time.Second {
			t.Errorf("the hanging inspection held the case %v past its %v deadline", took, deadline)
		}
		if text, err := os.ReadFile(started); err != nil || string(text) != "started\n" {
			t.Errorf("the deadline must reach the inspection after the command: %q, %v", text, err)
		}
		if left := entries(t, root); !slices.Equal(left, []string{"inspection-started"}) {
			t.Errorf("RunCase left its scratch folder behind: %v", left)
		}
	})

	t.Run("a command descendant holding output open is killed with its case process group", func(t *testing.T) {
		git := nativeGit(t)
		root := t.TempDir()
		pidFile := filepath.Join(root, "descendant.pid")
		c := fixtureCase(t, `cases:
  - name: descendant holds output open
    argv: [-c, 'sleep 600 & echo $! > "$PID_FILE"; sleep 600']
    env: { PID_FILE: "`+pidFile+`" }
    exit: 0
`)
		const deadline = 2 * time.Second
		began := time.Now()
		_, _, err := newRunner(t, git, root, deadline).RunCase(c)
		var expired *DeadlineError
		if !errors.As(err, &expired) {
			t.Fatalf("RunCase said %v, want the case's deadline", err)
		}
		if took := time.Since(began); took > deadline+20*time.Second {
			t.Errorf("the descendant held the case %v past its %v deadline", took, deadline)
		}
		text, err := os.ReadFile(pidFile)
		pid, _ := strconv.Atoi(strings.TrimSpace(string(text)))
		if err != nil || pid <= 0 {
			t.Fatalf("the command recorded no descendant: %q, %v", text, err)
		}
		gone := time.Now().Add(20 * time.Second)
		for running(pid) && time.Now().Before(gone) {
			time.Sleep(50 * time.Millisecond)
		}
		if running(pid) {
			t.Errorf("the descendant %d is still running after the case", pid)
		}
		if left := entries(t, root); !slices.Equal(left, []string{"descendant.pid"}) {
			t.Errorf("RunCase left its scratch folder behind: %v", left)
		}
	})
}

// A command that exits and leaves a descendant holding its output is done when it exits: its
// group is killed, so the case neither waits for the descendant nor fails.
func TestExitedCommandsGroupIsKilled(t *testing.T) {
	git := nativeGit(t)
	c := fixtureCase(t, `cases:
  - name: a descendant outlives the command
    argv: [-c, 'sleep 600 & echo done']
    exit: 0
    stdout_has: "done\n"
`)
	began := time.Now()
	out, s, err := newRunner(t, git, t.TempDir(), time.Minute).RunCase(c)
	if err != nil {
		t.Fatal(err)
	}
	if problems := compare(c, out, s, false); len(problems) > 0 {
		t.Errorf("the case should pass: %q", problems)
	}
	if took := time.Since(began); took > 30*time.Second {
		t.Errorf("the case waited %v for a descendant of the command", took)
	}
}
