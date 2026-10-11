package tests

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/source"
)

// StagesTestFile is whether one of paths, from the repository's top as git
// names them, is a test file of the kind (slice 117: the commit-msg hook runs
// the kind's smoke rule when one is staged). On a Gherkin kind it is a
// .feature file under the kind's root, as the adapter reads them, so a
// deletion counts too. On a command kind it is a file the adapter lists at
// the index or, for a path the current source lacks (a deletion, under the
// hook's reading of the index), at before, a commit; a tree that names no
// commit lists nothing, and a listing that fails names no file.
func StagesTestFile(cfg *config.Loaded, name string, paths []string, before string) bool {
	k, err := KindOf(cfg, name)
	if err != nil {
		return false
	}
	if k.Adapter.Command == "" {
		return featureAmong(k, paths)
	}
	m := NewMoves(cfg)
	list, err := m.testList(name, "index")
	if err != nil {
		return false
	}
	if listedAmong(k, list, paths) {
		return true
	}
	var gone []string
	for _, p := range paths {
		if !source.Has(p) {
			gone = append(gone, p)
		}
	}
	if len(gone) == 0 {
		return false
	}
	list, err = m.testList(name, before)
	return err == nil && listedAmong(k, list, gone)
}

// featureAmong is whether one of paths is a .feature file under the Gherkin
// kind's root, or anywhere when the kind names none.
func featureAmong(k config.Kind, paths []string) bool {
	root := "."
	if k.Root != nil {
		root = path.Clean(filepath.ToSlash(*k.Root))
	}
	for _, p := range paths {
		p = path.Clean(p)
		if strings.HasSuffix(p, ".feature") && (root == "." || strings.HasPrefix(p, root+"/")) {
			return true
		}
	}
	return false
}

// listedAmong is whether one of paths is a file of the listing, its path
// joined to the kind's root when it has one.
func listedAmong(k config.Kind, list List, paths []string) bool {
	files := map[string]bool{}
	for _, f := range list.Files {
		if k.Root != nil {
			f = path.Join(filepath.ToSlash(*k.Root), f)
		}
		files[path.Clean(f)] = true
	}
	for _, p := range paths {
		if files[path.Clean(p)] {
			return true
		}
	}
	return false
}
