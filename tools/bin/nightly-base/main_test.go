package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// now is the fake clock: 2026-10-12 11:44 UTC, the nightly's cron time.
var now = time.Date(2026, 10, 12, 11, 44, 0, 0, time.UTC)

// hoursAgo is a commit time h hours before now, as git log's %ct writes it.
func hoursAgo(h int) string { return fmt.Sprint(now.Add(-time.Duration(h) * time.Hour).Unix()) }

// fake answers gh run list with gh (or fails it with ghErr), the ancestry check of a commit and
// another ("a b") with ancestry when it names the pair, else with ancestor, and git log with
// history, logging every call.
type fake struct {
	gh       string
	ghErr    error
	ancestor bool
	ancestry map[string]bool
	history  string
	calls    []string
}

func (f *fake) run(name string, args ...string) (string, error) {
	call := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, call)
	switch {
	case strings.HasPrefix(call, "gh run list "):
		return f.gh, f.ghErr
	case strings.HasPrefix(call, "git merge-base --is-ancestor "):
		is, named := f.ancestry[strings.TrimPrefix(call, "git merge-base --is-ancestor ")]
		if !named {
			is = f.ancestor
		}
		if is {
			return "", nil
		}
		return "", errors.New("git: exit status 1")
	case call == "git log --first-parent --format=%H %ct HEAD":
		return f.history, nil
	}
	return "", fmt.Errorf("unexpected call %s", call)
}

// history is HEAD's first-parent line, newest first: c3 an hour ago, c2 23 hours ago, c1 30
// hours ago, c0 50.
var history = strings.Join([]string{
	"c3 " + hoursAgo(1), "c2 " + hoursAgo(23), "c1 " + hoursAgo(30), "c0 " + hoursAgo(50),
}, "\n") + "\n"

func TestTheBaseIsTheLastGreenNightlysHeadCommit(t *testing.T) {
	f := &fake{gh: `[{"databaseId":37772629517,"headSha":"9397855d","createdAt":"2026-10-08T11:49:36Z"}]`, ancestor: true, history: history}
	base, note, err := find(f.run, now, "nightly.yml", "main", "")
	if err != nil || base != "9397855d" {
		t.Fatalf("base %q, err %v", base, err)
	}
	if !strings.Contains(note, "last green run of nightly.yml on main (run 37772629517, 2026-10-08T11:49:36Z)") {
		t.Errorf("the note does not name the run: %s", note)
	}
	want := "gh run list --workflow nightly.yml --branch main --status success --limit 1 --json databaseId,headSha,createdAt"
	if f.calls[0] != want {
		t.Errorf("gh was asked %q, not %q", f.calls[0], want)
	}
	if f.calls[1] != "git merge-base --is-ancestor 9397855d HEAD" {
		t.Errorf("the base's reach was not checked: %v", f.calls)
	}
}

func TestWithNoGreenNightlyInReachTheBaseIsADayAgo(t *testing.T) {
	cases := []struct {
		name, gh, why string
		ghErr         error
		ancestor      bool
	}{
		{name: "gh failing", ghErr: errors.New("gh: exit status 1: HTTP 403: Resource not accessible\nmore"), why: "gh run list failed: gh: exit status 1: HTTP 403: Resource not accessible"},
		{name: "no green run", gh: "[]\n", why: "no run of nightly.yml on main concluded success"},
		{name: "not JSON", gh: "oops\n", why: "gh run list said no JSON: oops"},
		{name: "a head commit out of reach", gh: `[{"databaseId":1,"headSha":"abc","createdAt":"x"}]`, why: "the last green run's head commit abc is not in HEAD's history"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &fake{gh: c.gh, ghErr: c.ghErr, ancestor: c.ancestor, history: history}
			base, note, err := find(f.run, now, "nightly.yml", "main", "")
			if err != nil || base != "c1" {
				t.Fatalf("base %q, err %v; want c1, the newest commit 24 hours old", base, err)
			}
			want := "no green nightly in reach (" + c.why + "); judging the last 24 hours' first-parent range instead, since c1"
			if note != want {
				t.Errorf("note\n%s\nwant\n%s", note, want)
			}
		})
	}
}

// green is gh's answer naming the last green nightly of 2026-10-08, at 9397855d.
const green = `[{"databaseId":37772629517,"headSha":"9397855d","createdAt":"2026-10-08T11:49:36Z"}]`

