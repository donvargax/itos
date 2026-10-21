package porttest

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/store/tree/port"
)

// recorder runs TreesContract as testing does, each case on its own
// goroutine, a Fatalf ending it, and keeps what each case reported.
type recorder struct {
	name     string
	failures map[string][]string
}

func (r *recorder) Helper() {}

func (r *recorder) Context() context.Context { return context.Background() }

func (r *recorder) Errorf(format string, args ...any) {
	r.failures[r.name] = append(r.failures[r.name], fmt.Sprintf(format, args...))
}

func (r *recorder) Fatalf(format string, args ...any) {
	r.Errorf(format, args...)
	runtime.Goexit()
}

func (r *recorder) Run(name string, f func(t *recorder)) bool {
	done := make(chan struct{})
	go func() {
		defer close(done)
		f(&recorder{name: name, failures: r.failures})
	}()
	<-done
	return len(r.failures[name]) == 0
}

// broken is the fake broken for one fault.
type broken struct {
	Trees
	fault string
}

var errBroken = errors.New("broken")

func (b broken) At(ctx context.Context, tree string) (port.Source, error) {
	switch {
	case b.fault == "at fails" && b.Common != "" && tree == "HEAD":
		return nil, errBroken
	case b.fault == "at without repository" && b.Common == "":
		tree = "worktree"
	}
	s, err := b.Trees.At(ctx, tree)
	if err != nil {
		return nil, err
	}
	return brokenSource{s, b}, nil
}

func (b broken) Texts(ctx context.Context, tree, dir string, keep func(string) bool) map[string]string {
	switch {
	case b.fault == "texts ignore keep":
		keep = func(string) bool { return true }
	case b.fault == "texts of the worktree" && tree == "worktree":
		tree = "index"
	case b.fault == "texts of any commit" && b.Files[tree] == nil:
		tree = "HEAD"
	case b.fault == "texts without repository" && b.Common == "":
		return map[string]string{first: "worktree\n"}
	}
	texts := b.Trees.Texts(ctx, tree, dir, keep)
	if b.fault == "texts shallow" {
		for p := range texts {
			if strings.Count(p, "/") > 1 {
				delete(texts, p)
			}
		}
	}
	return texts
}

// brokenSource is a source broken's At opened.
type brokenSource struct {
	port.Source
	b broken
}

func (s brokenSource) Tree(ctx context.Context) string {
	if s.b.fault == "tree misnamed" {
		return "worktree"
	}
	return s.Source.Tree(ctx)
}

func (s brokenSource) Has(ctx context.Context, p string) bool {
	tree := s.Source.Tree(ctx)
	switch {
	case s.b.fault == "first lacked" && p == first:
		return false
	case s.b.fault == "commit has staged" && tree == "HEAD" && p == second,
		s.b.fault == "index has folders" && tree == "index" && p == "tasks",
		s.b.fault == "gone had" && p == gone:
		return true
	case s.b.fault == "aside unhad" && tree != "worktree" && p == aside:
		return false
	}
	return s.Source.Has(ctx, p)
}

func (s brokenSource) Read(ctx context.Context, p string) (string, error) {
	tree := s.Source.Tree(ctx)
	switch {
	case s.b.fault == "index reads worktree" && tree == "index" && p == first:
		return "worktree\n", nil
	case s.b.fault == "missing file unworded" && tree == "worktree" && p == gone,
		s.b.fault == "missing git file unworded" && tree != "worktree" && p == gone,
		s.b.fault == "aside unread" && tree != "worktree" && p == aside,
		s.b.fault == "no repository unread" && s.b.Common == "":
		return "", errBroken
	}
	return s.Source.Read(ctx, p)
}

