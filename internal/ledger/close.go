package ledger

import (
	"fmt"
	"os"

	"github.com/donvargax/itos/v7/internal/config"
	"github.com/donvargax/itos/v7/internal/value"
)

// Edited is a ledger file edited in place: its path, and its text before and
// after.
type Edited struct {
	Path, Old, Text string
}

// WithoutChecks is the ledger with the task's checks taken out, for itos
// work done (slice 101): a task's checks are the agent's progress while it
// is open, and once they have gated its close they leave the ledger, git's
// history keeping them. The task's done_when goes, key and list, from the
// file that holds the task (the first, in the order Files gives, when two
// do), edited in place (value.Doc's Drop): every other line, comment and
// quote kept, a block task's or a flow one's, the file's line breaks as they
// were. The task's why, its spec, stays. ok is false, with nothing to write,
// when no ledger file has the task or the task has no checks (no done_when,
// or an empty one).
func WithoutChecks(cfg *config.Loaded, id string) (edited Edited, ok bool, err error) {
	files, err := Files(cfg)
	if err != nil {
		return Edited{}, false, err
	}
	for _, f := range files {
		raw, err := os.ReadFile(f.Path)
		if err != nil {
			return Edited{}, false, err
		}
		text := string(raw)
		tasks, err := value.Parse(text)
		if err != nil {
			return Edited{}, false, fmt.Errorf("%s: %w", f.Path, err)
		}
		list, _ := tasks.([]any)
		for i, task := range list {
			if value.Prop(task, "id") != id {
				continue
			}
			if checks, _ := value.Prop(task, "done_when").([]any); len(checks) == 0 {
				return Edited{}, false, nil
			}
			doc, err := value.OpenDoc(text)
			if err != nil {
				return Edited{}, false, fmt.Errorf("%s cannot be edited in place: %w", f.Path, err)
			}
			if err := doc.Drop([]any{i, "done_when"}); err != nil {
				return Edited{}, false, fmt.Errorf("%s cannot be edited in place: %w", f.Path, err)
			}
			after, err := doc.Text()
			if err != nil {
				return Edited{}, false, fmt.Errorf("%s cannot be edited in place: %w", f.Path, err)
			}
			return Edited{Path: f.Path, Old: text, Text: after}, true, nil
		}
	}
	return Edited{}, false, nil
}