func TestWithAFloorTheBaseIsTheLaterOfItAndTheLastGreenNightly(t *testing.T) {
	const head = "the head commit of the last green run of nightly.yml on main (run 37772629517, 2026-10-08T11:49:36Z)"
	cases := []struct {
		name, gh, base, note string
		greenBelowFloor      bool
	}{
		{name: "a green head the floor descends from", gh: green, greenBelowFloor: true, base: "a9de53f6",
			note: "the base is the floor a9de53f6, as " + head + ", 9397855d, is the floor or one of its ancestors"},
		{name: "a green head later than the floor", gh: green, base: "9397855d",
			note: "the base is 9397855d, " + head + ", later than the floor a9de53f6"},
		{name: "no green run", gh: "[]", base: "a9de53f6",
			note: "no green nightly in reach (no run of nightly.yml on main concluded success); the base is the floor a9de53f6"},
		{name: "a green head out of reach", gh: `[{"databaseId":1,"headSha":"abc","createdAt":"x"}]`, base: "a9de53f6",
			note: "no green nightly in reach (the last green run's head commit abc is not in HEAD's history); the base is the floor a9de53f6"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &fake{gh: c.gh, ancestry: map[string]bool{"9397855d HEAD": true, "9397855d a9de53f6": c.greenBelowFloor}, history: history}
			base, note, err := find(f.run, now, "nightly.yml", "main", "a9de53f6")
			if err != nil || base != c.base {
				t.Fatalf("base %q, err %v; want %s", base, err, c.base)
			}
			if note != c.note {
				t.Errorf("note\n%s\nwant\n%s", note, c.note)
			}
			for _, call := range f.calls {
				if strings.HasPrefix(call, "git log ") {
					t.Errorf("a floor still read the last 24 hours: %v", f.calls)
				}
			}
		})
	}
	f := &fake{ghErr: errors.New("gh: exit status 1: HTTP 403"), history: history}
	if base, note, err := find(f.run, now, "nightly.yml", "main", "a9de53f6"); err != nil || base != "a9de53f6" ||
		note != "no green nightly in reach (gh run list failed: gh: exit status 1: HTTP 403); the base is the floor a9de53f6" {
		t.Errorf("gh failing: base %q, note %q, err %v", base, note, err)
	}
}

func TestRunHoldsTheFloorToHEADsHistory(t *testing.T) {
	var stdout, stderr strings.Builder
	f := &fake{gh: green, ancestry: map[string]bool{"a9de53f6 HEAD": true, "9397855d HEAD": true, "9397855d a9de53f6": true}, history: history}
	if code := run([]string{"-floor", "a9de53f6"}, f.run, now, &stdout, &stderr); code != 0 || stdout.String() != "a9de53f6\n" ||
		!strings.HasPrefix(stderr.String(), "nightly-base: the base is the floor a9de53f6, as ") {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	if f.calls[0] != "git merge-base --is-ancestor a9de53f6 HEAD" {
		t.Errorf("the floor was not held to HEAD's history first: %v", f.calls)
	}
	stdout.Reset()
	stderr.Reset()
	f = &fake{gh: green, ancestry: map[string]bool{"elsewhere HEAD": false}, ancestor: true, history: history}
	if code := run([]string{"-floor", "elsewhere"}, f.run, now, &stdout, &stderr); code != 2 || stdout.Len() != 0 ||
		stderr.String() != "nightly-base: the floor elsewhere is not in HEAD's history: git: exit status 1\n" {
		t.Errorf("a floor out of reach: exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	if len(f.calls) != 1 {
		t.Errorf("a floor out of reach still read gh or git: %v", f.calls)
	}
}

func TestADayAgoOnAQuietOrAYoungLine(t *testing.T) {
	quiet := "c1 " + hoursAgo(30) + "\nc0 " + hoursAgo(50) + "\n"
	young := "c1 " + hoursAgo(2) + "\nc0 " + hoursAgo(5) + "\n"
	exact := "c1 " + hoursAgo(2) + "\nc0 " + hoursAgo(24) + "\n"
	for _, c := range []struct{ name, history, want string }{
		{"nothing committed in a day: HEAD, an empty range", quiet, "c1"},
		{"every commit younger than a day: the oldest", young, "c0"},
		{"a commit exactly a day old is the base", exact, "c0"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := dayAgo((&fake{history: c.history}).run, now)
			if err != nil || got != c.want {
				t.Errorf("got %q, err %v; want %q", got, err, c.want)
			}
		})
	}
}

func TestAHistoryGitCannotReadStopsIt(t *testing.T) {
	broken := func(name string, args ...string) (string, error) {
		if name == "gh" {
			return "[]", nil
		}
		return "", errors.New("git: exit status 128: fatal: not a git repository")
	}
	if _, _, err := find(broken, now, "nightly.yml", "main", ""); err == nil || !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("err %v", err)
	}
	for _, history := range []string{"", "c1 yesterday\n", "c1\n"} {
		if got, err := dayAgo((&fake{history: history}).run, now); err == nil {
			t.Errorf("history %q gave %q, no error", history, got)
		}
	}
}

