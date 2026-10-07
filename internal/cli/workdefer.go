package cli

// work defer and work resume (slice 98, features/work.feature): an item was
// put off only by writing its deferred key by hand, which the working rules
// forbid for an item's status and owner. work defer sets the key to the
// reason and work resume removes it, each committing the registry alone
// through writeRegistry, "docs: defer <id>" with the reason the commit's body
// as work drop has it, and "docs: resume <id>". Two commands, not a flag on
// one: a flag changes an action and never selects another (docs/CLI.md rule
// 18).

import (
	"fmt"
	"strings"

	"github.com/donvargax/itos/v7/internal/work"
)

// workDefer is `work defer <id> --why <reason>`: the item deferred and the
// registry committed (work.Defer). A reason is required: it is what the
// deferred key holds and the commit records.
func workDefer(args []string, o Out) (int, error) {
	id, flags, err := workArgs("defer", args, "--why")
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(flags["--why"]) == "" {
		return 0, usage("work defer needs --why <reason>, why the item is put off, which its deferred key holds")
	}
	cfg, registry, text, release, code, err := soundRegistry(o)
	defer release()
	if cfg == nil {
		return code, err
	}
	change, found, err := work.Defer(cfg, registry, text, id, flags["--why"])
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
	return reportWork(fmt.Sprintf("%s is deferred: %s", id, committed(sha, change.Header)), change, sha, nil, o)
}

// workResume is `work resume <id>`: the item's deferral lifted and the
// registry committed (work.Resume).
func workResume(args []string, o Out) (int, error) {
	id, _, err := workArgs("resume", args)
	if err != nil {
		return 0, err
	}
	cfg, registry, text, release, code, err := soundRegistry(o)
	defer release()
	if cfg == nil {
		return code, err
	}
	change, found, err := work.Resume(cfg, registry, text, id)
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
	return reportWork(fmt.Sprintf("%s is resumed: %s", id, committed(sha, change.Header)), change, sha, nil, o)
}
