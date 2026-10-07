package cli

// work add and work edit (slice 54, features/work.feature): the rest of the
// registry written by commands, as take, promote and done write it
// (workwrite.go). add makes an item, edit changes one's title, dependencies,
// refs or tags (slice 97), or adds a paragraph to its why; each judges the result as work
// check does and commits the registry alone through writeRegistry. An
// owner changes by work take, a kind by work promote, a status by take and
// done, so edit has no flag for those. A note on a slice or a task is
// refused (slice 79; slice 76 warned of it), naming where its why belongs,
// the registry's whys being an idea's.

import (
	"fmt"
	"slices"
	"strings"

	"github.com/donvargax/itos/v6/internal/ledger"
	"github.com/donvargax/itos/v6/internal/out"
	"github.com/donvargax/itos/v6/internal/work"
)

// addKinds are the kinds work add makes.
var addKinds = []string{"idea", "slice", "task"}

// idList is a flag's comma-separated ids, each trimmed, the empty ones left
// out: "" is none.
func idList(s string) []string {
	list := []string{}
	for _, id := range strings.Split(s, ",") {
		if id = strings.TrimSpace(id); id != "" {
			list = append(list, id)
		}
	}
	return list
}

// workAdd is `work add <id> --title <t> --why <w> [--kind idea|slice|task]
// [--phase <p>] [--owner <handle>] [--depends-on <ids>] [--refs <refs>]
// [--tags <tags>]`: a new item, todo, at the end of the registry, committed
// (work.Add).
func workAdd(args []string, o Out) (int, error) {
	id, flags, err := workArgs("add", args, "--title", "--why", "--kind", "--phase", "--owner", "--depends-on", "--refs", "--tags")
	if err != nil {
		return 0, err
	}
	if flags["--title"] == "" || strings.TrimSpace(flags["--why"]) == "" {
		return 0, usage("work add needs --title <title> and --why <why>")
	}
	kind := flags["--kind"]
	if kind == "" {
		kind = "idea"
	}
	if !slices.Contains(addKinds, kind) {
		return 0, usage("work add takes --kind %s", strings.Join(addKinds, "|"))
	}
	cfg, registry, text, release, code, err := soundRegistry(o)
	defer release()
	if cfg == nil {
		return code, err
	}
	n := work.New{
		ID: id, Title: flags["--title"], Why: flags["--why"], Kind: kind, Phase: flags["--phase"], Owner: flags["--owner"],
		DependsOn: idList(flags["--depends-on"]), Refs: idList(flags["--refs"]), Tags: idList(flags["--tags"]),
	}
	change, found, err := work.Add(cfg, registry, text, n, ledger.IDPattern(cfg))
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
	return reportWork(fmt.Sprintf("%s is a new %s: %s", id, kind, committed(sha, change.Header)), change, sha, nil, o)
}

// workEdit is `work edit <id> [--title <t>] [--depends-on <ids>] [--refs
// <refs>] [--tags <tags>] [--note <paragraph>]`: the item's title,
// depends_on, refs or tags replaced, a paragraph added to its why, committed
// (work.Edit). An empty --depends-on, --refs or --tags empties the list.
func workEdit(args []string, o Out) (int, error) {
	id, flags, err := workArgsEmpty("edit", args, []string{"--depends-on", "--refs", "--tags"}, "--title", "--depends-on", "--refs", "--tags", "--note")
	if err != nil {
		return 0, err
	}
	if len(flags) == 0 {
		return 0, usage("work edit needs --title, --depends-on, --refs, --tags or --note")
	}
	var e work.Edits
	if title, ok := flags["--title"]; ok {
		e.Title = &title
	}
	if deps, ok := flags["--depends-on"]; ok {
		list := idList(deps)
		e.DependsOn = &list
	}
	if refs, ok := flags["--refs"]; ok {
		list := idList(refs)
		e.Refs = &list
	}
	if tags, ok := flags["--tags"]; ok {
		list := idList(tags)
		e.Tags = &list
	}
	e.Note = flags["--note"]
	cfg, registry, text, release, code, err := soundRegistry(o)
	defer release()
	if cfg == nil {
		return code, err
	}
	change, found, err := work.Edit(cfg, registry, text, id, e, taskIDs(cfg))
	if err != nil {
		return 0, uneditable(cfg.Work.Registry, err)
	}
	if len(found) > 0 {
		return refuseWork(found, ExitPolicy, o)
	}
	changed := make([]any, len(change.Changed))
	for i, key := range change.Changed {
		changed[i] = key
	}
	extra := []out.Field{{Key: "changed", Value: changed}}
	if change.Unchanged {
		return reportWork(id+" is already so; nothing to change", change, "", extra, o)
	}
	sha, code, err := writeRegistry(cfg, text, change, o)
	if err != nil || code != 0 {
		return code, err
	}
	line := fmt.Sprintf("%s's %s changed: %s", id, strings.Join(change.Changed, ", "), committed(sha, change.Header))
	return reportWork(line, change, sha, extra, o)
}