func TestRunPrintsTheBaseAloneOnStdout(t *testing.T) {
	f := &fake{gh: "[]", history: history}
	var stdout, stderr strings.Builder
	if code := run(nil, f.run, now, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if stdout.String() != "c1\n" {
		t.Errorf("stdout %q", stdout.String())
	}
	if !strings.HasPrefix(stderr.String(), "nightly-base: no green nightly in reach (") {
		t.Errorf("stderr %q", stderr.String())
	}
	f = &fake{gh: "[]", history: history}
	if code := run([]string{"-workflow", "other.yml", "-branch", "dev"}, f.run, now, &stdout, &stderr); code != 0 ||
		f.calls[0] != "gh run list --workflow other.yml --branch dev --status success --limit 1 --json databaseId,headSha,createdAt" {
		t.Errorf("the flags were not passed to gh: exit %d, %v", code, f.calls)
	}
	for _, args := range [][]string{{"extra"}, {"-nope"}} {
		stdout.Reset()
		if code := run(args, f.run, now, &stdout, &stderr); code != 2 || stdout.Len() != 0 {
			t.Errorf("%v: exit %d, stdout %q", args, code, stdout.String())
		}
	}
	broken := func(string, ...string) (string, error) { return "", errors.New("boom") }
	stdout.Reset()
	if code := run(nil, broken, now, &stdout, &stderr); code != 2 || stdout.Len() != 0 {
		t.Errorf("a broken git: exit %d, stdout %q", code, stdout.String())
	}
}

func TestExecRunNamesStderr(t *testing.T) {
	if _, err := execRun("go", "nightly-base-no-such-command"); err == nil ||
		!strings.Contains(err.Error(), "go: exit status 2: go nightly-base-no-such-command: unknown command") {
		t.Errorf("err %v", err)
	}
	if _, err := execRun("nightly-base-no-such-program"); err == nil || !strings.HasPrefix(err.Error(), "nightly-base-no-such-program: exec: ") {
		t.Errorf("err %v", err)
	}
	if out, err := execRun("go", "env", "GOOS"); err != nil || strings.TrimSpace(out) == "" {
		t.Errorf("out %q, err %v", out, err)
	}
	if firstLine("\n \n") != "(nothing)" {
		t.Error("an empty text's first line")
	}
}

// writeTree writes files, relative paths to their text, under a new folder, and returns it.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	top := t.TempDir()
	for name, text := range files {
		path := filepath.Join(top, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return top
}

func TestLinuxFilesAreTheGoFilesALinuxBuildCompiles(t *testing.T) {
	top := writeTree(t, map[string]string{
		"internal/a/plain.go":            "package a\n",
		"internal/a/plain_test.go":       "package a\n",
		"internal/a/file_windows.go":     "package a\n",
		"internal/a/file_darwin.go":      "package a\n",
		"internal/a/file_linux.go":       "package a\n",
		"internal/a/tagged.go":           "//go:build windows\n\npackage a\n",
		"internal/a/unix.go":             "//go:build unix\n\npackage a\n",
		"internal/a/notes.md":            "not Go\n",
		"internal/a/testdata/fixture.go": "package fixture\n",
		"internal/a/_skip/old.go":        "package old\n",
		"internal/nested/go.mod":         "module nested\n",
		"internal/nested/n.go":           "package nested\n",
		"cmd/tool/main.go":               "package main\n",
		"other/ignored.go":               "package other\n",
	})
	got, err := linuxFiles(top, []string{"cmd", "internal"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"cmd/tool/main.go", "internal/a/file_linux.go", "internal/a/plain.go", "internal/a/unix.go"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestLinuxFilesRefusesToPrintNone(t *testing.T) {
	top := writeTree(t, map[string]string{"internal/a/only_windows.go": "package a\n"})
	if files, err := linuxFiles(top, []string{"internal"}); err == nil || !strings.Contains(err.Error(), "no Go file a Linux build compiles under internal") {
		t.Errorf("files %v, err %v", files, err)
	}
	if _, err := linuxFiles(top, nil); err == nil {
		t.Error("no folders gave no error")
	}
	if _, err := linuxFiles(top, []string{"missing"}); err == nil {
		t.Error("a missing folder gave no error")
	}
	bad := writeTree(t, map[string]string{"internal/a/bad.go": "//go:build (\n\npackage a\n"})
	if _, err := linuxFiles(bad, []string{"internal"}); err == nil {
		t.Error("an unreadable build constraint gave no error")
	}
}

func TestRunPrintsTheLinuxFilesOneALine(t *testing.T) {
	top := writeTree(t, map[string]string{"internal/a/a.go": "package a\n", "internal/a/b_windows.go": "package a\n", "cmd/c/c.go": "package main\n"})
	t.Chdir(top)
	var stdout, stderr strings.Builder
	if code := run([]string{"-linux-files", "cmd", "internal"}, nil, now, &stdout, &stderr); code != 0 || stdout.String() != "cmd/c/c.go\ninternal/a/a.go\n" {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	if code := run([]string{"-linux-files", "nowhere"}, nil, now, &stdout, &stderr); code != 2 || stdout.Len() != 0 {
		t.Errorf("a missing folder: exit %d, stdout %q", code, stdout.String())
	}
}
