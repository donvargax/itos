package tests

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Row is one live test as tests list --tsv prints it: its ID, its file from
// the project's top, slash separated, and its fingerprint, "" for a test
// without one.
type Row struct {
	ID, File, Fingerprint string
}

// Line is the row as one line: its ID, a tab and its file, then a tab and its
// fingerprint when it has one, and an LF.
func (r Row) Line() string {
	if r.Fingerprint == "" {
		return r.ID + "\t" + r.File + "\n"
	}
	return r.ID + "\t" + r.File + "\t" + r.Fingerprint + "\n"
}

// TSV is the listing's live tests as rows, in its order, each file joined to
// root, the kind's ("" for none: a command kind's files are then the
// project's), and an absolute one read from project, the folder itos runs
// in. A live test no row can hold refuses them all: the second answer names
// each, by its place in the listing, and why: an empty ID or file, a tab, a
// carriage return, a newline or a NUL in one, an ID another live test has, a
// file outside the project.
func TSV(list List, root, project string) ([]Row, []string) {
	var rows []Row
	var refused []string
	first := map[string]int{}
	for i, t := range list.Tests {
		if !t.Live {
			continue
		}
		file, why := projectFile(root, t.File, project)
		whys := []string{}
		for _, field := range [][2]string{{"id", t.ID}, {"file", t.File}} {
			if bad := unprintable(field[1]); bad != "" {
				whys = append(whys, "its "+field[0]+" "+bad)
			}
		}
		if at, seen := first[t.ID]; seen && t.ID != "" {
			whys = append(whys, fmt.Sprintf("it has the same id as test %d", at))
		} else {
			first[t.ID] = i + 1
		}
		if why != "" && unprintable(t.File) == "" {
			whys = append(whys, why)
		}
		if len(whys) > 0 {
			refused = append(refused, fmt.Sprintf("test %d (id %q, file %q): %s", i+1, t.ID, t.File, strings.Join(whys, "; ")))
			continue
		}
		rows = append(rows, Row{ID: t.ID, File: file, Fingerprint: t.Fingerprint})
	}
	if len(refused) > 0 {
		return nil, refused
	}
	return rows, nil
}

// unprintable is what keeps a field out of a row: "is empty", or the first
// separator it holds; "" when it can be printed.
func unprintable(field string) string {
	if field == "" {
		return "is empty"
	}
	for _, c := range []struct{ char, name string }{{"\t", "a tab"}, {"\r", "a carriage return"}, {"\n", "a newline"}, {"\x00", "a NUL"}} {
		if strings.Contains(field, c.char) {
			return "holds " + c.name
		}
	}
	return ""
}

// projectFile is a test's file from the project's top, slash separated: the
// file joined to root, an absolute one made relative to project; or why it
// is outside the project.
func projectFile(root, file, project string) (string, string) {
	p := filepath.FromSlash(file)
	if root != "" && !filepath.IsAbs(p) {
		p = filepath.Join(filepath.FromSlash(root), p)
	}
	if filepath.IsAbs(p) {
		rel, err := filepath.Rel(project, p)
		if project == "" || err != nil {
			return "", "its file is outside the project"
		}
		p = rel
	}
	p = filepath.Clean(p)
	volume := filepath.VolumeName(p)
	p = filepath.ToSlash(p)
	if p == "." || p == ".." || strings.HasPrefix(p, "../") || strings.HasPrefix(p, "/") || volume != "" {
		return "", "its file is outside the project"
	}
	return p, ""
}