func (s brokenSource) List(ctx context.Context, dir string) ([]string, error) {
	tree := s.Source.Tree(ctx)
	switch {
	case s.b.fault == "folders listed" && tree == "HEAD" && dir == "tasks":
		return []string{"done", "phase-1.yaml"}, nil
	case s.b.fault == "missing folder unworded" && tree == "worktree" && dir == "gone",
		s.b.fault == "missing git folder unworded" && tree != "worktree" && dir == "gone",
		s.b.fault == "aside unlisted" && tree != "worktree" && dir == ".git/itos/tasks":
		return nil, errBroken
	}
	return s.Source.List(ctx, dir)
}

// The contract fails a tree reader broken for any one of its assertions,
// each case reporting what it asserts and no other case reporting anything:
// a contract that cannot fail proves nothing.
func TestTreesContractFailsEachBrokenReader(t *testing.T) {
	for _, c := range []struct {
		fault string
		want  map[string][]string
	}{
		{"", nil},
		{"at fails", map[string][]string{"reads": {"At(HEAD): "}, "folders": {"At(HEAD): "}, "missing": {"At(HEAD): "}, "git folder": {"At(HEAD): "}}},
		{"tree misnamed", map[string][]string{"reads": {"Tree: At(index)", "Tree: At(HEAD)"}}},
		{"index reads worktree", map[string][]string{"reads": {"Read: tasks/phase-1.yaml at index"}}},
		{"folders listed", map[string][]string{"reads": {"List: tasks at HEAD"}}},
		{"first lacked", map[string][]string{"reads": {"Has: worktree lacks", "Has: index lacks", "Has: HEAD lacks"}}},
		{"commit has staged", map[string][]string{"reads": {"a staged file: HEAD"}}},
		{"index has folders", map[string][]string{"folders": {"a folder: index"}}},
		{"missing file unworded", map[string][]string{"missing": {"a missing file at the worktree"}}},
		{"missing folder unworded", map[string][]string{"missing": {"a missing folder at the worktree"}}},
		{"missing git file unworded", map[string][]string{"missing": {"a missing file at index", "a missing file at HEAD"}}},
		{"missing git folder unworded", map[string][]string{"missing": {"a missing folder at index", "a missing folder at HEAD"}}},
		{"gone had", map[string][]string{"missing": {"Has: worktree has the missing", "Has: index has the missing", "Has: HEAD has the missing"}}},
		{"aside unhad", map[string][]string{"git folder": {"the git folder: index lacks", "the git folder: HEAD lacks"}}},
		{"aside unread", map[string][]string{"git folder": {"the git folder: index reads", "the git folder: HEAD reads"}}},
		{"aside unlisted", map[string][]string{"git folder": {"the git folder: index lists", "the git folder: HEAD lists"}}},
		{"texts ignore keep", map[string][]string{"texts": {"Texts: tasks at index"}}},
		{"texts shallow", map[string][]string{"texts": {"Texts: tasks at index", "Texts: tasks at HEAD"}}},
		{"texts of the worktree", map[string][]string{"texts": {"Texts: tasks at worktree"}}},
		{"texts of any commit", map[string][]string{"texts": {"Texts: tasks at no-such-commit"}}},
		{"no repository unread", map[string][]string{"no repository": {"no repository's worktree reads"}}},
		{"at without repository", map[string][]string{"no repository": {"no repository's index", "no repository's HEAD"}}},
		{"texts without repository", map[string][]string{"no repository": {"no repository's texts"}}},
	} {
		r := &recorder{failures: map[string][]string{}}
		TreesContract(r, func(_ *recorder, repository bool) port.Trees {
			return broken{layout(repository), c.fault}
		})
		for name, got := range r.failures {
			want := c.want[name]
			ok := len(got) == len(want)
			for i := 0; ok && i < len(got); i++ {
				ok = strings.HasPrefix(got[i], want[i])
			}
			if !ok {
				t.Errorf("%q: case %q reported %q; want %q", c.fault, name, got, want)
			}
		}
		for name := range c.want {
			if _, ok := r.failures[name]; !ok {
				t.Errorf("%q: case %q reported nothing; want %q", c.fault, name, c.want[name])
			}
		}
	}
}
