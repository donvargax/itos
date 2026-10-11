package cli

// work give-back (slice 122, features/work.feature): nothing moved an item
// in progress back to todo, while the working rules forbid editing a status
// by hand, so an item taken and then found blocked could be neither deferred
// nor picked up again, and itos guide repair's "give the item back" named no
// command. work give-back undoes a take: the status todo again, the owner
// kept, the reason the commit's body as work drop and work defer write
// theirs, the registry committed alone through writeRegistry, "docs: give
// back <id>". A command of its own rather than work defer taking doing (the
// coordinator's call, 2026-10-11), so an item given back can be taken again
// as well as deferred. It judges the owner as work take does (workPerson),
// so it takes --as.

import (
	"fmt"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
	"github.com/donvargax/itos/v8/internal/work"
)

// workGiveBack is `work give-back <id>… --why <reason> [--as <handle>]`:
// the item given back and the registry committed (work.GiveBack). A reason is
// required: it is what the commit records. The person is work take's
// (workPerson), exit 3 when nobody or not among the people. Several ids are
// each given back for the one reason (work.GiveBackEach, writeEach).
func workGiveBack(args []string, o Out) (int, error) {
	ids, flags, err := workIDs("give-back", args, nil, "--why", "--as")
	if err != nil {
		return 0, err
	}
	why := flags["--why"]
	if strings.TrimSpace(why) == "" {
		return 0, usage("work give-back needs --why <reason>, the commit's record of why the item is given back")
	}
	if len(ids) > 1 {
		return writeEach(o, "give-back", func(cfg *config.Loaded, r work.Registry, text string) (work.Several, []string, []out.Problem, error) {
			person, every, problem := workPerson(cfg, r, flags["--as"], providers.IdentityProvider(cfg))
			if problem != nil {
				return work.Several{}, nil, nil, refusal{code: ExitMissing, found: []out.Problem{*problem}}
			}
			return work.GiveBackEach(cfg, r, text, ids, person, every, why)
		}, eachLine(itemIs("given back"), ""), nil)
	}
	id := ids[0]
	cfg, registry, text, release, code, err := soundRegistry(o)
	defer release()
	if cfg == nil {
		return code, err
	}
	person, every, problem := workPerson(cfg, registry, flags["--as"], providers.IdentityProvider(cfg))
	if problem != nil {
		return refuseWork([]out.Problem{*problem}, ExitMissing, o)
	}
	change, found, err := work.GiveBack(cfg, registry, text, id, person, every, why)
	if err != nil {
		return 0, uneditable(cfg.Work.Registry, err)
	}
	if len(found) > 0 {
		return refuseWork(found, ExitPolicy, o)
	}
	sha, code, err := writeRegistry(cfg, text, change, o)
	if err != nil || code != 0 {
		return code, err
	}
	forWhom := ""
	if owner, ok := change.Item.At("owner").(string); ok && owner != "" {
		forWhom = " for " + owner
	}
	return reportWork(fmt.Sprintf("%s is given back, todo again%s: %s", id, forWhom, committed(sha, change.Header)), change, sha, nil, o)
}
