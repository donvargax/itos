//go:build unix

package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// enter moves the process to a folder and back, leaves it where it stands
// for none, and refuses what is no folder, the process left where it stood.
func TestEnter(t *testing.T) {
	here := t.TempDir()
	t.Chdir(here)
	stays := func(what string) {
		t.Helper()
		if now, _ := os.Getwd(); !sameDir(now, here) {
			t.Errorf("%s: the process stands in %s", what, now)
		}
	}
	back, err := enter("")
	if err != nil || back() != nil {
		t.Errorf("no folder: %v", err)
	}
	stays("no folder")
	there := t.TempDir()
	if back, err = enter(there); err != nil {
		t.Fatal(err)
	}
	if now, _ := os.Getwd(); !sameDir(now, there) {
		t.Errorf("entered %s, standing in %s", there, now)
	}
	if err := back(); err != nil {
		t.Error(err)
	}
	stays("back")
	file := filepath.Join(there, "file")
	writeFiles(t, map[string]string{file: "a file\n"})
	for _, dir := range []string{filepath.Join(there, "gone"), file} {
		if _, err := enter(dir); err == nil {
			t.Errorf("entered %s", dir)
		}
		stays(dir)
	}
}

// Where the folder the process stands in cannot be opened, there is
// nothing to come back to, and enter refuses.
func TestEnterFromAFolderNotToOpen(t *testing.T) {
	shut := filepath.Join(t.TempDir(), "shut")
	if err := os.Mkdir(shut, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(shut)
	if err := os.Chmod(shut, 0o111); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(shut, 0o755) })
	if f, err := os.Open("."); err == nil {
		f.Close()
		t.Skip("this system opens a folder no one may read")
	}
	if _, err := enter(t.TempDir()); err == nil {
		t.Error("entered a folder with none to come back to")
	}
}

// A program that cannot start is an error, the process back where it
// stood.
func TestRunProgramThatCannotStart(t *testing.T) {
	here := t.TempDir()
	t.Chdir(here)
	program := filepath.Join(t.TempDir(), "itos-nothing")
	writeFiles(t, map[string]string{program: "not a program\n"})
	if _, err := runProgram(program, nil, nil, Out{Dir: t.TempDir()}); err == nil {
		t.Error("a file that is no program started")
	}
	if now, _ := os.Getwd(); !sameDir(now, here) {
		t.Errorf("the process stands in %s", now)
	}
	if _, err := runProgram(program, nil, nil, Out{Dir: filepath.Join(here, "gone")}); err == nil {
		t.Error("a program started in a folder that is not there")
	}
}
