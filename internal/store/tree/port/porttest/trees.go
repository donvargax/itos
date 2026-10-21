package porttest

import (
	"context"
	"errors"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/store/tree/port"
)

// Trees is the tree reader in memory. Files holds each tree's files by the
// tree's name ("worktree", "index" or a commit), each a path, clean and
// slash-separated from the folder itos runs in, to its text. A path in
// Common, the git common dir, is the working tree's whatever the tree;
// Common "" is no repository, where At refuses every tree but the working
// tree and Texts has none. The zero Trees is an empty folder in no
// repository; Trees never changes its Files.
type Trees struct {
	Common string
	Files  map[string]map[string]string
}

var _ port.Trees = Trees{}

// At is the source over Files[tree].
func (t Trees) At(_ context.Context, tree string) (port.Source, error) {
	if tree != "worktree" && t.Common == "" {
		return nil, errors.New(tree + " cannot be read outside a git repository")
	}
	return source{t, tree}, nil
}

// Texts are tree's files under dir that keep accepts; none for the working
// tree or with no repository.
func (t Trees) Texts(_ context.Context, tree, dir string, keep func(string) bool) map[string]string {
	texts := map[string]string{}
	if tree == "worktree" || t.Common == "" {
		return texts
	}
	for p, text := range t.Files[tree] {
		if under(p, clean(dir)) && keep(p) {
			texts[p] = text
		}
	}
	return texts
}

// source is the tree named tree of trees.
type source struct {
	trees Trees
	tree  string
}

// at is the source that reads p: the working tree for a path in the git
// common dir, else s.
func (s source) at(p string) source {
	if s.tree != "worktree" && s.trees.Common != "" && under(clean(p), clean(s.trees.Common)) {
		return source{s.trees, "worktree"}
	}
	return s
}

// name is how a message names a tree git holds.
func (s source) name() string {
	if s.tree == "index" {
		return "the index"
	}
	return s.tree
}

func (s source) Tree(context.Context) string { return s.tree }

// Has is whether p is a file of the tree, or a folder holding one outside
// the index.
func (s source) Has(_ context.Context, p string) bool {
	s = s.at(p)
	files, p := s.trees.Files[s.tree], clean(p)
	if _, ok := files[p]; ok {
		return true
	}
	if s.tree == "index" {
		return false
	}
	for f := range files {
		if under(f, p) {
			return true
		}
	}
	return false
}

// Read is p's text, a missing one's error worded as the adapter's.
func (s source) Read(_ context.Context, p string) (string, error) {
	s = s.at(p)
	if text, ok := s.trees.Files[s.tree][clean(p)]; ok {
		return text, nil
	}
	if s.tree == "worktree" {
		return "", errors.New("ENOENT: no such file or directory, open '" + p + "'")
	}
	return "", errors.New(s.name() + " holds no " + p)
}

// List is the names directly in dir: in the working tree its files and
// folders, in a tree git holds its files.
func (s source) List(_ context.Context, dir string) ([]string, error) {
	s = s.at(dir)
	at := clean(dir)
	var names []string
	found := false
	for f := range s.trees.Files[s.tree] {
		rest, in := strings.CutPrefix(f, at+"/")
		if !in {
			continue
		}
		found = true
		name, _, deeper := strings.Cut(rest, "/")
		if (!deeper || s.tree == "worktree") && !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	if !found && s.tree == "worktree" {
		return nil, errors.New("ENOENT: no such file or directory, scandir '" + dir + "'")
	}
	if !found {
		return nil, errors.New(s.name() + " holds no folder " + dir)
	}
	slices.Sort(names)
	return names, nil
}

// clean is p clean and slash-separated.
func clean(p string) string { return path.Clean(filepath.ToSlash(p)) }

// under is whether the clean path p is dir or in it.
func under(p, dir string) bool {
	return p == dir || strings.HasPrefix(p, dir+"/")
}
