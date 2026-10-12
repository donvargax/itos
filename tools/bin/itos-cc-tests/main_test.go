package main

import (
	"strings"
	"testing"
)

func TestList(t *testing.T) {
	for _, c := range []struct {
		name, report, stdout, stderr string
		code                         int
	}{
		{"the live tests, a line each, the @wip ones left out",
			`{"tests": [{"id": "ID-A-01", "file": "a.feature", "live": true}, {"id": "ID-A-02", "file": "a.feature", "live": false}, {"id": "bug-3", "file": "b/c.feature", "live": true}]}`,
			"ID-A-01\tfeatures/a.feature\nbug-3\tfeatures/b/c.feature\n", "", 0},
		{"no test at all prints nothing", `{"tests": []}`, "", "", 0},
		{"output that is not JSON", `itos: no`, "", "returned invalid JSON: ", 1},
		{"JSON that is not an object", `[1]`, "", "returned no tests list", 1},
		{"no tests list", `{"test": []}`, "", "returned no tests list", 1},
		{"a tests list that is no list", `{"tests": {}}`, "", "returned no tests list", 1},
		{"a test that is not an object, after a live one",
			`{"tests": [{"id": "ID-A-01", "file": "a.feature", "live": true}, "ID-A-02"]}`,
			"ID-A-01\tfeatures/a.feature\n", "returned an invalid test", 1},
		{"a test with no file", `{"tests": [{"id": "ID-A-01", "live": true}]}`, "", "returned an invalid test", 1},
		{"a test whose id is no string", `{"tests": [{"id": 1, "file": "a.feature", "live": true}]}`, "", "returned an invalid test", 1},
		{"a test whose live is no boolean", `{"tests": [{"id": "ID-A-01", "file": "a.feature", "live": "yes"}]}`, "", "returned an invalid test", 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			var stdout, stderr strings.Builder
			code := list([]byte(c.report), &stdout, &stderr)
			if code != c.code || stdout.String() != c.stdout || !strings.Contains(stderr.String(), c.stderr) {
				t.Errorf("list said %d, stdout %q, stderr %q; want %d, %q, %q", code, stdout.String(), stderr.String(), c.code, c.stdout, c.stderr)
			}
		})
	}
}
