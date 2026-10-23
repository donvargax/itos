package tests

import (
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// The live tests are rows in the listing's order, each file joined to the
// kind's root and read from the project's top, a fingerprint the third
// column when the test has one; a test that is not live is left out.
func TestTSVProjectsTheLiveTests(t *testing.T) {
	project := t.TempDir()
	list := List{Tests: []Test{
		{ID: "B-01", File: "nested/b.feature", Live: true, Fingerprint: "f1"},
		{ID: "A-01", File: "a.feature", Live: false},
		{ID: "A-02", File: "./a.feature", Live: true},
		{ID: "C-01", File: filepath.Join(project, "specs", "c.feature"), Live: true},
	}}
	rows, refused := TSV(list, "specs with spaces/acceptance", project)
	if refused != nil {
		t.Fatalf("TSV refused %q", refused)
	}
	var lines []string
	for _, r := range rows {
		lines = append(lines, r.Line())
	}
	want := []string{
		"B-01\tspecs with spaces/acceptance/nested/b.feature\tf1\n",
		"A-02\tspecs with spaces/acceptance/a.feature\n",
		"C-01\tspecs/c.feature\n",
	}
	if !slices.Equal(lines, want) {
		t.Errorf("TSV = %q, want %q", lines, want)
	}
	rows, _ = TSV(List{Tests: []Test{{ID: "F-1", File: "tests/f.txt", Live: true}}}, "", project)
	if len(rows) != 1 || rows[0].Line() != "F-1\ttests/f.txt\n" {
		t.Errorf("TSV with no root = %+v, want the file as listed", rows)
	}
	if rows, refused := TSV(List{Tests: []Test{{ID: "W", File: "w.txt"}}}, "", project); rows != nil || refused != nil {
		t.Errorf("TSV of no live test = %+v, %q, want nothing", rows, refused)
	}
}

// A live test no row can hold refuses every row, each such test named by its
// place and why.
func TestTSVRefusesWhatNoRowCanHold(t *testing.T) {
	project := t.TempDir()
	valid := Test{ID: "F-1", File: "one.txt", Live: true}
	for _, c := range []struct {
		root string
		bad  Test
		why  string
	}{
		{"", Test{ID: "F\t2", File: "two.txt"}, "its id holds a tab"},
		{"", Test{ID: "F-2", File: "t\rwo.txt"}, "its file holds a carriage return"},
		{"", Test{ID: "F-2", File: "t\nwo.txt"}, "its file holds a newline"},
		{"", Test{ID: "F\x002", File: "two.txt"}, "its id holds a NUL"},
		{"", Test{ID: "", File: "two.txt"}, "its id is empty"},
		{"", Test{ID: "F-2", File: ""}, "its file is empty"},
		{"", Test{ID: "F-1", File: "again.txt"}, "it has the same id as test 1"},
		{"", Test{ID: "F-2", File: "../two.txt"}, "its file is outside the project"},
		{"", Test{ID: "F-2", File: "sub/.."}, "its file is outside the project"},
		{"sub", Test{ID: "F-2", File: "../../two.txt"}, "its file is outside the project"},
		{"", Test{ID: "F-2", File: filepath.Join(filepath.Dir(project), "elsewhere", "two.txt")}, "its file is outside the project"},
	} {
		c.bad.Live = true
		rows, refused := TSV(List{Tests: []Test{valid, {ID: "W", File: "w.txt"}, c.bad}}, c.root, project)
		if rows != nil || len(refused) != 1 || !strings.HasPrefix(refused[0], "test 3 (id ") || !strings.HasSuffix(refused[0], ": "+c.why) {
			t.Errorf("TSV with %+v = %+v, %q, want test 3 refused: %s", c.bad, rows, refused, c.why)
		}
	}
	_, refused := TSV(List{Tests: []Test{{ID: "", File: "", Live: true}, {ID: "x", File: "/abs", Live: true}}}, "", "")
	if len(refused) != 2 || !strings.HasSuffix(refused[0], ": its id is empty; its file is empty") ||
		!strings.HasSuffix(refused[1], ": its file is outside the project") {
		t.Errorf("TSV with two bad tests and no project = %q, want each refused with every reason", refused)
	}
}

// On Windows a root-relative \foo has no volume and a drive-relative C:foo
// is not absolute, so neither is made relative to the project: the leading
// slash refuses the one and the volume the other. Elsewhere both are names
// in the project's top.
func TestProjectFileRefusesWindowsRootAndDriveRelativePaths(t *testing.T) {
	project := t.TempDir()
	for _, c := range []struct {
		file               string
		windows, elsewhere [2]string
	}{
		{`\foo`, [2]string{"", "its file is outside the project"}, [2]string{`\foo`, ""}},
		{`C:foo`, [2]string{"", "its file is outside the project"}, [2]string{`C:foo`, ""}},
	} {
		want := c.elsewhere
		if runtime.GOOS == "windows" {
			want = c.windows
		}
		if file, why := projectFile("", c.file, project); file != want[0] || why != want[1] {
			t.Errorf("projectFile(%q) on %s = %q, %q, want %q, %q", c.file, runtime.GOOS, file, why, want[0], want[1])
		}
	}
}
