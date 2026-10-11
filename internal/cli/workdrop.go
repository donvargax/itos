package cli

// work drop (slice 78, features/work.feature): nothing took an item out of
// the registry, so ideas only piled up. work drop sets an item still to do
// to the status dropped, drops its why as work done drops a closed item's,
// takes it out of the queue and commits the registry alone through
// writeRegistry, "docs: drop <id>", the reason given as the commit's body:
// the commit is the record of why. The item stays in the registry, so its
// id is never given again.

import (
	"fmt"
	"strings"

	"github.com/donvargax/itos/v7/internal/config"
	"github.com/donvargax/itos/v7/internal/out"
	"github.com/donvargax/itos/v7/internal/work"
)

// workDrop is `work drop <id>… --why <reason>`: the item dropped and the
// registry committed (work.Drop). A reason is required: it is what the
// commit records. Several ids are each dropped for the one reason
// (work.DropEach, writeEach).
func workDrop(args []string, o Out) (int, error) {
	ids, flags, err := workIDs("drop", args, nil, "--why")
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(flags["--why"]) == "" {
		return 0, usage("work drop needs --why <reason>, the commit's record of why the item is dropped")
	}
	if len(ids) > 1 {
		return writeEach(o, "drop", func(cfg *config.Loaded, r work.Registry, text string) (work.Several, []string, []out.Problem, error) {
			return work.DropEach(cfg, r, text, ids, flags["--why"])
		}, eachLine(itemIs("dropped"), "is already dropped"), nil)
	}
	id := ids[0]
	cfg, registry, text, release, code, err := soundRegistry(o)
	defer release()
	if cfg == nil {
		return code, err
	}
	change, found, err := work.Drop(cfg, registry, text, id, flags["--why"])
	if err != nil {
		return 0, uneditable(cfg.Work.Registry, err)
	}
	if len(found) > 0 {
		return refuseWork(found, ExitPolicy, o)
	}
	if change.Unchanged {
		return reportWork(id+" is already dropped; nothing to change", change, "", nil, o)
	}
	sha, code, err := writeRegistry(cfg, text, change, o)
	if err != nil || code != 0 {
		return code, err
	}
	return reportWork(fmt.Sprintf("%s is dropped: %s", id, committed(sha, change.Header)), change, sha, nil, o)
}
