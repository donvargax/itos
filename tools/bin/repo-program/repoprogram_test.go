package repoprogram

import (
	"slices"
	"testing"
)

// TestCommand holds commandOn to what tools/bin/repo-program.ts gave on each platform, case by
// case (tools/parity/tools-bin/repo-program.json).
func TestCommand(t *testing.T) {
	for _, c := range []struct {
		name, goos, script string
		args               []string
		program            string
		argv               []string
	}{
		{"a script runs as itself on linux, its arguments kept", "linux",
			"tools/bin/itos", []string{"tests", "list"}, "tools/bin/itos", []string{"tests", "list"}},
		{"a script runs as itself on darwin", "darwin",
			"/repo/tools/bin/dev-version", []string{"/repo"}, "/repo/tools/bin/dev-version", []string{"/repo"}},
		{"a script with no arguments runs as itself", "linux",
			"tools/bin/dev-version", nil, "tools/bin/dev-version", nil},
		{"a script runs through sh on windows, its arguments after it", "windows",
			"tools/bin/itos", []string{"--version"}, "sh", []string{"tools/bin/itos", "--version"}},
		{"a windows path's backslashes are slashes for sh, its arguments untouched", "windows",
			`C:\repo\tools\bin\pinned`, []string{"goreleaser", `a\b`}, "sh", []string{"C:/repo/tools/bin/pinned", "goreleaser", `a\b`}},
		{"a script with no arguments runs through sh on windows", "windows",
			"tools/bin/dev-version", nil, "sh", []string{"tools/bin/dev-version"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			program, argv := commandOn(c.goos, c.script, c.args)
			if program != c.program || !slices.Equal(argv, c.argv) {
				t.Errorf("commandOn(%q, %q, %q) = %q, %q; want %q, %q", c.goos, c.script, c.args, program, argv, c.program, c.argv)
			}
		})
	}
}
