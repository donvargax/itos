package main

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

// listing is go list -e -test as it gives a module with a package a, which b imports, and a's
// external test package: a test variant, the test main and a Dep recompiled for the test.
const listing = `example.com/m/a|/repo/a|
example.com/m/a [example.com/m/a.test]|/repo/a|
example.com/m/a.test|/repo/a|example.com/m/a example.com/m/a_test [example.com/m/a.test]
example.com/m/a_test [example.com/m/a.test]|/repo/a|example.com/m/a
example.com/m/a/deep|/repo/a/deep|
example.com/m/b|/repo/b|example.com/m/a
example.com/m/b.test|/repo/b|example.com/m/b example.com/m/a [example.com/m/b.test]
example.com/m/c|/repo/c|
`

func TestAffected(t *testing.T) {
	for _, c := range []struct {
		name    string
		changed []string
		root    string
		listing string
		want    []string
	}{
		{"a file belongs to its package and to those that import it", []string{"a/a.go"}, "/repo", listing,
			[]string{"example.com/m/a", "example.com/m/b"}},
		{"a package nothing imports is alone", []string{"c/c_test.go"}, "/repo", listing, []string{"example.com/m/c"}},
		{"the deepest folder holds the file", []string{"a/deep/x.go"}, "/repo", listing, []string{"example.com/m/a/deep"}},
		{"testdata counts as its package", []string{"b/testdata/in.txt"}, "/repo", listing, []string{"example.com/m/b"}},
		{"a file outside every package reaches none", []string{"docs/x.md"}, "/repo", listing, nil},
		{"two files reach both packages, once each", []string{"c/c.go", "b/b.go"}, "/repo", listing,
			[]string{"example.com/m/b", "example.com/m/c"}},
		{"a native Windows folder is compared as a slash path", []string{"a/a.go"}, `C:/repo`,
			strings.NewReplacer("/repo/a", `C:\repo\a`, "/repo/b", `C:\repo\b`, "/repo/c", `C:\repo\c`).Replace(listing),
			[]string{"example.com/m/a", "example.com/m/b"}},
		{"no listing reaches none", []string{"a/a.go"}, "/repo", "", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := affected(c.changed, c.root, c.listing); !slices.Equal(got, c.want) {
				t.Errorf("affected(%q) = %q, want %q", c.changed, got, c.want)
			}
		})
	}
}

func TestAnyGoBuilt(t *testing.T) {
	for file, want := range map[string]bool{
		"cmd/itos/main.go": true, "internal/x/x.go": true, "go.mod": true, "go.sum": true,
		"README.md": false, "docs/x.md": false, "tools/bin/pre-commit": false, "internal": false, "sub/go.mod": false,
	} {
		if got := anyGoBuilt([]string{file}); got != want {
			t.Errorf("anyGoBuilt(%q) = %v, want %v", file, got, want)
		}
	}
}

func TestWithoutDropsNamesWhateverTheirCase(t *testing.T) {
	got := without([]string{"PATH=/bin", "GIT_DIR=/x", "git_index_file=/y", "KEEP=GIT_DIR=1"}, handed)
	if want := []string{"PATH=/bin", "KEEP=GIT_DIR=1"}; !slices.Equal(got, want) {
		t.Errorf("without = %q, want %q", got, want)
	}
}

func TestSplitNUL(t *testing.T) {
	if got := splitNUL("a b\x00c\x00"); !slices.Equal(got, []string{"a b", "c"}) {
		t.Errorf("splitNUL = %q", got)
	}
	if got := splitNUL(""); len(got) != 0 {
		t.Errorf("splitNUL of nothing = %q", got)
	}
}

func TestUsage(t *testing.T) {
	for _, args := range [][]string{nil, {""}, {"-x"}, {"--cached", "extra"}} {
		var out, errw bytes.Buffer
		if code := run(args, ".", &out, &errw); code != 2 || !strings.Contains(errw.String(), "usage:") {
			t.Errorf("run(%q) = %d, %q; want 2 and the usage", args, code, errw.String())
		}
	}
}
