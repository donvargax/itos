package ledger

import (
	"regexp"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/value"
)

// sliceID and bugID are the ids of a slice and a bug, as the registry's
// header names them.
var (
	sliceID = regexp.MustCompile(`^slice-\d+$`)
	bugID   = regexp.MustCompile(`^bug-\d+$`)
)

// KindOf is the kind of a registry item, which decides what its spec is: its
// kind key as written when it is text, else by its id (work show reads no
// bug), slice-<n> a slice, bug-<n> a bug, an id ledger.id matches a task,
// where the config has a ledger.id; "" for none.
func KindOf(item *value.Map, cfg *config.Loaded) string {
	if kind, ok := item.At("kind").(string); ok {
		return kind
	}
	id, _ := item.At("id").(string)
	switch {
	case sliceID.MatchString(id):
		return "slice"
	case bugID.MatchString(id):
		return "bug"
	case cfg.Ledger.ID != nil && IDPattern(cfg).MatchString(id):
		return "task"
	}
	return ""
}

// ChecksOf are the done_when checks of the ledger's task id, the first entry
// of that id giving them, and whether the ledger has an entry for it: a task's
// spec. A ledger that cannot be read is Tasks' error.
func ChecksOf(cfg *config.Loaded, id string) (checks []Check, entry bool, err error) {
	tasks, err := Tasks(cfg)
	if err != nil {
		return nil, false, err
	}
	for _, t := range tasks {
		if t.ID == id {
			return t.DoneWhen, true, nil
		}
	}
	return nil, false, nil
}
