package porttest

import (
	"testing"

	"github.com/donvargax/itos/v8/internal/store/tree/port"
)

// layout is the fake laid out as TreesContract asks: a repository whose git
// folder is .git, or a folder in none.
func layout(repository bool) Trees {
	if !repository {
		return Trees{Files: map[string]map[string]string{"worktree": {first: "worktree\n"}}}
	}
	return Trees{Common: ".git", Files: map[string]map[string]string{
		"HEAD":     {done: "zero\n", first: "one\n"},
		"index":    {done: "zero\n", first: "staged\n", second: "staged two\n"},
		"worktree": {done: "zero\n", first: "worktree\n", second: "staged two\n", aside: "aside\n"},
	}}
}

// The fake reads its trees as the adapter reads the disk and git.
func TestTreesKeepTheContract(t *testing.T) {
	TreesContract(t, func(_ *testing.T, repository bool) port.Trees { return layout(repository) })
}
